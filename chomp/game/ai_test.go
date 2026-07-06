package game

import (
	"math/rand"
	"testing"
)

// TestWinningForMoverSingleCell: only the poisoned cell left is always a
// loss for whoever must move — their sole legal move IS the poison.
func TestWinningForMoverSingleCell(t *testing.T) {
	s := State{1}
	if WinningForMover(s) {
		t.Fatal("a board with only the poisoned cell left must be a LOSS for the mover")
	}
	m, ok := BestMove(s)
	if !ok {
		t.Fatal("BestMove must still return the forced poison move")
	}
	if !m.IsPoison() {
		t.Fatalf("BestMove on {1} = %v, want the poisoned cell (0,0)", m)
	}
}

// TestWinningForMoverOneRow: a single row of length > 1 is always a win for
// the mover — eat everything except the poison in one move, by hand:
// Apply({0,1}) on {n} clamps the row to column 1, leaving just {1} (the
// poison alone), which TestWinningForMoverSingleCell already shows is a
// loss for whoever moves next. This is verified independently here by
// direct arithmetic on State, not by trusting solve() to agree with itself.
func TestWinningForMoverOneRow(t *testing.T) {
	for n := 2; n <= 6; n++ {
		s := State{n}
		if !WinningForMover(s) {
			t.Fatalf("a single row of length %d must be a win for the mover", n)
		}
		m, ok := BestMove(s)
		if !ok {
			t.Fatalf("BestMove must return a move for {%d}", n)
		}
		if m.IsPoison() {
			t.Fatalf("BestMove on a winning row {%d} must not resign by eating the poison", n)
		}
		child := s.Apply(m)
		if WinningForMover(child) {
			t.Fatalf("BestMove(%v) = %v left the opponent %v in a WINNING state; want a losing one", s, m, child)
		}
	}
}

// TestWinningForMover2x2HandSolved is a fully hand-worked, independent
// solution of the classic 2x2 Chomp game (see the comment for the by-hand
// derivation this mirrors):
//
//	Start {2,2}. The only sensible first move is (1,1) (the cell diagonally
//	opposite the poison), leaving {2,1}. From {2,1} the mover has two
//	non-poison replies: (0,1) -> {1,1}, or (1,0) -> {2,0}.
//	  {1,1}: the only non-poison move is (1,0) -> {1,0} (poison alone) which
//	         is a loss for ITS mover, so {1,1} is a WIN for its mover.
//	  {2,0}: the only non-poison move is (0,1) -> {1,0} (poison alone) which
//	         is a loss for ITS mover, so {2,0} is a WIN for its mover.
//	Since BOTH of {2,1}'s children are wins for their own mover (i.e. wins
//	for whoever replies to {2,1}), {2,1} itself is a LOSS for its mover.
//	Therefore {2,2} is a WIN for the first mover, achieved via (1,1).
func TestWinningForMover2x2HandSolved(t *testing.T) {
	full := State{2, 2}
	if !WinningForMover(full) {
		t.Fatal("2x2 Chomp must be a win for the first player (strategy-stealing theorem)")
	}

	afterDiagonal := full.Apply(Move{Row: 1, Col: 1})
	if !equalState(afterDiagonal, State{2, 1}) {
		t.Fatalf("Apply({1,1}) on {2,2} = %v, want {2,1}", afterDiagonal)
	}
	if WinningForMover(afterDiagonal) {
		t.Fatal("{2,1} must be a LOSS for its mover, by the hand-solved game tree above")
	}

	oneOne := afterDiagonal.Apply(Move{Row: 0, Col: 1})
	if !equalState(oneOne, State{1, 1}) {
		t.Fatalf("Apply({0,1}) on {2,1} = %v, want {1,1}", oneOne)
	}
	if !WinningForMover(oneOne) {
		t.Fatal("{1,1} must be a WIN for its mover, by the hand-solved game tree above")
	}

	twoZero := afterDiagonal.Apply(Move{Row: 1, Col: 0})
	if !equalState(twoZero, State{2, 0}) {
		t.Fatalf("Apply({1,0}) on {2,1} = %v, want {2,0}", twoZero)
	}
	if !WinningForMover(twoZero) {
		t.Fatal("{2,0} must be a WIN for its mover, by the hand-solved game tree above")
	}

	// BestMove on the winning {2,2} position must actually pick the
	// diagonal cell — it's the unique cell whose removal leaves a losing
	// position for the opponent in this hand-solved tree.
	m, ok := BestMove(full)
	if !ok || m != (Move{Row: 1, Col: 1}) {
		t.Fatalf("BestMove({2,2}) = %v (ok=%v), want (1,1)", m, ok)
	}
}

