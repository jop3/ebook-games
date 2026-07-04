//go:build playtest

package main

// Headless PLAYTHROUGH tests for Munkar. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): Black starts and may place anywhere; the line glyph of the cell
// just filled forces the opponent onto that row/column/diagonal (or
// anywhere, if that line is already full); custodial capture flips the
// ENEMY bookends around the MOVER's own bounded run (the inverted-from-
// Othello direction — this is the single easiest thing to get backwards, so
// it gets its own dedicated test that checks the correct side flips); 5 in a
// row wins immediately; a full board with no five is decided by the largest
// orthogonally-connected group. Covers both modes (2 spelare / Mot dator, all
// 3 AI difficulties), the direction-forcing constraint (including the
// "line full -> anywhere" fallback) and illegal-move rejection via real
// taps, both win paths, quitting, the rules screen, and a full game played
// out to a real AI win. Runs under the pure-Go inkview emulator
// (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"munkar/game"
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

// startMode picks the first menu row for the given mode (and, for the AI,
// the given search depth), taps it, and enters the game.
func startMode(t *testing.T, h *ink.Harness, a *app, mode game.Mode, aiLevel int) {
	t.Helper()
	for _, row := range a.menu.rows {
		if row.choice.mode == mode && (mode == game.ModeHotseat || row.choice.aiLevel == aiLevel) {
			h.TapRect(row.rect)
			if a.screen != screenGame || a.gs == nil || a.gs.Mode != mode {
				t.Fatalf("did not start mode %v (screen=%v)", mode, a.screen)
			}
			return
		}
	}
	t.Fatalf("no menu row for mode %v level %d; visible: %v", mode, aiLevel, texts(h))
}

// tapCell taps board cell p via the app's current layout.
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

// firstLegalMove is a deterministic human policy for full-game playthroughs:
// always play the first cell LegalMoves() offers (which already respects the
// forced-direction constraint, falling back to "anywhere" when unconstrained
// or the forced line is full).
func firstLegalMove(a *app) (image.Point, bool) {
	moves := a.gs.LegalMoves()
	if len(moves) == 0 {
		return image.Point{}, false
	}
	return moves[0], true
}

// ringTotal returns Black+White+Empty cells (must always be 36).
func ringTotal(b *game.Board) int {
	return b.Count(game.Black) + b.Count(game.White) + b.Count(game.Empty)
}

// --- RULE: placement, illegal rejection -------------------------------------

func TestPlayMunkarPlacementRules(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	// Black's first move is unconstrained: every empty cell is legal.
	if len(a.gs.LegalMoves()) != game.Size*game.Size {
		t.Fatalf("first move should allow all %d cells, got %d", game.Size*game.Size, len(a.gs.LegalMoves()))
	}

	before := a.gs.Board
	p := image.Pt(2, 3)
	if !tapCell(h, a, p) {
		t.Fatal("Black's opening move anywhere should be legal")
	}
	want, _ := game.Place(before, p, game.Black)
	if a.gs.Board != want {
		t.Fatalf("UI move %v did not match the rules' own Place result", p)
	}
	if a.gs.Turn != game.White {
		t.Fatal("turn did not pass to White after a legal move")
	}
	if ringTotal(&a.gs.Board) != game.Size*game.Size {
		t.Fatalf("board inconsistent: total %d", ringTotal(&a.gs.Board))
	}
}

// --- RULE: direction-forcing, via real taps ---------------------------------

func TestPlayMunkarDirectionForcing(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	// Force a known orientation so this test doesn't depend on the random
	// tile shuffle: (2,2) is horizontal, so placing there forces White onto
	// row 2.
	a.gs.Board.Line[2][2] = game.OrientH
	if !tapCell(h, a, image.Pt(2, 2)) {
		t.Fatal("Black's first move should be legal")
	}
	if a.gs.Turn != game.White {
		t.Fatal("setup: expected White to move next")
	}
	forced := a.gs.LegalMoves()
	if len(forced) == 0 {
		t.Fatal("White should be forced onto row 2")
	}
	for _, p := range forced {
		if p.Y != 2 {
			t.Fatalf("forced move %v is not on row 2", p)
		}
	}

	// GOTCHA: tapping off the forced line is illegal and must be rejected —
	// no ring placed, turn unchanged.
	before := a.gs.Board
	if tapCell(h, a, image.Pt(0, 5)) {
		t.Fatal("a tap off the forced line should have been rejected")
	}
	if a.gs.Board != before || a.gs.Turn != game.White {
		t.Fatal("a rejected tap must not mutate the board or the turn")
	}
	if _, ok := h.FindTextContains("Ogiltigt drag"); !ok {
		t.Fatalf("illegal tap should surface a hint; visible: %v", texts(h))
	}

	// A tap ON the forced line is accepted.
	if !tapCell(h, a, forced[0]) {
		t.Fatalf("forced move %v should be legal", forced[0])
	}
	if a.gs.Turn != game.Black {
		t.Fatal("turn should now be Black's again")
	}
}

