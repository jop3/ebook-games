package game

import (
	"image"
	"testing"
)

// --- WIN: the exact "zero legal moves = immediate loss" condition ----------

func winnerTestBoard() (Board, image.Point) {
	// A board where Black's pawn is completely walled in by missing tiles
	// (every queen-ray destination from Black's square removed), while White
	// still has plenty of room, so it is unambiguously Black's own lack of
	// moves — not White's presence — that traps it.
	var b Board
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			b.Present[y][x] = true
		}
	}
	b.BlackPawn = image.Pt(0, 0)
	b.WhitePawn = image.Pt(7, 7)
	// Remove every one of Black's 3 queen destinations from the corner
	// (right, down, down-right diagonal).
	b.Present[0][1] = false
	b.Present[1][0] = false
	b.Present[1][1] = false
	return b, image.Pt(0, 0)
}

func TestGameOverAndWinner(t *testing.T) {
	b, _ := winnerTestBoard()
	if len(b.LegalMoves(Black)) != 0 {
		t.Fatalf("setup failed: Black should have zero legal moves, got %v", b.LegalMoves(Black))
	}
	if !GameOver(&b, Black) {
		t.Fatal("GameOver(Black) should be true: Black has zero legal moves")
	}
	if GameOver(&b, White) {
		t.Fatal("GameOver(White) should be false: White still has legal moves")
	}
	if w := Winner(&b, Black); w != White {
		t.Fatalf("Winner when Black is to move and stuck = %v, want White", w)
	}
	if w := Winner(&b, White); w != Empty {
		t.Fatalf("Winner when White is to move (and can move) = %v, want Empty", w)
	}
}
