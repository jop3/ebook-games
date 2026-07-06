//go:build playtest

package main

// Headless PLAYTHROUGH tests for L-spelet (the L-Game). They drive the real
// touch path and check the gameplay against the rules as written (see
// rulesParagraphs in ui.go): each side has one L-tetromino piece placeable
// in any of its 8 orientations; a legal new placement must differ from the
// current one and not overlap any other piece; placing the L-piece is
// MANDATORY whenever any legal placement exists; after placing, moving a
// neutral piece is OPTIONAL (or skip via "Klar"); a side that has zero
// legal L-placements on its own turn loses immediately, without moving.
// Covers hotseat and all 3 AI difficulties, illegal-placement rejection,
// the mandatory-move win condition, quitting, restarting, and the rules
// screen. Runs under the pure-Go inkview emulator (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"lgame/game"
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

// placeL drives a full L-placement through the real UI: tap the
// orientation icon, then tap the placement's anchor cell (its topmost-
// leftmost occupied cell — see LegalLPlacements' doc comment for why that's
// a safe, unambiguous tap target).
func placeL(h *ink.Harness, a *app, pl game.Placement) bool {
	if pl.Orient < 0 || pl.Orient > 7 {
		return false
	}
	if !h.TapRect(a.orientRects[pl.Orient]) {
		return false
	}
	anchor := pl.Cells[0]
	return h.TapRect(a.layout.CellToScreen(anchor.X, anchor.Y))
}

// moveNeutral drives a full neutral-piece move: tap its current cell, then
// tap the destination cell.
func moveNeutral(h *ink.Harness, a *app, m game.NeutralMove) bool {
	if !h.TapRect(a.layout.CellToScreen(m.From.X, m.From.Y)) {
		return false
	}
	return h.TapRect(a.layout.CellToScreen(m.To.X, m.To.Y))
}

// skipNeutral taps the "Klar" button to decline the optional neutral move.
func skipNeutral(h *ink.Harness, a *app) bool {
	for _, b := range a.buttons {
		if b.Label == "Klar" {
			return h.TapRect(b.Rect)
		}
	}
	return false
}

// firstLegalPlacement returns some legal L-placement for the side to move
// (deterministic: LegalLPlacements' own enumeration order).
func firstLegalPlacement(a *app) (game.Placement, bool) {
	moves := game.LegalLPlacements(a.gs.Board, a.gs.Turn)
	if len(moves) == 0 {
		return game.Placement{}, false
	}
	return moves[0], true
}

// setCell places a piece directly on the board, bypassing move legality -
// used to construct specific test positions. Board is a plain
// [Size][Size]Cell array (indexed [y][x], matching the game package's own
// convention), so external packages can index it directly without an
// exported setter.
func setCell(b *game.Board, x, y int, c game.Cell) {
	b[y][x] = c
}

// neutralCells returns the board coordinates of every Neutral-occupied
// cell, in row-major order.
func neutralCells(b game.Board) []image.Point {
	var out []image.Point
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			if b.At(x, y) == game.Neutral {
				out = append(out, image.Pt(x, y))
			}
		}
	}
	return out
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- RULE: orientation picker + anchor placement via real taps -------------

func TestPlayLgameOrientationPickerAndPlacement(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	pl, ok := firstLegalPlacement(a)
	if !ok {
		t.Fatal("Black should have a legal placement from the starting position")
	}
	// The pure rules engine's own Apply must match exactly what the UI
	// produces via the two-tap (orientation, then anchor) flow.
	want := game.ApplyLPlacement(a.gs.Board, game.Black, pl)
	if !placeL(h, a, pl) {
		t.Fatalf("legal placement %+v via the UI was rejected", pl)
	}
	if a.gs.Board != want {
		t.Fatalf("UI placement did not match the rules' own ApplyLPlacement result")
	}
	if a.gs.Step != game.StepNeutralOptional {
		t.Fatalf("Step = %v, want StepNeutralOptional after placing the mandatory L", a.gs.Step)
	}
	if a.gs.Turn != game.Black {
		t.Fatal("turn must not pass to White until the neutral step is resolved")
	}
}

// --- GOTCHA: an illegal anchor tap must not mutate the board ---------------

