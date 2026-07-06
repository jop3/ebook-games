package game

import (
	"math/rand"
	"time"
)

// Mode selects the opponent type.
type Mode int

const (
	ModeHotseat Mode = iota // two humans take turns
	ModeAI                  // player 0 is human, player 1 is the AI
)

// AI difficulty levels (see ai.go).
const (
	DepthEasy   = 1
	DepthMedium = 2
	DepthHard   = 3
)

// Phase is the high-level state of a turn. Splendor's "end of turn" can
// require extra resolution steps beyond the main action (a noble choice, a
// forced token discard) before the turn actually passes — each gets its own
// phase so the UI can show a dedicated (still tap-driven) screen for it.
type Phase int

const (
	PhasePlaying     Phase = iota // the active player may take a main action
	PhaseNobleChoice              // 2+ nobles qualify at once; active player must pick one
	PhaseDiscard                  // active player holds >10 tokens; must discard down
	PhaseDone                     // game over; Winner() is meaningful
)

// lastTurnIndex is whichever player acts last within a round of this 2-player
// game (fixed turn order 0, 1, 0, 1, ...) — used to detect "the round has
// finished" once the end-game prestige trigger has fired.
const lastTurnIndex = 1

// PlayerState is one player's full board state.
type PlayerState struct {
	Tokens   [NumColors]int
	Gold     int
	Cards    []Card
	Bonuses  [NumColors]int // permanent 1-gem discount per color, from owned Cards
	Reserved []Card
	Nobles   []Noble
	Prestige int
}

// Clone returns a deep copy of p.
func (p PlayerState) Clone() PlayerState {
	np := p
	np.Cards = append([]Card(nil), p.Cards...)
	np.Reserved = append([]Card(nil), p.Reserved...)
	np.Nobles = append([]Noble(nil), p.Nobles...)
	return np
}

// TokensTotal is how many tokens (gems + gold) p currently holds.
func (p *PlayerState) TokensTotal() int {
	n := p.Gold
	for c := Color(0); c < NumColors; c++ {
		n += p.Tokens[c]
	}
	return n
}

// GameState is a full playable Juvelerna game between two players (index 0
// and 1; in ModeAI, 0 is the human and 1 is the AI).
type GameState struct {
	Bank     [NumColors]int
	BankGold int

	Decks   [NumTiers][]Card             // remaining draw pile per tier (index 0 == tier 1)
	Tableau [NumTiers][TableauSlots]Card // face-up slots; empty slot = Card{} (Tier 0)
	Nobles  []Noble                      // still-unclaimed face-up nobles

	Players [2]PlayerState

	Turn  int // 0 or 1: whose turn it is to act
	Phase Phase
	Mode  Mode

	AILevel int

	PendingNobles []int // indices into Nobles; valid when Phase == PhaseNobleChoice
	DiscardNeeded int   // valid when Phase == PhaseDiscard

	EndTriggered bool // some player reached PrestigeToWin; finish the round
	WinnerIdx    int  // valid once Phase == PhaseDone: 0, 1, or -1 for a tie

	rng *rand.Rand
}

// NewGame starts a fresh game with a randomly shuffled deck/noble draw.
func NewGame(mode Mode, aiLevel int) *GameState {
	return NewGameSeeded(mode, aiLevel, time.Now().UnixNano())
}

// NewGameSeeded starts a fresh game with a deterministic shuffle, for tests
// and reproducible screenshots.
func NewGameSeeded(mode Mode, aiLevel int, seed int64) *GameState {
	gs := &GameState{
		Mode:      mode,
		Phase:     PhasePlaying,
		AILevel:   aiLevel,
		WinnerIdx: -1,
		rng:       rand.New(rand.NewSource(seed)),
	}
	for c := Color(0); c < NumColors; c++ {
		gs.Bank[c] = TokensPerColor2P
	}
	gs.BankGold = GoldTokens

	decks := buildAllCards()
	for i := range decks {
		shuffle(decks[i], gs.rng)
	}
	gs.Decks = decks
	for t := 0; t < NumTiers; t++ {
		for s := 0; s < TableauSlots; s++ {
			gs.Tableau[t][s] = gs.drawCard(t)
		}
	}

	allNobles := buildAllNobles()
	shuffleNobles(allNobles, gs.rng)
	gs.Nobles = append([]Noble(nil), allNobles[:NumNobles2P]...)

	gs.Turn = 0
	return gs
}

func shuffle(cards []Card, rng *rand.Rand) {
	for i := len(cards) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		cards[i], cards[j] = cards[j], cards[i]
	}
}

func shuffleNobles(nobles []Noble, rng *rand.Rand) {
	for i := len(nobles) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		nobles[i], nobles[j] = nobles[j], nobles[i]
	}
}

// AITurn reports whether it is currently the AI's move (in any sub-phase of
// its turn: the main action, a pending noble choice, or a forced discard).
func (gs *GameState) AITurn() bool {
	return gs.Mode == ModeAI && gs.Turn == 1 && gs.Phase != PhaseDone
}

// drawCard removes and returns the top card of tier t's deck (t is 0-based:
// 0 == tier 1), or the zero Card{} (an empty slot) if the deck is empty.
func (gs *GameState) drawCard(t int) Card {
	d := gs.Decks[t]
	if len(d) == 0 {
		return Card{}
	}
	last := len(d) - 1
	c := d[last]
	gs.Decks[t] = d[:last]
	return c
}

// Winner returns the winning player index (0 or 1), or -1 for a genuine
// tie. Only meaningful once Phase == PhaseDone (mirrors WinnerIdx, computed
// once in finishTurn via FinalWinner).
func (gs *GameState) Winner() int {
	return gs.WinnerIdx
}

// Clone returns a deep copy of gs, independent of the original — used by the
// AI's lookahead search to simulate candidate actions without touching the
// real game.
func (gs *GameState) Clone() *GameState {
	ng := *gs
	ng.Players[0] = gs.Players[0].Clone()
	ng.Players[1] = gs.Players[1].Clone()
	for i := range ng.Decks {
		ng.Decks[i] = append([]Card(nil), gs.Decks[i]...)
	}
	ng.Nobles = append([]Noble(nil), gs.Nobles...)
	ng.PendingNobles = append([]int(nil), gs.PendingNobles...)
	return &ng
}
