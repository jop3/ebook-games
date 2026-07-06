package game

import (
	"math/rand"
	"testing"
)

func TestNewGameStartsInSetupPhase(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy)
	if s.Phase != PhaseSetup {
		t.Fatalf("Phase = %v, want PhaseSetup", s.Phase)
	}
	if s.Turn != Black {
		t.Fatalf("Turn = %v, want Black to place first", s.Turn)
	}
	for _, side := range [2]Side{Black, White} {
		for _, typ := range AllTypes {
			if got := s.RemainingCount(side, typ); got != StartCount(typ) {
				t.Errorf("RemainingCount(%v,%v) = %d, want %d", side, typ, got, StartCount(typ))
			}
		}
	}
}

// TestSetupAlternatesAndEndsAt60 places all 60 pieces (30 each) via
// PlacePiece directly and checks the turn strictly alternates, the phase
// flips to PhasePlaying only once BOTH sides have placed everything, and the
// board ends up with exactly 30 stacks per side.
func TestSetupAlternatesAndEndsAt60(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy)
	pts := AllPoints()
	wantTurn := Black
	for i := 0; i < TotalPerSide*2; i++ {
		if s.Turn != wantTurn {
			t.Fatalf("placement %d: turn = %v, want %v", i, s.Turn, wantTurn)
		}
		if s.Phase != PhaseSetup {
			t.Fatalf("placement %d: phase ended early at %v", i, s.Phase)
		}
		avail := s.AvailableTypes(s.Turn)
		if len(avail) == 0 {
			t.Fatalf("placement %d: %v has nothing left to place", i, s.Turn)
		}
		if !s.PlacePiece(avail[0], pts[i]) {
			t.Fatalf("placement %d at %v should be accepted", i, pts[i])
		}
		wantTurn = wantTurn.Opponent()
	}
	if s.Phase != PhasePlaying {
		t.Fatalf("after 60 placements, phase should be Playing, got %v", s.Phase)
	}
	if s.Board.StackCount(Black) != TotalPerSide || s.Board.StackCount(White) != TotalPerSide {
		t.Fatalf("each side should have %d stacks, got B=%d W=%d", TotalPerSide,
			s.Board.StackCount(Black), s.Board.StackCount(White))
	}
	for _, side := range [2]Side{Black, White} {
		for _, typ := range AllTypes {
			if got := s.Board.TypeCount(side, typ); got != StartCount(typ) {
				t.Errorf("board TypeCount(%v,%v) = %d, want %d", side, typ, got, StartCount(typ))
			}
			if got := s.RemainingCount(side, typ); got != 0 {
				t.Errorf("RemainingCount(%v,%v) after full setup = %d, want 0", side, typ, got)
			}
		}
	}
}

// TestPlacePieceRejectsOccupiedAndExhaustedType.
func TestPlacePieceRejectsOccupiedAndExhaustedType(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy)
	p := AllPoints()[0]
	if !s.PlacePiece(Tzaar, p) {
		t.Fatal("first placement should succeed")
	}
	// It's now White's turn; White placing on the SAME occupied cell must fail.
	if s.PlacePiece(Tzaar, p) {
		t.Fatal("placing on an occupied cell must fail")
	}
	if s.Turn != White {
		t.Fatal("a rejected placement must not have advanced the turn")
	}
	// Exhaust White's Tzaar allotment (6), then a 7th attempt must fail.
	s.Remaining[White][Tzaar] = 0
	before := s.Turn
	if s.PlacePiece(Tzaar, AllPoints()[1]) {
		t.Fatal("placing a type with 0 remaining must fail")
	}
	if s.Turn != before {
		t.Fatal("a rejected placement must not advance the turn")
	}
}

// TestQuickRandomSetupReachesPlayingWithFullArmies drives the "quick start"
// path end to end and checks it lands in exactly the same final state a
// fully manual setup would: PhasePlaying, 30 stacks each, full type counts.
func TestQuickRandomSetupReachesPlayingWithFullArmies(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	s := NewGame(OpponentHotseat, DepthEasy)
	s.QuickRandomSetup(rng)
	if s.Phase != PhasePlaying {
		t.Fatalf("Phase = %v, want PhasePlaying after QuickRandomSetup", s.Phase)
	}
	if s.Board.StackCount(Black) != TotalPerSide || s.Board.StackCount(White) != TotalPerSide {
		t.Fatalf("each side should have %d stacks after quick setup, got B=%d W=%d", TotalPerSide,
			s.Board.StackCount(Black), s.Board.StackCount(White))
	}
	for _, side := range [2]Side{Black, White} {
		for _, typ := range AllTypes {
			if got := s.Board.TypeCount(side, typ); got != StartCount(typ) {
				t.Errorf("board TypeCount(%v,%v) = %d, want %d", side, typ, got, StartCount(typ))
			}
		}
	}
}

