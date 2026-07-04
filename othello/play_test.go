//go:build playtest

package main

// Headless PLAYTHROUGH test: boots the real app, starts a game against the
// built-in AI, and plays it to the end through the real touch path — the human
// taps a legal square, the app's deferred-AI machinery replies on the next
// frame, and so on until the game is over. It asserts the match actually
// terminates, turn alternation and the AI reply both work, and the final board
// is internally consistent. Runs under the pure-Go inkview emulator
// (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"othello/game"
)

func empties(b *game.Board) int { return b.Count(game.Empty) }

func TestPlayOthelloVsAI(t *testing.T) {
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}

	// Splash -> menu.
	h.TapXY(500, 700)
	if a.screen != screenMenu {
		t.Fatalf("splash tap did not open menu, screen=%v", a.screen)
	}

	// Pick the first "vs computer" mode from the menu.
	started := false
	for _, row := range a.menu.rows {
		if row.choice.mode == game.ModeAI {
			h.TapRect(row.rect)
			started = true
			break
		}
	}
	if !started {
		t.Fatalf("no AI mode on menu; visible: %v", texts(h))
	}
	if a.screen != screenGame || a.gs == nil || a.gs.Mode != game.ModeAI {
		t.Fatalf("did not start an AI game (screen=%v)", a.screen)
	}

	// Play to the end. Each iteration: it must be the human's turn (the harness
	// drains the AI's deferred reply after every tap), so pick any legal move and
	// tap it. A ply cap guards against a stuck game.
	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 200 {
			t.Fatalf("game did not terminate after 200 plies")
		}
		if a.gs.AITurn() {
			t.Fatalf("harness returned control on the AI's turn (deferred move not drained)")
		}
		moves := a.gs.Board.LegalMoves(a.gs.Turn)
		if len(moves) == 0 {
			t.Fatalf("human to move but no legal moves, yet game not over (ply %d)", ply)
		}
		before := empties(&a.gs.Board)

		cell := a.layout.CellToScreen(moves[0][0], moves[0][1])
		h.TapRect(cell)

		after := empties(&a.gs.Board)
		if a.gs.Phase == game.PhasePlaying && after >= before {
			t.Fatalf("tapping legal move %v placed no disc (empties %d -> %d)", moves[0], before, after)
		}
		// The board must always stay consistent: 64 squares, no leaks.
		total := a.gs.Board.Count(game.Black) + a.gs.Board.Count(game.White) + after
		if total != game.Size*game.Size {
			t.Fatalf("board disc count inconsistent: %d (want %d)", total, game.Size*game.Size)
		}
	}

	// Game over: the winner must agree with the disc counts.
	bl := a.gs.Board.Count(game.Black)
	wh := a.gs.Board.Count(game.White)
	want := game.Empty
	switch {
	case bl > wh:
		want = game.Black
	case wh > bl:
		want = game.White
	}
	if a.gs.Winner() != want {
		t.Fatalf("winner %v disagrees with score B%d/W%d", a.gs.Winner(), bl, wh)
	}
	if _, ok := h.FindTextContains("vinner"); !ok {
		if _, tie := h.FindTextContains("Oavgjort"); !tie {
			t.Fatalf("no end-of-game banner shown; visible: %v", texts(h))
		}
	}

	if dir := os.Getenv("PLAYTEST_SHOTS"); dir != "" {
		if err := h.Screenshot(dir + "/othello_end.png"); err != nil {
			t.Fatalf("screenshot: %v", err)
		}
	}
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}
