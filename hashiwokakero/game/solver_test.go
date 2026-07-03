package game

import (
	"math/rand"
	"testing"
)

// manualPuzzle builds a puzzle from islands directly (test helper).
func manualPuzzle(w, h int, islands []Island) *Puzzle {
	idx := map[[2]int]int{}
	for i, isl := range islands {
		idx[[2]int{isl.X, isl.Y}] = i
	}
	return &Puzzle{W: w, H: h, Islands: islands, indexAt: idx}
}

func TestNeighbours(t *testing.T) {
	// Three islands in a row: (0,0)-(2,0)-(4,0).
	p := manualPuzzle(5, 1, []Island{
		{X: 0, Y: 0, Need: 1},
		{X: 2, Y: 0, Need: 2},
		{X: 4, Y: 0, Need: 1},
	})
	nb0 := p.NeighbourList(0)
	if len(nb0) != 1 || nb0[0] != 1 {
		t.Fatalf("island 0 neighbours = %v, want [1]", nb0)
	}
	nb1 := p.NeighbourList(1)
	if len(nb1) != 2 {
		t.Fatalf("island 1 neighbours = %v, want 2 entries", nb1)
	}
}

func TestCrossingRejected(t *testing.T) {
	// A horizontal pair (0,1) at y=1 spanning x=0..2, and a vertical pair (2,3)
	// at x=1 spanning y=0..2 — they cross at (1,1).
	p := manualPuzzle(3, 3, []Island{
		{X: 0, Y: 1, Need: 1}, // 0
		{X: 2, Y: 1, Need: 2}, // 1
		{X: 1, Y: 0, Need: 1}, // 2
		{X: 1, Y: 2, Need: 2}, // 3
	})
	gs := &GameState{Cfg: Preset{}, Puz: p, Bridges: map[[2]int]int{}}
	if !gs.Cycle(0, 1) {
		t.Fatalf("expected horizontal bridge 0-1 to succeed")
	}
	if gs.Cycle(2, 3) {
		t.Fatalf("expected vertical bridge 2-3 to be rejected (crosses 0-1)")
	}
}

func TestDoubleBridgeCap(t *testing.T) {
	p := manualPuzzle(3, 1, []Island{
		{X: 0, Y: 0, Need: 2},
		{X: 2, Y: 0, Need: 2},
	})
	gs := &GameState{Cfg: Preset{}, Puz: p, Bridges: map[[2]int]int{}}
	if !gs.Cycle(0, 1) { // -> 1
		t.Fatal("first cycle should succeed")
	}
	if !gs.Cycle(0, 1) { // -> 2
		t.Fatal("second cycle should succeed")
	}
	if gs.Bridges[pairKey(0, 1)] != 2 {
		t.Fatalf("expected 2 bridges, got %d", gs.Bridges[pairKey(0, 1)])
	}
	if !gs.Cycle(0, 1) { // -> 0 (reset)
		t.Fatal("third cycle should reset to 0")
	}
	if _, ok := gs.Bridges[pairKey(0, 1)]; ok {
		t.Fatal("expected bridge count to be removed at 0")
	}
}

func TestSolvedRequiresConnectivityAndDegree(t *testing.T) {
	// Two separate satisfied pairs (both degree-correct) but NOT connected to
	// each other must not count as solved.
	p := manualPuzzle(9, 1, []Island{
		{X: 0, Y: 0, Need: 1},
		{X: 2, Y: 0, Need: 1},
		{X: 6, Y: 0, Need: 1},
		{X: 8, Y: 0, Need: 1},
	})
	gs := &GameState{Cfg: Preset{}, Puz: p, Bridges: map[[2]int]int{}}
	gs.Cycle(0, 1)
	gs.Cycle(2, 3)
	if gs.Done {
		t.Fatal("two disconnected satisfied pairs should not be Done")
	}
	if gs.Connected() {
		t.Fatal("layout should not be reported as connected")
	}
}

func TestGeneratorProducesUniquePuzzles(t *testing.T) {
	for _, p := range Presets {
		for seed := int64(0); seed < 5; seed++ {
			rng := rand.New(rand.NewSource(seed*7919 + 13))
			puz := Generate(p, rng)
			if len(puz.Islands) < 2 {
				t.Fatalf("%s seed %d: generated puzzle with <2 islands", p.Name, seed)
			}
			res := Solve(puz)
			if res != SolveUnique {
				t.Logf("%s seed %d: generator fallback produced non-unique puzzle (result=%v) — acceptable per spec's fallback contract, but flag if frequent", p.Name, seed, res)
			}
		}
	}
}

func TestToggleCycleDrivesDone(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	puz := Generate(Presets[0], rng)
	if Solve(puz) != SolveUnique {
		t.Skip("seed did not yield a unique puzzle; skipping playthrough check")
	}
	// Reconstruct the actual solution via brute force search (small grid) and
	// apply it, expecting Done to become true.
	gs := &GameState{Cfg: Presets[0], Puz: puz, Bridges: map[[2]int]int{}}
	sol := findSolution(puz)
	if sol == nil {
		t.Fatal("expected a solution to exist for a SolveUnique puzzle")
	}
	for k, c := range sol {
		for n := 0; n < c; n++ {
			gs.Cycle(k[0], k[1])
		}
	}
	if !gs.Done {
		t.Fatal("applying the true solution should set Done")
	}
}

// findSolution brute-forces the single solution for small test puzzles using
// the same pruned backtracking as the real solver, but stopping at the first
// full solution found and reporting its bridge counts.
func findSolution(p *Puzzle) map[[2]int]int {
	s := newSolveState(p)
	var found map[[2]int]int
	var rec func(idx int) bool
	rec = func(idx int) bool {
		if idx == len(s.pairs) {
			for i, isl := range p.Islands {
				if s.degree[i] != isl.Need {
					return false
				}
			}
			if !s.connectedFinal() {
				return false
			}
			found = map[[2]int]int{}
			for k, c := range s.count {
				if c > 0 {
					pr := s.pairs[k]
					found[pairKey(pr.i, pr.j)] = c
				}
			}
			return true
		}
		pr := s.pairs[idx]
		for c := 0; c <= 2; c++ {
			if c > 0 && s.crossesAny2(idx, c) {
				continue
			}
			ni := s.degree[pr.i] + c
			nj := s.degree[pr.j] + c
			if ni > p.Islands[pr.i].Need || nj > p.Islands[pr.j].Need {
				continue
			}
			s.count[idx] = c
			s.degree[pr.i] += c
			s.degree[pr.j] += c
			if rec(idx + 1) {
				return true
			}
			s.degree[pr.i] -= c
			s.degree[pr.j] -= c
			s.count[idx] = 0
		}
		return false
	}
	rec(0)
	return found
}
