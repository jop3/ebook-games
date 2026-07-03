package game

import (
	"math/rand"
	"testing"
)

// TestSolve_BorderLoopUnique checks the simple border-rectangle puzzle (every
// cell's clue derived from the border loop) has at least one solution and
// that the solver doesn't crash on a fully-clued board.
func TestSolve_BorderLoopFullClueSolvable(t *testing.T) {
	W, H := 4, 4
	hEdge, vEdge := borderLoop(W, H)
	clue := deriveClues(W, H, hEdge, vEdge)
	p := &Puzzle{W: W, H: H, Clue: clue}
	if Solve(p) == SolveNone {
		t.Fatal("expected the fully-clued border loop puzzle to be solvable")
	}
}

func TestGenerate_SmallPuzzleIsUnique(t *testing.T) {
	for _, size := range []Preset{{"t5", 5, 5}, {"t7", 7, 7}} {
		rng := rand.New(rand.NewSource(42))
		puz := Generate(size, rng)
		res := Solve(puz)
		if res != SolveUnique {
			t.Fatalf("size %v: expected generated puzzle to be uniquely solvable, got %v", size, res)
		}
	}
}

func TestGenerate_Deterministic(t *testing.T) {
	p := Preset{"t5", 5, 5}
	a := Generate(p, rand.New(rand.NewSource(7)))
	b := Generate(p, rand.New(rand.NewSource(7)))
	for y := 0; y < p.H; y++ {
		for x := 0; x < p.W; x++ {
			if a.Clue[y][x] != b.Clue[y][x] {
				t.Fatalf("same seed produced different puzzles at (%d,%d): %d vs %d", x, y, a.Clue[y][x], b.Clue[y][x])
			}
		}
	}
}

func TestGenerate_HasSomeCluesStripped(t *testing.T) {
	// Sanity: stripping should remove at least some clues on a 7x7 (a fully
	// clued board is not the "puzzle" experience).
	rng := rand.New(rand.NewSource(99))
	puz := Generate(Preset{"t7", 7, 7}, rng)
	given := 0
	for y := 0; y < puz.H; y++ {
		for x := 0; x < puz.W; x++ {
			if puz.Clue[y][x] >= 0 {
				given++
			}
		}
	}
	if given == puz.W*puz.H {
		t.Fatal("expected stripClues to remove at least one clue on a 7x7 board")
	}
}

func TestGenerate_AllSizesWithinAttemptBudget(t *testing.T) {
	for _, p := range Presets {
		rng := rand.New(rand.NewSource(1234))
		puz := Generate(p, rng)
		if puz == nil {
			t.Fatalf("Generate returned nil for %v", p)
		}
		if puz.W != p.W || puz.H != p.H {
			t.Fatalf("Generate returned wrong dims for %v", p)
		}
	}
}

func TestSolve_DetectsMultipleSolutions(t *testing.T) {
	// A 2x2 board with NO clues at all has many possible loops (or none) —
	// use a 1-cell board with no clue: zero constraints, multiple solutions
	// (the empty board and... actually with 0 clues there's no unique
	// solution requirement, so the solver should not report Unique).
	p := &Puzzle{W: 2, H: 2, Clue: [][]int{{-1, -1}, {-1, -1}}}
	res := Solve(p)
	if res == SolveUnique {
		t.Fatal("a completely unclued board must not be uniquely solvable")
	}
}
