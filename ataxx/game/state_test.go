package game

import (
	"image"
	"testing"
)

func TestNewGameStartingState(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	if s.Turn != Black {
		t.Fatalf("Turn = %v, want Black to start", s.Turn)
	}
	if s.Phase != PhasePlaying {
		t.Fatalf("Phase = %v, want PhasePlaying", s.Phase)
	}
	if s.Board.Count(Black) != 2 || s.Board.Count(White) != 2 {
		t.Fatal("starting position should have 2 men per side")
	}
}

func TestPlayLegalMoveAdvancesTurn(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	legal := s.Board.LegalMoves(Black)
	if len(legal) == 0 {
		t.Fatal("Black should have legal moves at the start")
	}
	m := legal[0]
	want, wantFlipped := s.Board.Apply(m)
	if !s.Play(m.From, m.To) {
		t.Fatalf("legal move %v was rejected", m)
	}
	if s.Board != want {
		t.Fatal("GameState.Play did not match a pure Board.Apply on the same move")
	}
	if len(s.LastFlipped) != len(wantFlipped) {
		t.Fatalf("LastFlipped = %v, want %v", s.LastFlipped, wantFlipped)
	}
	if s.Turn != White {
		t.Fatal("turn should pass to White after Black's legal move")
	}
}

func TestPlayRejectsIllegalMove(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	before := s.Board
	// (3,3) is empty at the start; moving FROM an empty cell is illegal.
	if s.Play(image.Pt(3, 3), image.Pt(3, 4)) {
		t.Fatal("moving from an empty cell should be rejected")
	}
	if s.Board != before {
		t.Fatal("a rejected move must not change the board")
	}
	if s.Turn != Black {
		t.Fatal("a rejected move must not change the turn")
	}
}

func TestPlayRejectsAfterGameOver(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	s.Phase = PhaseDone
	if s.Play(image.Pt(0, 0), image.Pt(1, 0)) {
		t.Fatal("Play must reject any move once the game is over")
	}
}

// --- END CONDITION: board fills -------------------------------------------

func TestAdvanceEndsGameWhenBoardFills(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	// Fill the board except for one cell reachable by a legal clone, so the
	// final move both completes the board and ends the game.
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			s.Board.set(x, y, Black)
		}
	}
	s.Board.set(3, 3, Empty)
	s.Board.set(3, 4, Black) // adjacent to the empty cell: legal clone source
	s.Turn = Black
	if !s.Play(image.Pt(3, 4), image.Pt(3, 3)) {
		t.Fatal("the board-completing move should be legal")
	}
	if !s.Board.IsFull() {
		t.Fatal("setup error: board should be full after the move")
	}
	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done once the board fills")
	}
}

// --- END CONDITION: side to move has no legal move (but has pieces) -------

func TestAdvanceEndsGameWhenNextSideHasNoLegalMove(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			s.Board.set(x, y, Empty)
		}
	}
	// White has one man, boxed in by Black on every reachable (distance 1
	// and 2) cell so it truly has no legal move, while empty cells remain
	// elsewhere on the board (so this is NOT a board-full ending).
	s.Board.set(0, 0, White)
	blockReachableFrom(&s.Board, []image.Point{{X: 0, Y: 0}}, Black)
	s.Turn = Black // advance() will check White (Turn's opponent) next
	if len(s.Board.LegalMoves(White)) != 0 {
		t.Fatalf("setup error: White should have zero legal moves, got %v", s.Board.LegalMoves(White))
	}
	if s.Board.IsFull() {
		t.Fatal("setup error: board should not be full")
	}

	// advance() is what Play/StepAI call after applying a move; calling it
	// directly here isolates the end-condition logic from having to also
	// find and apply a legal Black move first (TestPlayLegalMoveAdvancesTurn
	// already covers the normal Play path end-to-end).
	s.advance()

	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done once White (to move) has no legal move")
	}
}

