package game

import "testing"

// --- Fångst (reduce-to-1) win condition -------------------------------------

func TestWinnerCaptureMode(t *testing.T) {
	cases := []struct {
		name      string
		black, wh int
		want      Cell
	}{
		{"both_full", 9, 9, Empty},
		{"white_reduced_to_1", 5, 1, Black},
		{"black_reduced_to_1", 1, 5, White},
		{"white_reduced_to_0", 5, 0, Black},
		{"black_reduced_to_0", 0, 5, White},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := emptyBoard()
			n := 0
			for y := 0; y < Size && n < tc.black; y++ {
				for x := 0; x < Size && n < tc.black; x++ {
					b.set(x, y, Black)
					n++
				}
			}
			n = 0
			for y := Size - 1; y >= 0 && n < tc.wh; y-- {
				for x := Size - 1; x >= 0 && n < tc.wh; x-- {
					if b.At(x, y) == Empty {
						b.set(x, y, White)
						n++
					}
				}
			}
			if got := Winner(&b, ModeCapture); got != tc.want {
				t.Fatalf("Winner = %v, want %v (black=%d white=%d)", got, tc.want, tc.black, tc.wh)
			}
		})
	}
}

// --- GOTCHA: Fem-i-rad must exclude the owner's home rank -------------------

func TestFiveInRowExcludesHomeRankAtStart(t *testing.T) {
	b := NewBoard()
	// The starting position already has 9-in-a-row for both colors on their
	// own home ranks; this must NOT trivially count as a win.
	if w := Winner(&b, ModeFiveInRow); w != Empty {
		t.Fatalf("Winner(FiveInRow) at the starting position = %v, want Empty", w)
	}
}

func TestFiveInRowHorizontalOutsideHomeRank(t *testing.T) {
	b := emptyBoard()
	for x := 0; x < 5; x++ {
		b.set(x, 4, Black) // row 4 is nobody's home rank
	}
	if w := Winner(&b, ModeFiveInRow); w != Black {
		t.Fatalf("Winner = %v, want Black (5 in a row on a neutral rank)", w)
	}
}

func TestFiveInRowVerticalOutsideHomeRank(t *testing.T) {
	b := emptyBoard()
	for y := 1; y <= 5; y++ {
		b.set(3, y, White)
	}
	if w := Winner(&b, ModeFiveInRow); w != White {
		t.Fatalf("Winner = %v, want White (5 in a column)", w)
	}
}

func TestFiveInRowHorizontalOnHomeRankDoesNotCount(t *testing.T) {
	b := emptyBoard()
	for x := 0; x < 5; x++ {
		b.set(x, Size-1, Black) // Black's own home rank
	}
	if w := Winner(&b, ModeFiveInRow); w != Empty {
		t.Fatalf("Winner = %v, want Empty (a line confined to Black's home rank must not count)", w)
	}
}

func TestFiveInRowVerticalTouchingHomeRankExcludesThatCell(t *testing.T) {
	// A vertical run of 4 immediately above Black's home rank (rows 4-7) plus
	// the home-rank cell itself (row 8) would be 5 cells tall, but the home
	// rank cell must not count toward the line.
	b := emptyBoard()
	for y := 4; y <= Size-1; y++ { // rows 4,5,6,7,8 — 5 cells including the home rank
		b.set(2, y, Black)
	}
	if w := Winner(&b, ModeFiveInRow); w != Empty {
		t.Fatalf("Winner = %v, want Empty (only 4 of the 5 cells are outside the home rank)", w)
	}
	// Extend one further off the home rank (rows 3-7) — now a genuine 5 cells
	// entirely outside the home rank.
	b.set(2, 3, Black)
	if w := Winner(&b, ModeFiveInRow); w != Black {
		t.Fatalf("Winner = %v, want Black (5 cells now entirely outside the home rank)", w)
	}
}

func TestFiveInRowBrokenRunDoesNotCount(t *testing.T) {
	b := emptyBoard()
	for x := 0; x < 4; x++ {
		b.set(x, 4, Black)
	}
	// gap at x=4
	b.set(5, 4, Black)
	if w := Winner(&b, ModeFiveInRow); w != Empty {
		t.Fatalf("Winner = %v, want Empty (the run is broken by a gap)", w)
	}
}
