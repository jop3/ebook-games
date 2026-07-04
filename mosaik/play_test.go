//go:build playtest

package main

// Headless PLAYTHROUGH tests for Mosaik. They drive the real touch path —
// tapping a tile swatch (or center chip) to select tiles, then a pattern
// line or the floor to place them — and check gameplay against the rules as
// written (see rulesParagraphs in ui.go): drafting legality, floor
// overflow, the start marker's floor penalty, round-end wall-tiling with
// its per-tile points, the game-end trigger firing only after that round's
// tiling (with end bonuses applied exactly once), the "Visa motståndare"
// toggle, both game modes (2 spelare / Mot dator at all 3 difficulties),
// quitting, the rules screen, and a full multi-round game played out to a
// real final score. Runs under the pure-Go inkview emulator
// (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"mosaik/game"
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

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// sourceRectFor finds the tappable rect for a specific (source, color) tile
// swatch / center chip, as last drawn.
func sourceRectFor(a *app, source int, c game.Color) (image.Rectangle, bool) {
	for _, sh := range a.layout.sourceHits {
		if sh.source == source && sh.color == c {
			return sh.rect, true
		}
	}
	return image.Rectangle{}, false
}

// targetRectFor finds the tappable rect for a pattern-line row (0..4) or
// the floor (-1) on the currently full-size board, as last drawn.
func targetRectFor(a *app, target int) (image.Rectangle, bool) {
	for _, th := range a.layout.targetHits {
		if th.target == target {
			return th.rect, true
		}
	}
	return image.Rectangle{}, false
}

// playMoveViaTap drives mv through the real 2-tap UI flow: tap the source
// tile/chip, then tap the target line/floor.
func playMoveViaTap(t *testing.T, h *ink.Harness, a *app, mv game.Move) bool {
	t.Helper()
	sr, ok := sourceRectFor(a, mv.Source, mv.Color)
	if !ok {
		t.Fatalf("no tappable source rect for %v (visible sourceHits=%v)", mv, a.layout.sourceHits)
	}
	if !h.TapRect(sr) {
		return false
	}
	if !a.sel.active || a.sel.source != mv.Source || a.sel.color != mv.Color {
		t.Fatalf("selecting source tap did not set selection state as expected for %v, got %+v", mv, a.sel)
	}
	tr, ok := targetRectFor(a, mv.TargetLine)
	if !ok {
		t.Fatalf("no tappable target rect for line %d", mv.TargetLine)
	}
	return h.TapRect(tr)
}

// policyMove is a deterministic (fixed-algorithm) move policy usable for
// EITHER hot-seat side: Mosaik is perfect-information, so the same greedy
// evaluator the AI itself uses (see game/ai.go) makes a fine, always-
// terminating stand-in for "a human playing sensibly" — unlike always
// taking LegalMoves()[0], which (since the floor is always legal and is
// enumerated first) would just dump every draft onto the floor forever and
// never complete a single wall row.
func policyMove(a *app) (game.Move, bool) {
	return game.BestMove(a.gs, a.gs.Turn, game.DepthEasy)
}

func tapContinue(t *testing.T, h *ink.Harness) {
	t.Helper()
	if err := h.TapText("Fortsätt"); err != nil {
		t.Fatalf("no Fortsätt button: %v", err)
	}
}

// boardEqual compares two Boards by value (game.Board contains slices, so
// it isn't `==`-comparable).
func boardEqual(a, b game.Board) bool {
	if a.Score != b.Score || a.HasMarker != b.HasMarker || a.Wall != b.Wall {
		return false
	}
	if len(a.Floor) != len(b.Floor) {
		return false
	}
	for i := range a.Floor {
		if a.Floor[i] != b.Floor[i] {
			return false
		}
	}
	for i := range a.Lines {
		if len(a.Lines[i]) != len(b.Lines[i]) {
			return false
		}
		for j := range a.Lines[i] {
			if a.Lines[i][j] != b.Lines[i][j] {
				return false
			}
		}
	}
	return true
}

// wallColFor returns the wall column at `row` belonging to color c (the
// inverse of game.ColorAt, computed by brute force since wallColOf itself
// isn't exported).
func wallColFor(row int, c game.Color) int {
	for col := 0; col < game.WallSize; col++ {
		if game.ColorAt(row, col) == c {
			return col
		}
	}
	return -1
}

// --- RULE: drafting selection + placement, via real taps --------------------

