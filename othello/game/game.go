// Package game implements the rules of Othello (Reversi) with no dependency on
// the inkview SDK, so it can be unit-tested cgo-free.
//
// The board is 8x8. Two players, Black and White. A move is legal only if it
// brackets at least one contiguous line of opponent discs between the placed
// disc and another disc of the mover's color; all bracketed discs flip. If a
// player has no legal move, the turn passes; if neither player can move, the
// game ends and the majority of discs wins.
package game

// Cell is one of the three states of a board square.
type Cell int8

const (
	Empty Cell = iota
	Black
	White
)

// Opponent returns the other player color. Only meaningful for Black/White.
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

// Size is the edge length of the board.
const Size = 8

// Board holds the 8x8 grid in row-major order (index = y*Size + x).
type Board [Size * Size]Cell

// eight compass directions for line scanning.
var dirs = [8][2]int{
	{-1, -1}, {0, -1}, {1, -1},
	{-1, 0}, {1, 0},
	{-1, 1}, {0, 1}, {1, 1},
}

func inBounds(x, y int) bool { return x >= 0 && x < Size && y >= 0 && y < Size }

// At returns the cell at (x,y).
func (b *Board) At(x, y int) Cell { return b[y*Size+x] }

func (b *Board) set(x, y int, c Cell) { b[y*Size+x] = c }

// NewBoard returns a board in the standard Othello starting position.
func NewBoard() Board {
	var b Board
	b.set(3, 3, White)
	b.set(4, 4, White)
	b.set(3, 4, Black)
	b.set(4, 3, Black)
	return b
}

// flips returns the list of opponent cells that would flip if player placed a
// disc at (x,y). Empty result means the move is illegal (or the cell is taken).
func (b *Board) flips(x, y int, player Cell) []int {
	if !inBounds(x, y) || b.At(x, y) != Empty {
		return nil
	}
	opp := player.Opponent()
	var out []int
	for _, d := range dirs {
		var line []int
		cx, cy := x+d[0], y+d[1]
		for inBounds(cx, cy) && b.At(cx, cy) == opp {
			line = append(line, cy*Size+cx)
			cx += d[0]
			cy += d[1]
		}
		// Must be closed by one of the player's own discs, with >=1 opp between.
		if len(line) > 0 && inBounds(cx, cy) && b.At(cx, cy) == player {
			out = append(out, line...)
		}
	}
	return out
}

// LegalMove reports whether player may place at (x,y).
func (b *Board) LegalMove(x, y int, player Cell) bool {
	return len(b.flips(x, y, player)) > 0
}

// LegalMoves returns all legal (x,y) placements for player.
func (b *Board) LegalMoves(player Cell) [][2]int {
	var moves [][2]int
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) == Empty && len(b.flips(x, y, player)) > 0 {
				moves = append(moves, [2]int{x, y})
			}
		}
	}
	return moves
}

// HasMove reports whether player has any legal move.
func (b *Board) HasMove(player Cell) bool {
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) == Empty && len(b.flips(x, y, player)) > 0 {
				return true
			}
		}
	}
	return false
}

// Apply places a disc at (x,y) for player and flips the bracketed discs. It
// reports whether the move was legal and actually applied.
func (b *Board) Apply(x, y int, player Cell) bool {
	fl := b.flips(x, y, player)
	if len(fl) == 0 {
		return false
	}
	b.set(x, y, player)
	for _, idx := range fl {
		b[idx] = player
	}
	return true
}

// Count returns the number of discs of the given color.
func (b *Board) Count(c Cell) int {
	n := 0
	for _, v := range b {
		if v == c {
			n++
		}
	}
	return n
}
