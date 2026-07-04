package game

// Phase is the high-level stage of a game.
type Phase int

const (
	PhasePlacement  Phase = iota // players are still placing their 5 rings each
	PhasePlaying                 // ring-move phase
	PhaseRowPending              // a completed row is awaiting resolution (window pick + ring removal)
	PhaseDone
)

// Opponent selects who the second player is.
type Opponent int

const (
	OpponentHotseat Opponent = iota
	OpponentAI
)

// RingsPerSide is the number of rings each player places (10 total).
const RingsPerSide = 5

// AIDepth is Ringar's single "Mot dator" search depth. YINSH's branching
// factor (many rings, each often with several sliding+jumping destinations)
// is much larger than the other games in this batch, so — as the spec asks
// — this is deliberately shallow and explicitly framed to the player as a
// casual opponent, not a strength target.
//
// Measured on this dev machine (see ai_test.go): depth 2 takes 8-51ms even
// on the most open positions (a freshly-placed, marker-free board — the
// worst case for branching, since every ring can slide across large empty
// stretches). Depth 3 was also measured and rejected: it spikes to ~1.06s on
// that same wide-open board, and the PocketBook's ARM CPU is considerably
// slower than this dev machine, so depth 3 risked feeling like a stall on
// real hardware. Depth 2 stays comfortably fast everywhere tested, which is
// the right trade for a casual opponent.
const AIDepth = 2

// GameState is a full playable Ringar game.
type GameState struct {
	Board    *Board
	Turn     Side // whose turn it is to place/move (also who resolves a pending row, unless PendingSide overrides during PhaseRowPending)
	Phase    Phase
	Opponent Opponent

	Removed map[Side]int // rings removed (the scoring track); Winner at 3

	// Row-resolution sub-state (valid only while Phase == PhaseRowPending).
	PendingSide   Side
	PendingRun    []Point // the full completed run awaiting resolution (len >= 5)
	PendingWindow []Point // nil until chosen; once chosen, exactly the 5 markers to remove

	// moveMover remembers who actually took the ring-move (or NOT set during
	// placement, which never triggers rows) so that once every row this move
	// created — the mover's own, and (defensively) any that would belong to
	// the opponent — has been resolved, the turn correctly passes to the
	// *other* side, regardless of whose row was being resolved last.
	moveMover Side

	// Last-move bookkeeping for the UI (briefly highlighting the marker
	// trail flipped by the most recent ring move).
	HasLast      bool
	LastFrom     Point
	LastTo       Point
	LastFlipped  []Point
	LastRemoved  []Point // markers removed by the most recently resolved row
	LastRingLost Point   // ring removed by the most recently resolved row
	HasLastRing  bool

	placed int // total rings placed so far (0..10)
}

// NewGame starts a fresh game. Per YINSH's placement rule, White places
// first, and the two placement/movement phases share one continuously
// alternating turn order, so White also makes the first ring-move.
// HumanColor is always Black (matching every other game in this house), so
// in OpponentAI mode the AI necessarily makes the opening placement — the
// caller's Draw loop must check AITurn() right after NewGame, exactly as it
// already does after every move.
func NewGame(opp Opponent) *GameState {
	return &GameState{
		Board:    NewBoard(),
		Turn:     White,
		Phase:    PhasePlacement,
		Opponent: opp,
		Removed:  map[Side]int{Black: 0, White: 0},
	}
}

// HumanColor is the side the local human plays in OpponentAI mode.
func (s *GameState) HumanColor() Side { return Black }

// aiSide is the side the built-in AI plays (only meaningful when
// Opponent == OpponentAI).
func (s *GameState) aiSide() Side { return White }

// currentActor is whoever must act right now (placer, mover, or the side
// resolving a pending row).
func (s *GameState) currentActor() Side {
	if s.Phase == PhaseRowPending {
		return s.PendingSide
	}
	return s.Turn
}

// AITurn reports whether it is currently the AI's turn to act (place, move,
// or resolve one of its own pending rows).
func (s *GameState) AITurn() bool {
	return s.Opponent == OpponentAI && s.Phase != PhaseDone && s.currentActor() == s.aiSide()
}

// CurrentActor is whoever must act right now (the placer, the mover, or —
// during PhaseRowPending — the side resolving a completed row), for the UI's
// status line.
func (s *GameState) CurrentActor() Side { return s.currentActor() }

// PlacedCount is the number of rings placed so far (0..10), for the UI/tests.
func (s *GameState) PlacedCount() int { return s.placed }

// Winner returns the winning side, or None if neither side has removed 3
// rings yet.
func (s *GameState) Winner() Side {
	if s.Removed[Black] >= 3 {
		return Black
	}
	if s.Removed[White] >= 3 {
		return White
	}
	return None
}

// PlaceRing places a ring for the side to move at p during PhasePlacement.
func (s *GameState) PlaceRing(p Point) bool {
	if s.Phase != PhasePlacement {
		return false
	}
	if !Valid(p) || s.Board.HasRing(p) {
		return false
	}
	s.Board.Rings[p] = s.Turn
	s.placed++
	s.HasLast = false
	if s.placed >= 2*RingsPerSide {
		s.Phase = PhasePlaying
	}
	s.Turn = s.Turn.Opponent()
	return true
}

