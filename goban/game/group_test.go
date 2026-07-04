package game

import (
	"image"
	"testing"
)

func TestGroupSingleStone(t *testing.T) {
	b := NewBoard(9)
	b.Set(image.Pt(4, 4), Black)
	grp := Group(b, image.Pt(4, 4))
	if len(grp) != 1 || grp[0] != image.Pt(4, 4) {
		t.Fatalf("isolated stone should be its own 1-stone group, got %v", grp)
	}
}

func TestGroupConnectedChain(t *testing.T) {
	b := NewBoard(9)
	b.Set(image.Pt(2, 2), Black)
	b.Set(image.Pt(3, 2), Black)
	b.Set(image.Pt(4, 2), Black)
	b.Set(image.Pt(4, 3), Black) // turns the corner
	b.Set(image.Pt(3, 3), White) // not connected: different color
	grp := Group(b, image.Pt(2, 2))
	if len(grp) != 4 {
		t.Fatalf("orthogonal chain should merge into one 4-stone group, got %d: %v", len(grp), grp)
	}
	// Diagonal adjacency must NOT connect stones in Go.
	b2 := NewBoard(9)
	b2.Set(image.Pt(2, 2), Black)
	b2.Set(image.Pt(3, 3), Black) // diagonal neighbor only
	if len(Group(b2, image.Pt(2, 2))) != 1 {
		t.Fatal("diagonally adjacent stones must not be considered connected")
	}
}

func TestLiberties(t *testing.T) {
	b := NewBoard(9)
	b.Set(image.Pt(4, 4), Black)
	grp := Group(b, image.Pt(4, 4))
	if libs := Liberties(b, grp); len(libs) != 4 {
		t.Fatalf("a lone interior stone should have 4 liberties, got %d: %v", len(libs), libs)
	}

	// Surround it on 3 sides; 1 liberty remains.
	b.Set(image.Pt(3, 4), White)
	b.Set(image.Pt(5, 4), White)
	b.Set(image.Pt(4, 3), White)
	grp = Group(b, image.Pt(4, 4))
	if libs := Liberties(b, grp); len(libs) != 1 || libs[0] != image.Pt(4, 5) {
		t.Fatalf("expected exactly 1 liberty at (4,5), got %v", libs)
	}
}

func TestBorderColorsSingleAndMixed(t *testing.T) {
	b := NewBoard(9)
	// A 2-point empty region bordered only by Black.
	b.Set(image.Pt(0, 2), Black)
	b.Set(image.Pt(2, 0), Black)
	b.Set(image.Pt(2, 2), Black)
	region := Group(b, image.Pt(1, 1)) // the empty pocket around (0,0)-(1,1)-(0,1)-(1,0)
	borders := BorderColors(b, region)
	if len(borders) != 1 || !borders[Black] {
		t.Fatalf("region should border only Black, got %v", borders)
	}

	// A region touching both colors is mixed.
	b2 := NewBoard(9)
	b2.Set(image.Pt(0, 1), Black)
	b2.Set(image.Pt(1, 0), White)
	mixed := Group(b2, image.Pt(0, 0))
	mb := BorderColors(b2, mixed)
	if !mb[Black] || !mb[White] {
		t.Fatalf("region touching both colors should report both, got %v", mb)
	}
}
