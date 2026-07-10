//go:build playtest

package main

// Headless PLAYTHROUGH tests for Hertigen. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): Black starts; a selected tile's CURRENT face determines its legal
// relocate/strike destinations; acting flips a tile to its other face;
// Strike captures in place without the striker relocating; Recruit places a
// reserve troop on an empty square orthogonally adjacent to the acting
// side's Duke and consumes the whole turn; capturing the opposing Duke ends
// the game. Covers all 3 AI difficulties, illegal-action rejection,
// quitting, and the rules/legend screens. Runs under the pure-Go inkview
// emulator (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"hertigen/game"
)

// --- helpers -----------------------------------------------------------------

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

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

func clearBoard(a *app) {
	for y := range a.gs.Board {
		for x := range a.gs.Board[y] {
			a.gs.Board[y][x] = nil
		}
	}
}

func put(a *app, x, y int, typ game.TileType, side game.Side, face game.Face) {
	a.gs.Board[y][x] = &game.Tile{Type: typ, Side: side, Face: face}
}

// tapCell taps the board cell (x,y) through the real Layout.
func tapCell(h *ink.Harness, a *app, x, y int) bool {
	return h.TapRect(a.layout.CellToScreen(x, y))
}

// tapAction drives one full Hertigen action through the real UI: for a
// relocate/strike it's origin-then-destination; for a recruit it's the
// Duke's square, the "Rekrytera" button, the reserve type's name button,
// then the destination square.
func tapAction(h *ink.Harness, a *app, act game.Action) bool {
	if act.Kind == game.ActRecruit {
		dukePos, ok := a.gs.Board.DukePos(a.gs.Turn)
		if !ok {
			return false
		}
		if !tapCell(h, a, dukePos.X, dukePos.Y) {
			return false
		}
		if err := h.TapText("Rekrytera"); err != nil {
			return false
		}
		if err := h.TapText(act.Recruit.Name()); err != nil {
			return false
		}
		return tapCell(h, a, act.To.X, act.To.Y)
	}
	if !tapCell(h, a, act.From.X, act.From.Y) {
		return false
	}
	return tapCell(h, a, act.To.X, act.To.Y)
}

// bestAction returns a legal action for the side to move, preferring a
// Duke-capture, then any other capture, then whatever's first — enough of a
// deterministic policy to drive a full game to termination.
func bestAction(s *game.GameState) (game.Action, bool) {
	actions := s.LegalActions()
	if len(actions) == 0 {
		return game.Action{}, false
	}
	best := actions[0]
	bestVal := -1
	for _, act := range actions {
		if act.Kind == game.ActRecruit {
			continue
		}
		if t := s.Board.At(act.To.X, act.To.Y); t != nil {
			val := 2
			if t.Type == game.Duke {
				val = 100
			}
			if val > bestVal {
				bestVal, best = val, act
			}
		}
	}
	return best, true
}

// --- RULE: selection, legal-move rejection, and the flip-on-act mechanic ----

func TestPlayHertigenSelectAndFlip(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(a)
	put(a, 2, 2, game.Footman, game.Black, game.FaceA)
	put(a, 5, 5, game.Duke, game.White, game.FaceA)
	put(a, 0, 5, game.Duke, game.Black, game.FaceA)
	a.gs.Turn = game.Black
	h.Draw()

	// Tapping an empty square with nothing selected does nothing.
	if tapCell(h, a, 3, 3) {
		t.Fatal("tapping an empty square with no selection should not be handled")
	}

	// Select the Footman: Face A is the orthogonal adjacency pattern, so
	// (3,2) should be offered as a destination.
	if !tapCell(h, a, 2, 2) {
		t.Fatal("selecting an own tile should be handled")
	}
	if a.mode != selTile || a.selected != (image.Point{X: 2, Y: 2}) {
		t.Fatalf("expected selTile at (2,2), got mode=%v selected=%v", a.mode, a.selected)
	}

	// Move it to (3,2) (empty, orthogonally adjacent).
	if !tapCell(h, a, 3, 2) {
		t.Fatal("the legal relocate should be accepted")
	}
	if a.gs.Turn != game.White {
		t.Fatal("turn did not pass to White after a legal move")
	}
	moved := a.gs.Board.At(3, 2)
	if moved == nil || moved.Type != game.Footman || moved.Side != game.Black {
		t.Fatalf("Footman did not land at (3,2): %+v", moved)
	}
	if moved.Face != game.FaceB {
		t.Fatal("acting must flip the tile to its other face")
	}
	if a.gs.Board.At(2, 2) != nil {
		t.Fatal("the origin square should now be empty")
	}
}

