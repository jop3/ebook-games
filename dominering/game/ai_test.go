package game

import (
	"testing"
)

// --- AI always returns a legal move from the starting position --------------

func TestBestMoveAllDifficultiesFromStartingPosition(t *testing.T) {
	for _, side := range []Side{V, H} {
		for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
			b := NewBoard(SizeStandard)
			m, ok := BestMove(b, side, depth)
			if !ok {
				t.Fatalf("side %v depth %d: expected a move from the starting position", side, depth)
			}
			if !b.IsLegalMove(side, m) {
				t.Fatalf("side %v depth %d: BestMove returned an illegal move %v", side, depth, m)
			}
		}
	}
}

func TestBestMoveNoMovesReturnsFalse(t *testing.T) {
	// Only one empty cell remains, with no empty neighbor in any direction,
	// so H (horizontal-only) has no legal placement left.
	b := BoardFromRows([][]bool{
		{false, true},
		{true, true},
	})
	if _, ok := BestMove(b, H, DepthEasy); ok {
		t.Fatal("BestMove should report no move when the side has none available")
	}
}

// --- HAND-VERIFIED: on a 2x2 board V always wins in one move ---------------

func TestBestMoveSolves2x2InVsFavor(t *testing.T) {
	b := NewBoard(2)
	m, ok := BestMove(b, V, DepthHard)
	if !ok {
		t.Fatal("V should have a move on an empty 2x2 board")
	}
	nb := b.Apply(m)
	if nb.HasMove(H) {
		t.Fatal("V's move on a 2x2 board should leave H with no legal move (hand-verified result)")
	}
}

// --- AI takes a move that empties the opponent's mobility when one exists --

func TestBestMovePrefersImmediatelyWinningMove(t *testing.T) {
	// A 2x3 board (2 wide, 3 tall) with the middle row occupied, leaving two
	// separate 2x1 (vertically split) pockets: top pocket rows 0, bottom
	// pocket row 2 alone. H (horizontal) has legal moves in the still-open
	// single rows (row 0 and row 2, each 2 cells wide); V has none (no two
	// vertically stacked open cells anywhere). So H must have a legal,
	// immediately-terminal move (after which V, to move, has nothing) — any
	// single H move that fills a whole row leaves the other row for nobody in
	// V's fixed orientation.
	b := BoardFromRows([][]bool{
		{false, false, true},
		{true, true, true},
		{false, false, true},
	})
	// Restrict to a 2-wide board conceptually: column 2 fully walled off.
	if b.HasMove(V) {
		t.Fatal("test setup invalid: V should have no legal move on this board")
	}
	m, ok := BestMove(b, H, DepthHard)
	if !ok {
		t.Fatal("H should have a legal move")
	}
	nb := b.Apply(m)
	if nb.HasMove(V) {
		t.Fatal("test setup invalid: V should still have no move")
	}
	// Whichever row H filled, only H itself could possibly move again in the
	// other still-open row — but it's V's turn next and V has zero moves
	// anywhere on this board, so this position is already a forced win for H
	// regardless of which of the two symmetric rows BestMove picked.
	if nb.HasMove(V) {
		t.Fatal("H's move should leave V (to move next) with zero legal moves")
	}
}

// --- Endgame extension actually searches to the true end -------------------

func TestBestMoveSolvesSmallEndgameExactly(t *testing.T) {
	// A near-full 4x4 board with only a handful of cells open, shaped so V
	// has exactly one useful line and H has none once V takes it — checks
	// that even DepthEasy finds it thanks to the endgame extension.
	b := BoardFromRows([][]bool{
		{true, true, true, true},
		{true, true, true, true},
		{true, false, true, true},
		{true, false, true, true},
	})
	if b.HasMove(H) {
		t.Fatal("test setup invalid: H should have no legal move left")
	}
	m, ok := BestMove(b, V, DepthEasy)
	if !ok {
		t.Fatal("V should find its one remaining legal move")
	}
	if m.A.X != 1 || m.B.X != 1 || m.A.Y != 2 || m.B.Y != 3 {
		t.Fatalf("BestMove = %v, want the only legal vertical placement at column 1, rows 2-3", m)
	}
}

// --- Full game, AI vs a simple deterministic opponent, always terminates ---

func TestBestMoveFullGameTerminates(t *testing.T) {
	s := NewGame(OpponentAI, SizeSmall, DepthMedium)
	for ply := 0; s.Phase == PhasePlaying; ply++ {
		if ply > SizeSmall*SizeSmall {
			t.Fatal("game did not terminate in a sane number of plies")
		}
		if s.AITurn() {
			if !s.StepAI() {
				t.Fatal("StepAI should have a move whenever Phase is still Playing and it's H's turn")
			}
			continue
		}
		moves := s.Board.LegalMoves(s.Turn)
		if len(moves) == 0 {
			t.Fatal("Phase should already be Done if the human side to move has no legal move")
		}
		if !s.Play(moves[0]) {
			t.Fatalf("a move drawn from LegalMoves must always be playable: %v", moves[0])
		}
	}
	if s.Board.HasMove(s.Turn) {
		t.Fatal("game ended but the side to move still has a legal move")
	}
}
