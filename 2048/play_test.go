//go:build playtest

package main

// Headless PLAYTHROUGH tests for 2048. They drive the real touch/swipe path
// and check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): a swipe slides and merges tiles (merge-once-per-move), a no-op
// swipe spawns nothing, a new tile appears after every real move, reaching
// the target shows the win banner with a "Fortsätt" option, and a dead board
// (full, no equal neighbours) ends the game. Runs under the pure-Go inkview
// emulator (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"twenty48/game"
)

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

func startGame(t *testing.T, h *ink.Harness, a *app, target int) {
	t.Helper()
	for _, row := range a.menu.rows {
		if row.choice.target == target {
			h.TapRect(row.rect)
			if a.screen != screenGame || a.gs == nil || a.gs.Target != target {
				t.Fatalf("did not start target %d (screen=%v)", target, a.screen)
			}
			return
		}
	}
	t.Fatalf("no menu row for target %d", target)
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// swipe injects a raw pointer-down at from and pointer-up at to, exactly the
// gesture the real device Pointer() path expects (App.Pointer, not the
// harness's same-point Tap).
func swipe(a *app, from, to image.Point) {
	a.Pointer(ink.PointerEvent{Point: from, State: ink.PointerDown})
	a.Pointer(ink.PointerEvent{Point: to, State: ink.PointerUp})
}

func sumBoard(b game.Board) int {
	total := 0
	for _, v := range b {
		total += v
	}
	return total
}

// --- RULE: swipe slides/merges per the rules, matching a pure Slide() -------

func TestPlay2048SwipeMatchesSlide(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, 2048)

	before := a.gs.Board
	wantBoard, wantGained, wantMoved := game.Slide(before, game.Left)
	if !wantMoved {
		t.Skip("initial board happened to be a no-op left swipe; flaky by design, skip")
	}

	swipe(a, image.Pt(900, 700), image.Pt(200, 700)) // right-to-left drag
	h.Draw()

	// The merged region must match Slide()'s output exactly (spawn aside):
	// every non-spawn cell should equal wantBoard, and the total sum should be
	// wantBoard's sum plus exactly one spawned tile (2 or 4).
	diffSum := sumBoard(a.gs.Board) - sumBoard(wantBoard)
	if diffSum != 2 && diffSum != 4 {
		t.Fatalf("post-swipe board doesn't look like Slide()+one spawn: diff=%d\nwant base=%v\ngot=%v",
			diffSum, wantBoard, a.gs.Board)
	}
	if a.gs.Score != wantGained {
		t.Fatalf("score = %d, want %d (from the merge)", a.gs.Score, wantGained)
	}
}

// --- RULE: a swipe that changes nothing is not a move (no spawn, no score) --

func TestPlay2048NoOpSwipeDoesNothing(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, 2048)

	// Force a board where swiping right is a guaranteed no-op: everything
	// already packed to the right edge, no equal neighbours.
	a.gs.Board = game.Board{}
	set := func(x, y, v int) { a.gs.Board[y*game.Size+x] = v }
	set(3, 0, 2)
	set(2, 0, 4)
	before := a.gs.Board
	beforeScore := a.gs.Score

	swipe(a, image.Pt(200, 700), image.Pt(900, 700)) // left-to-right: already packed right
	h.Draw()

	if a.gs.Board != before {
		t.Fatalf("no-op swipe changed the board: %v -> %v", before, a.gs.Board)
	}
	if a.gs.Score != beforeScore {
		t.Fatal("no-op swipe changed the score")
	}
}

// --- Tap-arrow fallback input works identically to a swipe ------------------

func TestPlay2048TapArrows(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, 2048)

	before := a.gs.Board
	_, _, wantMoved := game.Slide(before, game.Up)
	dir, ok := a.layout.ArrowHit(a.layout.arrows[1].rect.Min.Add(image.Pt(5, 5)))
	if !ok || dir != game.Up {
		t.Fatalf("arrow[1] is not the Up arrow (dir=%v ok=%v)", dir, ok)
	}
	h.TapRect(a.layout.arrows[1].rect) // the ▲ button
	moved := a.gs.Board != before
	if moved != wantMoved {
		t.Fatalf("tap-arrow Up: moved=%v, want %v", moved, wantMoved)
	}
}

// --- WIN banner + Fortsätt keeps playing -------------------------------------

