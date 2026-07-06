//go:build playtest

package main

// Headless PLAYTHROUGH tests for Stadskärnan. They drive the real touch path
// (splash -> menu -> Cathedral placement -> piece tray selection/rotation ->
// board tap-to-place) and check the gameplay against the rules as written
// (see rulesParagraphs in ui.go): Black places the neutral Cathedral first,
// then White moves first; players alternate placing one of their 13
// buildings per turn in any rotation/reflection; a fully walled-off,
// non-edge-touching region captures trapped opponent pieces (returned to
// hand) or is simply sealed; a side with no legal placement is skipped; the
// game ends once neither side can place anything, and the side with fewer
// remaining squares in hand wins. Runs under the pure-Go inkview emulator
// (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"stadskarnan/game"
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

func startOpponent(t *testing.T, h *ink.Harness, a *app, opp game.Opponent) {
	t.Helper()
	row := 0
	if opp == game.OpponentAI {
		row = 1
	}
	if !h.TapRect(a.menu.rows[row]) {
		t.Fatalf("tap on menu row %d did not register", row)
	}
	if a.screen != screenGame || a.gs == nil || a.gs.Opponent != opp {
		t.Fatalf("did not start opponent %v (screen=%v)", opp, a.screen)
	}
}

// placeCathedral taps the given legal Cathedral anchor via the real board.
func placeCathedral(h *ink.Harness, a *app, anchor image.Point) bool {
	return h.TapRect(a.layout.CellToScreen(anchor.X, anchor.Y))
}

// selectTrayPiece taps piece id's tray slot.
func selectTrayPiece(h *ink.Harness, a *app, id int) bool {
	return h.TapRect(a.layout.TrayRects[id])
}

// rotateSelection taps the "Rotera" button once.
func rotateSelection(h *ink.Harness, a *app) bool {
	for _, b := range a.buttons {
		if b.Label == "Rotera" {
			return h.TapRect(b.Rect)
		}
	}
	return false
}

// tapBoard taps the board cell at (x,y) directly.
func tapBoard(h *ink.Harness, a *app, x, y int) bool {
	return h.TapRect(a.layout.CellToScreen(x, y))
}

