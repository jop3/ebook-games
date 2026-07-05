//go:build playtest

package main

// Headless PLAYTHROUGH tests for Shong. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): a 4x6 board, Triangel/Kvadrat/X/Kung movement, the short-to-long
// (1-square then always-exactly-2-square) transition, no jumping (including
// the long move's clear-middle-square requirement), displacement capture,
// the King's alternating diagonal/orthogonal move-set (which flips only when
// the King itself moves), and both win conditions (King capture, King
// reaches the far edge). Covers both opponent modes (all 3 AI difficulties),
// illegal-move rejection, quitting, and the rules screen. Runs under the
// pure-Go inkview emulator (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"shong/game"
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

// tapMove drives a full Shong move through the real UI: tap the origin
// (selecting the piece), then tap the destination.
func tapMove(h *ink.Harness, a *app, m game.Move) bool {
	if !h.TapRect(a.layout.CellToScreen(m.From.X, m.From.Y)) {
		return false
	}
	return h.TapRect(a.layout.CellToScreen(m.To.X, m.To.Y))
}

func tapCell(h *ink.Harness, a *app, p image.Point) bool {
	return h.TapRect(a.layout.CellToScreen(p.X, p.Y))
}

// setCell places a piece directly on the board, bypassing move legality —
// used to construct specific test positions.
func setCell(b *game.Board, x, y int, p *game.Piece) {
	b[y][x] = p
}

func clearBoard(b *game.Board) {
	for y := range b {
		for x := range b[y] {
			b[y][x] = nil
		}
	}
}

func pieceCount(b *game.Board) int {
	return b.Count(game.Black) + b.Count(game.White)
}

// boardsEqual compares two boards cell by cell by VALUE (Kind/Side/Moved/
// Ortho). Board holds *Piece, so Go's built-in == on two Boards compares
// pointer identity — Apply always allocates a fresh *Piece for the mover, so
// two independently-Apply'd boards are never == even when every piece on
// them is identical; this is the value-equality a test actually wants.
func boardsEqual(a, b *game.Board) bool {
	for y := 0; y < game.Rows; y++ {
		for x := 0; x < game.Cols; x++ {
			pa, pb := a.At(x, y), b.At(x, y)
			if (pa == nil) != (pb == nil) {
				return false
			}
			if pa != nil && *pa != *pb {
				return false
			}
		}
	}
	return true
}

