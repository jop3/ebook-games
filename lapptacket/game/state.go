// Package game implements the pure rules engine for Lapptäcket (a Patchwork-
// style 2-player quilting/economy race) with zero dependency on the ink SDK,
// so it unit-tests without cgo. See track.go for the turn-order/crossing
// rules, shapes.go/board.go for the 9x9 quilt + polyomino placement, and
// ai.go for the heuristic opponent.
package game

import "image"

// Opponent selects hot-seat (both human) or vs-AI play. By convention (as
// in this repo's other 2-player games) the AI, when enabled, is always
// player index 1.
type Opponent int

const (
	OpponentHotseat Opponent = iota
	OpponentAI
)

// Phase is the coarse game state.
type Phase int

const (
	PhasePlaying Phase = iota
	PhaseDone
)

// StartButtons is each player's opening button count.
const StartButtons = 5

// GameState is the whole, perfect-information game: both quilt boards, both
// players' buttons/income/marker position, any free 1x1 patch(es) owed but
// not yet placed, the shared patch queue + neutral-token position, and the
// once-only 7x7 bonus owner.
type GameState struct {
	Boards  [2]Board
	Buttons [2]int
	Income  [2]int // sum of button-income values of all patches placed so far
	Marker  [2]int // position on the linear time track, 0..TrackEnd
	Pending [2]int // free 1x1 patches earned but not yet placed on the board

	ClaimedSpecial []bool // parallel to SpecialPatchPositions
	BonusOwner     int    // -1 = unclaimed, else the player index who scored the 7x7 bonus

	Queue    []Patch // remaining buyable patches, in circular queue order
	TokenIdx int     // index into Queue the neutral token currently precedes

	Opponent Opponent
	Phase    Phase

	// LastBuyPatchID/LastFreePatch are set on the most recent successful
	// action, purely for UI feedback (e.g. "you bought Kors").
	LastBuyPatchID int
}

// NewGame starts a fresh game against the given opponent type.
func NewGame(opp Opponent) *GameState {
	s := &GameState{
		Queue:          NewQueue(),
		Opponent:       opp,
		BonusOwner:     -1,
		ClaimedSpecial: make([]bool, len(SpecialPatchPositions)),
		LastBuyPatchID: -1,
	}
	s.Buttons[0] = StartButtons
	s.Buttons[1] = StartButtons
	return s
}

// ActingPlayer returns which player must act next: if either player owes a
// free-patch placement (crossed a special square) that always takes
// priority for THAT player; otherwise it's the trailing-marker rule.
func (s *GameState) ActingPlayer() int {
	if s.Pending[0] > 0 {
		return 0
	}
	if s.Pending[1] > 0 {
		return 1
	}
	return NextActor(s.Marker[0], s.Marker[1])
}

// AITurn reports whether it's currently the AI's move (player 1, by
// convention, when Opponent == OpponentAI).
func (s *GameState) AITurn() bool {
	return s.Opponent == OpponentAI && s.Phase == PhasePlaying && s.ActingPlayer() == 1
}

// GameOver reports whether both markers have reached the end of the track
// (with no free patches left owed) — the game-end condition.
func (s *GameState) GameOver() bool {
	return s.Marker[0] == TrackEnd && s.Marker[1] == TrackEnd && s.Pending[0] == 0 && s.Pending[1] == 0
}

// NextThree returns up to the 3 patches nearest clockwise to the neutral
// token (fewer if the queue has run low — a rare, harmless edge case).
func (s *GameState) NextThree() []Patch {
	n := len(s.Queue)
	if n == 0 {
		return nil
	}
	m := 3
	if n < m {
		m = n
	}
	out := make([]Patch, m)
	for i := 0; i < m; i++ {
		out[i] = s.Queue[(s.TokenIdx+i)%n]
	}
	return out
}

// moveMarker advances player's marker by delta squares (clamped to
// TrackEnd), then applies any income payouts and free-patch claims for
// every marked square crossed OR landed on along the way — a single big
// jump can trigger several at once.
func (s *GameState) moveMarker(player, delta int) {
	old := s.Marker[player]
	newPos := clampTrack(old + delta)
	s.Marker[player] = newPos

	if n := crossedCount(old, newPos, IncomeSquares); n > 0 {
		s.Buttons[player] += n * s.Income[player]
	}
	for _, i := range crossedSpecialIndices(old, newPos, SpecialPatchPositions, s.ClaimedSpecial) {
		s.ClaimedSpecial[i] = true
		s.Pending[player]++
	}
	s.maybeFinish()
}

