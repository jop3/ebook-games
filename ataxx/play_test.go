//go:build playtest

package main

// Headless PLAYTHROUGH tests for Ataxx. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): a CLONE (distance 1, any of the 8 directions) leaves the source
// occupied and creates a new man at the destination; a JUMP (distance 2)
// vacates the source; either way every one of the 8 neighbors of the
// destination that holds an enemy man flips to the mover's color — not just
// the 4 orthogonal ones. The game ends when the board fills or the side to
// move has no legal move (running out of moves is NOT an automatic loss —
// the higher piece count decides, ties are possible). Covers both opponent
// modes (all 3 AI difficulties), illegal-move rejection, quitting, and the
// rules screen. Runs under the pure-Go inkview emulator (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"ataxx/game"
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

// tapMove drives a full Ataxx move through the real UI: tap the origin
// (selecting the man), then tap the destination.
func tapMove(h *ink.Harness, a *app, m game.Move) bool {
	if !h.TapRect(a.layout.CellToScreen(m.From.X, m.From.Y)) {
		return false
	}
	return h.TapRect(a.layout.CellToScreen(m.To.X, m.To.Y))
}

func tapCell(h *ink.Harness, a *app, p image.Point) bool {
	return h.TapRect(a.layout.CellToScreen(p.X, p.Y))
}

// setCell places a man directly on the board, bypassing move legality — used
// to construct specific test positions. Board is a plain [Size][Size]Cell
// array (indexed [y][x], matching the game package's own convention), so
// external packages can index it directly without an exported setter.
func setCell(b *game.Board, x, y int, c game.Cell) {
	b[y][x] = c
}

func clearBoard(b *game.Board) {
	for y := range b {
		for x := range b[y] {
			b[y][x] = game.Empty
		}
	}
}

// manTotal returns black+white+empty (must always be Size*Size).
func manTotal(b *game.Board) int {
	return b.Count(game.Black) + b.Count(game.White) + b.Count(game.Empty)
}

// bestByFlips returns the legal move for side that flips the most enemy men
// (max=true) or simply the first available (max=false) — a deterministic
// policy for full-game playthroughs against the AI.
func bestByFlips(b *game.Board, side game.Cell, max bool) (game.Move, bool) {
	moves := b.LegalMoves(side)
	if len(moves) == 0 {
		return game.Move{}, false
	}
	if !max {
		return moves[0], true
	}
	best := moves[0]
	bestN := -1
	for _, m := range moves {
		_, flipped := b.Apply(m)
		if len(flipped) > bestN {
			bestN, best = len(flipped), m
		}
	}
	return best, true
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- RULE: clone/jump legality, illegal rejection ---------------------------

func TestPlayAtaxxMoveRules(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	legal := a.gs.Board.LegalMoves(game.Black)
	if len(legal) == 0 {
		t.Fatal("Black should have legal moves at the start")
	}

	// Tapping another own man after selecting one must switch the selection,
	// not apply an illegal move (distance 0 / occupied destination).
	blackBefore := a.gs.Board.Count(game.Black)
	tapCell(h, a, image.Pt(0, 0))                     // select Black's top-left man
	tapCell(h, a, image.Pt(game.Size-1, game.Size-1)) // Black's other man (too far, not empty anyway)
	if a.gs.Board.Count(game.Black) != blackBefore {
		t.Fatal("tapping another own man must not remove/duplicate a man")
	}
	if a.gs.Turn != game.Black {
		t.Fatal("switching selection must not change the turn")
	}

	// A legal move matches a pure Apply on a copy exactly.
	m := legal[0]
	wantBoard, wantFlipped := a.gs.Board.Apply(m)
	if !tapMove(h, a, m) {
		t.Fatalf("legal move %v via tap was rejected", m)
	}
	if a.gs.Board != wantBoard {
		t.Fatalf("UI move %v did not match the rules' own Apply result", m)
	}
	if a.gs.Turn != game.White {
		t.Fatal("turn did not pass to White after a legal move")
	}
	if manTotal(&a.gs.Board) != game.Size*game.Size {
		t.Fatalf("board inconsistent: total %d", manTotal(&a.gs.Board))
	}
	_ = wantFlipped
}

// --- GOTCHA: clone leaves the source occupied via a real tap ----------------

func TestPlayAtaxxCloneLeavesSourceOccupied(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 3, 3, game.Black)
	a.gs.Turn = game.Black
	h.Draw()

	m := game.Move{From: image.Pt(3, 3), To: image.Pt(4, 3)} // distance 1: clone
	if !tapMove(h, a, m) {
		t.Fatal("the clone move should be legal")
	}
	if a.gs.Board.At(3, 3) != game.Black {
		t.Fatal("a clone must leave the source man in place")
	}
	if a.gs.Board.At(4, 3) != game.Black {
		t.Fatal("a clone must place a new man at the destination")
	}
	if a.gs.Board.Count(game.Black) != 2 {
		t.Fatalf("clone should increase Black's count by one, got %d", a.gs.Board.Count(game.Black))
	}
}