// TestPlayRejectsIllegalAndAppliesLegal.
func TestPlayRejectsIllegalAndAppliesLegal(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy)
	rng := rand.New(rand.NewSource(2))
	s.QuickRandomSetup(rng)
	if s.Phase != PhasePlaying {
		t.Fatal("setup should have completed")
	}
	moves := LegalMoves(s.Board, s.Turn)
	if len(moves) == 0 {
		t.Fatal("side to move should have at least one legal move on a freshly randomized board")
	}
	m := moves[0]
	wantSide := s.Turn
	if !s.Play(m.From, m.To) {
		t.Fatalf("legal move %v should be applied", m)
	}
	if s.Turn == wantSide && s.Phase != PhaseDone {
		t.Fatal("turn should have passed to the opponent after a legal, non-terminal move")
	}
	// Playing the exact same (now stale) from/to again must be rejected (the
	// origin is empty or it isn't that side's turn's stack anymore).
	if s.Phase != PhaseDone && s.Play(m.From, m.To) {
		t.Fatal("replaying a stale move must be rejected")
	}
}

// TestPlayRejectedDuringSetupPhase: Play must be a no-op before setup ends.
func TestPlayRejectedDuringSetupPhase(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy)
	if s.Play(AllPoints()[0], AllPoints()[1]) {
		t.Fatal("Play must be rejected during PhaseSetup")
	}
}

// TestPlacePieceRejectedDuringPlayPhase: PlacePiece must be a no-op once
// play has started.
func TestPlacePieceRejectedDuringPlayPhase(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy)
	rng := rand.New(rand.NewSource(3))
	s.QuickRandomSetup(rng)
	before := s.Board.StackCount(Black) + s.Board.StackCount(White)
	if s.PlacePiece(Tzaar, Point{100, -100, 0}) {
		t.Fatal("PlacePiece must be rejected once PhasePlaying has begun")
	}
	after := s.Board.StackCount(Black) + s.Board.StackCount(White)
	if before != after {
		t.Fatal("a rejected placement must not change the board")
	}
}

// TestWinByTypeEliminationEndsGame constructs a play-phase position where
// Black's move captures White's only remaining Tzaar-holding stack, and
// checks GameState correctly transitions to PhaseDone with Black as winner.
func TestWinByTypeEliminationEndsGame(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy)
	s.Phase = PhasePlaying
	s.Turn = Black
	s.Board = NewBoard()

	from := Point{0, 0, 0}
	to := from.Add(Directions[0])
	s.Board.Stacks[from] = Stack{Owner: Black, Type: Tzarra, Height: 1, Comp: [3]int{Tzarra: 1}}
	s.Board.Stacks[to] = Stack{Owner: White, Type: Tzaar, Height: 1, Comp: [3]int{Tzaar: 1}} // White's ONLY Tzaar
	// Give White some other pieces too, so this is genuinely a type
	// elimination, not an incidental "White has zero pieces total".
	s.Board.Stacks[Point{2, -2, 0}] = Stack{Owner: White, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}
	s.Board.Stacks[Point{2, -1, -1}] = Stack{Owner: White, Type: Tzarra, Height: 1, Comp: [3]int{Tzarra: 1}}
	// Black also needs enough pieces of every type to not accidentally
	// trigger its OWN elimination first.
	s.Board.Stacks[Point{-2, 2, 0}] = Stack{Owner: Black, Type: Tzaar, Height: 1, Comp: [3]int{Tzaar: 1}}
	s.Board.Stacks[Point{-2, 1, 1}] = Stack{Owner: Black, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}

	if !s.Play(from, to) {
		t.Fatal("the type-eliminating capture should be a legal move")
	}
	if s.Phase != PhaseDone {
		t.Fatalf("Phase = %v, want PhaseDone", s.Phase)
	}
	if s.Winner() != Black {
		t.Fatalf("Winner() = %v, want Black", s.Winner())
	}
}

// TestWinnerEmptyUntilPhaseDone.
func TestWinnerEmptyUntilPhaseDone(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy)
	if s.Winner() != None {
		t.Fatal("Winner() must be None before the game ends")
	}
}

