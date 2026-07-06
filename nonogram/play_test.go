//go:build playtest

package main

// Headless PLAYTHROUGH tests for Nonogram. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): the row/column numbers give the lengths of the filled blocks in order;
// fill the cells to reveal the hidden picture; tapping cycles filled -> X -> empty.
// The solve tests also assert GENERATOR invariants — the clues describe the
// solution and uniquely determine it by line logic. Runs under the pure-Go
// inkview emulator (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"nonogram/game"
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

// fillSolution taps each cell the hidden picture wants filled (once: Unknown->Filled).
func fillSolution(h *ink.Harness, a *app) {
	for y := 0; y < a.gs.Puz.H; y++ {
		for x := 0; x < a.gs.Puz.W; x++ {
			if a.gs.Puz.Solution[y][x] {
				tapCell(h, a, x, y)
			}
		}
	}
}

func clueEqual(a, b game.Clue) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- SOLVE every preset + verify the clue/uniqueness invariants -------------

func TestPlayNonogramSolveAllPresets(t *testing.T) {
	for i := range game.Presets {
		i := i
		t.Run(game.Presets[i].Name, func(t *testing.T) {
			h, a := bootToMenu(t)
			start(t, h, a, i)
			puz := a.gs.Puz

			// RULE: the clues must describe the solution, row by row and column by
			// column (the numbers are the run lengths of the filled picture).
			for y := 0; y < puz.H; y++ {
				if !clueEqual(game.LineClue(puz.Solution[y]), puz.RowClues[y]) {
					t.Fatalf("row %d clue does not match the solution", y)
				}
			}
			for x := 0; x < puz.W; x++ {
				col := make([]bool, puz.H)
				for y := 0; y < puz.H; y++ {
					col[y] = puz.Solution[y][x]
				}
				if !clueEqual(game.LineClue(col), puz.ColClues[x]) {
					t.Fatalf("col %d clue does not match the solution", x)
				}
			}
			// GENERATOR RULE: the clues uniquely determine the picture by logic.
			if game.LineSolvable(puz.RowClues, puz.ColClues) != game.SolveUnique {
				t.Fatalf("clues do not uniquely determine the picture")
			}

			// Solve it through the UI.
			fillSolution(h, a)
			if !a.gs.Done {
				t.Fatal("filling the picture did not win")
			}
			if _, ok := h.FindTextContains("Löst"); !ok {
				t.Fatalf("win banner missing; visible: %v", texts(h))
			}
		})
	}
}

// --- RULE: tap cycles filled -> X -> empty ----------------------------------

func TestPlayNonogramToggleCycle(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	tapCell(h, a, 0, 0)
	if a.gs.Cells[0][0] != game.StateFilled {
		t.Fatalf("first tap should fill, got %v", a.gs.Cells[0][0])
	}
	tapCell(h, a, 0, 0)
	if a.gs.Cells[0][0] != game.StateMarked {
		t.Fatalf("second tap should mark (X), got %v", a.gs.Cells[0][0])
	}
	tapCell(h, a, 0, 0)
	if a.gs.Cells[0][0] != game.StateUnknown {
		t.Fatalf("third tap should clear, got %v", a.gs.Cells[0][0])
	}
}

// --- RULE: only exact FILLED cells win; X marks and over-fills don't --------
//
// The board locks the moment it is solved (Toggle is a no-op once Done), so
// these rule violations must be exercised BEFORE the puzzle is completed.

func TestPlayNonogramMarksDontCount(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	// Need two filled cells: S is the one we'll mark, S2 kept blank so marking S
	// never completes the picture (which would lock the board).
	var filled [][2]int
	for y := 0; y < a.gs.Puz.H; y++ {
		for x := 0; x < a.gs.Puz.W; x++ {
			if a.gs.Puz.Solution[y][x] {
				filled = append(filled, [2]int{x, y})
			}
		}
	}
	if len(filled) < 2 {
		t.Skip("solution has fewer than two filled cells")
	}
	s, s2 := filled[0], filled[1]

	// Fill every solution cell except S and S2.
	for _, c := range filled[2:] {
		tapCell(h, a, c[0], c[1])
	}
	tapCell(h, a, s[0], s[1]) // Unknown -> Filled (S2 still missing, so not Done)
	tapCell(h, a, s[0], s[1]) // Filled -> Marked (X)
	if a.gs.Cells[s[1]][s[0]] != game.StateMarked {
		t.Fatalf("cell not marked (got %v)", a.gs.Cells[s[1]][s[0]])
	}
	if a.gs.Done {
		t.Fatal("a marked cell wrongly counted as a fill")
	}
	// Make S a real fill and add S2: now the picture is complete.
	tapCell(h, a, s[0], s[1])   // Marked -> Unknown
	tapCell(h, a, s[0], s[1])   // Unknown -> Filled
	tapCell(h, a, s2[0], s2[1]) // fill the last cell
	if !a.gs.Done {
		t.Fatal("filling the last required cell did not win")
	}
}

func TestPlayNonogramOverfillDoesntWin(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	bx, by := firstBlank(a)
	if by < 0 {
		t.Skip("solution has no blank cell")
	}
	tapCell(h, a, bx, by) // fill a cell the picture wants blank
	fillSolution(h, a)    // then fill the whole real picture
	if a.gs.Done {
		t.Fatal("board 'solved' despite an extra filled cell")
	}
}

func firstBlank(a *app) (int, int) {
	for y := 0; y < a.gs.Puz.H; y++ {
		for x := 0; x < a.gs.Puz.W; x++ {
			if !a.gs.Puz.Solution[y][x] {
				return x, y
			}
		}
	}
	return -1, -1
}

// --- Clear/quit/rules -------------------------------------------------------

func TestPlayNonogramClearQuitRules(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, 0)

	tapCell(h, a, 0, 0)
	tapButton(t, h, a, "Rensa")
	if a.gs.Cells[0][0] != game.StateUnknown {
		t.Fatal("Rensa did not clear the board")
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
	if _, ok := h.FindTextContains("dolda bilden"); !ok {
		t.Fatalf("rules text missing; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave rules, screen=%v", a.screen)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayNonogramScreenshot(t *testing.T) {
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
	if e := h.Screenshot(dir + "/nonogram_splash.png"); e != nil {
		t.Fatal(e)
	}
	h.TapXY(500, 700) // dismiss splash -> menu
	if a.screen != screenMenu {
		t.Fatalf("splash tap did not open menu, screen=%v", a.screen)
	}
	_ = h.Screenshot(dir + "/nonogram_menu.png")

	h.TapRect(a.menu.RulesButton())
	_ = h.Screenshot(dir + "/nonogram_rules.png")
	h.Back()

	start(t, h, a, 0)

	// Partial fill: about half of the solution cells, for an in-progress shot.
	total := 0
	for y := 0; y < a.gs.Puz.H; y++ {
		for x := 0; x < a.gs.Puz.W; x++ {
			if a.gs.Puz.Solution[y][x] {
				total++
			}
		}
	}
	done := 0
	for y := 0; y < a.gs.Puz.H && done*2 < total; y++ {
		for x := 0; x < a.gs.Puz.W && done*2 < total; x++ {
			if a.gs.Puz.Solution[y][x] {
				tapCell(h, a, x, y)
				done++
			}
		}
	}
	_ = h.Screenshot(dir + "/nonogram_board.png")

	// Reset the partial work and solve fully for the win shot.
	tapButton(t, h, a, "Rensa")
	fillSolution(h, a)
	if err := h.Screenshot(dir + "/nonogram_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
