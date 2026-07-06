package game

// Player indexes the two sides of a game: P0 always moves first.
type Player int

const (
	P0 Player = iota
	P1
)

// Opponent returns the other player.
func (p Player) Opponent() Player {
	if p == P0 {
		return P1
	}
	return P0
}

// Opponent selects who the second player is.
type Opponent int

const (
	OpponentHotseat Opponent = iota // two humans take turns
	OpponentAI                      // P0 is human, P1 is the perfect AI
)

// Phase is the high-level state of a game.
type Phase int

const (
	PhasePlaying Phase = iota
	PhaseDone
)

// SizePreset is a named board size offered on the menu. Difficulty here
// means board size (more rows/columns to reason about), NOT a weaker AI —
// the AI is always the same perfect-play search regardless of size.
type SizePreset struct {
	Name       string
	Rows, Cols int
}

// Sizes are the three difficulty/board-size options offered in the menu.
var Sizes = []SizePreset{
	{Name: "Lätt", Rows: 4, Cols: 4},
	{Name: "Medel", Rows: 5, Cols: 6},
	{Name: "Svår", Rows: 6, Cols: 7},
}

// GameState is a full playable Chomp game.
type GameState struct {
	Board    State
	Turn     Player
	Opponent Opponent
	Phase    Phase
	Winner   Player // valid only once Phase == PhaseDone
	Rows     int
	Cols     int
}

// NewGame starts a fresh game on a full rows x cols rectangle.
func NewGame(rows, cols int, opp Opponent) *GameState {
	return &GameState{
		Board:    NewState(rows, cols),
		Turn:     P0,
		Opponent: opp,
		Phase:    PhasePlaying,
		Rows:     rows,
		Cols:     cols,
	}
}

// AITurn reports whether it is currently the AI's move.
func (g *GameState) AITurn() bool {
	return g.Opponent == OpponentAI && g.Phase == PhasePlaying && g.Turn == P1
}

// MoverCanWin reports whether the player currently to move holds a
// theoretical win with perfect play from here on — used for the same
// AI-honesty display nim's solo mode already ships. Only meaningful while
// the game is still in progress.
func (g *GameState) MoverCanWin() bool {
	if g.Phase != PhasePlaying {
		return false
	}
	return WinningForMover(g.Board)
}

// Play attempts to eat cell m for the side to move. Returns true if the move
// was legal and applied. Eating the poisoned cell (0,0) ends the game
// immediately: the mover loses, the OTHER player wins. Any other move just
// advances the turn.
func (g *GameState) Play(m Move) bool {
	if g.Phase != PhasePlaying {
		return false
	}
	if !g.Board.IsLegal(m) {
		return false
	}
	mover := g.Turn
	g.Board = g.Board.Apply(m)
	if m.IsPoison() {
		g.Phase = PhaseDone
		g.Winner = mover.Opponent()
		return true
	}
	g.Turn = mover.Opponent()
	return true
}

// StepAI plays the AI's move (OpponentAI, P1 to move). Returns true if a
// move was made. Caller should redraw after. ok is false only if the AI has
// no legal move at all, which cannot happen while Phase == PhasePlaying: the
// board always holds at least the poisoned cell until someone takes it, and
// taking it is itself always a legal move that ends the game.
func (g *GameState) StepAI() bool {
	if !g.AITurn() {
		return false
	}
	m, ok := BestMove(g.Board)
	if !ok {
		return false
	}
	return g.Play(m)
}
