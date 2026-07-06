package game

import "math/rand"

// Phase is the high-level stage of a game.
type Phase int

const (
	PhaseSetup   Phase = iota // players alternate freely placing their 30 pieces each
	PhasePlaying              // stack-move phase
	PhaseDone
)

// Opponent selects who the second player is.
type Opponent int

const (
	OpponentHotseat Opponent = iota // two humans take turns
	OpponentAI                      // human is Black, AI is White
)

// GameState is a full playable Staplarna (TZAAR) game.
type GameState struct {
	Board    *Board
	Turn     Side
	Phase    Phase
	Opponent Opponent
	AIDepth  int

	// Remaining[side][typ] is how many pieces of typ that side still has to
	// place during PhaseSetup (StartCount(typ) down to 0).
	Remaining map[Side]map[PieceType]int

	winner Side // meaningful only once Phase == PhaseDone

	// Last-move bookkeeping for the UI (briefly highlighting the most
	// recent action).
	HasLast      bool
	LastFrom     Point
	LastTo       Point
	LastCaptured bool
}

// NewGame starts a fresh game: an empty board, Black to place/move first.
func NewGame(opp Opponent, aiDepth int) *GameState {
	return &GameState{
		Board:    NewBoard(),
		Turn:     Black,
		Phase:    PhaseSetup,
		Opponent: opp,
		AIDepth:  aiDepth,
		Remaining: map[Side]map[PieceType]int{
			Black: newRemaining(),
			White: newRemaining(),
		},
	}
}

func newRemaining() map[PieceType]int {
	m := make(map[PieceType]int, 3)
	for _, t := range AllTypes {
		m[t] = StartCount(t)
	}
	return m
}

// HumanColor is the side the local human plays in OpponentAI mode.
func (s *GameState) HumanColor() Side { return Black }

// AITurn reports whether it is currently the AI's turn to act (place or
// move).
func (s *GameState) AITurn() bool {
	return s.Opponent == OpponentAI && s.Phase != PhaseDone && s.Turn == White
}

// Winner returns the winning side, or None if the game has no winner (yet).
// Only meaningful once Phase == PhaseDone.
func (s *GameState) Winner() Side {
	if s.Phase != PhaseDone {
		return None
	}
	return s.winner
}

// RemainingCount is how many of typ `side` still has left to place.
func (s *GameState) RemainingCount(side Side, typ PieceType) int {
	return s.Remaining[side][typ]
}

// AvailableTypes lists the types `side` can still place right now (remaining
// > 0), in AllTypes (scarcest-first) order.
func (s *GameState) AvailableTypes(side Side) []PieceType {
	var out []PieceType
	for _, t := range AllTypes {
		if s.Remaining[side][t] > 0 {
			out = append(out, t)
		}
	}
	return out
}

// PlacedCount is how many pieces total (0..60) have been placed so far.
func (s *GameState) PlacedCount() int {
	n := 0
	for _, side := range [2]Side{Black, White} {
		for _, t := range AllTypes {
			n += StartCount(t) - s.Remaining[side][t]
		}
	}
	return n
}

// PlacePiece places one piece of typ for the side to move at p during
// PhaseSetup. Returns true if legal and applied.
func (s *GameState) PlacePiece(typ PieceType, p Point) bool {
	if s.Phase != PhaseSetup {
		return false
	}
	side := s.Turn
	if s.Remaining[side][typ] <= 0 {
		return false
	}
	if !s.Board.PlaceNew(side, typ, p) {
		return false
	}
	s.Remaining[side][typ]--
	s.HasLast = false
	if s.setupDone() {
		s.Phase = PhasePlaying
	}
	s.Turn = s.Turn.Opponent()
	return true
}

func (s *GameState) setupDone() bool {
	for _, side := range [2]Side{Black, White} {
		for _, t := range AllTypes {
			if s.Remaining[side][t] > 0 {
				return false
			}
		}
	}
	return true
}

// QuickRandomSetup fills the REST of the setup phase with uniformly random
// legal placements — a random empty cell, and a random still-available type
// for whoever's turn it is — used for the menu's "quick start" option.
// PlacePiece itself already decides turn order and when setup ends, so this
// is simply PlacePiece driven by an RNG instead of taps, exercising the exact
// same legality path a manual placement would. No-op once PhaseSetup has
// already ended.
func (s *GameState) QuickRandomSetup(rng *rand.Rand) {
	for s.Phase == PhaseSetup {
		var empty []Point
		for _, p := range AllPoints() {
			if _, occ := s.Board.At(p); !occ {
				empty = append(empty, p)
			}
		}
		if len(empty) == 0 {
			break // defensive: the 61-cell board always has room for 60 pieces
		}
		p := empty[rng.Intn(len(empty))]
		avail := s.AvailableTypes(s.Turn)
		if len(avail) == 0 {
			break // defensive: should be unreachable while Phase == PhaseSetup
		}
		typ := avail[rng.Intn(len(avail))]
		if !s.PlacePiece(typ, p) {
			break // defensive: should always succeed given the checks above
		}
	}
}

// Play attempts to move the side-to-move's stack from `from` to `to` during
// PhasePlaying. Returns true if legal and applied, advancing the turn (or
// ending the game) as needed.
func (s *GameState) Play(from, to Point) bool {
	if s.Phase != PhasePlaying {
		return false
	}
	m := Move{From: from, To: to}
	if !IsLegalMove(s.Board, s.Turn, m) {
		return false
	}
	captured := s.Board.Apply(m)
	s.HasLast, s.LastFrom, s.LastTo, s.LastCaptured = true, from, to, captured
	s.advance()
	return true
}

// advance checks for a win (type-elimination first, then "no legal move"),
// then hands the turn to the opponent.
func (s *GameState) advance() {
	if loser := EliminatedSide(s.Board); loser != None {
		s.Phase = PhaseDone
		s.winner = loser.Opponent()
		return
	}
	next := s.Turn.Opponent()
	if len(LegalMoves(s.Board, next)) == 0 {
		s.Phase = PhaseDone
		s.winner = next.Opponent()
		s.Turn = next
		return
	}
	s.Turn = next
}

// StepAI performs one AI turn's worth of action (a setup placement, or a
// play-phase move) and returns true if it acted. Caller should redraw after.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	switch s.Phase {
	case PhaseSetup:
		typ, p := AIPlacement(s.Board, s.Remaining[s.Turn])
		return s.PlacePiece(typ, p)
	case PhasePlaying:
		m, ok := BestMove(s.Board, s.Turn, s.AIDepth)
		if !ok {
			// No legal move at all: forfeit (advance() would already have
			// caught this before handing White the turn; kept as a safety
			// net, mirroring hasami/ringar's identical defensive pattern).
			s.Phase = PhaseDone
			s.winner = s.Turn.Opponent()
			return true
		}
		return s.Play(m.From, m.To)
	}
	return false
}
