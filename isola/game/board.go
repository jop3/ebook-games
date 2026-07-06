// Package game implements the rules of Isola with no dependency on the
// inkview SDK, so it can be unit-tested cgo-free.
//
// The board is 8x8; every tile is present at the start. Each side has one
// pawn, starting on opposite sides of the board. A turn has two steps: (1)
// move your pawn any distance in a straight line in any of the 8 queen
// directions, stopping before (never onto or past) any missing tile or the
// opponent's pawn — no jumping over gaps or the opponent — then (2) remove
// any one tile from the board (any tile except the one your pawn is now
// standing on; removing the tile you just vacated is allowed). A side loses
// the instant it has zero legal pawn moves on its own turn.
package game

import "image"

// Side names a player color, and doubles as a "no side" sentinel (Empty) so
// PawnAt can report "nobody here" without a separate bool return.
type Side int

const (
	Empty Side = iota
	Black
	White
)

// Opponent returns the other player color. Only meaningful for Black/White.
func (s Side) Opponent() Side {
	switch s {
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

// queenDirs are the 8 queen-move directions (orthogonal + diagonal).
var queenDirs = [8]image.Point{
	{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1},
	{X: 1, Y: 1}, {X: 1, Y: -1}, {X: -1, Y: 1}, {X: -1, Y: -1},
}

func inBounds(x, y int) bool { return x >= 0 && x < Size && y >= 0 && y < Size }

// Board holds the 8x8 tile grid (present/missing) plus both pawns' positions.
type Board struct {
	Present   [Size][Size]bool
	BlackPawn image.Point
	WhitePawn image.Point
}

// NewBoard returns a board with every tile present and both pawns on their
// starting squares: Black at the bottom edge, White at the top edge, in
// point-symmetric squares near (but not exactly at, since 8 is even) the
// center of each edge.
func NewBoard() Board {
	var b Board
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			b.Present[y][x] = true
		}
	}
	b.BlackPawn = image.Pt(3, Size-1)
	b.WhitePawn = image.Pt(4, 0)
	return b
}

// IsPresent reports whether the tile at (x,y) is still on the board.
// Out-of-bounds coordinates report false.
func (b *Board) IsPresent(x, y int) bool {
	if !inBounds(x, y) {
		return false
	}
	return b.Present[y][x]
}

// PawnPos returns side's current pawn position.
func (b *Board) PawnPos(side Side) image.Point {
	if side == Black {
		return b.BlackPawn
	}
	return b.WhitePawn
}

func (b *Board) setPawnPos(side Side, p image.Point) {
	if side == Black {
		b.BlackPawn = p
	} else {
		b.WhitePawn = p
	}
}

// PawnAt reports which side's pawn (if any) sits at (x,y). Returns Empty if
// neither pawn is there.
func (b *Board) PawnAt(x, y int) Side {
	p := image.Pt(x, y)
	if b.BlackPawn == p {
		return Black
	}
	if b.WhitePawn == p {
		return White
	}
	return Empty
}

// DestinationsFrom returns every cell reachable from p by a queen move: a
// straight ray in each of the 8 directions, truncated (exclusive) at the
// first missing tile or occupied cell — no jumping over a gap or a pawn, and
// the stopping cell itself is never included. It does not check who (if
// anyone) occupies p.
func (b *Board) DestinationsFrom(p image.Point) []image.Point {
	var out []image.Point
	if !inBounds(p.X, p.Y) {
		return out
	}
	for _, d := range queenDirs {
		x, y := p.X+d.X, p.Y+d.Y
		for inBounds(x, y) && b.Present[y][x] && b.PawnAt(x, y) == Empty {
			out = append(out, image.Pt(x, y))
			x += d.X
			y += d.Y
		}
	}
	return out
}

// LegalMoves returns every legal destination for side's pawn.
func (b *Board) LegalMoves(side Side) []image.Point {
	return b.DestinationsFrom(b.PawnPos(side))
}

// IsLegalPawnMove reports whether side may legally move its pawn to "to".
func (b *Board) IsLegalPawnMove(side Side, to image.Point) bool {
	for _, d := range b.LegalMoves(side) {
		if d == to {
			return true
		}
	}
	return false
}

// LegalTileRemovals returns every tile that may legally be removed once the
// mover's pawn has landed on newPos: every present tile except newPos itself.
// Note this is independent of which side moved and does not exempt the
// opponent's pawn square (a present tile with the opponent's pawn standing on
// it may be removed — the tile vanishes out from under them, but since a
// pawn's own current square is never consulted when computing ITS destinations,
// this has no effect on their ability to move away next turn).
func (b *Board) LegalTileRemovals(newPos image.Point) []image.Point {
	var out []image.Point
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if !b.Present[y][x] {
				continue
			}
			if x == newPos.X && y == newPos.Y {
				continue
			}
			out = append(out, image.Pt(x, y))
		}
	}
	return out
}

// IsLegalRemoval reports whether removing "remove" is legal once the mover's
// pawn has landed on newPos.
func (b *Board) IsLegalRemoval(newPos, remove image.Point) bool {
	if remove == newPos {
		return false
	}
	if !inBounds(remove.X, remove.Y) {
		return false
	}
	return b.Present[remove.Y][remove.X]
}

// TotalPresent counts how many tiles remain on the board.
func (b *Board) TotalPresent() int {
	n := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.Present[y][x] {
				n++
			}
		}
	}
	return n
}

// Move is one full Isola turn: side's pawn moves From->To, then the tile at
// Remove is taken off the board. Callers should validate legality first (via
// IsLegalPawnMove/IsLegalRemoval) — Apply does not re-check.
type Move struct {
	Side     Side
	From, To image.Point
	Remove   image.Point
}

// Apply plays move m and returns the resulting board. Board is a small value
// type (like hasami's), so this is a plain copy-and-mutate.
func (b Board) Apply(m Move) Board {
	nb := b
	nb.setPawnPos(m.Side, m.To)
	nb.Present[m.Remove.Y][m.Remove.X] = false
	return nb
}
