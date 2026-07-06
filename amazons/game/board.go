// Package game implements the rules of Amazons with no dependency on the
// inkview SDK, so it can be unit-tested cgo-free.
//
// The board is 10x10. Each side starts with 4 queens at the standard
// (Zamkauskas 1988) symmetric starting squares. A turn has two phases: first
// move one of your queens like a chess queen (any distance, horizontally,
// vertically or diagonally, onto an empty square, never jumping over an
// occupied or burned square); then, from that new square, shoot an arrow —
// also a queen-line move — onto another empty square, which becomes
// permanently burned (impassable to every future queen move and arrow, for
// both sides, for the rest of the game). There are no captures. The side to
// move loses the instant it has no queen with any legal move at all (having
// a legal move always implies a legal follow-up shot — see SideHasMove).
package game

import "image"

// Cell is one of the four states of a board square.
type Cell uint8

const (
	Empty Cell = iota
	Burned
	QueenBlack
	QueenWhite
)

// Side names a player color (Black or White). It is a distinct type from
// Cell because a board square can also be Empty or Burned, states that are
// never meaningful as a "whose turn" value.
type Side uint8

const (
	Black Side = iota
	White
)

// Opponent returns the other side.
func (s Side) Opponent() Side {
	if s == Black {
		return White
	}
	return Black
}

// Queen returns the board Cell value representing one of this side's queens.
func (s Side) Queen() Cell {
	if s == Black {
		return QueenBlack
	}
	return QueenWhite
}

// SideOf returns the Side that owns a queen cell, and false for Empty/Burned.
func SideOf(c Cell) (Side, bool) {
	switch c {
	case QueenBlack:
		return Black, true
	case QueenWhite:
		return White, true
	default:
		return Black, false
	}
}

// Size is the edge length of the board.
const Size = 10

// Board holds the 10x10 grid, indexed [y][x] (row-major, y=0 is the top row).
type Board [Size][Size]Cell

// dirs8 are the eight queen directions: the 4 rook directions plus the 4
// bishop diagonals.
var dirs8 = [8]image.Point{
	{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1},
	{X: 1, Y: 1}, {X: 1, Y: -1}, {X: -1, Y: 1}, {X: -1, Y: -1},
}

func inBounds(x, y int) bool { return x >= 0 && x < Size && y >= 0 && y < Size }

// At returns the cell at (x,y). Out-of-bounds coordinates return Empty.
func (b *Board) At(x, y int) Cell {
	if !inBounds(x, y) {
		return Empty
	}
	return b[y][x]
}

func (b *Board) set(x, y int, c Cell) { b[y][x] = c }

// NewBoard returns a board in the standard Amazons starting position (the
// symmetric layout in use since Zamkauskas's original 1988 game): each side
// has 4 queens, two on its own home rank (columns 3 and 6) and two further
// out on the 4th rank from home (columns 0 and 9). Black's home rank is the
// top row (y=0); White's is the bottom row (y=Size-1) — mirroring this
// repo's Hasami convention of "Black moves first, White is the far side",
// even though the traditional Amazons literature usually calls the near
// side White. The exact squares (using this file's (x,y), y=0 at the top):
//
//	Black: (3,0) (6,0)  — home rank
//	       (0,3) (9,3)  — outer rank
//	White: (3,9) (6,9)  — home rank
//	       (0,6) (9,6)  — outer rank
//
// This is precisely the traditional start reflected top-to-bottom (in
// standard notation d1/g1/a4/j4 for one side and d10/g10/a7/j7 for the
// other), not an invented layout.
func NewBoard() Board {
	var b Board
	b.set(3, 0, QueenBlack)
	b.set(6, 0, QueenBlack)
	b.set(0, 3, QueenBlack)
	b.set(9, 3, QueenBlack)

	b.set(3, Size-1, QueenWhite)
	b.set(6, Size-1, QueenWhite)
	b.set(0, 6, QueenWhite)
	b.set(9, 6, QueenWhite)
	return b
}

// QueenPositions returns the board coordinates of every queen belonging to
// side.
func (b *Board) QueenPositions(side Side) []image.Point {
	var out []image.Point
	q := side.Queen()
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b[y][x] == q {
				out = append(out, image.Pt(x, y))
			}
		}
	}
	return out
}

// CountBurned returns the number of permanently burned squares.
func (b *Board) CountBurned() int {
	n := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b[y][x] == Burned {
				n++
			}
		}
	}
	return n
}

