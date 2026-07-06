//go:build playtest

package main

// Headless PLAYTHROUGH tests for Slitherlink. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): toggle edges between dots to draw a single closed loop such that each
// numbered cell has exactly that many of its four edges on. Tapping an edge
// cycles line -> X -> empty. The solve tests also assert GENERATOR invariants —
// the clues match the solution loop and uniquely determine it. Runs under the
// pure-Go inkview emulator (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"slitherlink/game"
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

// tapH/tapV tap the midpoint of a horizontal / vertical edge, matching the
// midpoints ScreenToEdge hit-tests against.
func tapH(h *ink.Harness, a *app, x, y int) {
	l := a.layout
	h.Tap(image.Pt(l.GridOrigin.X+x*l.CellSize+l.CellSize/2, l.GridOrigin.Y+y*l.CellSize))
}

func tapV(h *ink.Harness, a *app, x, y int) {
	l := a.layout
	h.Tap(image.Pt(l.GridOrigin.X+x*l.CellSize, l.GridOrigin.Y+y*l.CellSize+l.CellSize/2))
}

// solveLoop turns on exactly the solution's loop edges.
func solveLoop(h *ink.Harness, a *app) {
	puz := a.gs.Puz
	for y := 0; y < len(puz.SolutionH); y++ {
		for x := 0; x < len(puz.SolutionH[y]); x++ {
			if puz.SolutionH[y][x] {
				tapH(h, a, x, y)
			}
		}
	}
	for y := 0; y < len(puz.SolutionV); y++ {
		for x := 0; x < len(puz.SolutionV[y]); x++ {
			if puz.SolutionV[y][x] {
				tapV(h, a, x, y)
			}
		}
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

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- SOLVE every preset + verify clue/uniqueness invariants -----------------

func TestPlaySlitherlinkSolveAllPresets(t *testing.T) {
	for i := range game.Presets {
		i := i
		t.Run(game.Presets[i].Name, func(t *testing.T) {
			h, a := bootToMenu(t)
			start(t, h, a, i)
			puz := a.gs.Puz

			// RULE: each numbered cell's clue equals how many of its four edges the
			// solution loop uses.
			for y := 0; y < puz.H; y++ {
				for x := 0; x < puz.W; x++ {
					if puz.Clue[y][x] < 0 {
						continue
					}
					n := 0
					for _, on := range []bool{
						puz.SolutionH[y][x], puz.SolutionH[y+1][x],
						puz.SolutionV[y][x], puz.SolutionV[y][x+1],
					} {
						if on {
							n++
						}
					}
					if n != puz.Clue[y][x] {
						t.Fatalf("clue at (%d,%d)=%d but solution uses %d edges", x, y, puz.Clue[y][x], n)
					}
				}
			}
			// GENERATOR RULE: the clues uniquely determine the loop.
			if game.Solve(puz) != game.SolveUnique {
				t.Fatalf("puzzle is not uniquely solvable")
			}

			// Draw the loop through the UI and win.
			solveLoop(h, a)
			if !a.gs.Done {
				t.Fatal("drawing the solution loop did not win")
			}
			if !a.gs.Bd.AllCluesSatisfied() {
				t.Fatal("won but not all clues satisfied")
			}
			if !game.IsSingleLoop(a.gs.Bd.HEdge, a.gs.Bd.VEdge, a.gs.Bd.W, a.gs.Bd.H) {
				t.Fatal("won but the edges are not a single loop")
			}
			if _, ok := h.FindTextContains("Löst"); !ok {
				t.Fatalf("win banner missing; visible: %v", texts(h))
			}
		})
	}
}

// --- RULE: tapping an edge cycles line -> X -> empty ------------------------

func TestPlaySlitherlinkToggleCycle(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	tapH(h, a, 0, 0)
	if a.gs.Bd.HEdge[0][0] != game.EdgeOn {
		t.Fatalf("first tap should draw a line, got %v", a.gs.Bd.HEdge[0][0])
	}
	tapH(h, a, 0, 0)
	if a.gs.Bd.HEdge[0][0] != game.EdgeOff {
		t.Fatalf("second tap should be an X, got %v", a.gs.Bd.HEdge[0][0])
	}
	tapH(h, a, 0, 0)
	if a.gs.Bd.HEdge[0][0] != game.EdgeUnknown {
		t.Fatalf("third tap should clear, got %v", a.gs.Bd.HEdge[0][0])
	}
}

// --- RULE: an incomplete / non-loop edge set is not a win -------------------

func TestPlaySlitherlinkPartialIsNotWon(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	// Turn on all-but-one of the loop's edges: the loop is broken, so not solved.
	puz := a.gs.Puz
	type edge struct {
		isH  bool
		x, y int
	}
	var on []edge
	for y := 0; y < len(puz.SolutionH); y++ {
		for x := 0; x < len(puz.SolutionH[y]); x++ {
			if puz.SolutionH[y][x] {
				on = append(on, edge{true, x, y})
			}
		}
	}
	for y := 0; y < len(puz.SolutionV); y++ {
		for x := 0; x < len(puz.SolutionV[y]); x++ {
			if puz.SolutionV[y][x] {
				on = append(on, edge{false, x, y})
			}
		}
	}
	for _, e := range on[:len(on)-1] { // leave the last edge off
		if e.isH {
			tapH(h, a, e.x, e.y)
		} else {
			tapV(h, a, e.x, e.y)
		}
	}
	if a.gs.Done {
		t.Fatal("an open (incomplete) loop counted as solved")
	}
	// Close it and win.
	last := on[len(on)-1]
	if last.isH {
		tapH(h, a, last.x, last.y)
	} else {
		tapV(h, a, last.x, last.y)
	}
	if !a.gs.Done {
		t.Fatal("closing the final edge did not win")
	}
}

// --- Clear/quit/rules -------------------------------------------------------

func TestPlaySlitherlinkClearQuitRules(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	tapH(h, a, 0, 0)
	tapButton(t, h, a, "Rensa")
	if a.gs.Bd.HEdge[0][0] != game.EdgeUnknown {
		t.Fatal("Rensa did not clear the edges")
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
	if _, ok := h.FindTextContains("sluten slinga"); !ok {
		t.Fatalf("rules text missing; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave rules, screen=%v", a.screen)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlaySlitherlinkScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if e := h.Screenshot(dir + "/slitherlink_splash.png"); e != nil {
		t.Fatal(e)
	}
	h.TapXY(500, 700)
	_ = h.Screenshot(dir + "/slitherlink_menu.png")
	h.TapRect(a.menu.RulesButton())
	if a.screen == screenRules {
		_ = h.Screenshot(dir + "/slitherlink_rules.png")
		h.Back()
	}
	start(t, h, a, 0)
	// Draw only the horizontal loop edges: an in-progress board.
	puz := a.gs.Puz
	for y := 0; y < len(puz.SolutionH); y++ {
		for x := 0; x < len(puz.SolutionH[y]); x++ {
			if puz.SolutionH[y][x] {
				tapH(h, a, x, y)
			}
		}
	}
	_ = h.Screenshot(dir + "/slitherlink_board.png")
	// Close the loop with the vertical edges to win.
	for y := 0; y < len(puz.SolutionV); y++ {
		for x := 0; x < len(puz.SolutionV[y]); x++ {
			if puz.SolutionV[y][x] {
				tapV(h, a, x, y)
			}
		}
	}
	if err := h.Screenshot(dir + "/slitherlink_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
