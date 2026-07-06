package game

import (
	"image"
	"testing"
)

func emptyBoard() Board {
	var b Board
	return b
}

func TestNewBoardStartingPosition(t *testing.T) {
	b := NewBoard()
	if n := b.Count(Attacker); n != StartAttackers {
		t.Fatalf("Attacker count = %d, want %d", n, StartAttackers)
	}
	if n := b.Count(Defender); n != StartDefenders {
		t.Fatalf("Defender count = %d, want %d", n, StartDefenders)
	}
	if n := b.Count(King); n != 1 {
		t.Fatalf("King count = %d, want 1", n)
	}
	kp, ok := b.KingPos()
	if !ok || kp != throneCell {
		t.Fatalf("KingPos = %v,%v, want the throne %v", kp, ok, throneCell)
	}
	if b.At(throneCell.X, throneCell.Y) != King {
		t.Fatal("the king should start on the throne")
	}
	// Defenders orthogonally adjacent to the throne.
	for _, d := range dirs4 {
		x, y := throneCell.X+d.X, throneCell.Y+d.Y
		if b.At(x, y) != Defender {
			t.Fatalf("(%d,%d) = %v, want Defender adjacent to the throne", x, y, b.At(x, y))
		}
	}
}

func TestRookMoveAnyDistance(t *testing.T) {
	b := emptyBoard()
	b.set(1, 1, Attacker)
	dests := b.DestinationsFrom(image.Pt(1, 1), false)
	// Row y=1 and column x=1 neither pass through the throne (3,3) nor any
	// corner, so a piece at (1,1) reaches every other cell on its row and
	// column: the full (Size-1)*2 destinations.
	want := (Size - 1) * 2
	if len(dests) != want {
		t.Fatalf("got %d destinations, want %d: %v", len(dests), want, dests)
	}
}

func TestNonKingCannotStopOnOrPassThroughThrone(t *testing.T) {
	b := emptyBoard()
	b.set(3, 0, Attacker) // column 3 runs straight through the throne at (3,3)
	dests := b.DestinationsFrom(image.Pt(3, 0), false)
	var vertical []image.Point
	for _, d := range dests {
		if IsThrone(d.X, d.Y) {
			t.Fatalf("non-king destinations must never include the throne, got %v", dests)
		}
		if d.X == 3 {
			vertical = append(vertical, d)
			if d.Y >= 3 {
				t.Fatalf("a non-king piece must not pass through the throne to reach %v", d)
			}
		}
	}
	// Going down column 3, only (3,1) and (3,2) are reachable before the
	// throne at (3,3) blocks the ray entirely (row y=0's sideways moves are
	// unaffected and are not the point of this test).
	want := []image.Point{{X: 3, Y: 1}, {X: 3, Y: 2}}
	if !ptsEqual(vertical, want) {
		t.Fatalf("vertical dests = %v, want %v", vertical, want)
	}
}

