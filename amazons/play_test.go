//go:build playtest

package main

// Headless PLAYTHROUGH tests for Amazons. They drive the real touch path and
// check gameplay against the rules as written (see rulesParagraphs in ui.go
// and SPEC_STRATEGY_CANDIDATES.md's "GAME 12 — Amazons" section): Black
// starts; a turn is two phases — move a queen like a chess queen (any
// distance, straight or diagonal, no jumping), then shoot an arrow (same
// kind of ray) FROM THE QUEEN'S NEW SQUARE onto another empty square, which
// becomes permanently burned (blocks both movement and further shots
// forever, for both sides); there are no captures; the side to move loses
// the instant it has no queen with any legal move at all. Covers both
// opponent modes, illegal-move/-shot rejection, a full game played to a
// natural "no legal move" ending, quitting, restarting, and the rules
// screen. Runs under the pure-Go inkview emulator (playtest/play.sh).
//
// A natural 10x10 Amazons game can run many dozens of moves before either
// side runs out of legal turns — impractical to drive tap-by-tap in a fast
// test. Per the spec's own carve-out for this game ("if that makes the play
// test impractically slow/large, it's fine to engineer a smaller custom
// board position... to reach a natural 'no legal move' ending quickly"),
// the full-game tests below start from a hand-built position that confines
// both queens to a small pocket of the 10x10 board (every other square
// pre-burned), so the game exhausts its few empty squares and reaches a
// genuine, rules-legal "no legal move" ending within a handful of turns.

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"amazons/game"
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

// tapCell taps the board cell at p through the real layout, exactly like a
// finger would: during StepMove this selects a queen or moves the selected
// one; during StepShoot it shoots directly.
func tapCell(h *ink.Harness, a *app, p image.Point) bool {
	return h.TapRect(a.layout.CellToScreen(p.X, p.Y))
}

// tapTurn drives one complete Amazons turn through the real UI: tap the
// queen (select), tap its destination (move), then tap the arrow's
// destination (shoot) — three taps for one full (move, then shoot) turn.
func tapTurn(h *ink.Harness, a *app, tn game.Turn) bool {
	if !tapCell(h, a, tn.Move.From) {
		return false
	}
	if !tapCell(h, a, tn.Move.To) {
		return false
	}
	return tapCell(h, a, tn.Shot)
}

// setCell places a cell directly on the board, bypassing move legality —
// used to construct specific test positions. Board is a plain
// [Size][Size]Cell array (indexed [y][x], matching the game package's own
// convention), so external packages can index it directly without an
// exported setter.
func setCell(b *game.Board, x, y int, c game.Cell) {
	b[y][x] = c
}

// pocketBoard returns a board where only the pocket x pocket square in the
// top-left corner is open (Empty); every other square is pre-Burned. This
// keeps a full game small enough to finish in a handful of turns instead of
// the many dozens a real 10x10 game can take — see the file-level comment.
func pocketBoard(pocket int) game.Board {
	var b game.Board
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			if x >= pocket || y >= pocket {
				b[y][x] = game.Burned
			}
		}
	}
	return b
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- RULE: two-phase turn, move-then-shoot, illegal input rejection --------