func TestPlayLgameIllegalPlacementRejected(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	before := a.gs.Board
	// Pick an orientation, then tap a cell that's NOT one of its legal
	// anchors (e.g. dead center of the board, which usually isn't a legal
	// first-move anchor for the starting position's cramped opening).
	h.TapRect(a.orientRects[0])
	// Tap every cell that is NOT a legal anchor for orientation 0; at least
	// one must exist on a 4x4 board with a handful of legal placements.
	legalAnchors := map[image.Point]bool{}
	for _, at := range a.anchorTargets {
		legalAnchors[at.Pl.Cells[0]] = true
	}
	tappedIllegal := false
	for y := 0; y < game.Size && !tappedIllegal; y++ {
		for x := 0; x < game.Size && !tappedIllegal; x++ {
			if legalAnchors[image.Pt(x, y)] {
				continue
			}
			h.TapRect(a.layout.CellToScreen(x, y))
			tappedIllegal = true
		}
	}
	if !tappedIllegal {
		t.Fatal("test setup: expected at least one non-anchor cell to tap")
	}
	if a.gs.Board != before {
		t.Fatal("tapping a non-legal-anchor cell must not mutate the board")
	}
	if a.gs.Step != game.StepPlaceL {
		t.Fatal("an illegal tap must not advance past the mandatory L-placement step")
	}
}

// --- RULE: optional neutral move, and skipping it via Klar ------------------

func TestPlayLgameNeutralMoveThenTurnPasses(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	pl, _ := firstLegalPlacement(a)
	if !placeL(h, a, pl) {
		t.Fatal("setup: placement should succeed")
	}
	nm := game.LegalNeutralMoves(a.gs.Board)[0]
	if !moveNeutral(h, a, nm) {
		t.Fatalf("legal neutral move %+v via the UI was rejected", nm)
	}
	if a.gs.Board.At(nm.To.X, nm.To.Y) != game.Neutral {
		t.Error("destination cell should hold a neutral piece")
	}
	if a.gs.Turn != game.White {
		t.Fatal("turn should pass to White after the neutral move")
	}
	if a.gs.Step != game.StepPlaceL {
		t.Fatal("Step should reset to StepPlaceL for White's turn")
	}
}

func TestPlayLgameSkipNeutralViaKlarButton(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	pl, _ := firstLegalPlacement(a)
	placeL(h, a, pl)
	neutralsBefore := neutralCells(a.gs.Board)
	if !skipNeutral(h, a) {
		t.Fatal("Klar button should be present and tappable during the neutral-optional step")
	}
	neutralsAfter := neutralCells(a.gs.Board)
	if len(neutralsBefore) != len(neutralsAfter) {
		t.Fatal("skipping via Klar must not change the neutral piece count")
	}
	for i := range neutralsBefore {
		if neutralsBefore[i] != neutralsAfter[i] {
			t.Fatal("skipping via Klar must not move any neutral piece")
		}
	}
	if a.gs.Turn != game.White {
		t.Fatal("Klar should end Black's turn, passing to White")
	}
}

// --- WIN: no legal L-placement on your turn loses immediately --------------

func TestPlayLgameMandatoryMoveWinBanner(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	// Construct a position, reachable mid-UI-flow, where Black is about to
	// make a full turn that leaves White with zero legal L-placements:
	// White's own L is left in place, and every other cell becomes Black,
	// so no alternative placement (of any orientation) is available to
	// White afterward - the same construction style used by the game
	// package's own win-condition tests.
	var b game.Board
	setCell(&b, 1, 2, game.Black)
	setCell(&b, 2, 2, game.Black)
	setCell(&b, 3, 2, game.Black)
	setCell(&b, 1, 3, game.Black)
	setCell(&b, 0, 0, game.White)
	setCell(&b, 1, 0, game.White)
	setCell(&b, 2, 0, game.White)
	setCell(&b, 2, 1, game.White)
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			if b.At(x, y) == game.Empty {
				setCell(&b, x, y, game.Black)
			}
		}
	}
	if len(game.LegalLPlacements(b, game.White)) != 0 {
		t.Fatal("test setup bug: White should already have zero legal placements")
	}
	a.gs.Board = b
	a.gs.Turn = game.Black
	a.gs.Step = game.StepNeutralOptional // pretend Black just placed its L
	h.Draw()

	if !skipNeutral(h, a) {
		t.Fatal("Klar button should end Black's turn")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done: White has no legal L-placement on its turn")
	}
	if _, ok := h.FindTextContains("Svart vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- All 3 AI difficulties actually reply -----------------------------------

func TestPlayLgameAllDifficultiesReply(t *testing.T) {
	for _, depth := range []int{game.DepthEasy, game.DepthMedium, game.DepthHard} {
		depth := depth
		t.Run(itoa(depth), func(t *testing.T) {
			h, a := bootToMenu(t)
			startOpponent(t, h, a, game.OpponentAI, depth)
			if a.gs.AIDepth != depth {
				t.Fatalf("AIDepth = %d, want %d", a.gs.AIDepth, depth)
			}
			pl, ok := firstLegalPlacement(a)
			if !ok {
				t.Fatal("Black should have an opening placement")
			}
			before := a.gs.Board
			if !placeL(h, a, pl) {
				t.Fatal("Black's opening placement should be legal")
			}
			if a.gs.Phase == game.PhasePlaying && !skipNeutral(h, a) {
				t.Fatal("Black should be able to skip the neutral step")
			}
			if a.gs.AITurn() {
				t.Fatal("control returned on the AI's turn (deferred reply not drained)")
			}
			if a.gs.Board == before && a.gs.Phase == game.PhasePlaying {
				t.Fatal("White (the AI) did not reply")
			}
		})
	}
}

// --- Full game vs the AI, played to a real win/loss -------------------------

// TestPlayLgameFullGameVsAI drives a deterministic, none-too-clever human
// policy (always the first legal L-placement, always skip the neutral
// step) against the AI. This isn't the strongest possible defense, but it
// is a real, rule-following game played entirely through the touch path,
// and it reliably terminates quickly (an AI of any configured difficulty
// traps so simple a policy within a handful of plies - verified separately
// in game/ai_test.go-style exploration), unlike two near-optimal AIs facing
// off, which the L-Game is known to be able to draw out indefinitely.
func TestPlayLgameFullGameVsAI(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 200 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		pl, ok := firstLegalPlacement(a)
		if !ok {
			t.Fatalf("human to move but no legal placement at ply %d (should have ended the game)", ply)
		}
		if !placeL(h, a, pl) {
			t.Fatalf("legal placement %+v at ply %d was rejected", pl, ply)
		}
		if a.gs.Phase == game.PhasePlaying && a.gs.Step == game.StepNeutralOptional {
			if !skipNeutral(h, a) {
				t.Fatalf("Klar should always be available at ply %d", ply)
			}
		}
	}
	want := "Vit vann!"
	if a.gs.Winner() == game.Black {
		want = "Svart vann!"
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("end banner %q not shown; visible: %v", want, texts(h))
	}
}

