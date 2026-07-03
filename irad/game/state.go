package game

// GameState bundles the board with turn/phase/winner bookkeeping and the
// opponent configuration. It owns the state machine from §7 of the spec,
// generalised to 2..MaxPlayers hot-seat players (with an optional single AI
// opponent in the two-player case).
type GameState struct {
	Board      Board
	Preset     Preset // kept so "spela igen" can rebuild an identical board
	NumPlayers int    // active players this match, 2..MaxPlayers
	Turn       Player
	Phase      Phase
	Winner     Player // PlayerNone until the game ends
	Selected   int    // selected own stone during moving phase, -1 = none
	VsAI       bool
	AIPlayer   Player // which seat the AI controls when VsAI (only in 2-player)
}

// NewGame starts a fresh two-player match. If vsAI, the AI plays Player2.
// It is kept for backward compatibility and delegates to NewGameN.
func NewGame(p Preset, vsAI bool) *GameState {
	gs := NewGameN(p, 2)
	gs.VsAI = vsAI
	gs.AIPlayer = Player2
	return gs
}

// NewGameN starts a fresh hot-seat match with the given number of human
// players (clamped to 2..MaxPlayers). AI is off by default.
func NewGameN(p Preset, numPlayers int) *GameState {
	if numPlayers < 2 {
		numPlayers = 2
	}
	if numPlayers > MaxPlayers {
		numPlayers = MaxPlayers
	}
	return &GameState{
		Board:      NewBoard(p),
		Preset:     p,
		NumPlayers: numPlayers,
		Turn:       Player1,
		Phase:      PhasePlacing,
		Winner:     PlayerNone,
		Selected:   -1,
	}
}

// Restart rebuilds the board from the stored preset, keeping the same player
// configuration. Implements GAME_OVER --("spela igen")--> PLACING.
func (gs *GameState) Restart() {
	gs.Board = NewBoard(gs.Preset)
	gs.Turn = Player1
	gs.Phase = PhasePlacing
	gs.Winner = PlayerNone
	gs.Selected = -1
}

// nextTurn returns the player whose turn follows the current one, cycling
// Player1..PlayerN. With NumPlayers == 2 this matches Other().
func (gs *GameState) nextTurn() Player {
	n := gs.NumPlayers
	if n < 2 {
		n = 2
	}
	// Players are the contiguous range Player1..Player(n). Rotate within it.
	next := int(gs.Turn) + 1
	if next > n { // Player1==1, so the last seat is Player(n) == n
		next = int(Player1)
	}
	return Player(next)
}

// reachedPieceLimit reports whether every active player has placed all of
// their stones in a limited-piece game.
func (gs *GameState) reachedPieceLimit() bool {
	pl := gs.Board.PieceLimit
	if pl <= 0 {
		return false
	}
	for p := 1; p <= gs.NumPlayers; p++ {
		if gs.Board.Placed[p] < pl {
			return false
		}
	}
	return true
}

// ApplyMove performs a legal move for the current player and advances the
// state machine: win detection, draw detection, the global placing→moving
// transition, and turn handover. The move is assumed legal (validated by the
// input layer against ValidMoves). Returns true if the move was applied.
func (gs *GameState) ApplyMove(m Move) bool {
	if gs.Phase == PhaseGameOver {
		return false
	}

	gs.Board = gs.Board.Apply(m, gs.Turn)
	gs.Selected = -1

	// Win check from the last-played cell.
	if w := gs.Board.CheckWin(m.To); w != PlayerNone {
		gs.Winner = w
		gs.Phase = PhaseGameOver
		return true
	}

	// Draw: board full with no winner (only possible while placing).
	if gs.Phase == PhasePlacing && gs.Board.IsFull() {
		gs.Winner = PlayerNone
		gs.Phase = PhaseGameOver
		return true
	}

	// Global placing→moving transition once every player's budget is spent.
	if gs.Phase == PhasePlacing && gs.reachedPieceLimit() {
		gs.Phase = PhaseMoving
	}

	gs.Turn = gs.nextTurn()
	return true
}

// AITurn reports whether it is currently the AI's move. AI is only used in
// two-player matches; with more players every seat is a human (hot-seat).
func (gs *GameState) AITurn() bool {
	return gs.VsAI && gs.NumPlayers == 2 &&
		gs.Phase != PhaseGameOver && gs.Turn == gs.AIPlayer
}
