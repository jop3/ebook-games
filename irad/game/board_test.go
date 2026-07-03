package game

import "testing"

func place(t *testing.T, gs *GameState, x, y int) {
	t.Helper()
	idx := gs.Board.Idx(x, y)
	if !gs.ApplyMove(Move{From: -1, To: idx}) {
		t.Fatalf("move at (%d,%d) rejected", x, y)
	}
}

// Horizontal win for Player1 on tic-tac-toe.
func TestHorizontalWin(t *testing.T) {
	gs := NewGame(Presets[0], false) // Tre i rad
	place(t, gs, 0, 0)               // P1
	place(t, gs, 0, 1)               // P2
	place(t, gs, 1, 0)               // P1
	place(t, gs, 1, 1)               // P2
	place(t, gs, 2, 0)               // P1 -> wins row 0
	if gs.Winner != Player1 {
		t.Fatalf("expected Player1 win, got winner=%v phase=%v", gs.Winner, gs.Phase)
	}
	if gs.Phase != PhaseGameOver {
		t.Fatalf("expected game over, got %v", gs.Phase)
	}
}

func TestDiagonalWin(t *testing.T) {
	gs := NewGame(CustomPreset(5, 5, 4), false)
	// P1 builds diagonal (0,0)(1,1)(2,2)(3,3); P2 plays harmless top row.
	moves := [][2]int{{0, 0}, {0, 4}, {1, 1}, {1, 4}, {2, 2}, {2, 4}, {3, 3}}
	for _, m := range moves {
		place(t, gs, m[0], m[1])
	}
	if gs.Winner != Player1 {
		t.Fatalf("expected diagonal Player1 win, got %v", gs.Winner)
	}
}

func TestNoFalseWinAcrossBlocked(t *testing.T) {
	// A blocked cell must break a line.
	p := Preset{Name: "t", Width: 5, Height: 1, WinLength: 3}
	b := NewBoard(p)
	b.Blocked[2] = true
	b.Cells[0], b.Cells[1] = Player1, Player1
	b.Cells[3], b.Cells[4] = Player1, Player1
	if w := b.CheckWin(1); w != PlayerNone {
		t.Fatalf("blocked cell should break line, got winner %v", w)
	}
	if w := b.CheckWin(4); w != PlayerNone {
		t.Fatalf("two-run right of block is only length 2, got %v", w)
	}
}

func TestDrawDetection(t *testing.T) {
	gs := NewGame(Presets[0], false) // 3x3, win 3
	// Fill to a known draw:
	// P1 P2 P1
	// P1 P2 P2
	// P2 P1 P1
	seq := [][2]int{
		{0, 0}, // P1
		{1, 0}, // P2
		{2, 0}, // P1
		{1, 1}, // P2
		{0, 1}, // P1
		{2, 1}, // P2
		{1, 2}, // P1
		{0, 2}, // P2
		{2, 2}, // P1
	}
	for _, m := range seq {
		if gs.Phase == PhaseGameOver {
			break
		}
		place(t, gs, m[0], m[1])
	}
	if gs.Phase != PhaseGameOver {
		t.Fatalf("expected game over after full board, phase=%v", gs.Phase)
	}
	if gs.Winner != PlayerNone {
		t.Fatalf("expected draw, got winner %v", gs.Winner)
	}
}

func TestDropMode(t *testing.T) {
	gs := NewGame(Presets[2], false) // Fyra i rad, 7x6 drop
	// Dropping in column 3 four times alternates players, so stack is
	// P1,P2,P1,P2 bottom-up. Drop in col 0 for P2's spare turns instead to
	// give P1 a clean vertical four.
	drop := func(col int) {
		idx, ok := gs.Board.DropTarget(col)
		if !ok {
			t.Fatalf("column %d unexpectedly full", col)
		}
		if !gs.ApplyMove(Move{From: -1, To: idx}) {
			t.Fatalf("drop in col %d rejected", col)
		}
	}
	drop(3) // P1 bottom of col3
	drop(0) // P2
	drop(3) // P1
	drop(0) // P2
	drop(3) // P1
	drop(0) // P2
	drop(3) // P1 -> four vertical in col3
	if gs.Winner != Player1 {
		t.Fatalf("expected Player1 vertical four, got winner=%v", gs.Winner)
	}
}

