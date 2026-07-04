//go:build playtest

package main

// Headless PLAYTHROUGH tests for Hex. They drive the real touch path and check
// the gameplay against the rules as written (see rulesParagraphs in ui.go):
// players alternate placing one stone on any empty cell; stones never move;
// Black wins by connecting top<->bottom, White by connecting left<->right; solo
// is human=Black vs AI=White. Both colours are driven to a real win, and a full
// game is played against the AI. Runs under the pure-Go inkview emulator
// (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"hex/game"
)

const (
	labelHotseat7 = "7×7 – 2 spelare"
	labelAI7      = "7×7 – mot dator"
)

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

func start(t *testing.T, h *ink.Harness, a *app, label string) {
	t.Helper()
	if err := h.TapText(label); err != nil {
		t.Fatalf("could not start %q: %v", label, err)
	}
	if a.screen != screenGame || a.gs == nil {
		t.Fatalf("did not enter game for %q, screen=%v", label, a.screen)
	}
}

func tapCell(h *ink.Harness, a *app, x, y int) {
	h.Tap(a.layout.Center(x, y))
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- RULE: place on empty only; stones don't move; turn alternates ----------

func TestPlayHexMoveRules(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, labelHotseat7)

	if a.gs.Turn != game.Black {
		t.Fatal("Black should move first")
	}
	tapCell(h, a, 3, 3)
	if a.gs.Board.At(3, 3) != game.Black {
		t.Fatal("Black stone not placed")
	}
	if a.gs.Turn != game.White {
		t.Fatal("turn did not pass to White")
	}

	// Tapping the occupied cell is rejected: owner and turn unchanged.
	tapCell(h, a, 3, 3)
	if a.gs.Board.At(3, 3) != game.Black || a.gs.Turn != game.White {
		t.Fatal("re-tapping an occupied cell changed state")
	}

	tapCell(h, a, 4, 3)
	if a.gs.Board.At(4, 3) != game.White || a.gs.Turn != game.Black {
		t.Fatal("White move did not register / alternate")
	}
}

// --- WIN: Black connects top<->bottom (and White stays harmless) ------------

func TestPlayHexBlackConnects(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, labelHotseat7)
	n := a.gs.Board.N

	// Black hero fills column x=3 (top to bottom); White filler stacks in column
	// x=0, which can never connect left<->right.
	var heroCol, fillCol []int // y values
	for y := 0; y < n; y++ {
		heroCol = append(heroCol, y)
		fillCol = append(fillCol, y)
	}
	hi, fi := 0, 0
	for a.gs.Phase == game.PhasePlaying && hi < n {
		if a.gs.Turn == game.Black {
			// Not yet connected until the column is complete.
			if hi < n-1 && a.gs.Board.Winner() != game.Empty {
				t.Fatalf("Black 'won' with only %d of %d column cells", hi, n)
			}
			tapCell(h, a, 3, heroCol[hi])
			hi++
		} else {
			tapCell(h, a, 0, fillCol[fi])
			fi++
		}
	}
	if a.gs.Win != game.Black || a.gs.Board.Winner() != game.Black {
		t.Fatalf("Black did not win by connecting the column (win=%v)", a.gs.Win)
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("phase not Done after a win")
	}
	if _, ok := h.FindTextContains("Svart vinner"); !ok {
		t.Fatalf("black-win banner missing; visible: %v", texts(h))
	}
}

// --- WIN: White connects left<->right (the other side) ----------------------

func TestPlayHexWhiteConnects(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, labelHotseat7)
	n := a.gs.Board.N

	// White hero fills row y=3 (left to right). Black (moves first) fills row y=0.
	hi, fi := 0, 0
	for a.gs.Phase == game.PhasePlaying && hi < n {
		if a.gs.Turn == game.White {
			tapCell(h, a, hi, 3)
			hi++
		} else {
			tapCell(h, a, fi, 0)
			fi++
		}
	}
	if a.gs.Win != game.White || a.gs.Board.Winner() != game.White {
		t.Fatalf("White did not win by connecting the row (win=%v)", a.gs.Win)
	}
	if _, ok := h.FindTextContains("Vit vinner"); !ok {
		t.Fatalf("white-win banner missing; visible: %v", texts(h))
	}
}

// --- Full game vs the AI (real terminal state; deferred AI reply) -----------

func TestPlayHexVsAI(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, labelAI7)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 100 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		mv, ok := game.BestMove(a.gs.Board, game.Black)
		if !ok {
			t.Fatal("no move for Black though game not over")
		}
		tapCell(h, a, mv[0], mv[1])
	}
	if a.gs.Win == game.Empty || a.gs.Board.Winner() != a.gs.Win {
		t.Fatalf("inconsistent end: win=%v boardWinner=%v", a.gs.Win, a.gs.Board.Winner())
	}
	// Hex can never end in a draw; and the AI (White) must have actually moved.
	whites := 0
	for y := 0; y < a.gs.Board.N; y++ {
		for x := 0; x < a.gs.Board.N; x++ {
			if a.gs.Board.At(x, y) == game.White {
				whites++
			}
		}
	}
	if whites == 0 {
		t.Fatal("AI never moved")
	}
}

// --- Quit mid-game and restart ----------------------------------------------

func TestPlayHexQuitAndRestart(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, labelHotseat7)
	tapCell(h, a, 2, 2)

	if err := h.TapText("Meny"); err != nil {
		t.Fatalf("no Meny button: %v", err)
	}
	if a.screen != screenMenu {
		t.Fatalf("Meny did not return to menu, screen=%v", a.screen)
	}

	start(t, h, a, labelHotseat7)
	tapCell(h, a, 2, 2)
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not return to menu, screen=%v", a.screen)
	}
}

// --- Rules screen -----------------------------------------------------------

func TestPlayHexRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("obruten kedja"); !ok {
		t.Fatalf("rules text missing the goal; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave rules, screen=%v", a.screen)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayHexScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	h, a := bootToMenu(t)
	start(t, h, a, labelHotseat7)
	n := a.gs.Board.N
	hi, fi := 0, 0
	for a.gs.Phase == game.PhasePlaying && hi < n {
		if a.gs.Turn == game.Black {
			tapCell(h, a, 3, hi)
			hi++
		} else {
			tapCell(h, a, 0, fi)
			fi++
		}
	}
	if err := h.Screenshot(dir + "/hex_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
