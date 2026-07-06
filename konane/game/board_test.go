package game

import "testing"

func TestNewBoardFullyFilledCheckerboard(t *testing.T) {
	b := NewBoard()
	if got := b.Count(Black) + b.Count(White); got != Size*Size {
		t.Fatalf("board should start completely full, got %d of %d cells occupied", got, Size*Size)
	}
	if got := b.Count(Empty); got != 0 {
		t.Fatalf("fresh board should have zero empty cells, got %d", got)
	}
	if bc, wc := b.Count(Black), b.Count(White); bc != wc {
		t.Fatalf("checkerboard fill should split evenly, got Black=%d White=%d", bc, wc)
	}
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			want := Black
			if (x+y)%2 != 0 {
				want = White
			}
			if got := b.At(x, y); got != want {
				t.Fatalf("At(%d,%d)=%v, want %v (checkerboard pattern)", x, y, got, want)
			}
		}
	}
}

func TestBoardAtOutOfBoundsIsEmpty(t *testing.T) {
	b := NewBoard()
	cases := []struct{ x, y int }{
		{-1, 0}, {0, -1}, {Size, 0}, {0, Size}, {-1, -1}, {Size, Size},
	}
	for _, c := range cases {
		if got := b.At(c.x, c.y); got != Empty {
			t.Errorf("At(%d,%d) = %v, want Empty (out of bounds)", c.x, c.y, got)
		}
	}
}

func TestCellOpponent(t *testing.T) {
	if Black.Opponent() != White {
		t.Fatal("Black.Opponent() should be White")
	}
	if White.Opponent() != Black {
		t.Fatal("White.Opponent() should be Black")
	}
	if Empty.Opponent() != Empty {
		t.Fatal("Empty.Opponent() should be Empty (not a meaningful side)")
	}
}
