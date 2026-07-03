package game

import "testing"

// TestAITakesImmediateWin verifies BestPlacement grabs a winning placement
// when one is available, rather than playing something safer-looking.
func TestAITakesImmediateWin(t *testing.T) {
	s := NewGame(ModeAI, 2)
	// Row 0 has three Tall pieces; hand the AI a 4th Tall piece plus a
	// non-winning alternative on the board layout — there's only one empty
	// cell in the row, so any correct search takes it.
	s.Board.Place(0, 0, AttrTall)
	s.Board.Place(1, 0, AttrTall|AttrDark)
	s.Board.Place(2, 0, AttrTall|AttrSquare)
	for _, p := range []Piece{AttrTall, AttrTall | AttrDark, AttrTall | AttrSquare} {
		s.removeFromPool(p)
	}
	winPiece := Piece(AttrTall | AttrSolid)
	s.removeFromPool(winPiece)
	s.ActivePiece = winPiece
	s.Step = StepPlace
	s.Turn = 1 // AI's action

	x, y := BestPlacement(s, 2)
	if x != 3 || y != 0 {
		t.Fatalf("expected AI to complete the winning row at (3,0), got (%d,%d)", x, y)
	}
}

// TestAIAvoidsHandingAWinningPiece verifies BestGive doesn't hand over a
// piece that lets the opponent complete an already-3/4-filled shared line
// when a safe alternative piece exists.
func TestAIAvoidsHandingAWinningPiece(t *testing.T) {
	s := NewGame(ModeAI, 2)
	// Row 0: three Dark pieces waiting for a 4th Dark piece to win.
	s.Board.Place(0, 0, AttrDark)
	s.Board.Place(1, 0, AttrDark|AttrTall)
	s.Board.Place(2, 0, AttrDark|AttrSquare)
	for _, p := range []Piece{AttrDark, AttrDark | AttrTall, AttrDark | AttrSquare} {
		s.removeFromPool(p)
	}
	s.Step = StepGive
	s.Turn = 1 // AI is choosing what to hand to the human

	dangerous := Piece(AttrDark | AttrSolid) // would let the human win at (3,0)
	given := BestGive(s, 2)
	if given == dangerous {
		t.Fatalf("AI handed over a piece (%v) that completes an existing shared line", given)
	}
}

// TestSearchDepthStaysBounded is a light sanity/perf check: BestPlacement and
// BestGive must return quickly (bounded search) even from the opening
// position, where the branching factor is largest.
func TestSearchDepthStaysBounded(t *testing.T) {
	s := NewGame(ModeAI, 2)
	s.Step = StepGive
	s.Turn = 1
	p := BestGive(s, 2)
	if p < 0 || p >= NumPieces {
		t.Fatalf("expected a valid piece from BestGive, got %v", p)
	}
}
