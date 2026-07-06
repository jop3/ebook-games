package game

import "sort"

// Offset is a relative (x,y) cell within a piece's shape, before any
// placement or orientation transform.
type Offset [2]int

// PieceDef is one placeable building: a display name and its base shape (a
// list of offsets, normalized so the minimum x and y are both 0).
type PieceDef struct {
	Name  string
	Cells []Offset
}

// NumPieces is the number of secular building pieces each side owns.
const NumPieces = 13

// Pieces is our own original 13-piece roster of secular "building" shapes,
// sizes 1-4 squares, each side owning an identical set. The spec this game is
// built from explicitly flags that the real Cathedral board game's exact
// 13-piece geometry is not something to guess from memory, and that an
// original set is the safer (and expected) choice for this port — the
// enclosure/capture mechanic is what matters and is not itself copyrightable.
// Up to rotation and reflection there are only 9 distinct free polyominoes of
// size <=4 (1 monomino, 1 domino, 2 trominoes, 5 tetrominoes); to reach 13
// pieces per side (as called for in the spec) some shapes are deliberately
// duplicated under a second building name, exactly as a real building set
// would plausibly contain two small cottages or two garden walls of the same
// footprint.
var Pieces = [NumPieces]PieceDef{
	// Monominoes (1 square) x3.
	{Name: "Stuga", Cells: []Offset{{0, 0}}},
	{Name: "Koja", Cells: []Offset{{0, 0}}},
	{Name: "Skjul", Cells: []Offset{{0, 0}}},

	// Dominoes (2 squares, straight) x2.
	{Name: "Mur", Cells: []Offset{{0, 0}, {1, 0}}},
	{Name: "Hägnad", Cells: []Offset{{0, 0}, {1, 0}}},

	// Trominoes (3 squares): 1 straight, 2 L-shaped.
	{Name: "Gränd", Cells: []Offset{{0, 0}, {1, 0}, {2, 0}}},
	{Name: "Vinkelhus", Cells: []Offset{{0, 0}, {1, 0}, {0, 1}}},
	{Name: "Krök", Cells: []Offset{{0, 0}, {1, 0}, {0, 1}}},

	// Tetrominoes (4 squares): the 5 free tetromino shapes, one each.
	{Name: "Allé", Cells: []Offset{{0, 0}, {1, 0}, {2, 0}, {3, 0}}},   // I
	{Name: "Torg", Cells: []Offset{{0, 0}, {1, 0}, {0, 1}, {1, 1}}},   // O
	{Name: "Kapell", Cells: []Offset{{0, 0}, {1, 0}, {2, 0}, {1, 1}}}, // T
	{Name: "Bro", Cells: []Offset{{1, 0}, {2, 0}, {0, 1}, {1, 1}}},    // S/Z
	{Name: "Krog", Cells: []Offset{{0, 0}, {0, 1}, {0, 2}, {1, 2}}},   // L/J
}

// CathedralShape is the neutral 5-square cross-shaped Cathedral piece,
// placed first (unowned by either side).
var CathedralShape = PieceDef{
	Name:  "Katedralen",
	Cells: []Offset{{1, 0}, {0, 1}, {1, 1}, {2, 1}, {1, 2}},
}

// Size2 returns the number of squares a piece occupies.
func (p PieceDef) Size() int { return len(p.Cells) }

// rotate90 rotates offset p 90 degrees (direction is irrelevant since all 4
// quarter-turns are generated regardless of sense).
func rotate90(p Offset) Offset { return Offset{-p[1], p[0]} }

// reflectX mirrors offset p across a vertical axis.
func reflectX(p Offset) Offset { return Offset{-p[0], p[1]} }

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

func shapeKey(cells []Offset) [16]int8 {
	var k [16]int8 // up to 4 cells * 2 coords + up to... generous fixed bound
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
// normalized and sorted in row-major order. The result is deterministic and
// cached per call (cheap: at most 4x2 transforms of at most 5 cells).
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

// BoundingBox returns the max x and max y among cells (which are assumed
// already normalized so the mins are both 0).
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

// Anchor returns the shape's canonical tap-target cell: its top-left-most
// cell in row-major order (cells is assumed normalized+sorted, as returned
// by Orientations/normalize).
func Anchor(cells []Offset) Offset { return cells[0] }
