//go:build playtest

package main

// Headless PLAYTHROUGH tests for Kakuro. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): fill the white cells with 1-9 so each "word" (a run of white cells)
// sums to the clue in the adjoining black cell, with no digit repeated in a run.
// The solve tests also assert a GENERATOR invariant — the stored solution
// actually satisfies every clue with distinct digits. Runs under the pure-Go
// inkview emulator (playtest/play.sh).

import (
	"os"
	"strconv"
	"testing"

	ink "github.com/dennwc/inkview"

	"kakuro/game"
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

func selectCell(h *ink.Harness, a *app, row, col int) {
	h.TapRect(a.layout.CellRect(row, col))
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

// setCell selects an entry cell and taps a digit.
func setCell(t *testing.T, h *ink.Harness, a *app, row, col, d int) {
	selectCell(h, a, row, col)
	tapButton(t, h, a, strconv.Itoa(d))
}

// fillSolution enters every entry cell's solution digit.
func fillSolution(t *testing.T, h *ink.Harness, a *app) {
	g := a.gs.Puz.Grid
	for r := range g {
		for c := range g[r] {
			if g[r][c].Kind == game.KindEntry {
				setCell(t, h, a, r, c, g[r][c].Solution)
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

// --- SOLVE every preset + verify the clues describe the solution ------------

func TestPlayKakuroSolveAllPresets(t *testing.T) {
	for i := range game.Presets {
		i := i
		t.Run(game.Presets[i].Name, func(t *testing.T) {
			h, a := bootToMenu(t)
			start(t, h, a, i)

			checkSolutionMatchesClues(t, a)

			fillSolution(t, h, a)
			if !a.gs.Done {
				t.Fatal("filling the solution did not win")
			}
			if _, ok := h.FindTextContains("Löst"); !ok {
				t.Fatalf("win banner missing; visible: %v", texts(h))
			}
		})
	}
}

// checkSolutionMatchesClues walks every run (right of a RightClue, down from a
// DownClue) and asserts the stored solution digits sum to the clue and repeat no
// digit — the rules of Kakuro applied to the generator's own answer.
func checkSolutionMatchesClues(t *testing.T, a *app) {
	t.Helper()
	g := a.gs.Puz.Grid
	H, W := len(g), len(g[0])
	for r := 0; r < H; r++ {
		for c := 0; c < W; c++ {
			cell := g[r][c]
			if cell.Kind != game.KindBlock {
				continue
			}
			if cell.RightClue > 0 {
				sum, seen := 0, map[int]bool{}
				for cc := c + 1; cc < W && g[r][cc].Kind == game.KindEntry; cc++ {
					v := g[r][cc].Solution
					if v < 1 || v > 9 || seen[v] {
						t.Fatalf("right run at (%d,%d): bad/repeated digit %d", r, c, v)
					}
					seen[v] = true
					sum += v
				}
				if sum != cell.RightClue {
					t.Fatalf("right run at (%d,%d) sums to %d, clue %d", r, c, sum, cell.RightClue)
				}
			}
			if cell.DownClue > 0 {
				sum, seen := 0, map[int]bool{}
				for rr := r + 1; rr < H && g[rr][c].Kind == game.KindEntry; rr++ {
					v := g[rr][c].Solution
					if v < 1 || v > 9 || seen[v] {
						t.Fatalf("down run at (%d,%d): bad/repeated digit %d", r, c, v)
					}
					seen[v] = true
					sum += v
				}
				if sum != cell.DownClue {
					t.Fatalf("down run at (%d,%d) sums to %d, clue %d", r, c, sum, cell.DownClue)
				}
			}
		}
	}
}

// --- RULE: the keypad greys digits already used in the selected run ---------

func TestPlayKakuroNoRepeatHint(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	// Find a run of at least two entry cells (right of a RightClue).
	g := a.gs.Puz.Grid
	var a1, a2 [2]int
	found := false
	for r := 0; r < len(g) && !found; r++ {
		for c := 0; c < len(g[r]); c++ {
			if g[r][c].Kind == game.KindBlock && g[r][c].RightClue > 0 {
				if c+2 < len(g[r]) && g[r][c+1].Kind == game.KindEntry && g[r][c+2].Kind == game.KindEntry {
					a1, a2 = [2]int{r, c + 1}, [2]int{r, c + 2}
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Skip("no horizontal run of length >= 2")
	}
	// Put a digit in the first cell, then select the second: that digit must be
	// reported as used in the run.
	setCell(t, h, a, a1[0], a1[1], 3)
	selectCell(h, a, a2[0], a2[1])
	if !a.usedDigitsForSelection()[3] {
		t.Fatal("digit 3 already in the run was not flagged as used")
	}
}

// --- RULE: an incomplete / wrong fill is not a win --------------------------

func TestPlayKakuroWrongAndPartial(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	// Find two entry cells.
	var cells [][2]int
	g := a.gs.Puz.Grid
	for r := range g {
		for c := range g[r] {
			if g[r][c].Kind == game.KindEntry {
				cells = append(cells, [2]int{r, c})
			}
		}
	}
	if len(cells) < 2 {
		t.Skip("tiny puzzle")
	}

	// Fill all but the last cell correctly -> not done.
	for _, rc := range cells[:len(cells)-1] {
		setCell(t, h, a, rc[0], rc[1], g[rc[0]][rc[1]].Solution)
	}
	if a.gs.Done {
		t.Fatal("won with a cell still empty")
	}
	// Put a WRONG digit in the last cell -> still not done.
	last := cells[len(cells)-1]
	wrong := g[last[0]][last[1]].Solution%9 + 1
	setCell(t, h, a, last[0], last[1], wrong)
	if a.gs.Done {
		t.Fatal("won with a wrong digit")
	}
	// Correct it -> done.
	setCell(t, h, a, last[0], last[1], g[last[0]][last[1]].Solution)
	if !a.gs.Done {
		t.Fatal("correcting the last digit did not win")
	}
}

// --- Erase (Sudda), quit, rules ---------------------------------------------

func TestPlayKakuroEraseQuitRules(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	// A first entry cell.
	var er, ec int
	g := a.gs.Puz.Grid
outer:
	for r := range g {
		for c := range g[r] {
			if g[r][c].Kind == game.KindEntry {
				er, ec = r, c
				break outer
			}
		}
	}
	setCell(t, h, a, er, ec, 5)
	if a.gs.Puz.Grid[er][ec].Value != 5 {
		t.Fatal("digit not set")
	}
	selectCell(h, a, er, ec)
	tapButton(t, h, a, "Sudda")
	if a.gs.Puz.Grid[er][ec].Value != 0 {
		t.Fatalf("Sudda did not clear the cell (got %d)", a.gs.Puz.Grid[er][ec].Value)
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
	if _, ok := h.FindTextContains("summerar"); !ok {
		t.Fatalf("rules text missing; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave rules, screen=%v", a.screen)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayKakuroScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	h, a := bootToMenu(t)
	start(t, h, a, 0)
	fillSolution(t, h, a)
	if err := h.Screenshot(dir + "/kakuro_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
