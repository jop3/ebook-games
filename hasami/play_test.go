//go:build playtest

package main

// Headless PLAYTHROUGH tests for Hasami. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): Black starts; a man moves like a rook, any distance, no jumping;
// custodial capture brackets a run of enemy men between two of the mover's
// own, possibly in several directions at once off a single move; a corner
// man is captured by occupying both cells next to that corner; moving into
// the gap between two enemies is never self-capture; Fångst ends the game
// when a side is reduced to one man; Fem i rad ends it on an unbroken line of
// 5 outside the owner's home rank. Covers both opponent modes (all 3 AI
// difficulties), both win conditions, illegal-move rejection, quitting, and
// the rules screen. Runs under the pure-Go inkview emulator (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"hasami/game"
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

// setWinMode taps the given win-mode toggle button on the menu before
// starting a game.
func setWinMode(h *ink.Harness, a *app, mode game.WinMode) {
	label := "Fångst"
	if mode == game.ModeFiveInRow {
		label = "Fem i rad"
	}
	h.TapText(label)
}

// tapMove drives a full Hasami move through the real UI: tap the origin
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

// manTotal returns black+white+empty (must always be 81).
func manTotal(b *game.Board) int {
	return b.Count(game.Black) + b.Count(game.White) + b.Count(game.Empty)
}

// bestByCaptures returns the legal move for color that captures the most
// (max=true) or simply the first available (max=false) — a deterministic
// policy for full-game playthroughs against the AI.
func bestByCaptures(b *game.Board, color game.Cell, max bool) (game.Move, bool) {
	moves := b.LegalMoves(color)
	if len(moves) == 0 {
		return game.Move{}, false
	}
	if !max {
		return moves[0], true
	}
	best := moves[0]
	bestN := -1
	for _, m := range moves {
		nb, captured := b.Apply(m)
		_ = nb
		if len(captured) > bestN {
			bestN, best = len(captured), m
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

// --- RULE: rook movement, illegal rejection ---------------------------------

func TestPlayHasamiMoveRules(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	legal := a.gs.Board.LegalMoves(game.Black)
	if len(legal) == 0 {
		t.Fatal("Black should have legal moves at the start")
	}

	// Tapping an occupied square (another Black man on the home rank) after
	// selecting one must be rejected: no move applied, turn unchanged, and
	// the tap instead just switches the selection.
	blackBefore := a.gs.Board.Count(game.Black)
	tapCell(h, a, image.Pt(0, game.Size-1)) // select
	tapCell(h, a, image.Pt(1, game.Size-1)) // occupied by another Black man
	if a.gs.Board.Count(game.Black) != blackBefore {
		t.Fatal("tapping an occupied square must not remove/duplicate a man")
	}
	if a.gs.Turn != game.Black {
		t.Fatal("an illegal destination must not change the turn")
	}

	// A legal move matches a pure Apply on a copy exactly.
	m := legal[0]
	want, wantCaptured := a.gs.Board.Apply(m)
	if !tapMove(h, a, m) {
		t.Fatalf("legal move %v via tap was rejected", m)
	}
	if a.gs.Board != want {
		t.Fatalf("UI move %v did not match the rules' own Apply result", m)
	}
	if a.gs.Turn != game.White {
		t.Fatal("turn did not pass to White after a legal move")
	}
	if manTotal(&a.gs.Board) != game.Size*game.Size {
		t.Fatalf("board inconsistent: total %d", manTotal(&a.gs.Board))
	}
	_ = wantCaptured
}

// --- GOTCHA: multi-direction capture via a real tap -------------------------

func TestPlayHasamiMultiDirectionCapture(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	for i := range a.gs.Board {
		for j := range a.gs.Board[i] {
			a.gs.Board[i][j] = game.Empty
		}
	}
	setCell(&a.gs.Board, 4, 0, game.Black)
	setCell(&a.gs.Board, 3, 4, game.White)
	setCell(&a.gs.Board, 2, 4, game.Black)
	setCell(&a.gs.Board, 5, 4, game.White)
	setCell(&a.gs.Board, 6, 4, game.Black)
	a.gs.Turn = game.Black
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(4, 0), To: image.Pt(4, 4)}) {
		t.Fatal("the capturing move should be legal")
	}
	if a.gs.Board.At(3, 4) != game.Empty || a.gs.Board.At(5, 4) != game.Empty {
		t.Fatal("both bracketed White men should have been captured by one move")
	}
	if a.gs.Board.At(4, 4) != game.Black {
		t.Fatal("the mover should now sit at (4,4)")
	}
	if len(a.gs.LastCaptured) != 2 {
		t.Fatalf("LastCaptured = %v, want exactly the 2 captured cells", a.gs.LastCaptured)
	}
}

// --- GOTCHA: corner capture via a real tap ----------------------------------

func TestPlayHasamiCornerCapture(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	for i := range a.gs.Board {
		for j := range a.gs.Board[i] {
			a.gs.Board[i][j] = game.Empty
		}
	}
	setCell(&a.gs.Board, 0, 0, game.White) // enemy man in the corner
	setCell(&a.gs.Board, 1, 0, game.Black) // one adjacency already in place
	setCell(&a.gs.Board, 0, 5, game.Black) // mover: will complete the other adjacency
	a.gs.Turn = game.Black
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(0, 5), To: image.Pt(0, 1)}) {
		t.Fatal("the corner-completing move should be legal")
	}
	if a.gs.Board.At(0, 0) != game.Empty {
		t.Fatal("the White man in the corner should have been captured")
	}
	if a.gs.Board.At(1, 0) != game.Black || a.gs.Board.At(0, 1) != game.Black {
		t.Fatal("both Black men adjacent to the corner should remain")
	}
}

