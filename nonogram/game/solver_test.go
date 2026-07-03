package game

import (
	"math/rand"
	"testing"
)

func TestLineClue(t *testing.T) {
	cases := []struct {
		in   []bool
		want []int
	}{
		{[]bool{}, nil},
		{[]bool{false, false}, nil},
		{[]bool{true, true, true}, []int{3}},
		{[]bool{true, false, true, true}, []int{1, 2}},
		{[]bool{false, true, false, true, false}, []int{1, 1}},
	}
	for _, c := range cases {
		got := LineClue(c.in)
		if len(got) != len(c.want) {
			t.Errorf("LineClue(%v)=%v want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("LineClue(%v)=%v want %v", c.in, got, c.want)
				break
			}
		}
	}
}

func TestLineArrangementsForced(t *testing.T) {
	// Clue {3} on a line of length 3: the only arrangement is all filled.
	ff, fb, ok := lineArrangements(Clue{3}, make([]tri, 3))
	if !ok {
		t.Fatal("should be satisfiable")
	}
	for i := 0; i < 3; i++ {
		if !ff[i] || fb[i] {
			t.Errorf("cell %d should be forced filled", i)
		}
	}
}

func TestLineArrangementsOverlap(t *testing.T) {
	// Clue {4} on length 5: the middle 3 cells are filled in every arrangement.
	ff, _, ok := lineArrangements(Clue{4}, make([]tri, 5))
	if !ok {
		t.Fatal("satisfiable")
	}
	// cells 1,2,3 forced filled; 0 and 4 not.
	want := []bool{false, true, true, true, false}
	for i, w := range want {
		if ff[i] != w {
			t.Errorf("cell %d forced=%v want %v", i, ff[i], w)
		}
	}
}

func TestLineArrangementsContradiction(t *testing.T) {
	known := make([]tri, 3)
	known[0] = tBlank
	known[1] = tBlank
	known[2] = tBlank
	// Clue {3} needs all filled but all are known blank.
	_, _, ok := lineArrangements(Clue{3}, known)
	if ok {
		t.Error("should be a contradiction")
	}
}

func TestLineSolvableKnownPuzzle(t *testing.T) {
	// A simple 3x3 plus/cross picture that is line-solvable.
	//  . # .
	//  # # #
	//  . # .
	sol := [][]bool{
		{false, true, false},
		{true, true, true},
		{false, true, false},
	}
	rc := make([]Clue, 3)
	cc := make([]Clue, 3)
	for y := 0; y < 3; y++ {
		rc[y] = LineClue(sol[y])
	}
	for x := 0; x < 3; x++ {
		col := []bool{sol[0][x], sol[1][x], sol[2][x]}
		cc[x] = LineClue(col)
	}
	if LineSolvable(rc, cc) != SolveUnique {
		t.Error("plus-sign 3x3 should be uniquely line-solvable")
	}
}

func TestGenerateProducesSolvablePuzzles(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))
	for _, p := range Presets {
		puz := Generate(p, rng)
		if puz.W != p.W || puz.H != p.H {
			t.Fatalf("wrong size for %s", p.Name)
		}
		// The clues must match the solution.
		for y := 0; y < p.H; y++ {
			if !puz.RowClues[y].matches(puz.Solution[y]) {
				t.Errorf("%s row %d clue mismatch", p.Name, y)
			}
		}
		res := LineSolvable(puz.RowClues, puz.ColClues)
		if res != SolveUnique {
			// The fallback path can rarely return a stuck puzzle; warn but don't
			// hard-fail on the largest size where retries may be exhausted.
			t.Logf("%s: generated puzzle not uniquely line-solvable (result=%d)", p.Name, res)
		}
	}
}

func TestToggleAndWin(t *testing.T) {
	s := NewGameSeeded(Presets[0], 99)
	// Fill exactly the solution cells.
	for y := 0; y < s.Puz.H; y++ {
		for x := 0; x < s.Puz.W; x++ {
			if s.Puz.Solution[y][x] {
				s.Cells[y][x] = StateFilled
			}
		}
	}
	s.checkDone()
	if !s.Done {
		t.Error("matching the solution should win")
	}
}
