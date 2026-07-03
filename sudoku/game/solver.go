package game

// Solve fills the first solution found into a copy of the board and
// returns it with ok=true, or ok=false if the puzzle is unsolvable.
// The receiver is not modified.
func (b Board) Solve() (Board, bool) {
	work := b
	ok := work.solveFirst()
	return work, ok
}

// solveFirst is a backtracking solver using the "minimum remaining
// values" heuristic: it always fills the empty cell with the fewest
// legal candidates first, which prunes the search hard.
func (b *Board) solveFirst() bool {
	row, col, cands, found := b.bestCell()
	if !found {
		return true // no empty cell -> solved
	}
	if len(cands) == 0 {
		return false // dead end
	}
	for _, v := range cands {
		b[row][col] = v
		if b.solveFirst() {
			return true
		}
	}
	b[row][col] = 0
	return false
}

// CountSolutions counts solutions up to a limit, stopping early once
// the limit is reached. Passing limit=2 answers the practical question
// "is the solution unique?" cheaply: a result of 1 means unique.
func (b Board) CountSolutions(limit int) int {
	work := b
	count := 0
	work.countSolutions(limit, &count)
	return count
}

func (b *Board) countSolutions(limit int, count *int) {
	if *count >= limit {
		return
	}
	row, col, cands, found := b.bestCell()
	if !found {
		*count++
		return
	}
	for _, v := range cands {
		b[row][col] = v
		b.countSolutions(limit, count)
		b[row][col] = 0
		if *count >= limit {
			return
		}
	}
}

// bestCell returns the empty cell with the fewest legal candidates and
// that candidate list. found=false means the board is full.
func (b *Board) bestCell() (bestR, bestC int, bestCands []int, found bool) {
	best := 10
	for r := 0; r < N; r++ {
		for c := 0; c < N; c++ {
			if b[r][c] != 0 {
				continue
			}
			cands := b.candidates(r, c)
			if len(cands) < best {
				best = len(cands)
				bestR, bestC, bestCands, found = r, c, cands, true
				if best == 0 {
					return // dead cell: no point looking further
				}
			}
		}
	}
	return
}

// candidates returns the values that may legally go in an empty cell.
func (b *Board) candidates(row, col int) []int {
	var used [10]bool
	for i := 0; i < N; i++ {
		used[b[row][i]] = true
		used[b[i][col]] = true
	}
	br, bc := (row/3)*3, (col/3)*3
	for r := br; r < br+3; r++ {
		for c := bc; c < bc+3; c++ {
			used[b[r][c]] = true
		}
	}
	var out []int
	for v := 1; v <= 9; v++ {
		if !used[v] {
			out = append(out, v)
		}
	}
	return out
}
