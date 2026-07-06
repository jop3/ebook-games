//go:build playtest

package main

// Headless PLAYTHROUGH tests for Breakthrough. They drive the real touch
// path and check the gameplay against the rules as written (see
// rulesParagraphs in ui.go): Black starts, fills the bottom two ranks and
// advances toward row 0; White fills the top two ranks and advances toward
// the bottom row. A pawn moves one step straight onto an empty square
// (never a capture) or one step diagonally onto an enemy pawn (always a
// capture, never onto an empty square). Win by reaching the opponent's back
// rank, by eliminating every enemy pawn, or if the opponent has no legal
// move. Covers all 3 AI difficulties, all 3 win conditions, illegal-move
// rejection (including the classic straight-captures/diagonal-quiet swap),
// quitting, and the rules screen. Runs under the pure-Go inkview emulator
// (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"breakthrough/game"
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

// tapMove drives a full Breakthrough move through the real UI: tap the
// origin (selecting the pawn), then tap the destination.
func tapMove(h *ink.Harness, a *app, m game.Move) bool {
	if !h.TapRect(a.layout.CellToScreen(m.From.X, m.From.Y)) {
		return false
	}
	return h.TapRect(a.layout.CellToScreen(m.To.X, m.To.Y))
}

func tapCell(h *ink.Harness, a *app, p image.Point) bool {
	return h.TapRect(a.layout.CellToScreen(p.X, p.Y))
}

// setCell places a pawn directly on the board, bypassing move legality —
// used to construct specific test positions. Board is a plain
// [Rows][Cols]Cell array (indexed [y][x], matching the game package's own
// convention), so external packages can index it directly without an
// exported setter.
func setCell(b *game.Board, x, y int, c game.Cell) {
	b[y][x] = c
}

func clearBoard(a *app) {
	for i := range a.gs.Board {
		for j := range a.gs.Board[i] {
			a.gs.Board[i][j] = game.Empty
		}
	}
}

// manTotal returns black+white+empty (must always be Cols*Rows).
func manTotal(b *game.Board) int {
	return b.Count(game.Black) + b.Count(game.White) + b.Count(game.Empty)
}

// bestByCaptures returns a legal move for color that captures if one is
// available (max=true), or simply the first available move (max=false) — a
// deterministic policy for full-game playthroughs against the AI.
func bestByCaptures(b *game.Board, color game.Cell, max bool) (game.Move, bool) {
	moves := b.LegalMoves(color)
	if len(moves) == 0 {
		return game.Move{}, false
	}
	if !max {
		return moves[0], true
	}
	for _, m := range moves {
		if m.Capture {
			return m, true
		}
	}
	return moves[0], true
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- RULE: straight moves, illegal rejection ----------------------------

func TestPlayBreakthroughMoveRules(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	legal := a.gs.Board.LegalMoves(game.Black)
	if len(legal) == 0 {
		t.Fatal("Black should have legal moves at the start")
	}

	// Tapping an occupied square (another Black pawn on the home rank)
	// after selecting one must switch the selection, not move/capture.
	blackBefore := a.gs.Board.Count(game.Black)
	tapCell(h, a, image.Pt(0, game.Rows-1)) // select
	tapCell(h, a, image.Pt(1, game.Rows-1)) // occupied by another Black pawn
	if a.gs.Board.Count(game.Black) != blackBefore {
		t.Fatal("tapping an occupied square must not remove/duplicate a pawn")
	}
	if a.gs.Turn != game.Black {
		t.Fatal("switching selection must not change the turn")
	}

	// A legal move matches a pure Apply on a copy exactly.
	m := legal[0]
	want := a.gs.Board.Apply(m)
	if !tapMove(h, a, m) {
		t.Fatalf("legal move %v via tap was rejected", m)
	}
	if a.gs.Board != want {
		t.Fatalf("UI move %v did not match the rules' own Apply result", m)
	}
	if a.gs.Turn != game.White {
		t.Fatal("turn did not pass to White after a legal move")
	}
	if manTotal(&a.gs.Board) != game.Cols*game.Rows {
		t.Fatalf("board inconsistent: total %d", manTotal(&a.gs.Board))
	}
}

// --- GOTCHA (the classic chess-muscle-memory bug): straight onto an enemy
// pawn must be rejected; diagonal onto an empty square must be rejected;
// diagonal onto an enemy pawn must capture. Driven through the real tap
// path for BOTH sides, since direction is mirror-imaged per side.

func TestPlayBreakthroughStraightNeverCapturesBlack(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(a)
	setCell(&a.gs.Board, 3, 3, game.Black)
	setCell(&a.gs.Board, 3, 2, game.White) // straight ahead of Black
	a.gs.Turn = game.Black
	h.Draw()

	if tapMove(h, a, game.Move{From: image.Pt(3, 3), To: image.Pt(3, 2)}) {
		t.Fatal("a straight move onto an enemy pawn must be rejected — straight never captures")
	}
	if a.gs.Board.At(3, 2) != game.White || a.gs.Board.At(3, 3) != game.Black {
		t.Fatal("the rejected move must not have changed the board at all")
	}
	if a.gs.Turn != game.Black {
		t.Fatal("a rejected move must not change the turn")
	}
}

func TestPlayBreakthroughDiagonalOntoEmptyIllegalBlack(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(a)
	setCell(&a.gs.Board, 3, 3, game.Black) // no enemy on either diagonal
	a.gs.Turn = game.Black
	h.Draw()

	if tapMove(h, a, game.Move{From: image.Pt(3, 3), To: image.Pt(2, 2)}) {
		t.Fatal("a diagonal move onto an empty square must be rejected — diagonal never moves quietly")
	}
	if a.gs.Board.At(3, 3) != game.Black {
		t.Fatal("the pawn must not have moved")
	}
}

func TestPlayBreakthroughDiagonalCapturesBlack(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(a)
	setCell(&a.gs.Board, 3, 3, game.Black)
	setCell(&a.gs.Board, 2, 2, game.White) // forward-left diagonal
	a.gs.Turn = game.Black
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(3, 3), To: image.Pt(2, 2)}) {
		t.Fatal("a diagonal move onto an enemy pawn must be legal — diagonal always captures")
	}
	if a.gs.Board.At(2, 2) != game.Black {
		t.Fatal("the mover should now occupy (2,2)")
	}
	if a.gs.Board.Count(game.White) != 0 {
		t.Fatal("the captured White pawn should be gone")
	}
	if !a.gs.LastCaptured {
		t.Fatal("LastCaptured should be true after a diagonal capture")
	}
}