// --- Quit mid-game (Back key AND the Meny button), then restart ------------

func TestPlayLgameQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	pl, _ := firstLegalPlacement(a)
	placeL(h, a, pl) // a move in progress

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

func TestPlayLgameNyRestarts(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	pl, _ := firstLegalPlacement(a)
	placeL(h, a, pl)
	if a.gs.Step != game.StepNeutralOptional {
		t.Fatal("setup: expected a placement to have been made")
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
	if a.gs.Turn != game.Black || a.gs.Step != game.StepPlaceL {
		t.Fatal("Ny should reset to a fresh starting position")
	}
	if a.gs.Board.Count(game.Black) != 4 || a.gs.Board.Count(game.White) != 4 || a.gs.Board.Count(game.Neutral) != 2 {
		t.Fatal("Ny should reset to a fresh, fully-populated starting position")
	}
}

// --- Input guard: taps must be ignored once the game has ended -------------

func TestPlayLgameNoInputAfterGameEnds(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	var b game.Board
	setCell(&b, 1, 2, game.Black)
	setCell(&b, 2, 2, game.Black)
	setCell(&b, 3, 2, game.Black)
	setCell(&b, 1, 3, game.Black)
	setCell(&b, 0, 0, game.White)
	setCell(&b, 1, 0, game.White)
	setCell(&b, 2, 0, game.White)
	setCell(&b, 2, 1, game.White)
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			if b.At(x, y) == game.Empty {
				setCell(&b, x, y, game.Black)
			}
		}
	}
	a.gs.Board = b
	a.gs.Turn = game.Black
	a.gs.Step = game.StepNeutralOptional
	h.Draw()
	skipNeutral(h, a)
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("setup: game should have ended")
	}
	before := a.gs.Board
	// Tap all over the board; nothing should change post-game-over.
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			h.TapRect(a.layout.CellToScreen(x, y))
		}
	}
	if a.gs.Board != before {
		t.Fatal("taps after game-over must not mutate the board")
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlayLgameRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("MÅSTE"); !ok {
		t.Fatalf("rules text missing the mandatory-move rule; visible: %v", texts(h))
	}
	if _, ok := h.FindTextContains("förlorar du direkt"); !ok {
		t.Fatalf("rules text missing the lose-if-no-placement win condition; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayLgameScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/lgame_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/lgame_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/lgame_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startOpponent(t, h, a, game.OpponentHotseat, 0)
	if err := h.Screenshot(dir + "/lgame_board_orient_picker.png"); err != nil {
		t.Fatal(err)
	}
	pl, _ := firstLegalPlacement(a)
	placeL(h, a, pl)
	if err := h.Screenshot(dir + "/lgame_board_neutral_step.png"); err != nil {
		t.Fatal(err)
	}
}
