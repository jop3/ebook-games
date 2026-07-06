package game

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

// TurnStep is where the side to move is within their current turn: they
// must place their L-piece first (mandatory), then may optionally move a
// neutral piece before the turn passes.
type TurnStep int

const (
	StepPlaceL TurnStep = iota
	StepNeutralOptional
)

// GameState is a full playable L-Game.
type GameState struct {
	Board    Board
	Turn     Side
	Step     TurnStep
	Opponent Opponent
	Phase    Phase
	AIDepth  int // search depth (in full turns) for OpponentAI

	winner Side // meaningful only once Phase == PhaseDone

	// LastLPlacement/LastNeutralMove record the most recently applied move
	// (zero value if none yet this game / no neutral move was made) so the
	// UI can briefly highlight them.
	LastLPlacement  Placement
	LastNeutralMove NeutralMove
	LastHadNeutral  bool
}

// NewGame starts a fresh game in the standard starting position.
func NewGame(opp Opponent, aiDepth int) *GameState {
	return &GameState{
		Board:    NewBoard(),
		Turn:     Black,
		Step:     StepPlaceL,
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

// Winner returns the winning color; only meaningful once Phase == PhaseDone.
func (s *GameState) Winner() Cell { return s.winner }

// PlaceL attempts to play the mandatory L-placement part of the current
// side's turn. Returns true if pl was legal and applied; the turn then
// advances to the optional-neutral-move step (it does NOT hand the turn to
// the opponent yet).
func (s *GameState) PlaceL(pl Placement) bool {
	if s.Phase != PhasePlaying || s.Step != StepPlaceL {
		return false
	}
	if !IsLegalLPlacement(s.Board, s.Turn, pl) {
		return false
	}
	s.Board = ApplyLPlacement(s.Board, s.Turn, pl)
	s.LastLPlacement = pl
	s.LastHadNeutral = false
	s.Step = StepNeutralOptional
	return true
}

// MoveNeutral attempts the optional neutral-piece relocation. Returns true
// if m was legal and applied; this ends the current side's turn.
func (s *GameState) MoveNeutral(m NeutralMove) bool {
	if s.Phase != PhasePlaying || s.Step != StepNeutralOptional {
		return false
	}
	if !IsLegalNeutralMove(s.Board, m) {
		return false
	}
	s.Board = ApplyNeutralMove(s.Board, m)
	s.LastNeutralMove = m
	s.LastHadNeutral = true
	s.endTurn()
	return true
}

// SkipNeutral declines the optional neutral-piece move, ending the current
// side's turn as-is. Returns false if it isn't currently legal to skip
// (i.e. the mandatory L-placement hasn't been made yet this turn).
func (s *GameState) SkipNeutral() bool {
	if s.Phase != PhasePlaying || s.Step != StepNeutralOptional {
		return false
	}
	s.endTurn()
	return true
}

// endTurn hands the turn to the opponent, unless the opponent has no legal
// L-placement at all on the resulting board — in which case they lose
// immediately and the game ends without them moving.
func (s *GameState) endTurn() {
	s.Step = StepPlaceL
	next := s.Turn.Opponent()
	if w, over := Winner(s.Board, next); over {
		s.Phase = PhaseDone
		s.winner = w
		return
	}
	s.Turn = next
}

// StepAI plays the AI's full turn (OpponentAI, White to move): the L
// placement and, if BestMove chose one, the neutral move. Returns true if a
// move was made. Caller should redraw after.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	fm, ok := BestMove(s.Board, White, s.AIDepth)
	if !ok {
		// Defensive: endTurn() already catches "no legal L placement" before
		// handing White the turn, so this should be unreachable.
		return false
	}
	s.PlaceL(fm.L)
	if fm.HasNeutral {
		s.MoveNeutral(fm.Neutral)
	} else {
		s.SkipNeutral()
	}
	return true
}