// GOTCHA: when every cell on the forced line is already occupied, the next
// player may play anywhere — verified via a real tap far off that line.
func TestPlayMunkarLineFullFallback(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	for y := range a.gs.Board.Ring {
		for x := range a.gs.Board.Ring[y] {
			a.gs.Board.Ring[y][x] = game.Empty
		}
	}
	for x := 0; x < game.Size; x++ {
		a.gs.Board.Line[3][x] = game.OrientH
	}
	// Fill all of row 3 except the very last move, which we make explicit
	// below.
	for x := 0; x < game.Size-1; x++ {
		c := game.Black
		if x%2 == 1 {
			c = game.White
		}
		a.gs.Board.Ring[3][x] = c
	}
	a.gs.Turn = game.Black
	a.gs.HasLast = false
	h.Draw()

	// Black completes row 3 (fills the only remaining cell on it): row 3 is
	// now entirely full, so this placement's forced line has nothing left,
	// meaning White may play anywhere next — try a tap far away, off row 3.
	if !tapCell(h, a, image.Pt(5, 3)) {
		t.Fatal("completing the last cell of row 3 should be legal")
	}
	if a.gs.Turn != game.White {
		t.Fatal("setup: expected White to move next")
	}
	for _, p := range a.gs.LegalMoves() {
		if p.Y == 3 {
			t.Fatalf("row 3 is full, LegalMoves should not include any cell on it, got %v", p)
		}
	}
	if !tapCell(h, a, image.Pt(1, 1)) {
		t.Fatal("with the forced line full, White should be able to play anywhere, e.g. (1,1)")
	}
	if a.gs.Board.At(1, 1) != game.White {
		t.Fatal("White's ring should now sit at (1,1)")
	}
}

// --- GOTCHA: capture direction — the MOVER's own run gets bounded, and the
// ENEMY bookends are what flip (never the reverse) — via a real tap. -------

func TestPlayMunkarCaptureFlipsTheCorrectSide(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	for y := range a.gs.Board.Ring {
		for x := range a.gs.Board.Ring[y] {
			a.gs.Board.Ring[y][x] = game.Empty
		}
	}
	// "O_O -> OXO => XXX": White at (1,0) and (3,0), gap at (2,0). Black taps
	// the gap; BOTH White rings must flip to Black, and Black's own new ring
	// must remain Black (not somehow itself flip).
	a.gs.Board.Ring[0][1] = game.White
	a.gs.Board.Ring[0][3] = game.White
	a.gs.Turn = game.Black
	a.gs.HasLast = false
	h.Draw()

	if !tapCell(h, a, image.Pt(2, 0)) {
		t.Fatal("the capturing placement should be legal")
	}
	if a.gs.Board.At(2, 0) != game.Black {
		t.Fatal("the newly-placed ring must remain Black")
	}
	if a.gs.Board.At(1, 0) != game.Black || a.gs.Board.At(3, 0) != game.Black {
		t.Fatalf("both White bookends must flip to Black; got (1,0)=%v (3,0)=%v",
			a.gs.Board.At(1, 0), a.gs.Board.At(3, 0))
	}
	if len(a.gs.LastFlips) != 2 {
		t.Fatalf("LastFlips = %v, want exactly the 2 flipped White bookends", a.gs.LastFlips)
	}
}

// --- WIN: 5 in a row ---------------------------------------------------------

