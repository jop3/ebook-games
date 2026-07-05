// Package game implements the rules of Go (baduk/weiqi) with no dependency on
// the inkview SDK, so it can be unit-tested cgo-free.
//
// The board is square and size-parameterised (9x9, 13x13, or 19x19). Players
// alternate placing a stone on an empty intersection, Black first. Placing a
// stone first removes any enemy group left with no liberties, then rejects
// the move as an illegal suicide if the mover's own group is left with no
// liberties. A simple positional ko rule forbids immediately recreating the
// board position that existed before the opponent's last move. Two
// consecutive passes end the game; scoring is area (Chinese) scoring after an
// end-of-game "mark dead stones" phase.
package game

import "image"

// Color is one of the three states of a board intersection.
type Color uint8

const (
	Empty Color = iota
	Black
	White
)

// Opponent returns the other player color. Only meaningful for Black/White.
func (c Color) Opponent() Color {
	switch c {
	case Black:
		return White
	case White:
		return Black
	default:
		return Empty
	}
}

// Board holds a square grid of intersections, indexed [y][x] (row-major,
// y=0 is the top row), matching the house convention used by the other
// board games in this repo. Its size is whatever len(Board) is — 9, 13, or
// 19 — so a single type serves all three board sizes.
type Board [][]Color

// NewBoard returns an empty size x size board.
func NewBoard(size int) Board {
	b := make(Board, size)
	for y := range b {
		b[y] = make([]Color, size)
	}
	return b
}

// Size returns the board's edge length.
func (b Board) Size() int { return len(b) }

// InBounds reports whether p lies on the board.
func (b Board) InBounds(p image.Point) bool {
	n := len(b)
	return p.X >= 0 && p.X < n && p.Y >= 0 && p.Y < n
}

// At returns the color at p. Out-of-bounds points read as Empty.
func (b Board) At(p image.Point) Color {
	if !b.InBounds(p) {
		return Empty
	}
	return b[p.Y][p.X]
}

// Set writes the color at p. p must be in bounds.
func (b Board) Set(p image.Point, c Color) { b[p.Y][p.X] = c }

// Clone returns a deep copy of b.
func (b Board) Clone() Board {
	nb := make(Board, len(b))
	for y, row := range b {
		nb[y] = append([]Color(nil), row...)
	}
	return nb
}

// Equal reports whether a and b are the same size and hold identical stones.
func Equal(a, b Board) bool {
	if len(a) != len(b) {
		return false
	}
	for y := range a {
		if len(a[y]) != len(b[y]) {
			return false
		}
		for x := range a[y] {
			if a[y][x] != b[y][x] {
				return false
			}
		}
	}
	return true
}

// Count returns the number of intersections holding color c.
func (b Board) Count(c Color) int {
	n := 0
	for _, row := range b {
		for _, v := range row {
			if v == c {
				n++
			}
		}
	}
	return n
}

// orthoDirs are the four orthogonal neighbor offsets.
var orthoDirs = [4]image.Point{{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}}

// Neighbors returns the (up to 4) in-bounds orthogonal neighbors of p.
func (b Board) Neighbors(p image.Point) []image.Point {
	var out []image.Point
	for _, d := range orthoDirs {
		q := p.Add(d)
		if b.InBounds(q) {
			out = append(out, q)
		}
	}
	return out
}

// HoshiPoints returns the conventional star-point markers for a board of this
// size (9x9: the four 3-3 points plus tengen; 13x13: the four 3-3 points plus
// tengen; 19x19: the full 3-3/3-9/3-15 nine-point grid). Unrecognized sizes
// return nil (no markers drawn).
func HoshiPoints(size int) []image.Point {
	switch size {
	case 9:
		return starGrid(size, []int{2, 6}, true)
	case 13:
		return starGrid(size, []int{3, 9}, true)
	case 19:
		return starGrid(size, []int{3, 9, 15}, false)
	default:
		return nil
	}
}

// starGrid builds the cross product of coords x coords, plus the board
// center (tengen) when includeCenter is true and it isn't already listed
// (true center-inclusion for 19x19 happens naturally since 9 is one of the
// coords).
func starGrid(size int, coords []int, includeCenter bool) []image.Point {
	var pts []image.Point
	for _, y := range coords {
		for _, x := range coords {
			pts = append(pts, image.Pt(x, y))
		}
	}
	if includeCenter {
		c := size / 2
		pts = append(pts, image.Pt(c, c))
	}
	return pts
}
