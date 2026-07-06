package game

import "image"

// Opponent selects who the second player is.
type Opponent int

const (
	OpponentHotseat Opponent = iota // two humans take turns
	OpponentAI                      // human is Black, AI is White
)

// Phase is the high-level state of a game. The opening phase is a one-time,
// two-step special sequence (never repeated); every turn from PhasePlaying
// onward is a jump (or jump chain) — there is no other kind of move.
type Phase int

const (
	PhaseOpeningBlackRemove Phase = iota // Black removes one of the 2 center stones
	PhaseOpeningWhiteRemove               // White removes a stone adjacent to the gap
	PhasePlaying
	PhaseDone
)

// GameState is a full playable Konane game.
type GameState struct {
	Board    Board
	Turn     Cell
	Opponent Opponent
	Phase    Phase
	AIDepth  int // negamax search depth for OpponentAI

	openingGap image.Point // set once Black's opening removal has happened

	// ChainActive/ChainFrom describe a jump chain in progress: the side named
	// by Turn has made at least one jump this turn with the stone now sitting
	// at ChainFrom, and may either continue jumping with that same stone or
	// stop (the UI's "Klart" button), ending the turn.
	ChainActive bool
	ChainFrom   image.Point

	// LastCaptured accumulates every cell captured so far during the current
	// turn (reset when a new turn's first jump is played), so the UI can
	// briefly mark everything just removed.
	LastCaptured []image.Point

	winner Cell // meaningful only once Phase == PhaseDone
}

// NewGame starts a fresh game: a full board, Black to open.
func NewGame(opp Opponent, aiDepth int) *GameState {
	return &GameState{
		Board:    NewBoard(),
		Turn:     Black,
		Opponent: opp,
		Phase:    PhaseOpeningBlackRemove,
		AIDepth:  aiDepth,
	}
}

// HumanColor returns the color the local human controls in AI mode. In
// hotseat mode both colors are human; this is only meaningful for OpponentAI.
func (s *GameState) HumanColor() Cell { return Black }

// AITurn reports whether the next action (an opening removal, or a play-phase
// turn) belongs to the AI.
func (s *GameState) AITurn() bool {
	if s.Opponent != OpponentAI {
		return false
	}
	switch s.Phase {
	case PhaseOpeningWhiteRemove:
		return true
	case PhasePlaying:
		return s.Turn == White
	default:
		return false
	}
}

// Winner returns the winning color, or Empty if the game has no winner (yet).
// Only meaningful once Phase == PhaseDone.
func (s *GameState) Winner() Cell { return s.winner }

// RemoveOpeningBlack attempts Black's one-time opening move: removing one of
// the two center stones at p. Returns true if p was a legal choice and the
// phase advanced to PhaseOpeningWhiteRemove.
func (s *GameState) RemoveOpeningBlack(p image.Point) bool {
	if s.Phase != PhaseOpeningBlackRemove {
		return false
	}
	if !isCenterRemovalOption(p) {
		return false
	}
	if s.Board.At(p.X, p.Y) != Black {
		return false // defensive; always true on a fresh board
	}
	s.Board.set(p.X, p.Y, Empty)
	s.openingGap = p
	s.Phase = PhaseOpeningWhiteRemove
	return true
}

// OpeningWhiteOptions returns every stone White may legally remove for its
// one-time opening move: its own stones orthogonally adjacent to the gap
// Black's removal left behind.
func (s *GameState) OpeningWhiteOptions() []image.Point {
	var out []image.Point
	for _, d := range dirs4 {
		p := image.Pt(s.openingGap.X+d.X, s.openingGap.Y+d.Y)
		if inBounds(p.X, p.Y) && s.Board.At(p.X, p.Y) == White {
			out = append(out, p)
		}
	}
	return out
}

