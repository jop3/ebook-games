package game

import "testing"

func TestScorePlacementIsolatedTile(t *testing.T) {
	var wall [WallSize][WallSize]bool
	wall[2][2] = true
	if got := scorePlacement(&wall, 2, 2); got != 1 {
		t.Fatalf("isolated tile scored %d, want 1", got)
	}
}

// GOTCHA: a tile with BOTH horizontal and vertical neighbors scores both
// runs — the classic "cross" shape.
func TestScorePlacementCross(t *testing.T) {
	var wall [WallSize][WallSize]bool
	wall[1][2] = true // up
	wall[3][2] = true // down
	wall[2][1] = true // left
	wall[2][3] = true // right
	wall[2][2] = true // the new tile itself
	got := scorePlacement(&wall, 2, 2)
	want := 3 + 3 // hLen=3 (row 2: cols 1,2,3), vLen=3 (col 2: rows 1,2,3)
	if got != want {
		t.Fatalf("cross scored %d, want %d", got, want)
	}
}

// GOTCHA: a tile with a neighbor on only ONE axis scores only that axis's
// run — the "L" shape (up + right, forming an L with the new tile).
func TestScorePlacementL(t *testing.T) {
	var wall [WallSize][WallSize]bool
	wall[1][2] = true // up
	wall[2][3] = true // right
	wall[2][2] = true // the new tile
	got := scorePlacement(&wall, 2, 2)
	want := 2 + 2 // hLen=2 (cols 2,3), vLen=2 (rows 1,2)
	if got != want {
		t.Fatalf("L shape scored %d, want %d", got, want)
	}
}

func TestScorePlacementFullRow(t *testing.T) {
	var wall [WallSize][WallSize]bool
	for c := 0; c < WallSize; c++ {
		wall[0][c] = true
	}
	// No vertical neighbors anywhere else in column 4.
	got := scorePlacement(&wall, 0, 4)
	if got != 5 {
		t.Fatalf("full row (scored at its last cell) = %d, want 5 (hLen=5, vLen=1 contributes 0)", got)
	}
}

func TestScorePlacementFullColumn(t *testing.T) {
	var wall [WallSize][WallSize]bool
	for r := 0; r < WallSize; r++ {
		wall[r][0] = true
	}
	got := scorePlacement(&wall, 4, 0)
	if got != 5 {
		t.Fatalf("full column (scored at its last cell) = %d, want 5 (vLen=5, hLen=1 contributes 0)", got)
	}
}