// --- GOTCHA: jump vacates the source via a real tap -------------------------

func TestPlayAtaxxJumpVacatesSource(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 3, 3, game.Black)
	a.gs.Turn = game.Black
	h.Draw()

	m := game.Move{From: image.Pt(3, 3), To: image.Pt(5, 3)} // distance 2: jump
	if !tapMove(h, a, m) {
		t.Fatal("the jump move should be legal")
	}
	if a.gs.Board.At(3, 3) != game.Empty {
		t.Fatal("a jump must vacate the source cell")
	}
	if a.gs.Board.At(5, 3) != game.Black {
		t.Fatal("a jump must place the man at the destination")
	}
	if a.gs.Board.Count(game.Black) != 1 {
		t.Fatalf("jump must not change Black's total count, got %d", a.gs.Board.Count(game.Black))
	}
}

// --- GOTCHA: flip check scans all 8 neighbors of the destination ------------

func TestPlayAtaxxFlipsAllEightNeighbors(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 0, 0, game.Black) // mover: jumps to (2,2)
	// All 8 neighbors of the destination (2,2), including the 4 DIAGONAL
	// ones — a 4-neighbor-only bug would miss (1,1),(3,1),(1,3),(3,3).
	neighbors := []image.Point{
		{X: 1, Y: 1}, {X: 2, Y: 1}, {X: 3, Y: 1},
		{X: 1, Y: 2}, {X: 3, Y: 2},
		{X: 1, Y: 3}, {X: 2, Y: 3}, {X: 3, Y: 3},
	}
	for _, p := range neighbors {
		setCell(&a.gs.Board, p.X, p.Y, game.White)
	}
	a.gs.Turn = game.Black
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(0, 0), To: image.Pt(2, 2)}) {
		t.Fatal("the jump-and-flip move should be legal")
	}
	for _, p := range neighbors {
		if a.gs.Board.At(p.X, p.Y) != game.Black {
			t.Fatalf("neighbor %v should have flipped to Black, still %v", p, a.gs.Board.At(p.X, p.Y))
		}
	}
	if a.gs.Board.Count(game.White) != 0 {
		t.Fatalf("all 8 White neighbors should have flipped, %d remain", a.gs.Board.Count(game.White))
	}
	if len(a.gs.LastFlipped) != 8 {
		t.Fatalf("LastFlipped = %v, want all 8 flipped cells", a.gs.LastFlipped)
	}
}

// --- WIN: board fills, higher piece count wins ------------------------------

