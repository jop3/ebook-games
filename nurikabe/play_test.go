//go:build playtest

package main

// Headless PLAYTHROUGH tests for Nurikabe. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): paint each cell island (white) or sea (black) so every numbered seed
// grows into an island of exactly that many connected cells, all sea is
// connected, and no 2x2 block is entirely sea. Tapping a non-seed cell cycles
// unknown -> sea -> island -> unknown; seeds are fixed. The solve tests also
// assert the GENERATOR invariant — the stored solution is a valid Nurikabe
// layout. Runs under the pure-Go inkview emulator (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"nurikabe/game"
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

func tapCell(h *ink.Harness, a *app, x, y int) {
	h.TapRect(a.layout.CellRect(x, y))
}

func isSeed(a *app, x, y int) bool {
	_, ok := a.gs.Puz.Seeds[[2]int{x, y}]
	return ok
}

func tapButton(t *testing.T, h *ink.Harness, a *app, label string) {
	t.Helper()
	for _, b := range a.buttons {
		if b.Label == label {
			h.TapRect(b.Rect)
			return
		}
	}
	t.Fatalf("button %q not present", label)
}

// paintSolution paints every non-seed cell to match the hidden solution (one
// tap for sea, two for island). Runs from a fresh board where all non-seed
// cells are Unknown, so the tap counts are exact.
func paintSolution(h *ink.Harness, a *app) {
	for y := 0; y < a.gs.Puz.H; y++ {
		for x := 0; x < a.gs.Puz.W; x++ {
			if isSeed(a, x, y) {
				continue
			}
			if a.gs.Puz.Solution[y][x] { // sea
				tapCell(h, a, x, y)
			} else { // island
				tapCell(h, a, x, y)
				tapCell(h, a, x, y)
			}
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

func TestPlayNurikabeSolveAllPresets(t *testing.T) {
	for i := range game.Presets {
		i := i
		t.Run(game.Presets[i].Name, func(t *testing.T) {
			h, a := bootToMenu(t)
			start(t, h, a, i)

			// GENERATOR RULE: the stored solution is a valid Nurikabe layout.
			if !game.ValidateSolution(a.gs.Puz.W, a.gs.Puz.H, a.gs.Puz.Solution, a.gs.Puz.Seeds) {
				t.Fatal("generated solution is not a valid Nurikabe layout")
			}

			paintSolution(h, a)
			if !a.gs.Done {
				t.Fatal("painting the solution did not win")
			}
			if game.IsSea2x2Violation(a.gs) {
				t.Fatal("solved state contains a 2x2 sea block")
			}
			if _, ok := h.FindTextContains("Löst"); !ok {
				t.Fatalf("win banner missing; visible: %v", texts(h))
			}
		})
	}
}

// --- RULE: tap cycles unknown -> sea -> island -> unknown; seeds are fixed ---

func TestPlayNurikabeToggleCycle(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	cx, cy := firstNonSeed(a)
	tapCell(h, a, cx, cy)
	if a.gs.Cells[cy][cx] != game.StateSea {
		t.Fatalf("first tap should paint sea, got %v", a.gs.Cells[cy][cx])
	}
	tapCell(h, a, cx, cy)
	if a.gs.Cells[cy][cx] != game.StateIsland {
		t.Fatalf("second tap should mark island, got %v", a.gs.Cells[cy][cx])
	}
	tapCell(h, a, cx, cy)
	if a.gs.Cells[cy][cx] != game.StateUnknown {
		t.Fatalf("third tap should clear, got %v", a.gs.Cells[cy][cx])
	}

	// A seed cell cannot be repainted.
	for pos := range a.gs.Puz.Seeds {
		before := a.gs.Cells[pos[1]][pos[0]]
		tapCell(h, a, pos[0], pos[1])
		if a.gs.Cells[pos[1]][pos[0]] != before {
			t.Fatal("a seed cell was repainted")
		}
		break
	}
}

// --- RULE: a 2x2 all-sea block is a detectable violation --------------------

func TestPlayNurikabe2x2Rule(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	// Find a 2x2 of non-seed cells.
	fx, fy := -1, -1
	for y := 0; y+1 < a.gs.Puz.H && fy < 0; y++ {
		for x := 0; x+1 < a.gs.Puz.W; x++ {
			if !isSeed(a, x, y) && !isSeed(a, x+1, y) && !isSeed(a, x, y+1) && !isSeed(a, x+1, y+1) {
				fx, fy = x, y
				break
			}
		}
	}
	if fx < 0 {
		t.Skip("no 2x2 block free of seeds")
	}
	for _, c := range [][2]int{{fx, fy}, {fx + 1, fy}, {fx, fy + 1}, {fx + 1, fy + 1}} {
		tapCell(h, a, c[0], c[1]) // paint sea
	}
	if !game.IsSea2x2Violation(a.gs) {
		t.Fatal("a 2x2 sea block was not detected as a violation")
	}
	if a.gs.Done {
		t.Fatal("a board with a 2x2 sea block counted as solved")
	}
}

// --- RULE: incomplete / wrong paint is not a win ----------------------------

func TestPlayNurikabePartialAndWrong(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	// Hold out a non-seed ISLAND cell (solution=false). Painting it sea is the
	// wrong colour and — unlike holding out a sea cell — never passes through the
	// correct state, which would complete-and-lock the board.
	hx, hy := -1, -1
	for y := 0; y < a.gs.Puz.H && hy < 0; y++ {
		for x := 0; x < a.gs.Puz.W; x++ {
			if !isSeed(a, x, y) && !a.gs.Puz.Solution[y][x] {
				hx, hy = x, y
				break
			}
		}
	}
	if hx < 0 {
		t.Skip("no non-seed island cell to hold out")
	}

	// Paint every other non-seed cell correctly.
	for y := 0; y < a.gs.Puz.H; y++ {
		for x := 0; x < a.gs.Puz.W; x++ {
			if isSeed(a, x, y) || (x == hx && y == hy) {
				continue
			}
			if a.gs.Puz.Solution[y][x] { // sea
				tapCell(h, a, x, y)
			} else { // island
				tapCell(h, a, x, y)
				tapCell(h, a, x, y)
			}
		}
	}
	if a.gs.Done {
		t.Fatal("won with the held-out cell still unknown")
	}
	// Paint the held-out island cell as SEA (wrong) -> still not done.
	tapCell(h, a, hx, hy)
	if a.gs.Cells[hy][hx] != game.StateSea {
		t.Fatalf("held-out cell not sea (got %v)", a.gs.Cells[hy][hx])
	}
	if a.gs.Done {
		t.Fatal("won with the held-out cell painted the wrong colour")
	}
	// Correct it to island -> now the whole board matches and wins.
	tapCell(h, a, hx, hy)
	if !a.gs.Done {
		t.Fatal("correcting the held-out cell did not win")
	}
}

func firstNonSeed(a *app) (int, int) {
	for y := 0; y < a.gs.Puz.H; y++ {
		for x := 0; x < a.gs.Puz.W; x++ {
			if !isSeed(a, x, y) {
				return x, y
			}
		}
	}
	return 0, 0
}

// --- Clear/quit/rules -------------------------------------------------------

func TestPlayNurikabeClearQuitRules(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	cx, cy := firstNonSeed(a)
	tapCell(h, a, cx, cy)
	tapButton(t, h, a, "Rensa")
	if a.gs.Cells[cy][cx] != game.StateUnknown {
		t.Fatal("Rensa did not clear the paint")
	}

	tapButton(t, h, a, "Meny")
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
	if _, ok := h.FindTextContains("hav"); !ok {
		t.Fatalf("rules text missing; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave rules, screen=%v", a.screen)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayNurikabeScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	// Boot manually so we can grab the splash before it is dismissed.
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if e := h.Screenshot(dir + "/nurikabe_splash.png"); e != nil {
		t.Fatal(e)
	}
	h.TapXY(500, 700) // dismiss splash -> menu
	if a.screen != screenMenu {
		t.Fatalf("splash tap did not open menu, screen=%v", a.screen)
	}
	_ = h.Screenshot(dir + "/nurikabe_menu.png")

	h.TapRect(a.menu.RulesButton())
	_ = h.Screenshot(dir + "/nurikabe_rules.png")
	h.Back()

	start(t, h, a, 0)

	// Partial paint: the sea cells in the top half of the board (one tap each)
	// for an in-progress shot.
	for y := 0; y <= a.gs.Puz.H/2; y++ {
		for x := 0; x < a.gs.Puz.W; x++ {
			if !isSeed(a, x, y) && a.gs.Puz.Solution[y][x] {
				tapCell(h, a, x, y)
			}
		}
	}
	_ = h.Screenshot(dir + "/nurikabe_board.png")

	// Reset the partial work and paint the full solution for the win shot.
	tapButton(t, h, a, "Rensa")
	paintSolution(h, a)
	if err := h.Screenshot(dir + "/nurikabe_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