func TestPlayAmazonsMoveThenShootRules(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	if len(a.gs.Board.LegalQueenMoves(game.Black)) == 0 {
		t.Fatal("Black should have legal queen moves at the start")
	}

	// Tapping a different own queen after selecting one must switch the
	// selection, not attempt an (illegal) move onto an occupied square.
	before := a.gs.Board
	tapCell(h, a, image.Pt(3, 0)) // select Black queen at (3,0)
	tapCell(h, a, image.Pt(6, 0)) // another Black queen: switches selection
	if a.gs.Board != before {
		t.Fatal("tapping another own queen must not move or duplicate anything")
	}
	if a.gs.Step != game.StepMove {
		t.Fatal("switching the selection must not advance the turn")
	}

	// Tapping the selected queen again must deselect it.
	tapCell(h, a, image.Pt(6, 0))
	if a.hasSelection {
		t.Fatal("tapping the already-selected queen again should deselect it")
	}

	// A non-queen-line destination must be rejected outright. (5,3) is
	// neither on the same rank/file as (3,0) (dx=2) nor a diagonal (dy=3
	// != dx=2), so it is not reachable by any queen move.
	if a.gs.Board.IsLegalQueenMove(game.Black, game.QueenMove{From: image.Pt(3, 0), To: image.Pt(5, 3)}) {
		t.Fatal("sanity: (5,3) should not be a legal queen-line destination from (3,0)")
	}
	tapCell(h, a, image.Pt(3, 0)) // reselect
	if tapCell(h, a, image.Pt(5, 3)) {
		t.Fatal("an illegal (non-queen-line) destination must be rejected")
	}
	if a.gs.Step != game.StepMove || !a.hasSelection {
		t.Fatal("a rejected destination tap must not change Step or drop the selection")
	}

	// The legal move (3,0) -> (3,5), matching TestTwoPhaseTurnOrder's setup.
	if !tapCell(h, a, image.Pt(3, 5)) {
		t.Fatal("the legal move should be accepted")
	}
	if a.gs.Step != game.StepShoot {
		t.Fatal("after a legal move, Step must advance to StepShoot")
	}
	if a.gs.Pending != (image.Point{X: 3, Y: 5}) {
		t.Fatalf("Pending = %v, want (3,5)", a.gs.Pending)
	}
	if a.gs.Turn != game.Black {
		t.Fatal("Turn must not change until the shoot half also completes")
	}

	// A shot destination reachable only from the OLD square, not the new
	// one, must be rejected — the central two-phase gotcha.
	if a.gs.Board.IsLegalShot(image.Pt(3, 5), image.Pt(0, 0)) {
		t.Fatal("sanity: (0,0) should not be reachable from the queen's new square (3,5)")
	}
	if tapCell(h, a, image.Pt(0, 0)) {
		t.Fatal("a shot destination only reachable from the OLD square must be rejected")
	}
	if a.gs.Step != game.StepShoot {
		t.Fatal("a rejected shot must not advance the turn")
	}

	// The legal shot (3,5) -> (3,8) completes the turn.
	if !tapCell(h, a, image.Pt(3, 8)) {
		t.Fatal("the legal shot should be accepted")
	}
	if a.gs.Step != game.StepMove {
		t.Fatal("after a completed turn, Step should reset to StepMove")
	}
	if a.gs.Turn != game.White {
		t.Fatal("turn should pass to White after Black's completed turn")
	}
	if a.gs.Board.At(3, 8) != game.Burned {
		t.Fatal("the shot square should now be permanently Burned")
	}
	if a.gs.Board.At(3, 5) != game.QueenBlack {
		t.Fatal("Black's queen should remain at its new square")
	}
	if a.gs.Board.At(3, 0) != game.Empty {
		t.Fatal("the vacated origin square should remain Empty")
	}
}

// --- GOTCHA: a burned square blocks both movement and shots ----------------

func TestPlayAmazonsBurnedBlocksMovementAndShots(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	var b game.Board
	setCell(&b, 5, 5, game.QueenBlack)
	setCell(&b, 5, 6, game.Burned) // directly south: blocks the rook ray
	setCell(&b, 6, 6, game.Burned) // directly south-east: blocks the diagonal
	setCell(&b, 0, 0, game.QueenWhite)
	a.gs.Board = b
	a.gs.Turn = game.Black
	a.gs.Step = game.StepMove
	h.Draw()

	tapCell(h, a, image.Pt(5, 5)) // select
	if tapCell(h, a, image.Pt(5, 7)) {
		t.Fatal("a queen move must not pass through a burned square")
	}
	if tapCell(h, a, image.Pt(7, 7)) {
		t.Fatal("a queen move must not pass through a burned square diagonally")
	}
	// A legal move onto the burned square's own line up to (but not
	// touching) the block, e.g. straight up, must still work.
	if !tapCell(h, a, image.Pt(5, 0)) {
		t.Fatal("an unobstructed direction should still be legal")
	}
	if a.gs.Step != game.StepShoot || a.gs.Pending != (image.Point{X: 5, Y: 0}) {
		t.Fatal("the legal move should have advanced to the shoot half")
	}

	// From the new square, a shot back down the same column must stop
	// before the (still-burned, now-vacated-queen-side) blocked squares:
	// shooting AT (5,6), a burned square, must be rejected outright.
	if tapCell(h, a, image.Pt(5, 6)) {
		t.Fatal("shooting onto an already-burned square must be rejected")
	}
	if a.gs.Step != game.StepShoot {
		t.Fatal("a rejected shot must not advance the turn")
	}
	if !tapCell(h, a, image.Pt(5, 4)) {
		t.Fatal("a legal shot short of the burned square should be accepted")
	}
	if a.gs.Board.At(5, 6) != game.Burned || a.gs.Board.At(6, 6) != game.Burned {
		t.Fatal("pre-existing burned squares must remain burned")
	}
}

