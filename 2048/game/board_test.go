package game

import (
	"math/rand"
	"testing"
)

func mkBoard(rows [Size][Size]int) Board {
	var b Board
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			b.set(x, y, rows[y][x])
		}
	}
	return b
}

func rowOf(b Board, y int) [Size]int {
	var r [Size]int
	for x := 0; x < Size; x++ {
		r[x] = b.at(x, y)
	}
	return r
}

func colOf(b Board, x int) [Size]int {
	var c [Size]int
	for y := 0; y < Size; y++ {
		c[y] = b.at(x, y)
	}
	return c
}

// --- Merge-once-per-move: the classic bug -----------------------------------

func TestSlideLeftMergeOnce(t *testing.T) {
	cases := []struct {
		name   string
		in     [Size]int
		want   [Size]int
		gained int
	}{
		// 2 2 2 2 -> 4 4 0 0, NOT 8 0 0 0.
		{"four_equal", [Size]int{2, 2, 2, 2}, [Size]int{4, 4, 0, 0}, 8},
		// 4 4 2 2 -> 8 4 0 0.
		{"two_pairs", [Size]int{4, 4, 2, 2}, [Size]int{8, 4, 0, 0}, 12},
		// 2 2 4 -> 4 4 0.
		{"two_then_single", [Size]int{2, 2, 4, 0}, [Size]int{4, 4, 0, 0}, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, gained, moved := slideRowLeft(tc.in)
			if out != tc.want {
				t.Fatalf("slideRowLeft(%v) = %v, want %v", tc.in, out, tc.want)
			}
			if gained != tc.gained {
				t.Fatalf("gained = %d, want %d", gained, tc.gained)
			}
			if !moved {
				t.Fatal("expected moved=true")
			}
		})
	}
}

func TestSlideRowLeftNoChangeNoMove(t *testing.T) {
	in := [Size]int{2, 4, 8, 16} // already fully packed, no equal neighbours
	out, gained, moved := slideRowLeft(in)
	if out != in {
		t.Fatalf("row changed: %v -> %v", in, out)
	}
	if gained != 0 || moved {
		t.Fatalf("gained=%d moved=%v, want 0/false", gained, moved)
	}
}

// --- Direction plumbing: each of the 4 directions via hand-checked boards ---

func TestSlideDirections(t *testing.T) {
	// A single row problem placed along each axis, checked against the
	// independently-reasoned expected result for that direction.
	t.Run("left", func(t *testing.T) {
		b := mkBoard([Size][Size]int{
			{2, 2, 0, 0},
			{0, 0, 0, 0},
			{0, 0, 0, 0},
			{0, 0, 0, 0},
		})
		out, gained, moved := Slide(b, Left)
		if rowOf(out, 0) != [Size]int{4, 0, 0, 0} || gained != 4 || !moved {
			t.Fatalf("left: row=%v gained=%d moved=%v", rowOf(out, 0), gained, moved)
		}
	})
	t.Run("right", func(t *testing.T) {
		b := mkBoard([Size][Size]int{
			{2, 2, 0, 0},
			{0, 0, 0, 0},
			{0, 0, 0, 0},
			{0, 0, 0, 0},
		})
		out, gained, moved := Slide(b, Right)
		if rowOf(out, 0) != [Size]int{0, 0, 0, 4} || gained != 4 || !moved {
			t.Fatalf("right: row=%v gained=%d moved=%v", rowOf(out, 0), gained, moved)
		}
	})
	t.Run("up", func(t *testing.T) {
		b := mkBoard([Size][Size]int{
			{2, 0, 0, 0},
			{2, 0, 0, 0},
			{0, 0, 0, 0},
			{0, 0, 0, 0},
		})
		out, gained, moved := Slide(b, Up)
		if colOf(out, 0) != [Size]int{4, 0, 0, 0} || gained != 4 || !moved {
			t.Fatalf("up: col=%v gained=%d moved=%v", colOf(out, 0), gained, moved)
		}
	})
	t.Run("down", func(t *testing.T) {
		b := mkBoard([Size][Size]int{
			{2, 0, 0, 0},
			{2, 0, 0, 0},
			{0, 0, 0, 0},
			{0, 0, 0, 0},
		})
		out, gained, moved := Slide(b, Down)
		if colOf(out, 0) != [Size]int{0, 0, 0, 4} || gained != 4 || !moved {
			t.Fatalf("down: col=%v gained=%d moved=%v", colOf(out, 0), gained, moved)
		}
	})
	// A fuller board exercising compression + merge together, per direction.
	t.Run("left_full_row", func(t *testing.T) {
		b := mkBoard([Size][Size]int{
			{0, 2, 2, 4},
			{0, 0, 0, 0},
			{0, 0, 0, 0},
			{0, 0, 0, 0},
		})
		out, _, _ := Slide(b, Left)
		if rowOf(out, 0) != [Size]int{4, 4, 0, 0} {
			t.Fatalf("left_full_row: row=%v", rowOf(out, 0))
		}
	})
}

