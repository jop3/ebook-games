package game

import "testing"

func TestEmptyBoardNoWinner(t *testing.T) {
	b := NewBoard(5)
	if b.Winner() != Empty {
		t.Fatal("empty board has no winner")
	}
}

func TestBlackVerticalConnection(t *testing.T) {
	// Black connects top (y=0) to bottom (y=N-1) via a straight column.
	b := NewBoard(5)
	for y := 0; y < 5; y++ {
		b.Set(2, y, Black)
	}
	if b.Winner() != Black {
		t.Fatal("straight Black column top-to-bottom should win")
	}
}

func TestWhiteHorizontalConnection(t *testing.T) {
	b := NewBoard(5)
	for x := 0; x < 5; x++ {
		b.Set(x, 2, White)
	}
	if b.Winner() != White {
		t.Fatal("straight White row left-to-right should win")
	}
}

func TestDiagonalNeighborConnection(t *testing.T) {
	// Hex neighbors include (1,-1) and (-1,1). A zig-zag using them should still
	// connect. Build a Black path from top to bottom using diagonal steps.
	b := NewBoard(4)
	// path: (0,0)-(0,1)-(1,0)? ensure adjacency via defined neighbors.
	// Use column with one diagonal jog.
	b.Set(1, 0, Black)
	b.Set(1, 1, Black)
	b.Set(0, 2, Black) // neighbor of (1,1)? (1,1)+(-1,1)=(0,2) yes
	b.Set(0, 3, Black) // (0,2)+(0,1)=(0,3) reaches bottom
	if b.Winner() != Black {
		t.Fatal("diagonal Black path should connect top to bottom")
	}
}

func TestPlaceRejectsOccupied(t *testing.T) {
	b := NewBoard(3)
	if !b.Place(1, 1, Black) {
		t.Fatal("first placement should succeed")
	}
	if b.Place(1, 1, White) {
		t.Fatal("occupied cell should reject placement")
	}
	if b.Place(-1, 0, Black) {
		t.Fatal("out of bounds should reject")
	}
}
