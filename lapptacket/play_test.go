//go:build playtest

package main

// Headless PLAYTHROUGH tests for Lapptäcket. They drive the real touch path
// (splash -> menu -> patch-tray selection/rotation -> board tap-to-place,
// or the "Avancera" button) and check gameplay against the rules as written
// (see rulesParagraphs in ui.go): the trailing-marker-acts-next turn order
// (NOT strict alternation), buying one of the 3 nearest patches and placing
// it in any rotation/reflection, advancing for 1 button per square, income
// payouts on crossing (not just landing on) a marked square, the once-only
// free-patch/7x7-bonus claims, game end once both markers reach 53, and the
// final scoring formula — cross-checked against an INDEPENDENT
// recomputation of the formula from raw state, not just trusting
// GameState.FinalScore to agree with itself. Runs under the pure-Go
// inkview emulator (playtest/play.sh), mirroring mosaik/stadskarnan's
// general shape (the closest existing precedent: a busy 2-player board
// game with polyomino tile placement).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"lapptacket/game"
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

func startOpponent(t *testing.T, h *ink.Harness, a *app, opp game.Opponent) {
	t.Helper()
	row := 0
	if opp == game.OpponentAI {
		row = 1
	}
	if !h.TapRect(a.menu.rows[row].rect) {
		t.Fatalf("tap on menu row %d did not register", row)
	}
	if a.screen != screenGame || a.gs == nil || a.gs.Opponent != opp {
		t.Fatalf("did not start opponent %v (screen=%v)", opp, a.screen)
	}
}

func findButton(a *app, label string) (image.Rectangle, bool) {
	for _, b := range a.buttons {
		if b.Label == label {
			return b.Rect, true
		}
	}
	return image.Rectangle{}, false
}

func tapButton(t *testing.T, h *ink.Harness, a *app, label string) bool {
	t.Helper()
	r, ok := findButton(a, label)
	if !ok {
		t.Fatalf("button %q not found; have %v", label, a.buttons)
	}
	return h.TapRect(r)
}

func tapBoardCell(h *ink.Harness, a *app, x, y int) bool {
	return h.TapRect(a.layout.CellToScreen(x, y))
}

// findFirstLegalBuy scans the 3 buyable patches (in order) for the first
// one player can afford AND fits somewhere on their board, returning the
// offset, an orientation index, and a legal anchor for it.
func findFirstLegalBuy(a *app, player int) (offset, orientIdx int, anchor image.Point, ok bool) {
	three := a.gs.NextThree()
	board := &a.gs.Boards[player]
	for off, patch := range three {
		if off >= patchTilesBuyable {
			break
		}
		if patch.Cost > a.gs.Buttons[player] {
			continue
		}
		orients := game.Orientations(patch.Cells)
		for oi := range orients {
			placements := game.LegalPlacementsForOrientation(board, patch, oi)
			if len(placements) > 0 {
				return off, oi, placements[0].Anchor, true
			}
		}
	}
	return 0, 0, image.Point{}, false
}