func TestPlayHertigenIllegalDestinationRejected(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(a)
	put(a, 2, 2, game.Footman, game.Black, game.FaceA)
	put(a, 0, 5, game.Duke, game.Black, game.FaceA)
	put(a, 5, 5, game.Duke, game.White, game.FaceA)
	a.gs.Turn = game.Black
	h.Draw()

	tapCell(h, a, 2, 2) // select the Footman
	if tapCell(h, a, 4, 4) {
		t.Fatal("a far-away square is not a legal Face A destination and must be rejected")
	}
	if a.gs.Turn != game.Black {
		t.Fatal("an illegal destination must not change the turn")
	}
	if a.gs.Board.At(2, 2) == nil {
		t.Fatal("the Footman must not have moved")
	}
}

// --- GOTCHA: Strike captures without the striker relocating -----------------

func TestPlayHertigenStrikeWithoutRelocating(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(a)
	// Katapult Face A strikes exactly 2 squares orthogonally (needs a clear
	// intervening square) and NEVER relocates.
	put(a, 2, 2, game.Catapult, game.Black, game.FaceA)
	put(a, 2, 4, game.Footman, game.White, game.FaceA)
	put(a, 0, 5, game.Duke, game.Black, game.FaceA)
	put(a, 5, 5, game.Duke, game.White, game.FaceA)
	a.gs.Turn = game.Black
	h.Draw()

	if !tapCell(h, a, 2, 2) {
		t.Fatal("selecting the Katapult should be handled")
	}
	if !tapCell(h, a, 2, 4) {
		t.Fatal("the strike on the White Footman should be accepted")
	}
	if a.gs.Board.At(2, 4) != nil {
		t.Fatal("the struck Footman should be captured")
	}
	striker := a.gs.Board.At(2, 2)
	if striker == nil || striker.Type != game.Catapult {
		t.Fatal("the Katapult must stay at its own square — Strike never relocates")
	}
	if striker.Face != game.FaceB {
		t.Fatal("striking still flips the striker to its other face")
	}
	if len(a.gs.LastCaptured) != 1 || a.gs.LastCaptured[0] != (image.Point{X: 2, Y: 4}) {
		t.Fatalf("LastCaptured = %v, want exactly [(2,4)]", a.gs.LastCaptured)
	}
}

// --- GOTCHA: recruiting via Duke -> Rekrytera -> type -> square -------------

func TestPlayHertigenRecruitFlow(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(a)
	put(a, 2, 2, game.Duke, game.Black, game.FaceA)
	put(a, 5, 5, game.Duke, game.White, game.FaceA)
	a.gs.Turn = game.Black
	a.gs.Reserve[game.Black] = game.NewReserve() // Riddare, Ryttare, Diagonalvakt, Katapult
	h.Draw()

	if !tapCell(h, a, 2, 2) {
		t.Fatal("selecting the Duke should be handled")
	}
	if !a.canRecruitFromSelection() {
		t.Fatal("recruiting should be possible: empty squares adjacent to the Duke and a non-empty reserve")
	}
	if err := h.TapText("Rekrytera"); err != nil {
		t.Fatalf("no Rekrytera button after selecting the Duke: %v", err)
	}
	if a.mode != selRecruitType {
		t.Fatalf("expected selRecruitType, got %v", a.mode)
	}
	if err := h.TapText("Riddare"); err != nil {
		t.Fatalf("no Riddare reserve-type button: %v (visible: %v)", err, texts(h))
	}
	if a.mode != selRecruitSquare || a.recruitType != game.Knight {
		t.Fatalf("expected selRecruitSquare/Knight, got mode=%v type=%v", a.mode, a.recruitType)
	}
	if !tapCell(h, a, 3, 2) { // orthogonally adjacent to the Duke at (2,2)
		t.Fatal("placing the recruit on an adjacent empty square should be accepted")
	}
	placed := a.gs.Board.At(3, 2)
	if placed == nil || placed.Type != game.Knight || placed.Side != game.Black || placed.Face != game.FaceA {
		t.Fatalf("Riddare not placed correctly at (3,2): %+v", placed)
	}
	if a.gs.Reserve[game.Black].Has(game.Knight) {
		t.Fatal("Riddare should have been removed from Black's reserve")
	}
	if a.gs.Turn != game.White {
		t.Fatal("recruiting should consume the whole turn")
	}
}

// --- WIN: Duke capture via a real tap ---------------------------------------