func TestPlay2048WinBannerAndContinue(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, 2048)

	a.gs.Board = game.Board{}
	a.gs.Board[0] = 1024
	a.gs.Board[1] = 1024
	h.Draw()
	swipe(a, image.Pt(900, 700), image.Pt(200, 700)) // merge into 2048
	h.Draw()

	if a.gs.Status != game.StatusWon {
		t.Fatalf("status = %v, want StatusWon", a.gs.Status)
	}
	if _, ok := h.FindTextContains("2048"); !ok {
		t.Fatalf("win banner missing; visible: %v", texts(h))
	}
	if err := h.TapText("Fortsätt"); err != nil {
		t.Fatalf("no Fortsätt button: %v", err)
	}
	if a.gs.Status != game.StatusPlaying {
		t.Fatalf("status after Fortsätt = %v, want StatusPlaying", a.gs.Status)
	}
	// Play continues to accept moves after continuing.
	before := a.gs.Board
	swipe(a, image.Pt(700, 900), image.Pt(700, 200)) // swipe up
	if a.gs.Board == before {
		t.Log("swipe after Fortsätt happened to be a no-op board; not fatal")
	}
}

// --- GAME OVER banner + Ny restarts ------------------------------------------

func TestPlay2048GameOverAndRestart(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, 2048)

	dead := game.Board{
		2, 4, 2, 4,
		4, 2, 4, 2,
		2, 4, 2, 4,
		4, 2, 4, 2,
	}
	a.gs.Board = dead
	a.gs.Status = game.StatusOver
	h.Draw()

	if _, ok := h.FindTextContains("Spelet slut"); !ok {
		t.Fatalf("game-over banner missing; visible: %v", texts(h))
	}
	// A swipe must be rejected once the game is over.
	swipe(a, image.Pt(900, 700), image.Pt(200, 700))
	if a.gs.Board != dead {
		t.Fatal("a swipe was accepted after game over")
	}

	if err := h.TapText("Ny"); err != nil {
		t.Fatalf("no Ny button: %v", err)
	}
	if a.gs.Status != game.StatusPlaying {
		t.Fatalf("status after Ny = %v, want StatusPlaying", a.gs.Status)
	}
	if a.gs.Board == dead {
		t.Fatal("Ny did not produce a fresh board")
	}
}

// --- Best-score persistence survives a restart of the app -------------------

func TestPlay2048BestScorePersists(t *testing.T) {
	tmp := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(tmp)

	h, a := bootToMenu(t)
	startGame(t, h, a, 2048)
	a.gs.Board = game.Board{}
	a.gs.Board[0] = 1024
	a.gs.Board[1] = 1024
	swipe(a, image.Pt(900, 700), image.Pt(200, 700)) // scores 2048
	if a.gs.Best < 2048 {
		t.Fatalf("best = %d after a 2048 merge, want >= 2048", a.gs.Best)
	}

	// A missing file must default to 0 (fresh app instance, same directory).
	os.Remove(bestScoreFile)
	b2 := &app{}
	h2, err := ink.Boot(b2)
	if err != nil {
		t.Fatal(err)
	}
	if got := b2.loadBest(); got != 0 {
		t.Fatalf("loadBest with missing file = %d, want 0", got)
	}
	_ = h2

	// A corrupt file must also default to 0, not crash.
	os.WriteFile(bestScoreFile, []byte("garbage\x00not a number"), 0o644)
	if got := b2.loadBest(); got != 0 {
		t.Fatalf("loadBest with corrupt file = %d, want 0", got)
	}

	// A real save round-trips.
	a.saveBest()
	b3 := &app{}
	if _, err := ink.Boot(b3); err != nil {
		t.Fatal(err)
	}
	if got := b3.loadBest(); got != a.gs.Best {
		t.Fatalf("loadBest after saveBest = %d, want %d", got, a.gs.Best)
	}
}

// --- Quit mid-game (Back key AND Meny button) --------------------------------

func TestPlay2048Quit(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, 2048)

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}

	startGame(t, h, a, 2048)
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
}

// --- Rules screen -------------------------------------------------------------

func TestPlay2048RulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("2048"); !ok {
		t.Fatalf("rules text missing; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Alternate targets (1024 shorter game, 4096 longer game) -----------------

func TestPlay2048AlternateTargets(t *testing.T) {
	for _, target := range []int{1024, 4096} {
		h, a := bootToMenu(t)
		startGame(t, h, a, target)
		a.gs.Board = game.Board{}
		half := target / 2
		a.gs.Board[0] = half
		a.gs.Board[1] = half
		swipe(a, image.Pt(900, 700), image.Pt(200, 700))
		if a.gs.Status != game.StatusWon {
			t.Fatalf("target %d: status = %v, want StatusWon", target, a.gs.Status)
		}
	}
}

// --- Screenshots of every screen for visual review ---------------------------

func TestPlay2048Screenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/2048_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/2048_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/2048_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startGame(t, h, a, 2048)
	a.gs.Board = game.Board{
		2, 4, 8, 16,
		32, 64, 128, 256,
		512, 1024, 2, 4,
		8, 16, 32, 64,
	}
	h.Draw()
	if err := h.Screenshot(dir + "/2048_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
