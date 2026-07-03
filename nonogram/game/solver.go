package game

// solver.go: a line-based constraint solver. A nonogram is "line solvable" if
// repeatedly resolving each row and column against its clue drives every cell to
// a definite state. This is exactly the class of puzzles a human solves by pure
// deduction (no guessing), so we use it to accept only fair, uniquely-solvable
// generated puzzles.

// tri is a cell's solve state.
type tri int8

const (
	unknown tri = iota
	tFilled
	tBlank
)

// lineArrangements enumerates all placements of clue c over a line of length n,
// respecting any already-known cells in known (len n). It returns, for each
// cell, whether it is filled in EVERY valid arrangement (forcedFill) and blank
// in every valid arrangement (forcedBlank). ok is false if no arrangement fits
// (a contradiction).
func lineArrangements(c Clue, known []tri) (forcedFill, forcedBlank []bool, ok bool) {
	n := len(known)
	fillCount := make([]int, n) // arrangements where cell is filled
	blankCount := make([]int, n)
	total := 0

	// Recursive placement of run index ri starting at position pos.
	var place func(ri, pos int, acc []bool)
	place = func(ri, pos int, acc []bool) {
		if ri == len(c) {
			// Remaining cells are blank; validate against known and record.
			for i := pos; i < n; i++ {
				acc[i] = false
			}
			if consistent(acc, known) {
				total++
				for i := 0; i < n; i++ {
					if acc[i] {
						fillCount[i]++
					} else {
						blankCount[i]++
					}
				}
			}
			return
		}
		runLen := c[ri]
		// remaining minimal width for runs after ri (including gaps)
		restMin := 0
		for j := ri + 1; j < len(c); j++ {
			restMin += c[j] + 1
		}
		// latest start so the rest still fits
		last := n - restMin - runLen
		for start := pos; start <= last; start++ {
			// cells [pos,start) are blank
			feasible := true
			for i := pos; i < start; i++ {
				acc[i] = false
				if known[i] == tFilled {
					feasible = false
				}
			}
			if !feasible {
				continue
			}
			// place the run [start, start+runLen)
			for i := start; i < start+runLen; i++ {
				acc[i] = true
				if known[i] == tBlank {
					feasible = false
				}
			}
			if feasible {
				// one blank gap after the run (unless it's the last run)
				next := start + runLen
				if ri < len(c)-1 {
					if next < n && known[next] == tFilled {
						// need a gap here but cell is known filled -> invalid
					} else {
						if next < n {
							acc[next] = false
						}
						place(ri+1, next+1, acc)
					}
				} else {
					place(ri+1, next, acc)
				}
			}
		}
	}

	acc := make([]bool, n)
	// Special case: empty clue => all blank.
	if len(c) == 0 {
		for i := 0; i < n; i++ {
			acc[i] = false
		}
		if consistent(acc, known) {
			ff := make([]bool, n)
			fb := make([]bool, n)
			for i := range fb {
				fb[i] = true
			}
			return ff, fb, true
		}
		return nil, nil, false
	}

	place(0, 0, acc)
	if total == 0 {
		return nil, nil, false
	}
	forcedFill = make([]bool, n)
	forcedBlank = make([]bool, n)
	for i := 0; i < n; i++ {
		if fillCount[i] == total {
			forcedFill[i] = true
		}
		if blankCount[i] == total {
			forcedBlank[i] = true
		}
	}
	return forcedFill, forcedBlank, true
}

// consistent reports whether a candidate arrangement agrees with known cells.
func consistent(acc []bool, known []tri) bool {
	for i := range acc {
		switch known[i] {
		case tFilled:
			if !acc[i] {
				return false
			}
		case tBlank:
			if acc[i] {
				return false
			}
		}
	}
	return true
}

// SolveResult reports the outcome of the line solver.
type SolveResult int

const (
	// SolveUnique means the puzzle resolves fully by line deduction alone.
	SolveUnique SolveResult = iota
	// SolveStuck means deduction stalled with cells still unknown (the puzzle
	// would require guessing — we reject these when generating).
	SolveStuck
	// SolveContradiction means the clues admit no solution.
	SolveContradiction
)

// LineSolvable runs the row/column line solver to a fixpoint and reports whether
// the puzzle is uniquely solvable by deduction.
func LineSolvable(rowClues, colClues []Clue) SolveResult {
	w := len(colClues)
	h := len(rowClues)
	grid := make([][]tri, h)
	for y := range grid {
		grid[y] = make([]tri, w)
	}

	for {
		changed := false
		// Rows.
		for y := 0; y < h; y++ {
			ff, fb, ok := lineArrangements(rowClues[y], grid[y])
			if !ok {
				return SolveContradiction
			}
			for x := 0; x < w; x++ {
				if ff[x] && grid[y][x] != tFilled {
					grid[y][x] = tFilled
					changed = true
				}
				if fb[x] && grid[y][x] != tBlank {
					grid[y][x] = tBlank
					changed = true
				}
			}
		}
		// Columns.
		for x := 0; x < w; x++ {
			col := make([]tri, h)
			for y := 0; y < h; y++ {
				col[y] = grid[y][x]
			}
			ff, fb, ok := lineArrangements(colClues[x], col)
			if !ok {
				return SolveContradiction
			}
			for y := 0; y < h; y++ {
				if ff[y] && grid[y][x] != tFilled {
					grid[y][x] = tFilled
					changed = true
				}
				if fb[y] && grid[y][x] != tBlank {
					grid[y][x] = tBlank
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}

	// Fully resolved?
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if grid[y][x] == unknown {
				return SolveStuck
			}
		}
	}
	return SolveUnique
}