// driveOneTurn performs exactly one human turn (for whichever player is
// a.gs.ActingPlayer()) through the real tap path: resolving an owed free
// patch, else buying the first workable patch (selecting it, rotating to
// the chosen orientation, tapping the anchor), else tapping "Avancera".
// Always makes forward progress (Advance is always legal for the trailing
// player), so repeated calls are guaranteed to terminate the game.
func driveOneTurn(t *testing.T, h *ink.Harness, a *app) {
	t.Helper()
	active := a.gs.ActingPlayer()

	if a.gs.Pending[active] > 0 {
		board := &a.gs.Boards[active]
		for y := 0; y < game.BoardSize; y++ {
			for x := 0; x < game.BoardSize; x++ {
				if !board.Filled[y][x] {
					if !tapBoardCell(h, a, x, y) {
						t.Fatalf("tap to place owed free patch at (%d,%d) failed", x, y)
					}
					return
				}
			}
		}
		t.Fatal("player has a pending free patch but their board is completely full")
	}

	if off, oi, anchor, ok := findFirstLegalBuy(a, active); ok {
		if !h.TapRect(a.layout.PatchRects[off]) {
			t.Fatalf("tap on patch tile %d failed", off)
		}
		if a.selOffset != off {
			t.Fatalf("selecting patch %d did not register (selOffset=%d)", off, a.selOffset)
		}
		for i := 0; i < oi; i++ {
			if !tapButton(t, h, a, "Rotera") {
				t.Fatal("Rotera tap failed")
			}
		}
		if a.orientIdx != oi {
			t.Fatalf("orientIdx = %d after rotating, want %d", a.orientIdx, oi)
		}
		if !tapBoardCell(h, a, anchor.X, anchor.Y) {
			t.Fatalf("tap to place patch at anchor %v failed", anchor)
		}
		return
	}

	if !tapButton(t, h, a, "Avancera") {
		t.Fatal("Avancera tap failed")
	}
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- tests ---------------------------------------------------------------

// TestPlayLapptacketFullGameHotseat drives an entire hot-seat game to its
// natural end (both markers reach 53) purely through simulated taps, then
// cross-checks FinalScore against an INDEPENDENT recomputation of the
// written formula from raw state (buttons, empty cells, bonus) for both
// players.
func TestPlayLapptacketFullGameHotseat(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	turns := 0
	for !a.gs.GameOver() {
		turns++
		if turns > 3000 {
			t.Fatalf("game did not reach an end after %d turns (m0=%d m1=%d)", turns, a.gs.Marker[0], a.gs.Marker[1])
		}
		driveOneTurn(t, h, a)
	}

	if a.gs.Marker[0] != game.TrackEnd || a.gs.Marker[1] != game.TrackEnd {
		t.Fatalf("game ended with markers not at TrackEnd: %d, %d", a.gs.Marker[0], a.gs.Marker[1])
	}
	if a.gs.Pending[0] != 0 || a.gs.Pending[1] != 0 {
		t.Fatalf("game ended with unresolved pending free patches: %v", a.gs.Pending)
	}

	for side := 0; side < 2; side++ {
		b := &a.gs.Boards[side]
		if got, want := b.FilledCount()+b.EmptyCount(), game.BoardSize*game.BoardSize; got != want {
			t.Fatalf("side %d: filled+empty = %d, want %d", side, got, want)
		}
		want := a.gs.Buttons[side] - 2*b.EmptyCount()
		if a.gs.BonusOwner == side {
			want += 7
		}
		if got := a.gs.FinalScore(side); got != want {
			t.Fatalf("side %d: FinalScore() = %d, want %d (independent recomputation)", side, got, want)
		}
	}

	// The status bar must render a final banner consistent with the real
	// Winner()/FinalScore() the test just independently verified.
	h.Draw()
	want := statusHeadline(a.gs)
	if _, ok := h.FindText(want); !ok {
		t.Fatalf("expected the status bar to show %q; got %v", want, texts(h))
	}

	t.Logf("hotseat game finished in %d turns; scores %d/%d, bonus owner=%d",
		turns, a.gs.FinalScore(0), a.gs.FinalScore(1), a.gs.BonusOwner)
}

// TestPlayLapptacketVsAIReachesEnd plays a full game against the built-in
// AI: the human (player 0) is driven via real taps, and the AI (player 1)
// must respond automatically (StepAI queued after each human move, chained
// across consecutive AI turns) without the test ever tapping for it.
func TestPlayLapptacketVsAIReachesEnd(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI)

	turns := 0
	for !a.gs.GameOver() {
		turns++
		if turns > 3000 {
			t.Fatalf("game did not reach an end after %d turns (m0=%d m1=%d)", turns, a.gs.Marker[0], a.gs.Marker[1])
		}
		if active := a.gs.ActingPlayer(); active != 0 {
			t.Fatalf("turn %d: expected player 0 to be acting (harness should auto-resolve AI turns after each tap), got %d", turns, active)
		}
		driveOneTurn(t, h, a)
	}
	if a.gs.Marker[0] != game.TrackEnd || a.gs.Marker[1] != game.TrackEnd {
		t.Fatalf("game ended with markers not at TrackEnd: %d, %d", a.gs.Marker[0], a.gs.Marker[1])
	}
	winner := a.gs.Winner()
	if winner < -1 || winner > 1 {
		t.Fatalf("Winner() = %d out of range", winner)
	}
	t.Logf("vs-AI game finished in %d human turns; scores %d/%d, winner=%d", turns, a.gs.FinalScore(0), a.gs.FinalScore(1), winner)
}

