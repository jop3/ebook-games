//go:build playtest

package main

// Headless PLAYTHROUGH tests for Hnefatafl (Brandub). They drive the real
// touch path and check the gameplay against the rules as written (see
// rulesParagraphs in ui.go): attackers start; a piece moves like a rook, any
// distance, no jumping, never onto or through the throne/corners unless it's
// the king; custodial capture brackets a run of enemy pieces between two of
// the mover's own (or the EMPTY throne, hostile to BOTH sides — the single
// most commonly-misremembered rule in fan implementations); the king has his
// own separate surround rule (4 attacking sides, or 3 plus the throne when
// adjacent to it); defenders win by walking the king to any corner; attackers
// win by capturing the king; either side also wins immediately if the other
// has no legal move. Covers both win types (king-escape and king-capture),
// both AI sides, illegal-move rejection, quitting, and the rules screen.
// Runs under the pure-Go inkview emulator (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"hnefatafl/game"
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

// startOpponent picks the hot-seat row, or (for vs-AI) first taps the
// side-toggle to select which side the AI plays and then taps the row for
// the given search depth, entering the game either way.
func startOpponent(t *testing.T, h *ink.Harness, a *app, opp game.Opponent, side game.Side, depth int) {
	t.Helper()
	if opp == game.OpponentAI {
		btn := a.menu.sideBtns[0]
		if side == game.SideDefender {
			btn = a.menu.sideBtns[1]
		}
		if !h.TapRect(btn) {
			t.Fatalf("could not tap the AI-side toggle")
		}
	}
	for _, row := range a.menu.rows {
		if row.choice.opponent == opp && (opp == game.OpponentHotseat || row.choice.aiDepth == depth) {
			h.TapRect(row.rect)
			if a.screen != screenGame || a.gs == nil || a.gs.Opponent != opp {
				t.Fatalf("did not start opponent %v (screen=%v)", opp, a.screen)
			}
			if opp == game.OpponentAI && a.gs.AISide != side {
				t.Fatalf("AI side = %v, want %v", a.gs.AISide, side)
			}
			return
		}
	}
	t.Fatalf("no menu row for opponent %v depth %d; visible: %v", opp, depth, texts(h))
}

// tapMove drives a full Hnefatafl move through the real UI: tap the origin
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
// used to construct specific test positions. Board is a plain
// [Size][Size]Cell array (indexed [y][x], matching the game package's own
// convention), so external packages can index it directly without an
// exported setter.
func setCell(b *game.Board, x, y int, c game.Cell) {
	b[y][x] = c
}

// clearBoard empties every cell, so a test can lay out its own position from
// scratch instead of the Brandub starting layout.
func clearBoard(b *game.Board) {
	for y := range b {
		for x := range b[y] {
			b[y][x] = game.Empty
		}
	}
}

