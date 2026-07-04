package game

import (
	"math/rand"
	"time"
)

// Mode selects opponent type.
type Mode int

const (
	ModeHotseat Mode = iota // two humans take turns
	ModeAI                  // player 0 is human, player 1 is the AI
)

// AI search-difficulty levels (see ai.go).
const (
	DepthEasy   = 1
	DepthMedium = 2
	DepthHard   = 3
)

// Phase is the high-level state of a game. The 4 phases map directly onto
// the 4 transitional UI screens the spec calls for: normal play, the
// round-end wall-tiling reveal (per-tile points), the end-game bonus
// breakdown, and finally the winner banner.
type Phase int

const (
	PhasePlaying Phase = iota
	PhaseTiling        // a round just ended; TileBoard results are ready to show
	PhaseBonus         // the game just ended; end bonuses have been added exactly once
	PhaseDone          // final scores + winner are settled
)

// Move is a single drafting action: take every tile of Color from Source (a
// factory index 0..NumFactories-1, or -1 for the center), and place them on
// pattern line TargetLine (0..WallSize-1), or -1 for the floor.
type Move struct {
	Source     int
	Color      Color
	TargetLine int
}

// TilingResult records what happened when a board was tiled at round end,
// for the UI's per-tile-points display.
type TilingResult struct {
	Placements   []Placement
	FloorPenalty int
	ScoreBefore  int
	ScoreAfter   int
}

// Bonuses records the end-game bonus breakdown for one board.
type Bonuses struct {
	Rows, Cols, Colors, Total int
}

// GameState is a full playable Mosaik game between two boards (index 0 and
// 1; in ModeAI, 0 is the human and 1 is the AI).
type GameState struct {
	Boards [2]Board

	Bag []Color
	Lid []Color // discard pile, reshuffled into Bag once it runs dry

	Factories      [NumFactories][]Color
	Center         []Color
	CenterHasStart bool // the start-player marker is still unclaimed in the center

	Turn      int // 0 or 1: whose turn it is to draft (meaningful in PhasePlaying)
	StartNext int // who starts the next round (updated when the marker is claimed)
	RoundNum  int

	Mode    Mode
	Phase   Phase
	AILevel int

	LastTiling [2]TilingResult // valid once Phase == PhaseTiling (or later)
	Bonuses    [2]Bonuses      // valid once Phase == PhaseBonus (or later)

	rng *rand.Rand
}

// NewGame starts a fresh game with a randomly shuffled bag and factories.
func NewGame(mode Mode, aiLevel int) *GameState {
	return NewGameSeeded(mode, aiLevel, time.Now().UnixNano())
}

// NewGameSeeded starts a fresh game with a deterministic shuffle, for tests
// and reproducible screenshots.
func NewGameSeeded(mode Mode, aiLevel int, seed int64) *GameState {
	gs := &GameState{
		Mode:    mode,
		Phase:   PhasePlaying,
		AILevel: aiLevel,
		rng:     rand.New(rand.NewSource(seed)),
	}
	gs.Bag = NewBag(gs.rng)
	gs.refillFactories()
	gs.Turn = 0
	gs.StartNext = 0
	return gs
}

// AITurn reports whether it is currently the AI's move.
func (gs *GameState) AITurn() bool {
	return gs.Mode == ModeAI && gs.Phase == PhasePlaying && gs.Turn == 1
}

// LegalMoves returns every legal Move for the side currently to move.
func (gs *GameState) LegalMoves() []Move {
	return LegalMoves(gs, gs.Turn)
}

// LegalMoves returns every legal Move for `side`, drafting from the
// factories and center as they currently stand and placing per side's own
// board's pattern-line legality (see Board.lineAccepts). The floor is always
// offered as a target for every (source, color) pair — real Azul lets a
// player voluntarily dump tiles on the floor even when a legal line exists.
func LegalMoves(gs *GameState, side int) []Move {
	b := &gs.Boards[side]
	var out []Move
	addSource := func(source int, tiles []Color) {
		for _, c := range colorsPresent(tiles) {
			out = append(out, Move{Source: source, Color: c, TargetLine: -1})
			for i := 0; i < WallSize; i++ {
				if b.lineAccepts(i, c) {
					out = append(out, Move{Source: source, Color: c, TargetLine: i})
				}
			}
		}
	}
	for f := 0; f < NumFactories; f++ {
		if len(gs.Factories[f]) > 0 {
			addSource(f, gs.Factories[f])
		}
	}
	if len(gs.Center) > 0 {
		addSource(-1, gs.Center)
	}
	return out
}