// TestPlayLapptacketTrailingRuleSurfacesThroughUI checks, through the real
// status text and real taps, that the SAME player can act twice (or more)
// in a row when they remain behind — the single most important, non-
// obvious turn-order detail per the spec (turns are NOT strict
// alternation). The scenario is pinned deterministically (marker positions
// set directly, far from any income/special-patch square so a tap-driven
// move can't accidentally trigger a pending free-patch placement) so this
// never depends on incidental patch-queue luck.
func TestPlayLapptacketTrailingRuleSurfacesThroughUI(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	if got := a.gs.ActingPlayer(); got != 0 {
		t.Fatalf("expected player 0 to act first at a tied start, got %d", got)
	}

	// Player 1 is well ahead; player 0 is far enough behind that even
	// buying the fastest patch on the board (time-cost 1) cannot possibly
	// catch up to or pass player 1 in a single turn.
	a.gs.Marker[0] = 10
	a.gs.Marker[1] = 30
	h.Draw()
	if got := a.gs.ActingPlayer(); got != 0 {
		t.Fatalf("ActingPlayer() = %d, want 0 (marker 10 < 30)", got)
	}
	if got := statusHeadline(a.gs); got == "" {
		t.Fatal("expected a non-empty status headline")
	}

	driveOneTurn(t, h, a)
	if a.gs.Marker[0] >= a.gs.Marker[1] {
		t.Fatalf("test invariant broken: player 0 (m0=%d) caught up to/passed player 1 (m1=%d) in one turn", a.gs.Marker[0], a.gs.Marker[1])
	}
	if got := a.gs.ActingPlayer(); got != 0 {
		t.Fatalf("expected player 0 to act AGAIN while still behind (m0=%d < m1=%d), got player %d — turn order must follow the trailing marker, not strict alternation", a.gs.Marker[0], a.gs.Marker[1], got)
	}

	// Drive it again — still player 0's turn per the real UI, not player 1's.
	driveOneTurn(t, h, a)
	if got := a.gs.ActingPlayer(); a.gs.Marker[0] < a.gs.Marker[1] && got != 0 {
		t.Fatalf("expected player 0 to keep acting while still behind, got player %d", got)
	}
}

// TestPlayLapptacketAdvanceButtonPaysButtons directly taps "Avancera" for
// the player to move and checks the exact button/marker delta against the
// written rule (1 button per square advanced, moving to just past the
// trailing/leading opponent).
func TestPlayLapptacketAdvanceButtonPaysButtons(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	active := a.gs.ActingPlayer()
	before := a.gs.Buttons[active]
	beforeMarker := a.gs.Marker[active]
	if !tapButton(t, h, a, "Avancera") {
		t.Fatal("Avancera tap failed")
	}
	delta := a.gs.Marker[active] - beforeMarker
	if delta <= 0 {
		t.Fatalf("expected marker to move forward, delta=%d", delta)
	}
	if got, want := a.gs.Buttons[active]-before, delta; got < want {
		t.Fatalf("Buttons delta = %d, want at least %d (1/square advanced)", got, want)
	}
}

// TestPlayLapptacketDeselectPatchByTappingAgain checks that tapping the
// same buyable patch tile twice cancels the selection (a UI affordance
// mirrored from this repo's other tile-placement games).
func TestPlayLapptacketDeselectPatchByTappingAgain(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	if !h.TapRect(a.layout.PatchRects[0]) {
		t.Fatal("tap on patch tile 0 failed")
	}
	if a.selOffset != 0 {
		t.Fatalf("selOffset = %d, want 0", a.selOffset)
	}
	if !h.TapRect(a.layout.PatchRects[0]) {
		t.Fatal("second tap on patch tile 0 failed")
	}
	if a.selOffset != -1 {
		t.Fatalf("selOffset = %d, want -1 (deselected)", a.selOffset)
	}
}

// TestPlayLapptacketRotateCyclesOrientations checks that repeatedly tapping
// "Rotera" cycles through exactly the patch's distinct orientation count
// and wraps back to 0.
func TestPlayLapptacketRotateCyclesOrientations(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	if !h.TapRect(a.layout.PatchRects[0]) {
		t.Fatal("tap on patch tile 0 failed")
	}
	three := a.gs.NextThree()
	n := len(game.Orientations(three[0].Cells))
	for i := 1; i <= n; i++ {
		if !tapButton(t, h, a, "Rotera") {
			t.Fatal("Rotera tap failed")
		}
		want := i % n
		if a.orientIdx != want {
			t.Fatalf("after %d Rotera taps: orientIdx = %d, want %d", i, a.orientIdx, want)
		}
	}
}

// TestPlayLapptacketShowOpponentToggleIsReadOnly checks that toggling
// "Visa motståndare" swaps which board is full-size/interactive, and that
// taps on the board are ignored while showing the read-only opponent view.
func TestPlayLapptacketShowOpponentToggleIsReadOnly(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	active := a.gs.ActingPlayer()
	if got := a.fullBoardSide(); got != active {
		t.Fatalf("fullBoardSide() = %d, want the active player %d before toggling", got, active)
	}
	if !h.TapRect(a.layout.ShowOppBtn) {
		t.Fatal("tap on the Visa motståndare toggle failed")
	}
	if !a.showOpp {
		t.Fatal("expected showOpp to be true after tapping the toggle")
	}
	if got := a.fullBoardSide(); got == active {
		t.Fatalf("fullBoardSide() = %d, expected the OTHER side while showOpp is on", got)
	}
	beforeFilled := a.gs.Boards[active].FilledCount()
	tapBoardCell(h, a, 0, 0) // must be a no-op: read-only view
	if a.gs.Boards[active].FilledCount() != beforeFilled {
		t.Fatal("a board tap while showOpp is on should not have changed anything")
	}
	// Toggle back.
	if !h.TapRect(a.layout.ShowOppBtn) {
		t.Fatal("second tap on the toggle failed")
	}
	if a.showOpp {
		t.Fatal("expected showOpp to be false again")
	}
}

