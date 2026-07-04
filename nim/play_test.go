//go:build playtest

package main

// Headless PLAYTHROUGH tests for Nim. They drive the real touch path and check
// the gameplay against the rules as written (see rulesParagraphs in ui.go): take
// 1..pileSize sticks from ONE pile per turn; Normal = taking the last stick
// WINS, Misère = taking the last stick LOSES; solo AI plays perfectly. The
// perfect-play tests exercise a human win AND a human loss deterministically by
// starting from a known N-position and a known P-position. Runs under the pure-Go
// inkview emulator (playtest/play.sh).

import (
	"os"
	"strconv"
	"testing"

	ink "github.com/dennwc/inkview"

	"nim/game"
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

func tapMenuID(t *testing.T, h *ink.Harness, a *app, id string) {
	t.Helper()
	for _, b := range a.menu.Buttons() {
		if b.ID == id {
			h.TapRect(b.Rect)
			return
		}
	}
	t.Fatalf("no menu button %q", id)
}

// startGame configures the menu (variant/mode/preset) and starts the game.
func startGame(t *testing.T, h *ink.Harness, a *app, v game.Variant, m game.Mode, presetIdx int) {
	t.Helper()
	for i := 0; a.menu.Variant != v && i < 3; i++ {
		tapMenuID(t, h, a, "variant")
	}
	for i := 0; a.menu.Mode != m && i < 3; i++ {
		tapMenuID(t, h, a, "mode")
	}
	for i := 0; a.menu.PresetIdx != presetIdx && i < 5; i++ {
		tapMenuID(t, h, a, "preset")
	}
	if a.menu.Variant != v || a.menu.Mode != m || a.menu.PresetIdx != presetIdx {
		t.Fatalf("menu config failed: got v=%v m=%v p=%d", a.menu.Variant, a.menu.Mode, a.menu.PresetIdx)
	}
	tapMenuID(t, h, a, "start")
	if a.screen != screenGame || a.gs == nil {
		t.Fatalf("did not enter game, screen=%v", a.screen)
	}
}

// playMove selects a pile, dials the count with +/-, and confirms — the exact
// sequence a player performs.
func playMove(t *testing.T, h *ink.Harness, a *app, m game.Move) {
	t.Helper()
	if m.Pile < 0 || m.Pile >= len(a.layout.PileRects) {
		t.Fatalf("bad pile %d", m.Pile)
	}
	h.TapRect(a.layout.PileRects[m.Pile])
	if a.selPile != m.Pile {
		t.Fatalf("selecting pile %d failed (sel=%d)", m.Pile, a.selPile)
	}
	for a.selCount < m.Count {
		before := a.selCount
		h.TapRect(a.layout.Plus.Rect)
		if a.selCount == before {
			t.Fatalf("Plus capped at %d but pile %d has %d", a.selCount, m.Pile, a.gs.Piles[m.Pile])
		}
	}
	for a.selCount > m.Count {
		h.TapRect(a.layout.Minus.Rect)
	}
	h.TapRect(a.layout.Confirm.Rect)
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- Perfect play: human WIN from an N-position, LOSS from a P-position ------

func TestPlayNimPerfectPlayOutcomes(t *testing.T) {
	// Preset 0 (3-4-5) is an N-position: the mover (human) can force a win.
	// Preset 1 (1-3-5-7) is a P-position: the mover loses to perfect play.
	// This holds for both variants because BestMove/IsWinningForMover handle each.
	cases := []struct {
		variant game.Variant
		preset  int
	}{
		{game.Normal, 0}, {game.Normal, 1},
		{game.Misere, 0}, {game.Misere, 1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.variant.String()+"_preset"+strconv.Itoa(tc.preset), func(t *testing.T) {
			h, a := bootToMenu(t)
			startGame(t, h, a, tc.variant, game.SoloAI, tc.preset)

			humanShouldWin := a.gs.IsWinningForMover() // human moves first (Turn 0)

			for ply := 0; !a.gs.Over; ply++ {
				if ply > 100 {
					t.Fatal("game did not terminate")
				}
				if a.gs.Turn != 0 {
					t.Fatalf("control returned on the AI's turn (turn=%d)", a.gs.Turn)
				}
				m, ok := a.gs.BestMove()
				if !ok {
					t.Fatal("no move available though game not over")
				}
				playMove(t, h, a, m)
			}

			wantWinner := 1 // AI
			if humanShouldWin {
				wantWinner = 0 // human
			}
			if a.gs.Winner != wantWinner {
				t.Fatalf("perfect play: winner=%d, expected %d (humanShouldWin=%v)",
					a.gs.Winner, wantWinner, humanShouldWin)
			}
			banner := "Spelare 1 vinner!"
			if wantWinner == 1 {
				banner = "AI vinner!"
			}
			if _, ok := h.FindTextContains(banner); !ok {
				t.Fatalf("banner %q missing; visible: %v", banner, texts(h))
			}
		})
	}
}

// --- RULE: last stick decides, per variant (hotseat drives both players) -----

func TestPlayNimTwoPlayerLastStickRule(t *testing.T) {
	for _, v := range []game.Variant{game.Normal, game.Misere} {
		v := v
		t.Run(v.String(), func(t *testing.T) {
			h, a := bootToMenu(t)
			startGame(t, h, a, v, game.TwoPlayer, 0)

			var lastMover int
			movers := map[int]bool{}
			for ply := 0; !a.gs.Over; ply++ {
				if ply > 100 {
					t.Fatal("game did not terminate")
				}
				lastMover = a.gs.Turn
				movers[lastMover] = true
				m, ok := a.gs.BestMove()
				if !ok {
					t.Fatal("no move though not over")
				}
				playMove(t, h, a, m)
			}
			// Rule: whoever takes the last stick WINS in Normal, LOSES in Misère.
			want := lastMover
			if v == game.Misere {
				want = 1 - lastMover
			}
			if a.gs.Winner != want {
				t.Fatalf("%v: last mover %d, winner=%d, want %d", v, lastMover, a.gs.Winner, want)
			}
			if !movers[0] || !movers[1] {
				t.Fatal("hotseat did not exercise both players")
			}
		})
	}
}

// --- RULE: take 1..pileSize from ONE pile; +/- clamp; empty pile unselectable -

func TestPlayNimMoveConstraints(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, game.Normal, game.TwoPlayer, 0) // 3-4-5

	// Select the largest pile and try to over-dial the count: it must clamp at
	// the pile size (you cannot take more than a pile holds).
	pile := 2 // 5 sticks
	size := a.gs.Piles[pile]
	h.TapRect(a.layout.PileRects[pile])
	if a.selPile != pile || a.selCount != 1 {
		t.Fatalf("pile select set sel=%d count=%d", a.selPile, a.selCount)
	}
	for i := 0; i < size+5; i++ {
		h.TapRect(a.layout.Plus.Rect)
	}
	if a.selCount != size {
		t.Fatalf("count %d exceeded pile size %d", a.selCount, size)
	}
	// Minus clamps at 1 (must take at least one).
	for i := 0; i < size+5; i++ {
		h.TapRect(a.layout.Minus.Rect)
	}
	if a.selCount != 1 {
		t.Fatalf("count fell below 1 (%d)", a.selCount)
	}

	// Confirm removes exactly selCount from exactly that pile.
	other := a.gs.Piles[0]
	h.TapRect(a.layout.Confirm.Rect)
	if a.gs.Piles[pile] != size-1 {
		t.Fatalf("confirm took wrong amount: pile now %d (was %d)", a.gs.Piles[pile], size)
	}
	if a.gs.Piles[0] != other {
		t.Fatal("a move changed a different pile")
	}
}

// --- Quit mid-game (Meny button AND Back key), and restart when over ---------

func TestPlayNimQuitAndRestart(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a, game.Normal, game.TwoPlayer, 0)

	h.TapRect(a.layout.MenuBtn.Rect)
	if a.screen != screenMenu {
		t.Fatalf("Meny button did not return to menu, screen=%v", a.screen)
	}

	startGame(t, h, a, game.Normal, game.TwoPlayer, 0)
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not return to menu, screen=%v", a.screen)
	}

	// Play a game to the end, then a tap should restart it.
	startGame(t, h, a, game.Normal, game.SoloAI, 0)
	for ply := 0; !a.gs.Over && ply < 100; ply++ {
		if a.gs.Turn != 0 {
			t.Fatal("AI turn leaked")
		}
		m, _ := a.gs.BestMove()
		playMove(t, h, a, m)
	}
	if !a.gs.Over {
		t.Fatal("game did not finish")
	}
	h.TapXY(500, 500) // any non-Meny tap restarts
	if a.gs.Over {
		t.Fatal("tap after game-over did not restart")
	}
}

// --- Rules screen -----------------------------------------------------------

func TestPlayNimRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	tapMenuID(t, h, a, "rules")
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("Misère"); !ok {
		t.Fatalf("rules text missing the variants; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave rules, screen=%v", a.screen)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayNimScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	h, a := bootToMenu(t)
	startGame(t, h, a, game.Normal, game.SoloAI, 0)
	for ply := 0; !a.gs.Over && ply < 100; ply++ {
		if a.gs.Turn != 0 {
			break
		}
		m, _ := a.gs.BestMove()
		playMove(t, h, a, m)
	}
	if err := h.Screenshot(dir + "/nim_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
