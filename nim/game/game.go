// Package game holds the PURE Nim rules engine: pile state, move validation,
// win detection, and a perfect (Sprague-Grundy / nim-sum) AI for both the
// normal and misère variants. It must NOT import the ink SDK so it can be
// unit-tested without cgo.
package game

import (
	"errors"
	"math/rand"
)

// Variant selects the win condition.
type Variant int

const (
	// Normal: the player who takes the LAST stick WINS.
	Normal Variant = iota
	// Misere: the player who takes the LAST stick LOSES.
	Misere
)

func (v Variant) String() string {
	if v == Misere {
		return "Misère"
	}
	return "Normal"
}

// Mode selects who plays.
type Mode int

const (
	// TwoPlayer is hot-seat: two humans alternate.
	TwoPlayer Mode = iota
	// SoloAI: player 0 is human, player 1 is the perfect AI.
	SoloAI
)

// Move takes Count sticks from pile Pile.
type Move struct {
	Pile  int
	Count int
}

// Errors returned by ApplyMove / validate.
var (
	ErrGameOver  = errors.New("game is over")
	ErrBadPile   = errors.New("no such pile")
	ErrBadCount  = errors.New("count must be >= 1 and <= pile size")
	ErrEmptyPile = errors.New("pile is empty")
)

// GameState is the full mutable state of a Nim game.
type GameState struct {
	Piles   []int   // remaining sticks per pile
	Turn    int     // 0 or 1 — whose turn it is
	Variant Variant //
	Mode    Mode    //
	Over    bool    // true once no sticks remain
	Winner  int     // valid only when Over; 0 or 1
}

// NewGame starts a game with the given starting piles (copied), variant, mode.
func NewGame(piles []int, variant Variant, mode Mode) *GameState {
	p := make([]int, len(piles))
	copy(p, piles)
	return &GameState{
		Piles:   p,
		Turn:    0,
		Variant: variant,
		Mode:    mode,
	}
}

// Total returns the number of sticks left across all piles.
func (g *GameState) Total() int {
	t := 0
	for _, n := range g.Piles {
		t += n
	}
	return t
}

// NimSum is the XOR of all pile sizes (the Sprague-Grundy value for normal Nim).
func (g *GameState) NimSum() int {
	s := 0
	for _, n := range g.Piles {
		s ^= n
	}
	return s
}

// nonEmptyPiles counts piles with >= 1 stick.
func (g *GameState) nonEmptyPiles() int {
	c := 0
	for _, n := range g.Piles {
		if n > 0 {
			c++
		}
	}
	return c
}

// pilesAboveOne counts piles holding 2 or more sticks.
func (g *GameState) pilesAboveOne() int {
	c := 0
	for _, n := range g.Piles {
		if n > 1 {
			c++
		}
	}
	return c
}

// Validate checks a move against the current state without applying it.
func (g *GameState) Validate(m Move) error {
	if g.Over {
		return ErrGameOver
	}
	if m.Pile < 0 || m.Pile >= len(g.Piles) {
		return ErrBadPile
	}
	if g.Piles[m.Pile] == 0 {
		return ErrEmptyPile
	}
	if m.Count < 1 || m.Count > g.Piles[m.Pile] {
		return ErrBadCount
	}
	return nil
}

// ApplyMove validates and applies a move, advancing the turn and detecting the
// end of the game. The player who empties the board is recorded; Winner is set
// per the variant rule.
func (g *GameState) ApplyMove(m Move) error {
	if err := g.Validate(m); err != nil {
		return err
	}
	mover := g.Turn
	g.Piles[m.Pile] -= m.Count
	if g.Total() == 0 {
		g.Over = true
		// mover took the last stick.
		if g.Variant == Normal {
			g.Winner = mover // taking last WINS
		} else {
			g.Winner = 1 - mover // taking last LOSES
		}
		return nil
	}
	g.Turn = 1 - g.Turn
	return nil
}

// --- Perfect AI (Sprague-Grundy) ---

// IsWinningForMover reports whether the player about to move is in a theoretical
// winning position (i.e. a perfect move exists). Used to honestly show when the
// AI holds the win.
func (g *GameState) IsWinningForMover() bool {
	if g.Total() == 0 {
		return false
	}
	_, ok := g.perfectMove()
	return ok
}

// BestMove returns an optimal move for the player to move. The second return is
// false only when the board is empty (no move possible). When the position is
// theoretically lost, it returns a "best effort" move (documented below) that
// still forces the opponent to play perfectly to win.
func (g *GameState) BestMove() (Move, bool) {
	if g.Total() == 0 {
		return Move{}, false
	}
	if m, ok := g.perfectMove(); ok {
		return m, true
	}
	return g.fallbackMove(), true
}

