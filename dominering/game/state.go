package game

import "image"

// Opponent selects who the second player is.
type Opponent int

const (
	OpponentHotseat Opponent = iota // two humans take turns
	OpponentAI                      // human plays V, AI plays H
)

// Phase is the high-level state of a game.
type Phase int

const (
	PhasePlaying Phase = iota
	PhaseDone
)

// GameState is a full playable Domineering game.
type GameState struct {
	Board    Board
	Turn     Side
	Opponent Opponent
	Phase    Phase
	AIDepth  int // alpha-beta search depth for OpponentAI

	// LastMove holds the two cells covered by the most recently placed
	// domino (zero value before the first move), so the UI can briefly mark
	// them before the next move overwrites it.
	LastMove    [2]image.Point
	HasLastMove bool

	// Moves records every domino placed so far, in play order, so the UI can
	// render each one as its own 1x2 tile rather than just shading occupied
	// cells individually.
	Moves []Move
}

// NewGame starts a fresh game on an empty board of the given size. V always
// moves first.
func NewGame(opp Opponent, size int, aiDepth int) *GameState {
	s := &GameState{
		Board:    NewBoard(size),
		Turn:     V,
		Opponent: opp,
		Phase:    PhasePlaying,
		AIDepth:  aiDepth,
	}
	s.checkTerminal()
	return s
}

// HumanSide returns the side the local human controls. In hotseat mode both
// sides are human; this is only meaningful for OpponentAI, where the human
// always plays V (vertical) and the AI always plays H (horizontal).
func (s *GameState) HumanSide() Side { return V }

// AITurn reports whether it is currently the AI's move.
func (s *GameState) AITurn() bool {
	return s.Opponent == OpponentAI && s.Phase == PhasePlaying && s.Turn == H
}

// Winner returns the side that won. Only meaningful once Phase ==
// PhaseDone: the loss condition is explicit, not assumed — the game only
// ever reaches PhaseDone once Turn (the side to move) has zero legal moves,
// so the winner is always the OTHER side (the last side that was able to
// move), per normal play convention.
func (s *GameState) Winner() Side { return s.Turn.Opponent() }

// checkTerminal ends the game if the side to move (s.Turn) has no legal
// placement left in its fixed orientation. This is the ONLY win condition in
// Domineering: normal play convention, so the player who cannot move loses
// (equivalently, the last player who *could* move wins) — never "last move
// wins" applied as a shortcut assumption.
func (s *GameState) checkTerminal() {
	if !s.Board.HasMove(s.Turn) {
		s.Phase = PhaseDone
	}
}

// Play attempts to place the side-to-move's domino at m. Returns true if the
// move was legal (in that side's fixed orientation) and applied, advancing
// the turn and checking for game end.
func (s *GameState) Play(m Move) bool {
	if s.Phase != PhasePlaying {
		return false
	}
	if !s.Board.IsLegalMove(s.Turn, m) {
		return false
	}
	s.Board = s.Board.Apply(m)
	s.LastMove = [2]image.Point{m.A, m.B}
	s.HasLastMove = true
	s.Moves = append(s.Moves, m)
	s.Turn = s.Turn.Opponent()
	s.checkTerminal()
	return true
}

// StepAI plays the AI's move (OpponentAI, H to move). Returns true if a move
// was made. Caller should redraw after.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	mv, ok := BestMove(s.Board, s.Turn, s.AIDepth)
	if !ok {
		// checkTerminal() after the human's move would already have caught
		// this before ever handing H the turn; kept as a defensive net.
		s.Phase = PhaseDone
		return true
	}
	s.Board = s.Board.Apply(mv)
	s.LastMove = [2]image.Point{mv.A, mv.B}
	s.HasLastMove = true
	s.Moves = append(s.Moves, mv)
	s.Turn = s.Turn.Opponent()
	s.checkTerminal()
	return true
}
