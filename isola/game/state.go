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

// TurnStep is which half of the current turn is still to be played: an
// Isola turn is always (1) move the pawn, then (2) remove a tile.
type TurnStep int

const (
	StepMove TurnStep = iota
	StepRemove
)

// GameState is a full playable Isola game.
type GameState struct {
	Board    Board
	Turn     Side
	Opponent Opponent
	Phase    Phase
	AIDepth  int // minimax search depth for OpponentAI
	Step     TurnStep

	// PendingTo is the square the mover's pawn just landed on, once Step has
	// advanced to StepRemove (only meaningful then).
	PendingTo image.Point

	// HasLast, LastFrom/LastTo/LastRemoved describe the most recently
	// completed FULL turn, so the UI can briefly mark it. HasLast is false
	// until the first full turn (move+removal) has been completed.
	HasLast     bool
	LastFrom    image.Point
	LastTo      image.Point
	LastRemoved image.Point

	// stalemated is defensive: GameOver is already checked after every full
	// turn, so Phase is already PhaseDone before either side could ever be
	// asked to move with zero legal moves. Kept only as a description of why
	// StepAI's "no legal move" branch below can't normally trigger.
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
		Step:     StepMove,
	}
}

// HumanColor returns the color the local human controls in AI mode. In
// hotseat mode both colors are human; this is only meaningful for OpponentAI.
func (s *GameState) HumanColor() Side { return Black }

// AITurn reports whether it is currently the AI's move (either half of it).
func (s *GameState) AITurn() bool {
	return s.Opponent == OpponentAI && s.Phase == PhasePlaying && s.Turn == White
}

// Winner returns the winning color, or Empty if the game has no winner (yet).
// Only meaningful once Phase == PhaseDone.
func (s *GameState) Winner() Side {
	if s.stalemated {
		return s.Turn.Opponent()
	}
	return Winner(&s.Board, s.Turn)
}

// PlayMove plays the first half of the side-to-move's turn: moving its pawn
// to "to". Returns true if the move was legal and applied. On success, Step
// advances to StepRemove — the turn is not complete until PlayRemoval is
// also called.
func (s *GameState) PlayMove(to image.Point) bool {
	if s.Phase != PhasePlaying || s.Step != StepMove {
		return false
	}
	if !s.Board.IsLegalPawnMove(s.Turn, to) {
		return false
	}
	from := s.Board.PawnPos(s.Turn)
	nb := s.Board
	nb.setPawnPos(s.Turn, to)
	s.Board = nb
	s.PendingTo = to
	s.Step = StepRemove
	s.LastFrom, s.LastTo = from, to
	return true
}

// PlayRemoval plays the second half of the side-to-move's turn: removing
// tile "remove" (which must not be the square the mover just landed on).
// Returns true if the removal was legal and applied, completing the turn and
// advancing to the opponent (or ending the game if they have no reply).
func (s *GameState) PlayRemoval(remove image.Point) bool {
	if s.Phase != PhasePlaying || s.Step != StepRemove {
		return false
	}
	if !s.Board.IsLegalRemoval(s.PendingTo, remove) {
		return false
	}
	s.Board.Present[remove.Y][remove.X] = false
	s.LastRemoved = remove
	s.HasLast = true
	s.Step = StepMove
	s.advance()
	return true
}

// StepAI plays the AI's whole turn at once (OpponentAI, White to move).
// Returns true if a move was made. Caller should redraw after.
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
	s.Board = s.Board.Apply(mv)
	s.LastFrom, s.LastTo, s.LastRemoved = mv.From, mv.To, mv.Remove
	s.HasLast = true
	s.advance()
	return true
}

// advance hands the turn to the opponent, then checks whether THEY have any
// legal move at all — if not, the game ends immediately in the mover's
// favor, per Isola's win condition.
func (s *GameState) advance() {
	next := s.Turn.Opponent()
	s.Turn = next
	if GameOver(&s.Board, next) {
		s.Phase = PhaseDone
	}
}
