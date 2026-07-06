//go:build playtest

package main

// Headless PLAYTHROUGH tests for Konane. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): the board starts completely filled in a checkerboard; a one-time
// opening removes exactly two stones (Black takes a center stone, then White
// takes one of its own stones adjacent to the gap); every subsequent move is
// a mandatory jump-capture (or a chain of them, extendable at the player's
// choice, or ended early via "Klart"); a side with zero legal jumps on its
// turn loses immediately, with no pass. Covers both opponent modes (all 3 AI
// difficulties), the opening phase (legal and illegal taps), chain
// continuation and early-stop, the no-legal-jump loss condition, a full game
// played to a real win against the AI, quitting, restarting, and the rules
// screen. Runs under the pure-Go inkview emulator (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"konane/game"
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

// startOpponent picks the first menu row for the given opponent (and, for the
// AI, the given search depth), taps it, and enters the game (which always
// begins in PhaseOpeningBlackRemove, regardless of opponent).
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

func tapCell(h *ink.Harness, a *app, p image.Point) bool {
	return h.TapRect(a.layout.CellToScreen(p.X, p.Y))
}

// doOpening drives the one-time opening phase to completion via real taps:
// Black removes the first of the two center options; then, in hotseat mode,
// White (a human) removes the first of its adjacent options — in AI mode
// White's removal happens automatically (drained by the Tap that applied
// Black's removal), so no further input is needed.
func doOpening(t *testing.T, h *ink.Harness, a *app) {
	t.Helper()
	opts := game.CenterRemovalOptions()
	if !tapCell(h, a, opts[0]) {
		t.Fatal("Black's opening removal should be legal")
	}
	if a.gs.Opponent == game.OpponentHotseat {
		if a.gs.Phase != game.PhaseOpeningWhiteRemove {
			t.Fatalf("expected PhaseOpeningWhiteRemove after Black's removal, got %v", a.gs.Phase)
		}
		wopts := a.gs.OpeningWhiteOptions()
		if len(wopts) == 0 {
			t.Fatal("White should have at least one legal opening removal option")
		}
		if !tapCell(h, a, wopts[0]) {
			t.Fatal("White's opening removal should be legal")
		}
	}
	if a.gs.Phase != game.PhasePlaying {
		t.Fatalf("expected PhasePlaying once the opening completes, got %v", a.gs.Phase)
	}
}

// tapChain drives a complete jump chain through the real UI: tap the origin
// (selecting the stone and playing its first jump), then tap each further
// landing cell in turn. If the chain stops before the board would naturally
// end the turn (i.e. ChainActive is still true after the last requested
// jump), it presses "Klart" to end the turn there, exactly like a player
// choosing to stop a chain early.
func tapChain(t *testing.T, h *ink.Harness, a *app, chain []game.Jump) bool {
	t.Helper()
	if len(chain) == 0 {
		return false
	}
	if !tapCell(h, a, chain[0].From) {
		return false
	}
	if !tapCell(h, a, chain[0].To) {
		return false
	}
	for _, j := range chain[1:] {
		if !a.gs.ChainActive {
			t.Fatalf("chain ended early after %v; wanted %d jumps total", j, len(chain))
		}
		if !tapCell(h, a, j.To) {
			return false
		}
	}
	if a.gs.ChainActive {
		found := false
		for _, b := range a.buttons {
			if b.Label == "Klart" {
				h.TapRect(b.Rect)
				found = true
			}
		}
		if !found {
			t.Fatal("ChainActive but no Klart button present to stop the chain")
		}
	}
	return true
}

// setCell places a stone directly on the board, bypassing move legality —
// used to construct specific test positions. Board is a plain [Size][Size]Cell
// array (indexed [y][x], matching the game package's own convention), so
// external packages can index it directly without an exported setter.
func setCell(b *game.Board, x, y int, c game.Cell) {
	b[y][x] = c
}

