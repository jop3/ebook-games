package game

import (
	"sort"
	"testing"
)

func sortedHexes(hs []Hex) []Hex {
	out := make([]Hex, len(hs))
	copy(out, hs)
	sort.Slice(out, func(i, j int) bool { return LessHex(out[i], out[j]) })
	return out
}

func hexSetEqual(a, b []Hex) bool {
	if len(a) != len(b) {
		return false
	}
	sa, sb := sortedHexes(a), sortedHexes(b)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}

// --- Linje (line): all 3 axes, plus a 5-tile near-miss ----------------------

func TestHasShapeLineAllThreeAxes(t *testing.T) {
	cases := []struct {
		name  string
		cells []Hex
	}{
		{"axis A", []Hex{{-3, 3, 0}, {-2, 2, 0}, {-1, 1, 0}, {0, 0, 0}, {1, -1, 0}, {2, -2, 0}}},
		{"axis B", []Hex{{-3, 0, 3}, {-2, 0, 2}, {-1, 0, 1}, {0, 0, 0}, {1, 0, -1}, {2, 0, -2}}},
		{"axis C", []Hex{{0, -3, 3}, {0, -2, 2}, {0, -1, 1}, {0, 0, 0}, {0, 1, -1}, {0, 2, -2}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tiles := map[Hex]Side{}
			for _, p := range c.cells {
				tiles[p] = Black
			}
			kind, cells := HasShape(tiles, Black)
			if kind != ShapeLine {
				t.Fatalf("%s: expected ShapeLine, got %v", c.name, kind)
			}
			if !hexSetEqual(cells, c.cells) {
				t.Fatalf("%s: winning cells = %v, want %v", c.name, cells, c.cells)
			}
		})
	}
}

// GOTCHA: a 5-tile run (one short) must NOT win, for every axis.
func TestHasShapeLineNearMissDoesNotWin(t *testing.T) {
	full := []Hex{{-3, 3, 0}, {-2, 2, 0}, {-1, 1, 0}, {0, 0, 0}, {1, -1, 0}, {2, -2, 0}}
	tiles := map[Hex]Side{}
	for _, p := range full[:5] { // only 5 of the 6
		tiles[p] = Black
	}
	if kind, cells := HasShape(tiles, Black); kind != ShapeNone {
		t.Fatalf("a 5-of-6 line must not win, got %v cells=%v", kind, cells)
	}
}

// A line requires ALL 6 of the SAME side — an opponent tile anywhere in the
// run must break it.
func TestHasShapeLineBlockedByOpponent(t *testing.T) {
	full := []Hex{{-3, 3, 0}, {-2, 2, 0}, {-1, 1, 0}, {0, 0, 0}, {1, -1, 0}, {2, -2, 0}}
	tiles := map[Hex]Side{}
	for _, p := range full {
		tiles[p] = Black
	}
	tiles[full[2]] = White // break the run
	if kind, _ := HasShape(tiles, Black); kind != ShapeNone {
		t.Fatalf("an opponent tile inside the run must prevent the line win, got %v", kind)
	}
}

// --- Triangel: all 6 rotations, plus a 5-tile near-miss ---------------------

// triangleOrientationCells returns the 6 rotations of triangleTemplate as
// concrete cell sets (all anchored so that the template's own (0,0,0) point
// — a fixed point of rotate60 — stays at the board's center, so every
// rotation is guaranteed to stay in-board regardless of Radius).
func triangleOrientationCells(t *testing.T) [6][]Hex {
	t.Helper()
	var out [6][]Hex
	cur := triangleTemplate
	for r := 0; r < 6; r++ {
		cells := make([]Hex, 6)
		copy(cells, cur[:])
		out[r] = cells
		var next [6]Hex
		for i, p := range cur {
			next[i] = rotate60(p)
		}
		cur = next
	}
	return out
}