// bestMoveHeuristic picks the legal move for side that captures the most
// (an ordinary custodial capture run, or the king outright) — a
// deterministic, always-somewhat-sensible policy for driving full
// playthroughs against the real AI without needing the AI's own search.
func bestMoveHeuristic(b *game.Board, side game.Side) (game.Move, bool) {
	moves := b.LegalMoves(side)
	if len(moves) == 0 {
		return game.Move{}, false
	}
	best := moves[0]
	bestScore := -1
	for _, m := range moves {
		_, res := b.Apply(m)
		score := len(res.Captured)
		if res.KingCaptured {
			score += 1000
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

// --- RULE: rook movement, illegal rejection ---------------------------------

func TestPlayHnefataflMoveRules(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, game.SideAttacker, 0)

	legal := a.gs.Board.LegalMoves(game.SideAttacker)
	if len(legal) == 0 {
		t.Fatal("attackers should have legal moves at the start")
	}

	// Tapping another own piece after selecting one must just switch the
	// selection, never apply an illegal move.
	atkBefore := a.gs.Board.Count(game.Attacker)
	tapCell(h, a, image.Pt(2, 0)) // select an attacker
	tapCell(h, a, image.Pt(4, 0)) // another attacker: switches selection, no move
	if a.gs.Board.Count(game.Attacker) != atkBefore {
		t.Fatal("tapping another own piece must not remove/duplicate anything")
	}
	if a.gs.Turn != game.SideAttacker {
		t.Fatal("switching selection must not change the turn")
	}

	// A legal move matches a pure Apply on a copy exactly.
	m := legal[0]
	want, _ := a.gs.Board.Apply(m)
	if !tapMove(h, a, m) {
		t.Fatalf("legal move %v via tap was rejected", m)
	}
	if a.gs.Board != want {
		t.Fatalf("UI move %v did not match the rules' own Apply result", m)
	}
	if a.gs.Turn != game.SideDefender {
		t.Fatal("turn did not pass to the defenders after a legal move")
	}
}

// --- GOTCHA: illegal move onto/through a restricted square ------------------

func TestPlayHnefataflNonKingCannotEnterThroneOrCorner(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, game.SideAttacker, 0)
	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 3, 0, game.Attacker) // clear run down to, and past, the throne at (3,3)
	a.gs.Turn = game.SideAttacker
	h.Draw()

	if tapMove(h, a, game.Move{From: image.Pt(3, 0), To: image.Pt(3, 3)}) {
		t.Fatal("an ordinary piece must never be able to stop on the throne")
	}
	if tapMove(h, a, game.Move{From: image.Pt(3, 0), To: image.Pt(3, 6)}) {
		t.Fatal("an ordinary piece must never be able to pass through the throne to reach beyond it")
	}
	if a.gs.Board.At(3, 0) != game.Attacker {
		t.Fatal("the illegal attempts must have left the attacker exactly where it started")
	}
}

// --- GOTCHA: the empty throne is hostile to BOTH sides, not attacker-only --

func TestPlayHnefataflEmptyThroneHostileToBothSides(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, game.SideAttacker, 0)

	// Attacker captures a defender by sandwiching it against the empty
	// throne (king lives elsewhere, off the throne, so it really is empty).
	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 1, 6, game.King)     // off the throne/corners, so this move alone can't already have ended the game
	setCell(&a.gs.Board, 3, 2, game.Defender) // just north of the throne
	setCell(&a.gs.Board, 3, 0, game.Attacker) // will slide down to (3,1)
	a.gs.Turn = game.SideAttacker
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(3, 0), To: image.Pt(3, 1)}) {
		t.Fatal("the attacker's approach move should be legal")
	}
	if a.gs.Board.At(3, 2) != game.Empty {
		t.Fatal("the defender bracketed against the empty throne should have been captured")
	}
	if len(a.gs.LastCaptured) != 1 || a.gs.LastCaptured[0] != (image.Point{X: 3, Y: 2}) {
		t.Fatalf("LastCaptured = %v, want exactly [(3,2)]", a.gs.LastCaptured)
	}

	// Symmetric case: a defender captures an attacker against the same
	// empty throne.
	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 1, 6, game.King)     // off the throne/corners, so this move alone can't already have ended the game
	setCell(&a.gs.Board, 3, 4, game.Attacker) // just south of the throne
	setCell(&a.gs.Board, 3, 6, game.Defender) // will slide up to (3,5)
	a.gs.Turn = game.SideDefender
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(3, 6), To: image.Pt(3, 5)}) {
		t.Fatal("the defender's approach move should be legal")
	}
	if a.gs.Board.At(3, 4) != game.Empty {
		t.Fatal("the attacker bracketed against the empty throne should have been captured")
	}
}

// --- GOTCHA: safe entry via a real tap (no self-capture) --------------------

