package game

import (
	"math/rand"
	"testing"
)

// playOptimalVsOptimal runs a full game where both sides play BestMove and
// returns the winner. Used to sanity-check the engine terminates.
func playOut(g *GameState) int {
	for !g.Over {
		m, ok := g.BestMove()
		if !ok {
			break
		}
		if err := g.ApplyMove(m); err != nil {
			panic(err)
		}
	}
	return g.Winner
}

func TestApplyMoveNormalWinner(t *testing.T) {
	g := NewGame([]int{1}, Normal, TwoPlayer)
	// player 0 takes the last stick -> player 0 wins (normal).
	if err := g.ApplyMove(Move{Pile: 0, Count: 1}); err != nil {
		t.Fatal(err)
	}
	if !g.Over || g.Winner != 0 {
		t.Fatalf("normal: expected winner 0, got over=%v winner=%d", g.Over, g.Winner)
	}
}

func TestApplyMoveMisereLoser(t *testing.T) {
	g := NewGame([]int{1}, Misere, TwoPlayer)
	// player 0 forced to take the last stick -> player 0 LOSES (misère).
	if err := g.ApplyMove(Move{Pile: 0, Count: 1}); err != nil {
		t.Fatal(err)
	}
	if !g.Over || g.Winner != 1 {
		t.Fatalf("misère: expected winner 1, got over=%v winner=%d", g.Over, g.Winner)
	}
}

func TestValidation(t *testing.T) {
	g := NewGame([]int{3, 0, 5}, Normal, TwoPlayer)
	cases := []struct {
		m  Move
		ok bool
	}{
		{Move{0, 1}, true},
		{Move{0, 3}, true},
		{Move{0, 4}, false},  // too many
		{Move{0, 0}, false},  // zero
		{Move{1, 1}, false},  // empty pile
		{Move{2, 5}, true},   //
		{Move{9, 1}, false},  // no such pile
		{Move{-1, 1}, false}, // negative
	}
	for _, c := range cases {
		err := g.Validate(c.m)
		if (err == nil) != c.ok {
			t.Errorf("Validate(%v): ok=%v err=%v", c.m, c.ok, err)
		}
	}
}

func TestTurnAlternates(t *testing.T) {
	g := NewGame([]int{5, 5}, Normal, TwoPlayer)
	if g.Turn != 0 {
		t.Fatal("start turn should be 0")
	}
	g.ApplyMove(Move{0, 1})
	if g.Turn != 1 {
		t.Fatal("turn should be 1 after first move")
	}
	g.ApplyMove(Move{1, 1})
	if g.Turn != 0 {
		t.Fatal("turn should be back to 0")
	}
}

// bruteForceWin computes, by exhaustive minimax, whether the player to move can
// force a win under the given variant. Independent of the nim-sum logic so it
// validates the AI's theory. Memoized by pile-multiset key.
type solver struct {
	variant Variant
	memo    map[string]bool
}

func key(piles []int) string {
	// sorted copy as key (order doesn't matter for the game value).
	c := append([]int(nil), piles...)
	// simple insertion sort (small n)
	for i := 1; i < len(c); i++ {
		for j := i; j > 0 && c[j-1] > c[j]; j-- {
			c[j-1], c[j] = c[j], c[j-1]
		}
	}
	b := make([]byte, 0, len(c))
	for _, v := range c {
		b = append(b, byte(v), ',')
	}
	return string(b)
}

// moverWins reports whether the player to move wins with perfect play.
func (s *solver) moverWins(piles []int) bool {
	total := 0
	for _, n := range piles {
		total += n
	}
	if total == 0 {
		// No sticks: the PREVIOUS player took the last stick.
		// Normal: previous player won => mover LOSES.
		// Misère: previous player took last => previous LOSES => mover WINS.
		if s.variant == Normal {
			return false
		}
		return true
	}
	k := key(piles)
	if v, ok := s.memo[k]; ok {
		return v
	}
	// mover wins if any move leads to a position where opponent loses.
	win := false
	for i, n := range piles {
		for take := 1; take <= n && !win; take++ {
			piles[i] -= take
			if !s.moverWins(piles) {
				win = true
			}
			piles[i] += take
		}
		if win {
			break
		}
	}
	s.memo[k] = win
	return win
}

