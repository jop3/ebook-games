package game

// Cell identifies a grid position.
type Cell struct{ Row, Col int }

// Conflicts returns the set of cells that clash with another filled
// cell in the same row, column, or box (same value). Empty cells are
// never reported. Both members of a clashing pair are included, so the
// UI can highlight every offending cell.
func (b Board) Conflicts() map[Cell]bool {
	out := map[Cell]bool{}
	// Rows.
	for r := 0; r < N; r++ {
		var seen [10][]int // value -> list of columns
		for c := 0; c < N; c++ {
			v := b[r][c]
			if v == 0 {
				continue
			}
			seen[v] = append(seen[v], c)
		}
		for v := 1; v <= 9; v++ {
			if len(seen[v]) > 1 {
				for _, c := range seen[v] {
					out[Cell{r, c}] = true
				}
			}
		}
	}
	// Columns.
	for c := 0; c < N; c++ {
		var seen [10][]int
		for r := 0; r < N; r++ {
			v := b[r][c]
			if v == 0 {
				continue
			}
			seen[v] = append(seen[v], r)
		}
		for v := 1; v <= 9; v++ {
			if len(seen[v]) > 1 {
				for _, r := range seen[v] {
					out[Cell{r, c}] = true
				}
			}
		}
	}
	// Boxes.
	for br := 0; br < N; br += 3 {
		for bc := 0; bc < N; bc += 3 {
			var seen [10][]Cell
			for r := br; r < br+3; r++ {
				for c := bc; c < bc+3; c++ {
					v := b[r][c]
					if v == 0 {
						continue
					}
					seen[v] = append(seen[v], Cell{r, c})
				}
			}
			for v := 1; v <= 9; v++ {
				if len(seen[v]) > 1 {
					for _, cell := range seen[v] {
						out[cell] = true
					}
				}
			}
		}
	}
	return out
}
