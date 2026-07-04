// Package game implements the pure rules engine for Ringar (based on YINSH):
// board geometry, ring movement, marker flipping, row detection, ring
// removal and a small alpha-beta AI. It has no dependency on the inkview SDK
// and is fully unit-testable with a plain `go test`.
package game

// Point is a cube coordinate (x,y,z with x+y+z=0) identifying one of the
// board's 85 intersections. Cube coordinates make the three axes of the
// triangular grid trivial to reason about: moving along a "line" always
// holds exactly one of the three coordinates constant (see Axis below), and
// the six neighbour directions are just the six sign-balanced unit vectors.
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

// hexRadius is the "radius" of the full hexagon (before corner-truncation)
// that the 85-point YINSH board is carved from: all cube points with
// max(|x|,|y|,|z|) <= hexRadius form a 91-point hexagon (3*R^2+3*R+1 for
// R=5), and removing its 6 true corners (where two of the three absolute
// coordinates simultaneously hit R) leaves exactly the 85 points of the real
// board. See coords_test.go for the enumeration that proves this.
const hexRadius = 5

// InHex reports whether p lies within the radius-5 hexagon (ignoring the
// corner truncation).
func InHex(p Point) bool {
	return p.X+p.Y+p.Z == 0 && maxi(absi(p.X), absi(p.Y), absi(p.Z)) <= hexRadius
}

// isCorner reports whether p is one of the 6 true corners of the radius-5
// hexagon: the points where two of the three absolute coordinates are
// simultaneously at the maximum radius (e.g. (5,-5,0)). YINSH's board is
// this hexagon with those 6 single points snipped off.
func isCorner(p Point) bool {
	n := 0
	if absi(p.X) == hexRadius {
		n++
	}
	if absi(p.Y) == hexRadius {
		n++
	}
	if absi(p.Z) == hexRadius {
		n++
	}
	return n >= 2
}

// Valid reports whether p is one of the board's 85 intersections.
func Valid(p Point) bool {
	return InHex(p) && !isCorner(p)
}

// Directions lists the 6 neighbour directions, grouped into 3 axis pairs:
// indices (0,1) are one axis, (2,3) the second, (4,5) the third. Each pair
// are exact opposites, and stepping along either axis leaves one cube
// coordinate unchanged (see Axis).
var Directions = [6]Point{
	{1, -1, 0}, {-1, 1, 0}, // axis A: Z constant
	{1, 0, -1}, {-1, 0, 1}, // axis B: Y constant
	{0, 1, -1}, {0, -1, 1}, // axis C: X constant
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

// AllPoints enumerates the board's 85 intersections, in a fixed deterministic
// order (ascending X, then ascending Y) so callers can rely on stable
// iteration (e.g. for deterministic AI placement tie-breaks).
func AllPoints() []Point {
	pts := make([]Point, 0, 85)
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

// Axis identifies one of the board's 3 line directions (families of
// straight lines used both for ring-move rays, grouped in pairs by
// Directions, and for row detection).
type Axis int

const (
	AxisA Axis = iota // lines of constant Z
	AxisB             // lines of constant Y
	AxisC             // lines of constant X
)

// axisKey and axisOrder return, for a point and an axis, the coordinate that
// is invariant along that axis's lines and the coordinate that varies
// (ascending) along them.
func axisKey(p Point, axis Axis) int {
	switch axis {
	case AxisA:
		return p.Z
	case AxisB:
		return p.Y
	default:
		return p.X
	}
}

func axisOrder(p Point, axis Axis) int {
	switch axis {
	case AxisA:
		return p.X
	case AxisB:
		return p.X
	default:
		return p.Y
	}
}

// axisLines caches, per axis, the board's maximal straight lines (each a
// contiguous run of points sorted by axisOrder). Computed once since the
// board shape never changes.
var axisLines [3][][]Point

func init() {
	pts := AllPoints()
	for axis := AxisA; axis <= AxisC; axis++ {
		groups := map[int][]Point{}
		var keys []int
		for _, p := range pts {
			k := axisKey(p, axis)
			if _, ok := groups[k]; !ok {
				keys = append(keys, k)
			}
			groups[k] = append(groups[k], p)
		}
		// Sort keys for determinism, and sort each group by its order coord.
		sortInts(keys)
		lines := make([][]Point, 0, len(keys))
		for _, k := range keys {
			g := groups[k]
			sortPointsByOrder(g, axis)
			lines = append(lines, g)
		}
		axisLines[axis] = lines
	}
}

func sortInts(s []int) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

func sortPointsByOrder(pts []Point, axis Axis) {
	for i := 1; i < len(pts); i++ {
		for j := i; j > 0 && axisOrder(pts[j-1], axis) > axisOrder(pts[j], axis); j-- {
			pts[j-1], pts[j] = pts[j], pts[j-1]
		}
	}
}

// Lines returns the board's maximal straight lines along the given axis
// (each a contiguous run of adjacent points, sorted along the line).
func Lines(axis Axis) [][]Point {
	return axisLines[axis]
}
