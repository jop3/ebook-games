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

// GameState is a full playable Hasami game.
type GameState struct {
	Board    Board
	Turn     Cell
	Opponent Opponent
	WinMode  WinMode
	Phase    Phase
	AIDepth  int // minimax search depth for OpponentAI

	// LastCaptured holds the cells captured by the most recently applied
	// move (nil if that move captured nothing), so the UI can briefly mark
	// them before the next move replaces the list.
	LastCaptured []image.Point

	// stalemated records that the side named by Turn was unable to move at
	// all when it became their turn, and so forfeited: Hasami defines no
	// pass, and being completely unable to move is treated as a loss. This is
	// an essentially unreachable edge case on an open 9x9 rook-movement
	// board, handled defensively rather than as a documented rule.
	stalemated bool
}

// NewGame starts a fresh game in the standard starting position.
func NewGame(opp Opponent, winMode WinMode, aiDepth int) *GameState {
	return &GameState{
		Board:    NewBoard(),
		Turn:     Black,
		Opponent: opp,
		WinMode:  winMode,
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
	if s.stalemated {
		return s.Turn.Opponent()
	}
	return Winner(&s.Board, s.WinMode)
}

// Play attempts to move the side to move's man from "from" to "to". Returns
// true if the move was legal and applied, advancing turn/phase as needed.
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
	s.LastCaptured = captured
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
		s.stalemated = true
		return true
	}
	nb, captured := s.Board.Apply(mv)
	s.Board = nb
	s.LastCaptured = captured
	s.advance()
	return true
}

// advance checks for a win, then hands the turn to the opponent — unless the
// opponent has no legal move at all, in which case they forfeit immediately.
func (s *GameState) advance() {
	if w := Winner(&s.Board, s.WinMode); w != Empty {
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
