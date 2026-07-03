// Package game implements the connection game Hex with no dependency on the
// inkview SDK, so it is unit-tested cgo-free.
//
// Hex is played on an NxN rhombus of hexagons. Black connects the top edge to
// the bottom edge; White connects the left edge to the right edge. Players
// alternate placing one stone on any empty cell; stones are never moved or
// captured. Exactly one player can win, and draws are impossible.
//
// The board uses axial-ish array coordinates (col x, row y). Each interior cell
// has six neighbors; on the rhombus these are the offsets below.
package game

// Cell is the state of one hexagon.
type Cell int8

const (
	Empty Cell = iota
	Black      // connects top<->bottom
	White      // connects left<->right
)

// Opponent returns the other color.
func (c Cell) Opponent() Cell {
	switch c {
	case Black:
		return White
	case White:
		return Black
	default:
		return Empty
	}
}

// neighbors are the six hex-grid neighbor offsets in this coordinate system.
var neighbors = [6][2]int{
	{1, 0}, {-1, 0}, // same row
	{0, 1}, {0, -1}, // same col
	{1, -1}, {-1, 1}, // the two diagonal hex neighbors
}

// Board is an NxN Hex board addressed [y*N+x].
type Board struct {
	N     int
	cells []Cell
}

// NewBoard returns an empty NxN board.
func NewBoard(n int) *Board {
	return &Board{N: n, cells: make([]Cell, n*n)}
}

// Clone returns a deep copy.
func (b *Board) Clone() *Board {
	cp := &Board{N: b.N, cells: make([]Cell, len(b.cells))}
	copy(cp.cells, b.cells)
	return cp
}

func (b *Board) inBounds(x, y int) bool { return x >= 0 && x < b.N && y >= 0 && y < b.N }

// At returns the cell at (x,y).
func (b *Board) At(x, y int) Cell { return b.cells[y*b.N+x] }

// Set places c at (x,y).
func (b *Board) Set(x, y int, c Cell) { b.cells[y*b.N+x] = c }

// Place puts a stone for player at (x,y) if empty. Returns false if occupied or
// out of bounds.
func (b *Board) Place(x, y int, player Cell) bool {
	if !b.inBounds(x, y) || b.At(x, y) != Empty {
		return false
	}
	b.Set(x, y, player)
	return true
}

// EmptyCells returns all empty (x,y) coordinates.
func (b *Board) EmptyCells() [][2]int {
	var out [][2]int
	for y := 0; y < b.N; y++ {
		for x := 0; x < b.N; x++ {
			if b.At(x, y) == Empty {
				out = append(out, [2]int{x, y})
			}
		}
	}
	return out
}

// Winner returns Black or White if that player has a connecting path, else
// Empty. Black connects rows y=0 to y=N-1; White connects cols x=0 to x=N-1.
func (b *Board) Winner() Cell {
	if b.connects(Black) {
		return Black
	}
	if b.connects(White) {
		return White
	}
	return Empty
}

// connects runs a flood fill from the player's start edge to see if it reaches
// the far edge through same-colored cells.
func (b *Board) connects(player Cell) bool {
	n := b.N
	visited := make([]bool, n*n)
	var stack [][2]int

	// Seed from the start edge.
	if player == Black {
		for x := 0; x < n; x++ {
			if b.At(x, 0) == Black {
				stack = append(stack, [2]int{x, 0})
				visited[x] = true
			}
		}
	} else {
		for y := 0; y < n; y++ {
			if b.At(0, y) == White {
				stack = append(stack, [2]int{0, y})
				visited[y*n] = true
			}
		}
	}

	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		x, y := p[0], p[1]
		// Reached the far edge?
		if player == Black && y == n-1 {
			return true
		}
		if player == White && x == n-1 {
			return true
		}
		for _, d := range neighbors {
			nx, ny := x+d[0], y+d[1]
			if !b.inBounds(nx, ny) {
				continue
			}
			idx := ny*n + nx
			if !visited[idx] && b.At(nx, ny) == player {
				visited[idx] = true
				stack = append(stack, [2]int{nx, ny})
			}
		}
	}
	return false
}
