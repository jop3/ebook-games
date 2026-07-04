//go:build playtest

package main

// Headless PLAYTHROUGH tests for Ringar. They drive the real touch path
// against the rules as written (see rulesParagraphs in ui.go): White places
// first (10 placements total, 5 rings each), then rings slide along the 3
// axes dropping/flipping markers Othello-style, a completed row of 5 lets
// its owner remove those markers plus one of their own rings (any ring,
// their choice), and removing a 3rd ring wins. Runs under the pure-Go
// inkview emulator (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"ringar/game"
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

// startOpponent picks the menu row for the given opponent, taps it, and
// enters the game.
func startOpponent(t *testing.T, h *ink.Harness, a *app, opp game.Opponent) {
	t.Helper()
	for _, row := range a.menu.rows {
		if row.choice.opponent == opp {
			h.TapRect(row.rect)
			if a.screen != screenGame || a.gs == nil || a.gs.Opponent != opp {
				t.Fatalf("did not start opponent %v (screen=%v)", opp, a.screen)
			}
			return
		}
	}
	t.Fatalf("no menu row for opponent %v; visible: %v", opp, texts(h))
}

// tapPoint taps the screen location of board point p.
func tapPoint(h *ink.Harness, a *app, p game.Point) bool {
	c := a.layout.Center(p)
	return h.TapXY(c.X, c.Y)
}

