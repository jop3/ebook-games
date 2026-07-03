// Package game implements the rules of Quarto with no dependency on the
// inkview SDK, so it can be unit-tested cgo-free.
//
// Quarto is played on a 4x4 board with 16 unique pieces, each carrying four
// binary attributes (tall/short, light/dark, round/square, hollow/solid).
// Pieces are shared: on your turn you place the piece your opponent handed
// you, then you choose a piece from the remaining pool and hand it to your
// opponent. A player wins by completing a row, column, or main diagonal of
// four pieces that all share at least one attribute in common (either all
// share the same bit value of "1", or all share the same bit value of "0").
package game

// Piece is a 4-bit value, one bit per attribute:
//
//	bit0: Tall (1) / Short (0)
//	bit1: Dark (1) / Light (0)
//	bit2: Square (1) / Round (0)
//	bit3: Solid (1) / Hollow (0)
type Piece int8

const (
	AttrTall   = 1 << 0
	AttrDark   = 1 << 1
	AttrSquare = 1 << 2
	AttrSolid  = 1 << 3
	NumPieces  = 16
	AttrMask   = 0xF
)

// NoPiece marks an absent piece (empty cell, or "no piece handed over yet").
const NoPiece Piece = -1

// Size is the edge length of the board.
const Size = 4

// Board holds the 4x4 grid in row-major order (index = y*Size + x). A cell
// holds NoPiece if empty, otherwise the piece placed there.
type Board [Size * Size]Piece

func inBounds(x, y int) bool { return x >= 0 && x < Size && y >= 0 && y < Size }

// NewBoard returns an empty board.
func NewBoard() Board {
	var b Board
	for i := range b {
		b[i] = NoPiece
	}
	return b
}

// At returns the piece at (x,y), or NoPiece if empty.
func (b *Board) At(x, y int) Piece { return b[y*Size+x] }

func (b *Board) set(x, y int, p Piece) { b[y*Size+x] = p }

// Empty reports whether (x,y) has no piece.
func (b *Board) Empty(x, y int) bool { return inBounds(x, y) && b.At(x, y) == NoPiece }

// Place puts piece p at (x,y). Reports whether the move was legal (in bounds,
// cell empty, piece valid).
func (b *Board) Place(x, y int, p Piece) bool {
	if !inBounds(x, y) || b.At(x, y) != NoPiece || p < 0 || p >= NumPieces {
		return false
	}
	b.set(x, y, p)
	return true
}

// Full reports whether every cell is occupied.
func (b *Board) Full() bool {
	for _, c := range b {
		if c == NoPiece {
			return false
		}
	}
	return true
}

// lines enumerates the 4 rows, 4 columns, and 2 diagonals as cell-index
// quadruples.
func lines() [10][4]int {
	var ls [10][4]int
	n := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			ls[n][x] = y*Size + x
		}
		n++
	}
	for x := 0; x < Size; x++ {
		for y := 0; y < Size; y++ {
			ls[n][y] = y*Size + x
		}
		n++
	}
	for i := 0; i < Size; i++ {
		ls[n][i] = i*Size + i
	}
	n++
	for i := 0; i < Size; i++ {
		ls[n][i] = i*Size + (Size - 1 - i)
	}
	n++
	return ls
}

var allLines = lines()

// lineShares reports whether the 4 pieces at the given board indices are all
// filled and share at least one attribute bit as all-1 or all-0.
func (b *Board) lineShares(idx [4]int) bool {
	for _, i := range idx {
		if b[i] == NoPiece {
			return false
		}
	}
	p0, p1, p2, p3 := b[idx[0]], b[idx[1]], b[idx[2]], b[idx[3]]
	shareOne := p0 & p1 & p2 & p3
	shareZero := (^p0) & (^p1) & (^p2) & (^p3) & AttrMask
	return shareOne != 0 || shareZero != 0
}

// WinningLine returns the winning line's 4 cell indices and true if any line
// of the board is complete and shares an attribute. Only the first such line
// found is returned (there could be more than one simultaneously).
func (b *Board) WinningLine() ([4]int, bool) {
	for _, ln := range allLines {
		if b.lineShares(ln) {
			return ln, true
		}
	}
	return [4]int{}, false
}

// HasWin reports whether the board currently contains a winning line.
func (b *Board) HasWin() bool {
	_, ok := b.WinningLine()
	return ok
}
