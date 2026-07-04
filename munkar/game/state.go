package game

import (
	"image"
	"math/rand"
	"time"
)

// Mode selects opponent type.
type Mode int

const (
	ModeHotseat Mode = iota // two humans take turns
	ModeAI                  // human is Black, AI is White
)

// Phase is the high-level state of a game.
type Phase int

const (
	PhasePlaying Phase = iota
	PhaseDone
)

// GameState is a full playable Munkar game.
type GameState struct {
	Board   Board
	Turn    Cell
	Mode    Mode
	Phase   Phase
	AILevel int // minimax search depth for ModeAI

	HasLast bool        // has any ring been placed yet? (false only before the very first move)
	Last    image.Point // the most recently placed ring's cell; meaningful iff HasLast

	// LastFlips holds the cells captured (flipped) by the most recently
	// applied move (nil if it captured nothing), so the UI can briefly flash
	// them before the next move replaces the list.
	LastFlips []image.Point
}

// NewGame starts a fresh game with a randomly shuffled board layout.
func NewGame(mode Mode, aiLevel int) *GameState {
	return NewGameSeeded(mode, aiLevel, time.Now().UnixNano())
}

// NewGameSeeded starts a fresh game with a deterministic board layout, for
// tests and reproducible screenshots.
func NewGameSeeded(mode Mode, aiLevel int, seed int64) *GameState {
	rng := rand.New(rand.NewSource(seed))
	return &GameState{
		Board:   NewBoard(rng),
		Turn:    Black,
		Mode:    mode,
		Phase:   PhasePlaying,
		AILevel: aiLevel,
	}
}

// HumanColor returns the color the local human controls in AI mode. In
// hotseat mode both colors are human; this is only meaningful for ModeAI.
func (s *GameState) HumanColor() Cell { return Black }

// AITurn reports whether it is currently the AI's move.
func (s *GameState) AITurn() bool {
	return s.Mode == ModeAI && s.Phase == PhasePlaying && s.Turn == White
}

// LegalMoves returns the cells the side to move may legally place a ring on
// right now (see the package-level LegalMoves for the direction-forcing
// rule).
func (s *GameState) LegalMoves() []image.Point {
	return LegalMoves(s.Board, s.Last, s.HasLast)
}

// ForcedLine returns the full line (every cell, occupied or not) implied by
// the most recent placement, for the UI to highlight — or nil if there is
// no active constraint (before the first move, or the line was already
// full so the next player may play anywhere).
func (s *GameState) ForcedLine() []image.Point {
	if !s.HasLast {
		return nil
	}
	if len(ForcedCells(s.Board, s.Last)) == 0 {
		return nil
	}
	return axisCells(s.Board.Line[s.Last.Y][s.Last.X], s.Last)
}

func (s *GameState) legal(p image.Point) bool {
	if !inBounds(p.X, p.Y) || s.Board.At(p.X, p.Y) != Empty {
		return false
	}
	for _, m := range s.LegalMoves() {
		if m == p {
			return true
		}
	}
	return false
}

// Winner returns the winning color, or Empty for a draw. Only meaningful
// once Phase == PhaseDone.
func (s *GameState) Winner() Cell { return Winner(s.Board) }

// Play attempts to place a ring for the side to move at p. Returns true if
// it was legal and applied, resolving captures, win checks, and turn
// advancement.
func (s *GameState) Play(p image.Point) bool {
	if s.Phase != PhasePlaying {
		return false
	}
	if !s.legal(p) {
		return false
	}
	mover := s.Turn
	nb, flips := Place(s.Board, p, mover)
	s.Board = nb
	s.LastFlips = flips
	s.Last = p
	s.HasLast = true

	if Five(&nb, mover) {
		s.Phase = PhaseDone
		return true
	}
	if boardFull(&nb) {
		s.Phase = PhaseDone
		return true
	}
	s.Turn = mover.Opponent()
	return true
}

// StepAI plays the AI's move (ModeAI, White to move). Returns true if a
// move was made. Caller should redraw after.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	moves := s.LegalMoves()
	mv, ok := BestMove(s.Board, s.Turn, moves, s.AILevel)
	if !ok {
		// Every cell is occupied (LegalMoves always has a fallback to "play
		// anywhere" while any empty cell exists, so this only happens on a
		// full board, which Play() already turns into PhaseDone). Kept as a
		// defensive no-op.
		return false
	}
	return s.Play(mv)
}