func TestPlayMunkarFiveInRowWin(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	for y := range a.gs.Board.Ring {
		for x := range a.gs.Board.Ring[y] {
			a.gs.Board.Ring[y][x] = game.Empty
		}
	}
	for x := 0; x < 4; x++ {
		a.gs.Board.Ring[4][x] = game.Black
	}
	a.gs.Turn = game.Black
	a.gs.HasLast = false
	h.Draw()

	if !tapCell(h, a, image.Pt(4, 4)) {
		t.Fatal("the line-completing move should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once 5 in a row completes")
	}
	if _, ok := h.FindTextContains("Svart vinner!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- WIN: full board, largest-group tiebreak --------------------------------

func TestPlayMunkarFullBoardTiebreakWin(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	// A pre-verified layout (no five-in-a-row for either color): Black's
	// largest connected group is 13, White's is 8. One White cell, (1,2),
	// is left empty and refilling it captures nothing, so the final board
	// after the tap is exactly this layout.
	rows := [6]string{
		"B.BBBB",
		"...B..",
		"..BBBB",
		".BB.B.",
		".B...B",
		"B.BB.B",
	}
	for y, row := range rows {
		for x, ch := range row {
			c := game.White
			if ch == 'B' {
				c = game.Black
			}
			a.gs.Board.Ring[y][x] = c
		}
	}
	a.gs.Board.Ring[2][1] = game.Empty
	a.gs.Turn = game.White
	a.gs.HasLast = false
	h.Draw()

	if game.Five(&a.gs.Board, game.Black) || game.Five(&a.gs.Board, game.White) {
		t.Fatal("setup: no five should exist yet")
	}
	if !tapCell(h, a, image.Pt(1, 2)) {
		t.Fatal("filling the last cell should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once the board fills")
	}
	if _, ok := h.FindTextContains("Svart vinner!"); !ok {
		t.Fatalf("tiebreak win banner not shown; visible: %v", texts(h))
	}
}

// --- All 3 AI difficulties actually reply -----------------------------------

func TestPlayMunkarAllDifficultiesReply(t *testing.T) {
	for _, depth := range []int{game.DepthEasy, game.DepthMedium, game.DepthHard} {
		depth := depth
		t.Run(itoa(depth), func(t *testing.T) {
			h, a := bootToMenu(t)
			startMode(t, h, a, game.ModeAI, depth)
			if a.gs.AILevel != depth {
				t.Fatalf("AILevel = %d, want %d", a.gs.AILevel, depth)
			}
			before := a.gs.Board
			mv, ok := firstLegalMove(a)
			if !ok {
				t.Fatal("Black should have a legal opening move")
			}
			if !tapCell(h, a, mv) {
				t.Fatal("Black's opening move should be legal")
			}
			if a.gs.AITurn() {
				t.Fatal("control returned on the AI's turn (deferred reply not drained)")
			}
			if a.gs.Board == before {
				t.Fatal("White (the AI) did not reply")
			}
		})
	}
}

// --- Full game vs the AI, played to a real win -------------------------------

func TestPlayMunkarFullGameVsAI(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeAI, game.DepthMedium)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 100 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		mv, ok := firstLegalMove(a)
		if !ok {
			t.Fatalf("human to move but no legal move at ply %d", ply)
		}
		if !tapCell(h, a, mv) {
			t.Fatalf("legal move %v at ply %d was rejected", mv, ply)
		}
		if ringTotal(&a.gs.Board) != game.Size*game.Size {
			t.Fatalf("board inconsistent mid-game: %d", ringTotal(&a.gs.Board))
		}
	}
	want := "Oavgjort!"
	switch a.gs.Winner() {
	case game.Black:
		want = "Svart vinner!"
	case game.White:
		want = "Vit vinner!"
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("end banner %q not shown; visible: %v", want, texts(h))
	}
}

// --- Quit mid-game (Back key AND the Meny button), then restart -------------

func TestPlayMunkarQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)
	mv, _ := firstLegalMove(a)
	tapCell(h, a, mv) // a move in progress

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}

	startMode(t, h, a, game.ModeAI, game.DepthEasy)
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
	startMode(t, h, a, game.ModeHotseat, 0)
}

// --- "Ny" restarts the current configuration mid-game -----------------------

func TestPlayMunkarNyRestarts(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)
	mv, _ := firstLegalMove(a)
	tapCell(h, a, mv)
	if a.gs.Turn != game.White {
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
	if a.gs.Turn != game.Black || a.gs.Board.Count(game.Black) != 0 || a.gs.Board.Count(game.White) != 0 {
		t.Fatal("Ny should reset to a fresh, empty starting position")
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlayMunkarRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("FIENDERINGARNA"); !ok {
		t.Fatalf("rules text missing the explicit inverted-capture-direction statement; visible: %v", texts(h))
	}
	if _, ok := h.FindTextContains("Lika stora grupper ger oavgjort"); !ok {
		t.Fatalf("rules text missing the largest-group tiebreak statement; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayMunkarScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/munkar_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/munkar_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/munkar_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startMode(t, h, a, game.ModeAI, game.DepthMedium)
	// Play a few moves (including at least one capture, engineered
	// directly) to get a representative mid-game board with rings of both
	// colors, a faint-glyph background, and a highlighted forced line.
	a.gs.Board.Ring[2][1] = game.White
	a.gs.Board.Ring[2][3] = game.White
	tapCell(h, a, image.Pt(2, 2)) // Black captures both White bookends
	for i := 0; i < 3 && a.gs.Phase == game.PhasePlaying && !a.gs.AITurn(); i++ {
		mv, ok := firstLegalMove(a)
		if !ok {
			break
		}
		tapCell(h, a, mv)
	}
	if err := h.Screenshot(dir + "/munkar_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// Five-in-a-row end-game banner.
	for y := range a.gs.Board.Ring {
		for x := range a.gs.Board.Ring[y] {
			a.gs.Board.Ring[y][x] = game.Empty
		}
	}
	for x := 0; x < 5; x++ {
		a.gs.Board.Ring[3][x] = game.Black
	}
	a.gs.Phase = game.PhaseDone
	h.Draw()
	if err := h.Screenshot(dir + "/munkar_five_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// Largest-group tiebreak end-game banner (full board, no five).
	h.Back()
	startMode(t, h, a, game.ModeHotseat, 0)
	rows := [6]string{
		"B.BBBB",
		"...B..",
		"..BBBB",
		".BB.B.",
		".B...B",
		"B.BB.B",
	}
	for y, row := range rows {
		for x, ch := range row {
			c := game.White
			if ch == 'B' {
				c = game.Black
			}
			a.gs.Board.Ring[y][x] = c
		}
	}
	a.gs.Phase = game.PhaseDone
	h.Draw()
	if err := h.Screenshot(dir + "/munkar_group_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
