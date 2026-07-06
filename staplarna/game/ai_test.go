package game

import (
	"math/rand"
	"testing"
	"time"
)

// TestBestMoveReturnsLegalMove checks BestMove's chosen move is always
// actually legal, at every difficulty depth, from a randomized mid-setup
// position.
func TestBestMoveReturnsLegalMove(t *testing.T) {
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		s := NewGame(OpponentHotseat, depth)
		rng := rand.New(rand.NewSource(int64(depth)))
		s.QuickRandomSetup(rng)
		m, ok := BestMove(s.Board, s.Turn, depth)
		if !ok {
			t.Fatalf("depth %d: BestMove reported no legal move on a freshly randomized board", depth)
		}
		if !IsLegalMove(s.Board, s.Turn, m) {
			t.Fatalf("depth %d: BestMove returned an illegal move %v", depth, m)
		}
	}
}

// TestBestMoveTakesTheImmediateWin: if a move eliminates the opponent's last
// piece of a type right now, BestMove must take it regardless of search
// depth (the fast path).
func TestBestMoveTakesTheImmediateWin(t *testing.T) {
	b := NewBoard()
	from := Point{0, 0, 0}
	to := from.Add(Directions[0])
	b.Stacks[from] = Stack{Owner: Black, Type: Tzarra, Height: 1, Comp: [3]int{Tzarra: 1}}
	b.Stacks[to] = Stack{Owner: White, Type: Tzaar, Height: 1, Comp: [3]int{Tzaar: 1}} // White's only Tzaar
	// A decoy elsewhere so the winning move isn't the only legal one.
	decoyFrom := Point{-3, 3, 0}
	b.Stacks[decoyFrom] = Stack{Owner: Black, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}
	b.Stacks[Point{2, -2, 0}] = Stack{Owner: White, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}

	m, ok := BestMove(b, Black, DepthEasy)
	if !ok {
		t.Fatal("BestMove should find a move")
	}
	if m.From != from || m.To != to {
		t.Fatalf("BestMove = %v, want the immediate type-eliminating capture %v->%v", m, from, to)
	}
}

// TestBestMoveNeverCapturesOwnStack (sanity: material scoring should never
// lead the search to prefer an illegal move; combined with
// TestBestMoveReturnsLegalMove this is mostly a documentation test).
func TestBestMoveNeverLosesOnPurposeWhenAWinIsAvailable(t *testing.T) {
	b := NewBoard()
	// Black can either win immediately, or make a pointless non-winning move.
	winFrom := Point{0, 0, 0}
	winTo := winFrom.Add(Directions[0])
	b.Stacks[winFrom] = Stack{Owner: Black, Type: Tzaar, Height: 1, Comp: [3]int{Tzaar: 1}}
	b.Stacks[winTo] = Stack{Owner: White, Type: Tzarra, Height: 1, Comp: [3]int{Tzarra: 1}} // White's only Tzarra
	other := Point{3, -3, 0}
	b.Stacks[other] = Stack{Owner: Black, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}
	b.Stacks[Point{-3, 3, 0}] = Stack{Owner: White, Type: Tzaar, Height: 1, Comp: [3]int{Tzaar: 1}}
	b.Stacks[Point{-2, 2, 0}] = Stack{Owner: White, Type: Tott, Height: 3, Comp: [3]int{Tott: 3}}

	m, ok := BestMove(b, Black, DepthMedium)
	if !ok || m.From != winFrom || m.To != winTo {
		t.Fatalf("BestMove = %v (ok=%v), want the winning capture %v->%v", m, ok, winFrom, winTo)
	}
}

// TestAIPlacementRespectsRemainingAndBoard checks AIPlacement only ever
// chooses a type with remaining > 0, and an empty board cell.
func TestAIPlacementRespectsRemainingAndBoard(t *testing.T) {
	b := NewBoard()
	remaining := map[PieceType]int{Tzaar: 0, Tzarra: 0, Tott: 3}
	for i := 0; i < 3; i++ {
		typ, p := AIPlacement(b, remaining)
		if typ != Tott {
			t.Fatalf("iteration %d: AIPlacement chose %v, want Tott (the only type with remaining>0)", i, typ)
		}
		if _, occ := b.At(p); occ {
			t.Fatalf("iteration %d: AIPlacement chose an already-occupied cell %v", i, p)
		}
		if !Valid(p) {
			t.Fatalf("iteration %d: AIPlacement chose an invalid cell %v", i, p)
		}
		b.PlaceNew(White, typ, p)
	}
}

