package game

import (
	"math/rand"
	"testing"
)

// wallColOf must be a Latin square: every color appears exactly once per
// row, and exactly once per column.
func TestWallColOfIsLatinSquare(t *testing.T) {
	for r := 0; r < WallSize; r++ {
		seen := map[int]bool{}
		for c := Color(0); c < NumColors; c++ {
			col := wallColOf(r, c)
			if col < 0 || col >= WallSize {
				t.Fatalf("wallColOf(%d,%d) = %d out of range", r, c, col)
			}
			if seen[col] {
				t.Fatalf("row %d: color %d collides with another color at column %d", r, c, col)
			}
			seen[col] = true
		}
	}
	for c := Color(0); c < NumColors; c++ {
		seen := map[int]bool{}
		for r := 0; r < WallSize; r++ {
			col := wallColOf(r, c)
			if seen[col] {
				t.Fatalf("color %d appears twice in column %d across rows", c, col)
			}
			seen[col] = true
		}
	}
}

// ColorAt must be the exact inverse of wallColOf.
func TestColorAtIsInverseOfWallColOf(t *testing.T) {
	for r := 0; r < WallSize; r++ {
		for c := Color(0); c < NumColors; c++ {
			col := wallColOf(r, c)
			if got := ColorAt(r, col); got != c {
				t.Fatalf("ColorAt(%d,%d) = %d, want %d (wallColOf inverse)", r, col, got, c)
			}
		}
	}
}

// The known formula from the spec: wallColOf(row,color) = (color-row) mod 5.
func TestWallColOfMatchesStatedFormula(t *testing.T) {
	cases := []struct{ row, color, want int }{
		{0, 0, 0}, {0, 4, 4},
		{1, 0, 4}, {1, 1, 0},
		{4, 0, 1}, {4, 4, 0},
	}
	for _, c := range cases {
		if got := wallColOf(c.row, Color(c.color)); got != c.want {
			t.Errorf("wallColOf(%d,%d) = %d, want %d", c.row, c.color, got, c.want)
		}
	}
}

func TestNewBagHasCorrectCounts(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	bag := NewBag(rng)
	if len(bag) != NumColors*TilesPerColor {
		t.Fatalf("bag size = %d, want %d", len(bag), NumColors*TilesPerColor)
	}
	counts := map[Color]int{}
	for _, c := range bag {
		counts[c]++
	}
	for c := Color(0); c < NumColors; c++ {
		if counts[c] != TilesPerColor {
			t.Errorf("color %d count = %d, want %d", c, counts[c], TilesPerColor)
		}
	}
}

func TestExtractColor(t *testing.T) {
	tiles := []Color{ColorSolid, ColorRing, ColorSolid, ColorDot}
	taken, rest := extractColor(tiles, ColorSolid)
	if len(taken) != 2 {
		t.Fatalf("taken = %v, want 2 solid tiles", taken)
	}
	for _, c := range taken {
		if c != ColorSolid {
			t.Fatalf("taken contains non-solid tile: %v", taken)
		}
	}
	if len(rest) != 2 {
		t.Fatalf("rest = %v, want 2 tiles", rest)
	}
	for _, c := range rest {
		if c == ColorSolid {
			t.Fatalf("rest still contains a solid tile: %v", rest)
		}
	}
}

func TestColorsPresent(t *testing.T) {
	tiles := []Color{ColorDot, ColorSolid, ColorDot, ColorCross}
	got := colorsPresent(tiles)
	want := []Color{ColorSolid, ColorCross, ColorDot}
	if len(got) != len(want) {
		t.Fatalf("colorsPresent = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("colorsPresent = %v, want %v (color-index order)", got, want)
		}
	}
}