// Mirror image: White's forward direction is +1 in y, the opposite of
// Black's — direction must never be hardcoded.

func TestPlayBreakthroughStraightNeverCapturesWhite(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(a)
	setCell(&a.gs.Board, 3, 2, game.White)
	setCell(&a.gs.Board, 3, 3, game.Black) // straight ahead of White
	a.gs.Turn = game.White
	h.Draw()

	if tapMove(h, a, game.Move{From: image.Pt(3, 2), To: image.Pt(3, 3)}) {
		t.Fatal("White straight move onto an enemy pawn must be rejected")
	}
}

func TestPlayBreakthroughDiagonalCapturesWhite(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(a)
	setCell(&a.gs.Board, 3, 2, game.White)
	setCell(&a.gs.Board, 4, 3, game.Black) // forward-right diagonal for White
	a.gs.Turn = game.White
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(3, 2), To: image.Pt(4, 3)}) {
		t.Fatal("White diagonal move onto an enemy pawn must be legal")
	}
	if a.gs.Board.At(4, 3) != game.White || a.gs.Board.Count(game.Black) != 0 {
		t.Fatal("White should have captured Black's pawn at (4,3)")
	}
}

// --- WIN 1: reaching the opponent's back rank -------------------------

func TestPlayBreakthroughWinByReachingGoalRank(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(a)
	setCell(&a.gs.Board, 3, 1, game.Black) // one step from row 0
	setCell(&a.gs.Board, 0, 4, game.White)
	a.gs.Turn = game.Black
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(3, 1), To: image.Pt(3, 0)}) {
		t.Fatal("the winning advance should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once Black reaches row 0")
	}
	if _, ok := h.FindTextContains("Svart vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- WIN 2: eliminating every enemy pawn --------------------------------

func TestPlayBreakthroughWinByEliminatingLastPawn(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(a)
	setCell(&a.gs.Board, 3, 3, game.Black)
	setCell(&a.gs.Board, 2, 2, game.White) // White's only pawn
	a.gs.Turn = game.Black
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(3, 3), To: image.Pt(2, 2)}) {
		t.Fatal("the winning capture should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once White has zero pawns")
	}
	if _, ok := h.FindTextContains("Svart vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- WIN 3: the opponent has no legal move ------------------------------

func TestPlayBreakthroughWinByOpponentNoLegalMove(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(a)
	// After White's move below, Black (to move) has exactly one pawn, dead
	// ahead of a White pawn — straight is blocked (occupied), and both
	// diagonals are empty (a diagonal onto empty is illegal for a pawn) —
	// so Black has zero legal moves anywhere on the board.
	setCell(&a.gs.Board, 3, 3, game.Black)
	setCell(&a.gs.Board, 3, 2, game.White)
	setCell(&a.gs.Board, 0, 0, game.White) // White's move-making pawn
	a.gs.Turn = game.White
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(0, 0), To: image.Pt(0, 1)}) {
		t.Fatal("White's setup move should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once Black (to move) has no legal move")
	}
	if _, ok := h.FindTextContains("Vit vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- All 3 AI difficulties actually reply -------------------------------

func TestPlayBreakthroughAllDifficultiesReply(t *testing.T) {
	for _, depth := range []int{game.DepthEasy, game.DepthMedium, game.DepthHard} {
		depth := depth
		t.Run(itoa(depth), func(t *testing.T) {
			h, a := bootToMenu(t)
			startOpponent(t, h, a, game.OpponentAI, depth)
			if a.gs.AIDepth != depth {
				t.Fatalf("AIDepth = %d, want %d", a.gs.AIDepth, depth)
			}
			legal := a.gs.Board.LegalMoves(game.Black)
			before := a.gs.Board
			if !tapMove(h, a, legal[0]) {
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

// --- Full game vs the AI, played to a real win -------------------------

func TestPlayBreakthroughFullGameVsAI(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 400 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		m, ok := bestByCaptures(&a.gs.Board, a.gs.Turn, true)
		if !ok {
			t.Fatalf("human to move but no legal move at ply %d", ply)
		}
		if !tapMove(h, a, m) {
			t.Fatalf("legal move %v at ply %d was rejected", m, ply)
		}
		if manTotal(&a.gs.Board) != game.Cols*game.Rows {
			t.Fatalf("board inconsistent mid-game: %d", manTotal(&a.gs.Board))
		}
	}
	bl, wh := a.gs.Board.Count(game.Black), a.gs.Board.Count(game.White)
	want := ""
	switch a.gs.Winner() {
	case game.Black:
		want = "Svart vann!"
	case game.White:
		want = "Vit vann!"
	default:
		t.Fatalf("game ended with no winner (B%d/W%d) — Breakthrough has no draws", bl, wh)
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("end banner %q (B%d/W%d) not shown; visible: %v", want, bl, wh, texts(h))
	}
}

// A full hot-seat game, driving BOTH colors through the real tap path, must
// also terminate with a decisive winner.
func TestPlayBreakthroughFullHotseatGame(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 400 {
			t.Fatal("game did not terminate")
		}
		m, ok := bestByCaptures(&a.gs.Board, a.gs.Turn, true)
		if !ok {
			t.Fatalf("%v to move but no legal move at ply %d", a.gs.Turn, ply)
		}
		if !tapMove(h, a, m) {
			t.Fatalf("legal move %v at ply %d was rejected", m, ply)
		}
	}
	if a.gs.Winner() != game.Black && a.gs.Winner() != game.White {
		t.Fatalf("hot-seat game ended with no decisive winner: %v", a.gs.Winner())
	}
}

// --- Input guards: no input accepted once the game has ended -----------

func TestPlayBreakthroughNoInputAfterGameEnds(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(a)
	setCell(&a.gs.Board, 3, 1, game.Black)
	setCell(&a.gs.Board, 0, 4, game.White)
	a.gs.Turn = game.Black
	h.Draw()
	tapMove(h, a, game.Move{From: image.Pt(3, 1), To: image.Pt(3, 0)})
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("setup: game should have ended")
	}
	before := a.gs.Board
	tapCell(h, a, image.Pt(0, 4)) // try to select White's pawn after the game ended
	tapCell(h, a, image.Pt(0, 3))
	if a.gs.Board != before {
		t.Fatal("no move should be accepted once the game has ended")
	}
}

// --- Quit mid-game (Back key AND the Meny button), then restart --------

func TestPlayBreakthroughQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	m := a.gs.Board.LegalMoves(game.Black)[0]
	tapMove(h, a, m) // a move in progress

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

// --- "Ny" restarts the current configuration mid-game -------------------

func TestPlayBreakthroughNyRestarts(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	m := a.gs.Board.LegalMoves(game.Black)[0]
	tapMove(h, a, m)
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
	if a.gs.Turn != game.Black || a.gs.Board.Count(game.Black) != game.Cols*2 || a.gs.Board.Count(game.White) != game.Cols*2 {
		t.Fatal("Ny should reset to a fresh starting position")
	}
}

// --- Rules screen --------------------------------------------------------

func TestPlayBreakthroughRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("aldrig"); !ok {
		t.Fatalf("rules text missing the straight/diagonal distinction; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review ----------------------

func TestPlayBreakthroughScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/breakthrough_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/breakthrough_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/breakthrough_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)
	for i := 0; i < 3 && a.gs.Phase == game.PhasePlaying; i++ {
		m, ok := bestByCaptures(&a.gs.Board, a.gs.Turn, true)
		if !ok || a.gs.AITurn() {
			break
		}
		tapMove(h, a, m)
	}
	if err := h.Screenshot(dir + "/breakthrough_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// End-game banner.
	clearBoard(a)
	setCell(&a.gs.Board, 3, 0, game.Black)
	setCell(&a.gs.Board, 0, 4, game.White)
	a.gs.Phase = game.PhaseDone
	h.Draw()
	if err := h.Screenshot(dir + "/breakthrough_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
