//go:build playtest

package main

// Headless PLAYTHROUGH tests for Go (goban). They drive the real touch path
// and check gameplay against the rules as written (see rulesParagraphs in
// ui.go): alternating placement, capture-before-suicide, the simple
// positional ko rule, passing (single and double), the mark-dead phase and
// its effect on area (Chinese) scoring, both opponent modes (hotseat on all
// 3 sizes, the weak AI offered only on 9x9), a full game played to a real
// final score, quitting, and the rules screen. Runs under the pure-Go
// inkview emulator (playtest/play.sh).

import (
	"image"
	"os"
	"testing"
	"time"

	ink "github.com/dennwc/inkview"

	"goban/game"
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

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

var sizeLabel = map[int]string{9: "9x9", 13: "13x13", 19: "19x19"}

// selectSize taps the given board-size button on the menu.
func selectSize(t *testing.T, h *ink.Harness, a *app, size int) {
	t.Helper()
	if err := h.TapText(sizeLabel[size]); err != nil {
		t.Fatalf("no size button %q: %v; visible=%v", sizeLabel[size], err, texts(h))
	}
	if a.menu.size != size {
		t.Fatalf("size not selected: menu.size=%d want %d", a.menu.size, size)
	}
}

// startOpponent taps the given opponent row (starting the game).
func startOpponent(t *testing.T, h *ink.Harness, a *app, opp game.Opponent) {
	t.Helper()
	label := "2 spelare"
	if opp == game.OpponentAI {
		label = "Mot dator (svag)"
	}
	if err := h.TapText(label); err != nil {
		t.Fatalf("no opponent row %q: %v; visible=%v", label, err, texts(h))
	}
	if a.screen != screenGame || a.gs == nil || a.gs.Opponent != opp {
		t.Fatalf("did not start opponent %v (screen=%v)", opp, a.screen)
	}
}

// tapPoint drives a real tap at board intersection (x,y) through the layout.
func tapPoint(h *ink.Harness, a *app, x, y int) bool {
	return h.TapRect(a.layout.PointRect(image.Pt(x, y)))
}

// tapButton taps the game-screen button with the given label.
func tapButton(t *testing.T, h *ink.Harness, a *app, label string) bool {
	t.Helper()
	for _, b := range a.buttons {
		if b.Label == label {
			return h.TapRect(b.Rect)
		}
	}
	t.Fatalf("no %q button; visible=%v", label, texts(h))
	return false
}

// --- Board size selection, all 3 sizes ---------------------------------------

func TestPlayGobanSizeSelection(t *testing.T) {
	for _, size := range []int{9, 13, 19} {
		size := size
		t.Run(itoa(size), func(t *testing.T) {
			h, a := bootToMenu(t)
			if size != 9 {
				selectSize(t, h, a, size)
			}
			startOpponent(t, h, a, game.OpponentHotseat)
			if a.gs.Board.Size() != size {
				t.Fatalf("board size = %d, want %d", a.gs.Board.Size(), size)
			}
		})
	}
}

// --- "Mot dator" is offered only for 9x9 -------------------------------------

func TestPlayGobanAIOnlyOfferedFor9x9(t *testing.T) {
	h, a := bootToMenu(t)
	if _, ok := h.FindText("Mot dator (svag)"); !ok {
		t.Fatalf("9x9 (default) should offer Mot dator; visible=%v", texts(h))
	}
	selectSize(t, h, a, 13)
	if _, ok := h.FindText("Mot dator (svag)"); ok {
		t.Fatal("13x13 must not offer Mot dator")
	}
	selectSize(t, h, a, 19)
	if _, ok := h.FindText("Mot dator (svag)"); ok {
		t.Fatal("19x19 must not offer Mot dator")
	}
	selectSize(t, h, a, 9)
	if _, ok := h.FindText("Mot dator (svag)"); !ok {
		t.Fatal("switching back to 9x9 should re-offer Mot dator")
	}
}

// --- GOTCHA: capture via a real tap -------------------------------------------

func TestPlayGobanCaptureViaTap(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	a.gs.Board.Set(image.Pt(4, 4), game.White)
	a.gs.Board.Set(image.Pt(3, 4), game.Black)
	a.gs.Board.Set(image.Pt(5, 4), game.Black)
	a.gs.Board.Set(image.Pt(4, 3), game.Black)
	a.gs.Turn = game.Black
	h.Draw()

	if !tapPoint(h, a, 4, 5) {
		t.Fatal("the capturing move should be legal")
	}
	if a.gs.Board.At(image.Pt(4, 4)) != game.Empty {
		t.Fatal("the surrounded White stone should have been captured")
	}
	if len(a.gs.LastCaptured) != 1 || a.gs.LastCaptured[0] != image.Pt(4, 4) {
		t.Fatalf("LastCaptured = %v, want exactly [(4,4)]", a.gs.LastCaptured)
	}
	if a.gs.Turn != game.White {
		t.Fatal("turn should pass to White after Black's move")
	}
}

// --- GOTCHA: ko rejection via real taps, and its lift after an intervening
// move elsewhere ---------------------------------------------------------------

func TestPlayGobanKoRejectionViaTaps(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	// The classic ko shape (see game/rules_test.go for the same diagram).
	a.gs.Board.Set(image.Pt(2, 0), game.Black)
	a.gs.Board.Set(image.Pt(3, 1), game.Black)
	a.gs.Board.Set(image.Pt(2, 2), game.Black)
	a.gs.Board.Set(image.Pt(1, 1), game.Black)
	a.gs.Board.Set(image.Pt(1, 0), game.White)
	a.gs.Board.Set(image.Pt(0, 1), game.White)
	a.gs.Board.Set(image.Pt(1, 2), game.White)
	a.gs.Turn = game.White
	h.Draw()

	if !tapPoint(h, a, 2, 1) {
		t.Fatal("White's capturing move should be legal")
	}
	if a.gs.Board.At(image.Pt(1, 1)) != game.Empty {
		t.Fatal("the captured Black stone should be gone")
	}
	if a.gs.Turn != game.Black {
		t.Fatal("turn should now be Black's")
	}

	blackBefore := a.gs.Board.Count(game.Black)
	if tapPoint(h, a, 1, 1) {
		t.Fatal("the immediate ko recapture must be rejected")
	}
	if a.gs.Board.Count(game.Black) != blackBefore {
		t.Fatal("a rejected ko tap must not change the board")
	}
	if a.gs.Turn != game.Black {
		t.Fatal("a rejected move must not advance the turn")
	}

	// Play elsewhere (Black), then elsewhere (White): the ko should lift.
	if !tapPoint(h, a, 7, 7) {
		t.Fatal("Black's elsewhere move should be legal")
	}
	if !tapPoint(h, a, 7, 8) {
		t.Fatal("White's elsewhere move should be legal")
	}
	if !tapPoint(h, a, 1, 1) {
		t.Fatal("the recapture should now be legal — the ko lifted after an intervening move")
	}
	if a.gs.Board.At(image.Pt(2, 1)) != game.Empty {
		t.Fatal("the recapture should have removed White's stone at (2,1)")
	}
}

// --- Passing: single pass keeps playing, double pass enters marking ---------

func TestPlayGobanSingleAndDoublePass(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	if !tapButton(t, h, a, "Passa") {
		t.Fatal("Black's pass should be accepted")
	}
	if a.gs.Phase != game.PhasePlaying {
		t.Fatal("a single pass must not end the game")
	}
	if _, ok := h.FindTextContains("Pass!"); !ok {
		t.Fatalf("a pass should be reflected in the status text; visible=%v", texts(h))
	}
	if a.gs.Turn != game.White {
		t.Fatal("turn should pass to White")
	}

	if !tapButton(t, h, a, "Passa") {
		t.Fatal("White's pass should be accepted")
	}
	if a.gs.Phase != game.PhaseMarking {
		t.Fatal("two consecutive passes should enter the mark-dead phase")
	}
	if _, ok := h.FindTextContains("Markera"); !ok {
		t.Fatalf("mark-dead status text missing; visible=%v", texts(h))
	}
}

// --- GOTCHA: mark-dead tap-to-toggle flow, verified against an
// independently computed area score ------------------------------------------

func TestPlayGobanMarkDeadAndFinalScore(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat) // default menu: 9x9, komi 6.5

	for y := 0; y < 9; y++ {
		a.gs.Board.Set(image.Pt(3, y), game.Black)
		a.gs.Board.Set(image.Pt(5, y), game.White)
	}
	// A doomed lone White stone sitting inside Black's area — never actually
	// captured over the board, but everyone agrees it's dead.
	a.gs.Board.Set(image.Pt(1, 4), game.White)
	a.gs.Turn = game.Black
	h.Draw()

	if !tapButton(t, h, a, "Passa") { // Black passes
		t.Fatal("Black's pass should be accepted")
	}
	if !tapButton(t, h, a, "Passa") { // White passes
		t.Fatal("White's pass should be accepted")
	}
	if a.gs.Phase != game.PhaseMarking {
		t.Fatal("expected the mark-dead phase")
	}

	if !tapPoint(h, a, 1, 4) {
		t.Fatal("tapping the doomed White stone should toggle it dead")
	}
	if !a.gs.Dead[image.Pt(1, 4)] {
		t.Fatal("the tapped stone should now be marked dead")
	}

	if !tapButton(t, h, a, "Klar") {
		t.Fatal("Klar should finalize the score")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Klar should move to PhaseDone")
	}

	// Independently recompute the expected score from the exact same
	// (board, dead-set, komi) via the game package directly, rather than
	// hand-typing the constant twice.
	wantBlack, wantWhite := game.AreaScore(a.gs.Board, a.gs.Dead, a.gs.Komi)
	if a.gs.BlackScore != wantBlack || a.gs.WhiteScore != wantWhite {
		t.Fatalf("final score (%v, %v) does not match independently computed AreaScore (%v, %v)",
			a.gs.BlackScore, a.gs.WhiteScore, wantBlack, wantWhite)
	}
	// And sanity-check the actual numbers: Black 9 stones + 27 territory;
	// White 9 stones + 27 territory + 6.5 komi.
	if wantBlack != 36 {
		t.Fatalf("BlackScore = %v, want 36", wantBlack)
	}
	if wantWhite != 42.5 {
		t.Fatalf("WhiteScore = %v, want 42.5", wantWhite)
	}
	if _, ok := h.FindTextContains("Vit vinner"); !ok {
		t.Fatalf("win banner not shown; visible=%v", texts(h))
	}
}

