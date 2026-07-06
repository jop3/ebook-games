package game

import (
	"image"
	"testing"
)

func TestNewGameSetup(t *testing.T) {
	s := NewGame(OpponentHotseat, SideDefender, 0)
	if s.Turn != SideAttacker {
		t.Fatalf("Turn = %v, want SideAttacker to start", s.Turn)
	}
	if s.Phase != PhasePlaying {
		t.Fatalf("Phase = %v, want PhasePlaying", s.Phase)
	}
	if s.Board.Count(Attacker) != StartAttackers || s.Board.Count(Defender) != StartDefenders {
		t.Fatalf("starting counts wrong: attackers=%d defenders=%d", s.Board.Count(Attacker), s.Board.Count(Defender))
	}
	if s.AITurn() {
		t.Fatal("hotseat is never the AI's turn")
	}
}

func TestPlayLegalMoveAdvancesTurn(t *testing.T) {
	s := NewGame(OpponentHotseat, SideDefender, 0)
	legal := s.Board.LegalMoves(SideAttacker)
	if len(legal) == 0 {
		t.Fatal("attackers should have legal moves at the start")
	}
	m := legal[0]
	if !s.Play(m.From, m.To) {
		t.Fatalf("Play(%v) should have succeeded", m)
	}
	if s.Turn != SideDefender {
		t.Fatal("turn should pass to the defenders after attackers move")
	}
	if s.Board.At(m.To.X, m.To.Y) != Attacker {
		t.Fatal("the attacker should now be at the destination")
	}
}

func TestPlayRejectsIllegalMove(t *testing.T) {
	s := NewGame(OpponentHotseat, SideDefender, 0)
	// Moving a defender on the attacker's turn must be rejected.
	if s.Play(image.Pt(3, 2), image.Pt(3, 1)) {
		t.Fatal("attackers may not move a Defender")
	}
	if s.Turn != SideAttacker {
		t.Fatal("an illegal move must not change the turn")
	}
	if s.Play(image.Pt(1, 1), image.Pt(1, 4)) {
		t.Fatal("moving from an empty square must be rejected")
	}
}

func TestAITurnAndStepAI(t *testing.T) {
	s := NewGame(OpponentAI, SideDefender, DepthEasy)
	if s.AITurn() {
		t.Fatal("should not be the AI's turn before the attackers (human) have moved")
	}
	legal := s.Board.LegalMoves(SideAttacker)
	m := legal[0]
	s.Play(m.From, m.To)
	if !s.AITurn() {
		t.Fatal("should be the AI's (defender's) turn after attackers move")
	}
	if !s.StepAI() {
		t.Fatal("StepAI should have played a move")
	}
	if s.AITurn() {
		t.Fatal("turn should have passed back to the attackers after StepAI")
	}
}

func TestAICanPlayEitherSide(t *testing.T) {
	sAtk := NewGame(OpponentAI, SideAttacker, DepthEasy)
	if !sAtk.AITurn() {
		t.Fatal("AI playing attackers should move first")
	}
	if !sAtk.StepAI() {
		t.Fatal("StepAI should have played the attacker's opening move")
	}
	if sAtk.Turn != SideDefender {
		t.Fatal("turn should now belong to the (human) defenders")
	}

	sDef := NewGame(OpponentAI, SideDefender, DepthEasy)
	if sDef.AITurn() {
		t.Fatal("AI playing defenders should not move before the human attacker")
	}
}

func TestKingEscapeEndsGameImmediately(t *testing.T) {
	s := NewGame(OpponentHotseat, SideDefender, 0)
	for i := range s.Board {
		for j := range s.Board[i] {
			s.Board[i][j] = Empty
		}
	}
	s.Board.set(0, 1, King) // one step from the corner (0,0)
	s.Turn = SideDefender
	if !s.Play(image.Pt(0, 1), image.Pt(0, 0)) {
		t.Fatal("the king's escaping move should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done immediately once the king reaches a corner")
	}
	w, reason := s.Winner()
	if w != SideDefender || reason != ReasonKingEscaped {
		t.Fatalf("Winner() = %v,%v, want SideDefender/ReasonKingEscaped", w, reason)
	}
}

