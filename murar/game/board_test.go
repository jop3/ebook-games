package game

import (
	"image"
	"testing"
)

func TestNewBoardStartingPositions(t *testing.T) {
	b := NewBoard()
	if b.Pawns[P1] != image.Pt(Size/2, Size-1) {
		t.Fatalf("P1 start = %v, want centered bottom row", b.Pawns[P1])
	}
	if b.Pawns[P2] != image.Pt(Size/2, 0) {
		t.Fatalf("P2 start = %v, want centered top row", b.Pawns[P2])
	}
	if b.WallsLeft[P1] != StartingWalls || b.WallsLeft[P2] != StartingWalls {
		t.Fatalf("WallsLeft = %v, want both %d", b.WallsLeft, StartingWalls)
	}
}

func TestGoalRowAndOpponent(t *testing.T) {
	if GoalRow(P1) != 0 {
		t.Fatalf("GoalRow(P1) = %d, want 0", GoalRow(P1))
	}
	if GoalRow(P2) != Size-1 {
		t.Fatalf("GoalRow(P2) = %d, want %d", GoalRow(P2), Size-1)
	}
	if P1.Opponent() != P2 || P2.Opponent() != P1 {
		t.Fatal("Opponent() must be an involution swapping P1/P2")
	}
}

func TestWallBetweenBlocksAdjacentEdge(t *testing.T) {
	var b Board
	// Vertical wall at (4,7) blocks the edge between (4,8)-(5,8) and (4,7)-(5,7).
	b.place(Wall{X: 4, Y: 7, Orient: Vertical})
	if !b.wallBetween(image.Pt(4, 8), image.Pt(5, 8)) {
		t.Fatal("vertical wall at (4,7) should block (4,8)-(5,8)")
	}
	if !b.wallBetween(image.Pt(4, 7), image.Pt(5, 7)) {
		t.Fatal("vertical wall at (4,7) should also block (4,7)-(5,7)")
	}
	if b.wallBetween(image.Pt(4, 6), image.Pt(5, 6)) {
		t.Fatal("vertical wall at (4,7) must not block row 6")
	}
	if b.wallBetween(image.Pt(4, 8), image.Pt(4, 7)) {
		t.Fatal("a vertical wall must not block vertical movement")
	}
}

func TestWallBetweenHorizontal(t *testing.T) {
	var b Board
	// Horizontal wall at (0,7) blocks (0,7)-(0,8) and (1,7)-(1,8).
	b.place(Wall{X: 0, Y: 7, Orient: Horizontal})
	if !b.wallBetween(image.Pt(0, 7), image.Pt(0, 8)) {
		t.Fatal("horizontal wall at (0,7) should block (0,7)-(0,8)")
	}
	if !b.wallBetween(image.Pt(1, 7), image.Pt(1, 8)) {
		t.Fatal("horizontal wall at (0,7) should also block (1,7)-(1,8)")
	}
	if b.wallBetween(image.Pt(2, 7), image.Pt(2, 8)) {
		t.Fatal("horizontal wall at (0,7) must not block column 2")
	}
	if b.wallBetween(image.Pt(0, 7), image.Pt(1, 7)) {
		t.Fatal("a horizontal wall must not block horizontal movement")
	}
}
