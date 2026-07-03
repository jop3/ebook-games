// Package game contains the pure Sudoku logic: board representation,
// a constraint solver that counts solutions with early bail-out,
// a puzzle generator that guarantees a unique solution, and conflict
// detection. It imports NOTHING from the ink SDK so it can be unit
// tested cgo-free.
package game

// A Board is a 9x9 Sudoku grid. Cell values are 0 (empty) or 1..9.
// Index with Board[row][col], row and col in 0..8.
type Board [9][9]int

// N is the side length of the grid.
const N = 9

// Box returns the 3x3 box index (0..8) for the given cell.
func Box(row, col int) int { return (row/3)*3 + col/3 }

// Clone returns a copy of the board (arrays are values in Go, so this
// is just an assignment, but the helper documents intent).
func (b Board) Clone() Board { return b }

// IsComplete reports whether every cell is filled (no zeros). It does
// NOT check validity on its own; use IsSolved for that.
func (b Board) IsComplete() bool {
	for r := 0; r < N; r++ {
		for c := 0; c < N; c++ {
			if b[r][c] == 0 {
				return false
			}
		}
	}
	return true
}

// canPlace reports whether v can be placed at (row,col) without
// violating row/column/box constraints, ignoring the cell itself.
func (b *Board) canPlace(row, col, v int) bool {
	for i := 0; i < N; i++ {
		if b[row][i] == v && i != col {
			return false
		}
		if b[i][col] == v && i != row {
			return false
		}
	}
	br, bc := (row/3)*3, (col/3)*3
	for r := br; r < br+3; r++ {
		for c := bc; c < bc+3; c++ {
			if b[r][c] == v && !(r == row && c == col) {
				return false
			}
		}
	}
	return true
}

// IsValid reports whether the current filled cells contain no
// duplicate in any row, column, or box. Empty cells are ignored.
func (b Board) IsValid() bool {
	return len(b.Conflicts()) == 0
}

// IsSolved reports whether the board is completely filled and valid.
func (b Board) IsSolved() bool {
	return b.IsComplete() && b.IsValid()
}