func TestPlayHertigenDukeCaptureWin(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(a)
	// Black's Champion (Face A: adjacent MoveOrStrike, same as the Duke's
	// own shape) sits one step from White's undefended Duke.
	put(a, 3, 3, game.Champion, game.Black, game.FaceA)
	put(a, 3, 4, game.Duke, game.White, game.FaceA)
	put(a, 0, 0, game.Duke, game.Black, game.FaceA)
	a.gs.Turn = game.Black
	h.Draw()

	if !tapCell(h, a, 3, 3) {
		t.Fatal("selecting the Champion should be handled")
	}
	if !tapCell(h, a, 3, 4) {
		t.Fatal("the Duke-capturing move should be accepted")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once the White Duke is captured")
	}
	winner, ok := a.gs.Winner()
	if !ok || winner != game.Black {
		t.Fatalf("Winner() = (%v,%v), want (Black,true)", winner, ok)
	}
	if _, ok := h.FindTextContains("Svart vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- All 3 AI difficulties actually reply -----------------------------------

func TestPlayHertigenAllDifficultiesReply(t *testing.T) {
	for _, depth := range []int{game.DepthEasy, game.DepthMedium, game.DepthHard} {
		depth := depth
		t.Run(itoa(depth), func(t *testing.T) {
			h, a := bootToMenu(t)
			startOpponent(t, h, a, game.OpponentAI, depth)
			if a.gs.AIDepth != depth {
				t.Fatalf("AIDepth = %d, want %d", a.gs.AIDepth, depth)
			}
			act, ok := bestAction(a.gs)
			if !ok {
				t.Fatal("Black should have a legal opening action")
			}
			before := a.gs.Board
			if !tapAction(h, a, act) {
				t.Fatalf("Black's opening action %+v was rejected", act)
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

// --- Full game vs the AI, played to a real win/forfeit ----------------------

func TestPlayHertigenFullGameVsAI(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 400 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		act, ok := bestAction(a.gs)
		if !ok {
			t.Fatalf("human to move but no legal action at ply %d", ply)
		}
		if !tapAction(h, a, act) {
			t.Fatalf("legal action %+v at ply %d was rejected", act, ply)
		}
	}
	winner, ok := a.gs.Winner()
	want := "vann!"
	if ok {
		if winner == game.Black {
			want = "Svart vann!"
		} else {
			want = "Vit vann!"
		}
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("end banner %q not shown; visible: %v", want, texts(h))
	}
}

// --- Quit mid-game (Back key AND the Meny button), then restart ------------

func TestPlayHertigenQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	act, ok := bestAction(a.gs)
	if !ok {
		t.Fatal("expected a legal opening action")
	}
	tapAction(h, a, act) // a move in progress

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
	startOpponent(t, h, a, game.OpponentHotseat, 0)
}

// --- "Ny" restarts the current configuration mid-game -----------------------

func TestPlayHertigenNyRestarts(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	act, ok := bestAction(a.gs)
	if !ok {
		t.Fatal("expected a legal opening action")
	}
	tapAction(h, a, act)
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
	if a.gs.Turn != game.Black || a.gs.Board.Count(game.Black) != 3 || a.gs.Board.Count(game.White) != 3 {
		t.Fatal("Ny should reset to a fresh starting position (Duke + Fotknekt + Kämpe per side)")
	}
}

// --- Rules and legend screens ------------------------------------------------

func TestPlayHertigenRulesAndLegendScreens(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("Baserat på The Duke"); !ok {
		t.Fatalf("rules text missing the credit line; visible: %v", texts(h))
	}

	if err := h.TapText("Pjäslegend"); err != nil {
		t.Fatalf("no Pjäslegend button on the rules screen: %v", err)
	}
	if a.screen != screenLegend {
		t.Fatalf("Pjäslegend did not open the legend, screen=%v", a.screen)
	}
	// Every tile type's Swedish name should be listed.
	for _, name := range []string{"Hertig", "Fotknekt", "Riddare", "Ryttare", "Diagonalvakt", "Katapult", "Kämpe"} {
		if _, ok := h.FindTextContains(name); !ok {
			t.Fatalf("legend missing tile type %q; visible: %v", name, texts(h))
		}
	}

	h.Back()
	if a.screen != screenRules {
		t.Fatalf("Back from legend should return to rules, screen=%v", a.screen)
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back from rules should return to menu, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayHertigenScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/hertigen_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/hertigen_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/hertigen_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Pjäslegend")
	if err := h.Screenshot(dir + "/hertigen_legend.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()
	h.Back()

	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)
	tapCell(h, a, 1, game.Size-1) // select Black's starting Fotknekt (Black's home row)
	if err := h.Screenshot(dir + "/hertigen_selection.png"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3 && a.gs.Phase == game.PhasePlaying; i++ {
		act, ok := bestAction(a.gs)
		if !ok || a.gs.AITurn() {
			break
		}
		tapAction(h, a, act)
	}
	if err := h.Screenshot(dir + "/hertigen_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	clearBoard(a)
	put(a, 3, 3, game.Champion, game.Black, game.FaceA)
	put(a, 3, 4, game.Duke, game.White, game.FaceA)
	put(a, 0, 0, game.Duke, game.Black, game.FaceA)
	a.gs.Turn = game.Black
	a.gs.Phase = game.PhasePlaying
	a.mode = selNone
	h.Draw()
	tapCell(h, a, 3, 3)
	tapCell(h, a, 3, 4)
	if err := h.Screenshot(dir + "/hertigen_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
