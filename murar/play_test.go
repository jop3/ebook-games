//go:build playtest

package main

// Headless PLAYTHROUGH tests for Murar (Quoridor). They drive the real touch
// path and check the gameplay against the rules as written (see
// rulesParagraphs in ui.go): a pawn steps one cell orthogonally; the
// straight-jump and diagonal-jump-exception rules when the opponent is
// adjacent; wall placement via the Bygg mur / Flytta pjäs toggle (preview on
// first tap, confirm on a second tap at the same spot, Rotera to flip
// orientation); illegal wall placements are rejected outright; reaching the
// opposite edge wins. Covers hotseat, all 3 AI difficulties, quitting,
// restarting, and the rules screen. Runs under the pure-Go inkview emulator
// (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"murar/game"
)

// --- helpers -----------------------------------------------------------

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

// startOpponent picks the first menu row for the given opponent (and, for the
// AI, the given search depth), taps it, and enters the game.
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

// tapMove drives a pawn move through the real UI by tapping the destination
// cell (Murar has only one pawn per side, so there is no separate "select"
// step — the destination cells are shown highlighted directly).
func tapMove(h *ink.Harness, a *app, to image.Point) bool {
	return h.TapRect(a.layout.CellToScreen(to.X, to.Y))
}

// buildWall drives a full wall placement through the real UI: switches to
// build mode (if not already there), taps the intersection to preview,
// rotates if the requested orientation differs from the preview's, then taps
// again to confirm. Returns whether the confirm tap succeeded.
func buildWall(h *ink.Harness, a *app, w game.Wall) bool {
	if !a.buildMode {
		if !tapButton(h, a, "Bygg mur") {
			return false
		}
	}
	r := a.layout.IntersectionRect(w.X, w.Y)
	if !h.TapRect(r) { // preview
		return false
	}
	if a.pendingWall == nil || a.pendingWall.X != w.X || a.pendingWall.Y != w.Y {
		return false
	}
	if a.pendingWall.Orient != w.Orient {
		if !tapButton(h, a, "Rotera") {
			return false
		}
	}
	return h.TapRect(r) // confirm
}

func tapButton(h *ink.Harness, a *app, label string) bool {
	for _, b := range a.buttons {
		if b.Label == label {
			return h.TapRect(b.Rect)
		}
	}
	return false
}

// firstLegalMove returns side's first legal pawn destination, used as a
// simple deterministic policy for the opponent's turns in tests that don't
// care about that side's own path (only that hotseat play advances legally).
func firstLegalMove(b *game.Board, side game.Side) (image.Point, bool) {
	moves := game.LegalPawnMoves(b, side)
	if len(moves) == 0 {
		return image.Point{}, false
	}
	return moves[0], true
}

