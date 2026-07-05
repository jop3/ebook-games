package game

import (
	"image"
	"testing"
)

func TestAreaScoreEmptyBoardKomiOnly(t *testing.T) {
	b := NewBoard(9)
	bl, wh := AreaScore(b, nil, DefaultKomi)
	if bl != 0 {
		t.Fatalf("empty board: Black score should be 0, got %v", bl)
	}
	if wh != DefaultKomi {
		t.Fatalf("empty board: White score should be exactly komi (%v), got %v", DefaultKomi, wh)
	}
}

func TestAreaScoreStonesPlusTerritory(t *testing.T) {
	// Black owns the left 3 columns via a wall in column 3; White owns the
	// right 3 columns via a wall in column 5 (bounding the board so Black's
	// territory doesn't leak into unclaimed space with no rival color at
	// all, which — correctly — would also count as Black's, per Go rules).
	b := NewBoard(9)
	for y := 0; y < 9; y++ {
		b.Set(image.Pt(3, y), Black)
		b.Set(image.Pt(5, y), White)
	}
	bl, _ := AreaScore(b, nil, 0)
	// Black stones: 9. Territory: columns 0,1,2 (27 empty points), all
	// bordering only Black (column 3) plus the board edge. Column 4 borders
	// both colors (dame) and column 5+ is White's, so neither counts here.
	if bl != 9+27 {
		t.Fatalf("Black area = %v, want %v (9 stones + 27 territory)", bl, 9+27)
	}
}

// --- GOTCHA: a neutral/dame point bordering both colors scores for neither ---
func TestAreaScoreDamePointScoresForNeither(t *testing.T) {
	b := NewBoard(9)
	b.Set(image.Pt(1, 0), Black)
	b.Set(image.Pt(0, 1), White)
	// (0,0) is empty and borders both Black and White: a dame point.
	bl, wh := AreaScore(b, nil, 0)
	if bl != 1 { // just the Black stone itself, no territory
		t.Fatalf("Black score should be just its 1 stone (dame doesn't count), got %v", bl)
	}
	if wh != 1 { // just the White stone itself
		t.Fatalf("White score should be just its 1 stone (dame doesn't count), got %v", wh)
	}
}

// --- GOTCHA: a larger region touching both colors is also entirely dame -----
func TestAreaScoreMixedBorderRegionIsAllDame(t *testing.T) {
	b := NewBoard(9)
	// A vertical wall of alternating color at x=4 leaves one connected empty
	// column at x=3 (bordered on the left by nothing set, i.e. it flows into
	// the rest of the empty board) — build instead a fully enclosed 1x3
	// corridor bordered by both colors on either long side to force a mixed
	// region that must NOT be scored for anyone.
	b.Set(image.Pt(2, 0), Black)
	b.Set(image.Pt(2, 1), Black)
	b.Set(image.Pt(2, 2), Black)
	b.Set(image.Pt(4, 0), White)
	b.Set(image.Pt(4, 1), White)
	b.Set(image.Pt(4, 2), White)
	// Column x=3, y=0..2 is empty, bordered by Black on the left and White on
	// the right (and open at y=3 into the rest of the board, which is fine —
	// it just makes the region bigger, still touching both colors).
	bl, wh := AreaScore(b, nil, 0)
	if bl != 3 || wh != 3 {
		t.Fatalf("both sides should score only their 3 stones (the shared open region is dame), got black=%v white=%v", bl, wh)
	}
}

func TestAreaScoreDeadStonesCountForCapturer(t *testing.T) {
	b := NewBoard(9)
	for y := 0; y < 9; y++ {
		b.Set(image.Pt(3, y), Black)
		b.Set(image.Pt(5, y), White) // bounds the board (see TestAreaScoreStonesPlusTerritory)
	}
	// A doomed lone White stone sitting deep in Black's area — surrounded,
	// clearly dead, but never actually captured over the board. While it sits
	// there un-marked, it makes the whole left region border both colors
	// (mixed => dame => scoreless), exactly like a real unresolved dead stone
	// would in a naive scorer.
	b.Set(image.Pt(1, 4), White)
	dead := map[image.Point]bool{image.Pt(1, 4): true}

	blAlive, _ := AreaScore(b, nil, 0)
	blDead, _ := AreaScore(b, dead, 0)
	// Marking the stone dead must count that point (and the territory it now
	// merges into) for Black instead of it costing Black that territory.
	if blDead <= blAlive {
		t.Fatalf("marking the White stone dead should increase Black's score (alive=%v dead=%v)", blAlive, blDead)
	}
	if blDead != 9+27 {
		t.Fatalf("Black area with the dead stone removed = %v, want %v", blDead, 9+27)
	}
}
