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

// Step names which half of the current side's two-phase turn is next: first
// move a queen, then shoot an arrow from its new square.
type Step int

const (
	StepMove Step = iota
	StepShoot
)

// GameState is a full playable Amazons game.
type GameState struct {
	Board    Board
	Turn     Side
	Step     Step
	Opponent Opponent
	Phase    Phase

	// Pending is the queen's new square once StepMove has completed and a
	// shot is awaited from it (only meaningful when Step == StepShoot).
	Pending image.Point

	// LastBurned is the square burned by the most recently completed turn's
	// arrow (only meaningful once at least one turn has completed).
	LastBurned    image.Point
	HasLastBurned bool

	// aiTurn holds the AI's fully-decided (move, shot) pair across the two
	// separate StepAI calls that play it out one half per Draw frame, so the
	// player sees the AI's queen land before the arrow does (see main.go's
	// aiPend handling).
	aiTurn *Turn
}

// NewGame starts a fresh game in the standard starting position. Black
// always moves first (mirroring this repo's Hasami convention).
func NewGame(opp Opponent) *GameState {
	return &GameState{
		Board:    NewBoard(),
		Turn:     Black,
		Step:     StepMove,
		Opponent: opp,
		Phase:    PhasePlaying,
	}
}

// HumanColor returns the color the local human controls in AI mode. In
// hotseat mode both colors are human; this is only meaningful for OpponentAI.
func (s *GameState) HumanColor() Side { return Black }

// AITurn reports whether it is currently the AI's move (either half of it).
func (s *GameState) AITurn() bool {
	return s.Opponent == OpponentAI && s.Phase == PhasePlaying && s.Turn == White
}

// Winner returns the winning side's queen Cell, or Empty if the game has no
// winner (yet). Only meaningful once Phase == PhaseDone.
func (s *GameState) Winner() Cell {
	return Winner(&s.Board, s.Turn)
}

// MoveQueen attempts to play the move half of the side-to-move's turn: relocate
// one of its queens from "from" to "to". Returns true if the move was legal
// and applied, in which case Step advances to StepShoot and Pending is set
// to "to" — the shot must now come from there, not from "from".
func (s *GameState) MoveQueen(from, to image.Point) bool {
	if s.Phase != PhasePlaying || s.Step != StepMove {
		return false
	}
	m := QueenMove{From: from, To: to}
	if !s.Board.IsLegalQueenMove(s.Turn, m) {
		return false
	}
	s.Board = s.Board.MoveQueen(m)
	s.Pending = to
	s.Step = StepShoot
	return true
}

// Shoot attempts to play the shoot half of the side-to-move's turn: an arrow
// from Pending (the square its queen just moved to) to "at", which becomes
// permanently burned. Returns true if the shot was legal and applied, in
// which case the turn completes and passes to the opponent (or ends the
// game if the opponent then has no legal move at all).
func (s *GameState) Shoot(at image.Point) bool {
	if s.Phase != PhasePlaying || s.Step != StepShoot {
		return false
	}
	if !s.Board.IsLegalShot(s.Pending, at) {
		return false
	}
	s.Board = s.Board.Shoot(at)
	s.LastBurned = at
	s.HasLastBurned = true
	s.completeTurn()
	return true
}

// completeTurn hands the turn to the opponent, unless the opponent then has
// no legal move at all, in which case the game ends immediately (the side
// that just moved wins).
func (s *GameState) completeTurn() {
	next := s.Turn.Opponent()
	s.Turn = next
	s.Step = StepMove
	if !s.Board.SideHasMove(next) {
		s.Phase = PhaseDone
	}
}

// StepAI plays one half of the AI's turn (OpponentAI, White to move) per
// call — first the move (deciding the whole turn via BestTurn and applying
// just the move half), then, on the following call, the already-decided
// shot. Returns true if a half-turn was played; the caller (main.go's Draw)
// re-triggers it for the second half if AITurn() is still true afterward, so
// the player's screen shows the queen land before the arrow does, exactly
// like a human's two taps would.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	switch s.Step {
	case StepMove:
		t, ok := BestTurn(s.Board, s.Turn)
		if !ok {
			// Should not happen: completeTurn already checked SideHasMove
			// before handing White the turn. Kept as a defensive fallback.
			s.Phase = PhaseDone
			return true
		}
		s.aiTurn = &t
		s.Board = s.Board.MoveQueen(t.Move)
		s.Pending = t.Move.To
		s.Step = StepShoot
		return true
	case StepShoot:
		if s.aiTurn == nil {
			return false
		}
		shot := s.aiTurn.Shot
		s.aiTurn = nil
		s.Board = s.Board.Shoot(shot)
		s.LastBurned = shot
		s.HasLastBurned = true
		s.completeTurn()
		return true
	}
	return false
}
