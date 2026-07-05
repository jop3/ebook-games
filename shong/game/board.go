// Package game implements the rules of Shong, a lightning chess-like duel on
// a 4x6 board, with no dependency on the inkview SDK so it can be
// unit-tested cgo-free.
//
// Each side starts with 4 pieces on its own back rank: Triangel (moves
// diagonally), Kvadrat (moves orthogonally), X (moves in all 8 directions,
// like a queen's directions) and Kung (moves exactly one step, alternating
// each time it moves between the Triangel's diagonal directions and the
// Kvadrat's orthogonal directions). Triangel/Kvadrat/X start on a "short"
// move of exactly 1 square; once a piece has moved once it is "long" and
// every subsequent move is exactly 2 squares (never 1, never more) — an eye
// mark on the piece records this. No piece may jump: every square strictly
// between the origin and an exact-distance landing square must be empty.
// Landing on an enemy piece captures it by displacement. A side wins by
// capturing the enemy Kung, or by walking its own Kung to the far edge (the
// opponent's back rank).
package game

import "image"

// Kind names a piece's movement type.
type Kind uint8

const (
	Triangle Kind = iota // Triangel — diagonal moves
	Square               // Kvadrat — orthogonal moves
	Ex                   // X — all 8 directions
	King                 // Kung — 1 step, alternating diagonal/orthogonal
)

// Side names a player color.
type Side uint8

const (
	Black Side = iota
	White
)

// Opponent returns the other player color.
func (s Side) Opponent() Side {
	if s == Black {
		return White
	}
	return Black
}

// Piece is a single man on the board.
type Piece struct {
	Kind Kind
	Side Side

	// Moved records whether this piece has made its first move yet: false
	// means its next move is "short" (distance 1), true means "long"
	// (distance 2). Meaningless for King, whose moves are always distance 1
	// — but still set on a King's first move for uniformity.
	Moved bool

	// Ortho is meaningful only for a King: it records which of the two move
	// sets (orthogonal if true, diagonal if false) the King currently moves
	// in. It flips every time THIS King actually moves — not every ply — so
	// it lives on the piece itself rather than as separate per-side turn
	// state.
	Ortho bool
}

// Board dimensions: 4 columns, 6 rows (narrower than tall).
const (
	Cols = 4
	Rows = 6
)

// Board holds the grid, indexed [y][x] (row-major, y=0 is Black's back
// rank). A nil entry is an empty square.
type Board [Rows][Cols]*Piece

func inBounds(x, y int) bool { return x >= 0 && x < Cols && y >= 0 && y < Rows }

// At returns the piece at (x,y), or nil if the square is empty or
// out-of-bounds.
func (b *Board) At(x, y int) *Piece {
	if !inBounds(x, y) {
		return nil
	}
	return b[y][x]
}

// pieceValue is deliberately below.

// diagDirs, orthoDirs and allDirs are the direction vectors used by
// stepSet: diagonal (Triangel), orthogonal (Kvadrat), and all 8 (X).
var diagDirs = [4]image.Point{{X: 1, Y: 1}, {X: 1, Y: -1}, {X: -1, Y: 1}, {X: -1, Y: -1}}
var orthoDirs = [4]image.Point{{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}}
var allDirs = [8]image.Point{
	{X: 1, Y: 1}, {X: 1, Y: -1}, {X: -1, Y: 1}, {X: -1, Y: -1},
	{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1},
}

// stepSet returns p's current direction vectors.
func stepSet(p *Piece) []image.Point {
	switch p.Kind {
	case Triangle:
		return diagDirs[:]
	case Square:
		return orthoDirs[:]
	case Ex:
		return allDirs[:]
	case King:
		if p.Ortho {
			return orthoDirs[:]
		}
		return diagDirs[:]
	}
	return nil
}

// goalRank returns the row index side's King must reach to win by reaching
// the far edge: the opponent's back rank.
func goalRank(side Side) int {
	if side == Black {
		return Rows - 1
	}
	return 0
}

// homeRank returns side's own back rank.
func homeRank(side Side) int {
	if side == Black {
		return 0
	}
	return Rows - 1
}

// columnOrder is the shared back-rank layout, left to right: X, Triangel,
// Kvadrat, Kung. Both sides use the SAME column order (rather than a
// left-right mirrored order) so that each piece type faces its own type
// straight across the board — the same convention chess uses for its back
// rank (e.g. queens both on the d-file). This is the smallest, most natural
// reading of "mirrored" for a board where mirroring is really about the
// top/bottom reflection, not a left-right flip.
var columnOrder = [Cols]Kind{Ex, Triangle, Square, King}