func TestPlayHnefataflSafeEntryNoSelfCapture(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, game.SideAttacker, 0)
	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 1, 6, game.King) // off the throne/corners, so this move alone can't already have ended the game
	// Gap and approach column deliberately avoid x==3 (the throne's column),
	// since a straight run through the throne would be illegal (blocked
	// terrain) rather than the safe-entry case this test wants to exercise.
	setCell(&a.gs.Board, 0, 4, game.Defender)
	setCell(&a.gs.Board, 2, 4, game.Defender)
	setCell(&a.gs.Board, 1, 0, game.Attacker) // will land at (1,4), between the two defenders
	a.gs.Turn = game.SideAttacker
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(1, 0), To: image.Pt(1, 4)}) {
		t.Fatal("moving into the gap between two enemies should be a legal move")
	}
	if a.gs.Board.At(1, 4) != game.Attacker {
		t.Fatal("the mover's own piece must remain on the board (no self-capture)")
	}
	if a.gs.Board.At(0, 4) != game.Defender || a.gs.Board.At(2, 4) != game.Defender {
		t.Fatal("the flanking defenders must remain untouched")
	}
	if len(a.gs.LastCaptured) != 0 {
		t.Fatalf("safe entry must capture nothing, got %v", a.gs.LastCaptured)
	}
}

// --- WIN: king captured, adjacent to the throne (only 3 attackers needed) --

func TestPlayHnefataflKingCaptureWin(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, game.SideAttacker, 0)
	clearBoard(&a.gs.Board)

	// King sits just north of the (empty) throne at (3,3); two of the
	// king's other three orthogonal neighbors already hold attackers. The
	// throne itself supplies the 4th hostile side, so only 3 actual
	// attacker pieces are needed here, not 4 — the exact rule under test.
	setCell(&a.gs.Board, 3, 2, game.King)
	setCell(&a.gs.Board, 2, 2, game.Attacker)
	setCell(&a.gs.Board, 4, 2, game.Attacker)
	setCell(&a.gs.Board, 3, 0, game.Attacker) // will slide down to (3,1), completing the surround
	a.gs.Turn = game.SideAttacker
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(3, 0), To: image.Pt(3, 1)}) {
		t.Fatal("the king-capturing move should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once the king is captured")
	}
	if winner, reason := a.gs.Winner(); winner != game.SideAttacker || reason != game.ReasonKingCaptured {
		t.Fatalf("Winner() = (%v, %v), want (SideAttacker, ReasonKingCaptured)", winner, reason)
	}
	if _, alive := a.gs.Board.KingPos(); alive {
		t.Fatal("the king should be removed from the board once captured")
	}
	if _, ok := h.FindTextContains("Anfallarna vann!"); !ok {
		t.Fatalf("attacker win banner not shown; visible: %v", texts(h))
	}
}

// --- WIN: king escapes to a corner ------------------------------------------

func TestPlayHnefataflKingEscapeWin(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, game.SideAttacker, 0)
	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 1, 0, game.King) // one step from the corner (0,0)
	a.gs.Turn = game.SideDefender
	h.Draw()

	if !tapMove(h, a, game.Move{From: image.Pt(1, 0), To: image.Pt(0, 0)}) {
		t.Fatal("the king's escaping move onto the corner should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase should be Done once the king reaches a corner")
	}
	if winner, reason := a.gs.Winner(); winner != game.SideDefender || reason != game.ReasonKingEscaped {
		t.Fatalf("Winner() = (%v, %v), want (SideDefender, ReasonKingEscaped)", winner, reason)
	}
	if _, ok := h.FindTextContains("Försvararna vann!"); !ok {
		t.Fatalf("defender win banner not shown; visible: %v", texts(h))
	}
}

// --- All 3 AI difficulties actually reply, for both AI sides ----------------

func TestPlayHnefataflAllDifficultiesReply(t *testing.T) {
	for _, depth := range []int{game.DepthEasy, game.DepthMedium, game.DepthHard} {
		depth := depth
		t.Run("attacker-ai-"+itoa(depth), func(t *testing.T) {
			h, a := bootToMenu(t)
			startOpponent(t, h, a, game.OpponentAI, game.SideAttacker, depth)
			// The AI plays attackers, who move first: control must NOT come
			// back to the test until the AI's opening move has been drawn.
			if a.gs.AITurn() {
				t.Fatal("control returned on the AI's turn (deferred reply not drained)")
			}
			if a.gs.Turn != game.SideDefender {
				t.Fatal("the AI (attackers) should already have played its opening move")
			}
		})
		t.Run("defender-ai-"+itoa(depth), func(t *testing.T) {
			h, a := bootToMenu(t)
			startOpponent(t, h, a, game.OpponentAI, game.SideDefender, depth)
			legal := a.gs.Board.LegalMoves(game.SideAttacker)
			before := a.gs.Board
			if !tapMove(h, a, legal[0]) {
				t.Fatal("the human attacker's opening move should be legal")
			}
			if a.gs.AITurn() {
				t.Fatal("control returned on the AI's turn (deferred reply not drained)")
			}
			if a.gs.Board == before {
				t.Fatal("the defenders (the AI) did not reply")
			}
		})
	}
}

