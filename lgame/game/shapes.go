package game

import (
	"image"
	"sort"
)

// baseLShape is one orientation of the L-tetromino, as offsets from its
// bounding box's top-left corner:
//
//	X X X
//	. . X
var baseLShape = []image.Point{{0, 0}, {1, 0}, {2, 0}, {2, 1}}

// normalizeShape shifts pts so its bounding box's minimum is (0,0), then
// sorts the result into a canonical (y, then x) order so two equal shapes
// compare equal regardless of input order.
func normalizeShape(pts []image.Point) []image.Point {
	minX, minY := pts[0].X, pts[0].Y
	for _, p := range pts {
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
	}
	out := make([]image.Point, len(pts))
	for i, p := range pts {
		out[i] = image.Pt(p.X-minX, p.Y-minY)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].X < out[j].X
	})
	return out
}

// rotateShape90 rotates pts 90 degrees and re-normalizes.
func rotateShape90(pts []image.Point) []image.Point {
	out := make([]image.Point, len(pts))
	for i, p := range pts {
		out[i] = image.Pt(-p.Y, p.X)
	}
	return normalizeShape(out)
}

// reflectShape mirrors pts horizontally (x -> -x) and re-normalizes.
func reflectShape(pts []image.Point) []image.Point {
	out := make([]image.Point, len(pts))
	for i, p := range pts {
		out[i] = image.Pt(-p.X, p.Y)
	}
	return normalizeShape(out)
}

// shapeDims returns the (width, height) of a normalized shape's bounding box.
func shapeDims(pts []image.Point) (w, h int) {
	for _, p := range pts {
		if p.X+1 > w {
			w = p.X + 1
		}
		if p.Y+1 > h {
			h = p.Y + 1
		}
	}
	return w, h
}

// computeLOrientations derives all 8 rotation/reflection orientations of the
// L-tetromino from baseLShape programmatically: the 4 rotations of the base
// shape, then the 4 rotations of its mirror image. It is not a hand-typed
// table — see shapes_test.go for the independent, hand-derived reference set
// that guards against a rotation-math bug silently producing a wrong shape.
func computeLOrientations() [8][]image.Point {
	var out [8][]image.Point
	cur := normalizeShape(baseLShape)
	for i := 0; i < 4; i++ {
		out[i] = cur
		cur = rotateShape90(cur)
	}
	cur = reflectShape(baseLShape)
	for i := 0; i < 4; i++ {
		out[4+i] = cur
		cur = rotateShape90(cur)
	}
	return out
}

// LOrientations holds the 8 valid L-piece orientations (4 rotations x 2
// reflections), each a list of 4 cell offsets from its bounding box's
// top-left corner, normalized and sorted (see normalizeShape).
var LOrientations = computeLOrientations()