// --- WIN: a full game played to a natural "no legal move" ending -----------
//
// Both queens are confined to a 4x4 pocket (16 squares) in the corner of an
// otherwise fully burned board. Every turn burns exactly one more square, so
// the game is guaranteed to end within a handful of turns as the pocket
// fills in — a stand-in for a full 10x10 game, which the spec explicitly
// allows for this reason (see the file-level comment).

func TestPlayAmazonsFullGameHotseatToNaturalEnd(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)

	b := pocketBoard(4)
	setCell(&b, 0, 0, game.QueenBlack)
	setCell(&b, 3, 3, game.QueenWhite)
	a.gs.Board = b
	a.gs.Turn = game.Black
	a.gs.Step = game.StepMove
	a.gs.Phase = game.PhasePlaying
	h.Draw()

	burnedBefore := a.gs.Board.CountBurned()
	ply := 0
	for ; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 40 {
			t.Fatal("pocket game did not terminate — expected it to exhaust its 16 squares in well under 40 turns")
		}
		turns := a.gs.Board.LegalTurns(a.gs.Turn)
		if len(turns) == 0 {
			t.Fatalf("%v to move but LegalTurns is empty at ply %d, though the game isn't over yet (Winner should have ended it)", a.gs.Turn, ply)
		}
		tn := turns[0]
		burnedWas := a.gs.Board.CountBurned()
		if !tapTurn(h, a, tn) {
			t.Fatalf("legal turn %+v at ply %d was rejected", tn, ply)
		}
		if a.gs.Board.CountBurned() != burnedWas+1 {
			t.Fatalf("ply %d: exactly one more square should be burned after a completed turn", ply)
		}
	}
	if ply == 0 {
		t.Fatal("the loop should have played at least one turn")
	}
	if a.gs.Board.CountBurned() <= burnedBefore {
		t.Fatal("the pocket should have accumulated burned squares over the game")
	}

	winner := a.gs.Winner()
	if winner != game.QueenBlack && winner != game.QueenWhite {
		t.Fatalf("Winner() = %v, want a definite side once Phase is Done", winner)
	}
	want := "Svart vinner!"
	if winner == game.QueenWhite {
		want = "Vit vinner!"
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("win banner %q not shown; visible: %v", want, texts(h))
	}

	// No-captures invariant: exactly the 2 original queens are still on the
	// board somewhere (Amazons never removes a piece), win or lose.
	if n := len(a.gs.Board.QueenPositions(game.Black)); n != 1 {
		t.Fatalf("Black should still have exactly 1 queen (no captures), got %d", n)
	}
	if n := len(a.gs.Board.QueenPositions(game.White)); n != 1 {
		t.Fatalf("White should still have exactly 1 queen (no captures), got %d", n)
	}
}

// --- AI: replies, and a full game against it also reaches a natural end ----