func TestPlayMosaikBasicDraftAndPlace(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	a.gs.Factories[0] = []game.Color{game.ColorSolid, game.ColorRing, game.ColorRing, game.ColorDot}
	h.Draw()

	if !playMoveViaTap(t, h, a, game.Move{Source: 0, Color: game.ColorRing, TargetLine: 1}) {
		t.Fatal("taking 2 Ring tiles onto line 1 (cap 2) should be legal")
	}
	b := &a.gs.Boards[0]
	if len(b.Lines[1]) != 2 {
		t.Fatalf("line 1 should hold 2 Ring tiles, got %d", len(b.Lines[1]))
	}
	if countColorUI(a.gs.Center, game.ColorSolid) != 1 || countColorUI(a.gs.Center, game.ColorDot) != 1 {
		t.Fatalf("the un-drafted Solid+Dot tiles should be in the center, center=%v", a.gs.Center)
	}
	if a.gs.Turn != 1 {
		t.Fatalf("turn should have passed to player 1, got %d", a.gs.Turn)
	}
}

// Tapping the same selection again cancels it (no move applied).
func TestPlayMosaikTapSameSelectionCancels(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)
	a.gs.Factories[0] = []game.Color{game.ColorSolid, game.ColorSolid, game.ColorRing, game.ColorDot}
	h.Draw()

	sr, ok := sourceRectFor(a, 0, game.ColorSolid)
	if !ok {
		t.Fatal("no Solid swatch")
	}
	h.TapRect(sr)
	if !a.sel.active {
		t.Fatal("first tap should select")
	}
	h.TapRect(sr)
	if a.sel.active {
		t.Fatal("tapping the same swatch again should cancel the selection")
	}
	if a.gs.Turn != 0 {
		t.Fatal("cancelling a selection must not consume a turn")
	}
}

// GOTCHA (overflow): taking more tiles than a pattern line has room for
// routes the extras to the floor — via real taps.
func TestPlayMosaikOverflowToFloorViaTaps(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	a.gs.Factories[0] = []game.Color{game.ColorSolid, game.ColorSolid, game.ColorSolid, game.ColorRing}
	h.Draw()

	if !playMoveViaTap(t, h, a, game.Move{Source: 0, Color: game.ColorSolid, TargetLine: 0}) {
		t.Fatal("taking 3 Solid tiles onto line 0 (cap 1) should be legal, with overflow")
	}
	b := &a.gs.Boards[0]
	if len(b.Lines[0]) != 1 {
		t.Fatalf("line 0 should hold exactly 1 tile, got %d", len(b.Lines[0]))
	}
	if len(b.Floor) != 2 {
		t.Fatalf("2 Solid tiles should have overflowed to the floor, got %d", len(b.Floor))
	}
}

// GOTCHA (start marker): claiming from the center for the first time this
// round also claims the marker, which lands on the floor and penalizes —
// via a real tap.
func TestPlayMosaikStartMarkerViaTaps(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	a.gs.Center = []game.Color{game.ColorRing, game.ColorRing}
	a.gs.CenterHasStart = true
	h.Draw()

	if !playMoveViaTap(t, h, a, game.Move{Source: -1, Color: game.ColorRing, TargetLine: 2}) {
		t.Fatal("taking 2 Ring tiles from the center onto line 2 (cap 3) should be legal")
	}
	b := &a.gs.Boards[0]
	if !b.HasMarker {
		t.Fatal("the start marker should now be on player 0's floor line")
	}
	if a.gs.CenterHasStart {
		t.Fatal("the marker must be claimed")
	}
	if a.gs.StartNext != 0 {
		t.Fatalf("player 0 claimed the marker, should start next round, got %d", a.gs.StartNext)
	}
}

// Illegal target (a different color already committed to that line) is
// rejected via a real tap, with no state change.
func TestPlayMosaikIllegalTargetRejected(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	a.gs.Boards[0].Lines[2] = []game.Color{game.ColorDot}
	a.gs.Factories[0] = []game.Color{game.ColorSolid, game.ColorRing, game.ColorRing, game.ColorRing}
	h.Draw()

	before := a.gs.Boards[0]
	sr, ok := sourceRectFor(a, 0, game.ColorRing)
	if !ok {
		t.Fatal("no Ring swatch")
	}
	h.TapRect(sr)
	tr, ok := targetRectFor(a, 2) // line 2 is committed to Dot: Ring must be rejected
	if !ok {
		t.Fatal("no target rect for line 2")
	}
	if h.TapRect(tr) {
		t.Fatal("placing Ring on a Dot-committed line must be rejected")
	}
	if !boardEqual(a.gs.Boards[0], before) {
		t.Fatal("a rejected placement must not mutate the board")
	}
	if _, ok := h.FindTextContains("Ogiltigt drag"); !ok {
		t.Fatalf("an illegal placement should surface a hint; visible=%v", texts(h))
	}
}