// --- Both opponent modes: 2 spelare on all sizes, Mot dator replies on 9x9 --

func TestPlayGobanHotseatAllSizes(t *testing.T) {
	for _, size := range []int{9, 13, 19} {
		size := size
		t.Run(itoa(size), func(t *testing.T) {
			h, a := bootToMenu(t)
			if size != 9 {
				selectSize(t, h, a, size)
			}
			startOpponent(t, h, a, game.OpponentHotseat)
			mid := size / 2
			if !tapPoint(h, a, mid, mid) {
				t.Fatal("a center move should be legal on an empty board")
			}
			if a.gs.Turn != game.White {
				t.Fatal("turn should pass to White")
			}
		})
	}
}

func TestPlayGobanAIRepliesOn9x9(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI)
	before := a.gs.Board.Count(game.White)
	if !tapPoint(h, a, 4, 4) {
		t.Fatal("Black's opening move should be legal")
	}
	if a.gs.AITurn() {
		t.Fatal("control returned on the AI's turn (deferred reply not drained)")
	}
	if a.gs.Board.Count(game.White) <= before && a.gs.ConsecutivePasses == 0 {
		t.Fatal("the AI (White) should have replied with either a stone or a pass")
	}
}

// --- Full game vs the AI, played to a real final score ----------------------