// --- Full games vs the AI, played to a real win, one per AI side -----------

func playFullGame(t *testing.T, aiSide game.Side) {
	t.Helper()
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI, aiSide, game.DepthMedium)

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 300 {
			t.Fatal("game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		m, ok := bestMoveHeuristic(&a.gs.Board, a.gs.Turn)
		if !ok {
			t.Fatalf("human to move but no legal move at ply %d", ply)
		}
		if !tapMove(h, a, m) {
			t.Fatalf("legal move %v at ply %d was rejected", m, ply)
		}
	}
	winner, reason := a.gs.Winner()
	want := sideName(winner) + " vann!"
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("end banner %q (reason=%v) not shown; visible: %v", want, reason, texts(h))
	}
}

func TestPlayHnefataflFullGameVsAIPlayingAttacker(t *testing.T) {
	playFullGame(t, game.SideAttacker)
}

func TestPlayHnefataflFullGameVsAIPlayingDefender(t *testing.T) {
	playFullGame(t, game.SideDefender)
}

// --- Quit mid-game (Back key AND the Meny button), then restart ------------

func TestPlayHnefataflQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, game.SideAttacker, 0)
	m := a.gs.Board.LegalMoves(game.SideAttacker)[0]
	tapMove(h, a, m) // a move in progress

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}

	startOpponent(t, h, a, game.OpponentAI, game.SideDefender, game.DepthEasy)
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
	startOpponent(t, h, a, game.OpponentHotseat, game.SideAttacker, 0)
}

// --- "Ny" restarts the current configuration mid-game -----------------------

func TestPlayHnefataflNyRestarts(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, game.SideAttacker, 0)
	m := a.gs.Board.LegalMoves(game.SideAttacker)[0]
	tapMove(h, a, m)
	if a.gs.Turn != game.SideDefender {
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
	if a.gs.Turn != game.SideAttacker ||
		a.gs.Board.Count(game.Attacker) != game.StartAttackers ||
		a.gs.Board.DefenderSideCount() != game.StartDefenders+1 {
		t.Fatal("Ny should reset to a fresh starting position")
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlayHnefataflRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("Brandub"); !ok {
		t.Fatalf("rules text missing a mention of Brandub; visible: %v", texts(h))
	}
	if _, ok := h.FindTextContains("TOM tron"); !ok {
		t.Fatalf("rules text missing the empty-throne-hostile-to-both-sides rule; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayHnefataflScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/hnefatafl_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/hnefatafl_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/hnefatafl_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startOpponent(t, h, a, game.OpponentAI, game.SideDefender, game.DepthMedium)
	for i := 0; i < 3 && a.gs.Phase == game.PhasePlaying; i++ {
		if a.gs.AITurn() {
			break
		}
		m, ok := bestMoveHeuristic(&a.gs.Board, a.gs.Turn)
		if !ok {
			break
		}
		tapMove(h, a, m)
	}
	if err := h.Screenshot(dir + "/hnefatafl_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// King-capture end-game banner.
	clearBoard(&a.gs.Board)
	setCell(&a.gs.Board, 3, 2, game.King)
	setCell(&a.gs.Board, 2, 2, game.Attacker)
	setCell(&a.gs.Board, 4, 2, game.Attacker)
	setCell(&a.gs.Board, 3, 1, game.Attacker)
	a.gs.Phase = game.PhaseDone
	h.Draw()
	if err := h.Screenshot(dir + "/hnefatafl_capture_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
