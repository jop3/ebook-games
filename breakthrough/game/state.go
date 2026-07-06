package game

import "image"

// Opponent selects who the second player is.
type Opponent int

const (
	OpponentHotseat Opponent = iota // two humans take turns
	OpponentAI                      // human is Black, AI is White
)

// Phase is the high-level state of a game.
type Phase int

const (
	PhasePlaying Phase = iota
	PhaseDone
)

// GameState is a full playable Breakthrough game.
type GameState struct {
	Board    Board
	Turn     Cell
	Opponent Opponent
	Phase    Phase
	AIDepth  int // alpha-beta search depth for OpponentAI

	// LastMove is the most recently applied move (zero Move before any move
	// is made), so the UI can briefly highlight it.
	LastMove     Move
	LastCaptured bool

	// noMoveLoser records that the side named here was unable to move at
	// all when it became their turn, and so lost immediately: Breakthrough
	// defines no pass. Empty when the game ended some other way (or hasn't
	// ended).
	noMoveLoser Cell
}

// NewGame starts a fresh game in the standard starting position. Black
// always moves first.
func NewGame(opp Opponent, aiDepth int) *GameState {
	return &GameState{
		Board:    NewBoard(),
		Turn:     Black,
		Opponent: opp,
		Phase:    PhasePlaying,
		AIDepth:  aiDepth,
	}
}

// HumanColor returns the color the local human controls in AI mode. In
// hotseat mode both colors are human; this is only meaningful for OpponentAI.
func (s *GameState) HumanColor() Cell { return Black }

// AITurn reports whether it is currently the AI's move.
func (s *GameState) AITurn() bool {
	return s.Opponent == OpponentAI && s.Phase == PhasePlaying && s.Turn == White
}

// Winner returns the winning color, or Empty if the game has no winner (yet).
// Only meaningful once Phase == PhaseDone.
func (s *GameState) Winner() Cell {
	if s.noMoveLoser != Empty {
		return s.noMoveLoser.Opponent()
	}
	return Winner(&s.Board)
}

// Play attempts to move the side to move's pawn from "from" to "to". Returns
// true if the move was legal and applied, advancing turn/phase as needed.
func (s *GameState) Play(from, to image.Point) bool {
	if s.Phase != PhasePlaying {
		return false
	}
	m := Move{From: from, To: to}
	if !s.Board.IsLegalMove(s.Turn, m) {
		return false
	}
	// Recompute Capture from the board so the caller never has to know it.
	m.Capture = from.X != to.X
	s.Board = s.Board.Apply(m)
	s.LastMove = m
	s.LastCaptured = m.Capture
	s.advance()
	return true
}

// StepAI plays the AI's move (OpponentAI, White to move). Returns true if a
// move was made. Caller should redraw after.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	mv, ok := BestMove(s.Board, White, s.AIDepth)
	if !ok {
		// No legal move at all: forfeit (advance() below would already have
		// caught this before handing White the turn; kept as a safety net).
		s.Phase = PhaseDone
		s.noMoveLoser = White
		return true
	}
	s.Board = s.Board.Apply(mv)
	s.LastMove = mv
	s.LastCaptured = mv.Capture
	s.advance()
	return true
}

// advance checks for a win, then hands the turn to the opponent — unless the
// opponent has no legal move at all, in which case they lose immediately
// (Breakthrough defines no pass).
func (s *GameState) advance() {
	if w := Winner(&s.Board); w != Empty {
		s.Phase = PhaseDone
		return
	}
	next := s.Turn.Opponent()
	if len(s.Board.LegalMoves(next)) == 0 {
		s.Turn = next
		s.Phase = PhaseDone
		s.noMoveLoser = next
		return
	}
	s.Turn = next
}
