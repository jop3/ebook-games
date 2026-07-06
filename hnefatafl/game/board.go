// Package game implements the rules of Hnefatafl in its Brandub variant (7x7
// board, 8 attackers vs. 4 defenders + a king) with no dependency on the
// inkview SDK, so it can be unit-tested cgo-free.
//
// # Starting layout (our own symmetric choice — documented here, not lifted
// # from a specific historical reconstruction; the spec only requires "a
// # symmetric layout" for the Brandub de-risk build)
//
// Board coordinates are (x,y), 0..6, with (0,0) the top-left corner.
//
//	. . . A . . .      y=0   two attackers flanking the top edge
//	. . . . . . .      y=1
//	. . . D . . .      y=2   one defender north of the throne
//	A . D K D . A      y=3   king on the throne, defenders east/west, attackers on both side edges
//	. . . D . . .      y=4   one defender south of the throne
//	. . . . . . .      y=5
//	. . . A . . .      y=6   two attackers flanking the bottom edge
//
// Attackers (8): (2,0) (4,0) (2,6) (4,6) (0,2) (0,4) (6,2) (6,4) — two per
// edge, flanking the mid-point of each side, mirroring the "attackers near
// the 4 edge-midpoints" guidance in the spec.
// Defenders (4): (2,3) (4,3) (3,2) (3,4) — the four cells orthogonally
// adjacent to the throne.
// King (1): (3,3), the throne, the board's center on a 7x7 grid.
//
// # Special squares
//
// The center throne and the 4 corners are restricted terrain: only the king
// may ever stop there, and (the simplest, safest reading per the spec) no
// other piece may even pass through them — they behave like a permanent wall
// for attackers and ordinary defenders alike. The king itself may stop on
// (but, in this implementation, not continue moving past, in the same move)
// the throne or a corner.
//
// All pieces move like a rook: any distance orthogonally, blocked by other
// pieces and by the restricted squares (per above).
package game

import "image"

// Cell is the contents of one board square.
type Cell uint8

const (
	Empty Cell = iota
	Attacker
	Defender
	King
)

// Side names which army is to move: the attacking horde, or the defending
// side (which includes both the ordinary defenders and the king).
type Side uint8

const (
	SideAttacker Side = iota
	SideDefender
)

// Opponent returns the other side.
func (s Side) Opponent() Side {
	if s == SideAttacker {
		return SideDefender
	}
	return SideAttacker
}

// Owner returns which side a board cell belongs to. Empty has no meaningful
// owner (returns SideAttacker as an arbitrary zero value — callers must check
// for Empty separately before relying on Owner).
func Owner(c Cell) Side {
	if c == Attacker {
		return SideAttacker
	}
	return SideDefender // Defender or King
}

// Size is the edge length of the Brandub board.
const Size = 7

// StartAttackers and StartDefenders are the piece counts at the start of a
// game (the king is separate, always exactly 1 while alive), used by the AI's
// material terms.
const (
	StartAttackers = 8
	StartDefenders = 4
)

// Board holds the 7x7 grid, indexed [y][x] (row-major, y=0 is the top row).
type Board [Size][Size]Cell

// throneCell is the single center throne square.
var throneCell = image.Pt(Size/2, Size/2)

// cornerCells lists the board's four corner squares.
var cornerCells = [4]image.Point{
	{X: 0, Y: 0}, {X: Size - 1, Y: 0}, {X: 0, Y: Size - 1}, {X: Size - 1, Y: Size - 1},
}

// IsThrone reports whether (x,y) is the center throne square.
func IsThrone(x, y int) bool { return x == throneCell.X && y == throneCell.Y }

// IsCorner reports whether (x,y) is one of the four corner squares.
func IsCorner(x, y int) bool {
	for _, c := range cornerCells {
		if c.X == x && c.Y == y {
			return true
		}
	}
	return false
}

// IsRestricted reports whether (x,y) is the throne or a corner — squares only
// the king may ever occupy.
func IsRestricted(x, y int) bool { return IsThrone(x, y) || IsCorner(x, y) }

// Corners returns the four corner cells (a defensive copy).
func Corners() [4]image.Point { return cornerCells }

// Throne returns the throne cell.
func Throne() image.Point { return throneCell }

var dirs4 = [4]image.Point{{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}}

func inBounds(x, y int) bool { return x >= 0 && x < Size && y >= 0 && y < Size }

// At returns the cell at (x,y). Out-of-bounds coordinates return Empty.
func (b *Board) At(x, y int) Cell {
	if !inBounds(x, y) {
		return Empty
	}
	return b[y][x]
}

func (b *Board) set(x, y int, c Cell) { b[y][x] = c }