// placePieceAtOrientation selects pieceID, rotates to orientIdx (via the real
// "Rotera" button, wrapping as needed), then taps anchor to commit it.
func placePieceAtOrientation(t *testing.T, h *ink.Harness, a *app, pieceID, orientIdx int, anchor image.Point) bool {
	t.Helper()
	if !selectTrayPiece(h, a, pieceID) {
		t.Fatalf("could not select tray piece %d", pieceID)
	}
	if a.selectedPiece != pieceID {
		t.Fatalf("selecting tray piece %d did not register (selected=%d)", pieceID, a.selectedPiece)
	}
	n := len(game.Orientations(game.Pieces[pieceID].Cells))
	for i := 0; i < orientIdx%n; i++ {
		if !rotateSelection(h, a) {
			t.Fatal("Rotera button not found/tappable")
		}
	}
	if a.orientIdx != orientIdx%n {
		t.Fatalf("orientIdx = %d after rotating, want %d", a.orientIdx, orientIdx%n)
	}
	return tapBoard(h, a, anchor.X, anchor.Y)
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// place puts owner c with piece id pid at (x,y) directly on the app's board
// — used to construct specific test positions (mirrors game.place, but that
// helper is unexported in the game package and this is a different
// package/binary).
func setCell(gs *game.GameState, x, y int, c game.Cell, pid int8) {
	gs.Board.Owner[y][x] = c
	gs.Board.PieceID[y][x] = pid
}

func clearBoard(gs *game.GameState) {
	gs.Board = game.NewBoard()
}

// --- RULE: Cathedral placement then turn order ------------------------------

func TestPlayStadskarnanCathedralThenTurnOrder(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	if a.gs.Phase != game.PhaseCathedral {
		t.Fatalf("game should start in PhaseCathedral, got %v", a.gs.Phase)
	}
	placements := game.LegalCathedralPlacements(&a.gs.Board)
	if len(placements) == 0 {
		t.Fatal("empty board should offer Cathedral placements")
	}
	anchor := placements[0].Anchor
	if !placeCathedral(h, a, anchor) {
		t.Fatal("legal Cathedral placement via tap should succeed")
	}
	if a.gs.Phase != game.PhasePlaying {
		t.Fatalf("Phase should be PhasePlaying after the Cathedral is placed, got %v", a.gs.Phase)
	}
	if a.gs.Turn != game.White {
		t.Fatalf("White should move first, got %v", a.gs.Turn)
	}
	if a.gs.Board.Count(game.Cathedral) != game.CathedralShape.Size() {
		t.Fatalf("Cathedral cell count = %d, want %d", a.gs.Board.Count(game.Cathedral), game.CathedralShape.Size())
	}
}

// --- RULE: select-rotate-place via the real tray + board --------------------

func TestPlayStadskarnanSelectRotatePlace(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	placeCathedral(h, a, game.LegalCathedralPlacements(&a.gs.Board)[0].Anchor)

	// White to move. Pick the "Krog" L-tetromino (asymmetric: has multiple
	// distinct orientations) and place it in orientation 2.
	pieceID := 12 // Krog
	orients := game.Orientations(game.Pieces[pieceID].Cells)
	if len(orients) < 3 {
		t.Fatalf("setup: expected Krog to have >=3 orientations, got %d", len(orients))
	}
	placements := game.LegalPlacementsForOrientation(&a.gs.Board, pieceID, 2)
	if len(placements) == 0 {
		t.Fatal("setup: orientation 2 of Krog should have legal placements on an empty board")
	}
	anchor := placements[0].Anchor

	if !placePieceAtOrientation(t, h, a, pieceID, 2, anchor) {
		t.Fatal("legal select+rotate+place via real taps should succeed")
	}
	if a.gs.Hand(game.White)[pieceID] {
		t.Fatal("the placed piece should now be marked unavailable")
	}
	if a.selectedPiece != -1 {
		t.Fatal("selection should clear after a successful placement")
	}
	if a.gs.Turn != game.Black {
		t.Fatalf("turn should pass to Black, got %v", a.gs.Turn)
	}
	// Every covered cell should actually belong to White with this piece id.
	for _, c := range orients[2] {
		x, y := anchor.X+c[0]-orients[2][0][0], anchor.Y+c[1]-orients[2][0][1]
		if a.gs.Board.At(x, y) != game.White {
			t.Fatalf("cell (%d,%d) should be White after placement", x, y)
		}
	}
}

// --- RULE: tapping the selected tray piece again deselects ------------------

func TestPlayStadskarnanDeselectTrayPiece(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	placeCathedral(h, a, game.LegalCathedralPlacements(&a.gs.Board)[0].Anchor)

	if !selectTrayPiece(h, a, 0) {
		t.Fatal("selecting a tray piece should register")
	}
	if a.selectedPiece != 0 {
		t.Fatal("piece 0 should now be selected")
	}
	if !selectTrayPiece(h, a, 0) {
		t.Fatal("tapping the same tray piece again should register")
	}
	if a.selectedPiece != -1 {
		t.Fatal("tapping the selected piece again should deselect it")
	}
}

// --- GOTCHA: illegal taps are rejected ---------------------------------------

func TestPlayStadskarnanIllegalTapsRejected(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	placeCathedral(h, a, game.LegalCathedralPlacements(&a.gs.Board)[0].Anchor)

	// Selecting a piece then tapping an occupied cell (a Cathedral cell) must
	// not place anything or change the turn.
	if !selectTrayPiece(h, a, 0) {
		t.Fatal("selecting a piece should register")
	}
	var cathedralCell image.Point
	found := false
	for y := 0; y < game.Size && !found; y++ {
		for x := 0; x < game.Size && !found; x++ {
			if a.gs.Board.At(x, y) == game.Cathedral {
				cathedralCell = image.Pt(x, y)
				found = true
			}
		}
	}
	if !found {
		t.Fatal("setup: Cathedral should be on the board")
	}
	turnBefore := a.gs.Turn
	tapBoard(h, a, cathedralCell.X, cathedralCell.Y)
	if a.gs.Turn != turnBefore {
		t.Fatal("tapping an occupied cell must not advance the turn")
	}
	if a.gs.Hand(turnBefore)[0] == false {
		t.Fatal("tapping an occupied cell must not consume the selected piece")
	}
}

// --- GOTCHA: enclosure capture via real taps, and the sealed-cell rule ------

func TestPlayStadskarnanCaptureViaRealTaps(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	placeCathedral(h, a, game.LegalCathedralPlacements(&a.gs.Board)[0].Anchor)
	clearBoard(a.gs)

	// Black's monomino trapped on 3 sides at (5,5); White (to move) can
	// complete the capture at (5,6) with its own monomino (piece 0, "Stuga").
	setCell(a.gs, 5, 5, game.Black, 1)
	setCell(a.gs, 4, 5, game.White, 2)
	setCell(a.gs, 6, 5, game.White, 3)
	setCell(a.gs, 5, 4, game.White, 4)
	*a.gs.Hand(game.Black) = game.Hand{}
	a.gs.Hand(game.Black)[1] = false
	wh := game.NewHand()
	wh[2], wh[3], wh[4] = false, false, false
	a.gs.Hands[1] = wh
	a.gs.Turn = game.White
	h.Draw()

	if !placePieceAtOrientation(t, h, a, 0, 0, image.Pt(5, 6)) {
		t.Fatal("the capturing placement via real taps should succeed")
	}
	if a.gs.Board.At(5, 5) != game.Empty {
		t.Fatal("Black's trapped piece should have been captured")
	}
	if !a.gs.Hand(game.Black)[1] {
		t.Fatal("the captured piece should be returned to Black's hand")
	}
	if !a.gs.Board.IsSealed(5, 5) {
		t.Fatal("the captured cell should now be permanently sealed")
	}
	if len(a.gs.LastCaptured) != 1 {
		t.Fatalf("LastCaptured = %v, want exactly [(5,5)]", a.gs.LastCaptured)
	}

	// The sealed cell must never accept a future placement, even via a real
	// tap: select a monomino for Black and try to tap the sealed cell.
	if a.gs.Turn != game.Black {
		t.Fatal("setup: it should now be Black's turn")
	}
	if !selectTrayPiece(h, a, 1) {
		t.Fatal("Black should be able to select its returned monomino")
	}
	before := a.gs.Board
	tapBoard(h, a, 5, 5)
	if a.gs.Board != before {
		t.Fatal("tapping a sealed cell must never place a piece there")
	}
}

// --- RULE: a side with no legal placement is skipped, not ending the game --

func TestPlayStadskarnanStuckSideIsSkipped(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	placeCathedral(h, a, game.LegalCathedralPlacements(&a.gs.Board)[0].Anchor)
	clearBoard(a.gs)

	*a.gs.Hand(game.Black) = game.Hand{} // Black has nothing to place at all
	*a.gs.Hand(game.White) = game.NewHand()
	a.gs.Turn = game.White
	h.Draw()

	placements := game.LegalPlacementsForOrientation(&a.gs.Board, 0, 0)
	if !placePieceAtOrientation(t, h, a, 0, 0, placements[0].Anchor) {
		t.Fatal("White's placement should be legal")
	}
	if a.gs.Phase != game.PhasePlaying {
		t.Fatalf("game should continue (White still has room and pieces), got Phase=%v", a.gs.Phase)
	}
	if a.gs.Turn != game.White {
		t.Fatalf("Black has nothing to place, so it should stay White's turn, got %v", a.gs.Turn)
	}
}

// --- RULE: game end and winner banner (fewest remaining squares) -----------

func TestPlayStadskarnanWinnerBanner(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	placeCathedral(h, a, game.LegalCathedralPlacements(&a.gs.Board)[0].Anchor)
	clearBoard(a.gs)

	// Fill the board leaving one free cell; give White a monomino to place
	// into it (ending the game with both hands otherwise exhausted, White
	// strictly ahead).
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			if x == 0 && y == 0 {
				continue
			}
			a.gs.Board.Owner[y][x] = game.Black
		}
	}
	*a.gs.Hand(game.Black) = game.NewHand() // full hand, but the board is full: no room to place
	wh := game.Hand{}
	wh[0] = true // Stuga, a monomino: White's only remaining piece
	a.gs.Hands[1] = wh
	a.gs.Turn = game.White
	h.Draw()

	if !placePieceAtOrientation(t, h, a, 0, 0, image.Pt(0, 0)) {
		t.Fatal("White's placement into the last free cell should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatalf("Phase should be Done once neither side can place, got %v", a.gs.Phase)
	}
	if a.gs.Winner() != game.White {
		t.Fatalf("White (0 remaining) should beat Black (36 remaining), got %v", a.gs.Winner())
	}
	if _, ok := h.FindTextContains("Vit vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- vs AI: the AI actually replies -----------------------------------------

func TestPlayStadskarnanVsAIReplies(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI)
	before := a.gs.Board
	// h.TapRect (inside placeCathedral) already re-runs Draw() until the app
	// stops calling Repaint(), so by the time it returns the AI's first
	// reply (queued via aiPend right after the Cathedral goes down) has
	// already been drained — mirrors hasami's play_test.go pattern.
	if !placeCathedral(h, a, game.LegalCathedralPlacements(&a.gs.Board)[0].Anchor) {
		t.Fatal("Cathedral placement should succeed")
	}
	if a.gs.AITurn() {
		t.Fatal("control returned on the AI's turn (deferred reply not drained)")
	}
	if a.gs.Board == before {
		t.Fatal("the AI (White) should have taken its first turn right after the Cathedral was placed")
	}
	if a.gs.Turn != game.Black {
		t.Fatalf("turn should be back to Black after White's (AI) reply, got %v", a.gs.Turn)
	}
}

// --- Full hotseat game to completion, driving BOTH colors through real taps

func TestPlayStadskarnanFullHotseatGame(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	placeCathedral(h, a, game.LegalCathedralPlacements(&a.gs.Board)[0].Anchor)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 60 {
			t.Fatal("game did not terminate")
		}
		side := a.gs.Turn
		pieceID, orientIdx, anchor, ok := game.BestPlacement(a.gs, side)
		if !ok {
			t.Fatalf("side %v to move but no legal placement found (should have been passed already)", side)
		}
		if !placePieceAtOrientation(t, h, a, pieceID, orientIdx, anchor) {
			t.Fatalf("legal placement at ply %d was rejected via the real UI", ply)
		}
	}
	rb, rw := a.gs.Hand(game.Black).RemainingSquares(), a.gs.Hand(game.White).RemainingSquares()
	want := "Oavgjort!"
	switch a.gs.Winner() {
	case game.Black:
		want = "Svart vann!"
	case game.White:
		want = "Vit vann!"
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("end banner %q (B%d/W%d) not shown; visible: %v", want, rb, rw, texts(h))
	}
}

