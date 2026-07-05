package game

// shapes.go: the three winning-shape templates (Linje/Triangel/Hexagon) and
// their detection. All board geometry is fixed at compile time (Radius is a
// constant), so every possible occurrence of every shape on the board is
// precomputed once, in init(), as a flat list of 6-cell "instances" — both
// HasShape and the AI's progress heuristic (ai.go) just scan that list.

// ShapeKind identifies which of the three winning shapes was found.
type ShapeKind int

const (
	ShapeNone ShapeKind = iota
	ShapeLine
	ShapeTriangle
	ShapeHexRing
)

// String names a shape kind in Swedish, for the UI banner.
func (k ShapeKind) String() string {
	switch k {
	case ShapeLine:
		return "Linje"
	case ShapeTriangle:
		return "Triangel"
	case ShapeHexRing:
		return "Hexagon"
	default:
		return ""
	}
}

// shapeInstance is one concrete occurrence of a winning shape on this
// board: 6 specific cells, tagged with which kind of shape they'd form if
// all 6 belonged to one side.
type shapeInstance struct {
	Cells [6]Hex
	Kind  ShapeKind
}

// rotate60 rotates a cube-coordinate offset by 60° about the origin. Applied
// repeatedly it cycles through exactly the 6 orientations of the hex grid
// (rotate60^6 = identity) — the same trick used to enumerate Directions'
// six neighbours as one rotational family (verified in coords_test.go).
func rotate60(p Hex) Hex {
	return Hex{-p.Z, -p.X, -p.Y}
}

// triangleTemplate is a size-3 triangle of 6 hex cells — rows of 3, 2 and 1
// cells, each row's cells and each row-to-row step being a genuine hex
// adjacency (built directly from Directions so it is correct by
// construction, with no reliance on any axial-coordinate conversion):
//
//	row0 (bottom, 3 cells): (0,0,0) -> +Directions[0] -> +Directions[0]
//	row1 (middle, 2 cells): row0[0] + Directions[4], then +Directions[0]
//	row2 (top, 1 cell):     row1[0] + Directions[4]
var triangleTemplate = [6]Hex{
	{0, 0, 0}, {1, -1, 0}, {2, -2, 0},
	{0, 1, -1}, {1, 0, -1},
	{0, 2, -2},
}

// allShapeInstances is precomputed once at package init: every occurrence of
// every winning shape that fits entirely on the board.
var allShapeInstances []shapeInstance

func init() {
	pts := AllPoints()
	inBoard := make(map[Hex]bool, len(pts))
	for _, p := range pts {
		inBoard[p] = true
	}

	// Lines: for each canonical axis direction and each possible anchor,
	// a run of 6 consecutive cells along that direction, if all 6 are on
	// the board. Each maximal line produces (length-5) overlapping
	// 6-windows as the anchor slides along it; using only one direction
	// per axis (not its reverse) avoids ever generating the same window
	// twice from opposite ends.
	for _, d := range axisDirs {
		for _, anchor := range pts {
			var cells [6]Hex
			ok := true
			cur := anchor
			for i := 0; i < 6; i++ {
				if i > 0 {
					cur = cur.Add(d)
				}
				if !inBoard[cur] {
					ok = false
					break
				}
				cells[i] = cur
			}
			if ok {
				allShapeInstances = append(allShapeInstances, shapeInstance{cells, ShapeLine})
			}
		}
	}

	// Triangles: all 6 rotations of triangleTemplate, anchored at every
	// board cell (the anchor is the template's own (0,0,0) point under
	// that rotation), kept only if all 6 translated cells are on the
	// board.
	orientations := make([][6]Hex, 6)
	cur := triangleTemplate
	for r := 0; r < 6; r++ {
		orientations[r] = cur
		var next [6]Hex
		for i, p := range cur {
			next[i] = rotate60(p)
		}
		cur = next
	}
	for _, orient := range orientations {
		for _, anchor := range pts {
			var cells [6]Hex
			ok := true
			for i, off := range orient {
				c := anchor.Add(off)
				if !inBoard[c] {
					ok = false
					break
				}
				cells[i] = c
			}
			if ok {
				allShapeInstances = append(allShapeInstances, shapeInstance{cells, ShapeTriangle})
			}
		}
	}

	// Hexagon-rings: for every candidate center (which need not itself be
	// occupied, or even meaningfully "on the board" — only the 6 ring
	// cells matter), its 6 neighbours, kept only if all 6 are on the
	// board. Restricting candidate centers to AllPoints() (radius <=
	// Radius) loses no real ring: a center at radius Radius+1 always has
	// at least one neighbour at radius Radius+2, off the board, so it can
	// never have a fully occupied ring anyway.
	for _, center := range pts {
		var cells [6]Hex
		ok := true
		for i, d := range Directions {
			c := center.Add(d)
			if !inBoard[c] {
				ok = false
				break
			}
			cells[i] = c
		}
		if ok {
			allShapeInstances = append(allShapeInstances, shapeInstance{cells, ShapeHexRing})
		}
	}
}

// HasShape reports whether side has completed any winning shape, returning
// the kind and the 6 winning cells (for UI highlighting) if so. Iterates
// the precomputed instance list, so it is a plain "is every cell in this
// prospective shape occupied by side" scan.
func HasShape(tiles map[Hex]Side, side Side) (ShapeKind, []Hex) {
	for _, inst := range allShapeInstances {
		match := true
		for _, c := range inst.Cells {
			if tiles[c] != side {
				match = false
				break
			}
		}
		if match {
			cells := make([]Hex, 6)
			copy(cells, inst.Cells[:])
			return inst.Kind, cells
		}
	}
	return ShapeNone, nil
}
