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

// GameState is a full playable Shong game.
type GameState struct {
	Board    Board
	Turn     Side
	Opponent Opponent
	Phase    Phase
	AIDepth  int // negamax search depth for OpponentAI

	// LastCaptured holds the square of the piece captured by the most
	// recently applied move (nil if that move captured nothing), so the UI
	// can briefly mark it before the next move replaces it.
	LastCaptured *image.Point

	// stalemated records that the side named by Turn had no legal move at
	// all when it became their turn. Shong defines no pass, so being unable
	// to move at all is treated as a loss; this is a defensive rule for an
	// essentially unreachable edge case on this board, handled the same way
	// as the analogous case in this codebase's other 2-player games.
	stalemated bool
}

// NewGame starts a fresh game in the standard starting position.
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
func (s *GameState) HumanColor() Side { return Black }

// AITurn reports whether it is currently the AI's move.
func (s *GameState) AITurn() bool {
	return s.Opponent == OpponentAI && s.Phase == PhasePlaying && s.Turn == White
}

// Winner returns the winning side, and whether the game has one yet. Only
// meaningful once Phase == PhaseDone.
func (s *GameState) Winner() (Side, bool) {
	if s.stalemated {
		return s.Turn.Opponent(), true
	}
	return Winner(&s.Board)
}

// Play attempts to move the side to move's piece from "from" to "to".
// Returns true if the move was legal and applied, advancing turn/phase as
// needed.
func (s *GameState) Play(from, to image.Point) bool {
	if s.Phase != PhasePlaying {
		return false
	}
	m := Move{From: from, To: to}
	if !s.Board.IsLegalMove(s.Turn, m) {
		return false
	}
	nb, captured := s.Board.Apply(m)
	s.Board = nb
	s.setLastCaptured(captured, to)
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
		s.Phase = PhaseDone
		s.stalemated = true
		return true
	}
	nb, captured := s.Board.Apply(mv)
	s.Board = nb
	s.setLastCaptured(captured, mv.To)
	s.advance()
	return true
}

func (s *GameState) setLastCaptured(captured *Piece, at image.Point) {
	if captured == nil {
		s.LastCaptured = nil
		return
	}
	p := at
	s.LastCaptured = &p
}

// advance checks for a win, then hands the turn to the opponent — unless the
// opponent has no legal move at all, in which case they forfeit immediately.
func (s *GameState) advance() {
	if _, ok := Winner(&s.Board); ok {
		s.Phase = PhaseDone
		return
	}
	next := s.Turn.Opponent()
	if len(s.Board.LegalMoves(next)) == 0 {
		s.Turn = next
		s.Phase = PhaseDone
		s.stalemated = true
		return
	}
	s.Turn = next
}
