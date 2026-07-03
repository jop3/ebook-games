package game

// shapes.go: fixed hand-designed grid layouts (which cells are black/blocked
// vs white entry cells). Generating arbitrary valid Kakuro topologies from
// scratch is notoriously hard (every run must have length >= 2, no run may
// have length > 9, and the runs must interlock); using a small fixed pool of
// vetted shapes per difficulty is far more robust than procedural shape
// generation, at the cost of the same layout recurring across games at a
// given size — the digit fill (and therefore clues) still differs each time.
//
// Shape encoding: a string grid, one char per cell, rows separated by '\n'.
// '#' = block (no clue, border), '.' = entry cell. All shapes are rectangular
// and always have '#' cells framing entry runs (row 0 and col 0 are block
// borders so every run has a clue cell immediately above/left of it).

var shapeDefs = []string{
	// 0: Lätt 6x6 -- simple interlocking runs, short (length 2-4).
	"" +
		"######\n" +
		"#..#.#\n" +
		"#.....\n" +
		"##....\n" +
		"#.....\n" +
		"#.##..\n",
	// 1: Medel 8x8
	"" +
		"########\n" +
		"#...#..#\n" +
		"#.......\n" +
		"#.#.....\n" +
		"#.......\n" +
		"#....#..\n" +
		"#.......\n" +
		"##..#...\n",
	// 2: Svår 10x10
	"" +
		"##########\n" +
		"#....#...#\n" +
		"#.........\n" +
		"#.#.......\n" +
		"#.........\n" +
		"#....#....\n" +
		"#.........\n" +
		"#.......#.\n" +
		"#.........\n" +
		"##..#....#\n",
}

// parseShape builds a grid of Cell{Kind:KindBlock/KindEntry} from a shape
// definition string, with clues left at 0 (to be filled by the generator).
func parseShape(def string) [][]Cell {
	var rows []string
	cur := ""
	for _, r := range def {
		if r == '\n' {
			rows = append(rows, cur)
			cur = ""
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		rows = append(rows, cur)
	}
	grid := make([][]Cell, len(rows))
	for r, row := range rows {
		grid[r] = make([]Cell, len(row))
		for c, ch := range row {
			if ch == '.' {
				grid[r][c] = Cell{Kind: KindEntry}
			} else {
				grid[r][c] = Cell{Kind: KindBlock}
			}
		}
	}
	return grid
}

// findRuns scans a grid for horizontal and vertical entry-cell runs (length
// >= 2), returning them alongside the block cell whose Down/Right clue each
// run corresponds to (clues are filled in by the caller once sums are known).
func findRuns(grid [][]Cell) []Run {
	h := len(grid)
	w := 0
	if h > 0 {
		w = len(grid[0])
	}
	var runs []Run

	// Horizontal runs.
	for r := 0; r < h; r++ {
		c := 0
		for c < w {
			if grid[r][c].Kind != KindEntry {
				c++
				continue
			}
			start := c
			for c < w && grid[r][c].Kind == KindEntry {
				c++
			}
			if c-start >= 2 {
				var cells [][2]int
				for cc := start; cc < c; cc++ {
					cells = append(cells, [2]int{r, cc})
				}
				runs = append(runs, Run{Cells: cells})
			}
		}
	}
	// Vertical runs.
	for c := 0; c < w; c++ {
		r := 0
		for r < h {
			if grid[r][c].Kind != KindEntry {
				r++
				continue
			}
			start := r
			for r < h && grid[r][c].Kind == KindEntry {
				r++
			}
			if r-start >= 2 {
				var cells [][2]int
				for rr := start; rr < r; rr++ {
					cells = append(cells, [2]int{rr, c})
				}
				runs = append(runs, Run{Cells: cells})
			}
		}
	}
	return runs
}
