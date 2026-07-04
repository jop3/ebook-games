//go:build playtest

package main

// Headless PLAYTHROUGH tests for "I rad" (the X-in-a-row family). They drive the
// real touch path and check the gameplay against the rules as written (see
// ui.RulesParagraphs): place on any empty cell (or drop into a column in
// Connect-Four mode); N-in-a-row wins; a full board with no line is a draw;
// limited-piece variants switch to a moving phase where you slide a stone to an
// ADJACENT empty; 2–4 players rotate; solo is you vs the AI. Runs under the
// pure-Go inkview emulator (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"irad/game"
	"irad/ui"
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

// setMode taps the mode selector until it reads the wanted mode.
func setMode(t *testing.T, h *ink.Harness, a *app, want ui.Mode) {
	t.Helper()
	for i := 0; a.menu.Mode != want && i < 6; i++ {
		if err := h.TapTextContains("tryck för att byta"); err != nil {
			t.Fatalf("mode button not found: %v", err)
		}
	}
	if a.menu.Mode != want {
		t.Fatalf("could not select mode %v (stuck at %v)", want, a.menu.Mode)
	}
}

// startPreset picks mode + preset row i and enters the game.
func startPreset(t *testing.T, h *ink.Harness, a *app, mode ui.Mode, i int) {
	t.Helper()
	setMode(t, h, a, mode)
	rects := a.menu.PresetRowRects()
	if i < 0 || i >= len(rects) {
		t.Fatalf("preset index %d out of range (%d rows)", i, len(rects))
	}
	h.TapRect(rects[i])
	if a.screen != screenGame || a.gs == nil {
		t.Fatalf("did not enter game for preset %d, screen=%v", i, a.screen)
	}
}

// cellCenter taps the centre of board cell (x,y).
func tapCell(h *ink.Harness, a *app, x, y int) {
	h.TapRect(a.layout.CellToScreen(x, y))
}