func TestPlayAmazonsAIReplies(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI)

	legal := a.gs.Board.LegalTurns(game.Black)
	if len(legal) == 0 {
		t.Fatal("Black should have legal turns at the start")
	}
	before := a.gs.Board
	if !tapTurn(h, a, legal[0]) {
		t.Fatal("Black's opening turn should be legal")
	}
	if a.gs.AITurn() {
		t.Fatal("control returned on the AI's turn (deferred move+shoot reply not drained)")
	}
	if a.gs.Board == before {
		t.Fatal("White (the AI) did not reply")
	}
	if a.gs.Turn != game.Black {
		t.Fatal("after the AI's full reply turn, control should be back with Black")
	}
}

func TestPlayAmazonsFullGameVsAIToNaturalEnd(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI)

	b := pocketBoard(4)
	setCell(&b, 0, 0, game.QueenBlack) // human
	setCell(&b, 3, 3, game.QueenWhite) // AI
	a.gs.Board = b
	a.gs.Turn = game.Black
	a.gs.Step = game.StepMove
	a.gs.Phase = game.PhasePlaying
	h.Draw()

	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 40 {
			t.Fatal("pocket game vs AI did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		turns := a.gs.Board.LegalTurns(a.gs.Turn)
		if len(turns) == 0 {
			t.Fatalf("human to move but no legal turn at ply %d", ply)
		}
		if !tapTurn(h, a, turns[0]) {
			t.Fatalf("legal turn %+v at ply %d was rejected", turns[0], ply)
		}
	}
	winner := a.gs.Winner()
	want := "Svart vinner!"
	if winner == game.QueenWhite {
		want = "Vit vinner!"
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("win banner %q not shown; visible: %v", want, texts(h))
	}
}

// --- Quit mid-game (Back key AND the Meny button), then restart ------------

func TestPlayAmazonsQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	legal := a.gs.Board.LegalQueenMoves(game.Black)[0]
	tapCell(h, a, legal.From) // select
	tapCell(h, a, legal.To)   // a move in progress (mid-turn: awaiting the shot)

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

func TestPlayAmazonsNyRestarts(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat)
	legal := a.gs.Board.LegalTurns(game.Black)[0]
	if !tapTurn(h, a, legal) {
		t.Fatal("setup: expected the opening turn to be playable")
	}
	if a.gs.Turn != game.White {
		t.Fatal("setup: expected a completed turn to hand control to White")
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
	if a.gs.Turn != game.Black || a.gs.Step != game.StepMove {
		t.Fatal("Ny should reset to a fresh starting position (Black to move)")
	}
	if a.gs.Board != game.NewBoard() {
		t.Fatal("Ny should reset the board to the standard starting position")
	}
}

// --- Rules screen: must honestly label the AI as weak/experimental ---------

func TestPlayAmazonsRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("SVAG"); !ok {
		t.Fatalf("rules text should honestly label the AI as weak; visible: %v", texts(h))
	}
	if _, ok := h.FindTextContains("permanent"); !ok {
		// Swedish wording check is loose on purpose; just confirm burned-
		// square permanence is explained somewhere in the rules text.
		if _, ok := h.FindTextContains("bränd"); !ok {
			t.Fatalf("rules text should explain burned squares; visible: %v", texts(h))
		}
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayAmazonsScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/amazons_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/amazons_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/amazons_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startOpponent(t, h, a, game.OpponentAI)
	legal := a.gs.Board.LegalTurns(game.Black)
	if len(legal) > 0 {
		tapTurn(h, a, legal[0])
	}
	if err := h.Screenshot(dir + "/amazons_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// End-game banner: White (to move) is fully boxed in by burned squares,
	// so Black is the winner — matches TestCompleteTurnEndsGameWhenOpponentBoxedIn's
	// setup, just with Turn/Phase already advanced to reflect that ending.
	h.Back()
	startOpponent(t, h, a, game.OpponentHotseat)
	b := pocketBoard(4)
	setCell(&b, 0, 0, game.QueenBlack)
	setCell(&b, 3, 3, game.QueenWhite)
	setCell(&b, 2, 3, game.Burned)
	setCell(&b, 3, 2, game.Burned)
	setCell(&b, 2, 2, game.Burned)
	a.gs.Board = b
	a.gs.Turn = game.White
	a.gs.Phase = game.PhaseDone
	h.Draw()
	if err := h.Screenshot(dir + "/amazons_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
