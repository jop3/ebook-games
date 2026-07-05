package game

import "testing"

func TestFiveHorizontal(t *testing.T) {
	var b Board
	for x := 0; x < 5; x++ {
		b.Ring[3][x] = Black
	}
	if !Five(&b, Black) {
		t.Fatal("5 in a row horizontally should win")
	}
	if Five(&b, White) {
		t.Fatal("White has no rings, should not have five")
	}
}

func TestFiveVertical(t *testing.T) {
	var b Board
	for y := 1; y < 6; y++ {
		b.Ring[y][4] = White
	}
	if !Five(&b, White) {
		t.Fatal("5 in a row vertically should win")
	}
}

func TestFiveDiagonalRising(t *testing.T) {
	var b Board
	// x+y=5 line: (0,5),(1,4),(2,3),(3,2),(4,1),(5,0) — take 5 of the 6.
	pts := [][2]int{{0, 5}, {1, 4}, {2, 3}, {3, 2}, {4, 1}}
	for _, p := range pts {
		b.Ring[p[1]][p[0]] = Black
	}
	if !Five(&b, Black) {
		t.Fatal("5 in a row on the rising diagonal should win")
	}
}

func TestFiveDiagonalFalling(t *testing.T) {
	var b Board
	pts := [][2]int{{1, 0}, {2, 1}, {3, 2}, {4, 3}, {5, 4}}
	for _, p := range pts {
		b.Ring[p[1]][p[0]] = White
	}
	if !Five(&b, White) {
		t.Fatal("5 in a row on the falling diagonal should win")
	}
}

func TestFiveRequiresContiguity(t *testing.T) {
	var b Board
	// 5 Black rings in row 0 but with a gap — not a contiguous run of 5.
	b.Ring[0][0] = Black
	b.Ring[0][1] = Black
	b.Ring[0][2] = Black
	// (3,0) left empty — breaks contiguity
	b.Ring[0][4] = Black
	b.Ring[0][5] = Black
	if Five(&b, Black) {
		t.Fatal("a broken run of rings (not contiguous) must not count as five")
	}
}

func TestLargestGroupOrthogonalOnly(t *testing.T) {
	var b Board
	// An orthogonally-connected L-shape of 4 Black rings: (0,0)-(0,1)-(1,1)-(1,2).
	b.Ring[0][0] = Black
	b.Ring[1][0] = Black
	b.Ring[1][1] = Black
	b.Ring[2][1] = Black
	if got := LargestGroup(&b, Black); got != 4 {
		t.Fatalf("LargestGroup = %d, want 4 (orthogonally connected)", got)
	}
}

func TestLargestGroupExcludesDiagonalAdjacency(t *testing.T) {
	var b Board
	// Two Black rings that only touch diagonally must be 2 SEPARATE groups
	// of size 1 each, not one group of 2.
	b.Ring[0][0] = Black
	b.Ring[1][1] = Black
	if got := LargestGroup(&b, Black); got != 1 {
		t.Fatalf("LargestGroup = %d, want 1 (diagonal adjacency must not connect groups)", got)
	}
}

func TestLargestGroupPicksTheBiggestOfSeveral(t *testing.T) {
	var b Board
	// A group of 3 in one corner...
	b.Ring[0][0] = Black
	b.Ring[0][1] = Black
	b.Ring[1][0] = Black
	// ...and a lone ring elsewhere.
	b.Ring[5][5] = Black
	if got := LargestGroup(&b, Black); got != 3 {
		t.Fatalf("LargestGroup = %d, want 3 (the bigger of the two groups)", got)
	}
}

// --- GOTCHA: full-board tiebreak by largest orthogonal group ----------------

// boardFromRows builds a full board from 6 row strings ('B'=Black,
// otherwise White), used by the tiebreak tests below. Both grids here were
// found by random search and pre-verified (outside this test binary) to
// contain no five-in-a-row for either color, so the tests can focus purely
// on the tiebreak decision.
func boardFromRows(rows [Size]string) Board {
	var b Board
	for y, row := range rows {
		for x, ch := range row {
			if ch == 'B' {
				b.Ring[y][x] = Black
			} else {
				b.Ring[y][x] = White
			}
		}
	}
	return b
}

func TestTiebreakWinnerLargerGroupWins(t *testing.T) {
	// Black's largest connected group is 13, White's is 8; neither color has
	// a five-in-a-row anywhere on the board.
	b := boardFromRows([Size]string{
		"B.BBBB",
		"...B..",
		"..BBBB",
		".BB.B.",
		".B...B",
		"B.BB.B",
	})
	if !boardFull(&b) {
		t.Fatal("setup: board should be full")
	}
	if Five(&b, Black) || Five(&b, White) {
		t.Fatal("setup: neither color should have five in a row")
	}
	bg, wg := LargestGroup(&b, Black), LargestGroup(&b, White)
	if bg != 13 || wg != 8 {
		t.Fatalf("setup: expected group sizes 13/8, got Black=%d White=%d", bg, wg)
	}
	if got := tiebreakWinner(&b); got != Black {
		t.Fatalf("tiebreakWinner = %v, want Black (larger connected group)", got)
	}
	if got := Winner(b); got != Black {
		t.Fatalf("Winner (no five-in-a-row present) = %v, want Black via the tiebreak", got)
	}
}

// TestTiebreakWinnerDrawOnEqualGroups: our documented, explicit choice for a
// full board with no five-in-a-row AND equal largest-group sizes is a draw
// (Empty) — the simplest reasonable tiebreak given the source material
// states none. This must match what the rules screen tells the player.
func TestTiebreakWinnerDrawOnEqualGroups(t *testing.T) {
	b := boardFromRows([Size]string{
		"..B.B.",
		"B.BB.B",
		"B.BB..",
		"B..B..",
		"BB.BB.",
		".B..B.",
	})
	if !boardFull(&b) {
		t.Fatal("setup: board should be full")
	}
	if Five(&b, Black) || Five(&b, White) {
		t.Fatal("setup: neither color should have five in a row")
	}
	bg, wg := LargestGroup(&b, Black), LargestGroup(&b, White)
	if bg != wg || bg != 9 {
		t.Fatalf("setup: expected equal largest groups of 9, got Black=%d White=%d", bg, wg)
	}
	if got := tiebreakWinner(&b); got != Empty {
		t.Fatalf("tiebreakWinner on equal groups = %v, want Empty (a draw)", got)
	}
	if got := Winner(b); got != Empty {
		t.Fatalf("Winner on equal groups = %v, want Empty (a draw)", got)
	}
}
