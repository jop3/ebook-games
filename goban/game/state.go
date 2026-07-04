package game

import "image"

// Opponent selects who the second player is.
type Opponent int

const (
	OpponentHotseat Opponent = iota // two humans take turns
	OpponentAI                      // human is Black, AI is White (9x9 only)
)

// Phase is the high-level state of a game.
type Phase int

const (
	PhasePlaying Phase = iota // normal alternating placement
	PhaseMarking              // two passes happened; tap groups to mark them dead
	PhaseDone                 // final area score has been computed
)

// DefaultKomi is White's default compensation on a 9x9 board.
const DefaultKomi = 6.5

// GameState is a full playable Go game.
type GameState struct {
	Board    Board
	Turn     Color
	Opponent Opponent
	Komi     float64
	Phase    Phase

	// priorBoard is the board position immediately before the most recently
	// applied move, used for the simple positional ko check on the NEXT move.
	priorBoard    Board
	hasPriorBoard bool

	ConsecutivePasses int
	LastPass          bool // was the most recent transition a pass?

	LastMove     image.Point
	HasLastMove  bool
	LastCaptured []image.Point

	// Dead marks stones toggled dead during PhaseMarking; nil before then.
	Dead map[image.Point]bool

	BlackScore, WhiteScore float64
}

// NewGame starts a fresh, empty game on a size x size board.
func NewGame(size int, opp Opponent, komi float64) *GameState {
	return &GameState{
		Board:    NewBoard(size),
		Turn:     Black,
		Opponent: opp,
		Komi:     komi,
		Phase:    PhasePlaying,
	}
}

// HumanColor returns the color the local human controls in AI mode. In
// hotseat mode both colors are human; this is only meaningful for OpponentAI.
func (s *GameState) HumanColor() Color { return Black }

// AITurn reports whether it is currently the (9x9-only) AI's move.
func (s *GameState) AITurn() bool {
	return s.Opponent == OpponentAI && s.Phase == PhasePlaying && s.Turn == White
}

// koPrevPtr returns a pointer to the ko-check board, or nil if none is set
// yet (i.e. no move has been played).
func (s *GameState) koPrevPtr() *Board {
	if !s.hasPriorBoard {
		return nil
	}
	return &s.priorBoard
}

// Play attempts to place the side to move's stone at (x,y). Returns true if
// the move was legal and applied, advancing the turn and clearing the pass
// counter. Only legal during PhasePlaying.
func (s *GameState) Play(x, y int) bool {
	if s.Phase != PhasePlaying {
		return false
	}
	p := image.Pt(x, y)
	if !Legal(s.Board, p, s.Turn, s.koPrevPtr()) {
		return false
	}
	before := s.Board.Clone()
	nb, captured, ok := Place(s.Board, p, s.Turn)
	if !ok {
		return false
	}
	s.priorBoard = before
	s.hasPriorBoard = true
	s.Board = nb
	s.LastMove = p
	s.HasLastMove = true
	s.LastCaptured = captured
	s.ConsecutivePasses = 0
	s.LastPass = false
	s.Turn = s.Turn.Opponent()
	return true
}

// Pass records a pass for the side to move. Two consecutive passes end
// normal play and enter the mark-dead phase. Only legal during PhasePlaying.
func (s *GameState) Pass() bool {
	if s.Phase != PhasePlaying {
		return false
	}
	s.LastPass = true
	s.HasLastMove = false
	s.LastCaptured = nil
	s.ConsecutivePasses++
	if s.ConsecutivePasses >= 2 {
		s.Phase = PhaseMarking
		s.Dead = map[image.Point]bool{}
		return true
	}
	s.Turn = s.Turn.Opponent()
	return true
}

// ToggleDead toggles the dead/alive marking of the whole group occupying
// (x,y) during the mark-dead phase. Tapping an empty point does nothing.
func (s *GameState) ToggleDead(x, y int) bool {
	if s.Phase != PhaseMarking {
		return false
	}
	p := image.Pt(x, y)
	if s.Board.At(p) == Empty {
		return false
	}
	grp := Group(s.Board, p)
	anyDead := false
	for _, q := range grp {
		if s.Dead[q] {
			anyDead = true
			break
		}
	}
	for _, q := range grp {
		if anyDead {
			delete(s.Dead, q)
		} else {
			s.Dead[q] = true
		}
	}
	return true
}

// FinishMarking computes the final area score from the current dead-stone
// marks and moves to PhaseDone.
func (s *GameState) FinishMarking() {
	if s.Phase != PhaseMarking {
		return
	}
	s.BlackScore, s.WhiteScore = AreaScore(s.Board, s.Dead, s.Komi)
	s.Phase = PhaseDone
}

// Winner returns the winning color, or Empty for an exact tie. Only
// meaningful once Phase == PhaseDone.
func (s *GameState) Winner() Color {
	switch {
	case s.BlackScore > s.WhiteScore:
		return Black
	case s.WhiteScore > s.BlackScore:
		return White
	default:
		return Empty
	}
}

// StepAI plays the (9x9-only) AI's move. If the human's previous move was a
// pass, the AI mirrors it with a pass of its own, ending the game via the
// standard double-pass rule rather than trying to out-maneuver a human who
// has signaled the game is over. Otherwise it plays its best heuristic move,
// or passes if none is available. Returns true if an action was taken;
// caller should redraw after.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	if s.LastPass {
		return s.Pass()
	}
	p, ok := BestMove(s.Board, s.Turn, s.koPrevPtr())
	if !ok {
		return s.Pass()
	}
	return s.Play(p.X, p.Y)
}