// blockReachableFrom sets blocker on every currently-empty cell within
// Chebyshev distance 1 or 2 of any of the given sources, leaving the sources
// themselves and everything else untouched. Used to construct a position
// where a side is provably stuck (zero legal moves) without hand-listing
// every blocked coordinate.
func blockReachableFrom(b *Board, sources []image.Point, blocker Cell) {
	for _, src := range sources {
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				d := chebyshev(dx, dy)
				if d != 1 && d != 2 {
					continue
				}
				x, y := src.X+dx, src.Y+dy
				if inBounds(x, y) && b.At(x, y) == Empty {
					b.set(x, y, blocker)
				}
			}
		}
	}
}

// --- No forced loss for running out of moves: the count decides, and a tie
// is possible --------------------------------------------------------------

func TestGameOverWinnerIsCountBasedNotAutomaticLoss(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	// White owns almost the entire board; Black holds only a small pocket in
	// the bottom-right corner, one cell of which is left empty. The empty
	// cell's full Chebyshev-distance-2 neighborhood, clipped to the board,
	// is exactly that Black pocket -- no White man anywhere is within reach
	// of the only empty cell, so White (vastly ahead on material) ends up
	// with precisely zero legal moves. This demonstrates that running out of
	// moves is NOT an automatic loss in Ataxx (unlike Hasami's stalemate
	// rule): the side with more men still wins even though it's the one
	// stuck.
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			s.Board.set(x, y, White)
		}
	}
	for y := 4; y < Size; y++ {
		for x := 4; x < Size; x++ {
			s.Board.set(x, y, Black)
		}
	}
	s.Board.set(Size-1, Size-1, Empty)
	s.Turn = Black // advance() will check White (Turn's opponent) next

	if len(s.Board.LegalMoves(White)) != 0 {
		t.Fatalf("setup error: White must have zero legal moves, got %v", s.Board.LegalMoves(White))
	}
	if s.Board.IsFull() {
		t.Fatal("setup error: board should not be full (this test is about the no-legal-move ending)")
	}
	if s.Board.Count(White) <= s.Board.Count(Black) {
		t.Fatalf("setup error: White (%d) must outnumber Black (%d) for this test to be meaningful",
			s.Board.Count(White), s.Board.Count(Black))
	}

	s.advance()

	if s.Phase != PhaseDone {
		t.Fatal("game should be over: White (to move) has no legal move")
	}
	if s.Winner() != White {
		t.Fatalf("Winner() = %v, want White: White has far more men even though it ran out of moves", s.Winner())
	}
}

func TestStepAIPlaysAndAdvances(t *testing.T) {
	s := NewGame(OpponentAI, DepthEasy)
	// Hand White the move directly to exercise StepAI in isolation.
	s.Turn = White
	before := s.Board
	if !s.StepAI() {
		t.Fatal("StepAI should play a move when it is White's (the AI's) turn")
	}
	if s.Board == before {
		t.Fatal("StepAI did not change the board")
	}
	if s.Turn != Black && s.Phase == PhasePlaying {
		t.Fatal("after the AI's move, turn should pass back to Black (unless the game just ended)")
	}
}

func TestStepAIDoesNothingWhenNotAITurn(t *testing.T) {
	s := NewGame(OpponentHotseat, 0) // hotseat: AITurn() is always false
	before := s.Board
	if s.StepAI() {
		t.Fatal("StepAI must do nothing in hotseat mode")
	}
	if s.Board != before {
		t.Fatal("StepAI must not touch the board when it is not the AI's turn")
	}
}

func TestAITurnOnlyForAIOpponentAndWhite(t *testing.T) {
	s := NewGame(OpponentAI, DepthEasy)
	if s.AITurn() {
		t.Fatal("AITurn should be false on Black's (the human's) turn")
	}
	s.Turn = White
	if !s.AITurn() {
		t.Fatal("AITurn should be true once it's White's turn in AI mode")
	}
	s.Phase = PhaseDone
	if s.AITurn() {
		t.Fatal("AITurn should be false once the game is over")
	}
}