// perfectMove returns a move that leaves the opponent in a losing position, if
// one exists (the position is a win for the mover). Handles both variants.
func (g *GameState) perfectMove() (Move, bool) {
	if g.Variant == Misere {
		return g.perfectMoveMisere()
	}
	return g.perfectMoveNormal()
}

// perfectMoveNormal: leave nim-sum == 0 for the opponent.
func (g *GameState) perfectMoveNormal() (Move, bool) {
	sum := g.NimSum()
	if sum == 0 {
		return Move{}, false // already a P-position; no winning move
	}
	for i, n := range g.Piles {
		target := n ^ sum
		if target < n {
			// take n-target sticks from pile i, leaving nim-sum 0.
			return Move{Pile: i, Count: n - target}, true
		}
	}
	return Move{}, false
}

// perfectMoveMisere implements the standard misère Nim strategy.
//
// Play as in normal Nim (leave nim-sum 0) UNTIL your move would leave every
// remaining pile with at most one stick. At that point, instead leave an ODD
// number of one-stick piles (so the opponent is forced to take the last stick).
//
// Concretely: if there are >= 2 piles with more than one stick, the position is
// a win iff nim-sum != 0 and the normal move applies. If there is exactly one
// pile > 1, you can always win by reducing that big pile so that the number of
// one-stick piles left is odd. If all piles are <= 1, the position is a win iff
// the number of one-stick piles is EVEN (then opponent faces an odd count and
// must take the last).
func (g *GameState) perfectMoveMisere() (Move, bool) {
	big := g.pilesAboveOne()
	ones := g.nonEmptyPiles() - big

	if big == 0 {
		// Endgame: only 1-stick piles remain.
		// Mover wins iff the count of ones is EVEN (leave opponent an odd count).
		if ones%2 == 0 {
			// take one stick from any one-pile, leaving an odd number of ones.
			for i, n := range g.Piles {
				if n == 1 {
					return Move{Pile: i, Count: 1}, true
				}
			}
		}
		return Move{}, false
	}

	if big == 1 {
		// Exactly one pile > 1: always a winning position.
		// Reduce it so that the number of remaining one-piles is ODD.
		bigIdx := -1
		for i, n := range g.Piles {
			if n > 1 {
				bigIdx = i
				break
			}
		}
		// After our move the big pile becomes b sticks (0 or 1 depending on
		// desired parity of one-piles). We want ones' + (b==1?1:0) to be odd,
		// where ones' == ones (other one-piles untouched).
		// Choose to leave the big pile at 0 or 1 to make total one-count odd.
		leave := 0
		if (ones+0)%2 == 1 {
			// leaving 0 gives 'ones' one-piles (odd) -> good.
			leave = 0
		} else {
			// leaving 1 gives ones+1 (odd) -> good.
			leave = 1
		}
		return Move{Pile: bigIdx, Count: g.Piles[bigIdx] - leave}, true
	}

	// big >= 2: play normal Nim (leave nim-sum 0).
	return g.perfectMoveNormal()
}

// fallbackMove is used when the mover is in a losing position: make a legal,
// minimal, but non-forfeiting move. Take one stick from the largest pile so the
// game continues and the opponent must keep playing perfectly.
func (g *GameState) fallbackMove() Move {
	bestIdx, bestN := -1, 0
	for i, n := range g.Piles {
		if n > bestN {
			bestN = n
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return Move{}
	}
	return Move{Pile: bestIdx, Count: 1}
}

// --- Presets ---

// Preset is a named starting configuration.
type Preset struct {
	Name  string
	Piles []int
}

// Presets are the built-in starting layouts offered in the menu.
var Presets = []Preset{
	{Name: "Klassisk 3-4-5", Piles: []int{3, 4, 5}},
	{Name: "1-3-5-7", Piles: []int{1, 3, 5, 7}},
	{Name: "Slumpad", Piles: nil}, // filled at game start
}

// RandomPiles builds a random layout: 3..4 piles of 1..7 sticks, never all-zero
// and not a trivial single pile. Deterministic given rng.
func RandomPiles(rng *rand.Rand) []int {
	n := 3 + rng.Intn(2) // 3 or 4 piles
	p := make([]int, n)
	for {
		total := 0
		for i := range p {
			p[i] = 1 + rng.Intn(7)
			total += p[i]
		}
		if total >= 3 {
			return p
		}
	}
}