func TestPlayGobanFullGameVsAI(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI)

	start := time.Now()
	const maxPlies = 60
	humanMoves := 0
	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > maxPlies*2 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		// Deterministic scripted policy: play the same heuristic the AI
		// itself uses, capped at maxPlies human moves, then pass every
		// remaining turn so the AI (which mirrors a human pass) ends the
		// game via the double-pass rule.
		var acted bool
		if humanMoves < maxPlies {
			if p, ok := game.BestMove(a.gs.Board, a.gs.Turn, nil); ok {
				if tapPoint(h, a, p.X, p.Y) {
					humanMoves++
					acted = true
				}
			}
		}
		if !acted {
			if !tapButton(t, h, a, "Passa") {
				t.Fatalf("Black's pass at ply %d should be accepted", ply)
			}
		}
	}
	elapsed := time.Since(start)
	t.Logf("full 9x9 game vs AI: %d human plies, %v elapsed (%v/ply)",
		humanMoves, elapsed, elapsed/time.Duration(humanMoves+1))

	if a.gs.Phase != game.PhaseMarking {
		t.Fatalf("expected the mark-dead phase after the double pass, got %v", a.gs.Phase)
	}
	if !tapButton(t, h, a, "Klar") {
		t.Fatal("Klar should finalize the score")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("expected PhaseDone after Klar")
	}
	wantBlack, wantWhite := game.AreaScore(a.gs.Board, a.gs.Dead, a.gs.Komi)
	if a.gs.BlackScore != wantBlack || a.gs.WhiteScore != wantWhite {
		t.Fatalf("final score (%v,%v) doesn't match independent AreaScore (%v,%v)",
			a.gs.BlackScore, a.gs.WhiteScore, wantBlack, wantWhite)
	}
	total := a.gs.Board.Count(game.Black) + a.gs.Board.Count(game.White)
	if total == 0 {
		t.Fatal("a full game should have placed at least some stones")
	}
	want := "Oavgjort!"
	switch a.gs.Winner() {
	case game.Black:
		want = "Svart vinner!"
	case game.White:
		want = "Vit vinner!"
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("end banner %q not shown; visible=%v", want, texts(h))
	}
}

