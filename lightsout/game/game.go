// Package game holds the pure Lights Out logic. It must NOT import ink so it
// can be unit-tested without the cgo SDK.
package game

import "math/rand"

// Board is an N×N grid of lights. lit[r][c] == true means the light is ON.
type Board struct {
	N     int
	lit   [][]bool
	Moves int // number of taps the player has made this puzzle
}

// New returns an empty (all-off) board of size n×n.
func New(n int) *Board {
	lit := make([][]bool, n)
	for r := range lit {
		lit[r] = make([]bool, n)
	}
	return &Board{N: n, lit: lit}
}

// Lit reports whether the light at (r,c) is on. Out-of-range → false.
func (b *Board) Lit(r, c int) bool {
	if r < 0 || r >= b.N || c < 0 || c >= b.N {
		return false
	}
	return b.lit[r][c]
}

// Toggle flips a single cell (no neighbours). Used by the generator/solver.
func (b *Board) toggle(r, c int) {
	if r < 0 || r >= b.N || c < 0 || c >= b.N {
		return
	}
	b.lit[r][c] = !b.lit[r][c]
}

// Press applies a player press at (r,c): the cell and its orthogonal
// neighbours toggle. Increments the move counter. Returns false if the
// coordinate is off-board (no move counted).
func (b *Board) Press(r, c int) bool {
	if r < 0 || r >= b.N || c < 0 || c >= b.N {
		return false
	}
	b.press(r, c)
	b.Moves++
	return true
}

// press applies the plus-shaped toggle WITHOUT counting a move.
func (b *Board) press(r, c int) {
	b.toggle(r, c)
	b.toggle(r-1, c)
	b.toggle(r+1, c)
	b.toggle(r, c-1)
	b.toggle(r, c+1)
}

// Solved reports whether every light is off.
func (b *Board) Solved() bool {
	for r := 0; r < b.N; r++ {
		for c := 0; c < b.N; c++ {
			if b.lit[r][c] {
				return false
			}
		}
	}
	return true
}

// Count returns how many lights are currently on.
func (b *Board) Count() int {
	n := 0
	for r := 0; r < b.N; r++ {
		for c := 0; c < b.N; c++ {
			if b.lit[r][c] {
				n++
			}
		}
	}
	return n
}

// Generate builds a guaranteed-solvable puzzle by starting from the empty
// board and applying `presses` random plus-toggles. Because every press is its
// own inverse, the result is always reachable back to all-off. The move
// counter is reset to 0. If the random scramble happens to leave the board
// already solved, it retries so the player always gets a real puzzle.
func (b *Board) Generate(presses int, rng *rand.Rand) {
	if presses < 1 {
		presses = 1
	}
	for attempt := 0; attempt < 100; attempt++ {
		b.clear()
		for i := 0; i < presses; i++ {
			b.press(rng.Intn(b.N), rng.Intn(b.N))
		}
		if !b.Solved() {
			break
		}
	}
	b.Moves = 0
}

func (b *Board) clear() {
	for r := 0; r < b.N; r++ {
		for c := 0; c < b.N; c++ {
			b.lit[r][c] = false
		}
	}
}

// Solve returns the set of cells to press to turn every light off, as a bool
// grid (true = press here). It uses Gauss-Jordan elimination over GF(2) on the
// N²×N² toggle matrix, so it works for any board size and any solvable state.
// Returns (solution, true) if solvable, (nil, false) otherwise.
//
// The board itself is NOT modified.
func (b *Board) Solve() ([][]bool, bool) {
	n := b.N
	size := n * n

	// Build augmented matrix: rows = one equation per cell, columns = one
	// variable per cell (did we press it?) plus the RHS (current lit state).
	// A[cell][var] = 1 if pressing `var` toggles `cell`.
	mat := make([][]uint8, size)
	for i := range mat {
		mat[i] = make([]uint8, size+1)
	}
	idx := func(r, c int) int { return r*n + c }
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			cell := idx(r, c)
			// pressing (r,c) affects cell and neighbours
			mat[cell][idx(r, c)] = 1
			if r > 0 {
				mat[cell][idx(r-1, c)] = 1
			}
			if r < n-1 {
				mat[cell][idx(r+1, c)] = 1
			}
			if c > 0 {
				mat[cell][idx(r, c-1)] = 1
			}
			if c < n-1 {
				mat[cell][idx(r, c+1)] = 1
			}
			if b.lit[r][c] {
				mat[cell][size] = 1
			}
		}
	}

	// Gauss-Jordan over GF(2).
	pivotRow := make([]int, size) // pivotRow[col] = row that pivots col, or -1
	for i := range pivotRow {
		pivotRow[i] = -1
	}
	row := 0
	for col := 0; col < size && row < size; col++ {
		// find pivot
		sel := -1
		for r := row; r < size; r++ {
			if mat[r][col] == 1 {
				sel = r
				break
			}
		}
		if sel == -1 {
			continue // free variable
		}
		mat[row], mat[sel] = mat[sel], mat[row]
		// eliminate this column from all other rows
		for r := 0; r < size; r++ {
			if r != row && mat[r][col] == 1 {
				for k := col; k <= size; k++ {
					mat[r][k] ^= mat[row][k]
				}
			}
		}
		pivotRow[col] = row
		row++
	}

	// Consistency check: any row that is all-zero on the left but 1 on the RHS
	// means no solution.
	for r := 0; r < size; r++ {
		allZero := true
		for c := 0; c < size; c++ {
			if mat[r][c] == 1 {
				allZero = false
				break
			}
		}
		if allZero && mat[r][size] == 1 {
			return nil, false
		}
	}

	// Read off a particular solution: free variables = 0, pivots take RHS.
	x := make([]uint8, size)
	for col := 0; col < size; col++ {
		if pr := pivotRow[col]; pr != -1 {
			x[col] = mat[pr][size]
		}
	}

	sol := make([][]bool, n)
	for r := 0; r < n; r++ {
		sol[r] = make([]bool, n)
		for c := 0; c < n; c++ {
			sol[r][c] = x[idx(r, c)] == 1
		}
	}
	return sol, true
}
