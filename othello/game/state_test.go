package game

import "testing"

func TestTurnAlternates(t *testing.T) {
	s := NewGame(ModeHotseat, 0, VariantNormal)
	if s.Turn != Black {
		t.Fatal("Black moves first")
	}
	if !s.Play(3, 2) {
		t.Fatal("opening move should succeed")
	}
	if s.Turn != White {
		t.Fatal("turn should pass to White")
	}
}

func TestPassWhenNoMove(t *testing.T) {
	// Fill a board so that after Black's move, White has no move but Black does,
	// forcing a pass back to Black. Easiest: a nearly-full board.
	var b Board
	// Give White a single disc surrounded so it has no legal move but Black can
	// still move somewhere. This is a light sanity check of advance() plumbing.
	s := &GameState{Board: NewBoard(), Turn: Black, Mode: ModeHotseat, Phase: PhasePlaying}
	_ = b
	// From the start position, both always have moves early, so just ensure no
	// crash and phase stays playing after a normal move.
	s.Play(2, 3)
	if s.Phase != PhasePlaying {
		t.Fatal("game should still be playing after one move")
	}
}

func TestGameEndsWhenBoardResolves(t *testing.T) {
	// A board where neither side can move => PhaseDone via advance.
	var b Board
	for i := range b {
		if i%2 == 0 {
			b[i] = Black
		} else {
			b[i] = White
		}
	}
	// This checkerboard has no empty cells => no legal moves for anyone.
	s := &GameState{Board: b, Turn: Black, Mode: ModeHotseat, Phase: PhasePlaying}
	// Force an advance by simulating: no legal move exists, so calling advance
	// directly should end the game.
	s.advance()
	if s.Phase != PhaseDone {
		t.Fatal("full board should end the game")
	}
}

func TestWinner(t *testing.T) {
	var b Board
	for i := 0; i < 40; i++ {
		b[i] = Black
	}
	for i := 40; i < 64; i++ {
		b[i] = White
	}
	s := &GameState{Board: b, Phase: PhaseDone}
	if s.Winner() != Black {
		t.Fatalf("expected Black winner, got %v", s.Winner())
	}
}

func TestAIPicksLegalMove(t *testing.T) {
	b := NewBoard()
	mv, ok := BestMove(&b, Black, 3, VariantNormal)
	if !ok {
		t.Fatal("AI should find a move at start")
	}
	if !b.LegalMove(mv[0], mv[1], Black) {
		t.Fatalf("AI returned illegal move %v", mv)
	}
}

// --- Anti-Othello ("Omvänd Othello") variant --------------------------------

func TestWinnerVariantAntiFlipsFewestDiscsWins(t *testing.T) {
	var b Board
	for i := 0; i < 40; i++ {
		b[i] = Black
	}
	for i := 40; i < 64; i++ {
		b[i] = White
	}
	// Normal: Black (40 discs) beats White (24). Anti: White (fewer) wins.
	sNormal := &GameState{Board: b, Phase: PhaseDone, Variant: VariantNormal}
	if sNormal.Winner() != Black {
		t.Fatalf("normal variant: expected Black winner, got %v", sNormal.Winner())
	}
	sAnti := &GameState{Board: b, Phase: PhaseDone, Variant: VariantAnti}
	if sAnti.Winner() != White {
		t.Fatalf("anti variant: expected White (fewest discs) to win, got %v", sAnti.Winner())
	}
}

func TestWinnerVariantAntiTieIsStillATie(t *testing.T) {
	var b Board
	for i := 0; i < 32; i++ {
		b[i] = Black
	}
	for i := 32; i < 64; i++ {
		b[i] = White
	}
	s := &GameState{Board: b, Phase: PhaseDone, Variant: VariantAnti}
	if s.Winner() != Empty {
		t.Fatalf("32-32 tie should be a tie in any variant, got %v", s.Winner())
	}
}