// TestNoLegalMoveForfeits: if the side to move has no legal move at all,
// they lose immediately (mirrors hasami/ringar's identical stalemate rule).
func TestNoLegalMoveForfeits(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy)
	s.Phase = PhasePlaying
	s.Turn = Black
	s.Board = NewBoard()
	// Black has one lone stack completely boxed in by White stacks it can't
	// capture (all taller).
	center := Point{0, 0, 0}
	s.Board.Stacks[center] = Stack{Owner: Black, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}
	for _, n := range Neighbors(center) {
		s.Board.Stacks[n] = Stack{Owner: White, Type: Tzaar, Height: 5, Comp: [3]int{Tzaar: 5}}
	}
	// White needs a real move available elsewhere so the game doesn't end
	// on White's own forfeit first; give White a free stack with an open
	// neighbour far from the boxed-in cluster.
	far := Point{4, -4, 0}
	s.Board.Stacks[far] = Stack{Owner: White, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}
	// Ensure Black also has a piece of every type so this doesn't
	// accidentally look like an elimination instead of a stalemate. Height 9
	// guarantees zero legal moves on its own too (the board's diameter along
	// any axis is at most 8 steps, so a stack that must move exactly 9 steps
	// always runs off the board in every direction), so these don't need a
	// surrounding ring of blockers the way the center piece does.
	s.Board.Stacks[Point{-3, 3, 0}] = Stack{Owner: Black, Type: Tzaar, Height: 9, Comp: [3]int{Tzaar: 9}}
	s.Board.Stacks[Point{-3, 2, 1}] = Stack{Owner: Black, Type: Tzarra, Height: 9, Comp: [3]int{Tzarra: 9}}
	// White also needs one of every type for the same reason — the 6 ring
	// stacks are all Tzaar and `far` is Tott, so without this White reads as
	// "zero Tzarra" and EliminatedSide (checked before the no-legal-move
	// case) would wrongly declare White eliminated instead of Black
	// stalemated.
	s.Board.Stacks[Point{4, 0, -4}] = Stack{Owner: White, Type: Tzarra, Height: 1, Comp: [3]int{Tzarra: 1}}

	if len(LegalMoves(s.Board, Black)) != 0 {
		t.Fatal("test setup broken: Black should have zero legal moves")
	}
	// Play a harmless White move first isn't applicable since it's Black's
	// turn with no legal move — GameState only detects this via advance(),
	// which runs after a move. Simulate by directly checking what StepAI /
	// the UI would see: LegalMoves(Black) empty means the game must already
	// be over as soon as it becomes Black's turn. Exercise this exact path
	// via a White move that hands the turn to Black.
	s.Turn = White
	if !s.Play(far, far.Add(Directions[1])) { // Directions[1] points back toward center, staying on-board
		t.Fatal("White's harmless move should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatalf("Phase = %v, want PhaseDone (Black has no legal move)", s.Phase)
	}
	if s.Winner() != White {
		t.Fatalf("Winner() = %v, want White (Black forfeits with no legal move)", s.Winner())
	}
}

func TestAITurnOnlyForOpponentAI(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy)
	s.Turn = White
	if s.AITurn() {
		t.Fatal("hotseat mode should never report an AI turn")
	}
	s2 := NewGame(OpponentAI, DepthEasy)
	s2.Turn = Black
	if s2.AITurn() {
		t.Fatal("AITurn should be false when it's the human's (Black's) turn")
	}
	s2.Turn = White
	if !s2.AITurn() {
		t.Fatal("AITurn should be true when it's White's (the AI's) turn")
	}
}

// TestStepAIPlacesDuringSetup.
func TestStepAIPlacesDuringSetup(t *testing.T) {
	s := NewGame(OpponentAI, DepthEasy)
	s.Turn = Black // human first; AI (White) should not act yet
	if s.StepAI() {
		t.Fatal("StepAI must be a no-op when it isn't the AI's turn")
	}
	s.Turn = White
	before := s.PlacedCount()
	if !s.StepAI() {
		t.Fatal("StepAI should place a piece for the AI during setup")
	}
	if s.PlacedCount() != before+1 {
		t.Fatal("StepAI should have placed exactly one piece")
	}
	if s.Turn != Black {
		t.Fatal("turn should have passed back to the human after the AI's placement")
	}
}

// TestStepAIMovesDuringPlay.
func TestStepAIMovesDuringPlay(t *testing.T) {
	s := NewGame(OpponentAI, DepthEasy)
	rng := rand.New(rand.NewSource(4))
	s.QuickRandomSetup(rng)
	if s.Phase != PhasePlaying {
		t.Fatal("setup should have completed")
	}
	s.Turn = White
	before := s.Board.Clone()
	if !s.StepAI() {
		t.Fatal("StepAI should make a move for White")
	}
	same := true
	for p, st := range before.Stacks {
		if s.Board.Stacks[p] != st {
			same = false
		}
	}
	if same && len(before.Stacks) == len(s.Board.Stacks) {
		t.Fatal("StepAI should have changed the board")
	}
}
