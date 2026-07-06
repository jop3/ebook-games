//go:build playtest

package main

// Headless PLAYTHROUGH tests for Dominering. They drive the real touch path
// and check the gameplay against the rules as written (see rulesParagraphs
// in ui.go): V may only ever place a vertical domino, H only a horizontal
// one; the board never shrinks back (dominoes are never removed); whoever
// cannot legally place their domino on their turn loses outright (normal
// play convention, not "last move wins" applied as a shortcut); the
// ghost-preview tap flow (select an empty anchor cell, then its
// auto-highlighted partner cell) is how a placement is made. Covers both
// board sizes, all 3 AI difficulties, illegal-input rejection, quitting,
// restarting, and the rules screen. Runs under the pure-Go inkview emulator
// (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"dominering/game"
)

// --- helpers -----------------------------------------------------------------

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

// setSize taps the given board-size toggle button on the menu before
// starting a game.
func setSize(h *ink.Harness, size int) {
	label := "8x8 (Vanlig)"
	if size == game.SizeSmall {
		label = "6x6 (Lätt)"
	}
	h.TapText(label)
}

// startOpponent picks the first menu row for the given opponent (and, for
// the AI, the given search depth), taps it, and enters the game.
func startOpponent(t *testing.T, h *ink.Harness, a *app, opp game.Opponent, depth int) {
	t.Helper()
	for _, row := range a.menu.rows {
		if row.choice.opponent == opp && (opp == game.OpponentHotseat || row.choice.aiDepth == depth) {
			h.TapRect(row.rect)
			if a.screen != screenGame || a.gs == nil || a.gs.Opponent != opp {
				t.Fatalf("did not start opponent %v (screen=%v)", opp, a.screen)
			}
			return
		}
	}
	t.Fatalf("no menu row for opponent %v depth %d; visible: %v", opp, depth, texts(h))
}

// tapMove drives a full Dominering placement through the real UI: tap the
// anchor cell (A), then tap its partner cell (B) to confirm.
func tapMove(h *ink.Harness, a *app, m game.Move) bool {
	if !h.TapRect(a.layout.CellToScreen(m.A.X, m.A.Y)) {
		return false
	}
	return h.TapRect(a.layout.CellToScreen(m.B.X, m.B.Y))
}