func TestHasShapeTriangleAllSixOrientations(t *testing.T) {
	orientations := triangleOrientationCells(t)
	for r, cells := range orientations {
		t.Run(itoaTest(r), func(t *testing.T) {
			tiles := map[Hex]Side{}
			for _, p := range cells {
				tiles[p] = White
			}
			kind, got := HasShape(tiles, White)
			if kind != ShapeTriangle {
				t.Fatalf("rotation %d: expected ShapeTriangle, got %v (cells=%v)", r, kind, cells)
			}
			if !hexSetEqual(got, cells) {
				t.Fatalf("rotation %d: winning cells = %v, want %v", r, got, cells)
			}
		})
	}
}

// Triangle at a different anchor (not centered on the origin) — confirms
// detection isn't accidentally origin-specific.
func TestHasShapeTriangleOffCenterAnchor(t *testing.T) {
	anchor := Hex{-2, 3, -1}
	cells := make([]Hex, 6)
	for i, off := range triangleTemplate {
		cells[i] = anchor.Add(off)
	}
	tiles := map[Hex]Side{}
	for _, p := range cells {
		tiles[p] = Black
	}
	kind, got := HasShape(tiles, Black)
	if kind != ShapeTriangle {
		t.Fatalf("expected ShapeTriangle at off-center anchor, got %v", kind)
	}
	if !hexSetEqual(got, cells) {
		t.Fatalf("winning cells = %v, want %v", got, cells)
	}
}

// GOTCHA: a 5-of-6 triangle (one corner missing) must NOT win.
func TestHasShapeTriangleNearMissDoesNotWin(t *testing.T) {
	cells := triangleTemplate[:]
	tiles := map[Hex]Side{}
	for _, p := range cells[:5] {
		tiles[p] = White
	}
	if kind, got := HasShape(tiles, White); kind != ShapeNone {
		t.Fatalf("a 5-of-6 triangle must not win, got %v cells=%v", kind, got)
	}
}

// --- Hexagon (ring): several centers, plus a 5-tile near-miss ---------------

func TestHasShapeHexRingSeveralCenters(t *testing.T) {
	centers := []Hex{{0, 0, 0}, {2, -3, 1}, {-1, -2, 3}}
	for _, center := range centers {
		t.Run(center.String(), func(t *testing.T) {
			ring := Neighbors(center)
			if len(ring) != 6 {
				t.Fatalf("test setup error: center %v does not have 6 in-board neighbours (got %d) — pick a more interior test center", center, len(ring))
			}
			tiles := map[Hex]Side{}
			for _, p := range ring {
				tiles[p] = Black
			}
			kind, got := HasShape(tiles, Black)
			if kind != ShapeHexRing {
				t.Fatalf("center %v: expected ShapeHexRing, got %v", center, kind)
			}
			if !hexSetEqual(got, ring) {
				t.Fatalf("center %v: winning cells = %v, want %v", center, got, ring)
			}
		})
	}
}

// GOTCHA: the ring's centre cell is irrelevant — empty, the mover's own
// color, or the opponent's color must all still win the ring for the side
// that owns all 6 surrounding cells.
func TestHasShapeHexRingCenterIrrelevant(t *testing.T) {
	center := Hex{0, 0, 0}
	ring := Neighbors(center)
	for _, centerFill := range []Side{None, Black, White} {
		tiles := map[Hex]Side{}
		for _, p := range ring {
			tiles[p] = Black
		}
		if centerFill != None {
			tiles[center] = centerFill
		}
		kind, _ := HasShape(tiles, Black)
		if kind != ShapeHexRing {
			t.Fatalf("center fill=%v: expected ShapeHexRing regardless of centre occupant, got %v", centerFill, kind)
		}
	}
}

// GOTCHA: a 5-of-6 ring (one neighbour missing) must NOT win.
func TestHasShapeHexRingNearMissDoesNotWin(t *testing.T) {
	center := Hex{0, 0, 0}
	ring := Neighbors(center)
	tiles := map[Hex]Side{}
	for _, p := range ring[:5] {
		tiles[p] = Black
	}
	if kind, got := HasShape(tiles, Black); kind != ShapeNone {
		t.Fatalf("a 5-of-6 ring must not win, got %v cells=%v", kind, got)
	}
}

// String gives Hex a compact test-name-friendly representation.
func (p Hex) String() string {
	return "(" + itoaTest(p.X) + "," + itoaTest(p.Y) + "," + itoaTest(p.Z) + ")"
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