// legal reports whether m is currently a legal move for the side to move.
func (gs *GameState) legal(m Move) bool {
	for _, cand := range gs.LegalMoves() {
		if cand == m {
			return true
		}
	}
	return false
}

// Play attempts to apply m for the side currently to move. Returns true if
// it was legal and applied (drafting the tiles, placing/overflowing them,
// passing the turn, and — if that was the round's last draft action —
// running wall-tiling and transitioning to PhaseTiling).
func (gs *GameState) Play(m Move) bool {
	if gs.Phase != PhasePlaying {
		return false
	}
	if !gs.legal(m) {
		return false
	}
	gs.applyMove(m)
	return true
}

func (gs *GameState) applyMove(m Move) {
	side := gs.Turn
	b := &gs.Boards[side]

	var taken []Color
	if m.Source == -1 {
		taken, gs.Center = extractColor(gs.Center, m.Color)
		if gs.CenterHasStart {
			gs.CenterHasStart = false
			b.HasMarker = true
			gs.StartNext = side
		}
	} else {
		taken, gs.Factories[m.Source] = extractColor(gs.Factories[m.Source], m.Color)
		leftover := gs.Factories[m.Source]
		gs.Factories[m.Source] = nil
		gs.Center = append(gs.Center, leftover...)
	}

	gs.placeTiles(b, m.TargetLine, taken)
	gs.advanceTurnOrRound()
}

// placeTiles routes `tiles` (all one color) onto pattern line `line`
// (0..WallSize-1) or straight to the floor (line == -1), overflowing any
// tiles beyond the line's remaining room to the floor.
func (gs *GameState) placeTiles(b *Board, line int, tiles []Color) {
	if line == -1 {
		for _, t := range tiles {
			gs.addToFloor(b, t)
		}
		return
	}
	room := (line + 1) - len(b.Lines[line])
	if room < 0 {
		room = 0
	}
	n := len(tiles)
	if n <= room {
		b.Lines[line] = append(b.Lines[line], tiles...)
		return
	}
	b.Lines[line] = append(b.Lines[line], tiles[:room]...)
	for _, t := range tiles[room:] {
		gs.addToFloor(b, t)
	}
}

// addToFloor adds one tile to b's floor line if there is room, or discards
// it straight to the lid if the 7 floor slots (tiles + marker) are already
// full — matching real Azul, where tiles that can't fit on the floor line
// are simply returned to the box with no further penalty.
func (gs *GameState) addToFloor(b *Board, c Color) {
	if b.floorRoom() == 0 {
		gs.Lid = append(gs.Lid, c)
		return
	}
	b.Floor = append(b.Floor, c)
}

// advanceTurnOrRound passes the turn to the other player, or — if that was
// the last draft action of the round (every factory and the center are
// empty) — runs wall-tiling for both boards and moves to PhaseTiling.
func (gs *GameState) advanceTurnOrRound() {
	if gs.draftingOver() {
		gs.doTiling()
		return
	}
	gs.Turn = 1 - gs.Turn
}

// draftingOver reports whether every factory and the center are empty of
// tiles (the marker, if somehow still unclaimed, does not block this — see
// the comment on GameState.CenterHasStart in doc comments elsewhere; in
// practice any tile routed to the center is always claimed together with
// the marker on the first center take).
func (gs *GameState) draftingOver() bool {
	if len(gs.Center) > 0 {
		return false
	}
	for _, f := range gs.Factories {
		if len(f) > 0 {
			return false
		}
	}
	return true
}

// doTiling runs end-of-round wall-tiling + scoring for both boards, records
// the results for the UI, and transitions to PhaseTiling. Game-end detection
// and end bonuses are deliberately NOT applied here — they only happen once
// the UI/caller calls Continue(), so "the round finishes tiling before the
// game-end trigger fires" is an explicit, testable step.
func (gs *GameState) doTiling() {
	for i := range gs.Boards {
		before := gs.Boards[i].Score
		placements, pen, discarded := TileBoard(&gs.Boards[i])
		gs.Lid = append(gs.Lid, discarded...)
		gs.LastTiling[i] = TilingResult{
			Placements:   placements,
			FloorPenalty: pen,
			ScoreBefore:  before,
			ScoreAfter:   gs.Boards[i].Score,
		}
	}
	gs.Phase = PhaseTiling
}