// TestAIDifficultiesRunWithinBudget is a soft timing check (not a hard
// correctness test) to catch a depth setting that would feel like a stall on
// real (much slower) ARM hardware — mirroring the timing diligence in
// ringar/game/state.go's AIDepth comment. All 3 shipped difficulties must
// return within a generous few seconds on this dev machine, from the widest
// -- and thus slowest -- position: a freshly, fully-placed board (maximum
// piece count, maximum branching).
func TestAIDifficultiesRunWithinBudget(t *testing.T) {
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		s := NewGame(OpponentHotseat, depth)
		rng := rand.New(rand.NewSource(42))
		s.QuickRandomSetup(rng)
		start := time.Now()
		_, ok := BestMove(s.Board, s.Turn, depth)
		elapsed := time.Since(start)
		if !ok {
			t.Fatalf("depth %d: BestMove found no move", depth)
		}
		t.Logf("depth %d: BestMove took %v", depth, elapsed)
		if elapsed > 8*time.Second {
			t.Errorf("depth %d: BestMove took %v, too slow for a casual e-ink opponent", depth, elapsed)
		}
	}
}

// TestEvaluateFavorsMoreScarcePieces: losing a Tzaar should score worse than
// losing an equal number of Totts, since typeWeight/dangerPenalty weight
// scarce types higher.
func TestEvaluateFavorsMoreScarcePieces(t *testing.T) {
	base := NewBoard()
	for _, p := range AllPoints()[:6] {
		base.Stacks[p] = Stack{Owner: Black, Type: Tzaar, Height: 1, Comp: [3]int{Tzaar: 1}}
	}
	for _, p := range AllPoints()[6:15] {
		base.Stacks[p] = Stack{Owner: Black, Type: Tzarra, Height: 1, Comp: [3]int{Tzarra: 1}}
	}
	for _, p := range AllPoints()[15:20] {
		base.Stacks[p] = Stack{Owner: White, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}
	}

	lostTzaar := base.Clone()
	delete(lostTzaar.Stacks, AllPoints()[0]) // Black loses one Tzaar (now 5 of 6)

	lostTott := base.Clone()
	// Black has no Tott in `base`; instead remove one Tzarra as the
	// "cheaper" comparison loss (still a real loss, just of a less scarce type).
	delete(lostTott.Stacks, AllPoints()[6]) // Black loses one Tzarra (now 8 of 9)

	scoreLostTzaar := evaluate(lostTzaar, Black)
	scoreLostTott := evaluate(lostTott, Black)
	if scoreLostTzaar >= scoreLostTott {
		t.Fatalf("losing a Tzaar should score worse for Black than losing a Tzarra: lostTzaar=%d lostTott=%d",
			scoreLostTzaar, scoreLostTott)
	}
}

// TestEvaluateDangerPenaltyAtOne: a side down to exactly 1 of a scarce type
// should score noticeably worse than having 2 of that same type, all else
// equal.
func TestEvaluateDangerPenaltyAtOne(t *testing.T) {
	twoTzaar := NewBoard()
	twoTzaar.Stacks[Point{0, 0, 0}] = Stack{Owner: Black, Type: Tzaar, Height: 1, Comp: [3]int{Tzaar: 1}}
	twoTzaar.Stacks[Point{1, -1, 0}] = Stack{Owner: Black, Type: Tzaar, Height: 1, Comp: [3]int{Tzaar: 1}}
	twoTzaar.Stacks[Point{-2, 2, 0}] = Stack{Owner: White, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}

	oneTzaar := NewBoard()
	oneTzaar.Stacks[Point{0, 0, 0}] = Stack{Owner: Black, Type: Tzaar, Height: 1, Comp: [3]int{Tzaar: 1}}
	oneTzaar.Stacks[Point{-2, 2, 0}] = Stack{Owner: White, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}

	scoreTwo := material(twoTzaar, Black)
	scoreOne := material(oneTzaar, Black)
	// Losing a whole Tzaar normally costs typeWeight(Tzaar)=60; the danger
	// penalty at exactly 1 remaining should make the actual gap bigger than
	// that.
	if scoreTwo-scoreOne <= typeWeight(Tzaar) {
		t.Fatalf("material gap from 2->1 Tzaar = %d, want more than the plain per-piece weight %d (danger penalty not applied)",
			scoreTwo-scoreOne, typeWeight(Tzaar))
	}
}