// isQueenLine reports whether b is reachable from a via a straight rook or
// bishop ray (horizontal, vertical, or exact diagonal).
func isQueenLine(a, c image.Point) bool {
	dx, dy := c.X-a.X, c.Y-a.Y
	if dx == 0 && dy == 0 {
		return false
	}
	if dx == 0 || dy == 0 {
		return true
	}
	if dx == dy || dx == -dy {
		return true
	}
	return false
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

// rayClear reports whether there is a clear, unobstructed queen-line path
// from "from" to "to": a straight rook/bishop line, every intermediate
// square Empty, and "to" itself Empty. It says nothing about what (if
// anything) occupies "from" — that's checked separately by the callers,
// since the same ray-walk is used both for moving a queen (which must stand
// on "from") and for shooting an arrow (same requirement, checked by the
// caller against whichever queen just moved there).
func (b *Board) rayClear(from, to image.Point) bool {
	if !inBounds(from.X, from.Y) || !inBounds(to.X, to.Y) {
		return false
	}
	if !isQueenLine(from, to) {
		return false
	}
	dx, dy := sign(to.X-from.X), sign(to.Y-from.Y)
	x, y := from.X+dx, from.Y+dy
	for x != to.X || y != to.Y {
		if b.At(x, y) != Empty {
			return false // blocked by a queen or a burned square
		}
		x += dx
		y += dy
	}
	return b.At(to.X, to.Y) == Empty
}

// DestinationsFrom returns every empty square reachable by a queen-line ray
// (all 8 directions) from p, blocked by any occupied-or-burned square. It
// says nothing about what (if anything) occupies p itself — the same
// generator is used both for "where can this queen move" and "where can the
// queen that just landed here shoot", the two phases of a turn.
func (b *Board) DestinationsFrom(p image.Point) []image.Point {
	var out []image.Point
	if !inBounds(p.X, p.Y) {
		return out
	}
	for _, d := range dirs8 {
		cx, cy := p.X+d.X, p.Y+d.Y
		for inBounds(cx, cy) && b.At(cx, cy) == Empty {
			out = append(out, image.Pt(cx, cy))
			cx += d.X
			cy += d.Y
		}
	}
	return out
}

// QueenMove is one queen's move from From to To; it says nothing about who
// moved.
type QueenMove struct {
	From, To image.Point
}

// IsLegalQueenMove reports whether side may legally move a queen as m: m.From
// must hold one of side's queens, and it must reach m.To via a clear
// queen-line ray.
func (b *Board) IsLegalQueenMove(side Side, m QueenMove) bool {
	if b.At(m.From.X, m.From.Y) != side.Queen() {
		return false
	}
	return b.rayClear(m.From, m.To)
}

// IsLegalShot reports whether the queen presently standing at "from" (either
// side — the arrow always comes from wherever the mover's queen just landed)
// may shoot an arrow to "to" via a clear queen-line ray. Note that "from"'s
// own square became empty when its previous occupant moved there in the same
// turn, so a shot back along the very line the queen just traveled — even
// through the square it started this turn from — is completely legal, since
// that square is now Empty like any other.
func (b *Board) IsLegalShot(from, to image.Point) bool {
	if _, ok := SideOf(b.At(from.X, from.Y)); !ok {
		return false
	}
	return b.rayClear(from, to)
}

// MoveQueen returns a new board with the queen at m.From relocated to m.To.
// It does not check legality; callers must call IsLegalQueenMove first.
func (b Board) MoveQueen(m QueenMove) Board {
	nb := b
	c := nb.At(m.From.X, m.From.Y)
	nb.set(m.From.X, m.From.Y, Empty)
	nb.set(m.To.X, m.To.Y, c)
	return nb
}

// Shoot returns a new board with square "at" permanently Burned. It does not
// check legality; callers must call IsLegalShot first.
func (b Board) Shoot(at image.Point) Board {
	nb := b
	nb.set(at.X, at.Y, Burned)
	return nb
}

// SideHasMove reports whether side has at least one legal queen move
// anywhere on the board. This is exactly the condition for side to have a
// legal turn at all: any queen move m always has at least one legal
// follow-up shot, namely straight back to the square the queen just vacated
// (m.From is Empty again the instant the queen leaves it, and the ray back
// to it — through no other square — is by construction unobstructed). So a
// side that can move a queen at all can always complete a full
// move-then-shoot turn, and a side that cannot move any queen has, by
// definition, no legal turn — which is exactly this game's losing
// condition. (Board.LegalTurns / Winner rely on this fact; TestSideHasMove*
// checks it holds even in the tightest boxed-in positions.)
func (b *Board) SideHasMove(side Side) bool {
	for _, p := range b.QueenPositions(side) {
		if len(b.DestinationsFrom(p)) > 0 {
			return true
		}
	}
	return false
}

// LegalQueenMoves returns every legal queen move for side (not full turns —
// see LegalTurns for the full move+shot pairs).
func (b *Board) LegalQueenMoves(side Side) []QueenMove {
	var out []QueenMove
	for _, p := range b.QueenPositions(side) {
		for _, to := range b.DestinationsFrom(p) {
			out = append(out, QueenMove{From: p, To: to})
		}
	}
	return out
}

// Turn is one complete move: relocate a queen, then shoot an arrow from its
// new square.
type Turn struct {
	Move QueenMove
	Shot image.Point
}

// LegalTurns exhaustively enumerates every legal full turn (move, then
// shoot) for side. This is combinatorially large at midgame (low thousands)
// — fine for tests and small positions, but the AI does NOT use this for a
// full-width search; see ai.go.
func (b *Board) LegalTurns(side Side) []Turn {
	var out []Turn
	for _, p := range b.QueenPositions(side) {
		for _, to := range b.DestinationsFrom(p) {
			m := QueenMove{From: p, To: to}
			nb := b.MoveQueen(m)
			for _, shot := range nb.DestinationsFrom(to) {
				out = append(out, Turn{Move: m, Shot: shot})
			}
		}
	}
	return out
}

// Apply returns the board that results from playing t for side. It does not
// check legality.
func (b Board) Apply(t Turn) Board {
	return b.MoveQueen(t.Move).Shoot(t.Shot)
}
