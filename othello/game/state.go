package game

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

// GameState is a full playable Othello game.
type GameState struct {
	Board    Board
	Turn     Cell // Black or White
	Mode     Mode
	Phase    Phase
	Passed   bool // did the previous ply have to pass?
	LastPass bool // was the most recent transition a forced pass? (for status)
	AILevel  int  // minimax search depth for ModeAI
}

// NewGame starts a fresh game.
func NewGame(mode Mode, aiLevel int) *GameState {
	return &GameState{
		Board:   NewBoard(),
		Turn:    Black,
		Mode:    mode,
		Phase:   PhasePlaying,
		AILevel: aiLevel,
	}
}

// HumanColor returns the color the local human controls in AI mode. In hotseat
// both colors are human; this is only meaningful for ModeAI.
func (s *GameState) HumanColor() Cell { return Black }

// AITurn reports whether it is currently the AI's move.
func (s *GameState) AITurn() bool {
	return s.Mode == ModeAI && s.Phase == PhasePlaying && s.Turn == White
}

// Play attempts a human move at (x,y). Returns true if it was applied. After a
// legal move it advances the turn, handling passes and game end.
func (s *GameState) Play(x, y int) bool {
	if s.Phase != PhasePlaying {
		return false
	}
	if !s.Board.Apply(x, y, s.Turn) {
		return false
	}
	s.advance()
	return true
}

// advance moves to the next player, skipping a player with no legal move, and
// ends the game when neither can move.
func (s *GameState) advance() {
	next := s.Turn.Opponent()
	s.LastPass = false
	if s.Board.HasMove(next) {
		s.Turn = next
		return
	}
	// next must pass — does the current player still have a move?
	if s.Board.HasMove(s.Turn) {
		s.LastPass = true // opponent passed, same player continues
		return
	}
	// neither can move
	s.Phase = PhaseDone
}

// Winner returns the winning color, or Empty for a tie. Only meaningful when
// Phase == PhaseDone.
func (s *GameState) Winner() Cell {
	bl, wh := s.Board.Count(Black), s.Board.Count(White)
	switch {
	case bl > wh:
		return Black
	case wh > bl:
		return White
	default:
		return Empty
	}
}

// StepAI plays the AI's move (ModeAI, White to move). Returns true if a move was
// made. Caller should redraw after.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	mv, ok := BestMove(&s.Board, White, s.AILevel)
	if !ok {
		// Shouldn't happen (advance handles passes), but be safe.
		s.advance()
		return true
	}
	s.Board.Apply(mv[0], mv[1], White)
	s.advance()
	return true
}