func tapCell(h *ink.Harness, a *app, p image.Point) bool {
	return h.TapRect(a.layout.CellToScreen(p.X, p.Y))
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- RULE: fixed orientation — V is never allowed a horizontal placement,
// and vice versa ---------------------------------------------------------

func TestPlayDomineringFixedOrientationV(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	if a.gs.Turn != game.V {
		t.Fatal("V should move first")
	}

	// Select an interior cell: both its vertical partners should be
	// highlighted as candidates (via PartnersFrom), never a horizontal one.
	anchor := image.Pt(4, 4)
	if !tapCell(h, a, anchor) {
		t.Fatal("selecting an empty interior cell should succeed")
	}
	if !a.hasSelection || a.selected != anchor {
		t.Fatal("tapping an empty cell with legal partners should select it")
	}
	partners := a.gs.Board.PartnersFrom(game.V, anchor)
	for _, p := range partners {
		if p.X != anchor.X {
			t.Fatalf("V's highlighted partner %v is not in the same column as anchor %v", p, anchor)
		}
	}
	// Tapping a cell in the same row (not a legal V partner) must not place
	// a move; the board must remain unchanged and empty at both cells.
	sideCell := image.Pt(anchor.X+1, anchor.Y)
	before := a.gs.Board.EmptyCount()
	tapCell(h, a, sideCell)
	if a.gs.Board.EmptyCount() != before {
		t.Fatal("tapping a horizontally-adjacent cell must not place a domino for V")
	}
}

func TestPlayDomineringFixedOrientationH(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	// Get to H's turn: V plays any legal move first.
	legal := a.gs.Board.LegalMoves(game.V)
	if !tapMove(h, a, legal[0]) {
		t.Fatal("V's opening move should succeed")
	}
	if a.gs.Turn != game.H {
		t.Fatal("turn should now belong to H")
	}
	anchor := image.Pt(2, 2)
	if a.gs.Board.Empty(anchor.X, anchor.Y) {
		tapCell(h, a, anchor)
		partners := a.gs.Board.PartnersFrom(game.H, anchor)
		for _, p := range partners {
			if p.Y != anchor.Y {
				t.Fatalf("H's highlighted partner %v is not in the same row as anchor %v", p, anchor)
			}
		}
	}
}

// --- RULE: full placement via the real tap flow, cross-checked against a
// pure Apply -----------------------------------------------------------------

func TestPlayDomineringPlacementMatchesRules(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	legal := a.gs.Board.LegalMoves(game.V)
	if len(legal) == 0 {
		t.Fatal("V should have legal moves on an empty board")
	}
	m := legal[0]
	wantBoard := a.gs.Board.Apply(m)
	if !tapMove(h, a, m) {
		t.Fatalf("legal move %v via tap was rejected", m)
	}
	if a.gs.Board != wantBoard {
		t.Fatalf("UI move %v did not match the rules' own Apply result", m)
	}
	if a.gs.Turn != game.H {
		t.Fatal("turn did not pass to H after V's legal move")
	}
	if len(a.gs.Moves) != 1 || a.gs.Moves[0] != m {
		t.Fatalf("Moves history should record exactly the one placed domino, got %v", a.gs.Moves)
	}
}

// --- GOTCHA: tapping the same cell twice deselects, no move is made --------

func TestPlayDomineringTapSameCellDeselects(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	before := a.gs.Board.EmptyCount()
	anchor := image.Pt(3, 3)
	tapCell(h, a, anchor)
	if !a.hasSelection {
		t.Fatal("first tap on an empty, usable cell should select it")
	}
	tapCell(h, a, anchor)
	if a.hasSelection {
		t.Fatal("tapping the same cell again should deselect it")
	}
	if a.gs.Board.EmptyCount() != before {
		t.Fatal("a select+deselect must not place any domino")
	}
	if a.gs.Turn != game.V {
		t.Fatal("a select+deselect must not change the turn")
	}
}

// --- GOTCHA: illegal move rejected (occupied cell / out of bounds) --------

func TestPlayDomineringIllegalMoveRejected(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	legal := a.gs.Board.LegalMoves(game.V)
	m := legal[0]
	tapMove(h, a, m) // occupies m.A and m.B

	before := a.gs.Board.EmptyCount()
	turnBefore := a.gs.Turn
	// H's turn now; tap one of the now-occupied V cells — must be ignored.
	if tapCell(h, a, m.A) {
		if a.hasSelection && a.selected == m.A {
			t.Fatal("an occupied cell must never become selected")
		}
	}
	if a.gs.Board.EmptyCount() != before || a.gs.Turn != turnBefore {
		t.Fatal("tapping an occupied cell must not change board state or turn")
	}
	// Taps off the board are ignored entirely.
	tapCell(h, a, image.Pt(-1, -1))
	if a.gs.Board.EmptyCount() != before {
		t.Fatal("a tap off the board must be a no-op")
	}
}

// --- Both board sizes are selectable and produce the right board ----------

func TestPlayDomineringBoardSizes(t *testing.T) {
	for _, size := range []int{game.SizeStandard, game.SizeSmall} {
		h, a := bootToMenu(t)
		setSize(h, size)
		startOpponent(t, h, a, game.OpponentHotseat, 0)
		if a.gs.Board.Size != size {
			t.Fatalf("board size = %d, want %d", a.gs.Board.Size, size)
		}
		if a.gs.Board.EmptyCount() != size*size {
			t.Fatalf("fresh board should be fully empty, got %d empty of %d", a.gs.Board.EmptyCount(), size*size)
		}
	}
}

// --- WIN: normal play — a constructed "can't move" position ends the game -

func TestPlayDomineringWinBanner(t *testing.T) {
	h, a := bootToMenu(t)
	setSize(h, game.SizeSmall)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	// Build a 6x6 board where every column except column 0 is fully
	// occupied, and column 0 itself is occupied down to the last two cells:
	// H (horizontal-only) already has zero legal placements anywhere; V has
	// exactly one legal move left, filling the tail of column 0. Playing
	// that real move through the UI should end the game with H (to move
	// next) stuck, and V declared the winner — the actual Play() terminal
	// check runs, not a hand-set Phase/Winner.
	size := a.gs.Board.Size
	rows := make([][]bool, size)
	for y := range rows {
		row := make([]bool, size)
		for x := 1; x < size; x++ {
			row[x] = true
		}
		if y < size-2 {
			row[0] = true // column 0 occupied except its last two cells
		}
		rows[y] = row
	}
	a.gs.Board = game.BoardFromRows(rows)
	a.gs.Turn = game.V
	h.Draw()

	finalMove := game.Move{A: image.Pt(0, size-2), B: image.Pt(0, size-1)}
	if !tapMove(h, a, finalMove) {
		t.Fatal("V's final move filling the last open column should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once H (to move) has no legal placement left")
	}
	if a.gs.Winner() != game.V {
		t.Fatalf("Winner() = %v, want V", a.gs.Winner())
	}
	if _, ok := h.FindTextContains("Vertikal vinner"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
	// No further input should be accepted once the game is over.
	before := a.gs.Board.EmptyCount()
	tapCell(h, a, image.Pt(1, 1))
	if a.gs.Board.EmptyCount() != before {
		t.Fatal("no domino should be placeable after the game has ended")
	}
}

// --- All 3 AI difficulties actually reply -----------------------------------

func TestPlayDomineringAllDifficultiesReply(t *testing.T) {
	for _, depth := range []int{game.DepthEasy, game.DepthMedium, game.DepthHard} {
		depth := depth
		t.Run(itoa(depth), func(t *testing.T) {
			h, a := bootToMenu(t)
			setSize(h, game.SizeSmall) // keep the AI's search fast in tests
			startOpponent(t, h, a, game.OpponentAI, depth)
			if a.gs.AIDepth != depth {
				t.Fatalf("AIDepth = %d, want %d", a.gs.AIDepth, depth)
			}
			legal := a.gs.Board.LegalMoves(game.V)
			before := a.gs.Board
			if !tapMove(h, a, legal[0]) {
				t.Fatal("V's opening move should be legal")
			}
			if a.gs.AITurn() {
				t.Fatal("control returned on the AI's turn (deferred reply not drained)")
			}
			if a.gs.Board == before {
				t.Fatal("H (the AI) did not reply")
			}
		})
	}
}

// --- Full game vs the AI, played to a real win/loss -------------------------

func TestPlayDomineringFullGameVsAI(t *testing.T) {
	h, a := bootToMenu(t)
	setSize(h, game.SizeSmall)
	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 200 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		moves := a.gs.Board.LegalMoves(a.gs.Turn)
		if len(moves) == 0 {
			t.Fatalf("human to move but no legal move at ply %d, yet Phase is still Playing", ply)
		}
		if !tapMove(h, a, moves[0]) {
			t.Fatalf("legal move %v at ply %d was rejected", moves[0], ply)
		}
	}
	var want string
	switch a.gs.Winner() {
	case game.V:
		want = "Vertikal vinner"
	case game.H:
		want = "Horisontell vinner"
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("end banner %q not shown; visible: %v", want, texts(h))
	}
}

// --- Full hotseat game, both sides driven through taps ----------------------

func TestPlayDomineringFullHotseatGame(t *testing.T) {
	h, a := bootToMenu(t)
	setSize(h, game.SizeSmall)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 200 {
			t.Fatal("game did not terminate")
		}
		moves := a.gs.Board.LegalMoves(a.gs.Turn)
		if len(moves) == 0 {
			t.Fatalf("side to move (%v) has no legal move at ply %d, yet Phase is still Playing", a.gs.Turn, ply)
		}
		if !tapMove(h, a, moves[0]) {
			t.Fatalf("legal move %v at ply %d was rejected", moves[0], ply)
		}
	}
	if a.gs.Board.HasMove(a.gs.Turn) {
		t.Fatal("game ended but the side to move still has a legal move")
	}
}

