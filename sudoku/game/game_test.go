package game

import (
	"math/rand"
	"testing"
)

// A known puzzle with a single solution (a classic "easy" grid).
var knownPuzzle = Board{
	{5, 3, 0, 0, 7, 0, 0, 0, 0},
	{6, 0, 0, 1, 9, 5, 0, 0, 0},
	{0, 9, 8, 0, 0, 0, 0, 6, 0},
	{8, 0, 0, 0, 6, 0, 0, 0, 3},
	{4, 0, 0, 8, 0, 3, 0, 0, 1},
	{7, 0, 0, 0, 2, 0, 0, 0, 6},
	{0, 6, 0, 0, 0, 0, 2, 8, 0},
	{0, 0, 0, 4, 1, 9, 0, 0, 5},
	{0, 0, 0, 0, 8, 0, 0, 7, 9},
}

var knownSolution = Board{
	{5, 3, 4, 6, 7, 8, 9, 1, 2},
	{6, 7, 2, 1, 9, 5, 3, 4, 8},
	{1, 9, 8, 3, 4, 2, 5, 6, 7},
	{8, 5, 9, 7, 6, 1, 4, 2, 3},
	{4, 2, 6, 8, 5, 3, 7, 9, 1},
	{7, 1, 3, 9, 2, 4, 8, 5, 6},
	{9, 6, 1, 5, 3, 7, 2, 8, 4},
	{2, 8, 7, 4, 1, 9, 6, 3, 5},
	{3, 4, 5, 2, 8, 6, 1, 7, 9},
}

func TestSolveKnown(t *testing.T) {
	got, ok := knownPuzzle.Solve()
	if !ok {
		t.Fatal("known puzzle reported unsolvable")
	}
	if got != knownSolution {
		t.Fatalf("solver produced wrong solution:\n%v", got)
	}
}

func TestKnownPuzzleUnique(t *testing.T) {
	if n := knownPuzzle.CountSolutions(2); n != 1 {
		t.Fatalf("known puzzle should have exactly 1 solution, got %d", n)
	}
}

func TestEmptyBoardManySolutions(t *testing.T) {
	var empty Board
	// An empty grid has an enormous number of solutions; the counter
	// must bail out at the limit rather than enumerate them all.
	if n := empty.CountSolutions(2); n != 2 {
		t.Fatalf("empty board should hit the limit of 2, got %d", n)
	}
}

func TestConflictDetection(t *testing.T) {
	var b Board
	// Row conflict.
	b[0][0] = 5
	b[0][4] = 5
	// Column conflict.
	b[2][2] = 3
	b[6][2] = 3
	// Box conflict.
	b[7][7] = 9
	b[8][8] = 9

	conf := b.Conflicts()
	want := []Cell{
		{0, 0}, {0, 4}, // row
		{2, 2}, {6, 2}, // column
		{7, 7}, {8, 8}, // box
	}
	for _, c := range want {
		if !conf[c] {
			t.Errorf("expected conflict at %v", c)
		}
	}
	if len(conf) != len(want) {
		t.Errorf("expected %d conflicting cells, got %d: %v", len(want), len(conf), conf)
	}

	// A valid partial board has no conflicts.
	if len(knownPuzzle.Conflicts()) != 0 {
		t.Error("known valid puzzle reported conflicts")
	}
}

func TestSolvedDetection(t *testing.T) {
	if !knownSolution.IsSolved() {
		t.Error("known solution should be reported solved")
	}
	if knownPuzzle.IsSolved() {
		t.Error("incomplete puzzle should not be reported solved")
	}
	// Complete but invalid: swap two cells to create a duplicate.
	bad := knownSolution
	bad[0][0] = bad[0][1]
	if bad.IsSolved() {
		t.Error("complete-but-invalid board should not be reported solved")
	}
}

// TestGenerateUniqueSolution is the headline requirement: generate many
// puzzles across all difficulties and verify EACH has exactly one
// solution, and that the stated solution actually solves the start.
func TestGenerateUniqueSolution(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))
	diffs := []Difficulty{Easy, Medium, Hard}
	const perDiff = 20 // 60 puzzles total; > the required 50

	for _, d := range diffs {
		minClues, maxClues := 82, 0
		for i := 0; i < perDiff; i++ {
			p := Generate(d, rng)

			// Exactly one solution.
			if n := p.Start.CountSolutions(2); n != 1 {
				t.Fatalf("%s puzzle #%d has %d solutions, want 1", d, i, n)
			}
			// The stated solution must solve the start.
			solved, ok := p.Start.Solve()
			if !ok {
				t.Fatalf("%s puzzle #%d unsolvable", d, i)
			}
			if solved != p.Solution {
				t.Fatalf("%s puzzle #%d: solver disagrees with stated solution", d, i)
			}
			// Solution is itself a valid full grid.
			if !p.Solution.IsSolved() {
				t.Fatalf("%s puzzle #%d: stated solution is not a valid grid", d, i)
			}
			// Given flags match non-empty start cells.
			clues := 0
			for r := 0; r < N; r++ {
				for c := 0; c < N; c++ {
					if (p.Start[r][c] != 0) != p.Given[r][c] {
						t.Fatalf("%s puzzle #%d: Given flag mismatch at %d,%d", d, i, r, c)
					}
					if p.Given[r][c] {
						clues++
					}
				}
			}
			if clues < minClues {
				minClues = clues
			}
			if clues > maxClues {
				maxClues = clues
			}
		}
		t.Logf("difficulty %s: %d puzzles OK, clue count range %d..%d", d, perDiff, minClues, maxClues)
	}
}