func TestNonKingCannotStopOnOrPassThroughCorner(t *testing.T) {
	b := emptyBoard()
	b.set(0, 3, Attacker) // row 3 runs through the corner (0,0)? no: row y=3.
	// Use column instead: piece at (0,3) moving up column 0 passes through
	// no corner (corners are (0,0) and (0,6)). Move up: reaches (0,2),(0,1)
	// then corner (0,0) blocks. Move down: reaches (0,4),(0,5) then corner
	// (0,6) blocks.
	dests := b.DestinationsFrom(image.Pt(0, 3), false)
	for _, d := range dests {
		if IsCorner(d.X, d.Y) {
			t.Fatalf("non-king destinations must never include a corner, got %v", dests)
		}
	}
	wantUp := []image.Point{{X: 0, Y: 2}, {X: 0, Y: 1}}
	wantDown := []image.Point{{X: 0, Y: 4}, {X: 0, Y: 5}}
	for _, w := range append(wantUp, wantDown...) {
		found := false
		for _, d := range dests {
			if d == w {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected %v reachable, dests=%v", w, dests)
		}
	}
}

func TestKingMayStopOnThroneAndCorner(t *testing.T) {
	b := emptyBoard()
	b.set(3, 0, King) // column 3, straight through the throne
	dests := b.DestinationsFrom(image.Pt(3, 0), true)
	foundThrone := false
	for _, d := range dests {
		if d == throneCell {
			foundThrone = true
		}
	}
	if !foundThrone {
		t.Fatalf("the king should be able to land on the throne, dests=%v", dests)
	}
	// The king's ray stops AT the throne (does not continue past it in this
	// same move) — a documented simplification.
	for _, d := range dests {
		if d.Y > throneCell.Y {
			t.Fatalf("the king should not continue past the throne in one move, got %v", d)
		}
	}

	b2 := emptyBoard()
	b2.set(0, 3, King) // row 3, straight through corner (0,0) going up
	dests2 := b2.DestinationsFrom(image.Pt(0, 3), true)
	foundCorner := false
	for _, d := range dests2 {
		if d == (image.Point{X: 0, Y: 0}) {
			foundCorner = true
		}
	}
	if !foundCorner {
		t.Fatalf("the king should be able to land on a corner, dests=%v", dests2)
	}
}

func TestRookCannotJumpOverAPiece(t *testing.T) {
	b := emptyBoard()
	b.set(0, 4, Attacker)
	b.set(3, 4, Defender) // blocks the row beyond x=3
	if b.IsLegalMove(SideAttacker, Move{From: image.Pt(0, 4), To: image.Pt(3, 4)}) {
		t.Fatal("moving onto an occupied square must be illegal")
	}
	if b.IsLegalMove(SideAttacker, Move{From: image.Pt(0, 4), To: image.Pt(5, 4)}) {
		t.Fatal("jumping over the Defender at x=3 must be illegal")
	}
	if !b.IsLegalMove(SideAttacker, Move{From: image.Pt(0, 4), To: image.Pt(2, 4)}) {
		t.Fatal("moving up to (but not through) the blocker should be legal")
	}
}

func TestDiagonalMoveIsIllegal(t *testing.T) {
	b := emptyBoard()
	b.set(1, 1, Attacker)
	if b.IsLegalMove(SideAttacker, Move{From: image.Pt(1, 1), To: image.Pt(3, 3)}) {
		t.Fatal("a diagonal move must be illegal — pieces move like rooks, not bishops")
	}
}

func TestCannotMoveOntoOwnOrEnemyPiece(t *testing.T) {
	b := emptyBoard()
	b.set(1, 1, Attacker)
	b.set(1, 3, Attacker)
	b.set(1, 5, Defender)
	if b.IsLegalMove(SideAttacker, Move{From: image.Pt(1, 1), To: image.Pt(1, 3)}) {
		t.Fatal("moving onto your own piece must be illegal")
	}
	if b.IsLegalMove(SideAttacker, Move{From: image.Pt(1, 1), To: image.Pt(1, 5)}) {
		t.Fatal("moving onto an enemy piece must be illegal")
	}
}

func TestMovingSomeoneElsesPieceIsIllegal(t *testing.T) {
	b := emptyBoard()
	b.set(1, 1, Defender)
	if b.IsLegalMove(SideAttacker, Move{From: image.Pt(1, 1), To: image.Pt(1, 4)}) {
		t.Fatal("attackers cannot move a Defender")
	}
	b.set(1, 1, King)
	if b.IsLegalMove(SideAttacker, Move{From: image.Pt(1, 1), To: image.Pt(1, 4)}) {
		t.Fatal("attackers cannot move the King")
	}
}

func TestDefenderSideOwnsBothDefendersAndKing(t *testing.T) {
	b := emptyBoard()
	b.set(1, 1, Defender)
	b.set(4, 4, King)
	if !b.IsLegalMove(SideDefender, Move{From: image.Pt(1, 1), To: image.Pt(1, 4)}) {
		t.Fatal("the defender side must be able to move an ordinary Defender")
	}
	if !b.IsLegalMove(SideDefender, Move{From: image.Pt(4, 4), To: image.Pt(4, 5)}) {
		t.Fatal("the defender side must be able to move the King")
	}
}

func TestLegalMovesFromStartingPosition(t *testing.T) {
	b := NewBoard()
	moves := b.LegalMoves(SideAttacker)
	if len(moves) == 0 {
		t.Fatal("attackers should have legal moves at the start")
	}
	for _, m := range moves {
		if b.At(m.From.X, m.From.Y) != Attacker {
			t.Fatalf("move %v does not originate on an Attacker", m)
		}
		if b.At(m.To.X, m.To.Y) != Empty {
			t.Fatalf("move %v does not land on an empty cell", m)
		}
		if !b.IsLegalMove(SideAttacker, m) {
			t.Fatalf("LegalMoves produced a move IsLegalMove rejects: %v", m)
		}
	}
	defMoves := b.LegalMoves(SideDefender)
	if len(defMoves) == 0 {
		t.Fatal("defenders should have legal moves at the start")
	}
}

func ptsEqual(a, b []image.Point) bool {
	if len(a) != len(b) {
		return false
	}
	used := make([]bool, len(b))
	for _, p := range a {
		found := false
		for i, q := range b {
			if !used[i] && p == q {
				used[i] = true
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