// --- Quit mid-game (Back key AND the Meny button), then restart ------------

func TestPlayDomineringQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	m := a.gs.Board.LegalMoves(game.V)[0]
	tapMove(h, a, m) // a move in progress

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}

	startOpponent(t, h, a, game.OpponentAI, game.DepthEasy)
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
	startOpponent(t, h, a, game.OpponentHotseat, 0)
}

// --- "Ny" restarts the current configuration mid-game -----------------------

func TestPlayDomineringNyRestarts(t *testing.T) {
	h, a := bootToMenu(t)
	setSize(h, game.SizeSmall)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	m := a.gs.Board.LegalMoves(game.V)[0]
	tapMove(h, a, m)
	if a.gs.Turn != game.H {
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
	if a.gs.Turn != game.V || a.gs.Board.EmptyCount() != game.SizeSmall*game.SizeSmall {
		t.Fatal("Ny should reset to a fresh, empty starting position")
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlayDomineringRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("normalt spel"); !ok {
		t.Fatalf("rules text missing the normal-play convention explanation; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayDomineringScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/dominering_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/dominering_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/dominering_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)
	for i := 0; i < 4 && a.gs.Phase == game.PhasePlaying; i++ {
		if a.gs.AITurn() {
			break
		}
		moves := a.gs.Board.LegalMoves(a.gs.Turn)
		if len(moves) == 0 {
			break
		}
		tapMove(h, a, moves[0])
	}
	if err := h.Screenshot(dir + "/dominering_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