func TestSlideNoChangeNoSpawnInvariant(t *testing.T) {
	// A board where every direction is a no-op (fully packed, no equal
	// neighbours anywhere, standard 2048 "checkerboard of distinct powers").
	b := mkBoard([Size][Size]int{
		{2, 4, 2, 4},
		{4, 2, 4, 2},
		{2, 4, 2, 4},
		{4, 2, 4, 2},
	})
	for _, d := range []Dir{Left, Right, Up, Down} {
		out, gained, moved := Slide(b, d)
		if moved || gained != 0 || out != b {
			t.Fatalf("dir %v: expected no-op, got moved=%v gained=%d", d, moved, gained)
		}
	}
}

// --- Spawn -------------------------------------------------------------------

func TestSpawnPlacesOneTile(t *testing.T) {
	var b Board // all empty
	rng := rand.New(rand.NewSource(1))
	out := Spawn(b, rng)
	count := 0
	for _, v := range out {
		if v != 0 {
			count++
			if v != 2 && v != 4 {
				t.Fatalf("spawned value %d, want 2 or 4", v)
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 tile spawned, got %d", count)
	}
}

func TestSpawnOnFullBoardIsNoop(t *testing.T) {
	var b Board
	for i := range b {
		b[i] = 2
	}
	rng := rand.New(rand.NewSource(1))
	out := Spawn(b, rng)
	if out != b {
		t.Fatal("Spawn changed a full board")
	}
}

func TestSpawnDistributionRoughly90Percent(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	twos, fours := 0, 0
	for i := 0; i < 2000; i++ {
		var b Board
		out := Spawn(b, rng)
		for _, v := range out {
			if v == 2 {
				twos++
			} else if v == 4 {
				fours++
			}
		}
	}
	frac := float64(twos) / float64(twos+fours)
	if frac < 0.8 || frac > 0.97 {
		t.Fatalf("2-spawn fraction = %.3f, want roughly 0.9", frac)
	}
}

// --- CanMove / Won -----------------------------------------------------------

func TestCanMoveEmptyCell(t *testing.T) {
	b := mkBoard([Size][Size]int{
		{2, 4, 8, 16},
		{4, 8, 16, 2},
		{8, 16, 2, 4},
		{16, 2, 4, 0}, // one empty cell
	})
	if !CanMove(b) {
		t.Fatal("expected CanMove=true (empty cell present)")
	}
}

func TestCanMoveEqualNeighbourFullBoard(t *testing.T) {
	// Full board (no empty cells) but two equal neighbours exist, so a merge
	// is still possible — must not be reported as game-over.
	b := mkBoard([Size][Size]int{
		{2, 4, 8, 16},
		{4, 8, 16, 2},
		{8, 16, 2, 4},
		{16, 2, 4, 4}, // last two cells equal
	})
	if !CanMove(b) {
		t.Fatal("expected CanMove=true (equal neighbours present on full board)")
	}
}

func TestCanMoveFalseGameOver(t *testing.T) {
	// Full board, no two orthogonal neighbours equal anywhere: no move
	// possible.
	b := mkBoard([Size][Size]int{
		{2, 4, 2, 4},
		{4, 2, 4, 2},
		{2, 4, 2, 4},
		{4, 2, 4, 2},
	})
	if CanMove(b) {
		t.Fatal("expected CanMove=false (game over)")
	}
}

func TestWon(t *testing.T) {
	b := mkBoard([Size][Size]int{
		{2, 4, 8, 16},
		{0, 0, 0, 0},
		{0, 0, 2048, 0},
		{0, 0, 0, 0},
	})
	if !Won(b, 2048) {
		t.Fatal("expected Won(2048)=true")
	}
	if Won(b, 4096) {
		t.Fatal("expected Won(4096)=false")
	}
}