// towardGoalMove picks, from side's legal pawn moves, whichever minimizes
// side's own BFS shortest-path distance to its goal row after the move — an
// independent (not AI-heuristic-derived, just the rules' own pathfinder),
// deterministic "always advance toward the goal" policy for full-game tests
// against the real AI.
func towardGoalMove(b *game.Board, side game.Side) (image.Point, bool) {
	moves := game.LegalPawnMoves(b, side)
	if len(moves) == 0 {
		return image.Point{}, false
	}
	best := moves[0]
	bestD := 1 << 30
	for _, m := range moves {
		nb := *b
		nb.Pawns[side] = m
		d, ok := game.BFSDistance(&nb, m, game.GoalRow(side))
		if !ok {
			d = 1 << 30
		}
		if d < bestD {
			bestD, best = d, m
		}
	}
	return best, true
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- RULE: basic pawn movement, illegal rejection ---------------------------

func TestPlayMurarMoveRules(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	if a.gs.Turn != game.P1 {
		t.Fatal("P1 (Svart) should move first")
	}

	// Tapping a non-adjacent cell must be rejected.
	before := a.gs.Board.Pawns[game.P1]
	if tapMove(h, a, image.Pt(4, 5)) { // 3 cells away, straight-line only 1 step allowed
		t.Fatal("a 3-cell jump must not be legal without the opponent adjacent")
	}
	if a.gs.Board.Pawns[game.P1] != before {
		t.Fatal("an illegal destination must not move the pawn")
	}
	if a.gs.Turn != game.P1 {
		t.Fatal("an illegal destination must not change the turn")
	}

	// A legal one-step forward move.
	if !tapMove(h, a, image.Pt(4, 7)) {
		t.Fatal("stepping forward one cell should be legal")
	}
	if a.gs.Board.Pawns[game.P1] != image.Pt(4, 7) {
		t.Fatalf("P1 pawn = %v, want (4,7)", a.gs.Board.Pawns[game.P1])
	}
	if a.gs.Turn != game.P2 {
		t.Fatal("turn should pass to P2 after a legal move")
	}
}

// --- GOTCHA: straight jump over an adjacent opponent, via a real tap -------

func TestPlayMurarStraightJump(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	a.gs.Board.Pawns[game.P1] = image.Pt(4, 4)
	a.gs.Board.Pawns[game.P2] = image.Pt(4, 3)
	a.gs.Turn = game.P1
	h.Draw()

	if !tapMove(h, a, image.Pt(4, 2)) {
		t.Fatal("jumping straight over the adjacent opponent should be legal")
	}
	if a.gs.Board.Pawns[game.P1] != image.Pt(4, 2) {
		t.Fatalf("P1 should have landed at (4,2), got %v", a.gs.Board.Pawns[game.P1])
	}
}

// --- GOTCHA: diagonal jump when the straight landing is wall-blocked -------

func TestPlayMurarDiagonalJumpWallBlocked(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	a.gs.Board.Pawns[game.P1] = image.Pt(4, 4)
	a.gs.Board.Pawns[game.P2] = image.Pt(4, 3)
	a.gs.Board.WallH[2][4] = true // blocks the (4,2)-(4,3) straight-jump landing
	a.gs.Turn = game.P1
	h.Draw()

	if tapMove(h, a, image.Pt(4, 2)) {
		t.Fatal("the straight jump must be unavailable when its landing is wall-blocked")
	}
	if !tapMove(h, a, image.Pt(3, 3)) {
		t.Fatal("the diagonal jump exception should be legal instead")
	}
	if a.gs.Board.Pawns[game.P1] != image.Pt(3, 3) {
		t.Fatalf("P1 should have landed diagonally at (3,3), got %v", a.gs.Board.Pawns[game.P1])
	}
}

// --- GOTCHA: diagonal jump when the straight landing is off-board ----------

func TestPlayMurarDiagonalJumpOffBoard(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	a.gs.Board.Pawns[game.P1] = image.Pt(4, 1)
	a.gs.Board.Pawns[game.P2] = image.Pt(4, 0) // against the far edge: straight jump would fall off-board
	a.gs.Turn = game.P1
	h.Draw()

	if !tapMove(h, a, image.Pt(5, 0)) {
		t.Fatal("the diagonal jump exception should be legal when the straight landing is off-board")
	}
	if a.gs.Board.Pawns[game.P1] != image.Pt(5, 0) {
		t.Fatalf("P1 should have landed diagonally at (5,0), got %v", a.gs.Board.Pawns[game.P1])
	}
}

// --- Wall placement via the real UI: preview, confirm, reject --------------

func TestPlayMurarWallPlacementUI(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	if !tapButton(h, a, "Bygg mur") {
		t.Fatal("no Bygg mur button on the game screen")
	}
	if !a.buildMode {
		t.Fatal("Bygg mur should switch into wall-build mode")
	}

	before := a.gs.Board.WallsLeft[game.P1]
	w := game.Wall{X: 0, Y: 0, Orient: game.Horizontal}
	r := a.layout.IntersectionRect(w.X, w.Y)

	if !h.TapRect(r) { // preview
		t.Fatal("tapping an intersection in build mode should be accepted (previews a wall)")
	}
	if a.pendingWall == nil || *a.pendingWall != w {
		t.Fatalf("pendingWall = %v, want a preview of %v", a.pendingWall, w)
	}
	if a.gs.Board.WallH[0][0] {
		t.Fatal("a preview must not place the wall yet")
	}

	if !h.TapRect(r) { // confirm
		t.Fatal("tapping the same intersection again should confirm the wall")
	}
	if !a.gs.Board.WallH[0][0] {
		t.Fatal("the wall should now be placed on the board")
	}
	if a.gs.Board.WallsLeft[game.P1] != before-1 {
		t.Fatalf("WallsLeft[P1] = %d, want %d", a.gs.Board.WallsLeft[game.P1], before-1)
	}
	if a.gs.Turn != game.P2 {
		t.Fatal("turn should pass to P2 after a wall placement")
	}
	if a.pendingWall != nil {
		t.Fatal("pendingWall should be cleared after a successful confirm")
	}
}

func TestPlayMurarWallRotate(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	tapButton(h, a, "Bygg mur")

	r := a.layout.IntersectionRect(2, 2)
	h.TapRect(r) // preview, default Horizontal
	if a.pendingWall.Orient != game.Horizontal {
		t.Fatalf("first preview should default to Horizontal, got %v", a.pendingWall.Orient)
	}
	if !tapButton(h, a, "Rotera") {
		t.Fatal("no Rotera button while a wall is previewed")
	}
	if a.pendingWall.Orient != game.Vertical {
		t.Fatal("Rotera should flip the preview to Vertical")
	}
	if !h.TapRect(r) { // confirm as Vertical
		t.Fatal("confirming the rotated preview should succeed")
	}
	if !a.gs.Board.WallV[2][2] {
		t.Fatal("the wall should have been placed Vertical, matching the rotated preview")
	}
	if a.gs.Board.WallH[2][2] {
		t.Fatal("the wall must not also be placed Horizontal")
	}
}

func TestPlayMurarIllegalWallRejectedWithoutConfirming(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	tapButton(h, a, "Bygg mur")

	// Place one wall for real.
	first := a.layout.IntersectionRect(3, 3)
	h.TapRect(first)
	h.TapRect(first)
	if !a.gs.Board.WallH[3][3] {
		t.Fatal("setup: the first wall should have been placed")
	}
	if a.gs.Turn != game.P2 {
		t.Fatal("setup: turn should now be P2")
	}

	// P2 now tries to place an overlapping wall at the adjacent slot: legal
	// as a preview (previewing never fails), illegal to confirm.
	second := a.layout.IntersectionRect(4, 3)
	h.TapRect(second) // preview
	before := a.gs.Board
	if !h.TapRect(second) { // confirm attempt: the tap itself is handled...
		t.Fatal("the confirm tap should be handled (even though the placement is rejected)")
	}
	if a.gs.Board != before {
		t.Fatal("an illegal (overlapping) wall must not change the board")
	}
	if !a.wallRejected {
		t.Fatal("the app should flag the failed confirm so the UI can show a rejection cue")
	}
	if a.gs.Turn != game.P2 {
		t.Fatal("a rejected wall placement must not advance the turn")
	}
}

// --- WIN: pure pawn movement, no walls at all -------------------------------

func TestPlayMurarPureMovementWin(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 60 {
			t.Fatal("game did not terminate")
		}
		var to image.Point
		var ok bool
		if a.gs.Turn == game.P1 {
			to, ok = towardGoalMove(&a.gs.Board, game.P1) // walks straight up
		} else {
			to, ok = firstLegalMove(&a.gs.Board, game.P2) // arbitrary legal shuffle
		}
		if !ok {
			t.Fatalf("side %v has no legal move at ply %d", a.gs.Turn, ply)
		}
		mover := a.gs.Turn
		if !tapMove(h, a, to) {
			t.Fatalf("legal move to %v for %v at ply %d was rejected", to, mover, ply)
		}
	}
	if w, ok := a.gs.Winner(); !ok || w != game.P1 {
		t.Fatalf("expected P1 to win by pure movement, got %v,%v", w, ok)
	}
	if _, ok := h.FindTextContains("Svart vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- WIN: a game that also exercises at least one wall placement -----------

func TestPlayMurarGameWithWallPlacementReachesWin(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	// P1's opening turn: build one wall (far from either pawn's straight
	// path down/up column 4) via the real build-mode UI, instead of moving.
	w := game.Wall{X: 0, Y: 0, Orient: game.Horizontal}
	if !buildWall(h, a, w) {
		t.Fatal("the opening wall placement should succeed")
	}
	if !a.gs.Board.WallH[0][0] {
		t.Fatal("the wall should be on the board")
	}
	if a.gs.Board.WallsLeft[game.P1] != game.StartingWalls-1 {
		t.Fatalf("WallsLeft[P1] = %d, want %d", a.gs.Board.WallsLeft[game.P1], game.StartingWalls-1)
	}
	if a.gs.Turn != game.P2 {
		t.Fatal("turn should pass to P2 after the wall")
	}

	// Switch back to move mode and play out a pure-movement finish.
	if !tapButton(h, a, "Flytta pjäs") {
		t.Fatal("no Flytta pjäs button to switch back to move mode")
	}
	if a.buildMode {
		t.Fatal("Flytta pjäs should switch back to move mode")
	}

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 60 {
			t.Fatal("game did not terminate")
		}
		var to image.Point
		var ok bool
		if a.gs.Turn == game.P1 {
			to, ok = towardGoalMove(&a.gs.Board, game.P1)
		} else {
			to, ok = firstLegalMove(&a.gs.Board, game.P2)
		}
		if !ok {
			t.Fatalf("side %v has no legal move at ply %d", a.gs.Turn, ply)
		}
		mover := a.gs.Turn
		if !tapMove(h, a, to) {
			t.Fatalf("legal move to %v for %v at ply %d was rejected", to, mover, ply)
		}
	}
	if w, ok := a.gs.Winner(); !ok || w != game.P1 {
		t.Fatalf("expected P1 to win, got %v,%v", w, ok)
	}
	if _, ok := h.FindTextContains("Svart vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- All 3 AI difficulties actually reply -----------------------------------

func TestPlayMurarAllDifficultiesReply(t *testing.T) {
	for _, depth := range []int{game.DepthEasy, game.DepthMedium, game.DepthHard} {
		depth := depth
		t.Run(itoa(depth), func(t *testing.T) {
			h, a := bootToMenu(t)
			startOpponent(t, h, a, game.OpponentAI, depth)
			if a.gs.AIDepth != depth {
				t.Fatalf("AIDepth = %d, want %d", a.gs.AIDepth, depth)
			}
			before := a.gs.Board
			to, ok := towardGoalMove(&a.gs.Board, game.P1)
			if !ok {
				t.Fatal("P1 should have an opening move")
			}
			if !tapMove(h, a, to) {
				t.Fatal("P1's opening move should be legal")
			}
			if a.gs.AITurn() {
				t.Fatal("control returned on the AI's turn (deferred reply not drained)")
			}
			if a.gs.Board == before {
				t.Fatal("P2 (the AI) did not reply")
			}
		})
	}
}

// --- Full game vs the AI, played to a real win/loss -------------------------

func TestPlayMurarFullGameVsAI(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 300 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		to, ok := towardGoalMove(&a.gs.Board, a.gs.Turn)
		if !ok {
			t.Fatalf("human to move but no legal move at ply %d", ply)
		}
		if !tapMove(h, a, to) {
			t.Fatalf("legal move to %v at ply %d was rejected", to, ply)
		}
	}
	want := "Vit vann!"
	if w, ok := a.gs.Winner(); ok && w == game.P1 {
		want = "Svart vann!"
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("end banner %q not shown; visible: %v", want, texts(h))
	}
}

// --- Input guards: no legal input after the game ends -----------------------

func TestPlayMurarNoInputAfterGameEnds(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	a.gs.Board.Pawns[game.P1] = image.Pt(4, 1)
	a.gs.Board.Pawns[game.P2] = image.Pt(0, 4)
	a.gs.Turn = game.P1
	h.Draw()
	if !tapMove(h, a, image.Pt(4, 0)) {
		t.Fatal("setup: the winning move should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("setup: game should be over")
	}

	if tapMove(h, a, image.Pt(4, 1)) {
		t.Fatal("no pawn move should be accepted once the game has ended")
	}
	if buildWall(h, a, game.Wall{X: 5, Y: 5, Orient: game.Horizontal}) {
		t.Fatal("no wall placement should be accepted once the game has ended")
	}
}

// --- Quit mid-game (Back key AND the Meny button), then restart ------------

func TestPlayMurarQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	to, _ := firstLegalMove(&a.gs.Board, game.P1)
	tapMove(h, a, to) // a move in progress

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}

	startOpponent(t, h, a, game.OpponentAI, game.DepthEasy)
	if !tapButton(h, a, "Meny") {
		t.Fatalf("no Meny button in game; visible: %v", texts(h))
	}
	if a.screen != screenMenu {
		t.Fatalf("Meny button did not return to menu, screen=%v", a.screen)
	}
	// Menu still usable afterwards.
	startOpponent(t, h, a, game.OpponentHotseat, 0)
}

// --- "Ny" restarts the current configuration mid-game -----------------------

func TestPlayMurarNyRestarts(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	to, _ := firstLegalMove(&a.gs.Board, game.P1)
	tapMove(h, a, to)
	if a.gs.Turn != game.P2 {
		t.Fatal("setup: expected a move to have been made")
	}
	if !tapButton(h, a, "Ny") {
		t.Fatalf("no Ny button in game; visible: %v", texts(h))
	}
	if a.gs.Turn != game.P1 {
		t.Fatal("Ny should reset to a fresh starting position")
	}
	if a.gs.Board.Pawns[game.P1] != image.Pt(game.Size/2, game.Size-1) {
		t.Fatal("Ny should reset P1's pawn to its starting cell")
	}
	if a.gs.Board.WallsLeft[game.P1] != game.StartingWalls {
		t.Fatal("Ny should reset wall counts")
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlayMurarRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("Diagonalt hopp"); !ok {
		t.Fatalf("rules text missing the diagonal jump exception; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayMurarScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/murar_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/murar_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/murar_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)
	for i := 0; i < 3 && a.gs.Phase == game.PhasePlaying; i++ {
		if a.gs.AITurn() {
			break
		}
		to, ok := towardGoalMove(&a.gs.Board, a.gs.Turn)
		if !ok {
			break
		}
		tapMove(h, a, to)
	}
	if err := h.Screenshot(dir + "/murar_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	tapButton(h, a, "Bygg mur")
	h.TapRect(a.layout.IntersectionRect(4, 4))
	if err := h.Screenshot(dir + "/murar_wall_preview.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	a.gs.Board.Pawns[game.P1] = image.Pt(4, 0)
	a.gs.Phase = game.PhaseDone
	h.Draw()
	if err := h.Screenshot(dir + "/murar_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
