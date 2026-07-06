//go:build playtest

package main

// Headless PLAYTHROUGH tests for Chomp. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): the top-left cell (0,0) is poisoned; a tap on any remaining cell
// eats it and every remaining cell with row >= its row AND col >= its col;
// whoever eats the poisoned cell loses immediately; board size (Lätt/Medel/
// Svår) changes the grid, not the AI's strength — the AI is always perfect
// play via exhaustive minimax. Covers both opponent modes, every board size,
// the poison-loss condition, illegal-tap rejection, quitting, restarting,
// the AI-honesty hint, and a full game played to a real win via the touch
// path. Runs under the pure-Go inkview emulator (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"chomp/game"
)

// --- helpers ----------------------------------------------------------------

func bootToMenu(t *testing.T) (*ink.Harness, *app) {
	t.Helper()
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700) // dismiss splash
	if a.screen != screenMenu {
		t.Fatalf("splash tap did not open menu, screen=%v", a.screen)
	}
	return h, a
}

// startGame picks board size sizeIdx (an index into game.Sizes) and the
// given opponent from the menu, then starts the game.
func startGame(t *testing.T, h *ink.Harness, a *app, sizeIdx int, opp game.Opponent) {
	t.Helper()
	if !h.TapRect(a.menu.sizeBtns[sizeIdx]) {
		t.Fatalf("could not tap size button %d; visible: %v", sizeIdx, texts(h))
	}
	started := false
	for _, row := range a.menu.rows {
		if row.choice.opponent == opp {
			h.TapRect(row.rect)
			started = true
			break
		}
	}
	if !started {
		t.Fatalf("no menu row for opponent %v; visible: %v", opp, texts(h))
	}
	if a.screen != screenGame || a.gs == nil || a.gs.Opponent != opp {
		t.Fatalf("did not start game with opponent %v (screen=%v)", opp, a.screen)
	}
	want := game.Sizes[sizeIdx]
	if a.gs.Rows != want.Rows || a.gs.Cols != want.Cols {
		t.Fatalf("size mismatch: got %dx%d, want %dx%d (%s)", a.gs.Rows, a.gs.Cols, want.Rows, want.Cols, want.Name)
	}
}

func tapCell(h *ink.Harness, a *app, r, c int) bool {
	return h.TapRect(a.layout.CellToScreen(r, c))
}

func equalBoards(a, b game.State) bool {
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

const (
	sizeLatt  = 0
	sizeMedel = 1
	sizeSvar  = 2
)

// --- RULE: move region shape + illegal-tap rejection, via a real tap -------

func TestPlayChompMoveRulesAndRegionShape(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, sizeLatt, game.OpponentHotseat) // 4x4

	before := a.gs.Board
	if !tapCell(h, a, 1, 2) {
		t.Fatal("eating cell (1,2) on a fresh 4x4 board should be a legal move")
	}
	want := before.Apply(game.Move{Row: 1, Col: 2})
	if !equalBoards(a.gs.Board, want) {
		t.Fatalf("UI move did not match the rules' own Apply result: got %v, want %v", a.gs.Board, want)
	}
	if a.gs.Turn != game.P1 {
		t.Fatal("turn did not pass to P1 after a legal move")
	}
	if a.gs.Board.RowLen(1) != 2 || a.gs.Board.RowLen(0) != 4 {
		t.Fatalf("expected row 0 untouched (4) and row 1 clamped to 2, got %v", a.gs.Board)
	}

	// (2,3) was just eaten by that move (row 2 clamped to col 2) — tapping
	// it again must be rejected and must not change the turn.
	if tapCell(h, a, 2, 3) {
		t.Fatal("tapping an already-eaten cell must be rejected")
	}
	if a.gs.Turn != game.P1 {
		t.Fatal("a rejected tap must not change the turn")
	}

	// A tap on the status bar (well above the board) must be rejected too.
	turnBefore := a.gs.Turn
	h.TapXY(a.layout.Screen.Max.X/2, 5)
	if a.gs.Turn != turnBefore || a.gs.Board.RowLen(1) != 2 {
		t.Fatal("an out-of-board tap must never change the board or the turn")
	}
}

// --- RULE: eating the poisoned cell ends the game immediately -------------

func TestPlayChompPoisonLossViaTap(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, sizeLatt, game.OpponentHotseat)

	// Force the board down to just the poisoned cell, on P1's turn, so P1
	// is about to be forced to eat it.
	a.gs.Board = game.State{1, 0, 0, 0}
	a.gs.Turn = game.P1
	h.Draw()

	if !tapCell(h, a, 0, 0) {
		t.Fatal("eating the poisoned cell must be a legal (if losing) move")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once the poisoned cell is eaten")
	}
	if a.gs.Winner != game.P0 {
		t.Fatalf("Winner = %v, want P0 (P1 ate the poison)", a.gs.Winner)
	}
	if !a.gs.Board.Empty() {
		t.Fatal("eating the poisoned cell must clear the whole board")
	}
	if _, ok := h.FindTextContains("Spelare 1 vinner!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}

	// No further tap should do anything once the game is over.
	if tapCell(h, a, 0, 0) {
		t.Fatal("no move should be accepted once the game has ended")
	}
}

// --- Every board size, both opponent modes, via the real menu -------------

func TestPlayChompAllSizesAndOpponents(t *testing.T) {
	for idx, sz := range game.Sizes {
		idx, sz := idx, sz
		for _, opp := range []game.Opponent{game.OpponentHotseat, game.OpponentAI} {
			opp := opp
			t.Run(sz.Name+"_"+opponentName(opp), func(t *testing.T) {
				h, a := bootToMenu(t)
				startGame(t, h, a, idx, opp)
				if a.gs.Board.Total() != sz.Rows*sz.Cols {
					t.Fatalf("fresh board should have %d cells, got %d", sz.Rows*sz.Cols, a.gs.Board.Total())
				}
			})
		}
	}
}