// riggedPlayingState empties the game's board (bypassing the opening phase)
// and drops straight into PhasePlaying with the given side to move, so tests
// can construct exact jump/chain positions.
func riggedPlayingState(a *app, toMove game.Cell) {
	for y := range a.gs.Board {
		for x := range a.gs.Board[y] {
			a.gs.Board[y][x] = game.Empty
		}
	}
	a.gs.Phase = game.PhasePlaying
	a.gs.Turn = toMove
	a.gs.ChainActive = false
	a.gs.LastCaptured = nil
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- RULE: the one-time opening phase, including illegal-tap rejection -----

func TestPlayKonaneOpeningRejectsIllegal(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	// Tapping a non-center stone during Black's opening removal must be
	// rejected: phase and board stay untouched.
	blackBefore := a.gs.Board.Count(game.Black)
	if tapCell(h, a, image.Pt(0, 0)) {
		t.Fatal("tapping a non-center stone should be rejected during Black's opening removal")
	}
	if a.gs.Phase != game.PhaseOpeningBlackRemove || a.gs.Board.Count(game.Black) != blackBefore {
		t.Fatal("an illegal opening tap must not change phase or board")
	}

	opts := game.CenterRemovalOptions()
	if !tapCell(h, a, opts[0]) {
		t.Fatal("a legal center-removal option should be accepted")
	}
	if a.gs.Phase != game.PhaseOpeningWhiteRemove {
		t.Fatalf("phase should advance to PhaseOpeningWhiteRemove, got %v", a.gs.Phase)
	}

	// Tapping a stone not adjacent to the gap during White's removal must
	// also be rejected.
	whiteBefore := a.gs.Board.Count(game.White)
	if tapCell(h, a, image.Pt(0, 0)) {
		t.Fatal("tapping a non-adjacent stone should be rejected during White's opening removal")
	}
	if a.gs.Phase != game.PhaseOpeningWhiteRemove || a.gs.Board.Count(game.White) != whiteBefore {
		t.Fatal("an illegal opening tap must not change phase or board")
	}

	wopts := a.gs.OpeningWhiteOptions()
	if !tapCell(h, a, wopts[0]) {
		t.Fatal("a legal adjacent-removal option should be accepted")
	}
	if a.gs.Phase != game.PhasePlaying {
		t.Fatalf("phase should advance to PhasePlaying, got %v", a.gs.Phase)
	}
	if a.gs.Turn != game.Black {
		t.Fatal("Black should move first once normal play begins")
	}
}

// --- RULE: a plain jump matches the rules engine exactly --------------------

func TestPlayKonaneJumpRules(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	doOpening(t, h, a)

	legal := a.gs.Board.LegalJumpsAnywhere(a.gs.Turn)
	if len(legal) == 0 {
		t.Fatal("Black should have a legal jump after the opening")
	}
	j := legal[0]

	// Selecting a jumpable stone, then tapping it again, must deselect
	// without changing the board or the turn.
	blackBefore := a.gs.Board.Count(game.Black)
	if !tapCell(h, a, j.From) {
		t.Fatal("selecting a jumpable stone should be accepted")
	}
	if !tapCell(h, a, j.From) {
		t.Fatal("tapping the selected stone again should be accepted (deselect)")
	}
	if a.gs.Board.Count(game.Black) != blackBefore || a.gs.Turn != game.Black {
		t.Fatal("select/deselect must not change the board or the turn")
	}

	// Now actually perform the jump and check it matches the rules engine's
	// own Apply exactly.
	want := a.gs.Board.Apply(j, game.Black)
	if !tapCell(h, a, j.From) || !tapCell(h, a, j.To) {
		t.Fatalf("legal jump %v via tap was rejected", j)
	}
	if a.gs.Board != want {
		t.Fatalf("UI jump %v did not match the rules' own Apply result", j)
	}
}

// --- GOTCHA: chain jumps, both continuing and stopping early ---------------

func TestPlayKonaneChainContinueViaTap(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	riggedPlayingState(a, game.Black)
	// Black at (1,1) can jump White at (2,1) -> land (3,1); from (3,1) it can
	// jump White at (4,1) -> land (5,1).
	setCell(&a.gs.Board, 1, 1, game.Black)
	setCell(&a.gs.Board, 2, 1, game.White)
	setCell(&a.gs.Board, 4, 1, game.White)
	h.Draw()

	if !tapCell(h, a, image.Pt(1, 1)) {
		t.Fatal("selecting the jumping stone should be accepted")
	}
	if !tapCell(h, a, image.Pt(3, 1)) {
		t.Fatal("the first jump should be legal")
	}
	if !a.gs.ChainActive {
		t.Fatal("a further jump is available from (3,1); ChainActive should be true")
	}
	if a.gs.ChainFrom != (image.Point{X: 3, Y: 1}) {
		t.Fatalf("ChainFrom = %v, want (3,1)", a.gs.ChainFrom)
	}
	if a.gs.Board.At(2, 1) != game.Empty {
		t.Fatal("the first jumped stone should already be captured")
	}

	// A tap on a cell that is neither a valid continuation nor the chain's
	// own source must be rejected without disturbing the chain.
	if tapCell(h, a, image.Pt(0, 0)) {
		t.Fatal("an illegal continuation must be rejected")
	}
	if !a.gs.ChainActive {
		t.Fatal("a rejected continuation must not cancel the chain")
	}

	if !tapCell(h, a, image.Pt(5, 1)) {
		t.Fatal("the second jump should be legal")
	}
	if a.gs.ChainActive {
		t.Fatal("no further jump is available from (5,1); the chain should auto-end")
	}
	if a.gs.Turn != game.White {
		t.Fatal("turn should now belong to White")
	}
	if len(a.gs.LastCaptured) != 2 {
		t.Fatalf("LastCaptured should list both stones captured this turn, got %v", a.gs.LastCaptured)
	}
}

func TestPlayKonaneChainStopEarlyViaKlart(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	riggedPlayingState(a, game.Black)
	setCell(&a.gs.Board, 1, 1, game.Black)
	setCell(&a.gs.Board, 2, 1, game.White)
	setCell(&a.gs.Board, 4, 1, game.White) // a second jump WOULD be available, but is optional
	h.Draw()

	if !tapCell(h, a, image.Pt(1, 1)) || !tapCell(h, a, image.Pt(3, 1)) {
		t.Fatal("the first jump should be legal")
	}
	if !a.gs.ChainActive {
		t.Fatal("expected a chain to be active (a further jump exists)")
	}

	tappedKlart := false
	for _, b := range a.buttons {
		if b.Label == "Klart" {
			h.TapRect(b.Rect)
			tappedKlart = true
		}
	}
	if !tappedKlart {
		t.Fatalf("no Klart button shown while a chain is active; visible: %v", texts(h))
	}
	if a.gs.ChainActive {
		t.Fatal("ChainActive should be false after Klart")
	}
	if a.gs.Turn != game.White {
		t.Fatal("turn should pass to White once the player stops the chain early")
	}
	if a.gs.Board.At(4, 1) != game.White {
		t.Fatal("stopping the chain early must leave the not-yet-jumped stone in place")
	}
}

// --- GOTCHA: zero legal jumps anywhere -> immediate loss --------------------

func TestPlayKonaneNoLegalJumpLossViaTap(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	riggedPlayingState(a, game.Black)
	setCell(&a.gs.Board, 3, 3, game.Black)
	setCell(&a.gs.Board, 3, 4, game.White) // White's only stone
	h.Draw()

	if !tapCell(h, a, image.Pt(3, 3)) || !tapCell(h, a, image.Pt(3, 5)) {
		t.Fatal("Black's only legal jump should be accepted")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatalf("White should have zero stones (and thus zero jumps) after this capture, Phase=%v", a.gs.Phase)
	}
	if a.gs.Winner() != game.Black {
		t.Fatalf("Winner() = %v, want Black", a.gs.Winner())
	}
	if _, ok := h.FindTextContains("Svart vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- All 3 AI difficulties actually reply -----------------------------------

func TestPlayKonaneAllDifficultiesReply(t *testing.T) {
	for _, depth := range []int{game.DepthEasy, game.DepthMedium, game.DepthHard} {
		depth := depth
		t.Run(itoa(depth), func(t *testing.T) {
			h, a := bootToMenu(t)
			startOpponent(t, h, a, game.OpponentAI, depth)
			if a.gs.AIDepth != depth {
				t.Fatalf("AIDepth = %d, want %d", a.gs.AIDepth, depth)
			}
			doOpening(t, h, a) // White's opening removal is automatic in AI mode
			if a.gs.Phase != game.PhasePlaying {
				t.Fatalf("expected PhasePlaying after the opening, got %v", a.gs.Phase)
			}
			before := a.gs.Board
			chain, ok := game.BestMove(a.gs.Board, a.gs.Turn, game.DepthEasy)
			if !ok {
				t.Fatal("Black's first move should be legal")
			}
			if !tapChain(t, h, a, chain) {
				t.Fatal("Black's opening jump should be legal")
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

// --- Full game vs the AI, played to a real win --------------------------

func TestPlayKonaneFullGameVsAI(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)
	doOpening(t, h, a)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 300 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		chain, ok := game.BestMove(a.gs.Board, a.gs.Turn, game.DepthEasy)
		if !ok {
			t.Fatalf("human to move but no legal jump at ply %d", ply)
		}
		if !tapChain(t, h, a, chain) {
			t.Fatalf("legal chain %v at ply %d was rejected", chain, ply)
		}
	}

	want := "Vit vann!"
	if a.gs.Winner() == game.Black {
		want = "Svart vann!"
	}
	if _, ok := h.FindTextContains(want); !ok {
		bl, wh := a.gs.Board.Count(game.Black), a.gs.Board.Count(game.White)
		t.Fatalf("end banner %q (B%d/W%d) not shown; visible: %v", want, bl, wh, texts(h))
	}
}

// --- Quit mid-game (Back key AND the Meny button), then restart ------------

func TestPlayKonaneQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	doOpening(t, h, a)
	chain, ok := game.BestMove(a.gs.Board, a.gs.Turn, game.DepthEasy)
	if !ok {
		t.Fatal("setup: Black should have a legal jump")
	}
	tapChain(t, h, a, chain) // a move in progress

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

func TestPlayKonaneNyRestarts(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	doOpening(t, h, a)
	chain, ok := game.BestMove(a.gs.Board, a.gs.Turn, game.DepthEasy)
	if !ok {
		t.Fatal("setup: expected a legal jump")
	}
	if !tapChain(t, h, a, chain) {
		t.Fatal("setup: expected the move to be applied")
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
	if a.gs.Phase != game.PhaseOpeningBlackRemove {
		t.Fatalf("Ny should reset to a fresh opening phase, got Phase=%v", a.gs.Phase)
	}
	if a.gs.Board.Count(game.Black) != 32 || a.gs.Board.Count(game.White) != 32 {
		t.Fatal("Ny should reset to a fresh, fully-filled board")
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlayKonaneRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("MÅSTE"); !ok {
		t.Fatalf("rules text missing the mandatory-first-jump rule; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayKonaneScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/konane_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/konane_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/konane_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)
	if err := h.Screenshot(dir + "/konane_opening.png"); err != nil {
		t.Fatal(err)
	}
	doOpening(t, h, a)
	// Play a couple of plies to get a representative mid-game board.
	for i := 0; i < 2 && a.gs.Phase == game.PhasePlaying; i++ {
		if a.gs.AITurn() {
			break
		}
		chain, ok := game.BestMove(a.gs.Board, a.gs.Turn, game.DepthEasy)
		if !ok {
			break
		}
		tapChain(t, h, a, chain)
	}
	if err := h.Screenshot(dir + "/konane_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// The no-legal-jump win banner.
	riggedPlayingState(a, game.Black)
	setCell(&a.gs.Board, 3, 3, game.Black)
	setCell(&a.gs.Board, 3, 4, game.White)
	tapCell(h, a, image.Pt(3, 3))
	tapCell(h, a, image.Pt(3, 5))
	if err := h.Screenshot(dir + "/konane_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
