//go:build playtest

package main

// Headless PLAYTHROUGH tests for Akari (Light Up). They drive the real touch
// path and check the gameplay against the rules as written (see rulesParagraphs
// in ui.go): tapping a white cell cycles empty -> bulb -> dot -> empty; a bulb
// lights its row/column until a wall; two bulbs may never see each other;
// numbered walls constrain adjacent bulbs; you win when every white cell is lit
// and all rules hold. The solve tests also assert the GENERATOR invariant —
// every puzzle is uniquely solvable by logic — and then solve it via the UI
// using the game's own deduced solution. Runs under the pure-Go inkview emulator.

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"akari/game"
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
	h.TapRect(a.layout.CellToScreen(x, y))
}

func tapButton(t *testing.T, h *ink.Harness, a *app, label string) {
	t.Helper()
	for _, b := range a.buttons {
		if b.Label == label {
			h.TapRect(b.Rect)
			return
		}
	}
	t.Fatalf("button %q not present (have %v)", label, a.buttonLabels())
}

func isWhite(a *app, x, y int) bool {
	return a.gs.Board.Cells[y][x].Kind == game.White
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- SOLVE every preset + verify the generator invariant --------------------

func TestPlayAkariSolveAllPresets(t *testing.T) {
	for i := range game.Presets {
		i := i
		t.Run(game.Presets[i].Name, func(t *testing.T) {
			h, a := bootToMenu(t)
			start(t, h, a, i)

			// GENERATOR RULE: the puzzle must be uniquely solvable by logic.
			if game.Solve(a.gs.Board) != game.SolveUnique {
				t.Fatalf("generated puzzle is not uniquely solvable")
			}
			bulbs, ok := game.SolveBulbs(a.gs.Board)
			if !ok {
				t.Fatal("SolveBulbs failed on a supposedly unique puzzle")
			}

			// Place exactly the solution bulbs through the UI.
			for y := 0; y < a.gs.Board.H; y++ {
				for x := 0; x < a.gs.Board.W; x++ {
					if bulbs[y][x] {
						tapCell(h, a, x, y) // empty -> bulb
					}
				}
			}
			if !a.gs.Done {
				t.Fatalf("placing the solution did not win the puzzle")
			}
			if _, ok := h.FindTextContains("Löst"); !ok {
				t.Fatalf("win banner missing; visible: %v", texts(h))
			}
		})
	}
}

// --- RULE: tap cycles empty -> bulb -> dot -> empty; walls are inert ---------

func TestPlayAkariToggleCycle(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	wx, wy := firstWhite(a)
	if a.gs.Marks[wy][wx] != game.MarkEmpty {
		t.Fatal("fresh white cell not empty")
	}
	tapCell(h, a, wx, wy)
	if a.gs.Marks[wy][wx] != game.MarkBulb {
		t.Fatalf("first tap should place a bulb, got %v", a.gs.Marks[wy][wx])
	}
	tapCell(h, a, wx, wy)
	if a.gs.Marks[wy][wx] != game.MarkDot {
		t.Fatalf("second tap should be a dot, got %v", a.gs.Marks[wy][wx])
	}
	tapCell(h, a, wx, wy)
	if a.gs.Marks[wy][wx] != game.MarkEmpty {
		t.Fatalf("third tap should clear, got %v", a.gs.Marks[wy][wx])
	}

	// Tapping a wall cell does nothing.
	if wallx, wally, found := firstWall(a); found {
		before := a.gs.Marks[wally][wallx]
		tapCell(h, a, wallx, wally)
		if a.gs.Marks[wally][wallx] != before {
			t.Fatal("a wall cell accepted a mark")
		}
	}
}

// --- RULE: two bulbs may not see each other ---------------------------------

func TestPlayAkariBulbConflict(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	// Find two white cells in one row with clear line of sight (no wall between).
	x1, x2, y := -1, -1, -1
	for yy := 0; yy < a.gs.Board.H && y < 0; yy++ {
		prev := -1
		for xx := 0; xx < a.gs.Board.W; xx++ {
			if a.gs.Board.Cells[yy][xx].Kind == game.Wall {
				prev = -1
				continue
			}
			if prev >= 0 {
				x1, x2, y = prev, xx, yy
				break
			}
			prev = xx
		}
	}
	if y < 0 {
		t.Skip("no two mutually-visible white cells in a row (tiny board)")
	}
	tapCell(h, a, x1, y)
	tapCell(h, a, x2, y)
	conf := a.gs.ConflictBulbs()
	if !conf[y][x1] || !conf[y][x2] {
		t.Fatalf("two bulbs sharing a row were not flagged as a conflict")
	}
	if a.gs.Done {
		t.Fatal("puzzle 'solved' with a bulb conflict")
	}
}

// --- "Rensa" clears the board -----------------------------------------------

func TestPlayAkariClear(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	wx, wy := firstWhite(a)
	tapCell(h, a, wx, wy) // a bulb
	if a.gs.Marks[wy][wx] != game.MarkBulb {
		t.Fatal("bulb not placed")
	}
	tapButton(t, h, a, "Rensa")
	for y := 0; y < a.gs.Board.H; y++ {
		for x := 0; x < a.gs.Board.W; x++ {
			if a.gs.Marks[y][x] != game.MarkEmpty {
				t.Fatalf("Rensa left a mark at (%d,%d)", x, y)
			}
		}
	}
}

// --- Quit (Meny + Back), new puzzle, rules ----------------------------------

func TestPlayAkariQuitAndRules(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

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
	if _, ok := h.FindTextContains("Två lampor"); !ok {
		t.Fatalf("rules text missing the no-see rule; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave rules, screen=%v", a.screen)
	}
}

func firstWhite(a *app) (int, int) {
	for y := 0; y < a.gs.Board.H; y++ {
		for x := 0; x < a.gs.Board.W; x++ {
			if isWhite(a, x, y) {
				return x, y
			}
		}
	}
	return 0, 0
}

func firstWall(a *app) (int, int, bool) {
	for y := 0; y < a.gs.Board.H; y++ {
		for x := 0; x < a.gs.Board.W; x++ {
			if a.gs.Board.Cells[y][x].Kind == game.Wall {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

// --- Screenshot -------------------------------------------------------------

func TestPlayAkariScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if e := h.Screenshot(dir + "/akari_splash.png"); e != nil {
		t.Fatalf("screenshot: %v", e)
	}
	h.TapXY(500, 700) // dismiss splash -> menu
	_ = h.Screenshot(dir + "/akari_menu.png")
	if _, ok := h.FindText("Regler"); ok {
		_ = h.TapText("Regler")
		_ = h.Screenshot(dir + "/akari_rules.png")
		h.Back()
	}

	start(t, h, a, 0)
	_ = h.Screenshot(dir + "/akari_board.png") // fresh, unsolved puzzle

	bulbs, _ := game.SolveBulbs(a.gs.Board)
	for y := 0; y < a.gs.Board.H; y++ {
		for x := 0; x < a.gs.Board.W; x++ {
			if bulbs[y][x] {
				tapCell(h, a, x, y)
			}
		}
	}
	if err := h.Screenshot(dir + "/akari_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