// Continue advances past a transitional phase: from PhaseTiling, either into
// PhaseBonus (if a wall row was completed by either board this round —
// applying end bonuses exactly once) or straight into the next round
// (PhasePlaying); from PhaseBonus into PhaseDone. Returns false if the
// current phase has nothing to continue from (PhasePlaying or PhaseDone).
func (gs *GameState) Continue() bool {
	switch gs.Phase {
	case PhaseTiling:
		if gs.overAfterTiling() {
			gs.applyEndBonusesOnce()
			gs.Phase = PhaseBonus
		} else {
			gs.startNewRound()
		}
		return true
	case PhaseBonus:
		gs.Phase = PhaseDone
		return true
	}
	return false
}

func (gs *GameState) overAfterTiling() bool {
	return gameOver(&gs.Boards[0].Wall) || gameOver(&gs.Boards[1].Wall)
}

// applyEndBonusesOnce computes and adds the end-game bonuses for both
// boards. Called exactly once, from Continue(), on the PhaseTiling ->
// PhaseBonus transition.
func (gs *GameState) applyEndBonusesOnce() {
	for i := range gs.Boards {
		rows, cols, colors, total := endBonusesDetailed(&gs.Boards[i].Wall)
		gs.Bonuses[i] = Bonuses{Rows: rows, Cols: cols, Colors: colors, Total: total}
		gs.Boards[i].Score += total
	}
}

func (gs *GameState) startNewRound() {
	gs.RoundNum++
	gs.refillFactories()
	gs.Turn = gs.StartNext
	gs.Phase = PhasePlaying
}

// refillFactories draws FactoryFill tiles into each factory from the bag
// (reshuffling the lid into the bag if it runs dry) and puts the start
// marker back in the (now-cleared) center.
func (gs *GameState) refillFactories() {
	for i := range gs.Factories {
		gs.Factories[i] = gs.draw(FactoryFill)
	}
	gs.Center = nil
	gs.CenterHasStart = true
}

// draw removes up to n tiles from the bag, reshuffling the lid into the bag
// if it runs dry. Returns fewer than n tiles only once the entire 100-tile
// supply (bag + lid) is genuinely exhausted.
func (gs *GameState) draw(n int) []Color {
	out := make([]Color, 0, n)
	for len(out) < n {
		if len(gs.Bag) == 0 {
			if len(gs.Lid) == 0 {
				break
			}
			gs.Bag, gs.Lid = gs.Lid, nil
			shuffle(gs.Bag, gs.rng)
		}
		last := len(gs.Bag) - 1
		out = append(out, gs.Bag[last])
		gs.Bag = gs.Bag[:last]
	}
	return out
}

// Winner returns the winning player index (0 or 1), or -1 for a shared win
// (equal score AND equal complete-row count). Only meaningful once Phase ==
// PhaseDone.
func (gs *GameState) Winner() int {
	s0, s1 := gs.Boards[0].Score, gs.Boards[1].Score
	if s0 != s1 {
		if s0 > s1 {
			return 0
		}
		return 1
	}
	r0, r1 := completeRows(&gs.Boards[0].Wall), completeRows(&gs.Boards[1].Wall)
	if r0 != r1 {
		if r0 > r1 {
			return 0
		}
		return 1
	}
	return -1
}

// StepAI plays the AI's move (ModeAI, player 1's turn). Returns true if a
// move was made.
func (gs *GameState) StepAI() bool {
	if !gs.AITurn() {
		return false
	}
	mv, ok := BestMove(gs, gs.Turn, gs.AILevel)
	if !ok {
		return false
	}
	return gs.Play(mv)
}

// Clone returns a deep copy of gs, independent of the original — used by the
// AI's lookahead search to simulate candidate moves without touching the
// real game. The search never recurses past a round ending (PhaseTiling), so
// it never calls startNewRound/draw and never needs its own independent rng;
// the shared *rand.Rand reference is therefore never consumed by a clone.
func (gs *GameState) Clone() *GameState {
	ng := *gs
	for i := range ng.Boards {
		ng.Boards[i] = gs.Boards[i].Clone()
	}
	ng.Bag = append([]Color(nil), gs.Bag...)
	ng.Lid = append([]Color(nil), gs.Lid...)
	for i := range ng.Factories {
		ng.Factories[i] = append([]Color(nil), gs.Factories[i]...)
	}
	ng.Center = append([]Color(nil), gs.Center...)
	return &ng
}