// --- Quit mid-game (Back key AND the Meny button) ----------------------------

func TestPlayGobanQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	tapPoint(h, a, 4, 4)

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}

	startOpponent(t, h, a, game.OpponentAI)
	if !tapButton(t, h, a, "Meny") {
		t.Fatal("Meny button should be present and tappable")
	}
	if a.screen != screenMenu {
		t.Fatalf("Meny button did not return to menu, screen=%v", a.screen)
	}
	// Menu still usable afterwards.
	startOpponent(t, h, a, game.OpponentHotseat)
}

// --- "Ny" restarts the current configuration mid-game -----------------------

func TestPlayGobanNyRestarts(t *testing.T) {
	h, a := bootToMenu(t)
	selectSize(t, h, a, 13)
	startOpponent(t, h, a, game.OpponentHotseat)
	tapPoint(h, a, 4, 4)
	if a.gs.Board.Count(game.Black) != 1 {
		t.Fatal("setup: expected a move to have been made")
	}
	if !tapButton(t, h, a, "Ny") {
		t.Fatal("Ny should be present and tappable")
	}
	if a.gs.Board.Size() != 13 {
		t.Fatal("Ny should preserve the board size")
	}
	if a.gs.Board.Count(game.Black) != 0 || a.gs.Turn != game.Black {
		t.Fatal("Ny should reset to a fresh empty board")
	}
}

// --- Rules screen -------------------------------------------------------------

func TestPlayGobanRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("Ko:"); !ok {
		t.Fatalf("rules text missing the ko rule; visible=%v", texts(h))
	}
	if _, ok := h.FindTextContains("komi"); !ok {
		t.Fatalf("rules text missing komi; visible=%v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayGobanScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/goban_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/goban_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/goban_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	// 19x19: the worst-case grid size in the whole library.
	selectSize(t, h, a, 19)
	startOpponent(t, h, a, game.OpponentHotseat)
	seq := [][2]int{{9, 9}, {3, 3}, {15, 15}, {3, 15}, {15, 3}, {9, 3}, {9, 15}, {3, 9}, {15, 9}}
	for _, xy := range seq {
		tapPoint(h, a, xy[0], xy[1])
	}
	if err := h.Screenshot(dir + "/goban_19x19_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// A capture happening: build a one-tap capture on a fresh 9x9 board.
	h.Back()
	selectSize(t, h, a, 9) // the 19x19 selection above otherwise persists
	startOpponent(t, h, a, game.OpponentHotseat)
	a.gs.Board.Set(image.Pt(4, 4), game.White)
	a.gs.Board.Set(image.Pt(3, 4), game.Black)
	a.gs.Board.Set(image.Pt(5, 4), game.Black)
	a.gs.Board.Set(image.Pt(4, 3), game.Black)
	a.gs.Turn = game.Black
	h.Draw()
	tapPoint(h, a, 4, 5)
	if err := h.Screenshot(dir + "/goban_capture.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// Mark-dead phase with some groups toggled.
	h.Back()
	startOpponent(t, h, a, game.OpponentHotseat)
	for y := 0; y < 9; y++ {
		a.gs.Board.Set(image.Pt(3, y), game.Black)
		a.gs.Board.Set(image.Pt(5, y), game.White)
	}
	a.gs.Board.Set(image.Pt(1, 4), game.White)
	a.gs.Board.Set(image.Pt(7, 2), game.Black)
	a.gs.Turn = game.Black
	h.Draw()
	tapButton(t, h, a, "Passa")
	tapButton(t, h, a, "Passa")
	tapPoint(h, a, 1, 4)
	tapPoint(h, a, 7, 2)
	if err := h.Screenshot(dir + "/goban_markdead.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// Final score screen.
	tapButton(t, h, a, "Klar")
	if err := h.Screenshot(dir + "/goban_final_score.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
