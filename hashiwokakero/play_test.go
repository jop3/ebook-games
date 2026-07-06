//go:build playtest

package main

// Headless PLAYTHROUGH tests for Hashiwokakero (Bridges). They drive the real
// touch path and check the gameplay against the rules as written (see
// rulesParagraphs in ui.go): connect the islands with bridges so each island's
// bridge count equals its number, bridges run straight with no island between,
// bridges never cross, and every island ends up in one connected network.
// Tapping island A then a neighbour cycles that bridge 0->1->2->0. The solve
// tests also assert the GENERATOR invariant — every puzzle is uniquely solvable.
// Runs under the pure-Go inkview emulator (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"hashiwokakero/game"
)

// --- helpers ----------------------------------------------------------------

func bootToMenu(t *testing.T) (*ink.Harness, *app) {
	t.Helper()
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if a.screen != screenMenu {
		t.Fatalf("splash tap did not open menu, screen=%v", a.screen)
	}
	return h, a
}

func start(t *testing.T, h *ink.Harness, a *app, i int) {
	t.Helper()
	if err := h.TapText(game.Presets[i].Name); err != nil {
		t.Fatalf("could not start %q: %v", game.Presets[i].Name, err)
	}
	if a.screen != screenGame || a.gs == nil {
		t.Fatalf("did not enter game for preset %d, screen=%v", i, a.screen)
	}
}

func key(i, j int) [2]int {
	if i > j {
		i, j = j, i
	}
	return [2]int{i, j}
}

func tapIsland(h *ink.Harness, a *app, idx int) {
	isl := a.gs.Puz.Islands[idx]
	h.Tap(a.layout.IslandCenter(isl.X, isl.Y))
}

// buildBridge adds one bridge across the pair (select i, tap neighbour j).
func buildBridge(h *ink.Harness, a *app, i, j int) {
	tapIsland(h, a, i)
	tapIsland(h, a, j)
}

// solveViaUI builds the deduced solution's bridges through the UI.
func solveViaUI(h *ink.Harness, a *app) {
	sol, ok := game.SolveBridges(a.gs.Puz)
	if !ok {
		return
	}
	for k, v := range sol {
		for n := 0; n < v; n++ {
			buildBridge(h, a, k[0], k[1])
		}
	}
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- SOLVE every preset + verify the generator invariant --------------------

func TestPlayHashiSolveAllPresets(t *testing.T) {
	for i := range game.Presets {
		i := i
		t.Run(game.Presets[i].Name, func(t *testing.T) {
			h, a := bootToMenu(t)
			start(t, h, a, i)

			// GENERATOR RULE: the puzzle must be uniquely solvable.
			if game.Solve(a.gs.Puz) != game.SolveUnique {
				t.Fatalf("generated puzzle is not uniquely solvable")
			}

			solveViaUI(h, a)
			if !a.gs.Done {
				t.Fatal("building the solution did not win")
			}
			// RULES: every island has exactly its needed bridges, all connected.
			for idx, isl := range a.gs.Puz.Islands {
				if a.gs.Degree(idx) != isl.Need {
					t.Fatalf("island %d has %d bridges, needs %d", idx, a.gs.Degree(idx), isl.Need)
				}
			}
			if !a.gs.Connected() {
				t.Fatal("won but islands are not all connected")
			}
			if _, ok := h.FindTextContains("Löst"); !ok {
				t.Fatalf("win banner missing; visible: %v", texts(h))
			}
		})
	}
}

// --- RULE: tapping a neighbour cycles the bridge 0 -> 1 -> 2 -> 0 ------------

func TestPlayHashiBridgeCycle(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	nbrs := a.gs.Puz.NeighbourList(0)
	if len(nbrs) == 0 {
		t.Skip("island 0 has no neighbour")
	}
	j := nbrs[0]
	k := key(0, j)

	buildBridge(h, a, 0, j)
	if a.gs.Bridges[k] != 1 {
		t.Fatalf("first build should give 1 bridge, got %d", a.gs.Bridges[k])
	}
	buildBridge(h, a, 0, j)
	if a.gs.Bridges[k] != 2 {
		t.Fatalf("second build should give 2 bridges, got %d", a.gs.Bridges[k])
	}
	buildBridge(h, a, 0, j)
	if a.gs.Bridges[k] != 0 {
		t.Fatalf("third build should clear the bridge, got %d", a.gs.Bridges[k])
	}
}

// --- RULE: an incomplete network is not a win -------------------------------

func TestPlayHashiPartialIsNotWon(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	sol, ok := game.SolveBridges(a.gs.Puz)
	if !ok {
		t.Fatal("no solution")
	}
	// Flatten to a list of single-bridge builds.
	var builds [][2]int
	for k, v := range sol {
		for n := 0; n < v; n++ {
			builds = append(builds, k)
		}
	}
	if len(builds) < 2 {
		t.Skip("trivial puzzle")
	}
	for _, b := range builds[:len(builds)-1] {
		buildBridge(h, a, b[0], b[1])
	}
	if a.gs.Done {
		t.Fatal("an incomplete bridge network counted as solved")
	}
	last := builds[len(builds)-1]
	buildBridge(h, a, last[0], last[1])
	if !a.gs.Done {
		t.Fatal("completing the last bridge did not win")
	}
}

// --- Clear/quit/rules -------------------------------------------------------

func TestPlayHashiClearQuitRules(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	j := a.gs.Puz.NeighbourList(0)[0]
	buildBridge(h, a, 0, j)
	if a.gs.Degree(0) == 0 {
		t.Fatal("bridge not built")
	}
	for _, b := range a.buttons {
		if b.Label == "Rensa" {
			h.TapRect(b.Rect)
		}
	}
	if len(a.gs.Bridges) != 0 {
		t.Fatalf("Rensa left %d bridges", len(a.gs.Bridges))
	}

	for _, b := range a.buttons {
		if b.Label == "Meny" {
			h.TapRect(b.Rect)
		}
	}
	if a.screen != screenMenu {
		t.Fatalf("Meny did not return to menu, screen=%v", a.screen)
	}

	start(t, h, a, 0)
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not return to menu, screen=%v", a.screen)
	}

	h.TapRect(a.menu.RulesButton())
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("broar"); !ok {
		t.Fatalf("rules text missing; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave rules, screen=%v", a.screen)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayHashiScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/hashiwokakero_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/hashiwokakero_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapRect(a.menu.RulesButton())
	if err := h.Screenshot(dir + "/hashiwokakero_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	// A fresh, unsolved puzzle: numbered islands, no bridges yet.
	start(t, h, a, 0)
	if err := h.Screenshot(dir + "/hashiwokakero_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// Build roughly half the solution's bridges for an in-progress shot.
	if sol, ok := game.SolveBridges(a.gs.Puz); ok {
		var builds [][2]int
		for k, v := range sol {
			for n := 0; n < v; n++ {
				builds = append(builds, k)
			}
		}
		for _, b := range builds[:len(builds)/2] {
			buildBridge(h, a, b[0], b[1])
		}
		if err := h.Screenshot(dir + "/hashiwokakero_progress.png"); err != nil {
			t.Fatalf("screenshot: %v", err)
		}
		// Clear the partial network, then build the full solution cleanly.
		a.gs.Reset()
		h.Draw()
	}

	solveViaUI(h, a)
	if err := h.Screenshot(dir + "/hashiwokakero_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