// TestAIPerfectPlayNeverLosesExhaustive plays out the ENTIRE game tree for
// the 2x2 board (small enough to enumerate completely): whichever side is
// given the winning first move via BestMove must win against literally
// every possible sequence of opponent replies, including the opponent
// eating the poison outright at any point.
func TestAIPerfectPlayNeverLosesExhaustive(t *testing.T) {
	start := NewState(2, 2)
	if !WinningForMover(start) {
		t.Fatal("setup: {2,2} must be a winning position for its first mover")
	}
	verifyPerfectMoverAlwaysWins(t, start, true)
}

// verifyPerfectMoverAlwaysWins recursively checks every branch of the game
// tree from s. When aiTurn, the AI's own BestMove is played (it must never
// resign by eating the poison, since we only ever enter an aiTurn call on a
// state that is winning for its mover). When !aiTurn, EVERY legal reply
// (including the poison) is tried, adversarially, and the AI must still
// come out on top down every one of those branches.
func verifyPerfectMoverAlwaysWins(t *testing.T, s State, aiTurn bool) {
	t.Helper()
	if aiTurn {
		m, ok := BestMove(s)
		if !ok {
			t.Fatalf("AI has no move at state %v", s)
		}
		if m.IsPoison() {
			t.Fatalf("AI ate the poison at state %v while it should hold a winning position", s)
		}
		verifyPerfectMoverAlwaysWins(t, s.Apply(m), false)
		return
	}
	for _, m := range s.LegalMoves() {
		if m.IsPoison() {
			continue // the opponent conceding by self-poisoning is fine: an immediate AI win, nothing to recurse into.
		}
		verifyPerfectMoverAlwaysWins(t, s.Apply(m), true)
	}
}

// TestAIPerfectPlayNeverLosesRandomized runs the same "AI must never lose
// its own winning position" property against a pseudo-random adversary on
// every board size actually offered on the menu (Sizes), where full
// exhaustive enumeration would be far too slow. Confirms both the perfect AI
// never loses AND that every offered board size is (as Chomp's
// strategy-stealing theorem guarantees for any rectangle bigger than 1x1) a
// first-player win.
func TestAIPerfectPlayNeverLosesRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, sz := range Sizes {
		start := NewState(sz.Rows, sz.Cols)
		if !WinningForMover(start) {
			t.Fatalf("%s (%dx%d) should be a first-player win", sz.Name, sz.Rows, sz.Cols)
		}
		for trial := 0; trial < 12; trial++ {
			s := start
			aiTurn := true
			for ply := 0; ; ply++ {
				if ply > 200 {
					t.Fatalf("%s: game did not terminate", sz.Name)
				}
				if aiTurn {
					m, ok := BestMove(s)
					if !ok {
						t.Fatalf("%s: AI has no move at %v", sz.Name, s)
					}
					if m.IsPoison() {
						t.Fatalf("%s: AI ate the poison at %v (trial %d)", sz.Name, s, trial)
					}
					s = s.Apply(m)
					aiTurn = false
					continue
				}
				// Adversary: a uniformly random legal move, poison included.
				moves := s.LegalMoves()
				m := moves[rng.Intn(len(moves))]
				s = s.Apply(m)
				if m.IsPoison() {
					break // opponent poisoned itself: AI wins this trial.
				}
				aiTurn = true
			}
		}
	}
}

// TestBestMoveNeverIllegal fuzzes BestMove against many reachable states and
// checks the move it returns is always actually legal there.
func TestBestMoveNeverIllegal(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	s := NewState(6, 7)
	for !s.Empty() {
		m, ok := BestMove(s)
		if !ok {
			t.Fatal("BestMove returned ok=false on a non-empty board")
		}
		if !s.IsLegal(m) {
			t.Fatalf("BestMove(%v) = %v is not a legal move on that board", s, m)
		}
		s = s.Apply(m)
		if m.IsPoison() {
			break
		}
		// Alternate with a random legal move so BestMove sees a variety of
		// mid-game shapes, not just the perfect-play line.
		moves := s.LegalMoves()
		if len(moves) == 0 {
			break
		}
		mv := moves[rng.Intn(len(moves))]
		s = s.Apply(mv)
		if mv.IsPoison() {
			break
		}
	}
}
