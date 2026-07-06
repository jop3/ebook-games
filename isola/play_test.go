//go:build playtest

package main

// Headless PLAYTHROUGH tests for Isola. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): a turn is (1) move the pawn any distance in one of the 8 queen
// directions, stopping dead at (never past) the first missing tile or the
// opponent's pawn, then (2) remove any one present tile except the square
// just landed on — the square just vacated is fair game. A side loses the
// instant it has zero legal pawn moves on its own turn. Covers both opponent
// modes (all 3 AI difficulties), a full game played to a real win, the two
// signature gotchas (no jumping over a gap; the new position is excluded
// from removal but the old one isn't), illegal-move rejection, quitting, and
// the rules screen. Runs under the pure-Go inkview emulator
// (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"isola/game"
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
// AI, the given search depth), taps it, and enters the game.
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

// tapCell taps the board cell at p (whatever it means right now — a move
// destination or a removal target — depends on gs.Step) and reports whether
// the tap was handled (i.e. accepted as a legal action).
func tapCell(h *ink.Harness, a *app, p image.Point) bool {
	return h.TapRect(a.layout.CellToScreen(p.X, p.Y))
}

// tapTurn drives one full Isola turn through the real UI: tap the move
// destination, then tap the tile to remove.
func tapTurn(h *ink.Harness, a *app, to, remove image.Point) bool {
	if !tapCell(h, a, to) {
		return false
	}
	return tapCell(h, a, remove)
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- RULE: queen movement, illegal rejection, two-step turn -----------------

func TestPlayIsolaMoveRules(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	legal := a.gs.Board.LegalMoves(game.Black)
	if len(legal) == 0 {
		t.Fatal("Black should have legal moves at the start")
	}

	// Tapping an illegal destination (White's own pawn square) must be
	// rejected: no pawn moved, still in the move step.
	before := a.gs.Board
	if tapCell(h, a, a.gs.Board.WhitePawn) {
		t.Fatal("tapping the opponent's pawn square must not be accepted as a move")
	}
	if a.gs.Board != before {
		t.Fatal("a rejected move must not change the board")
	}
	if a.gs.Step != game.StepMove {
		t.Fatal("a rejected move must not advance to the removal step")
	}

	// A legal move matches the destination set exactly, and only advances
	// the turn to the removal step — Turn does not pass yet.
	m := legal[0]
	if !tapCell(h, a, m) {
		t.Fatalf("legal move to %v via tap was rejected", m)
	}
	if a.gs.Board.BlackPawn != m {
		t.Fatalf("BlackPawn = %v, want %v", a.gs.Board.BlackPawn, m)
	}
	if a.gs.Step != game.StepRemove || a.gs.PendingTo != m {
		t.Fatal("move should advance to the removal step with PendingTo = the new position")
	}
	if a.gs.Turn != game.Black {
		t.Fatal("the turn must not pass until the removal half is also played")
	}
}

// --- GOTCHA: no jumping over a missing tile ---------------------------------

func TestPlayIsolaGapBlocksMove(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	a.gs.Board = game.NewBoard()
	a.gs.Board.BlackPawn = image.Pt(0, 0)
	a.gs.Board.WhitePawn = image.Pt(7, 7)
	a.gs.Board.Present[0][2] = false // (2,0): a gap two squares right of Black
	a.gs.Turn = game.Black
	a.gs.Step = game.StepMove
	h.Draw()

	if tapCell(h, a, image.Pt(3, 0)) {
		t.Fatal("a queen move must not jump over a missing tile")
	}
	if a.gs.Step != game.StepMove || a.gs.Board.BlackPawn != (image.Point{X: 0, Y: 0}) {
		t.Fatal("the rejected jump must not have moved the pawn")
	}

	// The square just short of the gap is still legal.
	if !tapCell(h, a, image.Pt(1, 0)) {
		t.Fatal("moving up to (but not past) the gap should be legal")
	}
	if a.gs.Board.BlackPawn != (image.Point{X: 1, Y: 0}) {
		t.Fatalf("BlackPawn = %v, want (1,0)", a.gs.Board.BlackPawn)
	}
}

// --- GOTCHA: the new position is excluded from removal, the old one isn't --

func TestPlayIsolaCannotRemoveNewPosition(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	from := a.gs.Board.BlackPawn
	to := a.gs.Board.LegalMoves(game.Black)[0]
	if !tapCell(h, a, to) {
		t.Fatalf("the move half of the turn should be legal")
	}

	// Tapping the square the pawn just landed on must be rejected as a
	// removal target.
	if tapCell(h, a, to) {
		t.Fatal("removing the mover's own new position must be rejected")
	}
	if a.gs.Step != game.StepRemove {
		t.Fatal("a rejected removal must not advance the turn")
	}

	// But the square it just vacated is fair game.
	if !tapCell(h, a, from) {
		t.Fatal("removing the square just vacated should be legal")
	}
	if a.gs.Board.IsPresent(from.X, from.Y) {
		t.Fatal("the vacated square should now be missing")
	}
	if a.gs.Turn != game.White {
		t.Fatal("turn should pass to White once the removal completes the turn")
	}
}

// --- WIN: cornering the opponent's pawn --------------------------------------

func TestPlayIsolaWin(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	// White is boxed into the corner with exactly one escape: (1,1). Black,
	// elsewhere on the board, moves and then removes that one remaining
	// escape tile — the classic Isola finish.
	a.gs.Board = game.NewBoard()
	a.gs.Board.WhitePawn = image.Pt(0, 0)
	a.gs.Board.BlackPawn = image.Pt(7, 7)
	a.gs.Board.Present[0][1] = false // (1,0): blocks White's rightward ray
	a.gs.Board.Present[1][0] = false // (0,1): blocks White's downward ray
	a.gs.Board.Present[2][2] = false // (2,2): caps White's diagonal ray at (1,1)
	a.gs.Turn = game.Black
	a.gs.Step = game.StepMove
	a.gs.Phase = game.PhasePlaying
	h.Draw()

	white := a.gs.Board.LegalMoves(game.White)
	if len(white) != 1 || white[0] != (image.Point{X: 1, Y: 1}) {
		t.Fatalf("setup: White should have exactly one legal move (1,1), got %v", white)
	}

	if !tapTurn(h, a, image.Pt(6, 7), image.Pt(1, 1)) {
		t.Fatal("Black's winning turn should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once White has zero legal moves")
	}
	if a.gs.Winner() != game.Black {
		t.Fatalf("Winner() = %v, want Black", a.gs.Winner())
	}
	if _, ok := h.FindTextContains("Svart vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- All 3 AI difficulties actually reply -----------------------------------

func TestPlayIsolaAllDifficultiesReply(t *testing.T) {
	for _, depth := range []int{game.DepthEasy, game.DepthMedium, game.DepthHard} {
		depth := depth
		t.Run(itoa(depth), func(t *testing.T) {
			h, a := bootToMenu(t)
			startOpponent(t, h, a, game.OpponentAI, depth)
			if a.gs.AIDepth != depth {
				t.Fatalf("AIDepth = %d, want %d", a.gs.AIDepth, depth)
			}
			before := a.gs.Board
			m, ok := game.BestMove(a.gs.Board, game.Black, game.DepthEasy)
			if !ok {
				t.Fatal("Black should have a legal opening turn")
			}
			if !tapTurn(h, a, m.To, m.Remove) {
				t.Fatalf("Black's opening turn %+v should be legal", m)
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

func TestPlayIsolaFullGameVsAI(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 80 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		before := a.gs.Board.TotalPresent()
		m, ok := game.BestMove(a.gs.Board, a.gs.Turn, game.DepthEasy)
		if !ok {
			t.Fatalf("human to move but no legal move at ply %d", ply)
		}
		if !tapTurn(h, a, m.To, m.Remove) {
			t.Fatalf("legal turn %+v at ply %d was rejected", m, ply)
		}
		if a.gs.Board.TotalPresent() >= before {
			t.Fatalf("tile count must strictly decrease each turn: before=%d after=%d", before, a.gs.Board.TotalPresent())
		}
	}

	var want string
	switch a.gs.Winner() {
	case game.Black:
		want = "Svart vann!"
	case game.White:
		want = "Vit vann!"
	default:
		t.Fatal("game ended with no winner, which should be impossible in Isola")
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("end banner %q not shown; visible: %v", want, texts(h))
	}
}

// --- Quit mid-game (Back key AND the Meny button), then restart ------------

func TestPlayIsolaQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	tapCell(h, a, a.gs.Board.LegalMoves(game.Black)[0]) // a move in progress

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

func TestPlayIsolaNyRestarts(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	tapCell(h, a, a.gs.Board.LegalMoves(game.Black)[0])
	if a.gs.Step != game.StepRemove {
		t.Fatal("setup: expected the move half of the turn to have been played")
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
	if a.gs.Turn != game.Black || a.gs.Step != game.StepMove || a.gs.Board.TotalPresent() != game.Size*game.Size {
		t.Fatal("Ny should reset to a fresh starting position")
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlayIsolaRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("hoppa"); !ok {
		t.Fatalf("rules text missing the no-jump-over-a-gap rule; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayIsolaScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/isola_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/isola_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/isola_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)
	for i := 0; i < 3 && a.gs.Phase == game.PhasePlaying; i++ {
		if a.gs.AITurn() {
			break
		}
		m, ok := game.BestMove(a.gs.Board, a.gs.Turn, game.DepthEasy)
		if !ok {
			break
		}
		tapTurn(h, a, m.To, m.Remove)
	}
	if err := h.Screenshot(dir + "/isola_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// End-game banner: White fully boxed in, zero legal moves.
	a.gs.Board = game.NewBoard()
	a.gs.Board.WhitePawn = image.Pt(0, 0)
	a.gs.Board.BlackPawn = image.Pt(7, 7)
	a.gs.Board.Present[0][1] = false
	a.gs.Board.Present[1][0] = false
	a.gs.Board.Present[1][1] = false
	a.gs.Turn = game.White
	a.gs.Phase = game.PhaseDone
	a.gs.HasLast = false // clear the stale "last removed" marker from moves played earlier in this test
	h.Draw()
	if err := h.Screenshot(dir + "/isola_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
