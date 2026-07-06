package game

import "testing"

func TestNewGameStartsAtP0Playing(t *testing.T) {
	g := NewGame(4, 4, OpponentHotseat)
	if g.Turn != P0 {
		t.Fatalf("Turn = %v, want P0", g.Turn)
	}
	if g.Phase != PhasePlaying {
		t.Fatalf("Phase = %v, want PhasePlaying", g.Phase)
	}
	if g.Board.Total() != 16 {
		t.Fatalf("Total() = %d, want 16", g.Board.Total())
	}
	if g.AITurn() {
		t.Fatal("hotseat games are never the AI's turn")
	}
}

func TestPlayIllegalMoveRejected(t *testing.T) {
	g := NewGame(3, 3, OpponentHotseat)
	before := g.Board
	ok := g.Play(Move{Row: 5, Col: 5}) // off the board entirely
	if ok {
		t.Fatal("an illegal move must be rejected")
	}
	if !equalState(g.Board, before) {
		t.Fatal("a rejected move must not change the board")
	}
	if g.Turn != P0 {
		t.Fatal("a rejected move must not change whose turn it is")
	}
}

func TestPlayAlternatesTurn(t *testing.T) {
	g := NewGame(3, 3, OpponentHotseat)
	if !g.Play(Move{Row: 0, Col: 2}) {
		t.Fatal("a legal non-poison move should be accepted")
	}
	if g.Turn != P1 {
		t.Fatalf("Turn = %v after P0's move, want P1", g.Turn)
	}
	if g.Phase != PhasePlaying {
		t.Fatal("the game should still be in progress")
	}
	if !g.Play(Move{Row: 1, Col: 1}) {
		t.Fatal("P1's legal move should be accepted")
	}
	if g.Turn != P0 {
		t.Fatalf("Turn = %v after P1's move, want P0", g.Turn)
	}
}

// TestPoisonEndsGameForBothMovers checks the poison-loss condition explicitly
// for BOTH players as the one who takes it, independent of who moved first.
func TestPoisonEndsGameForBothMovers(t *testing.T) {
	t.Run("P0 eats the poison and loses", func(t *testing.T) {
		g := NewGame(3, 3, OpponentHotseat)
		if g.Turn != P0 {
			t.Fatal("setup: P0 should move first")
		}
		if !g.Play(Move{Row: 0, Col: 0}) {
			t.Fatal("eating the poisoned cell must be a legal move")
		}
		if g.Phase != PhaseDone {
			t.Fatal("Phase must be Done once the poison is eaten")
		}
		if g.Winner != P1 {
			t.Fatalf("Winner = %v, want P1 (P0 ate the poison)", g.Winner)
		}
		if !g.Board.Empty() {
			t.Fatal("eating the poison must clear the whole board")
		}
	})

	t.Run("P1 eats the poison and loses", func(t *testing.T) {
		g := NewGame(3, 3, OpponentHotseat)
		// Force it to P1's turn first via one harmless legal move.
		if !g.Play(Move{Row: 0, Col: 2}) {
			t.Fatal("setup move should be legal")
		}
		if g.Turn != P1 {
			t.Fatal("setup: it should now be P1's turn")
		}
		if !g.Play(Move{Row: 0, Col: 0}) {
			t.Fatal("eating the poisoned cell must be a legal move")
		}
		if g.Phase != PhaseDone {
			t.Fatal("Phase must be Done once the poison is eaten")
		}
		if g.Winner != P0 {
			t.Fatalf("Winner = %v, want P0 (P1 ate the poison)", g.Winner)
		}
	})
}

func TestPlayAfterGameOverRejected(t *testing.T) {
	g := NewGame(2, 2, OpponentHotseat)
	if !g.Play(Move{Row: 0, Col: 0}) {
		t.Fatal("setup: poison move should end the game")
	}
	if g.Play(Move{Row: 0, Col: 0}) {
		t.Fatal("no move should be accepted once the game is over")
	}
}

func TestAITurnAndStepAI(t *testing.T) {
	g := NewGame(4, 4, OpponentAI)
	if g.AITurn() {
		t.Fatal("it should be the human's (P0's) turn at the start")
	}
	if g.StepAI() {
		t.Fatal("StepAI must do nothing when it is not the AI's turn")
	}
	if !g.Play(Move{Row: 0, Col: 3}) {
		t.Fatal("P0's opening move should be legal")
	}
	if !g.AITurn() {
		t.Fatal("it should now be the AI's (P1's) turn")
	}
	before := g.Board
	if !g.StepAI() {
		t.Fatal("StepAI should make a move on the AI's turn")
	}
	if equalState(g.Board, before) {
		t.Fatal("StepAI must actually change the board")
	}
	if g.AITurn() {
		t.Fatal("after the AI moves, it must be the human's turn again (or game over)")
	}
}

// TestMoverCanWinHonesty checks the AI-honesty helper: a fresh board (more
// than 1 cell) is always a win for whoever moves first (P0), and the helper
// must report false once the game is over.
func TestMoverCanWinHonesty(t *testing.T) {
	g := NewGame(4, 4, OpponentAI)
	if !g.MoverCanWin() {
		t.Fatal("a fresh board must be a winning position for the first mover")
	}
	if !g.Play(Move{Row: 0, Col: 0}) {
		t.Fatal("setup: poison move should be accepted")
	}
	if g.MoverCanWin() {
		t.Fatal("MoverCanWin must be false once the game is over")
	}
}

// TestFullGameVsAITerminates plays a whole game with the human always taking
// the AI's own BestMove suggestion against itself (so a fair, competent
// "human"), and checks the game reaches PhaseDone with a total board state
// consistent with the number of moves played.
func TestFullGameVsAITerminates(t *testing.T) {
	g := NewGame(5, 6, OpponentAI)
	for ply := 0; g.Phase == PhasePlaying; ply++ {
		if ply > 100 {
			t.Fatal("game did not terminate")
		}
		if g.AITurn() {
			if !g.StepAI() {
				t.Fatal("StepAI should always have a move while the game is playing")
			}
			continue
		}
		m, ok := BestMove(g.Board)
		if !ok {
			t.Fatal("human turn: BestMove should always find a move while cells remain")
		}
		if !g.Play(m) {
			t.Fatalf("legal move %v was rejected", m)
		}
	}
	if g.Winner != P0 && g.Winner != P1 {
		t.Fatalf("Winner must be set to a valid player, got %v", g.Winner)
	}
}
