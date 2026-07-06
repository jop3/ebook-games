//go:build playtest

package main

// Headless PLAYTHROUGH tests for Sudoku. They drive the real touch path and
// check the gameplay against the rules as written (see the rules text in
// render.go): fill every empty cell so each row, column and 3x3 box holds 1..9
// once; given cells are immutable; "Klar?" reports conflicts / not-done / right /
// wrong. The solve tests also assert the GENERATOR invariant — every puzzle has
// a unique solution — which is exactly where a generator bug would hide. Runs
// under the pure-Go inkview emulator (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"sudoku/game"
)

// --- helpers ----------------------------------------------------------------

func bootToMenu(t *testing.T) (*ink.Harness, *app) {
	t.Helper()
	a := newApp()
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

func startDiff(t *testing.T, h *ink.Harness, a *app, d game.Difficulty) {
	t.Helper()
	h.TapRect(menuButtonRect(int(d)))
	if a.screen != screenPlay {
		t.Fatalf("did not enter play for difficulty %d, screen=%v", d, a.screen)
	}
	if a.diff != d {
		t.Fatalf("started difficulty %d, wanted %d", a.diff, d)
	}
}

func selectCell(h *ink.Harness, a *app, r, c int) {
	l := computeLayout()
	h.TapRect(l.cellRect(r, c))
}

func tapNum(h *ink.Harness, a *app, d int) {
	l := computeLayout()
	h.TapRect(l.numButtonRect(d))
}

// action indices: 0=Anteckn 1=Sudda 2=Klar? 3=Ny
func tapAction(h *ink.Harness, a *app, i int) {
	l := computeLayout()
	h.TapRect(l.actionButtonRect(i, len(a.actionLabels())))
}

// fillSolution enters each non-given cell's solution digit. It only taps cells
// that aren't already correct, because tapping the digit a cell already holds
// CLEARS it (placeDigit's toggle-off) — so this stays correct even on a board
// that was dirtied earlier in a test.
func fillSolution(h *ink.Harness, a *app) {
	for r := 0; r < game.N; r++ {
		for c := 0; c < game.N; c++ {
			if a.puzzle.Given[r][c] {
				continue
			}
			if a.board[r][c] == a.puzzle.Solution[r][c] {
				continue
			}
			selectCell(h, a, r, c)
			tapNum(h, a, a.puzzle.Solution[r][c])
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

func TestPlaySudokuSolveAllDifficulties(t *testing.T) {
	for _, d := range []game.Difficulty{game.Easy, game.Medium, game.Hard} {
		d := d
		t.Run(menuLabels[int(d)], func(t *testing.T) {
			h, a := bootToMenu(t)
			startDiff(t, h, a, d)

			// GENERATOR RULES: the start must have exactly one solution, that
			// solution is the stored one, and it is itself a valid full grid.
			if n := a.puzzle.Start.CountSolutions(2); n != 1 {
				t.Fatalf("generated puzzle has %d solutions, want unique", n)
			}
			if solved, ok := a.puzzle.Start.Solve(); !ok || solved != a.puzzle.Solution {
				t.Fatalf("Solve(Start) != stored Solution (ok=%v)", ok)
			}
			if !a.puzzle.Solution.IsSolved() {
				t.Fatal("stored Solution is not a valid solved grid")
			}
			// Given flags must match the non-empty start cells.
			for r := 0; r < game.N; r++ {
				for c := 0; c < game.N; c++ {
					if a.puzzle.Given[r][c] != (a.puzzle.Start[r][c] != 0) {
						t.Fatalf("Given[%d][%d] disagrees with Start", r, c)
					}
				}
			}

			// Solve it through the UI and confirm the board matches the solution.
			fillSolution(h, a)
			if a.board != a.puzzle.Solution {
				t.Fatal("filling every cell with the solution did not reproduce it")
			}
			if !a.board.IsSolved() {
				t.Fatal("completed board is not solved")
			}
			tapAction(h, a, 2) // "Klar?"
			if _, ok := h.FindTextContains("Ratt"); !ok {
				t.Fatalf("solved board not confirmed; visible: %v", texts(h))
			}
		})
	}
}

// --- RULE: given cells are immutable ----------------------------------------

func TestPlaySudokuGivensImmutable(t *testing.T) {
	h, a := bootToMenu(t)
	startDiff(t, h, a, game.Easy)

	// Find a given cell.
	gr, gc := -1, -1
	for r := 0; r < game.N && gr < 0; r++ {
		for c := 0; c < game.N; c++ {
			if a.puzzle.Given[r][c] {
				gr, gc = r, c
				break
			}
		}
	}
	if gr < 0 {
		t.Fatal("no given cell?!")
	}
	orig := a.board[gr][gc]
	selectCell(h, a, gr, gc)
	want := orig%9 + 1 // some different digit
	tapNum(h, a, want)
	if a.board[gr][gc] != orig {
		t.Fatalf("given cell changed from %d to %d", orig, a.board[gr][gc])
	}
}

// --- RULE: "Klar?" flags conflicts, incompleteness, and wrong completion ----

func TestPlaySudokuCheckStates(t *testing.T) {
	h, a := bootToMenu(t)
	startDiff(t, h, a, game.Easy)

	// Two empty non-given cells in one row, same digit -> a row conflict.
	row, c1, c2 := -1, -1, -1
	for r := 0; r < game.N && row < 0; r++ {
		var cols []int
		for c := 0; c < game.N; c++ {
			if !a.puzzle.Given[r][c] {
				cols = append(cols, c)
			}
		}
		if len(cols) >= 2 {
			row, c1, c2 = r, cols[0], cols[1]
		}
	}
	if row < 0 {
		t.Fatal("no row with two empty cells")
	}
	selectCell(h, a, row, c1)
	tapNum(h, a, 5)
	selectCell(h, a, row, c2)
	tapNum(h, a, 5)
	if len(a.board.Conflicts()) == 0 {
		t.Fatal("two equal digits in a row produced no conflict")
	}
	tapAction(h, a, 2) // Klar?
	if _, ok := h.FindTextContains("Konflikter"); !ok {
		t.Fatalf("conflict not reported; visible: %v", texts(h))
	}

	// Now fill correctly but make one cell wrong -> complete but not solved.
	fillSolution(h, a)
	wr, wc := -1, -1
	for r := 0; r < game.N && wr < 0; r++ {
		for c := 0; c < game.N; c++ {
			if !a.puzzle.Given[r][c] {
				wr, wc = r, c
				break
			}
		}
	}
	selectCell(h, a, wr, wc)
	wrong := a.board[wr][wc]%9 + 1
	tapNum(h, a, wrong)
	if a.board.IsComplete() && a.board.IsSolved() {
		t.Fatal("a wrong digit still counted as solved")
	}
	tapAction(h, a, 2) // Klar?
	if _, ok := h.FindTextContains("Fel"); !ok {
		t.Fatalf("wrong completion not reported; visible: %v", texts(h))
	}
}

// --- Erase and pencil-note mode ---------------------------------------------

func TestPlaySudokuEraseAndNotes(t *testing.T) {
	h, a := bootToMenu(t)
	startDiff(t, h, a, game.Easy)

	// A non-given cell.
	r, c := firstEmpty(a)
	selectCell(h, a, r, c)
	tapNum(h, a, 7)
	if a.board[r][c] != 7 {
		t.Fatalf("digit not placed (got %d)", a.board[r][c])
	}
	tapAction(h, a, 1) // Sudda
	if a.board[r][c] != 0 {
		t.Fatalf("Sudda did not clear the cell (got %d)", a.board[r][c])
	}

	// Note mode: digits become pencil marks, the big value stays empty.
	tapAction(h, a, 0) // Anteckn
	if !a.noteMode {
		t.Fatal("note mode did not turn on")
	}
	selectCell(h, a, r, c)
	tapNum(h, a, 3)
	if a.board[r][c] != 0 {
		t.Fatal("note mode wrote a real digit")
	}
	if a.notes[r][c]&(1<<3) == 0 {
		t.Fatal("pencil mark 3 not recorded")
	}
}

func firstEmpty(a *app) (int, int) {
	for r := 0; r < game.N; r++ {
		for c := 0; c < game.N; c++ {
			if !a.puzzle.Given[r][c] {
				return r, c
			}
		}
	}
	return 0, 0
}

// --- Quit (Ny + Back) and rules ---------------------------------------------

func TestPlaySudokuQuitAndRules(t *testing.T) {
	h, a := bootToMenu(t)
	startDiff(t, h, a, game.Medium)

	tapAction(h, a, 3) // "Ny" -> menu
	if a.screen != screenMenu {
		t.Fatalf("Ny did not return to menu, screen=%v", a.screen)
	}

	startDiff(t, h, a, game.Medium)
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not return to menu, screen=%v", a.screen)
	}

	h.TapRect(a.menuRules)
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("Klar?"); !ok {
		t.Fatalf("rules text missing; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave rules, screen=%v", a.screen)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlaySudokuScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := newApp()
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if e := h.Screenshot(dir + "/sudoku_splash.png"); e != nil {
		t.Fatal(e)
	}
	h.TapXY(500, 700)
	_ = h.Screenshot(dir + "/sudoku_menu.png")
	h.TapRect(a.menuRules)
	if a.screen == screenRules {
		_ = h.Screenshot(dir + "/sudoku_rules.png")
		h.Back()
	}
	startDiff(t, h, a, game.Easy)
	// Partially fill the first four rows: an in-progress board.
	for r := 0; r < 4; r++ {
		for c := 0; c < game.N; c++ {
			if a.puzzle.Given[r][c] {
				continue
			}
			selectCell(h, a, r, c)
			tapNum(h, a, a.puzzle.Solution[r][c])
		}
	}
	_ = h.Screenshot(dir + "/sudoku_board.png")
	// Complete and confirm to win.
	fillSolution(h, a)
	tapAction(h, a, 2) // Klar?
	if err := h.Screenshot(dir + "/sudoku_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