func TestDropStacking(t *testing.T) {
	gs := NewGame(Presets[2], false)
	first, _ := gs.Board.DropTarget(2)
	gs.ApplyMove(Move{From: -1, To: first})
	second, _ := gs.Board.DropTarget(2)
	// Second drop in same column must land one row above the first.
	fx, fy := gs.Board.XY(first)
	sx, sy := gs.Board.XY(second)
	if sx != fx || sy != fy-1 {
		t.Fatalf("stacking wrong: first=(%d,%d) second=(%d,%d)", fx, fy, sx, sy)
	}
}

func TestPlacingToMovingTransition(t *testing.T) {
	gs := NewGame(Presets[1], false) // Tre i kvarn, limit 3
	// Place all 6 stones (3 each), no three-in-a-row.
	// Board 3x3. Use a layout that avoids any line of 3.
	//  P1 P2 P1
	//  P2 P1 .
	//  P2 .  .
	seq := [][2]int{
		{0, 0}, // P1
		{1, 0}, // P2
		{2, 0}, // P1
		{0, 1}, // P2
		{1, 1}, // P1
		{0, 2}, // P2
	}
	for _, m := range seq {
		place(t, gs, m[0], m[1])
	}
	if gs.Phase != PhaseMoving {
		t.Fatalf("expected MOVING after both reached piece limit, got %v (placed=%v)", gs.Phase, gs.Board.Placed)
	}
	// In moving phase, placement-style moves should not be offered.
	moves := gs.Board.ValidMoves(gs.Turn, gs.Phase)
	for _, m := range moves {
		if m.From < 0 {
			t.Fatalf("moving phase produced a placement move: %+v", m)
		}
	}
}

func TestAIBlocksImmediateThreat(t *testing.T) {
	gs := NewGame(CustomPreset(7, 7, 4), true)
	// Player1 (human) has three in a row horizontally at row 3, cols 1-3,
	// with both ends open. AI (Player2) to move must block col 0 or col 4.
	b := &gs.Board
	b.Cells[b.Idx(1, 3)] = Player1
	b.Cells[b.Idx(2, 3)] = Player1
	b.Cells[b.Idx(3, 3)] = Player1
	b.Placed[Player1] = 3
	gs.Turn = Player2

	m, ok := BestMove(gs.Board, Player2, PhasePlacing)
	if !ok {
		t.Fatal("AI returned no move")
	}
	end0 := b.Idx(0, 3)
	end4 := b.Idx(4, 3)
	if m.To != end0 && m.To != end4 {
		x, y := b.XY(m.To)
		t.Fatalf("AI failed to block open three; played (%d,%d) idx=%d, wanted idx %d or %d", x, y, m.To, end0, end4)
	}
}

func TestAITakesWin(t *testing.T) {
	gs := NewGame(CustomPreset(7, 7, 4), true)
	b := &gs.Board
	// AI (Player2) has three in a row, can complete a four.
	b.Cells[b.Idx(1, 2)] = Player2
	b.Cells[b.Idx(2, 2)] = Player2
	b.Cells[b.Idx(3, 2)] = Player2
	m, ok := BestMove(gs.Board, Player2, PhasePlacing)
	if !ok {
		t.Fatal("AI returned no move")
	}
	end0 := b.Idx(0, 2)
	end4 := b.Idx(4, 2)
	if m.To != end0 && m.To != end4 {
		x, y := b.XY(m.To)
		t.Fatalf("AI failed to take the win; played (%d,%d), wanted idx %d or %d", x, y, end0, end4)
	}
}
