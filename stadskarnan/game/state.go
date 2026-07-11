package game

import "image"

// Hand records, per piece ID (0..NumPieces-1), whether that piece is still
// available to place (true) — either never yet placed, or placed and then
// captured back. It says nothing about board position.
type Hand [NumPieces]bool

// NewHand returns a hand with every piece available.
func NewHand() Hand {
	var h Hand
	for i := range h {
		h[i] = true
	}
	return h
}

// RemainingSquares sums the sizes of every still-available piece in the
// hand — the quantity the win condition compares (fewer is better).
func (h *Hand) RemainingSquares() int {
	total := 0
	for id, avail := range h {
		if avail {
			total += Pieces[id].Size()
		}
	}
	return total
}

// Opponent selects who the second player is.
type Opponent int

const (
	OpponentHotseat Opponent = iota // two humans take turns
	OpponentAI                      // human is Black, AI is White
)

// Phase is the high-level state of a game.
type Phase int

const (
	// PhaseCathedral: the neutral Cathedral piece has not yet been placed.
	// By convention Black places it (see NewGame); this uses up Black's
	// first turn, so White moves first once it is down.
	PhaseCathedral Phase = iota
	PhasePlaying
	PhaseDone
)

// GameState is a full playable Stadskärnan game.
type GameState struct {
	Board    Board
	Turn     Cell // Black or White; meaningful in every phase
	Phase    Phase
	Opponent Opponent
	Hands    [2]Hand // index via handIndex(side)

	// LastCaptured/LastSealed record the cells affected by the most recently
	// applied placement (nil if none), so the UI can briefly mark them
	// before the next placement replaces the list.
	LastCaptured []image.Point
	LastSealed   []image.Point
}

func handIndex(side Cell) int {
	if side == Black {
		return 0
	}
	return 1
}

// Hand returns the given side's hand (Black or White only).
func (s *GameState) Hand(side Cell) *Hand { return &s.Hands[handIndex(side)] }

// NewGame starts a fresh game: an empty board, both hands full, Black about
// to place the Cathedral.
func NewGame(opp Opponent) *GameState {
	return &GameState{
		Board:    NewBoard(),
		Turn:     Black,
		Phase:    PhaseCathedral,
		Opponent: opp,
		Hands:    [2]Hand{NewHand(), NewHand()},
	}
}

// HumanColor returns the color the local human controls in AI mode (and the
// side that always places the Cathedral). In hotseat mode both colors are
// human; this is only meaningful for OpponentAI.
func (s *GameState) HumanColor() Cell { return Black }

// AITurn reports whether it is currently the AI's move (placing a building;
// the AI never places the Cathedral, since Black always does that and Black
// is always the human).
func (s *GameState) AITurn() bool {
	return s.Opponent == OpponentAI && s.Phase == PhasePlaying && s.Turn == White
}

// PlaceCathedral attempts to place the neutral Cathedral piece so that its
// canonical anchor cell lands on anchor. Only legal during PhaseCathedral.
// On success, advances to PhasePlaying with White to move first.
func (s *GameState) PlaceCathedral(anchor image.Point) bool {
	if s.Phase != PhaseCathedral {
		return false
	}
	for _, p := range LegalCathedralPlacements(&s.Board) {
		if p.Anchor == anchor {
			for _, c := range p.Cells {
				s.Board.Owner[c.Y][c.X] = Cathedral
			}
			s.Phase = PhasePlaying
			s.Turn = White
			return true
		}
	}
	return false
}

// Place attempts to have side place piece pieceID, in orientation
// orientIdx (an index into Orientations(Pieces[pieceID].Cells)), anchored at
// anchor. Returns true if the placement was legal and applied: the turn
// (and, if the game just ended, Winner) is updated as needed.
func (s *GameState) Place(side Cell, pieceID, orientIdx int, anchor image.Point) bool {
	if s.Phase != PhasePlaying || s.Turn != side {
		return false
	}
	if pieceID < 0 || pieceID >= NumPieces || !s.Hand(side)[pieceID] {
		return false
	}
	placements := LegalPlacementsForOrientation(&s.Board, pieceID, orientIdx)
	var chosen *Placement
	for i := range placements {
		if placements[i].Anchor == anchor {
			chosen = &placements[i]
			break
		}
	}
	if chosen == nil {
		return false
	}
	for _, c := range chosen.Cells {
		s.Board.Owner[c.Y][c.X] = side
		s.Board.PieceID[c.Y][c.X] = int8(pieceID)
	}
	s.Hand(side)[pieceID] = false

	// Enclosure re-floods regions sealed on EARLIER moves too (its membership
	// test ignores Sealed), so filter against the pre-move state to record
	// only this placement's newly sealed cells — otherwise LastSealed grows
	// into the whole seal history.
	wasSealed := s.Board.Sealed
	sealed, captured := Enclosure(&s.Board)
	newly := sealed[:0]
	for _, p := range sealed {
		if !wasSealed[p.Y][p.X] {
			newly = append(newly, p)
		}
	}
	s.LastSealed = newly
	var flash []image.Point
	for _, cp := range captured {
		s.Hand(cp.Owner)[cp.PieceID] = true
		flash = append(flash, cp.Cells...)
	}
	s.LastCaptured = flash

	s.advance(side)
	return true
}

// advance hands the turn to the opponent, unless the opponent currently has
// no legal placement at all (in which case they pass and it stays this
// side's turn), unless NEITHER side can place anything, in which case the
// game ends. The check is performed fresh every call rather than assuming a
// stuck side stays stuck, per the rules (a later placement elsewhere on the
// board can free up space for a side that was previously unable to move).
func (s *GameState) advance(mover Cell) {
	if !s.Board.HasAnyLegalPlacement(s.Hand(Black)) && !s.Board.HasAnyLegalPlacement(s.Hand(White)) {
		s.Phase = PhaseDone
		return
	}
	next := mover.Opponent()
	if s.Board.HasAnyLegalPlacement(s.Hand(next)) {
		s.Turn = next
	} else {
		s.Turn = mover // next side passes; mover goes again
	}
}

// StepAI plays the AI's move (OpponentAI, White to move). Returns true if a
// move was made. Caller should redraw after.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	pieceID, orientIdx, anchor, ok := BestPlacement(s, White)
	if !ok {
		// Shouldn't happen: advance() only ever hands White the turn when
		// White has at least one legal placement. Handled defensively.
		s.Phase = PhaseDone
		return true
	}
	return s.Place(White, pieceID, orientIdx, anchor)
}

// Winner returns the winning color, or Empty for a tie. Only meaningful once
// Phase == PhaseDone: the side with fewer total unplaced squares remaining in
// hand wins (i.e. whoever got more of their own building area onto the
// board).
func (s *GameState) Winner() Cell {
	rb := s.Hand(Black).RemainingSquares()
	rw := s.Hand(White).RemainingSquares()
	switch {
	case rb < rw:
		return Black
	case rw < rb:
		return White
	default:
		return Empty
	}
}