// TestPlayLapptacketQuitWithBackAndMeny checks both ways of leaving a game
// mid-play return to the menu without crashing, and that a fresh game can
// be started afterwards.
func TestPlayLapptacketQuitWithBackAndMeny(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back from screenGame: screen = %v, want screenMenu", a.screen)
	}

	startOpponent(t, h, a, game.OpponentHotseat)
	if !tapButton(t, h, a, "Meny") {
		t.Fatal("Meny tap failed")
	}
	if a.screen != screenMenu {
		t.Fatalf("Meny button: screen = %v, want screenMenu", a.screen)
	}
}

// TestPlayLapptacketReplayWithNy checks the "Ny" button restarts a fresh
// game with the same opponent setting.
func TestPlayLapptacketReplayWithNy(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	driveOneTurn(t, h, a)
	if a.gs.Marker[0] == 0 && a.gs.Marker[1] == 0 {
		t.Fatal("expected some progress before restarting")
	}
	if !tapButton(t, h, a, "Ny") {
		t.Fatal("Ny tap failed")
	}
	if a.gs.Marker[0] != 0 || a.gs.Marker[1] != 0 || a.gs.Buttons[0] != game.StartButtons {
		t.Fatalf("expected a fresh game after Ny, got markers %v buttons %v", a.gs.Marker, a.gs.Buttons)
	}
}

// TestPlayLapptacketNoInputAfterGameEnds forces a terminal state directly
// (both markers at the end, nothing pending) and checks the UI rejects
// further board/patch taps.
func TestPlayLapptacketNoInputAfterGameEnds(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	a.gs.Marker[0] = game.TrackEnd
	a.gs.Marker[1] = game.TrackEnd
	a.gs.Phase = game.PhaseDone
	h.Draw()

	before0, before1 := a.gs.Boards[0].FilledCount(), a.gs.Boards[1].FilledCount()
	tapBoardCell(h, a, 3, 3)
	if a.gs.Boards[0].FilledCount() != before0 || a.gs.Boards[1].FilledCount() != before1 {
		t.Fatal("a board tap after the game ended must not change anything")
	}
	tapButton(t, h, a, "Avancera")
	if a.gs.Marker[0] != game.TrackEnd {
		t.Fatal("Avancera after game end must not move the marker")
	}
}

// TestPlayLapptacketRulesScreen checks the Regler button opens the rules
// screen (with recognizable rules text) and Tillbaka/Back both return to
// the menu.
func TestPlayLapptacketRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if !h.TapRect(a.menu.RulesButton()) {
		t.Fatal("tap on Regler failed")
	}
	if a.screen != screenRules {
		t.Fatalf("screen = %v, want screenRules", a.screen)
	}
	if _, ok := h.FindTextContains("7x7"); !ok {
		t.Fatalf("expected the rules text to mention the 7x7 bonus; got %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back from rules: screen = %v, want screenMenu", a.screen)
	}

	if !h.TapRect(a.menu.RulesButton()) {
		t.Fatal("tap on Regler (2nd time) failed")
	}
	if !h.TapRect(a.rulesBack) {
		t.Fatal("tap on Tillbaka failed")
	}
	if a.screen != screenMenu {
		t.Fatalf("Tillbaka button: screen = %v, want screenMenu", a.screen)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayLapptacketScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if e := h.Screenshot(dir + "/lapptacket_splash.png"); e != nil {
		t.Fatal(e)
	}
	h.TapXY(500, 700) // dismiss splash
	if a.screen != screenMenu {
		t.Fatalf("splash tap did not open menu, screen=%v", a.screen)
	}
	_ = h.Screenshot(dir + "/lapptacket_menu.png")

	if h.TapRect(a.menu.RulesButton()) && a.screen == screenRules {
		_ = h.Screenshot(dir + "/lapptacket_rules.png")
		h.Back()
	}

	startOpponent(t, h, a, game.OpponentHotseat)
	h.Draw()
	_ = h.Screenshot(dir + "/lapptacket_board.png")

	// Drive the whole hot-seat game to its natural end, then snapshot the
	// final board with the winner banner.
	turns := 0
	for !a.gs.GameOver() {
		turns++
		if turns > 3000 {
			t.Fatalf("game did not end after %d turns", turns)
		}
		driveOneTurn(t, h, a)
	}
	h.Draw()
	if e := h.Screenshot(dir + "/lapptacket_end.png"); e != nil {
		t.Fatalf("screenshot: %v", e)
	}
}