// playMove performs move m through the UI: placements/drops are a single tap on
// the target; relocations tap the source then the destination.
func playMove(h *ink.Harness, a *app, m game.Move) {
	if m.From >= 0 {
		fx, fy := a.gs.Board.XY(m.From)
		tapCell(h, a, fx, fy)
	}
	tx, ty := a.gs.Board.XY(m.To)
	tapCell(h, a, tx, ty)
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- Tic-tac-toe: win, turn rotation, illegal-move rejection ----------------

func TestPlayIradTicTacToeWin(t *testing.T) {
	h, a := bootToMenu(t)
	startPreset(t, h, a, ui.Mode2, 0) // "Tre i rad" 3x3, win 3, free placement

	if a.gs.Turn != game.Player1 {
		t.Fatal("Player1 should start")
	}
	tapCell(h, a, 0, 0) // P1
	if a.gs.Board.Cells[a.gs.Board.Idx(0, 0)] != game.Player1 {
		t.Fatal("P1 stone not placed")
	}
	if a.gs.Turn != game.Player2 {
		t.Fatal("turn did not advance to Player2")
	}

	// Illegal: tapping the occupied cell does nothing and does not pass the turn.
	tapCell(h, a, 0, 0)
	if a.gs.Turn != game.Player2 || a.gs.Board.Cells[0] != game.Player1 {
		t.Fatal("tapping an occupied cell changed state")
	}

	tapCell(h, a, 1, 0) // P2
	tapCell(h, a, 0, 1) // P1
	tapCell(h, a, 1, 1) // P2
	tapCell(h, a, 0, 2) // P1 completes column x=0 -> win

	if a.gs.Winner != game.Player1 || a.gs.Phase != game.PhaseGameOver {
		t.Fatalf("Player1 should have won by a column (winner=%v phase=%v)", a.gs.Winner, a.gs.Phase)
	}
	if _, ok := h.FindTextContains("Spelare 1 (X) vann"); !ok {
		t.Fatalf("win banner missing; visible: %v", texts(h))
	}
}

func TestPlayIradTicTacToeDraw(t *testing.T) {
	h, a := bootToMenu(t)
	startPreset(t, h, a, ui.Mode2, 0)

	// A filled 3x3 with no three-in-a-row:
	//   X O X
	//   X O O
	//   O X X
	seq := [][2]int{
		{0, 0}, {1, 0}, {2, 0}, // X O X
		{1, 1}, {0, 1}, {2, 1}, // (X at 0,1; O at 1,1 and 2,1)
		{1, 2}, {0, 2}, {2, 2}, // X O ... X
	}
	for i, c := range seq {
		if a.gs.Phase == game.PhaseGameOver {
			t.Fatalf("game ended early after %d moves", i)
		}
		tapCell(h, a, c[0], c[1])
	}
	if a.gs.Phase != game.PhaseGameOver || a.gs.Winner != game.PlayerNone {
		t.Fatalf("expected a draw, got winner=%v phase=%v", a.gs.Winner, a.gs.Phase)
	}
	if _, ok := h.FindTextContains("Oavgjort"); !ok {
		t.Fatalf("draw banner missing; visible: %v", texts(h))
	}
}

// --- Connect Four: drop physics + vertical win ------------------------------

func TestPlayIradConnectFourDrop(t *testing.T) {
	h, a := bootToMenu(t)
	startPreset(t, h, a, ui.Mode2, 2) // "Fyra i rad" 7x6 drop, win 4
	b := &a.gs.Board
	floor := b.Height - 1

	// Tapping anywhere in column 0 drops to the floor.
	tapCell(h, a, 0, 0)
	if b.Cells[b.Idx(0, floor)] != game.Player1 {
		t.Fatalf("drop did not land on the floor of column 0")
	}
	// P1 stacks column 0; P2 stacks column 1. P1 reaches four first.
	for i := 0; i < 3; i++ {
		tapCell(h, a, 1, 0) // P2
		tapCell(h, a, 0, 0) // P1
	}
	if a.gs.Winner != game.Player1 || a.gs.Phase != game.PhaseGameOver {
		t.Fatalf("P1 should win a vertical four (winner=%v)", a.gs.Winner)
	}
	// The four P1 stones are stacked on the floor of column 0.
	for y := floor; y > floor-4; y-- {
		if b.Cells[b.Idx(0, y)] != game.Player1 {
			t.Fatalf("column 0 row %d not P1", y)
		}
	}
}

// --- Moving phase (Tre i kvarn): place-then-slide, adjacency rule, move-to-win

func TestPlayIradMovingPhase(t *testing.T) {
	h, a := bootToMenu(t)
	startPreset(t, h, a, ui.Mode2, 1) // "Tre i kvarn" 3x3, win 3, 3 pieces each
	b := &a.gs.Board

	// Placement phase: X and O each place three, avoiding any line.
	// X: (0,0),(0,1),(1,1)   O: (1,0),(2,0),(2,1)
	place := [][2]int{
		{0, 0}, {1, 0}, {0, 1}, {2, 0}, {1, 1}, {2, 1},
	}
	for _, c := range place {
		if a.gs.Phase != game.PhasePlacing {
			t.Fatalf("left placing phase early at %v", c)
		}
		tapCell(h, a, c[0], c[1])
	}
	if a.gs.Phase != game.PhaseMoving {
		t.Fatalf("did not enter moving phase after all pieces placed (phase=%v)", a.gs.Phase)
	}
	if a.gs.Turn != game.Player1 {
		t.Fatalf("expected Player1 to move first in moving phase, got %v", a.gs.Turn)
	}

	// RULE: a stone may only slide to an ADJACENT empty. Select X(0,0) then tap a
	// far empty (2,2): nothing happens.
	tapCell(h, a, 0, 0)
	if a.gs.Selected != b.Idx(0, 0) {
		t.Fatalf("selecting own stone failed (selected=%d)", a.gs.Selected)
	}
	snap := append([]game.Player(nil), b.Cells...)
	tapCell(h, a, 2, 2) // non-adjacent empty
	if !equalCells(b.Cells, snap) {
		t.Fatal("a non-adjacent slide was allowed")
	}

	// Slide X(1,1) -> (0,2), completing column x=0 -> win.
	tapCell(h, a, 1, 1) // switch selection to (1,1)
	if a.gs.Selected != b.Idx(1, 1) {
		t.Fatalf("did not switch selection to (1,1) (selected=%d)", a.gs.Selected)
	}
	tapCell(h, a, 0, 2) // adjacent empty, completes the column
	if a.gs.Winner != game.Player1 || a.gs.Phase != game.PhaseGameOver {
		t.Fatalf("slide-to-win failed (winner=%v phase=%v)", a.gs.Winner, a.gs.Phase)
	}
}

func equalCells(a, b []game.Player) bool {
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

// --- Three players rotate 1->2->3 and a real win ----------------------------

func TestPlayIradThreePlayerRotation(t *testing.T) {
	h, a := bootToMenu(t)
	startPreset(t, h, a, ui.Mode3, 0) // 3x3, three humans
	if a.gs.NumPlayers != 3 {
		t.Fatalf("expected 3 players, got %d", a.gs.NumPlayers)
	}

	// P1 builds column 0; P2 and P3 fill columns 1 and 2. Turn must cycle 1->2->3.
	want := []game.Player{game.Player1, game.Player2, game.Player3, game.Player1, game.Player2, game.Player3, game.Player1}
	cells := [][2]int{{0, 0}, {1, 0}, {2, 0}, {0, 1}, {1, 1}, {2, 1}, {0, 2}}
	for i, c := range cells {
		if a.gs.Turn != want[i] {
			t.Fatalf("move %d: turn %v, expected %v", i, a.gs.Turn, want[i])
		}
		tapCell(h, a, c[0], c[1])
	}
	if a.gs.Winner != game.Player1 || a.gs.Phase != game.PhaseGameOver {
		t.Fatalf("Player1 should win column 0 (winner=%v)", a.gs.Winner)
	}
}

// --- Vs AI: the AI replies, and must NOT lose to trivial play ---------------

func TestPlayIradVsAI(t *testing.T) {
	h, a := bootToMenu(t)
	startPreset(t, h, a, ui.ModeAI, 0) // "Tre i rad" 3x3 vs the AI
	if !a.gs.VsAI {
		t.Fatal("expected a vs-AI game")
	}

	// Human (Player1) plays the first legal move each turn — deliberately weak.
	for ply := 0; a.gs.Phase != game.PhaseGameOver; ply++ {
		if ply > 20 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (AI reply not applied)")
		}
		moves := a.gs.Board.ValidMoves(a.gs.Turn, a.gs.Phase)
		if len(moves) == 0 {
			t.Fatalf("human to move but no legal move at ply %d", ply)
		}
		playMove(h, a, moves[0])
	}
	// A competent tic-tac-toe AI must never lose to this trivial human.
	if a.gs.Winner == game.Player1 {
		t.Fatalf("the AI lost to first-empty play — likely an AI bug")
	}
	// The AI must actually have placed stones.
	if a.gs.Board.Placed[int(game.Player2)] == 0 {
		t.Fatal("AI never moved")
	}
}

// --- Quit (Byt variant + Back), restart (Spela igen), rules -----------------

func TestPlayIradQuitRestartRules(t *testing.T) {
	h, a := bootToMenu(t)
	startPreset(t, h, a, ui.Mode2, 0)
	tapCell(h, a, 0, 0)

	// "Byt variant" returns to the menu.
	if err := h.TapText("Byt variant"); err != nil {
		t.Fatalf("no Byt variant button: %v", err)
	}
	if a.screen != screenMenu {
		t.Fatalf("Byt variant did not return to menu, screen=%v", a.screen)
	}

	// Back key also quits to the menu.
	startPreset(t, h, a, ui.Mode2, 0)
	tapCell(h, a, 0, 0)
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not return to menu, screen=%v", a.screen)
	}

	// "Spela igen" restarts a finished game.
	startPreset(t, h, a, ui.Mode2, 0)
	for _, c := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}, {0, 2}} {
		tapCell(h, a, c[0], c[1]) // P1 wins column 0
	}
	if a.gs.Phase != game.PhaseGameOver {
		t.Fatal("setup game did not end")
	}
	if err := h.TapText("Spela igen"); err != nil {
		t.Fatalf("no Spela igen button: %v", err)
	}
	if a.gs.Phase != game.PhasePlacing || a.gs.Winner != game.PlayerNone {
		t.Fatalf("Spela igen did not reset the game (phase=%v winner=%v)", a.gs.Phase, a.gs.Winner)
	}

	// Rules screen.
	if err := h.TapText("Byt variant"); err != nil {
		t.Fatalf("back to menu: %v", err)
	}
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave rules, screen=%v", a.screen)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayIradScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	h, a := bootToMenu(t)
	startPreset(t, h, a, ui.Mode2, 0)
	for _, c := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}, {0, 2}} {
		tapCell(h, a, c[0], c[1])
	}
	if err := h.Screenshot(dir + "/irad_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
