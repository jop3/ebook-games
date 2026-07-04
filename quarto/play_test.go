//go:build playtest

package main

// Headless PLAYTHROUGH tests for Quarto. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): each turn is place-the-handed-piece THEN give-a-piece; your opponent
// chooses which piece you must place; you win by completing a row/column/
// diagonal of four pieces that share at least one attribute; a full board with
// no such line is a draw. Both players are driven to a win, and a full game is
// played against the AI. Runs under the pure-Go inkview emulator (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"quarto/game"
)

const (
	labelHotseat = "2 spelare (hot-seat)"
	labelAI      = "Mot dator – Lätt"
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

func start(t *testing.T, h *ink.Harness, a *app, label string) {
	t.Helper()
	if err := h.TapText(label); err != nil {
		t.Fatalf("could not start %q: %v", label, err)
	}
	if a.screen != screenGame || a.gs == nil {
		t.Fatalf("did not enter game for %q, screen=%v", label, a.screen)
	}
}

// give hands piece p to the opponent by tapping its pool button.
func give(t *testing.T, h *ink.Harness, a *app, p game.Piece) {
	t.Helper()
	if a.gs.Step != game.StepGive {
		t.Fatalf("give(%d) called but step is not StepGive", p)
	}
	for _, pb := range a.poolBtn {
		if pb.Piece == p {
			h.TapRect(pb.Rect)
			// The given piece must leave the pool. (We don't assert ActivePiece==p:
			// in AI mode the give immediately triggers the AI's place+give, which
			// re-sets ActivePiece to the AI's return gift.)
			for _, q := range a.gs.Pool {
				if q == p {
					t.Fatalf("gave piece %d but it is still in the pool", p)
				}
			}
			return
		}
	}
	t.Fatalf("piece %d not in the pool", p)
}

// place puts the handed piece on (x,y).
func place(t *testing.T, h *ink.Harness, a *app, x, y int) {
	t.Helper()
	if a.gs.Step != game.StepPlace {
		t.Fatalf("place called but step is not StepPlace")
	}
	h.TapRect(a.layout.CellToScreen(x, y))
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- RULE: place-then-give cadence; can't place before being handed a piece --

func TestPlayQuartoTurnStructure(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, labelHotseat)

	// Fresh game: player 0's first action is to GIVE; there is no piece to place.
	if a.gs.Turn != 0 || a.gs.Step != game.StepGive || a.gs.ActivePiece != game.NoPiece {
		t.Fatalf("bad opening state: turn=%d step=%v active=%d", a.gs.Turn, a.gs.Step, a.gs.ActivePiece)
	}
	// Tapping the board now does nothing (nothing to place).
	place2 := a.gs.Board
	h.TapRect(a.layout.CellToScreen(0, 0))
	if a.gs.Board != place2 {
		t.Fatal("placed a piece before one was handed over")
	}

	poolBefore := len(a.gs.Pool)
	give(t, h, a, game.Piece(1)) // P0 hands piece 1 to P1
	if a.gs.Turn != 1 || a.gs.Step != game.StepPlace {
		t.Fatalf("after give: turn=%d step=%v (want 1/place)", a.gs.Turn, a.gs.Step)
	}
	if len(a.gs.Pool) != poolBefore-1 {
		t.Fatal("given piece did not leave the pool")
	}

	place(t, h, a, 0, 0) // P1 places it
	if a.gs.Board.At(0, 0) != game.Piece(1) {
		t.Fatal("piece not placed at (0,0)")
	}
	if a.gs.Turn != 1 || a.gs.Step != game.StepGive {
		t.Fatalf("after place: turn=%d step=%v (want 1/give)", a.gs.Turn, a.gs.Step)
	}
	// Placing on the occupied cell is impossible now (it's the give step); and a
	// piece already handed over cannot be given again — it's gone from the pool.
	for _, pb := range a.poolBtn {
		if pb.Piece == game.Piece(1) {
			t.Fatal("a placed piece is still offered in the pool")
		}
	}
}

// buildLine drives a line of four all-"tall" pieces so a chosen player makes the
// winning 4th placement. wasteFirst inserts one throwaway placement so the line
// parity flips (letting player 1 land the winner).
func buildTallRowWin(t *testing.T, wantWinner int) (*ink.Harness, *app) {
	t.Helper()
	h, a := bootToMenu(t)
	start(t, h, a, labelHotseat)

	// Tall pieces (bit0 set): 1,3,5,7,9. All share AttrTall, so a full row of any
	// four of them is a winning line.
	if wantWinner == 1 {
		// Throwaway: P0 gives 1, P1 places it OFF the target row (0,3).
		give(t, h, a, game.Piece(1))
		place(t, h, a, 0, 3)
	}
	// Fill row y=0 left to right with tall pieces.
	tall := []game.Piece{3, 5, 7, 9}
	for i, x := range []int{0, 1, 2, 3} {
		give(t, h, a, tall[i])
		place(t, h, a, x, 0)
	}
	return h, a
}

// --- WIN for player 1 (banner "Spelare 1 vinner!") --------------------------

func TestPlayQuartoPlayer1Wins(t *testing.T) {
	h, a := buildTallRowWin(t, 0)
	if a.gs.Phase != game.PhaseWon || a.gs.Winner() != 0 {
		t.Fatalf("expected player 0 to win, phase=%v winner=%d", a.gs.Phase, a.gs.Winner())
	}
	if _, ok := h.FindTextContains("Spelare 1 vinner"); !ok {
		t.Fatalf("player-1 win banner missing; visible: %v", texts(h))
	}
}

// --- WIN for player 2 (the other side; banner "Spelare 2 vinner!") ----------

func TestPlayQuartoPlayer2Wins(t *testing.T) {
	h, a := buildTallRowWin(t, 1)
	if a.gs.Phase != game.PhaseWon || a.gs.Winner() != 1 {
		t.Fatalf("expected player 1 to win, phase=%v winner=%d", a.gs.Phase, a.gs.Winner())
	}
	if _, ok := h.FindTextContains("Spelare 2 vinner"); !ok {
		t.Fatalf("player-2 win banner missing; visible: %v", texts(h))
	}
}

// --- DRAW banner (end-state rendering) --------------------------------------

func TestPlayQuartoDrawBanner(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, labelHotseat)
	// Force the terminal draw state and confirm it renders. (A natural 16-piece
	// no-line board is extremely hard to reach by hand; the detection is unit
	// tested — here we check the end-state UI.)
	a.gs.Phase = game.PhaseDraw
	h.Draw()
	if _, ok := h.FindTextContains("Oavgjort"); !ok {
		t.Fatalf("draw banner missing; visible: %v", texts(h))
	}
	for _, want := range []string{"Spela igen", "Meny"} {
		if _, ok := h.FindText(want); !ok {
			t.Fatalf("finished game missing %q; visible: %v", want, texts(h))
		}
	}
}