// NewBoard returns a board in the standard Shong starting position: Black
// fills the back rank at y=0, White fills the back rank at y=Rows-1, both
// using columnOrder left to right.
func NewBoard() Board {
	var b Board
	for x := 0; x < Cols; x++ {
		b[homeRank(Black)][x] = &Piece{Kind: columnOrder[x], Side: Black}
		b[homeRank(White)][x] = &Piece{Kind: columnOrder[x], Side: White}
	}
	return b
}

// Move is a single move of one piece from From to To (both board
// coordinates); it says nothing about who moved or what it captures.
type Move struct {
	From, To image.Point
}

// movesFrom returns every legal move for the piece (if any) sitting at
// (x,y): for each of its current direction vectors, the exact distance
// required (1 if the piece has not yet moved, or is a King; 2 once it has
// moved and isn't a King), with every square strictly between the origin and
// that exact landing square required to be empty (no jumping), and the
// landing square itself required to be empty or occupied by an enemy piece
// (never by the mover's own side).
func (b *Board) movesFrom(x, y int) []Move {
	p := b[y][x]
	if p == nil {
		return nil
	}
	dist := 1
	if p.Kind != King && p.Moved {
		dist = 2
	}
	from := image.Pt(x, y)
	var moves []Move
	for _, d := range stepSet(p) {
		tx, ty := x, y
		blocked := false
		for step := 1; step < dist; step++ {
			tx += d.X
			ty += d.Y
			if !inBounds(tx, ty) || b[ty][tx] != nil {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		tx += d.X
		ty += d.Y
		if !inBounds(tx, ty) {
			continue
		}
		if target := b[ty][tx]; target != nil && target.Side == p.Side {
			continue
		}
		moves = append(moves, Move{From: from, To: image.Pt(tx, ty)})
	}
	return moves
}

// LegalMoves returns every legal move for side.
func (b *Board) LegalMoves(side Side) []Move {
	var moves []Move
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			if p := b[y][x]; p != nil && p.Side == side {
				moves = append(moves, b.movesFrom(x, y)...)
			}
		}
	}
	return moves
}

// DestinationsFrom returns every square the piece at p may legally move to
// (nil if p is empty or out of bounds).
func (b *Board) DestinationsFrom(p image.Point) []image.Point {
	if !inBounds(p.X, p.Y) {
		return nil
	}
	var out []image.Point
	for _, m := range b.movesFrom(p.X, p.Y) {
		out = append(out, m.To)
	}
	return out
}

// IsLegalMove reports whether side may legally play m.
func (b *Board) IsLegalMove(side Side, m Move) bool {
	if !inBounds(m.From.X, m.From.Y) || !inBounds(m.To.X, m.To.Y) {
		return false
	}
	p := b.At(m.From.X, m.From.Y)
	if p == nil || p.Side != side {
		return false
	}
	for _, mv := range b.movesFrom(m.From.X, m.From.Y) {
		if mv.To == m.To {
			return true
		}
	}
	return false
}

// Apply plays move m (assumed legal — callers should check IsLegalMove
// first) and returns the resulting board plus the piece captured by
// displacement (nil if the destination was empty). The board itself is a
// plain array of pointers, so nb := b makes an independent shallow copy;
// the moved piece is always replaced by a freshly allocated *Piece (never
// mutated in place) so that two independent copies of a board (as explored
// by the AI's search) never alias — and never corrupt — the same piece.
func (b Board) Apply(m Move) (Board, *Piece) {
	mover := b[m.From.Y][m.From.X]
	captured := b[m.To.Y][m.To.X]

	moved := &Piece{Kind: mover.Kind, Side: mover.Side, Moved: true, Ortho: mover.Ortho}
	if mover.Kind == King {
		moved.Ortho = !mover.Ortho
	}

	nb := b
	nb[m.From.Y][m.From.X] = nil
	nb[m.To.Y][m.To.X] = moved
	return nb, captured
}

// Count returns the number of side's pieces still on the board.
func (b *Board) Count(side Side) int {
	n := 0
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			if p := b[y][x]; p != nil && p.Side == side {
				n++
			}
		}
	}
	return n
}

// KingPos returns the position of side's King, and false if it has been
// captured.
func (b *Board) KingPos(side Side) (image.Point, bool) {
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			if p := b[y][x]; p != nil && p.Kind == King && p.Side == side {
				return image.Pt(x, y), true
			}
		}
	}
	return image.Point{}, false
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
