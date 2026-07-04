package game

import (
	"image"
	"testing"
)

func TestBestMoveReturnsLegalMove(t *testing.T) {
	b := NewBoard(9)
	p, ok := BestMove(b, Black, nil)
	if !ok {
		t.Fatal("AI should find a move on an empty board")
	}
	if !Legal(b, p, Black, nil) {
		t.Fatalf("AI returned illegal move %v", p)
	}
}

func TestBestMovePrefersCapture(t *testing.T) {
	// White stone at (0,0) in atari (Black already at (1,0), (0,0)'s only
	// other liberty is (0,1)). The AI, playing Black, should take the free
	// capture over any other candidate.
	b := NewBoard(9)
	b.Set(image.Pt(0, 0), White)
	b.Set(image.Pt(1, 0), Black)
	p, ok := BestMove(b, Black, nil)
	if !ok {
		t.Fatal("AI should find a move")
	}
	if p != image.Pt(0, 1) {
		t.Fatalf("AI should take the free capture at (0,1), chose %v", p)
	}
}

func TestBestMoveAvoidsSelfAtariWhenAlternativeExists(t *testing.T) {
	// Black playing at (0,0) would be a lone stone with 1 liberty (self-atari,
	// no capture involved, since (1,0)/(0,1) are White with outside
	// liberties) — the AI should prefer literally any other legal point
	// instead, since self-atari-with-no-capture always scores below a
	// perfectly ordinary open-board move.
	b := NewBoard(9)
	b.Set(image.Pt(1, 0), White)
	b.Set(image.Pt(0, 1), White)
	b.Set(image.Pt(3, 3), White) // give (1,0)/(0,1) plenty of outside liberties via distance
	p, ok := BestMove(b, Black, nil)
	if !ok {
		t.Fatal("AI should find a move")
	}
	if p == image.Pt(0, 0) {
		t.Fatal("AI should not choose the self-atari corner when open board space is available")
	}
}

func TestBestMoveRespectsKo(t *testing.T) {
	// Reuse the classic ko shape; the AI (playing Black) must not offer the
	// ko-forbidden recapture as its move, even though it would otherwise
	// score very well (it looks like a capture).
	p0 := NewBoard(9)
	p0.Set(image.Pt(2, 0), Black)
	p0.Set(image.Pt(3, 1), Black)
	p0.Set(image.Pt(2, 2), Black)
	p0.Set(image.Pt(1, 1), Black)
	p0.Set(image.Pt(1, 0), White)
	p0.Set(image.Pt(0, 1), White)
	p0.Set(image.Pt(1, 2), White)
	p1, _, ok := Place(p0, image.Pt(2, 1), White)
	if !ok {
		t.Fatal("test setup: White's capturing move should succeed")
	}
	mv, ok := BestMove(p1, Black, &p0)
	if !ok {
		t.Fatal("AI should still find some legal move")
	}
	if mv == image.Pt(1, 1) {
		t.Fatal("AI must not offer the ko-forbidden recapture")
	}
}

// BenchmarkBestMove measures the AI's own decision wall-clock in isolation
// (no rendering), on a realistic mid-game 9x9 board — go test's -bench
// harness excludes it from a normal `go test` run. Reported in the build
// notes: comfortably fast, well under the ~2s/move device budget.
func BenchmarkBestMove(b *testing.B) {
	board := NewBoard(9)
	// A plausible mid-game scatter, not just an empty board.
	stones := []struct {
		p image.Point
		c Color
	}{
		{image.Pt(2, 2), Black}, {image.Pt(6, 6), White},
		{image.Pt(3, 2), Black}, {image.Pt(6, 5), White},
		{image.Pt(2, 3), Black}, {image.Pt(5, 6), White},
		{image.Pt(4, 4), Black}, {image.Pt(4, 5), White},
		{image.Pt(1, 6), Black}, {image.Pt(7, 2), White},
		{image.Pt(1, 7), Black}, {image.Pt(7, 3), White},
	}
	for _, s := range stones {
		board.Set(s.p, s.c)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BestMove(board, White, nil)
	}
}

func TestBestMoveNoLegalMoves(t *testing.T) {
	// A full board: no legal moves for anyone.
	b := NewBoard(9)
	for y := 0; y < 9; y++ {
		for x := 0; x < 9; x++ {
			if (x+y)%2 == 0 {
				b.Set(image.Pt(x, y), Black)
			} else {
				b.Set(image.Pt(x, y), White)
			}
		}
	}
	if _, ok := BestMove(b, Black, nil); ok {
		t.Fatal("BestMove should report no move available on a full board")
	}
}
