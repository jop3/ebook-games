package game

import "testing"

// Turn rotation must cycle through exactly NumPlayers seats.
func TestTurnRotation(t *testing.T) {
	for _, n := range []int{2, 3, 4} {
		gs := NewGameN(CustomPreset(9, 9, 5), n)
		seen := []Player{gs.Turn}
		// Apply n placements on distinct cells and record the turn each time.
		for i := 0; i < n; i++ {
			place(t, gs, i, 0)
			seen = append(seen, gs.Turn)
		}
		// After n moves we should be back to Player1.
		if seen[0] != Player1 {
			t.Fatalf("n=%d: first turn not Player1", n)
		}
		if seen[n] != Player1 {
			t.Fatalf("n=%d: expected to return to Player1 after %d moves, got %v (seq=%v)", n, n, seen[n], seen)
		}
		// The intermediate turns must be Player1..Player(n) in order.
		for i := 0; i < n; i++ {
			want := Player(i + 1)
			if seen[i] != want {
				t.Fatalf("n=%d: turn %d was %v, want %v (seq=%v)", n, i, seen[i], want, seen)
			}
		}
	}
}

// NewGameN clamps player counts to the valid range.
func TestNewGameNClamping(t *testing.T) {
	if gs := NewGameN(Presets[0], 1); gs.NumPlayers != 2 {
		t.Fatalf("1 player should clamp to 2, got %d", gs.NumPlayers)
	}
	if gs := NewGameN(Presets[0], 9); gs.NumPlayers != MaxPlayers {
		t.Fatalf("9 players should clamp to %d, got %d", MaxPlayers, gs.NumPlayers)
	}
}

// A four-player game: Player3 should be able to win like anyone else.
func TestThirdPlayerWins(t *testing.T) {
	gs := NewGameN(CustomPreset(9, 9, 3), 3)
	// Round-robin: P1, P2, P3 each move. Drive P3 to three in a row on row 4
	// while P1/P2 play scattered cells that never form their own line of 3.
	// Turn order is P1,P2,P3,P1,P2,P3,...
	// P1 plays (0,0),(2,0),(4,0); P2 plays (6,0),(8,0),(0,8) — both gapped.
	seq := [][2]int{
		{0, 0}, // P1
		{6, 0}, // P2
		{0, 4}, // P3  (1st of its line)
		{2, 0}, // P1  (gap from (0,0))
		{8, 0}, // P2  (gap from (6,0))
		{1, 4}, // P3  (2nd)
		{4, 0}, // P1  (gap from (2,0))
		{0, 8}, // P2
		{2, 4}, // P3  -> three in a row on row 4
	}
	for _, m := range seq {
		if gs.Phase == PhaseGameOver {
			break
		}
		place(t, gs, m[0], m[1])
	}
	if gs.Winner != Player3 {
		t.Fatalf("expected Player3 win, got winner=%v phase=%v", gs.Winner, gs.Phase)
	}
}

// Four-player three-men's-morris-style: piece limit transition must wait for
// ALL players to exhaust their budget.
func TestMultiPlayerPieceLimitTransition(t *testing.T) {
	// 5x5 board, win 3, each player limited to 2 pieces, 3 players.
	p := Preset{Name: "x", Width: 5, Height: 5, WinLength: 4, PieceLimit: 2}
	gs := NewGameN(p, 3)
	// 3 players * 2 pieces = 6 placements, none forming a win (win=4).
	cells := [][2]int{
		{0, 0}, {1, 0}, {2, 0}, // round 1: P1,P2,P3
		{0, 2}, {1, 2}, {2, 2}, // round 2: P1,P2,P3 -> all at limit
	}
	for i, c := range cells {
		// Before the final placement we must still be in placing phase.
		if i < len(cells)-1 && gs.Phase != PhasePlacing {
			t.Fatalf("transitioned too early at move %d (phase=%v placed=%v)", i, gs.Phase, gs.Board.Placed)
		}
		place(t, gs, c[0], c[1])
	}
	if gs.Phase != PhaseMoving {
		t.Fatalf("expected MOVING after all 3 players hit the limit, got %v (placed=%v)", gs.Phase, gs.Board.Placed)
	}
}

// AI must stay disabled when there are more than two players.
func TestAIDisabledInMultiplayer(t *testing.T) {
	gs := NewGameN(Presets[3], 3)
	gs.VsAI = true // even if set, must be ignored for >2 players
	gs.AIPlayer = Player2
	gs.Turn = Player2
	if gs.AITurn() {
		t.Fatal("AITurn must be false in a 3-player game")
	}
}