// Play attempts to slide the side-to-move's ring from `from` to `to`.
// Returns true if the move was legal and applied (which may leave the game
// in PhaseRowPending awaiting a row resolution rather than immediately
// passing the turn).
func (s *GameState) Play(from, to Point) bool {
	if s.Phase != PhasePlaying {
		return false
	}
	mover := s.Turn
	if !IsLegalRingMove(s.Board, mover, from, to) {
		return false
	}
	flipped := ApplyRingMove(s.Board, from, to)
	s.HasLast = true
	s.LastFrom, s.LastTo, s.LastFlipped = from, to, flipped
	s.LastRemoved, s.HasLastRing = nil, false
	s.beginResolution(mover)
	return true
}

// beginResolution enters row resolution after a move, checking the mover's
// own completed rows first and only then (defensively — see rows.go and
// state_test.go's TestBothSidesRowMoverResolvesFirst) the opponent's.
func (s *GameState) beginResolution(mover Side) {
	s.moveMover = mover
	if s.tryEnterRowPending(mover) {
		return
	}
	if s.tryEnterRowPending(mover.Opponent()) {
		return
	}
	s.endTurn()
}

func (s *GameState) tryEnterRowPending(side Side) bool {
	rows := FindRows(s.Board, side)
	if len(rows) == 0 {
		return false
	}
	s.Phase = PhaseRowPending
	s.PendingSide = side
	s.PendingRun = rows[0].Points
	if len(s.PendingRun) == 5 {
		win := make([]Point, 5)
		copy(win, s.PendingRun)
		s.PendingWindow = win
	} else {
		s.PendingWindow = nil
	}
	return true
}

func (s *GameState) endTurn() {
	s.Phase = PhasePlaying
	s.Turn = s.moveMover.Opponent()
}

// ChooseWindow picks which 5-marker slice of the (longer than 5) pending run
// to claim, based on a tap at p (see Window). Only valid while a row is
// pending and no window has been chosen yet (runs of exactly 5 never need
// this step — the window is fixed automatically).
func (s *GameState) ChooseWindow(p Point) bool {
	if s.Phase != PhaseRowPending || s.PendingWindow != nil {
		return false
	}
	win, ok := Window(s.PendingRun, p)
	if !ok {
		return false
	}
	s.PendingWindow = win
	return true
}

// RemoveRingChoice completes a pending row: p must be one of PendingSide's
// own rings anywhere on the board (any ring, not necessarily one touching
// the row — exactly like real YINSH, this is the single hardest tactical
// choice in the game, since giving up a ring reduces your own future
// mobility). Removes the chosen 5-marker window and that ring, advances the
// score, and either ends the game (3 rings removed), continues resolving
// further rows the same move created, or hands the turn to the other side.
func (s *GameState) RemoveRingChoice(p Point) bool {
	if s.Phase != PhaseRowPending || s.PendingWindow == nil {
		return false
	}
	if s.Board.Rings[p] != s.PendingSide {
		return false
	}
	side := s.PendingSide
	RemoveRow(s.Board, s.PendingWindow)
	RemoveRing(s.Board, p)
	s.Removed[side]++
	s.LastRemoved = s.PendingWindow
	s.LastRingLost, s.HasLastRing = p, true
	s.PendingRun, s.PendingWindow = nil, nil

	if s.Removed[side] >= 3 {
		s.Phase = PhaseDone
		return true
	}

	if s.tryEnterRowPending(s.moveMover) {
		return true
	}
	if s.tryEnterRowPending(s.moveMover.Opponent()) {
		return true
	}
	s.endTurn()
	return true
}

// StepAI performs exactly one turn's worth of AI action — a placement, a
// ring move (including fully resolving every row that move creates, so the
// human never has to wait through the AI's own window/ring-removal taps),
// or (defensively) finishing a pending row that ended up on the AI's side —
// and returns true if it acted. Mirrors this house's established
// aiPend-after-paint pattern: the caller's Draw() shows the human's move
// first, then calls StepAI on the next frame.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	switch s.Phase {
	case PhasePlacement:
		return s.PlaceRing(AIPlacement(s.Board))
	case PhasePlaying:
		from, to, ok := BestMove(s.Board, s.Turn, AIDepth)
		if !ok {
			// No legal ring move at all: an unreachable-in-practice edge
			// case (every ring fully boxed in); treated as a forfeit rather
			// than crashing.
			s.Phase = PhaseDone
			s.Removed[s.Turn.Opponent()] = 3
			return true
		}
		s.Play(from, to)
		s.resolveAIRows()
		return true
	case PhaseRowPending:
		s.resolveAIRows()
		return true
	}
	return false
}

// resolveAIRows drives the AI's own pending-row resolution (window pick,
// then ring choice) to completion, looping in case one move completed
// several rows.
func (s *GameState) resolveAIRows() {
	ai := s.aiSide()
	for s.Phase == PhaseRowPending && s.PendingSide == ai {
		if s.PendingWindow == nil {
			win := AIPickWindow(s.PendingRun)
			s.PendingWindow = win
		}
		ring := AIPickRingToRemove(s.Board, ai)
		s.RemoveRingChoice(ring)
	}
}
