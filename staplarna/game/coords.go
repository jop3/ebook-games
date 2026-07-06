// Package game implements the pure rules engine for Staplarna (based on
// TZAAR, Kris Burm / the GIPF project): hex board geometry, setup-phase
// placement of each side's 30 pieces, stack movement and capture, win
// detection, and a small alpha-beta AI. It has no dependency on the inkview
// SDK and is fully unit-testable with a plain `go test`.
package game

// Point is a cube coordinate (x,y,z with x+y+z=0) identifying one of the
// board's 61 cells.
//
// GEOMETRY REUSE: this coordinate scheme, Add, and the 6 Directions below
// are copied (with one change, noted below) from ringar/game/coords.go —
// this repo's YINSH port, which already solved hex adjacency/geometry for a
// PocketBook e-ink board. Rather than reimplement hex coordinates from
// scratch, this file keeps ringar's cube-coordinate Point type and
// Directions UNCHANGED, since TZAAR needs exactly the same "straight line
// along one of 3 axes, 6 neighbour directions" hex geometry YINSH does.
//
// The one real difference is board SHAPE: YINSH's board is a radius-5
// hexagon (91 cube points) with its 6 true corners truncated away, leaving
// 85 points (see ringar's isCorner/Valid). TZAAR's board has no such
// truncation — it is a PLAIN hexagon of hexagonal cells, 5 cells per edge,
// 61 cells total. Under the standard cube-coordinate hexagon formula
// (3*R^2 + 3*R + 1 points for all cube points with max(|x|,|y|,|z|) <= R),
// R=4 gives 3*16+12+1 = 61 — so this board is exactly ringar's hexRadius-5
// shape one ring smaller, MINUS the corner-truncation step (there is nothing
// to truncate; the full hexagon at R=4 already has 61 cells with no snipped
// corners). "Radius 5" in the spec's shorthand refers to the board's 5
// cells-per-edge, not this file's cube-coordinate R — the two numbering
// conventions differ by one, which is why hexRadius below is 4, not 5.
type Point struct {
	X, Y, Z int
}

// Add returns p shifted by direction d.
func (p Point) Add(d Point) Point {
	return Point{p.X + d.X, p.Y + d.Y, p.Z + d.Z}
}

func absi(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func maxi(a, b, c int) int {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}

// hexRadius is the cube-coordinate radius of Staplarna's board (see the
// package doc above for why this is 4, not 5, despite the board having 5
// cells per edge): all cube points with max(|x|,|y|,|z|) <= hexRadius form
// the board, 3*hexRadius^2 + 3*hexRadius + 1 = 61 cells for hexRadius = 4.
// See coords_test.go for the enumeration confirming this count.
const hexRadius = 4

// Valid reports whether p is one of the board's 61 cells. Unlike ringar's
// YINSH board, TZAAR's board is the full hexagon with no corner truncation,
// so this is exactly the radius check (no isCorner step).
func Valid(p Point) bool {
	return p.X+p.Y+p.Z == 0 && maxi(absi(p.X), absi(p.Y), absi(p.Z)) <= hexRadius
}

// Directions lists the 6 neighbour directions, identical to ringar's: pairs
// (0,1), (2,3), (4,5) are each exact opposites, one per axis.
var Directions = [6]Point{
	{1, -1, 0}, {-1, 1, 0}, // axis A
	{1, 0, -1}, {-1, 0, 1}, // axis B
	{0, 1, -1}, {0, -1, 1}, // axis C
}

// Neighbors returns the (0-6) valid board points adjacent to p.
func Neighbors(p Point) []Point {
	var out []Point
	for _, d := range Directions {
		q := p.Add(d)
		if Valid(q) {
			out = append(out, q)
		}
	}
	return out
}

// AllPoints enumerates the board's 61 cells, in a fixed deterministic order
// (ascending X, then ascending Y) so callers can rely on stable iteration.
func AllPoints() []Point {
	pts := make([]Point, 0, 61)
	for x := -hexRadius; x <= hexRadius; x++ {
		for y := -hexRadius; y <= hexRadius; y++ {
			z := -x - y
			p := Point{x, y, z}
			if Valid(p) {
				pts = append(pts, p)
			}
		}
	}
	return pts
}

// Distance returns the hex (cube) distance between two points — the number
// of single-step moves needed to walk from a to b along the grid.
func Distance(a, b Point) int {
	return (absi(a.X-b.X) + absi(a.Y-b.Y) + absi(a.Z-b.Z)) / 2
}
