package game

import "testing"

func TestSevenBySevenDetectsAnyWindow(t *testing.T) {
	var b Board
	if b.SevenBySeven() {
		t.Fatal("empty board should not have a full 7x7 window")
	}
	// Fill the bottom-right 7x7 window (offset 2,2 .. 8,8).
	for y := 2; y < 9; y++ {
		for x := 2; x < 9; x++ {
			b.Filled[y][x] = true
		}
	}
	if !b.SevenBySeven() {
		t.Fatal("expected the filled bottom-right 7x7 window to be detected")
	}
}

func TestSevenBySevenRequiresFullWindowNotJustCount(t *testing.T) {
	var b Board
	// Fill 48 of 49 cells in the top-left 7x7 window, leaving one hole.
	for y := 0; y < 7; y++ {
		for x := 0; x < 7; x++ {
			if x == 3 && y == 3 {
				continue // the hole
			}
			b.Filled[y][x] = true
		}
	}
	if b.SevenBySeven() {
		t.Fatal("a 7x7 window with one empty cell must not count")
	}
}

func TestFilledAndEmptyCount(t *testing.T) {
	var b Board
	if got := b.EmptyCount(); got != BoardSize*BoardSize {
		t.Fatalf("EmptyCount on fresh board = %d, want %d", got, BoardSize*BoardSize)
	}
	b.Filled[0][0] = true
	b.Filled[3][4] = true
	if got := b.FilledCount(); got != 2 {
		t.Fatalf("FilledCount = %d, want 2", got)
	}
	if got := b.EmptyCount(); got != BoardSize*BoardSize-2 {
		t.Fatalf("EmptyCount = %d, want %d", got, BoardSize*BoardSize-2)
	}
}