// --- Quit mid-game (Back key AND the Meny button), then restart ------------

func TestPlayStadskarnanQuitAndRestart(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	placeCathedral(h, a, game.LegalCathedralPlacements(&a.gs.Board)[0].Anchor)

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}

	startOpponent(t, h, a, game.OpponentAI)
	placeCathedral(h, a, game.LegalCathedralPlacements(&a.gs.Board)[0].Anchor)
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

	// "Ny" restarts the current configuration mid-game.
	startOpponent(t, h, a, game.OpponentHotseat)
	placeCathedral(h, a, game.LegalCathedralPlacements(&a.gs.Board)[0].Anchor)
	placements := game.LegalPlacementsForOrientation(&a.gs.Board, 0, 0)
	placePieceAtOrientation(t, h, a, 0, 0, placements[0].Anchor)
	if a.gs.Turn != game.Black {
		t.Fatal("setup: expected a move to have been made (turn back to Black)")
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
	if a.gs.Phase != game.PhaseCathedral {
		t.Fatalf("Ny should reset to a fresh game (PhaseCathedral), got %v", a.gs.Phase)
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlayStadskarnanRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("Baserat på Cathedral"); !ok {
		t.Fatalf("rules text missing the attribution credit; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayStadskarnanScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/stadskarnan_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/stadskarnan_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/stadskarnan_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startOpponent(t, h, a, game.OpponentHotseat)
	if err := h.Screenshot(dir + "/stadskarnan_cathedral.png"); err != nil {
		t.Fatal(err)
	}
	placeCathedral(h, a, game.LegalCathedralPlacements(&a.gs.Board)[0].Anchor)
	selectTrayPiece(h, a, 0)
	if err := h.Screenshot(dir + "/stadskarnan_board.png"); err != nil {
		t.Fatal(err)
	}
}