// --- Full game vs the AI (two-action AI, real terminal state) ---------------

func TestPlayQuartoVsAI(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, labelAI)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 100 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred action not drained)")
		}
		switch a.gs.Step {
		case game.StepPlace:
			placed := false
			for y := 0; y < game.Size && !placed; y++ {
				for x := 0; x < game.Size && !placed; x++ {
					if a.gs.Board.Empty(x, y) {
						place(t, h, a, x, y)
						placed = true
					}
				}
			}
			if !placed {
				t.Fatal("no empty cell but game not over")
			}
		case game.StepGive:
			if len(a.gs.Pool) == 0 {
				t.Fatal("give step with an empty pool")
			}
			give(t, h, a, a.gs.Pool[0])
		}
	}
	if a.gs.Phase != game.PhaseWon && a.gs.Phase != game.PhaseDraw {
		t.Fatalf("game ended in unexpected phase %v", a.gs.Phase)
	}
	if a.gs.Phase == game.PhaseWon {
		// The winning line really does share an attribute.
		if !a.gs.Board.HasWin() {
			t.Fatal("PhaseWon but the board has no winning line")
		}
	}
}

// --- Quit mid-game and restart ----------------------------------------------

func TestPlayQuartoQuitAndRestart(t *testing.T) {
	h, a := bootToMenu(t)
	start(t, h, a, labelHotseat)
	give(t, h, a, game.Piece(2)) // a move in progress

	if err := h.TapText("Meny"); err != nil {
		t.Fatalf("no Meny button: %v", err)
	}
	if a.screen != screenMenu {
		t.Fatalf("Meny did not return to menu, screen=%v", a.screen)
	}

	start(t, h, a, labelHotseat)
	give(t, h, a, game.Piece(2))
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not return to menu, screen=%v", a.screen)
	}

	// Spela igen after a finished game.
	buildInto(t, h, a) // reach a won state
	if err := h.TapText("Spela igen"); err != nil {
		t.Fatalf("no Spela igen button: %v", err)
	}
	if a.gs.Phase != game.PhasePlaying || a.gs.Board.HasWin() {
		t.Fatalf("Spela igen did not reset (phase=%v)", a.gs.Phase)
	}
}

// buildInto plays the tall-row win into the already-started hotseat game a.
func buildInto(t *testing.T, h *ink.Harness, a *app) {
	t.Helper()
	start(t, h, a, labelHotseat)
	tall := []game.Piece{3, 5, 7, 9}
	for i, x := range []int{0, 1, 2, 3} {
		give(t, h, a, tall[i])
		place(t, h, a, x, 0)
	}
	if a.gs.Phase != game.PhaseWon {
		t.Fatalf("setup win failed, phase=%v", a.gs.Phase)
	}
}

// --- Rules screen -----------------------------------------------------------

func TestPlayQuartoRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("delar minst en egenskap"); !ok {
		t.Fatalf("rules text missing the win condition; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave rules, screen=%v", a.screen)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayQuartoScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	h, _ := buildTallRowWin(t, 0)
	if err := h.Screenshot(dir + "/quarto_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
