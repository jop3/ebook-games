//go:build playtest

package main

// Headless PLAYTHROUGH tests for Black Box. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): fire rays from the edge points to probe for hidden atoms, then switch
// to guessing, mark the suspected atom cells and submit; you win when every atom
// is marked with no wrong or missing guesses. Covers a full win on all presets,
// a wrong guess, the ray-firing/dedupe mechanic, and quit/replay/rules. Runs
// under the pure-Go inkview emulator (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"blackbox/game"
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

func tapCell(h *ink.Harness, a *app, x, y int) {
	h.TapRect(a.layout.CellToScreen(x, y))
}

func fireEdge(h *ink.Harness, a *app, i int) {
	h.TapRect(a.layout.edgeRects[i])
}

// markAtoms taps every hidden atom cell (guessing phase).
func markAtoms(h *ink.Harness, a *app) {
	for _, c := range a.gs.Grid.Atoms() {
		tapCell(h, a, c.X, c.Y)
	}
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- WIN on every preset ----------------------------------------------------

func TestPlayBlackboxWinAllPresets(t *testing.T) {
	for i := range game.Presets {
		i := i
		t.Run(game.Presets[i].Name, func(t *testing.T) {
			h, a := bootToMenu(t)
			start(t, h, a, i)

			// Probe with a couple of rays.
			fireEdge(h, a, 0)
			fireEdge(h, a, 1)
			if a.gs.RaysFired() != 2 {
				t.Fatalf("expected 2 rays fired, got %d", a.gs.RaysFired())
			}

			// Switch to guessing and mark the true atoms.
			tapButton(t, h, a, "Gissa atomer")
			if a.gs.Phase != game.PhaseGuessing {
				t.Fatalf("Gissa atomer did not enter guessing, phase=%v", a.gs.Phase)
			}
			markAtoms(h, a)
			if a.gs.GuessCount() != a.gs.Grid.AtomCount() {
				t.Fatalf("marked %d cells, there are %d atoms", a.gs.GuessCount(), a.gs.Grid.AtomCount())
			}

			tapButton(t, h, a, "Lämna in")
			if a.gs.Phase != game.PhaseDone {
				t.Fatalf("Lämna in did not finish the game, phase=%v", a.gs.Phase)
			}
			if !a.gs.Solved() {
				t.Fatalf("marking every atom did not solve it (correct=%d wrong=%d missed=%d)",
					a.gs.CorrectAtoms, a.gs.WrongAtoms, a.gs.MissedAtoms)
			}
			if _, ok := h.FindTextContains("Löst"); !ok {
				t.Fatalf("win banner missing; visible: %v", texts(h))
			}
		})
	}
}

// --- A wrong guess does not solve it ----------------------------------------

func TestPlayBlackboxWrongGuess(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	tapButton(t, h, a, "Gissa atomer")
	// Mark a cell that has NO atom.
	wx, wy := -1, -1
	for y := 0; y < a.gs.Cfg.H && wy < 0; y++ {
		for x := 0; x < a.gs.Cfg.W; x++ {
			if !a.gs.Grid.HasAtom(x, y) {
				wx, wy = x, y
				break
			}
		}
	}
	if wx < 0 {
		t.Skip("grid is entirely atoms?!")
	}
	tapCell(h, a, wx, wy)
	tapButton(t, h, a, "Lämna in")
	if a.gs.Solved() {
		t.Fatal("a wrong guess was reported solved")
	}
	if a.gs.WrongAtoms < 1 {
		t.Fatalf("wrong guess not counted (wrong=%d)", a.gs.WrongAtoms)
	}
	// Score includes the wrong-atom penalty.
	if a.gs.Score < a.gs.Cfg.WrongAtomPenalty {
		t.Fatalf("score %d does not include the wrong penalty %d", a.gs.Score, a.gs.Cfg.WrongAtomPenalty)
	}
}

// --- RULE: firing the same edge twice does not record a second ray ----------

func TestPlayBlackboxRayDedup(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	fireEdge(h, a, 0)
	if a.gs.RaysFired() != 1 {
		t.Fatalf("first fire gave %d rays", a.gs.RaysFired())
	}
	fireEdge(h, a, 0) // same edge again
	if a.gs.RaysFired() != 1 {
		t.Fatalf("re-firing the same edge added a ray (%d)", a.gs.RaysFired())
	}
	fireEdge(h, a, 2)
	if a.gs.RaysFired() != 2 {
		t.Fatalf("firing a new edge did not add a ray (%d)", a.gs.RaysFired())
	}
	// Every ray has a defined outcome.
	for _, fr := range a.gs.Fired {
		switch fr.Result.Outcome {
		case game.OutcomeHit, game.OutcomeReflection, game.OutcomeDetour:
		default:
			t.Fatalf("ray has an undefined outcome %v", fr.Result.Outcome)
		}
	}
}

// --- Replay / quit / rules --------------------------------------------------

func TestPlayBlackboxReplayQuitRules(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	// Win, then "Spela igen" restarts.
	tapButton(t, h, a, "Gissa atomer")
	markAtoms(h, a)
	tapButton(t, h, a, "Lämna in")
	if !a.gs.Solved() {
		t.Fatal("setup win failed")
	}
	tapButton(t, h, a, "Spela igen")
	if a.gs.Phase != game.PhaseProbing || a.gs.RaysFired() != 0 || a.gs.GuessCount() != 0 {
		t.Fatalf("Spela igen did not reset (phase=%v rays=%d guesses=%d)",
			a.gs.Phase, a.gs.RaysFired(), a.gs.GuessCount())
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
	if _, ok := h.FindTextContains("dolda atomerna"); !ok {
		t.Fatalf("rules text missing; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave rules, screen=%v", a.screen)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayBlackboxScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	h, a := bootToMenu(t)
	start(t, h, a, 0)
	fireEdge(h, a, 0)
	fireEdge(h, a, 3)
	tapButton(t, h, a, "Gissa atomer")
	markAtoms(h, a)
	tapButton(t, h, a, "Lämna in")
	if err := h.Screenshot(dir + "/blackbox_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