// TestAIOptimalNormal: for MANY starting positions, whenever the position is a
// theoretical win for the mover, the AI's BestMove must lead to a position that
// is a LOSS for the opponent — i.e. the AI never squanders a winning position.
// Additionally, from a winning position, the AI must win a full game against a
// worst-case (brute-force) adversary.
func TestAIOptimalNormal(t *testing.T) {
	assertAIOptimal(t, Normal)
}

func TestAIOptimalMisere(t *testing.T) {
	assertAIOptimal(t, Misere)
}

func assertAIOptimal(t *testing.T, variant Variant) {
	s := &solver{variant: variant, memo: map[string]bool{}}
	tested, winning := 0, 0
	// enumerate all positions with up to 4 piles, each 0..6.
	var enum func(prefix []int, depth int)
	enum = func(prefix []int, depth int) {
		if depth == 0 {
			piles := append([]int(nil), prefix...)
			total := 0
			for _, n := range piles {
				total += n
			}
			if total == 0 {
				return
			}
			tested++
			g := NewGame(piles, variant, TwoPlayer)
			theoreticalWin := s.moverWins(piles)

			// The engine's own honesty flag must agree with the solver.
			if g.IsWinningForMover() != theoreticalWin {
				t.Fatalf("%s %v: IsWinningForMover=%v, solver=%v",
					variant, piles, g.IsWinningForMover(), theoreticalWin)
			}

			if theoreticalWin {
				winning++
				// AI's chosen move must leave opponent in a LOSING position.
				m, ok := g.BestMove()
				if !ok {
					t.Fatalf("%s %v: no move from winning position", variant, piles)
				}
				after := append([]int(nil), piles...)
				after[m.Pile] -= m.Count
				if s.moverWins(after) {
					t.Fatalf("%s %v: BestMove %v leaves opponent WINNING (not optimal)",
						variant, piles, m)
				}
				// And play a full game: AI (mover) vs brute-force optimal
				// adversary. AI must win.
				if w := playAIvsBrute(variant, piles, s); w != 0 {
					t.Fatalf("%s %v: AI started winning but lost full game (winner=%d)",
						variant, piles, w)
				}
			}
			return
		}
		for v := 0; v <= 6; v++ {
			enum(append(prefix, v), depth-1)
		}
	}
	for np := 1; np <= 4; np++ {
		enum(nil, np)
	}
	t.Logf("%s: tested %d positions, %d winning-for-mover", variant, tested, winning)
	if winning == 0 {
		t.Fatal("no winning positions exercised")
	}
}

// playAIvsBrute plays a full game: player 0 is our AI (BestMove), player 1 is a
// brute-force adversary that always plays a winning move if one exists (else
// any legal move). Returns the winner. Player 0 starts.
func playAIvsBrute(variant Variant, piles []int, s *solver) int {
	g := NewGame(piles, variant, TwoPlayer)
	for !g.Over {
		if g.Turn == 0 {
			m, _ := g.BestMove()
			g.ApplyMove(m)
		} else {
			m := bruteMove(g, s)
			g.ApplyMove(m)
		}
	}
	return g.Winner
}

// bruteMove picks a move leading to an opponent-losing position if possible.
func bruteMove(g *GameState, s *solver) Move {
	for i, n := range g.Piles {
		for take := 1; take <= n; take++ {
			after := append([]int(nil), g.Piles...)
			after[i] -= take
			if !s.moverWins(after) {
				return Move{Pile: i, Count: take}
			}
		}
	}
	// no winning move: play anything legal.
	for i, n := range g.Piles {
		if n > 0 {
			return Move{Pile: i, Count: 1}
		}
	}
	return Move{}
}

func TestGameTerminates(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 200; i++ {
		piles := RandomPiles(rng)
		v := Normal
		if i%2 == 0 {
			v = Misere
		}
		g := NewGame(piles, v, SoloAI)
		w := playOut(g)
		if w != 0 && w != 1 {
			t.Fatalf("bad winner %d for %v", w, piles)
		}
		if !g.Over {
			t.Fatalf("game did not terminate for %v", piles)
		}
	}
}

func TestRandomPilesValid(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		p := RandomPiles(rng)
		if len(p) < 3 || len(p) > 4 {
			t.Fatalf("pile count %d out of range", len(p))
		}
		total := 0
		for _, n := range p {
			if n < 1 || n > 7 {
				t.Fatalf("pile size %d out of range", n)
			}
			total += n
		}
		if total < 3 {
			t.Fatalf("total %d too small", total)
		}
	}
}
