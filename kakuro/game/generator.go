package game

import "math/rand"

// Generate builds a Kakuro puzzle on the preset's fixed shape by filling a
// valid digit assignment into its runs (backtracking with per-run no-repeat
// pruning), deriving each run's sum as its clue, recording the fill as each
// cell's Solution, then blanking the entries so the player starts empty.
func Generate(p Preset, rng *rand.Rand) *Puzzle {
	def := shapeDefs[p.Shape%len(shapeDefs)]
	const attempts = 40
	var grid [][]Cell
	var runs []Run
	solved := false
	for a := 0; a < attempts; a++ {
		grid = parseShape(def)
		runs = findRuns(grid)
		if fillRuns(grid, runs, rng) {
			solved = true
			break
		}
	}
	if !solved {
		// Fallback: shouldn't happen for these small vetted shapes, but never
		// leave the app unable to start.
		grid = parseShape(def)
		runs = findRuns(grid)
		fillRunsGreedy(grid, runs, rng)
	}

	// Derive each run's target from the filled solution, write the clue onto
	// its block cell, then commit Solution and blank the player-facing Value.
	for i := range runs {
		sum := 0
		for _, rc := range runs[i].Cells {
			sum += grid[rc[0]][rc[1]].Value
		}
		runs[i].Target = sum
		assignClue(grid, runs[i])
	}
	for r := range grid {
		for c := range grid[r] {
			if grid[r][c].Kind == KindEntry {
				grid[r][c].Solution = grid[r][c].Value
				grid[r][c].Value = 0
			}
		}
	}
	return &Puzzle{W: len(grid[0]), H: len(grid), Grid: grid, Runs: runs}
}

// assignClue writes a run's target sum onto the block cell immediately
// above (for a vertical run) or to the left (for a horizontal run).
func assignClue(grid [][]Cell, run Run) {
	if len(run.Cells) < 2 {
		return
	}
	r0, c0 := run.Cells[0][0], run.Cells[0][1]
	r1, c1 := run.Cells[1][0], run.Cells[1][1]
	if r0 == r1 {
		grid[r0][c0-1].RightClue = run.Target
	} else {
		_ = c1
		grid[r0-1][c0].DownClue = run.Target
	}
}

// fillRuns assigns 1-9 to every entry cell via randomized backtracking over
// cells in reading order, pruning against every run each cell belongs to (no
// repeated digit within a run). Returns false if no valid fill exists.
func fillRuns(grid [][]Cell, runs []Run, rng *rand.Rand) bool {
	h, w := len(grid), 0
	if h > 0 {
		w = len(grid[0])
	}
	runsAt := map[[2]int][]int{}
	for i, run := range runs {
		for _, rc := range run.Cells {
			runsAt[rc] = append(runsAt[rc], i)
		}
	}
	var cells [][2]int
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			if grid[r][c].Kind == KindEntry {
				cells = append(cells, [2]int{r, c})
			}
		}
	}

	var rec func(idx int) bool
	rec = func(idx int) bool {
		if idx == len(cells) {
			return true
		}
		rc := cells[idx]
		order := rng.Perm(9)
		for _, oi := range order {
			v := oi + 1
			if !fitsRuns(grid, runs, runsAt[rc], rc, v) {
				continue
			}
			grid[rc[0]][rc[1]].Value = v
			if rec(idx + 1) {
				return true
			}
			grid[rc[0]][rc[1]].Value = 0
		}
		return false
	}
	return rec(0)
}

// fitsRuns reports whether placing v at rc conflicts with any other filled
// cell sharing one of rc's runs.
func fitsRuns(grid [][]Cell, runs []Run, runIdxs []int, rc [2]int, v int) bool {
	for _, ri := range runIdxs {
		for _, other := range runs[ri].Cells {
			if other != rc && grid[other[0]][other[1]].Value == v {
				return false
			}
		}
	}
	return true
}

// fillRunsGreedy is a non-backtracking best-effort fallback that never fails
// to terminate; guards against a hypothetical bad shape rather than the
// vetted shapes shipped here, which always succeed via fillRuns.
func fillRunsGreedy(grid [][]Cell, runs []Run, rng *rand.Rand) {
	h, w := len(grid), 0
	if h > 0 {
		w = len(grid[0])
	}
	runsAt := map[[2]int][]int{}
	for i, run := range runs {
		for _, rc := range run.Cells {
			runsAt[rc] = append(runsAt[rc], i)
		}
	}
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			if grid[r][c].Kind != KindEntry {
				continue
			}
			rc := [2]int{r, c}
			for v := 1; v <= 9; v++ {
				if fitsRuns(grid, runs, runsAt[rc], rc, v) {
					grid[r][c].Value = v
					break
				}
			}
			if grid[r][c].Value == 0 {
				grid[r][c].Value = 1 + rng.Intn(9)
			}
		}
	}
}
