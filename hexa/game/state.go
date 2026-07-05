package game

// Phase is the high-level stage of a game.
type Phase int

const (
	PhasePlacement Phase = iota // players are still placing tiles from their supply
	PhaseMoving                 // all 42 tiles placed: players slide tiles instead
	PhaseDone
)

// Opponent selects who the second player is.
type Opponent int

const (
	OpponentHotseat Opponent = iota
	OpponentAI
)

// AI search depths ("Lätt"/"Medel"/"Svår" on the menu). See ai.go's doc
// comment and ai_test.go for the measured wall-clock reasoning behind these
// specific numbers.
const (
	DepthEasy   = 1
	DepthMedium = 2
	DepthHard   = 3
)

// GameState is a full playable Hexa (Six) game.
type GameState struct {
	Board     *Board
	Remaining map[Side]int // tiles still in hand (supply), starts at TilesPerSide each
	Turn      Side
	Phase     Phase
	Opponent  Opponent
	AIDepth   int
	Advanced  bool // the optional "Avancerat" disconnect-and-strand rule

	WinSide  Side
	WinKind  ShapeKind
	WinCells []Hex

	// EdgeReached goes true (and stays true) the first time any tile ever
	// occupies the outer boundary ring of the bounded v1 board — an honest
	// nod to the fact that a real "Six" board is unbounded (see coords.go's
	// Radius doc comment); the UI surfaces this as a small status note.
	EdgeReached bool

	// Last-move bookkeeping for the UI (briefly highlighting the tile(s)
	// just placed/moved, and any advanced-rule casualties).
	HasLast      bool
	LastPlaced   Hex   // valid during/just after PhasePlacement moves
	LastFrom     Hex   // valid after a movement-phase move
	LastTo       Hex   // valid after a placement OR a movement-phase move
	LastStranded []Hex // cells removed by an advanced-rule disconnect, if any
}

// NewGame starts a fresh game. Black is always the local human (matching
// every other game in this house); Black moves first.
func NewGame(opp Opponent, aiDepth int, advanced bool) *GameState {
	if aiDepth < 1 {
		aiDepth = DepthEasy
	}
	return &GameState{
		Board:     NewBoard(),
		Remaining: map[Side]int{Black: TilesPerSide, White: TilesPerSide},
		Turn:      Black,
		Phase:     PhasePlacement,
		Opponent:  opp,
		AIDepth:   aiDepth,
		Advanced:  advanced,
	}
}

// HumanColor is the side the local human plays in OpponentAI mode.
func (s *GameState) HumanColor() Side { return Black }

// aiSide is the side the built-in AI plays (only meaningful when
// Opponent == OpponentAI).
func (s *GameState) aiSide() Side { return White }

// AITurn reports whether it is currently the AI's turn to act.
func (s *GameState) AITurn() bool {
	return s.Opponent == OpponentAI && s.Phase != PhaseDone && s.Turn == s.aiSide()
}

// TilesLeft is the side's total tiles still "in the game" — in hand plus on
// the board. It only ever decreases, and only via the advanced rule's
// permanent tile removal (placement/movement never destroys a tile). A side
// at TilesLeft <= 5 can never again assemble a 6-tile shape and loses under
// the advanced rule.
func (s *GameState) TilesLeft(side Side) int {
	return s.Remaining[side] + s.Board.Count(side)
}

// Winner returns the side that has won, or None if the game isn't over.
func (s *GameState) Winner() Side {
	if s.Phase != PhaseDone {
		return None
	}
	return s.WinSide
}

func (s *GameState) markEdge(p Hex) {
	if OnEdge(p) {
		s.EdgeReached = true
	}
}