// playMove drives a full ring move through the real UI: tap the ring
// (selecting it), then tap the destination.
func playMove(h *ink.Harness, a *app, from, to game.Point) bool {
	if !tapPoint(h, a, from) {
		return false
	}
	return tapPoint(h, a, to)
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// approachPoint returns a board neighbour of dest that is not already
// occupied by a ring or marker in gs — used to place a ring that can slide
// onto dest with zero markers jumped. It deliberately does NOT step further
// along dest's own line (a maximal run's endpoint has no valid point beyond
// it in that same direction, by definition), so it tries every one of
// dest's neighbours rather than guessing a single direction.
func approachPoint(t *testing.T, gs *game.GameState, dest game.Point) game.Point {
	t.Helper()
	for _, n := range game.Neighbors(dest) {
		if !gs.Board.HasRing(n) && !gs.Board.HasMarker(n) {
			return n
		}
	}
	t.Fatalf("no free neighbour of %v to approach from", dest)
	return game.Point{}
}

// clearBoard empties the board directly (used to build specific test
// positions, exactly like every other game's play_test.go setCell helper —
// Board's Rings/Markers maps are exported so no extra accessor is needed).
func clearBoard(gs *game.GameState) {
	gs.Board.Rings = map[game.Point]game.Side{}
	gs.Board.Markers = map[game.Point]game.Side{}
}

// --- RULE: placement phase, 10 real taps, correct turn order ---------------

func TestPlayRingarPlacement(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	if a.gs.Phase != game.PhasePlacement || a.gs.Turn != game.White {
		t.Fatalf("game should start in placement, White first; got phase=%v turn=%v", a.gs.Phase, a.gs.Turn)
	}
	pts := game.AllPoints()
	wantTurn := game.White
	for i := 0; i < 10; i++ {
		if a.gs.Turn != wantTurn {
			t.Fatalf("placement %d: turn = %v, want %v", i, a.gs.Turn, wantTurn)
		}
		if !tapPoint(h, a, pts[i*3]) { // spread them out so none collide
			t.Fatalf("placement tap %d at %v should be accepted", i, pts[i*3])
		}
		wantTurn = wantTurn.Opponent()
	}
	if a.gs.Phase != game.PhasePlaying {
		t.Fatalf("after 10 placements, phase should be Playing, got %v", a.gs.Phase)
	}
	if a.gs.Board.RingCount(game.Black) != 5 || a.gs.Board.RingCount(game.White) != 5 {
		t.Fatalf("each side should have 5 rings, got B=%d W=%d",
			a.gs.Board.RingCount(game.Black), a.gs.Board.RingCount(game.White))
	}

	// Tapping an occupied point during placement must be rejected.
	before := a.gs.Turn
	if tapPoint(h, a, pts[0]) && a.gs.Phase == game.PhasePlacement {
		t.Fatal("should not still be placing")
	}
	_ = before
}

// --- GOTCHA: a real move that jumps and flips several markers --------------

func TestPlayRingarMoveJumpAndFlip(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	// Fast-forward past placement directly (already covered by its own test).
	a.gs.Phase = game.PhasePlaying
	a.gs.Turn = game.Black
	clearBoard(a.gs)

	from := game.Point{X: -4, Y: 0, Z: 4}
	to := game.Point{X: 0, Y: 0, Z: 0}
	a.gs.Board.Rings[from] = game.Black
	a.gs.Board.Markers[game.Point{X: -3, Y: 0, Z: 3}] = game.White
	a.gs.Board.Markers[game.Point{X: -2, Y: 0, Z: 2}] = game.Black
	a.gs.Board.Markers[game.Point{X: -1, Y: 0, Z: 1}] = game.White
	h.Draw()

	if !playMove(h, a, from, to) {
		t.Fatal("the jump-3-markers move should be legal via real taps")
	}
	if a.gs.Board.Rings[to] != game.Black || a.gs.Board.HasRing(from) {
		t.Fatal("the ring should have slid from `from` to `to`")
	}
	if a.gs.Board.Markers[game.Point{X: -3, Y: 0, Z: 3}] != game.Black ||
		a.gs.Board.Markers[game.Point{X: -2, Y: 0, Z: 2}] != game.White ||
		a.gs.Board.Markers[game.Point{X: -1, Y: 0, Z: 1}] != game.Black {
		t.Fatal("all 3 jumped markers should have been flipped to Black")
	}
	if a.gs.Board.Markers[from] != game.Black {
		t.Fatal("a Black marker should have been dropped at `from`")
	}
	if a.gs.Turn != game.White {
		t.Fatalf("turn should pass to White after a move that completes no row, got %v", a.gs.Turn)
	}
}

// --- GOTCHA: a row of 5 via real taps, then the explicit ring-removal ------

func TestPlayRingarRowCompletionAndRingRemoval(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	a.gs.Phase = game.PhasePlaying
	a.gs.Turn = game.Black
	clearBoard(a.gs)

	line := lineOf(game.AxisA, 5)
	for _, p := range line[:4] {
		a.gs.Board.Markers[p] = game.Black
	}
	// ApplyRingMove drops a marker at the ring's DEPARTURE point (`from`),
	// not at its destination — so to complete the row at line[4], a Black
	// ring must start there and slide away to some free neighbour.
	a.gs.Board.Rings[line[4]] = game.Black
	to := approachPoint(t, a.gs, line[4])
	spareRing := approachPoint(t, a.gs, line[0])
	a.gs.Board.Rings[spareRing] = game.Black
	h.Draw()

	if !playMove(h, a, line[4], to) {
		t.Fatal("the row-completing move should be legal via real taps")
	}
	if a.gs.Phase != game.PhaseRowPending || a.gs.PendingSide != game.Black {
		t.Fatalf("completing a row of 5 should enter RowPending for Black, got phase=%v side=%v", a.gs.Phase, a.gs.PendingSide)
	}
	if len(a.gs.PendingWindow) != 5 {
		t.Fatalf("a run of exactly 5 should auto-fix the window, got %v", a.gs.PendingWindow)
	}
	if _, ok := h.FindTextContains("välj en ring"); !ok {
		t.Fatalf("status should prompt to pick a ring to remove; visible: %v", texts(h))
	}

	// Tapping a point that is NOT one of Black's own rings must be rejected.
	if tapPoint(h, a, game.Point{X: 3, Y: -3, Z: 0}) && a.gs.Phase != game.PhaseRowPending {
		t.Fatal("tapping empty space must not resolve the row")
	}

	// Tap the spare ring to complete the resolution.
	if !tapPoint(h, a, spareRing) {
		t.Fatal("tapping one of Black's own rings should complete the row resolution")
	}
	if a.gs.Removed[game.Black] != 1 {
		t.Fatalf("Removed[Black] = %d, want 1", a.gs.Removed[game.Black])
	}
	if a.gs.Board.HasRing(spareRing) {
		t.Fatal("the chosen ring should have been removed from the board")
	}
	for _, p := range line {
		if a.gs.Board.HasMarker(p) {
			t.Fatalf("row marker at %v should have been cleared", p)
		}
	}
	if a.gs.Phase != game.PhasePlaying || a.gs.Turn != game.White {
		t.Fatalf("after the only pending row resolves, turn should pass to White; got phase=%v turn=%v", a.gs.Phase, a.gs.Turn)
	}
}

// --- GOTCHA: a 6-in-a-row window choice via a real tap ----------------------

func TestPlayRingarSixInARowWindowChoice(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	a.gs.Phase = game.PhasePlaying
	a.gs.Turn = game.White
	clearBoard(a.gs)

	line := lineOf(game.AxisB, 6)
	for _, p := range line[:5] {
		a.gs.Board.Markers[p] = game.White
	}
	// A White ring sitting on the 6th point of the line slides away, dropping
	// the marker that completes the 6-run.
	a.gs.Board.Rings[line[5]] = game.White
	to := approachPoint(t, a.gs, line[5])
	spareRing := approachPoint(t, a.gs, line[0])
	a.gs.Board.Rings[spareRing] = game.White
	h.Draw()

	if !playMove(h, a, line[5], to) {
		t.Fatal("the 6th-marker move should be legal via real taps")
	}
	if a.gs.Phase != game.PhaseRowPending || len(a.gs.PendingRun) != 6 {
		t.Fatalf("expected a pending 6-run, got phase=%v run=%v", a.gs.Phase, a.gs.PendingRun)
	}
	if a.gs.PendingWindow != nil {
		t.Fatal("a run longer than 5 must require an explicit window choice, not auto-fix one")
	}
	if _, ok := h.FindTextContains("välj vilka 5"); !ok {
		t.Fatalf("status should prompt to choose the window; visible: %v", texts(h))
	}

	// Tap the very first marker in the run: should select the leftmost window.
	if !tapPoint(h, a, line[0]) {
		t.Fatal("tapping a marker in the run should choose the window")
	}
	if a.gs.PendingWindow[0] != line[0] || a.gs.PendingWindow[4] != line[4] {
		t.Fatalf("tapping the first marker should select window [0:5], got %v", a.gs.PendingWindow)
	}

	if !tapPoint(h, a, spareRing) {
		t.Fatal("completing the resolution with the spare ring should succeed")
	}
	if a.gs.Board.Markers[line[5]] != game.White {
		t.Fatal("the marker outside the chosen window (the 6th) must remain on the board")
	}
	for _, p := range line[:5] {
		if a.gs.Board.HasMarker(p) {
			t.Fatalf("marker %v inside the chosen window should have been removed", p)
		}
	}
}

// --- Both opponent modes ----------------------------------------------------

func TestPlayRingarBothOpponentModes(t *testing.T) {
	t.Run("hotseat", func(t *testing.T) {
		h, a := bootToMenu(t)
		startOpponent(t, h, a, game.OpponentHotseat)
		if a.gs.AITurn() {
			t.Fatal("hotseat mode should never report an AI turn")
		}
	})
	t.Run("ai", func(t *testing.T) {
		h, a := bootToMenu(t)
		startOpponent(t, h, a, game.OpponentAI)
		// White places first, and White is the AI side, so the very first
		// placement should happen automatically without any human tap.
		for i := 0; i < 50 && a.gs.PlacedCount() == 0; i++ {
			h.Draw()
		}
		if a.gs.PlacedCount() == 0 {
			t.Fatal("AI should have made the opening placement automatically")
		}
	})
}

// --- Full game to a real 3-rings win ----------------------------------------

// TestPlayRingarFullGameToWin drives 3 separate row completions through the
// real UI (a move via taps, then the window choice when needed, then the
// ring-removal tap), each a deterministic, hand-picked position — this is
// the game's actual state machine validating a genuine 3-rings win end to
// end, not a shortcut that sets Removed directly.
func TestPlayRingarFullGameToWin(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	a.gs.Phase = game.PhasePlaying
	a.gs.Turn = game.Black

	axes := []game.Axis{game.AxisA, game.AxisB, game.AxisC}
	for round := 0; round < 3; round++ {
		clearBoard(a.gs)
		line := lineOf(axes[round], 5)
		for _, p := range line[:4] {
			a.gs.Board.Markers[p] = game.Black
		}
		// The completing ring starts ON line[4] and slides away, dropping the
		// marker that finishes the row (ApplyRingMove marks the DEPARTURE
		// point, not the destination).
		a.gs.Board.Rings[line[4]] = game.Black
		to := approachPoint(t, a.gs, line[4])
		ringToLose := approachPoint(t, a.gs, line[0])
		a.gs.Board.Rings[ringToLose] = game.Black
		a.gs.Turn = game.Black
		h.Draw()

		if !playMove(h, a, line[4], to) {
			t.Fatalf("round %d: the row-completing move should be legal", round)
		}
		if a.gs.Phase != game.PhaseRowPending {
			t.Fatalf("round %d: expected RowPending, got %v", round, a.gs.Phase)
		}
		if !tapPoint(h, a, ringToLose) {
			t.Fatalf("round %d: ring-removal tap should succeed", round)
		}
		if a.gs.Removed[game.Black] != round+1 {
			t.Fatalf("round %d: Removed[Black] = %d, want %d", round, a.gs.Removed[game.Black], round+1)
		}
	}

	if a.gs.Phase != game.PhaseDone {
		t.Fatalf("Phase should be Done at 3 removed rings, got %v", a.gs.Phase)
	}
	if _, ok := h.FindTextContains("Svart vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- Quit mid-game (Back key AND the Meny button), then restart ------------

func TestPlayRingarQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	tapPoint(h, a, game.AllPoints()[0]) // a placement in progress

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}

	startOpponent(t, h, a, game.OpponentAI)
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
	startOpponent(t, h, a, game.OpponentHotseat)
}

// --- "Ny" restarts the current configuration mid-game -----------------------

func TestPlayRingarNyRestarts(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	tapPoint(h, a, game.AllPoints()[0])
	if a.gs.Turn != game.Black {
		t.Fatal("setup: expected turn to have passed to Black after White's placement")
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
	if a.gs.Turn != game.White || a.gs.Phase != game.PhasePlacement || a.gs.PlacedCount() != 0 {
		t.Fatal("Ny should reset to a fresh placement-phase game")
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlayRingarRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("YINSH"); !ok {
		t.Fatalf("rules text missing the YINSH credit; visible: %v", texts(h))
	}
	if _, ok := h.FindTextContains("avslappnad"); !ok {
		t.Fatalf("rules text missing the honest casual-AI framing; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayRingarScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/ringar_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/ringar_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/ringar_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startOpponent(t, h, a, game.OpponentHotseat)
	// Full 85-point board, partway through placement (the worst-case
	// legibility test: every point drawn, some already occupied).
	pts := game.AllPoints()
	for i := 0; i < 6; i++ {
		tapPoint(h, a, pts[i*7])
	}
	if err := h.Screenshot(dir + "/ringar_placement.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// Mid-movement-phase: rings, markers, and some flipped markers visible.
	a.gs.Phase = game.PhasePlaying
	a.gs.Turn = game.Black
	clearBoard(a.gs)
	for i, p := range pts {
		switch {
		case i%17 == 0:
			a.gs.Board.Rings[p] = game.Black
		case i%17 == 3:
			a.gs.Board.Rings[p] = game.White
		case i%5 == 1:
			a.gs.Board.Markers[p] = game.Black
		case i%5 == 2:
			a.gs.Board.Markers[p] = game.White
		}
	}
	from := game.Point{X: -4, Y: 0, Z: 4}
	to := game.Point{X: 0, Y: 0, Z: 0}
	for _, mid := range []game.Point{{X: -3, Y: 0, Z: 3}, {X: -2, Y: 0, Z: 2}, {X: -1, Y: 0, Z: 1}} {
		delete(a.gs.Board.Rings, mid)
		a.gs.Board.Markers[mid] = game.White
	}
	for _, p := range []game.Point{from, to} {
		delete(a.gs.Board.Rings, p)
		delete(a.gs.Board.Markers, p)
	}
	a.gs.Board.Rings[from] = game.Black
	h.Draw()
	if !playMove(h, a, from, to) {
		t.Fatal("demo move for the mid-game screenshot should be legal")
	}
	if err := h.Screenshot(dir + "/ringar_movement.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// Row-of-5 highlight + ring-removal prompt.
	clearBoard(a.gs)
	line := lineOf(game.AxisA, 5)
	for _, p := range line[:4] {
		a.gs.Board.Markers[p] = game.Black
	}
	a.gs.Board.Rings[line[4]] = game.Black
	ringTo := approachPoint(t, a.gs, line[4])
	spareRing := approachPoint(t, a.gs, line[0])
	a.gs.Board.Rings[spareRing] = game.Black
	a.gs.Turn = game.Black
	h.Draw()
	playMove(h, a, line[4], ringTo)
	if err := h.Screenshot(dir + "/ringar_row_removal.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// Win banner.
	tapPoint(h, a, spareRing)
	a.gs.Removed[game.Black] = 3
	a.gs.Phase = game.PhaseDone
	h.Draw()
	if err := h.Screenshot(dir + "/ringar_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}

func lineOf(axis game.Axis, length int) []game.Point {
	for _, line := range game.Lines(axis) {
		if len(line) >= length {
			return line[:length]
		}
	}
	return nil
}