// --- GOTCHA: safe entry via a real tap (no self-capture) --------------------

func TestPlayHasamiSafeEntryNoSelfCapture(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	for i := range a.gs.Board {
		for j := range a.gs.Board[i] {
			a.gs.Board[i][j] = game.Empty
		}
	}
	setCell(&a.gs.Board, 2, 4, game.White)
	setCell(&a.gs.Board, 4, 4, game.White)
	setCell(&a.gs.Board, 3, 0, game.Black) // mover: will land at (3,4), between the two White men
	a.gs.Turn = game.Black
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(3, 0), To: image.Pt(3, 4)}) {
		t.Fatal("moving into the gap between two enemies should be a legal move")
	}
	if a.gs.Board.At(3, 4) != game.Black {
		t.Fatal("the mover's own man must remain on the board (no self-capture)")
	}
	if a.gs.Board.At(2, 4) != game.White || a.gs.Board.At(4, 4) != game.White {
		t.Fatal("the flanking White men must remain untouched")
	}
	if len(a.gs.LastCaptured) != 0 {
		t.Fatalf("safe entry must capture nothing, got %v", a.gs.LastCaptured)
	}
}

// --- WIN: Fångst (reduce to 1 man) ------------------------------------------

func TestPlayHasamiFangstWin(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	for i := range a.gs.Board {
		for j := range a.gs.Board[i] {
			a.gs.Board[i][j] = game.Empty
		}
	}
	setCell(&a.gs.Board, 4, 0, game.Black)
	setCell(&a.gs.Board, 3, 4, game.White)
	setCell(&a.gs.Board, 2, 4, game.Black)
	setCell(&a.gs.Board, 4, 8, game.White) // White's sole survivor
	a.gs.Turn = game.Black
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(4, 0), To: image.Pt(4, 4)}) {
		t.Fatal("the winning move should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once White is reduced to 1 man")
	}
	if _, ok := h.FindTextContains("Svart vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- WIN: Fem i rad (line of 5 outside the home rank) -----------------------

func TestPlayHasamiFemIRadWin(t *testing.T) {
	h, a := bootToMenu(t)
	setWinMode(h, a, game.ModeFiveInRow)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	if a.gs.WinMode != game.ModeFiveInRow {
		t.Fatalf("game did not start in Fem i rad mode: %v", a.gs.WinMode)
	}

	for i := range a.gs.Board {
		for j := range a.gs.Board[i] {
			a.gs.Board[i][j] = game.Empty
		}
	}
	setCell(&a.gs.Board, 0, 4, game.Black)
	setCell(&a.gs.Board, 1, 4, game.Black)
	setCell(&a.gs.Board, 2, 4, game.Black)
	setCell(&a.gs.Board, 3, 4, game.Black)
	setCell(&a.gs.Board, 4, 0, game.Black) // completes the line at (4,4)
	a.gs.Turn = game.Black
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(4, 0), To: image.Pt(4, 4)}) {
		t.Fatal("the line-completing move should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once the 5-in-a-row line completes")
	}
	if _, ok := h.FindTextContains("Svart vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- All 3 AI difficulties actually reply -----------------------------------

func TestPlayHasamiAllDifficultiesReply(t *testing.T) {
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

func TestPlayHasamiFullGameVsAI(t *testing.T) {
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

// --- Quit mid-game (Back key AND the Meny button), then restart ------------

func TestPlayHasamiQuit(t *testing.T) {
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

func TestPlayHasamiNyRestarts(t *testing.T) {
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
	if a.gs.Turn != game.Black || a.gs.Board.Count(game.Black) != 9 || a.gs.Board.Count(game.White) != 9 {
		t.Fatal("Ny should reset to a fresh starting position")
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlayHasamiRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("Säkert att gå in"); !ok {
		t.Fatalf("rules text missing the safe-entry rule; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayHasamiScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/hasami_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/hasami_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/hasami_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startOpponent(t, h, a, game.OpponentAI, game.DepthMedium)
	// Play a few moves to get a representative mid-game board.
	for i := 0; i < 3 && a.gs.Phase == game.PhasePlaying; i++ {
		m, ok := bestByCaptures(&a.gs.Board, a.gs.Turn, true)
		if !ok {
			break
		}
		if a.gs.AITurn() {
			break
		}
		tapMove(h, a, m)
	}
	if err := h.Screenshot(dir + "/hasami_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// Fångst end-game banner.
	for i := range a.gs.Board {
		for j := range a.gs.Board[i] {
			a.gs.Board[i][j] = game.Empty
		}
	}
	setCell(&a.gs.Board, 4, 4, game.Black)
	setCell(&a.gs.Board, 2, 2, game.Black)
	setCell(&a.gs.Board, 6, 6, game.Black)
	setCell(&a.gs.Board, 0, 0, game.White) // White's sole survivor -> Black wins
	a.gs.Phase = game.PhaseDone
	h.Draw()
	if err := h.Screenshot(dir + "/hasami_fangst_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// Fem i rad end-game banner.
	h.Back()
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	a.gs.WinMode = game.ModeFiveInRow
	for i := range a.gs.Board {
		for j := range a.gs.Board[i] {
			a.gs.Board[i][j] = game.Empty
		}
	}
	for x := 0; x < 5; x++ {
		setCell(&a.gs.Board, x, 4, game.Black)
	}
	a.gs.Phase = game.PhaseDone
	h.Draw()
	if err := h.Screenshot(dir + "/hasami_femirad_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