// NewBoard returns a board in the Brandub starting position documented above.
func NewBoard() Board {
	var b Board
	attackers := []image.Point{
		{X: 2, Y: 0}, {X: 4, Y: 0},
		{X: 2, Y: Size - 1}, {X: 4, Y: Size - 1},
		{X: 0, Y: 2}, {X: 0, Y: 4},
		{X: Size - 1, Y: 2}, {X: Size - 1, Y: 4},
	}
	for _, p := range attackers {
		b.set(p.X, p.Y, Attacker)
	}
	defenders := []image.Point{
		{X: throneCell.X - 1, Y: throneCell.Y}, {X: throneCell.X + 1, Y: throneCell.Y},
		{X: throneCell.X, Y: throneCell.Y - 1}, {X: throneCell.X, Y: throneCell.Y + 1},
	}
	for _, p := range defenders {
		b.set(p.X, p.Y, Defender)
	}
	b.set(throneCell.X, throneCell.Y, King)
	return b
}

// Count returns the number of cells holding the given piece type.
func (b *Board) Count(c Cell) int {
	n := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b[y][x] == c {
				n++
			}
		}
	}
	return n
}

// KingPos returns the king's current position, or ok=false if the king has
// been captured and removed from the board.
func (b *Board) KingPos() (image.Point, bool) {
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b[y][x] == King {
				return image.Pt(x, y), true
			}
		}
	}
	return image.Point{}, false
}

// DefenderSideCount returns the total number of defender-side pieces alive:
// ordinary defenders plus the king (0 or 1).
func (b *Board) DefenderSideCount() int {
	n := b.Count(Defender)
	if _, ok := b.KingPos(); ok {
		n++
	}
	return n
}

// Move is a single rook move of one piece from From to To.
type Move struct {
	From, To image.Point
}

// DestinationsFrom returns every square reachable by a rook move from p, for
// a piece that is (isKing) or is not the king. Non-king pieces cannot stop on
// or pass through a restricted square (throne/corner) — the ray simply stops
// there without adding it. The king may stop on a restricted square, but (a
// deliberate simplification, since a single move already lets the king reach
// any restricted square in its own row/column) does not continue past it in
// the same move.
func (b *Board) DestinationsFrom(p image.Point, isKing bool) []image.Point {
	var out []image.Point
	if !inBounds(p.X, p.Y) {
		return out
	}
	for _, d := range dirs4 {
		x, y := p.X+d.X, p.Y+d.Y
		for inBounds(x, y) {
			if b.At(x, y) != Empty {
				break
			}
			restricted := IsRestricted(x, y)
			if restricted && !isKing {
				break // impassable terrain for non-king pieces: blocks stopping AND passing through
			}
			out = append(out, image.Pt(x, y))
			if restricted {
				break // king may land here, but a single move doesn't continue past it
			}
			x += d.X
			y += d.Y
		}
	}
	return out
}

// LegalMoves returns every legal move for side: a rook move from each of
// side's pieces (attackers if side==SideAttacker; defenders AND the king if
// side==SideDefender) to each reachable square.
func (b *Board) LegalMoves(side Side) []Move {
	var moves []Move
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			c := b.At(x, y)
			if c == Empty || Owner(c) != side {
				continue
			}
			from := image.Pt(x, y)
			for _, to := range b.DestinationsFrom(from, c == King) {
				moves = append(moves, Move{From: from, To: to})
			}
		}
	}
	return moves
}

// IsLegalMove reports whether side may legally play m.
func (b *Board) IsLegalMove(side Side, m Move) bool {
	if !inBounds(m.From.X, m.From.Y) || !inBounds(m.To.X, m.To.Y) {
		return false
	}
	piece := b.At(m.From.X, m.From.Y)
	if piece == Empty || Owner(piece) != side {
		return false
	}
	if m.From == m.To {
		return false
	}
	if m.From.X != m.To.X && m.From.Y != m.To.Y {
		return false // not a straight line
	}
	if b.At(m.To.X, m.To.Y) != Empty {
		return false
	}
	isKing := piece == King
	if !isKing && IsRestricted(m.To.X, m.To.Y) {
		return false
	}
	dx, dy := sign(m.To.X-m.From.X), sign(m.To.Y-m.From.Y)
	x, y := m.From.X+dx, m.From.Y+dy
	for x != m.To.X || y != m.To.Y {
		if b.At(x, y) != Empty {
			return false // blocked; a piece cannot jump over another
		}
		if IsRestricted(x, y) {
			// Restricted squares block passage for everyone mid-path: a
			// non-king piece can never enter one, and even the king only
			// ever lands on one as a final destination, never passes
			// through it to reach a square beyond (see DestinationsFrom).
			return false
		}
		x += dx
		y += dy
	}
	return true
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	default:
		return 0
	}
}