// --- Round-end wall-tiling, shown as its own screen with per-tile points ---

func TestPlayMosaikRoundEndTilingScreen(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	for i := range a.gs.Factories {
		a.gs.Factories[i] = nil
	}
	a.gs.Center = []game.Color{game.ColorDot}
	a.gs.CenterHasStart = false
	h.Draw()

	if !playMoveViaTap(t, h, a, game.Move{Source: -1, Color: game.ColorDot, TargetLine: 0}) {
		t.Fatal("taking the last Dot tile onto line 0 should be legal and end the round")
	}
	if a.gs.Phase != game.PhaseTiling {
		t.Fatalf("Phase = %v, want PhaseTiling", a.gs.Phase)
	}
	if _, ok := h.FindTextContains("plattor på väggen"); !ok {
		t.Fatalf("tiling screen title not shown; visible=%v", texts(h))
	}
	if _, ok := h.FindTextContains("Prickig"); !ok {
		t.Fatalf("the scored Dot placement should be named on the tiling screen; visible=%v", texts(h))
	}
	if _, ok := h.FindTextContains("+1p"); !ok {
		t.Fatalf("the isolated placement should show +1p; visible=%v", texts(h))
	}

	tapContinue(t, h)
	if a.gs.Phase != game.PhasePlaying {
		t.Fatalf("Phase = %v, want PhasePlaying after Fortsätt (no row completed)", a.gs.Phase)
	}
	if a.gs.RoundNum != 1 {
		t.Fatalf("RoundNum = %d, want 1", a.gs.RoundNum)
	}
}

// --- GOTCHA: game-end fires only after that round's tiling, and end bonuses
// are applied exactly once — driven via real taps all the way to the
// winner banner. ------------------------------------------------------------

func TestPlayMosaikGameEndBonusOnceThenWinner(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	completerColor := game.ColorCross
	col := wallColFor(4, completerColor)
	if col < 0 {
		t.Fatal("setup bug: could not find a wall column for the completer color")
	}
	for c := 0; c < game.WallSize; c++ {
		if c != col {
			a.gs.Boards[0].Wall[4][c] = true
		}
	}
	a.gs.Boards[0].Lines[4] = []game.Color{completerColor, completerColor, completerColor, completerColor, completerColor}

	for i := range a.gs.Factories {
		a.gs.Factories[i] = nil
	}
	a.gs.Center = []game.Color{game.ColorRing}
	a.gs.CenterHasStart = false
	a.gs.Turn = 0
	h.Draw()

	if !playMoveViaTap(t, h, a, game.Move{Source: -1, Color: game.ColorRing, TargetLine: 1}) {
		t.Fatal("the round's last draft action should be legal")
	}
	if a.gs.Phase != game.PhaseTiling {
		t.Fatalf("Phase = %v, want PhaseTiling immediately (game-end must not fire before tiling)", a.gs.Phase)
	}

	tapContinue(t, h)
	if a.gs.Phase != game.PhaseBonus {
		t.Fatalf("Phase = %v, want PhaseBonus (a wall row completed this round)", a.gs.Phase)
	}
	if _, ok := h.FindTextContains("bonuspoäng"); !ok {
		t.Fatalf("bonus screen title not shown; visible=%v", texts(h))
	}
	scoreAfterBonus := a.gs.Boards[0].Score

	tapContinue(t, h)
	if a.gs.Phase != game.PhaseDone {
		t.Fatalf("Phase = %v, want PhaseDone", a.gs.Phase)
	}
	if a.gs.Boards[0].Score != scoreAfterBonus {
		t.Fatal("end bonuses must be applied exactly once")
	}
	want := winnerText(a.gs)
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("winner banner %q not shown; visible=%v", want, texts(h))
	}
}

// --- "Visa motståndare" toggle -----------------------------------------------

func TestPlayMosaikShowOpponentToggle(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	if a.showOpp {
		t.Fatal("should start with the toggle off")
	}
	if a.fullBoardSide() != a.gs.Turn {
		t.Fatal("full board should be the side to move by default")
	}
	btn := a.layout.ShowOppBtn
	if !h.TapRect(btn) {
		t.Fatal("the toggle button should be tappable")
	}
	if !a.showOpp {
		t.Fatal("toggle should now be on")
	}
	if a.fullBoardSide() != 1-a.gs.Turn {
		t.Fatal("full board should now show the OTHER side while toggled on")
	}
	// While toggled on, drafting taps must be inert (read-only view).
	a.gs.Factories[0] = []game.Color{game.ColorSolid, game.ColorSolid, game.ColorRing, game.ColorDot}
	h.Draw()
	if sr, ok := sourceRectFor(a, 0, game.ColorSolid); ok && h.TapRect(sr) {
		t.Fatal("tapping a factory swatch while showing the opponent must be a no-op")
	}

	if !h.TapRect(a.layout.ShowOppBtn) {
		t.Fatal("toggling back off should be tappable")
	}
	if a.showOpp {
		t.Fatal("toggle should now be off again")
	}
}

