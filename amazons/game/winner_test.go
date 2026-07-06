package game

import (
	"image"
	"testing"
)

func TestWinnerNoWinnerOnStartingBoard(t *testing.T) {
	b := NewBoard()
	if w := Winner(&b, Black); w != Empty {
		t.Fatalf("Winner on the starting board = %v, want Empty (nobody has won yet)", w)
	}
	if w := Winner(&b, White); w != Empty {
		t.Fatalf("Winner on the starting board = %v, want Empty (nobody has won yet)", w)
	}
}

// boxInQueen surrounds a single queen at p with Burned squares in all 8
// directions immediately adjacent, so it (and, if it's the only queen of its
// side on the board, that whole side) has no legal move at all.
func boxInQueen(b *Board, p image.Point) {
	for _, d := range dirs8 {
		x, y := p.X+d.X, p.Y+d.Y
		if inBounds(x, y) {
			b.set(x, y, Burned)
		}
	}
}

func TestSideHasMoveFalseWhenFullyBoxedIn(t *testing.T) {
	var b Board
	b.set(4, 4, QueenBlack)
	boxInQueen(&b, image.Pt(4, 4))
	if b.SideHasMove(Black) {
		t.Fatal("a queen surrounded on all 8 sides by burned squares should have no legal move")
	}
}

func TestSideHasMoveFalseAtBoardCorner(t *testing.T) {
	var b Board
	b.set(0, 0, QueenBlack)
	// A queen in the corner only has 3 directions to begin with (right,
	// down, down-right diagonal); burn those.
	b.set(1, 0, Burned)
	b.set(0, 1, Burned)
	b.set(1, 1, Burned)
	if b.SideHasMove(Black) {
		t.Fatal("a corner queen with its only 3 rays burned should have no legal move")
	}
}

func TestWinnerWhenSideToMoveIsFullyBoxedIn(t *testing.T) {
	var b Board
	b.set(4, 4, QueenBlack)
	boxInQueen(&b, image.Pt(4, 4))
	b.set(0, 0, QueenWhite) // White still has a queen with plenty of room

	if w := Winner(&b, Black); w != QueenWhite {
		t.Fatalf("Winner(sideToMove=Black boxed in) = %v, want QueenWhite (Black cannot move, so White — the last to have moved — wins)", w)
	}
	// From White's perspective (White still has moves), nobody has won yet.
	if w := Winner(&b, White); w != Empty {
		t.Fatalf("Winner(sideToMove=White, White still has moves) = %v, want Empty", w)
	}
}

// TestMoveAlwaysHasAFollowUpShot empirically backs the reasoning documented
// on SideHasMove: any queen that CAN move can always complete the shoot half
// of the turn too (at worst, by shooting back through the square it just
// vacated) — so "no legal move" alone is the correct, sufficient losing
// condition, never "has a move but literally cannot shoot afterward".
func TestMoveAlwaysHasAFollowUpShot(t *testing.T) {
	var b Board
	// Box the queen in almost completely, leaving exactly one square it can
	// step to, which is itself a dead end in every other direction.
	b.set(4, 4, QueenBlack)
	for _, d := range dirs8 {
		x, y := 4+d.X, 4+d.Y
		if d == (image.Point{X: 1, Y: 0}) {
			continue // leave the single square (5,4) reachable
		}
		b.set(x, y, Burned)
	}
	// Also seal off (5,4) itself except back toward (4,4).
	for _, d := range dirs8 {
		x, y := 5+d.X, 4+d.Y
		if x == 4 && y == 4 {
			continue // that's the origin square, must stay Empty after the move
		}
		if inBounds(x, y) {
			b.set(x, y, Burned)
		}
	}

	moves := b.LegalQueenMoves(Black)
	if len(moves) != 1 || moves[0].To != (image.Point{X: 5, Y: 4}) {
		t.Fatalf("expected exactly one legal move to (5,4), got %v", moves)
	}
	after := b.MoveQueen(moves[0])
	shots := after.DestinationsFrom(moves[0].To)
	if len(shots) != 1 || shots[0] != (image.Point{X: 4, Y: 4}) {
		t.Fatalf("expected exactly one legal shot, back to the vacated origin (4,4); got %v", shots)
	}
}
