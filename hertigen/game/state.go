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

// GameState is a full playable Hertigen game.
type GameState struct {
	Board    Board
	Reserve  [2]ReserveMask // indexed by Side
	Turn     Side
	Opponent Opponent
	Phase    Phase
	AIDepth  int // alpha-beta search depth for OpponentAI

	// LastCaptured holds the cell(s) that lost a tile to the most recently
	// applied action (nil if it captured nothing), so the UI can briefly
	// mark them before the next action replaces the list.
	LastCaptured []image.Point

	// stalemated records that the side named by Turn had no legal action at
	// all (no tile can move/jump/strike, and no legal recruit either) when
	// it became their turn. Hertigen defines no pass, so this is treated as
	// an immediate forfeit — mirroring the same defensive convention hasami
	// uses for its own essentially-unreachable no-moves edge case.
	stalemated bool
}

// NewGame starts a fresh game in the standard starting position.
func NewGame(opp Opponent, aiDepth int) *GameState {
	return &GameState{
		Board:    NewBoard(),
		Reserve:  [2]ReserveMask{Black: NewReserve(), White: NewReserve()},
		Turn:     Black,
		Opponent: opp,
		Phase:    PhasePlaying,
		AIDepth:  aiDepth,
	}
}

// HumanColor returns the color the local human controls in AI mode. In
// hotseat mode both colors are human; this is only meaningful for OpponentAI.
func (s *GameState) HumanColor() Side { return Black }

// AITurn reports whether it is currently the AI's move.
func (s *GameState) AITurn() bool {
	return s.Opponent == OpponentAI && s.Phase == PhasePlaying && s.Turn == White
}

// Winner returns the winning side and true once the game is over; the
// second return is false while play continues. Capturing the opposing
// Duke is the only way to win, aside from the defensive stalemate-forfeit
// rule.
func (s *GameState) Winner() (Side, bool) {
	if s.stalemated {
		return s.Turn.Opponent(), true
	}
	if _, ok := s.Board.DukePos(Black); !ok {
		return White, true
	}
	if _, ok := s.Board.DukePos(White); !ok {
		return Black, true
	}
	return Black, false
}

// LegalActions returns every legal action for the side to move right now.
func (s *GameState) LegalActions() []Action {
	if s.Phase != PhasePlaying {
		return nil
	}
	return s.Board.LegalActions(s.Turn, s.Reserve[s.Turn])
}

// Play attempts to apply action a on behalf of the side to move. Returns
// true if it was legal and applied, advancing the turn/phase as needed.
func (s *GameState) Play(a Action) bool {
	if s.Phase != PhasePlaying {
		return false
	}
	if !s.Board.IsLegalAction(s.Turn, s.Reserve[s.Turn], a) {
		return false
	}
	s.apply(a)
	s.advance()
	return true
}

// apply mutates Board/Reserve/LastCaptured for action a, assumed legal.
func (s *GameState) apply(a Action) {
	if a.Kind == ActRecruit {
		nb := s.Board
		nb.set(a.To.X, a.To.Y, &Tile{Type: a.Recruit, Side: s.Turn, Face: FaceA})
		s.Board = nb
		s.Reserve[s.Turn] = s.Reserve[s.Turn].Remove(a.Recruit)
		s.LastCaptured = nil
		return
	}
	nb, captured := s.Board.Apply(a)
	s.Board = nb
	s.LastCaptured = captured
}

// StepAI plays the AI's move (OpponentAI, White to move). Returns true if a
// move was made. Caller should redraw after.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	a, ok := BestMove(s.Board, s.Reserve, White, s.AIDepth)
	if !ok {
		// No legal action at all: forfeit (advance() below would already
		// have caught this before handing White the turn; kept as a safety
		// net, matching hasami's convention).
		s.Phase = PhaseDone
		s.stalemated = true
		return true
	}
	s.apply(a)
	s.advance()
	return true
}

// advance checks for a Duke-capture win, then hands the turn to the
// opponent — unless the opponent has no legal action at all, in which case
// they forfeit immediately (see the stalemated doc comment above).
func (s *GameState) advance() {
	if _, ok := s.Board.DukePos(Black); !ok {
		s.Phase = PhaseDone
		return
	}
	if _, ok := s.Board.DukePos(White); !ok {
		s.Phase = PhaseDone
		return
	}
	next := s.Turn.Opponent()
	if len(s.Board.LegalActions(next, s.Reserve[next])) == 0 {
		s.Turn = next
		s.Phase = PhaseDone
		s.stalemated = true
		return
	}
	s.Turn = next
}
