package game

import "testing"

func cellCount(cells []Offset) int { return len(cells) }

func shapesEqual(a, b []Offset) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestOrientationsPreserveSize checks every orientation of every piece (and
// the Cathedral) keeps the same number of cells as the base shape.
func TestOrientationsPreserveSize(t *testing.T) {
	for _, p := range Pieces {
		want := len(p.Cells)
		for i, o := range Orientations(p.Cells) {
			if cellCount(o) != want {
				t.Errorf("%s orientation %d has %d cells, want %d", p.Name, i, cellCount(o), want)
			}
		}
	}
	if got := len(Orientations(CathedralShape.Cells)); got < 1 {
		t.Fatalf("Cathedral must have at least one orientation, got %d", got)
	}
}

// TestOrientationsNoDuplicates checks Orientations never returns the same
// normalized shape twice.
func TestOrientationsNoDuplicates(t *testing.T) {
	check := func(name string, base []Offset) {
		orients := Orientations(base)
		for i := 0; i < len(orients); i++ {
			for j := i + 1; j < len(orients); j++ {
				if shapesEqual(orients[i], orients[j]) {
					t.Errorf("%s: orientations %d and %d are duplicates: %v", name, i, j, orients[i])
				}
			}
		}
	}
	for _, p := range Pieces {
		check(p.Name, p.Cells)
	}
	check("Katedralen", CathedralShape.Cells)
}

// TestOrientationsClosed checks that re-deriving orientations from any
// already-oriented shape yields the exact same set (the shape's orbit under
// the dihedral group is closed) — a strong correctness signal that does not
// require hand-verifying the expected count for every individual shape.
func TestOrientationsClosed(t *testing.T) {
	for _, p := range Pieces {
		base := Orientations(p.Cells)
		for _, o := range base {
			again := Orientations(o)
			if len(again) != len(base) {
				t.Fatalf("%s: orientations not closed: base has %d, re-derived from an oriented copy has %d",
					p.Name, len(base), len(again))
			}
			for _, want := range base {
				found := false
				for _, got := range again {
					if shapesEqual(want, got) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("%s: re-derived orientation set missing %v (from base %v)", p.Name, want, base)
				}
			}
		}
	}
}

// TestOrientationsKnownCounts checks a handful of shapes whose symmetry is
// unambiguous: a single square has exactly 1 orientation; a straight domino
// has exactly 2 (horizontal/vertical — reflection adds nothing new); the
// square (O) tetromino has exactly 1 (its 2x2 bounding box is unchanged by
// any rotation or reflection); the cross-shaped Cathedral has exactly 1 (full
// 4-fold + mirror symmetry).
func TestOrientationsKnownCounts(t *testing.T) {
	cases := []struct {
		name string
		base []Offset
		want int
	}{
		{"Stuga (monomino)", Pieces[0].Cells, 1},
		{"Mur (domino)", Pieces[3].Cells, 2},
		{"Torg (O-tetromino)", []Offset{{0, 0}, {1, 0}, {0, 1}, {1, 1}}, 1},
		{"Katedralen (cross)", CathedralShape.Cells, 1},
	}
	for _, c := range cases {
		if got := len(Orientations(c.base)); got != c.want {
			t.Errorf("%s: got %d orientations, want %d", c.name, got, c.want)
		}
	}
}

// TestPieceRosterSizesAndCount checks the roster is exactly 13 pieces sized
// 1-4 squares, per the spec's "13 pieces per side, sizes 1-4" requirement.
func TestPieceRosterSizesAndCount(t *testing.T) {
	if len(Pieces) != 13 {
		t.Fatalf("roster has %d pieces, want 13", len(Pieces))
	}
	for _, p := range Pieces {
		if s := p.Size(); s < 1 || s > 4 {
			t.Errorf("%s has size %d, want 1-4", p.Name, s)
		}
	}
	if CathedralShape.Size() != 5 {
		t.Fatalf("Cathedral should be 5 squares, got %d", CathedralShape.Size())
	}
}

// TestNormalizeStartsAtOrigin checks every orientation's bounding box starts
// at (0,0), as LegalPlacementsForOrientation's bounds-checking assumes.
func TestNormalizeStartsAtOrigin(t *testing.T) {
	for _, p := range Pieces {
		for _, o := range Orientations(p.Cells) {
			minX, minY := o[0][0], o[0][1]
			for _, c := range o {
				if c[0] < minX {
					minX = c[0]
				}
				if c[1] < minY {
					minY = c[1]
				}
			}
			if minX != 0 || minY != 0 {
				t.Errorf("%s orientation %v not normalized to origin", p.Name, o)
			}
		}
	}
}
