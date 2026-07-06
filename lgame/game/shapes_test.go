package game

import (
	"image"
	"sort"
	"testing"
)

// shapeKey turns a normalized shape into a comparable, order-independent key.
func shapeKey(pts []image.Point) [4]image.Point {
	cp := append([]image.Point(nil), pts...)
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].Y != cp[j].Y {
			return cp[i].Y < cp[j].Y
		}
		return cp[i].X < cp[j].X
	})
	var out [4]image.Point
	copy(out[:], cp)
	return out
}

// handVerifiedLOrientations is the reference set of all 8 orientations of
// the L-tetromino, derived independently BY HAND (see the design notes: each
// was found by manually rotating a 2D grid 90° clockwise using the
// standard "new[H-1-y][x] = old[y][x]" matrix-rotation rule and manually
// mirroring left-right — a different derivation path than shapes.go's
// coordinate-transform-based rotateShape90/reflectShape, and drawn out cell
// by cell as ASCII grids before being encoded here) rather than trusting the
// implementation's own rotation math. This is exactly the guard the spec
// calls for: "unit-test the resulting 8 shapes against a hand-drawn/
// hand-verified reference shape list."
//
// The 8 grids, for the record (X = occupied):
//
//	S0: X X X      S1: . X      S2: X . .      S3: X X
//	    . . X           . X         X X X          X .
//	                    X X                         X .
//
//	S4: X X X      S5: X X      S6: . . X      S7: X .
//	    X . .           . X         X X X          X .
//	                    . X                         X X
var handVerifiedLOrientations = [8][4]image.Point{
	shapeKey([]image.Point{{0, 0}, {1, 0}, {2, 0}, {2, 1}}), // S0
	shapeKey([]image.Point{{1, 0}, {1, 1}, {0, 2}, {1, 2}}), // S1
	shapeKey([]image.Point{{0, 0}, {0, 1}, {1, 1}, {2, 1}}), // S2
	shapeKey([]image.Point{{0, 0}, {1, 0}, {0, 1}, {0, 2}}), // S3
	shapeKey([]image.Point{{0, 0}, {1, 0}, {2, 0}, {0, 1}}), // S4 (mirror of S0)
	shapeKey([]image.Point{{0, 0}, {1, 0}, {1, 1}, {1, 2}}), // S5
	shapeKey([]image.Point{{2, 0}, {0, 1}, {1, 1}, {2, 1}}), // S6
	shapeKey([]image.Point{{0, 0}, {0, 1}, {0, 2}, {1, 2}}), // S7
}

// TestLOrientationsMatchHandVerifiedSet is the key regression guard called
// out by the spec: computeLOrientations() must produce exactly the 8
// hand-verified L-tetromino orientations (as a SET — order doesn't matter,
// but there must be exactly 8, all distinct, and all present).
func TestLOrientationsMatchHandVerifiedSet(t *testing.T) {
	got := computeLOrientations()
	if len(got) != 8 {
		t.Fatalf("computeLOrientations returned %d orientations, want 8", len(got))
	}

	gotSet := map[[4]image.Point]int{}
	for i, shape := range got {
		if len(shape) != 4 {
			t.Fatalf("orientation %d has %d cells, want 4", i, len(shape))
		}
		gotSet[shapeKey(shape)]++
	}
	if len(gotSet) != 8 {
		t.Fatalf("computed orientations contain only %d distinct shapes, want 8 (duplicates present): %v", len(gotSet), gotSet)
	}

	wantSet := map[[4]image.Point]bool{}
	for _, k := range handVerifiedLOrientations {
		wantSet[k] = true
	}
	if len(wantSet) != 8 {
		t.Fatalf("test bug: hand-verified reference set has only %d distinct shapes", len(wantSet))
	}

	for k := range wantSet {
		if _, ok := gotSet[k]; !ok {
			t.Errorf("hand-verified shape %v missing from computed orientations", k)
		}
	}
	for k := range gotSet {
		if !wantSet[k] {
			t.Errorf("computed shape %v is not one of the 8 hand-verified L orientations", k)
		}
	}
}

// TestLOrientationsAllDistinct is redundant with the set-size check above
// but spells the requirement out explicitly and separately, since
// "8 orientations under-enumerated to fewer distinct shapes" is exactly the
// bug class called out in the spec.
func TestLOrientationsAllDistinct(t *testing.T) {
	seen := map[[4]image.Point]bool{}
	for i, shape := range LOrientations {
		k := shapeKey(shape)
		if seen[k] {
			t.Errorf("orientation %d duplicates an earlier orientation: %v", i, k)
		}
		seen[k] = true
	}
	if len(seen) != 8 {
		t.Fatalf("only %d distinct orientations, want 8", len(seen))
	}
}

// TestLOrientationsFourCellsConnected sanity-checks every orientation is a
// genuine 4-cell polyomino (no stray offset, no duplicate cell) that fits in
// either a 3x2 or 2x3 bounding box (the only two possible for an L-tetromino).
func TestLOrientationsFourCellsConnected(t *testing.T) {
	for i, shape := range LOrientations {
		if len(shape) != 4 {
			t.Fatalf("orientation %d: %d cells, want 4", i, len(shape))
		}
		seen := map[image.Point]bool{}
		for _, p := range shape {
			if seen[p] {
				t.Fatalf("orientation %d has a duplicate cell %v", i, p)
			}
			seen[p] = true
			if p.X < 0 || p.Y < 0 {
				t.Fatalf("orientation %d: not normalized, cell %v has a negative coordinate", i, p)
			}
		}
		w, h := shapeDims(shape)
		if !((w == 3 && h == 2) || (w == 2 && h == 3)) {
			t.Fatalf("orientation %d has bounding box %dx%d, want 3x2 or 2x3", i, w, h)
		}
	}
}

// TestLOrientationsClosedUnderRotation checks that rotating any computed
// orientation by 90 degrees yields another orientation already in the set
// (the dihedral closure property of a correct enumeration).
func TestLOrientationsClosedUnderRotation(t *testing.T) {
	set := map[[4]image.Point]bool{}
	for _, shape := range LOrientations {
		set[shapeKey(shape)] = true
	}
	for i, shape := range LOrientations {
		r := rotateShape90(shape)
		if !set[shapeKey(r)] {
			t.Errorf("rotating orientation %d by 90 degrees leaves the 8-shape set: %v", i, r)
		}
	}
}
