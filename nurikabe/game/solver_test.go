package game

import (
	"math/rand"
	"testing"
)

func TestIslandsOKDetectsWrongSize(t *testing.T) {
	// 3x1 grid: one island cell should be sea, one white, seed says size 1;
	// but region size is 2 -> invalid.
	sea := [][]bool{{false, false, true}}
	seeds := map[[2]int]int{{0, 0}: 1}
	if islandsOK(3, 1, sea, seeds) {
		t.Fatal("expected mismatched island size to fail")
	}
}

func TestIslandsOKRequiresExactlyOneSeed(t *testing.T) {
	// A single white region containing two seeds must fail.
	sea := [][]bool{{true, false, false, true}}
	seeds := map[[2]int]int{{1, 0}: 1, {2, 0}: 1}
	if islandsOK(4, 1, sea, seeds) {
		t.Fatal("expected a region with two seeds to fail")
	}
}

func TestHasSea2x2(t *testing.T) {
	sea := [][]bool{
		{true, true, false},
		{true, true, false},
		{false, false, false},
	}
	if !hasSea2x2(3, 3, sea) {
		t.Fatal("expected 2x2 sea block to be detected")
	}
	sea2 := [][]bool{
		{true, true, false},
		{true, false, false},
		{false, false, false},
	}
	if hasSea2x2(3, 3, sea2) {
		t.Fatal("did not expect a 2x2 sea block")
	}
}

func TestSeaConnected(t *testing.T) {
	// Two disconnected sea blobs.
	sea := [][]bool{
		{true, false, true},
		{false, false, false},
	}
	if seaConnected(3, 2, sea) {
		t.Fatal("expected disconnected sea to fail")
	}
	sea2 := [][]bool{
		{true, true, true},
		{false, false, false},
	}
	if !seaConnected(3, 2, sea2) {
		t.Fatal("expected connected sea to pass")
	}
}

func TestValidateSolutionAcceptsSimpleValidBoard(t *testing.T) {
	// 3x3: one island of size 2 (top-left two cells), rest sea, no 2x2 sea
	// block since it's an L: sea covers (2,0),(0,1),(1,1),(2,1),(0,2),(1,2),(2,2)
	// which does contain a 2x2 at (1,1)-(2,2). Use a smaller deliberately-valid
	// layout instead: a single seed covering the whole 2x2 grid (no sea).
	sea := [][]bool{
		{false, false},
		{false, false},
	}
	seeds := map[[2]int]int{{0, 0}: 4}
	if !ValidateSolution(2, 2, sea, seeds) {
		t.Fatal("expected an all-island grid to validate")
	}
}

func TestGeneratorProducesValidSolutions(t *testing.T) {
	for _, p := range Presets {
		for seed := int64(0); seed < 15; seed++ {
			rng := rand.New(rand.NewSource(seed*131 + 17))
			puz := Generate(p, rng)
			if len(puz.Seeds) == 0 {
				t.Fatalf("%s seed %d: generated puzzle has no seeds", p.Name, seed)
			}
			if !ValidateSolution(puz.W, puz.H, puz.Solution, puz.Seeds) {
				t.Fatalf("%s seed %d: generated solution fails validation", p.Name, seed)
			}
		}
	}
}

func TestToggleCyclesAndSkipsSeeds(t *testing.T) {
	gs := NewGameSeeded(Presets[0], 5)
	// Find a seed cell; toggling it must fail.
	var seedX, seedY int
	for pos := range gs.Puz.Seeds {
		seedX, seedY = pos[0], pos[1]
		break
	}
	if gs.Toggle(seedX, seedY) {
		t.Fatal("toggling a seed cell should be rejected")
	}
	// Find a non-seed cell and cycle it.
	nx, ny := -1, -1
outer:
	for y := 0; y < gs.Puz.H; y++ {
		for x := 0; x < gs.Puz.W; x++ {
			if _, isSeed := gs.Puz.Seeds[[2]int{x, y}]; !isSeed {
				nx, ny = x, y
				break outer
			}
		}
	}
	if nx == -1 {
		t.Fatal("expected at least one non-seed cell")
	}
	if !gs.Toggle(nx, ny) || gs.Cells[ny][nx] != StateSea {
		t.Fatal("first toggle should set Sea")
	}
	if !gs.Toggle(nx, ny) || gs.Cells[ny][nx] != StateIsland {
		t.Fatal("second toggle should set Island")
	}
	if !gs.Toggle(nx, ny) || gs.Cells[ny][nx] != StateUnknown {
		t.Fatal("third toggle should reset to Unknown")
	}
}

func TestApplyingSolutionSetsDone(t *testing.T) {
	gs := NewGameSeeded(Presets[0], 9)
	for y := 0; y < gs.Puz.H; y++ {
		for x := 0; x < gs.Puz.W; x++ {
			if _, isSeed := gs.Puz.Seeds[[2]int{x, y}]; isSeed {
				continue
			}
			if gs.Puz.Solution[y][x] {
				gs.Cells[y][x] = StateSea
			} else {
				gs.Cells[y][x] = StateIsland
			}
		}
	}
	gs.checkDone()
	if !gs.Done {
		t.Fatal("applying the true solution should set Done")
	}
}
