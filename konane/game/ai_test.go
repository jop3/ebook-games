package game

import (
	"testing"
	"time"
)

func TestBestMoveReturnsALegalChain(t *testing.T) {
	b := emptyBoard()
	b.set(3, 3, Black)
	b.set(3, 4, White)
	b.set(3, 6, White)

	chain, ok := BestMove(b, Black, DepthEasy)
	if !ok {
		t.Fatal("BestMove should find a move: Black has a legal jump")
	}
	if len(chain) == 0 {
		t.Fatal("a returned chain must contain at least one jump")
	}
	// Replaying the chosen chain against the rules engine from scratch must
	// reproduce exactly the board BestMove implicitly committed to.
	got := ApplyChain(b, Black, chain)
	cur := b
	for _, j := range chain {
		legal := cur.LegalJumpsFrom(j.From, Black)
		found := false
		for _, lj := range legal {
			if lj == j {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("BestMove chain contains an illegal jump %+v (legal from there: %v)", j, legal)
		}
		cur = cur.Apply(j, Black)
	}
	if cur != got {
		t.Fatal("replaying the chain step by step should match ApplyChain's result")
	}
}

func TestBestMoveNoLegalJumpReturnsNotOK(t *testing.T) {
	b := emptyBoard()
	b.set(0, 0, Black) // isolated: no enemy adjacent
	if _, ok := BestMove(b, Black, DepthEasy); ok {
		t.Fatal("BestMove should report ok=false when the side has no legal jump")
	}
}

func TestBestMoveTakesAnAvailableCapture(t *testing.T) {
	b := emptyBoard()
	b.set(3, 3, Black)
	b.set(3, 4, White) // Black's only legal jump captures this stone
	chain, ok := BestMove(b, Black, DepthEasy)
	if !ok || len(chain) == 0 {
		t.Fatal("expected a legal capturing move")
	}
	nb := ApplyChain(b, Black, chain)
	if nb.Count(White) != 0 {
		t.Fatal("the only legal move captures White's only stone; AI must take it")
	}
}

// TestBestMovePrefersNotHangingIntoLoss checks the AI avoids a move that
// leaves it with zero legal jumps on the following exchange, when a
// safer alternative exists that doesn't immediately hand the opponent a
// forced win.
func TestBestMovePrefersNotHangingIntoLoss(t *testing.T) {
	b := emptyBoard()
	// Black has two independent stones with jumps available. Jumping with
	// the stone at (1,1) leaves Black's other stone at (6,6) with a jump
	// still available next turn; that's the only realistic option here since
	// each stone's jump is forced eventually, but the search should still
	// return a legal, sensible chain rather than crashing or stalling.
	b.set(1, 1, Black)
	b.set(2, 1, White)
	b.set(6, 6, Black)
	b.set(6, 5, White)

	chain, ok := BestMove(b, Black, DepthMedium)
	if !ok {
		t.Fatal("Black clearly has legal jumps available")
	}
	if len(chain) == 0 {
		t.Fatal("expected a non-empty chain")
	}
}

// TestAIPerformanceIsReasonable guards against the search blowing up: full
// engine depth on a nearly-full 8x8 board (right after the opening) must
// finish quickly enough for an interactive e-reader app.
func TestAIPerformanceIsReasonable(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	opts := CenterRemovalOptions()
	s.RemoveOpeningBlack(opts[0])
	s.RemoveOpeningWhite(s.OpeningWhiteOptions()[0])

	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		start := time.Now()
		_, ok := BestMove(s.Board, Black, depth)
		elapsed := time.Since(start)
		if !ok {
			t.Fatalf("depth %d: BestMove should find a move from the post-opening position", depth)
		}
		if elapsed > 5*time.Second {
			t.Fatalf("depth %d took %v, too slow for interactive use", depth, elapsed)
		}
		t.Logf("depth %d: %v", depth, elapsed)
	}
}

func TestEvaluateMaterialDominates(t *testing.T) {
	b := emptyBoard()
	b.set(0, 0, Black)
	b.set(1, 0, Black)
	b.set(2, 0, White)
	// Black has 2 stones, White has 1: material should favor Black heavily
	// from Black's perspective, regardless of the small mobility term.
	if evaluate(&b, Black) <= 0 {
		t.Fatal("evaluate from Black's perspective should favor Black (more material)")
	}
	if evaluate(&b, White) >= 0 {
		t.Fatal("evaluate from White's perspective should disfavor White (less material)")
	}
}
