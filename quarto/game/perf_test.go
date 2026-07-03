package game

import (
	"testing"
	"time"
)

func TestOpeningGiveTiming(t *testing.T) {
	for _, lvl := range []int{2, 4, 6} {
		s := NewGame(ModeAI, lvl)
		s.Step = StepGive
		s.Turn = 1
		start := time.Now()
		BestGive(s, lvl)
		el := time.Since(start)
		t.Logf("opening BestGive(level=%d) took %v", lvl, el)
		if el > 5*time.Second {
			t.Fatalf("too slow at level %d: %v", lvl, el)
		}
	}
}

// TestMidgameTiming exercises the AI when the pool has shrunk to <=10 and
// <=6 pieces, where searchDepth intentionally adds extra plies (endgame gets
// deeper, cheap search since fewer branches remain).
func TestMidgameTiming(t *testing.T) {
	s := NewGame(ModeAI, 6)
	// Place 10 pieces on the board (arbitrary distinct pieces/cells, no win),
	// shrinking the pool to 6 remaining.
	placements := []struct {
		x, y int
		p    Piece
	}{
		{0, 0, 9}, {1, 0, 6}, {2, 0, 1}, {3, 0, 12},
		{0, 1, 7}, {1, 1, 4}, {2, 1, 2}, {3, 1, 13},
		{0, 2, 11}, {1, 2, 10},
	}
	for _, pl := range placements {
		s.Board.Place(pl.x, pl.y, pl.p)
		s.removeFromPool(pl.p)
	}
	if s.Board.HasWin() {
		t.Fatal("unexpected win in fixture; adjust placements")
	}
	s.Step = StepGive
	s.Turn = 1
	start := time.Now()
	p := BestGive(s, 6)
	el := time.Since(start)
	t.Logf("midgame (pool=%d) BestGive(level=6) took %v, chose %v", len(s.Pool), el, p)
	if el > 8*time.Second {
		t.Fatalf("too slow: %v", el)
	}
}

// TestEndgameTiming checks the deepest, most expensive regime: a small pool
// where searchDepth intentionally adds +4 plies.
func TestEndgameTiming(t *testing.T) {
	s := NewGame(ModeAI, 6)
	placements := []struct {
		x, y int
		p    Piece
	}{
		{0, 0, 9}, {1, 0, 2}, {2, 0, 11}, {3, 0, 6},
		{0, 1, 4}, {1, 1, 12}, {2, 1, 0}, {3, 1, 15},
		{0, 2, 1}, {1, 2, 10}, {2, 2, 3}, {3, 2, 14},
	}
	for _, pl := range placements {
		s.Board.Place(pl.x, pl.y, pl.p)
		s.removeFromPool(pl.p)
	}
	if s.Board.HasWin() {
		t.Fatal("unexpected win in fixture; adjust placements")
	}
	s.Step = StepGive
	s.Turn = 1
	start := time.Now()
	p := BestGive(s, 6)
	el := time.Since(start)
	t.Logf("endgame (pool=%d) BestGive(level=6) took %v, chose %v", len(s.Pool), el, p)
	if el > 8*time.Second {
		t.Fatalf("too slow: %v", el)
	}
}
