// Package game implements the pure rules engine for Hexa (based on the tile
// game "Six" by Steffen Spiele / Steffen Mühlhäuser): hex-tile placement and
// movement, connectivity, and the three winning-shape checks (line,
// triangle, hexagon-ring). It has no dependency on the inkview SDK and is
// fully unit-testable with a plain `go test`.
package game

// Hex is a cube coordinate (x,y,z with x+y+z=0) identifying one of the
// board's cells. Cube coordinates make the three axes of the hex grid
// trivial to reason about (see Directions/axisDirs below) and make a 60°
// rotation a simple linear map (see rotate60 in shapes.go) — exactly the
// same trick ringar/game/coords.go uses for YINSH's 85-point board. Six's
// board is simpler: a PLAIN full hexagon of radius Radius, with no corner
// truncation (unlike YINSH's 85-point board carved from a radius-5 hexagon).
type Hex struct {
	X, Y, Z int
}

// Add returns p shifted by direction/offset d.
func (p Hex) Add(d Hex) Hex {
	return Hex{p.X + d.X, p.Y + d.Y, p.Z + d.Z}
}

// Sub returns p shifted by the negation of d.
func (p Hex) Sub(d Hex) Hex {
	return Hex{p.X - d.X, p.Y - d.Y, p.Z - d.Z}
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

// Radius is the bounded v1 playing field's radius: all cube points with
// max(|x|,|y|,|z|) <= Radius are on the board. The original "Six" is played
// on an unbounded surface; per the spec, v1 ships a fixed large hexagonal
// field instead of an auto-fit/rescaling camera, on the reasoning that a
// real game (42 tiles total) essentially never reaches the edge of a
// generously-sized bounded field. Radius 6 gives 127 cells (3*6^2+3*6+1) —
// checked for legibility against real screenshots with 30+ tiles placed
// (see play_test.go's screenshot test and the task write-up); it held up
// fine, so this was kept rather than shrunk or grown.
const Radius = 6

// InBoard reports whether p lies within the bounded radius-Radius hexagon.
func InBoard(p Hex) bool {
	return p.X+p.Y+p.Z == 0 && maxi(absi(p.X), absi(p.Y), absi(p.Z)) <= Radius
}

// CubeRadius is p's hex-distance from the board's center (0,0,0).
func CubeRadius(p Hex) int {
	return maxi(absi(p.X), absi(p.Y), absi(p.Z))
}

// OnEdge reports whether p sits on the outer boundary ring of the bounded
// board (used to flag, honestly, when a real game has reached the edge of
// the v1 fixed field — see GameState.EdgeReached).
func OnEdge(p Hex) bool {
	return CubeRadius(p) == Radius
}

// Directions lists the 6 neighbour directions, grouped into 3 axis pairs:
// indices (0,1) are one axis, (2,3) the second, (4,5) the third. Each pair
// are exact opposites, and stepping along either axis leaves one cube
// coordinate unchanged.
var Directions = [6]Hex{
	{1, -1, 0}, {-1, 1, 0}, // axis A: Z constant
	{1, 0, -1}, {-1, 0, 1}, // axis B: Y constant
	{0, 1, -1}, {0, -1, 1}, // axis C: X constant
}

// axisDirs picks one canonical direction per axis (the other of each pair is
// just its negation) — used by the line-shape check, which only needs to
// walk each axis once per anchor since walking the opposite direction from
// a different anchor covers the same runs.
var axisDirs = [3]Hex{Directions[0], Directions[2], Directions[4]}

// Neighbors returns the (0-6) valid board points adjacent to p.
func Neighbors(p Hex) []Hex {
	var out []Hex
	for _, d := range Directions {
		q := p.Add(d)
		if InBoard(q) {
			out = append(out, q)
		}
	}
	return out
}

// AllPoints enumerates the board's cells, in a fixed deterministic order
// (ascending X, then ascending Y) so callers can rely on stable iteration
// (e.g. for deterministic AI tie-breaks and precomputed shape instances).
func AllPoints() []Hex {
	pts := make([]Hex, 0, 3*Radius*Radius+3*Radius+1)
	for x := -Radius; x <= Radius; x++ {
		for y := -Radius; y <= Radius; y++ {
			z := -x - y
			p := Hex{x, y, z}
			if InBoard(p) {
				pts = append(pts, p)
			}
		}
	}
	return pts
}

// LessHex orders two hexes for deterministic sorting (ascending X, then Y).
func LessHex(a, b Hex) bool {
	if a.X != b.X {
		return a.X < b.X
	}
	if a.Y != b.Y {
		return a.Y < b.Y
	}
	return a.Z < b.Z
}
