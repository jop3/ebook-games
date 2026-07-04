//go:build playtest

package main

// Headless PLAYTHROUGH tests for Einsteins Gåta (a logic-grid deduction puzzle).
// They drive the real touch path and check the gameplay against the rules as
// written (see rulesParagraphs in ui.go): from the clues, deduce the one correct
// combination by marking the note grid; each puzzle has exactly one solution and
// needs no guessing. Tapping a cell cycles unknown -> possible -> impossible ->
// unknown; "submit" checks the deduced answer. The solve tests also assert the
// GENERATOR invariant — every puzzle has a unique solution. Runs under the
// pure-Go inkview emulator (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"
)

// --- helpers ----------------------------------------------------------------

func bootToMenu(t *testing.T) (*ink.Harness, *Game) {
	t.Helper()
	g := NewGame()
	h, err := ink.Boot(g)
	if err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if g.scr != screenMenu {
		t.Fatalf("splash tap did not open menu, scr=%v", g.scr)
	}
	return h, g
}

func tapBtn(t *testing.T, h *ink.Harness, g *Game, action string) {
	t.Helper()
	for _, b := range g.btnHits {
		if b.action == action {
			h.TapRect(b.r)
			return
		}
	}
	t.Fatalf("no button with action %q", action)
}

func startDifficulty(t *testing.T, h *ink.Harness, g *Game, d Difficulty) {
	t.Helper()
	for _, b := range g.btnHits {
		if b.action == "difficulty" && b.data == int(d) {
			h.TapRect(b.r)
			break
		}
	}
	if g.menuChoice != d {
		t.Fatalf("difficulty %d not selected (got %d)", d, g.menuChoice)
	}
	tapBtn(t, h, g, "start")
	if g.scr != screenPlay {
		t.Fatalf("did not enter play, scr=%v", g.scr)
	}
}

// tapNoteCell taps the grid cell for pair (anchor 0, av) x (category c, rv).
func tapNoteCell(t *testing.T, h *ink.Harness, g *Game, av, c, rv int) {
	t.Helper()
	for _, ch := range g.cellHits {
		if ch.ca == 0 && ch.va == av && ch.cb == c && ch.vb == rv {
			h.TapRect(ch.r)
			return
		}
	}
	t.Fatalf("no note cell for (0,%d)x(%d,%d)", av, c, rv)
}

