package game

import "image"

// Opponent selects who the second player is.
type Opponent int

const (
	OpponentHotseat Opponent = iota // two humans take turns
	OpponentAI                      // human is P1 (Svart), AI is P2 (Vit)
)

// Phase is the high-level state of a game.
type Phase int

const (
	PhasePlaying Phase = iota
	PhaseDone
)

// GameState is a full playable Murar (Quoridor) game.
type GameState struct {
	Board    Board
	Turn     Side
	Opponent Opponent
	Phase    Phase
	AIDepth  int // minimax search depth for OpponentAI

	// LastWall holds the most recently placed wall, if the last completed
	// action was a wall placement (nil otherwise), so the UI can highlight
	// it briefly.
	LastWall *Wall
}

// NewGame starts a fresh game in the standard starting position.
func NewGame(opp Opponent, aiDepth int) *GameState {
	return &GameState{
		Board:    NewBoard(),
		Turn:     P1,
		Opponent: opp,
		Phase:    PhasePlaying,
		AIDepth:  aiDepth,
	}
}

// HumanColor returns the side the local human controls in AI mode. In
// hotseat mode both sides are human; this is only meaningful for OpponentAI.
func (s *GameState) HumanColor() Side { return P1 }

// AITurn reports whether it is currently the AI's move.
func (s *GameState) AITurn() bool {
	return s.Opponent == OpponentAI && s.Phase == PhasePlaying && s.Turn == P2
}

// Winner returns the winning side and ok=true once the game has ended.
func (s *GameState) Winner() (Side, bool) { return Winner(&s.Board) }

// PlayMove attempts to move the side to move's pawn to "to". Returns true if
// the move was legal and applied, advancing the turn/phase as needed.
func (s *GameState) PlayMove(to image.Point) bool {
	if s.Phase != PhasePlaying {
		return false
	}
	if !IsLegalPawnMove(&s.Board, s.Turn, to) {
		return false
	}
	s.Board.Pawns[s.Turn] = to
	s.LastWall = nil
	s.advance()
	return true
}

// PlaceWall attempts to place wall w for the side to move. Returns true if it
// was legal (the side has a wall left, the placement doesn't overlap/cross an
// existing wall, and leaves both players a path to their goal) and applied.
func (s *GameState) PlaceWall(w Wall) bool {
	if s.Phase != PhasePlaying {
		return false
	}
	if s.Board.WallsLeft[s.Turn] <= 0 {
		return false
	}
	if !CanPlaceWall(&s.Board, w) {
		return false
	}
	s.Board.place(w)
	s.Board.WallsLeft[s.Turn]--
	wc := w
	s.LastWall = &wc
	s.advance()
	return true
}

// StepAI plays the AI's move (OpponentAI, P2 to move). Returns true if a move
// was made. Caller should redraw after.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	act, ok := BestMove(s.Board, s.Turn, s.AIDepth)
	if !ok {
		// Every reachable Murar position has at least one legal pawn move
		// (CanPlaceWall guarantees a wall never fully closes a path, and a
		// pawn can always at least retreat), so this should be unreachable;
		// kept as a defensive safety net rather than a documented rule.
		s.Phase = PhaseDone
		return true
	}
	s.applyAction(act)
	return true
}

func (s *GameState) applyAction(act Action) {
	if act.IsWall {
		s.Board.place(act.Wall)
		s.Board.WallsLeft[s.Turn]--
		wc := act.Wall
		s.LastWall = &wc
	} else {
		s.Board.Pawns[s.Turn] = act.To
		s.LastWall = nil
	}
	s.advance()
}

// advance checks for a win, then hands the turn to the opponent.
func (s *GameState) advance() {
	if _, ok := Winner(&s.Board); ok {
		s.Phase = PhaseDone
		return
	}
	s.Turn = s.Turn.Opponent()
}
