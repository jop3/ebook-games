package game

import (
	"image"
	"testing"
	"time"
)

func TestBestMoveReturnsOkWhenMovesExist(t *testing.T) {
	b := NewBoard()
	m, ok := BestMove(b, Black, DepthEasy)
	if !ok {
		t.Fatal("BestMove should find a move from the starting position")
	}
	if !b.IsLegalMove(Black, m) {
		t.Fatalf("BestMove returned an illegal move %v", m)
	}
}

func TestBestMoveNoMoveWhenNoneExist(t *testing.T) {
	var b Board
	b.set(0, 0, White) // no Black men anywhere
	_, ok := BestMove(b, Black, DepthEasy)
	if ok {
		t.Fatal("BestMove should report ok=false when the side has no legal move")
	}
}

// TestBestMovePrefersTheFlippingMove constructs a position where Black has
// two choices: a move that flips an adjacent White man, and a "quiet" move
// that flips nothing. The AI (at any positive depth) should prefer the
// immediate material gain.
func TestBestMovePrefersTheFlippingMove(t *testing.T) {
	var b Board
	b.set(3, 3, Black)
	b.set(4, 3, White)  // Black cloning to (4,4) flips this
	b.set(0, 0, Black)  // a second Black man with only quiet moves available
	m, ok := BestMove(b, Black, DepthEasy)
	if !ok {
		t.Fatal("BestMove should find a move")
	}
	nb, flipped := b.Apply(m)
	if len(flipped) == 0 {
		t.Fatalf("BestMove chose %v which flips nothing; wanted the flipping move", m)
	}
	if nb.Count(White) != 0 {
		t.Fatalf("expected the flip to eliminate White's only man, got %d remaining", nb.Count(White))
	}
}

// TestBestMoveAllDepthsTerminatePromptly is a coarse performance guard: the
// AI must answer within a generous time budget at every offered difficulty,
// from a realistic mid-game position (moderate branching factor), so a
// future change to search depth/move generation doesn't silently make the
// device (which has no time to spare) painfully slow.
func TestBestMoveAllDepthsTerminatePromptly(t *testing.T) {
	b := midGameBoard()
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		depth := depth
		start := time.Now()
		m, ok := BestMove(b, Black, depth)
		elapsed := time.Since(start)
		if !ok {
			t.Fatalf("depth %d: BestMove found no move", depth)
		}
		if !b.IsLegalMove(Black, m) {
			t.Fatalf("depth %d: BestMove returned an illegal move %v", depth, m)
		}
		if elapsed > 5*time.Second {
			t.Fatalf("depth %d: BestMove took %v, want < 5s", depth, elapsed)
		}
	}
}

// midGameBoard builds a plausible mid-game position (several men per side,
// some empty cells) used to benchmark/exercise the AI beyond the sparse
// starting position.
func midGameBoard() Board {
	b := NewBoard()
	placements := []struct {
		p image.Point
		c Cell
	}{
		{image.Pt(1, 0), Black}, {image.Pt(2, 1), Black}, {image.Pt(3, 3), Black},
		{image.Pt(1, 2), Black}, {image.Pt(4, 5), Black},
		{image.Pt(5, 1), White}, {image.Pt(4, 2), White}, {image.Pt(2, 4), White},
		{image.Pt(5, 5), White}, {image.Pt(3, 5), White},
	}
	for _, pl := range placements {
		b.set(pl.p.X, pl.p.Y, pl.c)
	}
	return b
}

func TestNegamaxRespectsAlphaBetaOrdering(t *testing.T) {
	// A light structural check: increasing depth should not make Black's
	// evaluated position at the root worse than a shallower search when
	// Black has a clearly available flip -- i.e. the search doesn't regress
	// into picking a strictly worse move as depth grows on a simple board.
	var b Board
	b.set(3, 3, Black)
	b.set(4, 3, White)
	b.set(0, 6, Black)
	m1, ok1 := BestMove(b, Black, 1)
	m2, ok2 := BestMove(b, Black, 3)
	if !ok1 || !ok2 {
		t.Fatal("BestMove should find a move at both depths")
	}
	_, f1 := b.Apply(m1)
	_, f2 := b.Apply(m2)
	if len(f1) == 0 && len(f2) == 0 {
		t.Fatal("expected at least one depth to choose the available flip")
	}
}