// GOTCHA: the -1/-1/-2/-2/-2/-3/-3 floor track, cumulative, and clamped at 7
// slots.
func TestFloorPenaltyTrack(t *testing.T) {
	cases := []struct{ n, want int }{
		{0, 0},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 6},
		{5, 8},
		{6, 11},
		{7, 14},
		{8, 14},  // beyond the 7 slots: no further penalty
		{20, 14}, // still clamped
	}
	for _, c := range cases {
		if got := floorPenalty(c.n); got != c.want {
			t.Errorf("floorPenalty(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

func TestGameOver(t *testing.T) {
	var wall [WallSize][WallSize]bool
	if gameOver(&wall) {
		t.Fatal("empty wall should not be game over")
	}
	for c := 0; c < WallSize; c++ {
		wall[3][c] = true
	}
	if !gameOver(&wall) {
		t.Fatal("a complete row should trigger game over")
	}
}

func TestEndBonusesRowsOnly(t *testing.T) {
	var wall [WallSize][WallSize]bool
	for c := 0; c < WallSize; c++ {
		wall[0][c] = true
	}
	rows, cols, colors, total := endBonusesDetailed(&wall)
	if rows != 1 || cols != 0 || colors != 0 {
		t.Fatalf("rows=%d cols=%d colors=%d, want 1,0,0", rows, cols, colors)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
}

func TestEndBonusesColumnOnly(t *testing.T) {
	var wall [WallSize][WallSize]bool
	for r := 0; r < WallSize; r++ {
		wall[r][2] = true
	}
	rows, cols, colors, total := endBonusesDetailed(&wall)
	if rows != 0 || cols != 1 || colors != 0 {
		t.Fatalf("rows=%d cols=%d colors=%d, want 0,1,0", rows, cols, colors)
	}
	if total != 7 {
		t.Fatalf("total = %d, want 7", total)
	}
}

// A full straight column never completes a color, since each column holds
// all 5 different colors (one per row) in this diagonal-shift Latin square —
// only the diagonal set of cells wallColOf(r,color) for r=0..4 does.
func TestEndBonusesColorOnly(t *testing.T) {
	var wall [WallSize][WallSize]bool
	for r := 0; r < WallSize; r++ {
		wall[r][wallColOf(r, ColorSolid)] = true
	}
	rows, cols, colors, total := endBonusesDetailed(&wall)
	if rows != 0 || cols != 0 || colors != 1 {
		t.Fatalf("rows=%d cols=%d colors=%d, want 0,0,1", rows, cols, colors)
	}
	if total != 10 {
		t.Fatalf("total = %d, want 10", total)
	}
}

func TestEndBonusesFullWall(t *testing.T) {
	var wall [WallSize][WallSize]bool
	for r := range wall {
		for c := range wall[r] {
			wall[r][c] = true
		}
	}
	rows, cols, colors, total := endBonusesDetailed(&wall)
	if rows != 5 || cols != 5 || colors != 5 {
		t.Fatalf("rows=%d cols=%d colors=%d, want 5,5,5", rows, cols, colors)
	}
	if total != 5*2+5*7+5*10 {
		t.Fatalf("total = %d, want %d", total, 5*2+5*7+5*10)
	}
	if endBonuses(&wall) != total {
		t.Fatalf("endBonuses = %d, want %d", endBonuses(&wall), total)
	}
}

// TileBoard: two complete lines score onto separate, non-adjacent wall
// cells (so each scores as isolated = 1 point); the rest of each line's
// tiles and all floor tiles are discarded; the floor penalty is applied and
// clamped at 0.
func TestTileBoardScoresAndClamps(t *testing.T) {
	var b Board
	b.Lines[0] = []Color{ColorSolid}                      // cap 1, full
	b.Lines[2] = []Color{ColorRing, ColorRing, ColorRing} // cap 3, full
	b.Lines[1] = []Color{ColorDot}                        // cap 2, NOT full: must stay untouched
	b.Floor = []Color{ColorCross, ColorStripe, ColorSolid}
	b.HasMarker = true // 3 floor tiles + marker = 4 items

	col0 := wallColOf(0, ColorSolid)
	col2 := wallColOf(2, ColorRing)
	if col0 == col2 {
		t.Fatalf("test setup bug: both placements land in the same column (%d) — pick different colors/rows", col0)
	}

	placements, pen, discarded := TileBoard(&b)

	if len(placements) != 2 {
		t.Fatalf("placements = %v, want 2 entries", placements)
	}
	for _, p := range placements {
		if p.Points != 1 {
			t.Errorf("placement %+v scored %d, want 1 (isolated tile)", p, p.Points)
		}
	}
	if !b.Wall[0][col0] || !b.Wall[2][col2] {
		t.Fatal("both wall cells should now be filled")
	}
	if len(b.Lines[0]) != 0 || len(b.Lines[2]) != 0 {
		t.Fatal("completed lines must be cleared after tiling")
	}
	if len(b.Lines[1]) != 1 {
		t.Fatal("an incomplete line must be left untouched")
	}
	wantPen := floorPenalty(4)
	if pen != wantPen {
		t.Fatalf("floor penalty charged = %d, want %d", pen, wantPen)
	}
	// score: +1 +1 (placements) - floorPenalty(4)=6 => -4, clamped to 0.
	if b.Score != 0 {
		t.Fatalf("score = %d, want 0 (clamped)", b.Score)
	}
	// discarded: 2 leftover Ring tiles from line 2, + 3 floor tiles = 5.
	if len(discarded) != 5 {
		t.Fatalf("discarded = %v (len %d), want 5", discarded, len(discarded))
	}
	if len(b.Floor) != 0 || b.HasMarker {
		t.Fatal("floor line (and marker) must be cleared after tiling")
	}
}

func TestTileBoardNoClampWhenPositive(t *testing.T) {
	var b Board
	b.Lines[4] = []Color{ColorSolid, ColorSolid, ColorSolid, ColorSolid, ColorSolid} // cap 5, full
	_, pen, _ := TileBoard(&b)
	if pen != 0 {
		t.Fatalf("no floor tiles should mean 0 penalty, got %d", pen)
	}
	if b.Score != 1 { // isolated placement on an empty wall
		t.Fatalf("score = %d, want 1", b.Score)
	}
}

func TestLineAcceptsThreePartLegality(t *testing.T) {
	var b Board

	// (a) room: line 0 (cap 1) already holds a tile -> full, rejects more.
	b.Lines[0] = []Color{ColorSolid}
	if b.lineAccepts(0, ColorSolid) {
		t.Error("a full line must reject more tiles, even of the same color")
	}

	// (b) same color: line 1 (cap 2) holds Ring; Dot must be rejected.
	b.Lines[1] = []Color{ColorRing}
	if b.lineAccepts(1, ColorDot) {
		t.Error("a line committed to one color must reject a different color")
	}
	if !b.lineAccepts(1, ColorRing) {
		t.Error("a line with room, matching color, should accept more")
	}

	// (c) wall cell already filled: line 3 is empty, but the wall cell for
	// (row=3, ColorCross) is already filled -> must reject even though the
	// line itself has room and no committed color yet.
	col := wallColOf(3, ColorCross)
	b.Wall[3][col] = true
	if b.lineAccepts(3, ColorCross) {
		t.Error("a line must reject a color whose wall cell in that row is already filled")
	}
	// A different color for the same empty line is still fine.
	if !b.lineAccepts(3, ColorDot) {
		t.Error("an empty line should accept a color whose wall cell isn't filled")
	}
}