// PlaceTile places one of the side-to-move's supply tiles at p, during
// PhasePlacement. Returns true if the placement was legal and applied.
func (s *GameState) PlaceTile(p Hex) bool {
	if s.Phase != PhasePlacement {
		return false
	}
	side := s.Turn
	if s.Remaining[side] <= 0 {
		return false
	}
	legal := false
	for _, m := range PlaceMoves(s.Board.Tiles) {
		if m == p {
			legal = true
			break
		}
	}
	if !legal {
		return false
	}
	s.Board.Tiles[p] = side
	s.Remaining[side]--
	s.markEdge(p)
	s.HasLast = true
	s.LastPlaced, s.LastTo = p, p
	s.LastStranded = nil

	if s.checkShapeWin(side) {
		return true
	}
	if s.Remaining[Black] == 0 && s.Remaining[White] == 0 {
		s.Phase = PhaseMoving
	}
	s.Turn = side.Opponent()
	return true
}

// MoveTile slides the side-to-move's tile at from to the empty cell to,
// during PhaseMoving. Returns true if the move was legal and applied. If the
// move disconnects the board (only possible when Advanced is on — see
// MoveMoves), every component not containing the moved tile is permanently
// removed from the game (never returned to supply), and any side thereby
// reduced to <=5 total tiles loses immediately.
func (s *GameState) MoveTile(from, to Hex) bool {
	if s.Phase != PhaseMoving {
		return false
	}
	side := s.Turn
	if s.Board.Tiles[from] != side {
		return false
	}
	legal := false
	for _, m := range MoveMoves(s.Board.Tiles, side, s.Advanced) {
		if m.From == from && m.To == to {
			legal = true
			break
		}
	}
	if !legal {
		return false
	}

	candidate := withoutTile(s.Board.Tiles, from)
	candidate[to] = side
	s.markEdge(to)

	var stranded []Hex
	if !Connected(candidate) {
		// Only reachable when Advanced is on (MoveMoves would not have
		// offered a disconnecting move otherwise). Keep only the
		// component containing `to` (the moved tile); every other
		// component is removed from the game outright.
		comps := Components(candidate)
		for _, comp := range comps {
			keep := false
			for _, c := range comp {
				if c == to {
					keep = true
					break
				}
			}
			if keep {
				continue
			}
			for _, c := range comp {
				stranded = append(stranded, c)
				delete(candidate, c)
			}
		}
	}
	s.Board.Tiles = candidate
	s.HasLast = true
	s.LastFrom, s.LastTo = from, to
	s.LastStranded = stranded

	if len(stranded) > 0 {
		// A single disconnecting move can strand tiles belonging to
		// either or both colors. Check the mover's opponent first (a
		// self-sacrificing split that only hurts the opponent should
		// still let the mover go on to win normally), then the mover's
		// own supply — either dropping to <=5 ends the game at once.
		opp := side.Opponent()
		if s.TilesLeft(opp) <= 5 {
			s.Phase = PhaseDone
			s.WinSide = side
			s.WinKind, s.WinCells = ShapeNone, nil
			return true
		}
		if s.TilesLeft(side) <= 5 {
			s.Phase = PhaseDone
			s.WinSide = opp
			s.WinKind, s.WinCells = ShapeNone, nil
			return true
		}
	}

	if s.checkShapeWin(side) {
		return true
	}
	s.Turn = side.Opponent()
	return true
}

// checkShapeWin ends the game if side has just completed a winning shape.
func (s *GameState) checkShapeWin(side Side) bool {
	kind, cells := HasShape(s.Board.Tiles, side)
	if kind == ShapeNone {
		return false
	}
	s.Phase = PhaseDone
	s.WinSide = side
	s.WinKind = kind
	s.WinCells = cells
	return true
}

// StepAI performs exactly one turn's worth of AI action (a placement or a
// tile move) and returns true if it acted. Mirrors this house's established
// aiPend-after-paint pattern: the caller's Draw() shows the human's move
// first, then calls StepAI on the next frame.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	switch s.Phase {
	case PhasePlacement:
		p, ok := AIPlacement(s)
		if !ok {
			return false
		}
		return s.PlaceTile(p)
	case PhaseMoving:
		m, ok := BestMove(s)
		if !ok {
			// No legal move at all: an unreachable-in-practice edge case
			// (every own tile fully boxed in with no legal destination
			// anywhere); treated as a forfeit rather than crashing.
			s.Phase = PhaseDone
			s.WinSide = s.Turn.Opponent()
			return true
		}
		return s.MoveTile(m.From, m.To)
	}
	return false
}
