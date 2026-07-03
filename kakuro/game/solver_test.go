package game

import (
	"math/rand"
	"testing"
)

func TestShapesAreRectangular(t *testing.T) {
	for i, def := range shapeDefs {
		grid := parseShape(def)
		w := len(grid[0])
		for r, row := range grid {
			if len(row) != w {
				t.Fatalf("shape %d row %d has width %d, want %d", i, r, len(row), w)
			}
		}
	}
}

func TestShapeRunLengthsValid(t *testing.T) {
	for i, def := range shapeDefs {
		grid := parseShape(def)
		runs := findRuns(grid)
		if len(runs) == 0 {
			t.Fatalf("shape %d has no runs at all", i)
		}
		for _, run := range runs {
			if len(run.Cells) < 2 || len(run.Cells) > 9 {
				t.Fatalf("shape %d has a run of length %d (must be 2-9)", i, len(run.Cells))
			}
		}
	}
}

func TestEveryEntryCellHasAtLeastOneRun(t *testing.T) {
	for i, def := range shapeDefs {
		grid := parseShape(def)
		runs := findRuns(grid)
		covered := map[[2]int]bool{}
		for _, run := range runs {
			for _, rc := range run.Cells {
				covered[rc] = true
			}
		}
		for r := range grid {
			for c := range grid[r] {
				if grid[r][c].Kind == KindEntry && !covered[[2]int{r, c}] {
					t.Fatalf("shape %d entry cell (%d,%d) belongs to no run", i, r, c)
				}
			}
		}
	}
}

func TestGeneratorProducesValidFill(t *testing.T) {
	for _, p := range Presets {
		for seed := int64(0); seed < 10; seed++ {
			rng := rand.New(rand.NewSource(seed*97 + 3))
			puz := Generate(p, rng)
			for _, run := range puz.Runs {
				seen := map[int]bool{}
				sum := 0
				for _, rc := range run.Cells {
					v := puz.Grid[rc[0]][rc[1]].Solution
					if v < 1 || v > 9 {
						t.Fatalf("%s seed %d: solution digit %d out of range", p.Name, seed, v)
					}
					if seen[v] {
						t.Fatalf("%s seed %d: repeated digit %d in a run", p.Name, seed, v)
					}
					seen[v] = true
					sum += v
				}
				if sum != run.Target {
					t.Fatalf("%s seed %d: run sums to %d, target says %d", p.Name, seed, sum, run.Target)
				}
			}
			// Entry cells should start blank (Value 0) even though Solution is set.
			for r := range puz.Grid {
				for c := range puz.Grid[r] {
					cell := puz.Grid[r][c]
					if cell.Kind == KindEntry && cell.Value != 0 {
						t.Fatalf("%s seed %d: entry cell (%d,%d) not blank at start", p.Name, seed, r, c)
					}
				}
			}
		}
	}
}

func TestSetDigitAndRunOK(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	gs := NewGameSeeded(Presets[0], 1)
	_ = rng
	// Find the first run and fill it with its solution to confirm RunOK passes.
	run := gs.Puz.Runs[0]
	for _, rc := range run.Cells {
		sol := gs.Puz.Grid[rc[0]][rc[1]].Solution
		gs.SetDigit(rc[0], rc[1], sol)
	}
	if !RunOK(gs.Puz, run) {
		t.Fatal("run filled with its own solution should be OK")
	}
	// Now break it: set the first cell to something equal to the second cell's
	// value if possible, to trigger the no-repeat rule.
	if len(run.Cells) >= 2 {
		v2 := gs.Puz.Grid[run.Cells[1][0]][run.Cells[1][1]].Value
		gs.SetDigit(run.Cells[0][0], run.Cells[0][1], v2)
		if RunOK(gs.Puz, run) {
			t.Fatal("run with a repeated digit should not be OK")
		}
	}
}

func TestFullSolutionSetsDone(t *testing.T) {
	gs := NewGameSeeded(Presets[0], 7)
	for r := range gs.Puz.Grid {
		for c := range gs.Puz.Grid[r] {
			cell := gs.Puz.Grid[r][c]
			if cell.Kind == KindEntry {
				gs.SetDigit(r, c, cell.Solution)
			}
		}
	}
	if !gs.Done {
		t.Fatal("filling every entry cell with its solution should set Done")
	}
}

func TestResetClearsDigits(t *testing.T) {
	gs := NewGameSeeded(Presets[0], 3)
	gs.SetDigit(gs.Puz.Runs[0].Cells[0][0], gs.Puz.Runs[0].Cells[0][1], 5)
	gs.Reset()
	for r := range gs.Puz.Grid {
		for c := range gs.Puz.Grid[r] {
			if gs.Puz.Grid[r][c].Kind == KindEntry && gs.Puz.Grid[r][c].Value != 0 {
				t.Fatal("Reset should clear all entry values")
			}
		}
	}
	if gs.Done {
		t.Fatal("Reset should clear Done")
	}
}