// solveViaUI marks exactly the solution's pairings against anchor category 0.
func solveViaUI(t *testing.T, h *ink.Harness, g *Game) {
	sol := g.puzzle.Solution
	n, cats := g.puzzle.N, g.puzzle.Categories
	solPosOfAnchorVal := make([]int, n)
	for p := 0; p < n; p++ {
		solPosOfAnchorVal[int(sol.Assignment[0][p])] = p
	}
	for av := 0; av < n; av++ {
		sp := solPosOfAnchorVal[av]
		for c := 1; c < cats; c++ {
			rv := int(sol.Assignment[c][sp])
			tapNoteCell(t, h, g, av, c, rv) // unknown -> possible
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

// --- SOLVE every difficulty + verify the generator invariant ----------------

func TestPlayEinsteinSolveAllDifficulties(t *testing.T) {
	for _, d := range []Difficulty{Easy, Medium, Hard} {
		d := d
		t.Run(playDiffName(d), func(t *testing.T) {
			h, g := bootToMenu(t)
			startDifficulty(t, h, g, d)

			// GENERATOR RULE: the puzzle has exactly one solution.
			if n := CountSolutions(g.puzzle.N, g.puzzle.Categories, g.puzzle.Clues); n != 1 {
				t.Fatalf("puzzle has %d solutions, want unique", n)
			}

			solveViaUI(t, h, g)
			// The deduced answer should already be correct+complete.
			if correct, complete := g.notes.CheckAgainst(g.puzzle.Solution, 0); !correct || !complete {
				t.Fatalf("marked solution not accepted (correct=%v complete=%v)", correct, complete)
			}
			tapBtn(t, h, g, "submit")
			if g.scr != screenResult || !g.resultCorrect {
				t.Fatalf("submit did not report a solved puzzle (scr=%v correct=%v)", g.scr, g.resultCorrect)
			}
			if _, ok := h.FindTextContains("Rätt"); !ok {
				t.Fatalf("win banner missing; visible: %v", texts(h))
			}
		})
	}
}

func playDiffName(d Difficulty) string {
	switch d {
	case Easy:
		return "Easy"
	case Medium:
		return "Medium"
	default:
		return "Hard"
	}
}

// --- RULE: tapping a cell cycles unknown -> possible -> impossible -> unknown -

func TestPlayEinsteinNoteCycle(t *testing.T) {
	h, g := bootToMenu(t)
	startDifficulty(t, h, g, Easy)

	// Use the first recorded cell.
	if len(g.cellHits) == 0 {
		t.Fatal("no note cells drawn")
	}
	c := g.cellHits[0]
	get := func() Mark { return g.notes.Get(c.ca, c.va, c.cb, c.vb) }

	h.TapRect(c.r)
	if get() != Possible {
		t.Fatalf("first tap should mark possible, got %v", get())
	}
	h.TapRect(c.r)
	if get() != Impossible {
		t.Fatalf("second tap should mark impossible, got %v", get())
	}
	h.TapRect(c.r)
	if get() != Unknown {
		t.Fatalf("third tap should clear, got %v", get())
	}
}

// --- RULE: submitting an incomplete or wrong grid is not a win --------------

func TestPlayEinsteinWrongSubmit(t *testing.T) {
	h, g := bootToMenu(t)
	startDifficulty(t, h, g, Easy)

	// Submit an empty grid: not complete, so not correct.
	tapBtn(t, h, g, "submit")
	if g.scr != screenResult || g.resultCorrect {
		t.Fatalf("empty grid reported solved (scr=%v correct=%v)", g.scr, g.resultCorrect)
	}
	// Back to the puzzle.
	tapBtn(t, h, g, "menu")
	if g.scr != screenMenu {
		t.Fatalf("Till meny did not return to menu, scr=%v", g.scr)
	}
	startDifficulty(t, h, g, Easy)

	// Solve correctly, then corrupt one pairing to a different value -> wrong.
	solveViaUI(t, h, g)
	sol := g.puzzle.Solution
	n := g.puzzle.N
	solPos := make([]int, n)
	for p := 0; p < n; p++ {
		solPos[int(sol.Assignment[0][p])] = p
	}
	// For anchor value 0, category 1: clear the correct mark, set a wrong one.
	correctRv := int(sol.Assignment[1][solPos[0]])
	wrongRv := (correctRv + 1) % n
	tapNoteCell(t, h, g, 0, 1, correctRv) // possible -> impossible (no longer the pick)
	// Mark a different value as the (wrong) possible for this row.
	// Cycle wrongRv up to Possible.
	if g.notes.Get(0, 0, 1, wrongRv) == Unknown {
		tapNoteCell(t, h, g, 0, 1, wrongRv) // -> possible
	}
	tapBtn(t, h, g, "submit")
	if g.resultCorrect {
		t.Fatal("a corrupted grid was reported correct")
	}
}

// --- Quit / rules -----------------------------------------------------------

func TestPlayEinsteinQuitRules(t *testing.T) {
	h, g := bootToMenu(t)
	startDifficulty(t, h, g, Easy)

	h.Back() // hardware Back leaves play
	if g.scr != screenMenu {
		t.Fatalf("Back did not return to menu, scr=%v", g.scr)
	}

	tapBtn(t, h, g, "rules")
	if g.scr != screenRules {
		t.Fatalf("rules button did not open rules, scr=%v", g.scr)
	}
	if _, ok := h.FindTextContains("ledtråd"); !ok {
		t.Fatalf("rules text missing; visible: %v", texts(h))
	}
	h.Back()
	if g.scr != screenMenu {
		t.Fatalf("Back did not leave rules, scr=%v", g.scr)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayEinsteinScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	h, g := bootToMenu(t)
	startDifficulty(t, h, g, Easy)
	solveViaUI(t, h, g)
	tapBtn(t, h, g, "submit")
	if err := h.Screenshot(dir + "/einstein_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
