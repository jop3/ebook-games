package game

// Mode selects opponent type.
type Mode int

const (
	ModeHotseat Mode = iota // two humans take turns
	ModeAI                  // human is player 1, AI is player 2
)

// Phase is the high-level state of a game.
type Phase int

const (
	PhasePlaying Phase = iota // normal play: PlaceThenGive drives whose action is next
	PhaseWon                  // CurrentPlayer just placed the winning piece
	PhaseDraw                 // board full, no winning line
)

// Step distinguishes the two actions within a turn.
type Step int

const (
	StepPlace Step = iota // must place ActivePiece on the board
	StepGive               // must choose a piece from the pool to hand to the opponent
)

// GameState is a full playable Quarto game.
type GameState struct {
	Board   Board
	Pool    []Piece // remaining unplaced, unhanded pieces, ascending order
	Turn    int     // 0 or 1: whose action it is
	Step    Step
	Phase   Phase
	Mode    Mode
	AILevel int // minimax search depth for ModeAI

	ActivePiece Piece // the piece the current player must place (NoPiece before the first give)
	WinLine     [4]int
}

// NewGame starts a fresh game. Player 0 always starts by choosing the first
// piece to hand to player 1 (there's no piece to place yet).
func NewGame(mode Mode, aiLevel int) *GameState {
	pool := make([]Piece, 0, NumPieces)
	for p := Piece(0); p < NumPieces; p++ {
		pool = append(pool, p)
	}
	return &GameState{
		Board:       NewBoard(),
		Pool:        pool,
		Turn:        0,
		Step:        StepGive, // player 0's first action is to hand a piece to player 1
		Phase:       PhasePlaying,
		Mode:        mode,
		AILevel:     aiLevel,
		ActivePiece: NoPiece,
	}
}

// HumanPlayer returns the player index the local human controls in AI mode.
func (s *GameState) HumanPlayer() int { return 0 }

// AITurn reports whether it is currently the AI's action.
func (s *GameState) AITurn() bool {
	return s.Mode == ModeAI && s.Phase == PhasePlaying && s.Turn == 1
}

// removeFromPool removes p from the pool (must be present).
func (s *GameState) removeFromPool(p Piece) bool {
	for i, q := range s.Pool {
		if q == p {
			s.Pool = append(s.Pool[:i], s.Pool[i+1:]...)
			return true
		}
	}
	return false
}

// PlacePiece places s.ActivePiece at (x,y) for the current player. Returns
// true if applied. Handles win/draw detection and advances to StepGive (or
// ends the game).
func (s *GameState) PlacePiece(x, y int) bool {
	if s.Phase != PhasePlaying || s.Step != StepPlace {
		return false
	}
	if s.ActivePiece == NoPiece {
		return false
	}
	if !s.Board.Place(x, y, s.ActivePiece) {
		return false
	}
	s.ActivePiece = NoPiece
	if ln, ok := s.Board.WinningLine(); ok {
		s.WinLine = ln
		s.Phase = PhaseWon
		return true
	}
	if s.Board.Full() {
		s.Phase = PhaseDraw
		return true
	}
	s.Step = StepGive
	return true
}

// GivePiece hands piece p from the pool to the opponent, passing the turn.
func (s *GameState) GivePiece(p Piece) bool {
	if s.Phase != PhasePlaying || s.Step != StepGive {
		return false
	}
	if !s.removeFromPool(p) {
		return false
	}
	s.ActivePiece = p
	s.Turn = 1 - s.Turn
	s.Step = StepPlace
	return true
}

// Winner returns the player index (0 or 1) who completed the winning line.
// Only meaningful when Phase == PhaseWon: the player who just placed (i.e.
// NOT s.Turn, since PlacePiece doesn't advance Turn) is the winner.
func (s *GameState) Winner() int { return s.Turn }

// StepAI performs the AI's action (place or give) for player 1. Returns true
// if an action was taken. Caller should redraw after; call again if the AI
// needs to both place and give in immediate succession (typical: two calls).
func (s *GameState) StepAI() bool {
	if !s.AITurn() {
		return false
	}
	switch s.Step {
	case StepPlace:
		x, y := BestPlacement(s, s.AILevel)
		return s.PlacePiece(x, y)
	case StepGive:
		p := BestGive(s, s.AILevel)
		return s.GivePiece(p)
	}
	return false
}