// bestByCaptures is a deterministic (non-AI) move policy used to drive full
// playthroughs: prefer capturing the enemy King outright, then any capture
// (bigger target first), then simply the first legal move.
func bestByCaptures(b *game.Board, side game.Side) (game.Move, bool) {
	moves := b.LegalMoves(side)
	if len(moves) == 0 {
		return game.Move{}, false
	}
	best := moves[0]
	bestScore := -1
	for _, m := range moves {
		score := 0
		if target := b.At(m.To.X, m.To.Y); target != nil {
			if target.Kind == game.King {
				score = 1000
			} else {
				score = 10
			}
		}
		if score > bestScore {
			bestScore, best = score, m
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

// --- RULE: basic move + selection-switch + illegal-destination rejection ---

func TestPlayShongMoveRules(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	legal := a.gs.Board.LegalMoves(game.Black)
	if len(legal) == 0 {
		t.Fatal("Black should have legal moves at the start")
	}

	// Selecting a Black piece, then tapping another Black piece's square,
	// must just switch the selection — no move applied, turn unchanged.
	before := a.gs.Board
	tapCell(h, a, image.Pt(0, 0)) // select the X on Black's back rank
	tapCell(h, a, image.Pt(1, 0)) // Kvadrat's own square... actually Triangel
	if a.gs.Board != before {
		t.Fatal("tapping another own piece must not move/capture anything")
	}
	if a.gs.Turn != game.Black {
		t.Fatal("switching the selection must not change the turn")
	}
	if !a.hasSelection || a.selected != (image.Point{X: 1, Y: 0}) {
		t.Fatal("the selection should have switched to the newly tapped own piece")
	}

	// A legal move matches a pure Apply on a copy exactly.
	m := legal[0]
	wantBoard, wantCaptured := a.gs.Board.Apply(m)
	tapCell(h, a, m.From) // (re)select the actual mover
	if !tapCell(h, a, m.To) {
		t.Fatalf("legal move %v via tap was rejected", m)
	}
	if !boardsEqual(&a.gs.Board, &wantBoard) {
		t.Fatalf("UI move %v did not match the rules' own Apply result", m)
	}
	if a.gs.Turn != game.White {
		t.Fatal("turn did not pass to White after a legal move")
	}
	_ = wantCaptured
}

// --- GOTCHA: exact-distance and no-jumping illegal moves are rejected -----

func TestPlayShongIllegalMoveRejections(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(&a.gs.Board)

	// A long (Moved=true) piece must not be able to move only 1 square, even
	// onto an empty square.
	setCell(&a.gs.Board, 2, 2, &game.Piece{Kind: game.Square, Side: game.Black, Moved: true})
	a.gs.Turn = game.Black
	h.Draw()
	before := a.gs.Board
	if tapMove(h, a, game.Move{From: image.Pt(2, 2), To: image.Pt(2, 1)}) {
		t.Fatal("a long piece's distance-1 move must be rejected")
	}
	if a.gs.Board != before {
		t.Fatal("a rejected move must not change the board")
	}

	// A long piece's distance-2 move must be rejected if its middle square is
	// blocked (by either side).
	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 2, 2, &game.Piece{Kind: game.Square, Side: game.Black, Moved: true})
	setCell(&a.gs.Board, 1, 2, &game.Piece{Kind: game.Triangle, Side: game.White}) // middle square of (2,2)->(0,2)
	h.Draw()
	before = a.gs.Board
	if tapMove(h, a, game.Move{From: image.Pt(2, 2), To: image.Pt(0, 2)}) {
		t.Fatal("a long move with a blocked middle square must be rejected")
	}
	if a.gs.Board != before {
		t.Fatal("a rejected move must not change the board")
	}
}

// --- GOTCHA: a piece transitions from short (1) to long (2) after its own
// first move, driven entirely through real tap sequences ------------------

func TestPlayShongShortToLongTransition(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 2, 2, &game.Piece{Kind: game.Triangle, Side: game.Black})
	setCell(&a.gs.Board, 0, 0, &game.Piece{Kind: game.King, Side: game.Black}) // both Kings present so the
	setCell(&a.gs.Board, 0, 5, &game.Piece{Kind: game.King, Side: game.White}) // game doesn't end after the move
	a.gs.Turn = game.Black
	h.Draw()

	// First move: short, distance 1, diagonal.
	if !tapMove(h, a, game.Move{From: image.Pt(2, 2), To: image.Pt(1, 1)}) {
		t.Fatal("the piece's first (short) move should be legal")
	}
	moved := a.gs.Board.At(1, 1)
	if moved == nil || !moved.Moved {
		t.Fatal("the piece must be marked Moved (long) after its first move")
	}

	// It is now White's turn; hand it back to Black directly so this test
	// can focus purely on the one piece's own move sequence.
	a.gs.Turn = game.Black

	// Now-long piece must NOT be able to move just 1 square...
	before := a.gs.Board
	if tapMove(h, a, game.Move{From: image.Pt(1, 1), To: image.Pt(2, 2)}) {
		t.Fatal("the now-long piece must not be able to move only 1 square")
	}
	if a.gs.Board != before {
		t.Fatal("a rejected move must not change the board")
	}

	// ...but should move exactly 2 squares along a clear diagonal. The piece
	// is still selected from the rejected attempt above (an illegal-move tap
	// leaves the selection as-is), so only the destination needs tapping —
	// re-tapping the origin here would instead be read as "deselect".
	if !tapCell(h, a, image.Pt(3, 3)) {
		t.Fatal("the long piece's clear distance-2 move should be legal")
	}
}

// --- GOTCHA: the King's alternating move-set flips only when it itself
// moves, driven entirely through real tap sequences ------------------------

func TestPlayShongKingAlternatingMoveSet(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 1, 2, &game.Piece{Kind: game.King, Side: game.Black}) // Ortho=false: diagonal first
	setCell(&a.gs.Board, 3, 0, &game.Piece{Kind: game.Square, Side: game.White})
	setCell(&a.gs.Board, 0, 5, &game.Piece{Kind: game.King, Side: game.White}) // present so the game doesn't end
	a.gs.Turn = game.Black
	h.Draw()

	king := a.gs.Board.At(1, 2)
	if king.Ortho {
		t.Fatal("setup: King should start in diagonal mode")
	}

	// The King moves diagonally (its current mode): legal, and flips it to
	// orthogonal.
	if !tapMove(h, a, game.Move{From: image.Pt(1, 2), To: image.Pt(2, 3)}) {
		t.Fatal("the King's diagonal move should be legal in diagonal mode")
	}
	king = a.gs.Board.At(2, 3)
	if king == nil || !king.Ortho {
		t.Fatal("the King's move-set must flip to orthogonal after it moves")
	}

	// White moves an unrelated piece; the King's mode must be unaffected.
	if !tapMove(h, a, game.Move{From: image.Pt(3, 0), To: image.Pt(3, 1)}) {
		t.Fatal("White's move should be legal")
	}
	king = a.gs.Board.At(2, 3)
	if king == nil || !king.Ortho {
		t.Fatal("the King's move-set must not change just because a different piece moved")
	}

	// Now in orthogonal mode, a diagonal King move must be rejected...
	a.gs.Turn = game.Black
	before := a.gs.Board
	if tapMove(h, a, game.Move{From: image.Pt(2, 3), To: image.Pt(1, 2)}) {
		t.Fatal("a diagonal King move must be rejected while in orthogonal mode")
	}
	if a.gs.Board != before {
		t.Fatal("a rejected move must not change the board")
	}
	// ...but an orthogonal one should succeed. The King is still selected
	// from the rejected attempt above, so only the destination needs
	// tapping — re-tapping the origin here would instead be read as
	// "deselect".
	if !tapCell(h, a, image.Pt(1, 3)) {
		t.Fatal("an orthogonal King move should be legal in orthogonal mode")
	}
}