func TestKingCaptureEndsGameImmediately(t *testing.T) {
	s := NewGame(OpponentHotseat, SideDefender, 0)
	for i := range s.Board {
		for j := range s.Board[i] {
			s.Board[i][j] = Empty
		}
	}
	s.Board.set(1, 1, King)
	s.Board.set(1, 0, Attacker)
	s.Board.set(1, 2, Attacker)
	s.Board.set(0, 1, Attacker)
	s.Board.set(2, 4, Attacker) // will slide up to (2,1), completing the surround
	s.Turn = SideAttacker
	if !s.Play(image.Pt(2, 4), image.Pt(2, 1)) {
		t.Fatal("the king-capturing move should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done immediately once the king is captured")
	}
	w, reason := s.Winner()
	if w != SideAttacker || reason != ReasonKingCaptured {
		t.Fatalf("Winner() = %v,%v, want SideAttacker/ReasonKingCaptured", w, reason)
	}
	if len(s.LastCaptured) != 1 || s.LastCaptured[0] != (image.Point{X: 1, Y: 1}) {
		t.Fatalf("LastCaptured = %v, want exactly the king's cell (1,1)", s.LastCaptured)
	}
}

// --- "No legal move" win conditions, for BOTH sides -------------------------

func TestAttackersWinWhenDefendersHaveNoLegalMove(t *testing.T) {
	s := NewGame(OpponentHotseat, SideDefender, 0)
	for i := range s.Board {
		for j := range s.Board[i] {
			s.Board[i][j] = Empty
		}
	}
	// A lone king (no other defenders) boxed in on all 4 sides by attackers,
	// with no room to move at all — but not yet "surrounded" in the
	// king-capture sense because one side is the board edge, not a hostile
	// square (edge-adjacent kings can never be captured under this rule set,
	// see isKingSurrounded — but they CAN be stalemated).
	s.Board.set(0, 3, King)     // on the left edge
	s.Board.set(1, 3, Attacker) // east: blocks movement, but the west
	// neighbor is off-board (never hostile), so this does NOT capture the
	// king outright — it just leaves it with zero legal moves once north
	// and south are also blocked.
	s.Board.set(0, 2, Attacker) // north
	s.Board.set(0, 4, Attacker) // south
	s.Board.set(6, 6, Attacker) // mover: makes an unrelated move, handing the turn to the defenders
	s.Turn = SideAttacker
	if !s.Play(image.Pt(6, 6), image.Pt(5, 6)) {
		t.Fatal("setup move should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatal("defenders (only the boxed-in king) should have no legal move at all")
	}
	w, reason := s.Winner()
	if w != SideAttacker || reason != ReasonNoMoves {
		t.Fatalf("Winner() = %v,%v, want SideAttacker/ReasonNoMoves", w, reason)
	}
}

func TestDefendersWinWhenAttackersHaveNoLegalMove(t *testing.T) {
	s := NewGame(OpponentHotseat, SideDefender, 0)
	for i := range s.Board {
		for j := range s.Board[i] {
			s.Board[i][j] = Empty
		}
	}
	// A single attacker boxed in on all 4 sides (3 defenders plus the board
	// edge) with zero legal moves.
	s.Board.set(0, 3, Attacker)
	s.Board.set(0, 2, Defender) // north
	s.Board.set(0, 4, Defender) // south
	s.Board.set(1, 3, Defender) // east
	// west is the board edge.
	s.Board.set(5, 5, King) // king, elsewhere, makes an unrelated move
	s.Turn = SideDefender
	if !s.Play(image.Pt(5, 5), image.Pt(5, 4)) {
		t.Fatal("setup move should be legal")
	}
	if s.Phase != PhaseDone {
		t.Fatal("the lone attacker should have no legal move at all")
	}
	w, reason := s.Winner()
	if w != SideDefender || reason != ReasonNoMoves {
		t.Fatalf("Winner() = %v,%v, want SideDefender/ReasonNoMoves", w, reason)
	}
}
