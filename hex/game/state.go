package game

// Mode selects opponent type.
type Mode int

const (
	ModeHotseat Mode = iota
	ModeAI           // human plays Black, AI plays White
)

// Phase is the high-level game state.
type Phase int

const (
	PhasePlaying Phase = iota
	PhaseDone
)

// Size presets for the board.
type Preset struct {
	Name string
	N    int
}

// Presets offered on the menu.
var Presets = []Preset{
	{"Litet 7×7", 7},
	{"Mellan 9×9", 9},
	{"Stort 11×11", 11},
}

// GameState is a full playable Hex game.
type GameState struct {
	Board *Board
	Turn  Cell
	Mode  Mode
	Phase Phase
	Win   Cell
}

// NewGame starts a fresh game. Black moves first.
func NewGame(n int, mode Mode) *GameState {
	return &GameState{
		Board: NewBoard(n),
		Turn:  Black,
		Mode:  mode,
		Phase: PhasePlaying,
	}
}

// AITurn reports whether it's the AI's move.
func (s *GameState) AITurn() bool {
	return s.Mode == ModeAI && s.Phase == PhasePlaying && s.Turn == White
}

// Play attempts a move at (x,y) for the current player. Returns true if applied.
func (s *GameState) Play(x, y int) bool {
	if s.Phase != PhasePlaying {
		return false
	}
	if !s.Board.Place(x, y, s.Turn) {
		return false
	}
	s.afterMove()
	return true
}

// afterMove checks for a win and advances the turn. Hex has no passes or draws.
func (s *GameState) afterMove() {
	if w := s.Board.Winner(); w != Empty {
		s.Win = w
		s.Phase = PhaseDone
		return
	}
	s.Turn = s.Turn.Opponent()
}

// StepAI plays the AI's move. Returns true if a move was made.
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	mv, ok := BestMove(s.Board, White)
	if !ok {
		return false
	}
	s.Board.Set(mv[0], mv[1], White)
	s.afterMove()
	return true
}