func (s *GameState) maybeFinish() {
	if s.GameOver() {
		s.Phase = PhaseDone
	}
}

func (s *GameState) checkSevenBySeven(player int) {
	if s.BonusOwner == -1 && s.Boards[player].SevenBySeven() {
		s.BonusOwner = player
	}
}

// BuyPatch attempts to have player buy the patch at `offset` (0,1,2) into
// NextThree(), in orientation orientIdx (index into
// Orientations(patch.Cells)), anchored at anchor. On success: pays the
// cost, places the patch, removes it from the queue (advancing the neutral
// token past it), and advances the buyer's own marker by the patch's time
// cost (which may itself trigger income/free-patch crossings).
func (s *GameState) BuyPatch(player, offset, orientIdx int, anchor image.Point) bool {
	if s.Phase != PhasePlaying || s.Pending[player] > 0 || player != s.ActingPlayer() {
		return false
	}
	three := s.NextThree()
	if offset < 0 || offset >= len(three) {
		return false
	}
	patch := three[offset]
	if s.Buttons[player] < patch.Cost {
		return false
	}
	orients := Orientations(patch.Cells)
	if orientIdx < 0 || orientIdx >= len(orients) {
		return false
	}
	board := &s.Boards[player]
	cells, ok := canPlaceAt(board, orients[orientIdx], anchor)
	if !ok {
		return false
	}

	s.Buttons[player] -= patch.Cost
	board.Place(cells)
	s.Income[player] += patch.Income
	s.LastBuyPatchID = patch.ID
	s.checkSevenBySeven(player)

	n := len(s.Queue)
	idx := (s.TokenIdx + offset) % n
	s.Queue = append(s.Queue[:idx], s.Queue[idx+1:]...)
	if len(s.Queue) > 0 {
		s.TokenIdx = idx % len(s.Queue)
	} else {
		s.TokenIdx = 0
	}

	s.moveMarker(player, patch.TimeCost)
	return true
}

// Advance has player skip buying, instead moving their own marker to just
// past the OTHER player's marker, gaining 1 button per square advanced.
func (s *GameState) Advance(player int) bool {
	if s.Phase != PhasePlaying || s.Pending[player] > 0 || player != s.ActingPlayer() {
		return false
	}
	other := 1 - player
	target := clampTrack(s.Marker[other] + 1)
	delta := target - s.Marker[player]
	if delta <= 0 {
		return false
	}
	s.Buttons[player] += delta
	s.moveMarker(player, delta)
	return true
}

// PlaceFreePatch places one of player's owed free 1x1 special patches at
// anchor (must be empty and on-board). Worth +1 permanent income.
func (s *GameState) PlaceFreePatch(player int, anchor image.Point) bool {
	if s.Pending[player] <= 0 {
		return false
	}
	x, y := anchor.X, anchor.Y
	if x < 0 || x >= BoardSize || y < 0 || y >= BoardSize {
		return false
	}
	board := &s.Boards[player]
	if board.Filled[y][x] {
		return false
	}
	board.Filled[y][x] = true
	board.Bonus[y][x] = true
	s.Income[player] += FreePatch.Income
	s.Pending[player]--
	s.checkSevenBySeven(player)
	s.maybeFinish()
	return true
}

// FinalScore computes player's end-of-game score: buttons in hand minus 2
// per empty square remaining, plus +7 if they earned the once-only 7x7
// bonus.
func (s *GameState) FinalScore(player int) int {
	score := s.Buttons[player] - 2*s.Boards[player].EmptyCount()
	if s.BonusOwner == player {
		score += 7
	}
	return score
}

// Winner returns the player index with the higher FinalScore, or -1 on a
// tie. Only meaningful once GameOver() is true.
func (s *GameState) Winner() int {
	a, b := s.FinalScore(0), s.FinalScore(1)
	if a > b {
		return 0
	}
	if b > a {
		return 1
	}
	return -1
}