// --- Both game modes, all 3 AI difficulties ---------------------------------

func TestPlayMosaikHotseatMode(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)
	mv, ok := policyMove(a)
	if !ok {
		t.Fatal("no legal opening move")
	}
	if !playMoveViaTap(t, h, a, mv) {
		t.Fatal("a legal opening move should be accepted")
	}
	if a.gs.AITurn() {
		t.Fatal("hot-seat mode should never report an AI turn")
	}
}

// GOTCHA: the AI's reply is computed AFTER the human's move is drawn (so the
// player sees their own move land first) but still within the SAME tap call
// — Draw()'s aiPend mechanism is drained synchronously by the emulator's
// drainRepeat before h.TapRect returns. So by the time control comes back to
// this test, the AI (player 1) has already moved and it's the human's turn
// again — there is no observable window where AITurn() reads true from the
// caller's side. (This matches othello/hasami/munkar's own
// AllDifficultiesReply tests, which check the same thing the same way.)
func TestPlayMosaikAllDifficultiesReply(t *testing.T) {
	for _, diff := range []int{game.DepthEasy, game.DepthMedium, game.DepthHard} {
		diff := diff
		t.Run(itoa(diff), func(t *testing.T) {
			h, a := bootToMenu(t)
			startMode(t, h, a, game.ModeAI, diff)
			if a.gs.AILevel != diff {
				t.Fatalf("AILevel = %d, want %d", a.gs.AILevel, diff)
			}
			before := a.gs.Boards[1]
			mv, ok := policyMove(a)
			if !ok {
				t.Fatal("no legal opening move for the human")
			}
			if !playMoveViaTap(t, h, a, mv) {
				t.Fatal("the human's opening move should be legal")
			}
			if a.gs.AITurn() {
				t.Fatal("control returned on the AI's turn (deferred reply not drained)")
			}
			if a.gs.Phase == game.PhasePlaying && boardEqual(a.gs.Boards[1], before) {
				t.Fatal("the AI (player 1) never replied")
			}
		})
	}
}

// --- Quit mid-game (Back key AND the Meny button) ---------------------------

func TestPlayMosaikQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)
	mv, _ := policyMove(a)
	playMoveViaTap(t, h, a, mv)

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
	startMode(t, h, a, game.ModeHotseat, 0) // menu still usable afterwards
}

// --- Rules screen ------------------------------------------------------------

func TestPlayMosaikRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("Baserat på Azul"); !ok {
		t.Fatalf("rules text missing the original-game credit; visible: %v", texts(h))
	}
	if _, ok := h.FindTextContains("-1,-1,-2,-2,-2,-3,-3"); !ok {
		t.Fatalf("rules text missing the explicit floor penalty track; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- A full multi-round game, played to a real final score ------------------

func TestPlayMosaikFullHotseatGame(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	finished := false
	for iter := 0; iter < 3000 && !finished; iter++ {
		switch a.gs.Phase {
		case game.PhasePlaying:
			mv, ok := policyMove(a)
			if !ok {
				t.Fatalf("no legal move at iter %d", iter)
			}
			if !playMoveViaTap(t, h, a, mv) {
				t.Fatalf("policy move %v rejected at iter %d", mv, iter)
			}
		case game.PhaseTiling, game.PhaseBonus:
			tapContinue(t, h)
		case game.PhaseDone:
			finished = true
		}
	}
	if !finished {
		t.Fatal("game did not reach PhaseDone within the iteration budget")
	}
	if a.gs.RoundNum < 1 {
		t.Fatal("a real multi-round game should have played at least one full round")
	}
	want := winnerText(a.gs)
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("winner banner %q not shown; visible=%v", want, texts(h))
	}
	t.Logf("full game finished after %d rounds, final score %d–%d, %s",
		a.gs.RoundNum+1, a.gs.Boards[0].Score, a.gs.Boards[1].Score, want)
}

// --- Screenshots of every screen for visual review --------------------------

// setupNearFullBoards engineers a busy, representative mid-game position on
// both boards (several pattern lines at various fill levels, scattered wall
// tiles including adjacency, a non-empty floor with the marker) plus varied
// factories/center, so the screenshot actually exercises the dense layout
// instead of a mostly-empty starting position.
func setupNearFullBoards(a *app) {
	a.gs.Factories[0] = []game.Color{game.ColorSolid, game.ColorSolid, game.ColorRing, game.ColorDot}
	a.gs.Factories[1] = []game.Color{game.ColorCross, game.ColorCross, game.ColorStripe, game.ColorStripe}
	a.gs.Factories[2] = []game.Color{game.ColorDot, game.ColorDot, game.ColorDot, game.ColorSolid}
	a.gs.Factories[3] = []game.Color{game.ColorRing, game.ColorStripe, game.ColorCross, game.ColorDot}
	a.gs.Factories[4] = []game.Color{game.ColorSolid, game.ColorRing, game.ColorRing, game.ColorCross}
	a.gs.Center = []game.Color{game.ColorStripe, game.ColorDot}
	a.gs.CenterHasStart = true

	b0 := &a.gs.Boards[0]
	b0.Lines[0] = []game.Color{game.ColorSolid}
	b0.Lines[1] = []game.Color{game.ColorRing}
	b0.Lines[2] = []game.Color{game.ColorCross, game.ColorCross}
	b0.Lines[3] = []game.Color{game.ColorDot, game.ColorDot, game.ColorDot}
	b0.Wall[0][wallColFor(0, game.ColorStripe)] = true
	b0.Wall[1][wallColFor(1, game.ColorStripe)] = true
	b0.Wall[0][wallColFor(0, game.ColorDot)] = true
	b0.Floor = []game.Color{game.ColorSolid, game.ColorRing}
	b0.HasMarker = true
	b0.Score = 7

	b1 := &a.gs.Boards[1]
	b1.Lines[1] = []game.Color{game.ColorDot, game.ColorDot}
	b1.Lines[4] = []game.Color{game.ColorSolid, game.ColorSolid, game.ColorSolid}
	b1.Wall[2][wallColFor(2, game.ColorRing)] = true
	b1.Wall[3][wallColFor(3, game.ColorRing)] = true
	b1.Wall[2][wallColFor(2, game.ColorSolid)] = true
	b1.Score = 5
}

func TestPlayMosaikScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	shot := func(name string) {
		t.Helper()
		if err := h.Screenshot(dir + "/" + name); err != nil {
			t.Fatalf("screenshot %s: %v", name, err)
		}
	}

	shot("mosaik_splash.png")
	h.TapXY(500, 700)
	shot("mosaik_menu.png")
	h.TapText("Regler")
	shot("mosaik_rules.png")
	h.Back()

	startMode(t, h, a, game.ModeAI, game.DepthMedium)
	setupNearFullBoards(a)
	h.Draw()
	shot("mosaik_board_midgame.png")

	if !h.TapRect(a.layout.ShowOppBtn) {
		t.Fatal("toggle button should be tappable")
	}
	shot("mosaik_board_show_opponent.png")
	h.TapRect(a.layout.ShowOppBtn) // toggle back off

	// Round-end wall-tiling reveal: clear the draft pool down to one final
	// legal move so the tap that lands it also ends the round, tiling both
	// boards' already-set-up full lines.
	for i := range a.gs.Factories {
		a.gs.Factories[i] = nil
	}
	a.gs.Center = []game.Color{game.ColorSolid}
	a.gs.CenterHasStart = false
	h.Draw()
	if !playMoveViaTap(t, h, a, game.Move{Source: -1, Color: game.ColorSolid, TargetLine: 4}) {
		t.Fatal("the round's last draft action should be legal")
	}
	shot("mosaik_round_end_tiling.png")

	// End-game bonus screen + winner banner: engineer a completed wall row
	// on top of the tiling that already ran, then continue through.
	col := wallColFor(4, game.ColorCross)
	for c := 0; c < game.WallSize; c++ {
		if c != col {
			a.gs.Boards[0].Wall[4][c] = true
		}
	}
	a.gs.Phase = game.PhaseBonus
	// Continue() would normally compute this via the PhaseTiling ->
	// PhaseBonus transition; set it directly since we jumped the phase
	// here purely to get a representative bonus-screen screenshot.
	total := 2
	a.gs.Bonuses[0] = game.Bonuses{Rows: 1, Cols: 0, Colors: 0, Total: total}
	a.gs.Boards[0].Score += total
	h.Draw()
	shot("mosaik_bonus.png")

	tapContinue(t, h)
	shot("mosaik_winner.png")
}