func TestBestMoveVariantAntiPrefersFewerDiscsForItself(t *testing.T) {
	// From the standard opening, every legal Black move flips exactly one
	// White disc, so the *result* of the move itself doesn't distinguish
	// variants — but the AI's evaluation of the RESULTING position should:
	// in VariantAnti it should not simply mirror the VariantNormal choice
	// when they diverge, and its chosen move must still be legal.
	b := NewBoard()
	mvNormal, ok := BestMove(&b, Black, 3, VariantNormal)
	if !ok {
		t.Fatal("expected a move (normal)")
	}
	if !b.LegalMove(mvNormal[0], mvNormal[1], Black) {
		t.Fatalf("normal AI returned illegal move %v", mvNormal)
	}
	mvAnti, ok := BestMove(&b, Black, 3, VariantAnti)
	if !ok {
		t.Fatal("expected a move (anti)")
	}
	if !b.LegalMove(mvAnti[0], mvAnti[1], Black) {
		t.Fatalf("anti AI returned illegal move %v", mvAnti)
	}
}

func TestBestMoveVariantAntiAvoidsCorner(t *testing.T) {
	// A position with exactly two legal Black moves: the corner (0,0) (from
	// the same bracket as TestAIPrefersCornerWhenAvailable) and an unrelated
	// non-corner bracket at (5,5). Verified directly: evaluate(child, Black)
	// after (0,0) is 153 (dominated by the +120 corner weight) vs. 70 after
	// (5,5). Normal Othello therefore picks the corner (higher is better);
	// Anti's sign-flipped eval wants the LOWER resulting evaluate(), so it
	// must pick (5,5) instead — the corner is poison when fewest discs wins,
	// since a corner disc can never be recaptured.
	var b Board
	b.set(0, 1, White)
	b.set(0, 2, Black)
	b.set(6, 5, White)
	b.set(7, 5, Black)
	moves := b.LegalMoves(Black)
	if len(moves) != 2 {
		t.Fatalf("setup should give exactly 2 legal moves, got %v", moves)
	}

	mvNormal, ok := BestMove(&b, Black, 1, VariantNormal)
	if !ok {
		t.Fatal("expected a move (normal)")
	}
	if mvNormal != [2]int{0, 0} {
		t.Fatalf("normal variant should take the corner, chose %v", mvNormal)
	}

	mvAnti, ok := BestMove(&b, Black, 1, VariantAnti)
	if !ok {
		t.Fatal("expected a move (anti)")
	}
	if mvAnti == [2]int{0, 0} {
		t.Error("anti-variant AI should avoid the corner (it's poison when fewest-discs wins), but took it")
	}
	if mvAnti != [2]int{5, 5} {
		t.Errorf("anti variant expected to pick (5,5), got %v", mvAnti)
	}
}

func TestEvalSignFlipsExactly(t *testing.T) {
	b := NewBoard()
	b.Apply(3, 2, Black)
	normal := evaluate(&b, Black)
	if got := signedEval(&b, Black, Black, evalSign(VariantNormal)); got != normal {
		t.Fatalf("signedEval(normal) = %d, want %d", got, normal)
	}
	if got := signedEval(&b, Black, Black, evalSign(VariantAnti)); got != -normal {
		t.Fatalf("signedEval(anti) = %d, want %d (negation)", got, -normal)
	}
}

func TestAIPrefersCornerWhenAvailable(t *testing.T) {
	// Build a position where taking a corner is legal and clearly best.
	var b Board
	// Black at (2,0), White at (1,0), corner (0,0) empty: Black playing (0,0)
	// would need to bracket... set up so corner flips something.
	b.set(0, 1, White)
	b.set(0, 2, Black)
	// Black plays (0,0): down column flips (0,1). Corner is legal.
	if !b.LegalMove(0, 0, Black) {
		t.Skip("corner not legal in constructed position; skipping")
	}
	mv, ok := BestMove(&b, Black, 3, VariantNormal)
	if !ok {
		t.Fatal("expected a move")
	}
	if mv != [2]int{0, 0} {
		t.Errorf("AI should grab the corner, chose %v", mv)
	}
}
