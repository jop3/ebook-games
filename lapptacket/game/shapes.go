package game

import (
	"image"
	"sort"
)

// Offset is a relative (x,y) cell within a patch's shape, before any
// placement or orientation transform. The same small polyomino-orientation
// approach used by this repo's stadskarnan (Cathedral) port is reused here:
// normalize to a canonical, sorted form and generate the (up to 8) distinct
// rotations/reflections on demand.
type Offset [2]int

func rotate90(p Offset) Offset  { return Offset{-p[1], p[0]} }
func reflectX(p Offset) Offset  { return Offset{-p[0], p[1]} }

// normalize shifts cells so the minimum x and y are both 0, then returns them
// sorted in row-major order (by y, then x) for a stable, canonical form.
func normalize(cells []Offset) []Offset {
	minX, minY := cells[0][0], cells[0][1]
	for _, c := range cells {
		if c[0] < minX {
			minX = c[0]
		}
		if c[1] < minY {
			minY = c[1]
		}
	}
	out := make([]Offset, len(cells))
	for i, c := range cells {
		out[i] = Offset{c[0] - minX, c[1] - minY}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][1] != out[j][1] {
			return out[i][1] < out[j][1]
		}
		return out[i][0] < out[j][0]
	})
	return out
}

// shapeKey is a hashable canonical form, used to dedupe orientations that
// coincide because of symmetry (e.g. a 2x2 square has only 1 orientation).
func shapeKey(cells []Offset) [16]int8 {
	var k [16]int8 // patches here max out at 8 cells * 2 coords
	i := 0
	for _, c := range cells {
		if i+1 < len(k) {
			k[i] = int8(c[0])
			k[i+1] = int8(c[1])
		}
		i += 2
	}
	return k
}

// Orientations returns every distinct oriented shape reachable from base by
// rotation and reflection (up to 8, fewer for shapes with symmetry), each
// normalized and sorted in row-major order. Deterministic given the same
// input, and cheap enough to call on demand (at most 8 transforms of at most
// 8 cells per patch).
func Orientations(base []Offset) [][]Offset {
	seen := map[[16]int8]bool{}
	var out [][]Offset
	for reflect := 0; reflect < 2; reflect++ {
		cur := make([]Offset, len(base))
		copy(cur, base)
		if reflect == 1 {
			for i, c := range cur {
				cur[i] = reflectX(c)
			}
		}
		for r := 0; r < 4; r++ {
			norm := normalize(cur)
			key := shapeKey(norm)
			if !seen[key] {
				seen[key] = true
				out = append(out, norm)
			}
			next := make([]Offset, len(cur))
			for i, c := range cur {
				next[i] = rotate90(c)
			}
			cur = next
		}
	}
	return out
}

// BoundingBox returns the max x and max y among cells (cells are assumed
// already normalized, so the mins are both 0).
func BoundingBox(cells []Offset) (maxX, maxY int) {
	for _, c := range cells {
		if c[0] > maxX {
			maxX = c[0]
		}
		if c[1] > maxY {
			maxY = c[1]
		}
	}
	return
}

// Anchor is the shape's canonical tap-target cell: its top-left-most cell in
// row-major order (cells assumed normalized+sorted, as Orientations returns).
func Anchor(cells []Offset) Offset { return cells[0] }

// Placement is one concrete way to lay a shape on a board: the absolute
// cells it would cover, plus the Anchor world cell the UI highlights/taps to
// commit it.
type Placement struct {
	Cells  []image.Point
	Anchor image.Point
}

// canPlaceAt reports whether oriented (a single normalized+sorted element of
// Orientations(...)) fits entirely on empty board cells when its anchor cell
// (oriented[0]) is placed at the world point anchor.
func canPlaceAt(b *Board, oriented []Offset, anchor image.Point) ([]image.Point, bool) {
	anchorRel := Anchor(oriented)
	offX, offY := anchor.X-anchorRel[0], anchor.Y-anchorRel[1]
	cells := make([]image.Point, len(oriented))
	for i, c := range oriented {
		x, y := offX+c[0], offY+c[1]
		if x < 0 || x >= BoardSize || y < 0 || y >= BoardSize || b.Filled[y][x] {
			return nil, false
		}
		cells[i] = image.Pt(x, y)
	}
	return cells, true
}

// legalPlacementsForOriented returns every legal Placement of the given
// oriented shape on board b.
func legalPlacementsForOriented(b *Board, oriented []Offset) []Placement {
	maxX, maxY := BoundingBox(oriented)
	var out []Placement
	for oy := 0; oy+maxY < BoardSize; oy++ {
		for ox := 0; ox+maxX < BoardSize; ox++ {
			ok := true
			cells := make([]image.Point, len(oriented))
			for i, c := range oriented {
				x, y := ox+c[0], oy+c[1]
				if b.Filled[y][x] {
					ok = false
					break
				}
				cells[i] = image.Pt(x, y)
			}
			if !ok {
				continue
			}
			anchorRel := Anchor(oriented)
			out = append(out, Placement{
				Cells:  cells,
				Anchor: image.Pt(ox+anchorRel[0], oy+anchorRel[1]),
			})
		}
	}
	return out
}

// LegalPlacementsForOrientation returns every legal Placement of patch
// orientation orientIdx (as indexed into Orientations(patch.Cells)) on b.
func LegalPlacementsForOrientation(b *Board, patch Patch, orientIdx int) []Placement {
	orients := Orientations(patch.Cells)
	if orientIdx < 0 || orientIdx >= len(orients) {
		return nil
	}
	return legalPlacementsForOriented(b, orients[orientIdx])
}

// LegalPlacements returns every legal Placement of patch, over all of its
// distinct orientations, on b. Used by the AI to search candidate moves.
func LegalPlacements(b *Board, patch Patch) []Placement {
	var out []Placement
	for _, o := range Orientations(patch.Cells) {
		out = append(out, legalPlacementsForOriented(b, o)...)
	}
	return out
}

// HasLegalPlacement reports whether patch fits anywhere on b, without
// allocating the full placement list.
func HasLegalPlacement(b *Board, patch Patch) bool {
	for _, o := range Orientations(patch.Cells) {
		maxX, maxY := BoundingBox(o)
		for oy := 0; oy+maxY < BoardSize; oy++ {
			for ox := 0; ox+maxX < BoardSize; ox++ {
				ok := true
				for _, c := range o {
					x, y := ox+c[0], oy+c[1]
					if b.Filled[y][x] {
						ok = false
						break
					}
				}
				if ok {
					return true
				}
			}
		}
	}
	return false
}
