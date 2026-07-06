package game

import "image"

// Opponent selects who the second player is.
type Opponent int

const (
	OpponentHotseat Opponent = iota // two humans take turns
	OpponentAI                      // human plays one side, the AI plays the other
)

// Phase is the high-level state of a game.
type Phase int

const (
	PhasePlaying Phase = iota
	PhaseDone
)

// GameState is a full playable Hnefatafl (Brandub) game.
type GameState struct {
	Board    Board
	Turn     Side
	Opponent Opponent
	AISide   Side // which side the AI plays, only meaningful if Opponent==OpponentAI
	Phase    Phase
	AIDepth  int

	// LastCaptured holds the cells emptied by the most recently applied move
	// (ordinary captures, plus the king's cell if it was just captured), so
	// the UI can briefly mark them before the next move replaces the list.
	LastCaptured []image.Point

	winSide   Side
	winReason Reason
}

// NewGame starts a fresh game in the Brandub starting position. Attackers
// move first (the standard convention for this family of games).
func NewGame(opp Opponent, aiSide Side, aiDepth int) *GameState {
	return &GameState{
		Board:    NewBoard(),
		Turn:     SideAttacker,
		Opponent: opp,
		AISide:   aiSide,
		AIDepth:  aiDepth,
		Phase:    PhasePlaying,
	}
}

// AITurn reports whether it is currently the AI's move.
func (s *GameState) AITurn() bool {
	return s.Opponent == OpponentAI && s.Phase == PhasePlaying && s.Turn == s.AISide
}

// Winner returns the winning side and why, valid only once Phase==PhaseDone.
func (s *GameState) Winner() (Side, Reason) {
	return s.winSide, s.winReason
}

// Play attempts to move the side to move's piece from "from" to "to".
// Returns true if the move was legal and applied, advancing turn/phase.
func (s *GameState) Play(from, to image.Point) bool {
	if s.Phase != PhasePlaying {
		return false
	}
	m := Move{From: from, To: to}
	if !s.Board.IsLegalMove(s.Turn, m) {
		return false
	}
	nb, res := s.Board.Apply(m)
	s.Board = nb
	s.LastCaptured = collectMarks(res)
	s.advance()
	return true
}

// StepAI plays the AI's move. Returns true if a move was made.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	mv, ok := BestMove(s.Board, s.Turn, s.AIDepth)
	if !ok {
		// No legal move at all: the side to move forfeits (advance() below
		// would already have caught this before handing them the turn; kept
		// as a safety net).
		s.endGame(s.Turn.Opponent(), ReasonNoMoves)
		return true
	}
	nb, res := s.Board.Apply(mv)
	s.Board = nb
	s.LastCaptured = collectMarks(res)
	s.advance()
	return true
}

func collectMarks(res ApplyResult) []image.Point {
	marks := append([]image.Point(nil), res.Captured...)
	if res.KingCaptured {
		marks = append(marks, res.KingCell)
	}
	return marks
}

func (s *GameState) endGame(winner Side, reason Reason) {
	s.Phase = PhaseDone
	s.winSide = winner
	s.winReason = reason
}

// advance checks the board-only win conditions, then hands the turn to the
// opponent — unless that opponent has no legal move at all, in which case
// the OTHER side wins immediately (attackers win if defenders can't move;
// defenders win if attackers can't move).
func (s *GameState) advance() {
	if w, reason, ok := Winner(&s.Board); ok {
		s.endGame(w, reason)
		return
	}
	next := s.Turn.Opponent()
	if len(s.Board.LegalMoves(next)) == 0 {
		s.Turn = next
		s.endGame(next.Opponent(), ReasonNoMoves)
		return
	}
	s.Turn = next
}