func TestPlayAtaxxWinByPieceCount(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	clearBoard(&a.gs.Board)
	// Fill the whole board except one cell, Black outnumbering White, then
	// have Black play the board-filling move.
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			setCell(&a.gs.Board, x, y, game.Black)
		}
	}
	setCell(&a.gs.Board, 6, 6, game.White)
	setCell(&a.gs.Board, 0, 0, game.Empty) // the one gap, adjacent to (1,1) Black
	a.gs.Turn = game.Black
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(1, 1), To: image.Pt(0, 0)}) {
		t.Fatal("the board-filling move should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once the board fills")
	}
	if _, ok := h.FindTextContains("Svart vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- WIN: game ends the instant the side to move is boxed in (no pass) -----
//
// Note: that a stuck side is NOT automatically the loser (only the final
// piece count decides — see the doc comment on GameState.advance) is already
// unit-tested directly in game/state_test.go
// (TestGameOverWinnerIsCountBasedNotAutomaticLoss). This playtest instead
// exercises the same "no legal move ends the game" trigger through the real
// UI and confirms the banner matches the actual piece count on the real
// board, computed independently of GameState.Winner so the assertion isn't
// circular.
func TestPlayAtaxxNoLegalMoveEndsGameByCount(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	clearBoard(&a.gs.Board)
	// White's only man, at corner (6,6), is boxed in by Black on its full
	// clone ring (5,5)/(5,6)/(6,5) and jump ring
	// (4,4)/(4,5)/(4,6)/(5,4)/(6,4) — except (6,4), left empty so White still
	// has one legal jump before Black's move. Black then clones (5,4)->(6,4)
	// to complete the box; (6,4) is not adjacent to (6,6) so this move does
	// not flip White's man.
	setCell(&a.gs.Board, 6, 6, game.White)
	for _, p := range []image.Point{
		{X: 5, Y: 5}, {X: 5, Y: 6}, {X: 6, Y: 5},
		{X: 4, Y: 4}, {X: 4, Y: 5}, {X: 4, Y: 6}, {X: 5, Y: 4},
	} {
		setCell(&a.gs.Board, p.X, p.Y, game.Black)
	}
	a.gs.Turn = game.Black
	h.Draw()

	// Sanity: before the move, White still has a legal move (jump to (6,4)).
	if len(a.gs.Board.LegalMoves(game.White)) == 0 {
		t.Fatal("setup error: White should still have a legal move before Black plays")
	}

	if !tapMove(h, a, game.Move{From: image.Pt(5, 4), To: image.Pt(6, 4)}) {
		t.Fatal("Black's boxing-in move should be legal")
	}
	if a.gs.Board.At(6, 6) != game.White {
		t.Fatal("setup error: the boxing move must not flip White's man")
	}
	if len(a.gs.Board.LegalMoves(game.White)) != 0 {
		t.Fatal("setup error: White should now have zero legal moves")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once the side to move has no legal move")
	}
	bl, wh := a.gs.Board.Count(game.Black), a.gs.Board.Count(game.White)
	want := "Oavgjort!"
	switch {
	case bl > wh:
		want = "Svart vann!"
	case wh > bl:
		want = "Vit vann!"
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("end banner %q (B%d/W%d) not shown; visible: %v", want, bl, wh, texts(h))
	}
}

// --- All 3 AI difficulties actually reply -----------------------------------

func TestPlayAtaxxAllDifficultiesReply(t *testing.T) {
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

// --- Full game vs the AI, played to a real win/loss/tie ---------------------

func TestPlayAtaxxFullGameVsAI(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 200 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		m, ok := bestByFlips(&a.gs.Board, a.gs.Turn, true)
		if !ok {
			t.Fatalf("human to move but no legal move at ply %d", ply)
		}
		if !tapMove(h, a, m) {
			t.Fatalf("legal move %v at ply %d was rejected", m, ply)
		}
		if manTotal(&a.gs.Board) != game.Size*game.Size {
			t.Fatalf("board inconsistent mid-game: %d", manTotal(&a.gs.Board))
		}
	}
	bl, wh := a.gs.Board.Count(game.Black), a.gs.Board.Count(game.White)
	want := "Oavgjort!"
	switch a.gs.Winner() {
	case game.Black:
		want = "Svart vann!"
	case game.White:
		want = "Vit vann!"
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("end banner %q (B%d/W%d) not shown; visible: %v", want, bl, wh, texts(h))
	}
}

// --- Full game hot-seat, played to a real win/loss/tie ----------------------

func TestPlayAtaxxFullGameHotseat(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 200 {
			t.Fatal("game did not terminate")
		}
		m, ok := bestByFlips(&a.gs.Board, a.gs.Turn, true)
		if !ok {
			t.Fatalf("no legal move at ply %d while game still playing", ply)
		}
		if !tapMove(h, a, m) {
			t.Fatalf("legal move %v at ply %d was rejected", m, ply)
		}
	}
	if manTotal(&a.gs.Board) != game.Size*game.Size {
		t.Fatalf("board inconsistent at game end: %d", manTotal(&a.gs.Board))
	}
	bl, wh := a.gs.Board.Count(game.Black), a.gs.Board.Count(game.White)
	want := "Oavgjort!"
	switch a.gs.Winner() {
	case game.Black:
		want = "Svart vann!"
	case game.White:
		want = "Vit vann!"
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("end banner %q (B%d/W%d) not shown; visible: %v", want, bl, wh, texts(h))
	}
}

// --- Quit mid-game (Back key AND the Meny button), then restart ------------

func TestPlayAtaxxQuit(t *testing.T) {
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

// --- "Ny" restarts the current configuration mid-game -----------------------

func TestPlayAtaxxNyRestarts(t *testing.T) {
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
	if a.gs.Turn != game.Black || a.gs.Board.Count(game.Black) != 2 || a.gs.Board.Count(game.White) != 2 {
		t.Fatal("Ny should reset to a fresh starting position")
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlayAtaxxRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("i alla 8 riktningar"); !ok {
		t.Fatalf("rules text missing the flip-all-8-neighbors rule; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayAtaxxScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/ataxx_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/ataxx_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/ataxx_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)
	// Play a few moves to get a representative mid-game board, with a
	// selection active so clone/jump hints are visible.
	for i := 0; i < 2 && a.gs.Phase == game.PhasePlaying; i++ {
		m, ok := bestByFlips(&a.gs.Board, a.gs.Turn, true)
		if !ok {
			break
		}
		if a.gs.AITurn() {
			break
		}
		tapMove(h, a, m)
	}
	if a.gs.Phase == game.PhasePlaying {
		if legal := a.gs.Board.LegalMoves(a.gs.Turn); len(legal) > 0 {
			tapCell(h, a, legal[0].From) // select a man to show clone/jump hints
		}
	}
	if err := h.Screenshot(dir + "/ataxx_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// End-game banner (board full, Black ahead).
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			setCell(&a.gs.Board, x, y, game.Black)
		}
	}
	setCell(&a.gs.Board, 6, 6, game.White)
	a.gs.Phase = game.PhaseDone
	h.Draw()
	if err := h.Screenshot(dir + "/ataxx_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