// RemoveOpeningWhite attempts White's one-time opening move: removing its own
// stone at p, which must be orthogonally adjacent to the gap Black created.
// On success the opening ends and normal play begins with Black to move.
func (s *GameState) RemoveOpeningWhite(p image.Point) bool {
	if s.Phase != PhaseOpeningWhiteRemove {
		return false
	}
	valid := false
	for _, o := range s.OpeningWhiteOptions() {
		if o == p {
			valid = true
			break
		}
	}
	if !valid {
		return false
	}
	s.Board.set(p.X, p.Y, Empty)
	s.beginTurn(Black)
	return true
}

// beginTurn hands the turn to side, immediately ending the game (a loss for
// side) if side has no legal jump anywhere — Konane's only win condition.
func (s *GameState) beginTurn(side Cell) {
	s.Phase = PhasePlaying
	s.Turn = side
	s.ChainActive = false
	if !s.Board.HasAnyJump(side) {
		s.Phase = PhaseDone
		s.winner = side.Opponent()
	}
}

// finishTurn ends the current side's turn (their chain, if any, is over) and
// hands play to the opponent, checking for a win.
func (s *GameState) finishTurn() {
	s.beginTurn(s.Turn.Opponent())
}

// StartJump attempts the first jump of a turn: from must hold a stone of the
// side to move, with no chain already in progress, and (from,to) must be a
// legal single jump. Returns true if applied.
func (s *GameState) StartJump(from, to image.Point) bool {
	if s.Phase != PhasePlaying || s.ChainActive {
		return false
	}
	if s.Board.At(from.X, from.Y) != s.Turn {
		return false
	}
	j, ok := findJump(s.Board.LegalJumpsFrom(from, s.Turn), to)
	if !ok {
		return false
	}
	s.Board = s.Board.Apply(j, s.Turn)
	s.LastCaptured = []image.Point{j.Over}
	s.ChainFrom = j.To
	if len(s.Board.LegalJumpsFrom(j.To, s.Turn)) > 0 {
		s.ChainActive = true
	} else {
		s.finishTurn()
	}
	return true
}

// ContinueJump extends an in-progress chain from ChainFrom to `to`. Returns
// true if applied.
func (s *GameState) ContinueJump(to image.Point) bool {
	if s.Phase != PhasePlaying || !s.ChainActive {
		return false
	}
	j, ok := findJump(s.Board.LegalJumpsFrom(s.ChainFrom, s.Turn), to)
	if !ok {
		return false
	}
	s.Board = s.Board.Apply(j, s.Turn)
	s.LastCaptured = append(s.LastCaptured, j.Over)
	s.ChainFrom = j.To
	if len(s.Board.LegalJumpsFrom(j.To, s.Turn)) == 0 {
		s.finishTurn()
	}
	return true
}

// EndChain lets the player voluntarily stop an in-progress chain early (the
// UI's "Klart" button), ending the turn even though further jumps may still
// be legal. Returns false if there is no chain in progress to stop.
func (s *GameState) EndChain() bool {
	if s.Phase != PhasePlaying || !s.ChainActive {
		return false
	}
	s.finishTurn()
	return true
}

// StepAI plays the AI's next action (an opening removal, or a full play-phase
// move) when it is White's turn to act. Returns true if an action was taken;
// the caller should redraw after.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	switch s.Phase {
	case PhaseOpeningWhiteRemove:
		opts := s.OpeningWhiteOptions()
		if len(opts) == 0 {
			return false // unreachable: the gap always has a White neighbor
		}
		return s.RemoveOpeningWhite(opts[0])
	case PhasePlaying:
		chain, ok := BestMove(s.Board, White, s.AIDepth)
		if !ok {
			// Defensive: AITurn() only reports true here when White has a
			// legal jump, so BestMove should always find one.
			s.Phase = PhaseDone
			s.winner = Black
			return true
		}
		s.Board = ApplyChain(s.Board, White, chain)
		s.LastCaptured = capturedCells(chain)
		s.finishTurn()
		return true
	default:
		return false
	}
}
