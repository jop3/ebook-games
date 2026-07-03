package game

import "testing"

func TestConnectionDistanceEmpty(t *testing.T) {
	b := NewBoard(5)
	// On an empty board Black must fill a whole column: distance N.
	if d := connectionDistance(b, Black); d != 5 {
		t.Errorf("empty 5x5 Black distance = %d, want 5", d)
	}
}

func TestConnectionDistanceConnected(t *testing.T) {
	b := NewBoard(5)
	for y := 0; y < 5; y++ {
		b.Set(2, y, Black)
	}
	if d := connectionDistance(b, Black); d != 0 {
		t.Errorf("connected Black distance = %d, want 0", d)
	}
}

func TestAITakesWinningMove(t *testing.T) {
	b := NewBoard(5)
	// Black one move from winning: column with a gap at (2,2).
	for y := 0; y < 5; y++ {
		if y != 2 {
			b.Set(2, y, Black)
		}
	}
	mv, ok := BestMove(b, Black)
	if !ok {
		t.Fatal("should find a move")
	}
	if mv != [2]int{2, 2} {
		t.Errorf("AI should complete the win at (2,2), chose %v", mv)
	}
}

func TestAIBlocksOpponentWin(t *testing.T) {
	b := NewBoard(5)
	// White one move from winning across row 2 (gap at (2,2)); Black should
	// block there (no Black win available).
	for x := 0; x < 5; x++ {
		if x != 2 {
			b.Set(x, 2, White)
		}
	}
	mv, ok := BestMove(b, Black)
	if !ok {
		t.Fatal("should find a move")
	}
	if mv != [2]int{2, 2} {
		t.Errorf("AI should block White's win at (2,2), chose %v", mv)
	}
}

func TestAIMoveIsLegal(t *testing.T) {
	s := NewGame(7, ModeAI)
	s.Play(3, 3) // human
	if !s.StepAI() {
		t.Fatal("AI should move")
	}
	// AI placed a White stone somewhere legal.
	whites := 0
	for y := 0; y < 7; y++ {
		for x := 0; x < 7; x++ {
			if s.Board.At(x, y) == White {
				whites++
			}
		}
	}
	if whites != 1 {
		t.Errorf("expected exactly 1 White stone, got %d", whites)
	}
}