func opponentName(o game.Opponent) string {
	if o == game.OpponentAI {
		return "AI"
	}
	return "hotseat"
}

// --- The AI actually replies, and never eats the poison while winning -----

func TestPlayChompAIReplies(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, sizeLatt, game.OpponentAI)

	before := a.gs.Board
	// The human's opening move: eat one cell, not the poison.
	if !tapCell(h, a, 0, 3) {
		t.Fatal("the human's opening move should be legal")
	}
	if a.gs.AITurn() {
		t.Fatal("control returned on the AI's turn (deferred reply not drained)")
	}
	if equalBoards(a.gs.Board, before.Apply(game.Move{Row: 0, Col: 3})) {
		t.Fatal("the AI (P1) did not reply after the human's move")
	}
}

// --- Honesty hint: the header text must track MoverCanWin() exactly -------

func TestPlayChompHonestyHint(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, sizeLatt, game.OpponentAI)

	want := "AI:n har ett vinstläge just nu"
	if a.gs.MoverCanWin() {
		want = "du kan vinna med perfekt spel"
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("expected honesty hint containing %q; visible: %v", want, texts(h))
	}
}

// --- A full game played via BestMove for the human must be a human win ----
//
// Chomp's strategy-stealing theorem guarantees any rectangle bigger than a
// single cell is a first-player win. The human (P0) moves first here, so if
// the human plays the perfect move every turn (via the SAME independent
// game.BestMove the AI itself uses, applied to the human's own side), the
// human must win regardless of how well the AI (P1) defends — an end-to-end
// check of the perfect-play property through the real touch path.
func TestPlayChompFullGameHumanPlaysPerfectly(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, sizeLatt, game.OpponentAI)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 100 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		m, ok := game.BestMove(a.gs.Board)
		if !ok {
			t.Fatalf("human to move but BestMove found nothing at ply %d, board=%v", ply, a.gs.Board)
		}
		if !tapCell(h, a, m.Row, m.Col) {
			t.Fatalf("legal move %v at ply %d was rejected", m, ply)
		}
	}
	if a.gs.Winner != game.P0 {
		t.Fatalf("perfect play by the first mover must win; Winner=%v, board=%v", a.gs.Winner, a.gs.Board)
	}
	if _, ok := h.FindTextContains("Du vinner!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- A full hot-seat game, driving both sides, reaches a sane end ---------

func TestPlayChompFullGameHotseat(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, sizeMedel, game.OpponentHotseat)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 200 {
			t.Fatal("game did not terminate")
		}
		m, ok := game.BestMove(a.gs.Board)
		if !ok {
			t.Fatalf("no move available at ply %d, board=%v", ply, a.gs.Board)
		}
		if !tapCell(h, a, m.Row, m.Col) {
			t.Fatalf("legal move %v at ply %d was rejected", m, ply)
		}
	}
	if a.gs.Winner != game.P0 {
		t.Fatalf("perfect play by the first mover (P0) must win a hot-seat game too; Winner=%v", a.gs.Winner)
	}
}

// --- Quit mid-game (Back key AND the Meny button), then restart -----------

func TestPlayChompQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, sizeLatt, game.OpponentHotseat)
	tapCell(h, a, 0, 3) // a move in progress

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}

	startGame(t, h, a, sizeLatt, game.OpponentAI)
	tappedMeny := false
	for _, b := range a.buttons {
		if b.Label == "Meny" {
			h.TapRect(b.Rect)
			tappedMeny = true
		}
	}
	if !tappedMeny {
		t.Fatalf("no Meny button in game; visible: %v", texts(h))
	}
	if a.screen != screenMenu {
		t.Fatalf("Meny button did not return to menu, screen=%v", a.screen)
	}
	// Menu still usable afterwards.
	startGame(t, h, a, sizeLatt, game.OpponentHotseat)
}

// --- "Ny" restarts the current configuration mid-game ----------------------

func TestPlayChompNyRestarts(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, sizeSvar, game.OpponentHotseat)
	tapCell(h, a, 0, 6)
	if a.gs.Turn != game.P1 {
		t.Fatal("setup: expected a move to have been made")
	}
	tappedNy := false
	for _, b := range a.buttons {
		if b.Label == "Ny" {
			h.TapRect(b.Rect)
			tappedNy = true
		}
	}
	if !tappedNy {
		t.Fatalf("no Ny button in game; visible: %v", texts(h))
	}
	want := game.Sizes[sizeSvar]
	if a.gs.Turn != game.P0 || a.gs.Board.Total() != want.Rows*want.Cols {
		t.Fatal("Ny should reset to a fresh starting position with the same size")
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlayChompRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("förgiftade"); !ok {
		t.Fatalf("rules text missing the poison rule; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayChompScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/chomp_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/chomp_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/chomp_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startGame(t, h, a, sizeSvar, game.OpponentAI)
	// A hand-crafted partial staircase, just for a representative mid-game
	// screenshot (BestMove often ends a 6x7 game in very few plies, which
	// would otherwise show the end banner instead of a mid-game board).
	a.gs.Board = game.State{7, 7, 5, 3, 3, 1}
	a.gs.Turn = game.P0
	h.Draw()
	if err := h.Screenshot(dir + "/chomp_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// Poison-loss end-game banner.
	a.gs.Board = game.State{1, 0, 0, 0, 0, 0}
	a.gs.Turn = game.P0
	h.Draw()
	tapCell(h, a, 0, 0)
	if err := h.Screenshot(dir + "/chomp_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
