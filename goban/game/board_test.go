package game

import (
	"image"
	"testing"
)

func TestNewBoardEmpty(t *testing.T) {
	b := NewBoard(9)
	if b.Size() != 9 {
		t.Fatalf("Size() = %d, want 9", b.Size())
	}
	if b.Count(Black) != 0 || b.Count(White) != 0 {
		t.Fatal("a new board should be entirely empty")
	}
}

func TestSetAtInBounds(t *testing.T) {
	b := NewBoard(9)
	b.Set(image.Pt(3, 4), Black)
	if b.At(image.Pt(3, 4)) != Black {
		t.Fatal("At should read back what Set wrote")
	}
	if b.At(image.Pt(-1, 0)) != Empty || b.At(image.Pt(9, 0)) != Empty {
		t.Fatal("out-of-bounds At should read Empty, not panic")
	}
	if b.InBounds(image.Pt(9, 9)) || !b.InBounds(image.Pt(8, 8)) {
		t.Fatal("InBounds boundary check is off by one")
	}
}

func TestCloneIsIndependent(t *testing.T) {
	b := NewBoard(9)
	b.Set(image.Pt(0, 0), Black)
	c := b.Clone()
	c.Set(image.Pt(0, 0), White)
	if b.At(image.Pt(0, 0)) != Black {
		t.Fatal("mutating a clone must not affect the original board")
	}
}

func TestEqual(t *testing.T) {
	a := NewBoard(9)
	b := NewBoard(9)
	if !Equal(a, b) {
		t.Fatal("two empty boards of the same size should be equal")
	}
	b.Set(image.Pt(4, 4), Black)
	if Equal(a, b) {
		t.Fatal("boards differing in one stone should not be equal")
	}
	c := NewBoard(13)
	if Equal(a, c) {
		t.Fatal("boards of different sizes should not be equal")
	}
}

func TestNeighborsCornerAndEdge(t *testing.T) {
	b := NewBoard(9)
	corner := b.Neighbors(image.Pt(0, 0))
	if len(corner) != 2 {
		t.Fatalf("corner should have 2 neighbors, got %d: %v", len(corner), corner)
	}
	edge := b.Neighbors(image.Pt(0, 4))
	if len(edge) != 3 {
		t.Fatalf("edge should have 3 neighbors, got %d: %v", len(edge), edge)
	}
	center := b.Neighbors(image.Pt(4, 4))
	if len(center) != 4 {
		t.Fatalf("interior point should have 4 neighbors, got %d: %v", len(center), center)
	}
}

func TestHoshiPoints(t *testing.T) {
	if got := HoshiPoints(9); len(got) != 5 {
		t.Fatalf("9x9 should have 5 hoshi points (4 corners + tengen), got %d: %v", len(got), got)
	}
	if got := HoshiPoints(13); len(got) != 5 {
		t.Fatalf("13x13 should have 5 hoshi points, got %d: %v", len(got), got)
	}
	if got := HoshiPoints(19); len(got) != 9 {
		t.Fatalf("19x19 should have 9 hoshi points, got %d: %v", len(got), got)
	}
	for _, p := range HoshiPoints(9) {
		if p.X < 0 || p.X >= 9 || p.Y < 0 || p.Y >= 9 {
			t.Fatalf("hoshi point %v out of bounds for 9x9", p)
		}
	}
}