// --- WIN: capturing the enemy King -----------------------------------------

func TestPlayShongWinByKingCapture(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 2, 2, &game.Piece{Kind: game.Triangle, Side: game.Black})
	setCell(&a.gs.Board, 1, 1, &game.Piece{Kind: game.King, Side: game.White})
	setCell(&a.gs.Board, 2, 0, &game.Piece{Kind: game.King, Side: game.Black}) // neutral row
	a.gs.Turn = game.Black
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(2, 2), To: image.Pt(1, 1)}) {
		t.Fatal("the King-capturing move should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once White's King is captured")
	}
	if _, ok := h.FindTextContains("Svart vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- WIN: King reaches the far edge -----------------------------------------

func TestPlayShongWinByKingReachingFarEdge(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 1, 4, &game.Piece{Kind: game.King, Side: game.Black}) // diagonal mode, one step from y=5
	setCell(&a.gs.Board, 3, 2, &game.Piece{Kind: game.King, Side: game.White}) // neutral row
	a.gs.Turn = game.Black
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(1, 4), To: image.Pt(2, 5)}) {
		t.Fatal("the goal-rank move should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once Black's King reaches the far edge")
	}
	if _, ok := h.FindTextContains("Svart vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- All 3 AI difficulties actually reply -----------------------------------

func TestPlayShongAllDifficultiesReply(t *testing.T) {
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

// --- Full game vs the AI, played to a real win/loss -------------------------

func TestPlayShongFullGameVsAI(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 300 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		m, ok := bestByCaptures(&a.gs.Board, a.gs.Turn)
		if !ok {
			t.Fatalf("human to move but no legal move at ply %d", ply)
		}
		if !tapMove(h, a, m) {
			t.Fatalf("legal move %v at ply %d was rejected", m, ply)
		}
		if pieceCount(&a.gs.Board) > 2*game.Cols {
			t.Fatalf("board inconsistent mid-game: %d pieces", pieceCount(&a.gs.Board))
		}
	}
	want := "Oavgjort!"
	if w, ok := a.gs.Winner(); ok {
		if w == game.Black {
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

func TestPlayShongQuit(t *testing.T) {
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

func TestPlayShongNyRestarts(t *testing.T) {
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
	if a.gs.Turn != game.Black || a.gs.Board.Count(game.Black) != game.Cols || a.gs.Board.Count(game.White) != game.Cols {
		t.Fatal("Ny should reset to a fresh starting position")
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlayShongRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("Higher Plain Games"); !ok {
		t.Fatalf("rules text missing the original-game credit; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayShongScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/shong_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/shong_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/shong_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)
	// Play a few moves both ways to get a representative mid-game board with
	// a mix of short (fresh) and long (moved) pieces visible.
	for i := 0; i < 3 && a.gs.Phase == game.PhasePlaying; i++ {
		if a.gs.AITurn() {
			break
		}
		m, ok := bestByCaptures(&a.gs.Board, a.gs.Turn)
		if !ok {
			break
		}
		tapMove(h, a, m)
	}
	if err := h.Screenshot(dir + "/shong_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// King-capture end-game banner. The capturing Triangel is unmoved (short,
	// distance 1) so its diagonal capture of the adjacent White King is
	// legal in one tap.
	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 2, 2, &game.Piece{Kind: game.Triangle, Side: game.Black})
	setCell(&a.gs.Board, 1, 1, &game.Piece{Kind: game.King, Side: game.White})
	setCell(&a.gs.Board, 2, 0, &game.Piece{Kind: game.King, Side: game.Black})
	a.gs.Turn = game.Black
	if !tapMove(h, a, game.Move{From: image.Pt(2, 2), To: image.Pt(1, 1)}) {
		t.Fatal("setup: the King-capturing move should be legal")
	}
	if err := h.Screenshot(dir + "/shong_kingcapture_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// Goal-rank end-game banner.
	h.Back()
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 1, 4, &game.Piece{Kind: game.King, Side: game.Black})
	setCell(&a.gs.Board, 3, 2, &game.Piece{Kind: game.King, Side: game.White})
	a.gs.Turn = game.Black
	if !tapMove(h, a, game.Move{From: image.Pt(1, 4), To: image.Pt(2, 5)}) {
		t.Fatal("setup: the goal-rank move should be legal")
	}
	if err := h.Screenshot(dir + "/shong_goaledge_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
