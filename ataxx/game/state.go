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

// GameState is a full playable Ataxx game.
type GameState struct {
	Board    Board
	Turn     Cell
	Opponent Opponent
	Phase    Phase
	AIDepth  int // search depth for OpponentAI

	// LastMove/LastFlipped describe the most recently applied move, so the UI
	// can distinguish a clone from a jump and briefly mark flipped men.
	LastMove    Move
	LastIsJump  bool
	LastFlipped []image.Point
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
func (s *GameState) HumanColor() Cell { return Black }

// AITurn reports whether it is currently the AI's move.
func (s *GameState) AITurn() bool {
	return s.Opponent == OpponentAI && s.Phase == PhasePlaying && s.Turn == White
}

// Winner returns the winning color, or Empty for a tie. Only meaningful once
// Phase == PhaseDone.
func (s *GameState) Winner() Cell { return Winner(&s.Board) }

// Play attempts to move the side to move's man from "from" to "to" (a clone
// or a jump). Returns true if the move was legal and applied, advancing the
// turn/phase as needed.
func (s *GameState) Play(from, to image.Point) bool {
	if s.Phase != PhasePlaying {
		return false
	}
	m := Move{From: from, To: to}
	if !s.Board.IsLegalMove(s.Turn, m) {
		return false
	}
	nb, flipped := s.Board.Apply(m)
	s.Board = nb
	s.LastMove = m
	s.LastIsJump = m.IsJump()
	s.LastFlipped = flipped
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
		// No legal move at all: advance() would already have ended the game
		// before handing White the turn, so this is only a defensive
		// safety net, not a path expected to run in practice.
		s.Phase = PhaseDone
		return true
	}
	nb, flipped := s.Board.Apply(mv)
	s.Board = nb
	s.LastMove = mv
	s.LastIsJump = mv.IsJump()
	s.LastFlipped = flipped
	s.advance()
	return true
}

// advance ends the game once the board fills or the side about to move has
// no legal move (which also covers that side having no pieces left, since a
// side with zero men trivially has zero legal moves) — otherwise it hands
// the turn to that side.
//
// Unlike Hasami shogi, Ataxx as specified has no "pass": running out of
// moves does not automatically lose, it simply ends the game immediately and
// the higher piece count decides (see Winner in winner.go). This is a
// deliberate simplification of the traditional arcade rule (which lets the
// stuck side pass until either the board fills or BOTH sides are stuck) —
// the spec text ("Win: when the board fills or the side to move has no
// legal move / no pieces, whoever has more pieces wins") is read literally
// here rather than adding an unrequested pass mechanic.
func (s *GameState) advance() {
	if s.Board.IsFull() {
		s.Phase = PhaseDone
		return
	}
	next := s.Turn.Opponent()
	if len(s.Board.LegalMoves(next)) == 0 {
		s.Phase = PhaseDone
		return
	}
	s.Turn = next
}
