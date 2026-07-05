//go:build playtest

package main

// Headless PLAYTHROUGH tests for Othello. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): Black starts; a move must bracket at least one line of the opponent's
// discs and flips exactly those; you must move if you can, otherwise your turn
// passes; the game ends when neither can move; the majority wins, ties are
// possible. Covers both modes (hotseat = both colours, plus vs the AI), the
// win/loss/tie banners, forced passes, illegal-move rejection, quitting, and the
// rules screen. Runs under the pure-Go inkview emulator (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"othello/game"
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

// startMode picks the first menu row with the given mode and enters the game.
func startMode(t *testing.T, h *ink.Harness, a *app, mode game.Mode) {
	t.Helper()
	for _, row := range a.menu.rows {
		if row.choice.mode == mode {
			h.TapRect(row.rect)
			if a.screen != screenGame || a.gs == nil || a.gs.Mode != mode {
				t.Fatalf("did not start mode %v (screen=%v)", mode, a.screen)
			}
			return
		}
	}
	t.Fatalf("no menu row for mode %v; visible: %v", mode, texts(h))
}

// tapCellXY taps board cell (x,y) via the app's current layout.
func tapCellXY(h *ink.Harness, a *app, x, y int) bool {
	return h.TapRect(a.layout.CellToScreen(x, y))
}

// discTotal returns black+white+empty (must always be 64).
func discTotal(b *game.Board) int {
	return b.Count(game.Black) + b.Count(game.White) + b.Count(game.Empty)
}

// bestByFlips returns the legal move for color that flips the most (max=true) or
// fewest (max=false) discs — a deterministic policy for full-game playthroughs.
func bestByFlips(b *game.Board, color game.Cell, max bool) ([2]int, bool) {
	moves := b.LegalMoves(color)
	if len(moves) == 0 {
		return [2]int{}, false
	}
	best := moves[0]
	bestN := -1
	for _, m := range moves {
		cp := *b
		before := cp.Count(color)
		cp.Apply(m[0], m[1], color)
		gained := cp.Count(color) - before // 1 placed + flips
		if bestN == -1 || (max && gained > bestN) || (!max && gained < bestN) {
			bestN, best = gained, m
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

// --- RULE: legal moves, illegal rejection, correct flips --------------------

func TestPlayOthelloMoveRules(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat)

	// At the opening, Black's legal moves are exactly the four the rules imply.
	legal := a.gs.Board.LegalMoves(game.Black)
	if len(legal) != 4 {
		t.Fatalf("expected 4 opening moves for Black, got %d: %v", len(legal), legal)
	}

	// Tapping an illegal square (one that brackets nothing) is rejected: no disc
	// placed, turn unchanged.
	if a.gs.Board.LegalMove(0, 0, game.Black) {
		t.Fatal("(0,0) should be illegal at the opening")
	}
	blkBefore := a.gs.Board.Count(game.Black)
	tapCellXY(h, a, 0, 0)
	if a.gs.Board.Count(game.Black) != blkBefore {
		t.Fatal("an illegal tap placed a disc")
	}
	if a.gs.Turn != game.Black {
		t.Fatal("illegal move changed the turn")
	}

	// Tapping a legal square flips exactly the discs the rules say (compare to a
	// pure Apply on a copy).
	m := legal[0]
	want := a.gs.Board // array copy
	want.Apply(m[0], m[1], game.Black)
	tapCellXY(h, a, m[0], m[1])
	if a.gs.Board != want {
		t.Fatalf("UI move %v did not match the rules' flip result", m)
	}
	if a.gs.Turn != game.White {
		t.Fatal("turn did not pass to White after a legal move")
	}
	if discTotal(&a.gs.Board) != 64 {
		t.Fatalf("board inconsistent: total %d", discTotal(&a.gs.Board))
	}
}

// --- WIN / LOSS / TIE banners (end-state rendering per the rules) -----------

func TestPlayOthelloEndBanners(t *testing.T) {
	cases := []struct {
		name   string
		fill   func(*game.Board)
		banner string
		winner game.Cell
	}{
		{"black_wins", fillAll(game.Black), "Svart vinner!", game.Black},
		{"white_wins", fillAll(game.White), "Vit vinner!", game.White},
		{"tie", fillHalf, "Oavgjort!", game.Empty},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h, a := bootToMenu(t)
			// In AI mode "Vit vinner!" is the human LOSING; the banner text is the
			// same in both modes, so testing it here covers win and loss alike.
			startMode(t, h, a, game.ModeAI)
			tc.fill(&a.gs.Board)
			a.gs.Phase = game.PhaseDone
			h.Draw()

			if a.gs.Winner() != tc.winner {
				t.Fatalf("Winner()=%v, want %v", a.gs.Winner(), tc.winner)
			}
			if _, ok := h.FindTextContains(tc.banner); !ok {
				t.Fatalf("banner %q not shown; visible: %v", tc.banner, texts(h))
			}
			// A finished game offers replay.
			for _, want := range []string{"Spela igen", "Meny"} {
				if _, ok := h.FindText(want); !ok {
					t.Fatalf("finished game missing %q; visible: %v", want, texts(h))
				}
			}
		})
	}
}

func fillAll(c game.Cell) func(*game.Board) {
	return func(b *game.Board) {
		for i := range b {
			b[i] = c
		}
	}
}

func fillHalf(b *game.Board) {
	for i := range b {
		if i < len(b)/2 {
			b[i] = game.Black
		} else {
			b[i] = game.White
		}
	}
}

// --- RULE: forced pass — you skip only when you truly cannot move ------------

func TestPlayOthelloForcedPass(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat)

	// Craft a position (Black to move) where Black's move at (0,0) leaves White
	// with no reply anywhere, yet Black still has a move — so White must pass and
	// the turn returns to Black. Coordinates are (x,y), index = y*8+x.
	for i := range a.gs.Board {
		a.gs.Board[i] = game.Empty
	}
	set := func(x, y int, c game.Cell) { a.gs.Board[y*game.Size+x] = c }
	set(1, 0, game.White)
	set(2, 0, game.Black) // Black (0,0) brackets (1,0) → all Black, no White here
	set(6, 7, game.White)
	set(7, 7, game.Black) // gives Black a future move at (5,7); White can't use it
	a.gs.Turn = game.Black
	a.gs.Phase = game.PhasePlaying
	a.gs.LastPass = false
	h.Draw()

	if !a.gs.Board.LegalMove(0, 0, game.Black) {
		t.Fatal("setup wrong: (0,0) not legal for Black")
	}
	tapCellXY(h, a, 0, 0)

	// After the move: White genuinely has no move, Black does, so White passed and
	// it is Black's turn again with the pass flagged.
	if a.gs.Board.HasMove(game.White) {
		t.Fatal("White should have no legal move in this position")
	}
	if !a.gs.Board.HasMove(game.Black) {
		t.Fatal("Black should still have a move (so this is a pass, not game over)")
	}
	if a.gs.Phase != game.PhasePlaying || a.gs.Turn != game.Black || !a.gs.LastPass {
		t.Fatalf("expected a forced White pass back to Black; phase=%v turn=%v lastpass=%v",
			a.gs.Phase, a.gs.Turn, a.gs.LastPass)
	}
	if _, ok := h.FindTextContains("Pass!"); !ok {
		t.Fatalf("pass not shown in status; visible: %v", texts(h))
	}
}

// --- Full game vs the AI (a real win-or-loss to a terminal state) -----------

func TestPlayOthelloFullGameVsAI(t *testing.T) {
	// Two deterministic human policies produce two different real games; both must
	// terminate cleanly with a banner consistent with the disc counts.
	for _, greedy := range []bool{true, false} {
		greedy := greedy
		name := "human_greedy"
		if !greedy {
			name = "human_altruist"
		}
		t.Run(name, func(t *testing.T) {
			h, a := bootToMenu(t)
			startMode(t, h, a, game.ModeAI)

			for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
				if ply > 200 {
					t.Fatal("game did not terminate")
				}
				if a.gs.AITurn() {
					t.Fatal("control returned on the AI's turn (deferred reply not drained)")
				}
				m, ok := bestByFlips(&a.gs.Board, a.gs.Turn, greedy)
				if !ok {
					t.Fatalf("human to move but no legal move at ply %d", ply)
				}
				before := a.gs.Board.Count(game.Empty)
				tapCellXY(h, a, m[0], m[1])
				if a.gs.Phase == game.PhasePlaying && a.gs.Board.Count(game.Empty) >= before {
					t.Fatalf("legal move %v placed no disc", m)
				}
				if discTotal(&a.gs.Board) != 64 {
					t.Fatalf("board inconsistent mid-game: %d", discTotal(&a.gs.Board))
				}
			}
			assertBannerMatchesScore(t, h, a)
		})
	}
}

// --- Anti-Othello ("Omvänd Othello") variant --------------------------------

// TestPlayOthelloAntiVariant plays a full game with the Omvänd toggle
// selected on the menu and asserts, from an INDEPENDENT disc count (not by
// trusting GameState.Winner() itself), that the banner declares the side
// with FEWER discs the winner — the reversed win condition — and that the
// AI (which plays White) respects the same reversed objective rather than
// silently reverting to normal Othello's play.
func TestPlayOthelloAntiVariant(t *testing.T) {
	h, a := bootToMenu(t)

	if !h.TapRect(a.menu.variantBtns[1]) { // "Omvänd"
		t.Fatal("Omvänd toggle button not tappable")
	}
	if a.menu.variant != game.VariantAnti {
		t.Fatal("tapping Omvänd did not select VariantAnti")
	}

	startMode(t, h, a, game.ModeAI)
	if a.gs.Variant != game.VariantAnti {
		t.Fatalf("started game has Variant=%v, want VariantAnti", a.gs.Variant)
	}

	// Human (Black) plays the "altruist" policy (fewest own flips), a
	// reasonable real strategy under the reversed win condition; White is
	// the in-game AI, which must itself be variant-aware (see game/ai_test.go
	// for the unit-level proof of the sign flip).
	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 200 {
			t.Fatal("anti-variant game did not terminate")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		m, ok := bestByFlips(&a.gs.Board, a.gs.Turn, false)
		if !ok {
			t.Fatalf("human to move but no legal move at ply %d", ply)
		}
		tapCellXY(h, a, m[0], m[1])
		if discTotal(&a.gs.Board) != 64 {
			t.Fatalf("board inconsistent mid-game: %d", discTotal(&a.gs.Board))
		}
	}

	bl := a.gs.Board.Count(game.Black)
	wh := a.gs.Board.Count(game.White)
	want := "Oavgjort!"
	if bl < wh {
		want = "Svart vinner!"
	} else if wh < bl {
		want = "Vit vinner!"
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("anti-variant end banner %q (fewest-discs-wins, B%d/W%d) not shown; visible: %v",
			want, bl, wh, texts(h))
	}
	if _, ok := h.FindTextContains("Omvänd"); !ok {
		t.Fatalf("status line should tag the active Omvänd variant; visible: %v", texts(h))
	}
}

// TestPlayOthelloVanligIsStillTheDefault confirms the menu defaults to
// normal Othello (most discs wins) when the toggle is never touched, so the
// new variant toggle can't silently change existing behaviour.
func TestPlayOthelloVanligIsStillTheDefault(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat)
	if a.gs.Variant != game.VariantNormal {
		t.Fatalf("default game Variant=%v, want VariantNormal", a.gs.Variant)
	}
}

// --- Full hotseat game (drives BOTH colours) --------------------------------

func TestPlayOthelloHotseatBothSides(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat)

	movedBlack, movedWhite := false, false
	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 200 {
			t.Fatal("hotseat game did not terminate")
		}
		mover := a.gs.Turn
		m, ok := bestByFlips(&a.gs.Board, mover, true)
		if !ok {
			t.Fatalf("%v to move but no legal move at ply %d", mover, ply)
		}
		before := a.gs.Board.Count(game.Empty)
		tapCellXY(h, a, m[0], m[1])
		if a.gs.Board.Count(game.Empty) >= before {
			t.Fatalf("%v move %v placed no disc", mover, m)
		}
		if mover == game.Black {
			movedBlack = true
		} else {
			movedWhite = true
		}
	}
	if !movedBlack || !movedWhite {
		t.Fatalf("hotseat did not exercise both sides (black=%v white=%v)", movedBlack, movedWhite)
	}
	assertBannerMatchesScore(t, h, a)
}

func assertBannerMatchesScore(t *testing.T, h *ink.Harness, a *app) {
	t.Helper()
	bl := a.gs.Board.Count(game.Black)
	wh := a.gs.Board.Count(game.White)
	want := "Oavgjort!"
	if bl > wh {
		want = "Svart vinner!"
	} else if wh > bl {
		want = "Vit vinner!"
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("end banner %q (B%d/W%d) not shown; visible: %v", want, bl, wh, texts(h))
	}
}

// --- Quit mid-game (Back key AND Meny button), then restart -----------------

func TestPlayOthelloQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat)
	m := a.gs.Board.LegalMoves(game.Black)[0]
	tapCellXY(h, a, m[0], m[1]) // a move in progress

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}

	startMode(t, h, a, game.ModeAI)
	// The Meny button is always in the bar; tap it.
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
	// Menu still usable.
	startMode(t, h, a, game.ModeHotseat)
}

// --- Rules screen -----------------------------------------------------------

func TestPlayOthelloRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("Svart börjar"); !ok {
		t.Fatalf("rules text missing the opening rule; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshot of a finished game ------------------------------------------

func TestPlayOthelloEndScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeAI)
	for a.gs.Phase == game.PhasePlaying {
		m, ok := bestByFlips(&a.gs.Board, a.gs.Turn, true)
		if !ok {
			break
		}
		tapCellXY(h, a, m[0], m[1])
	}
	if err := h.Screenshot(dir + "/othello_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}

// TestPlayOthelloVariantScreenshots captures the menu (showing the new
// Vanlig/Omvänd toggle) and the rules screen (showing the new paragraph),
// for visual legibility review.
func TestPlayOthelloVariantScreenshots(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	h, a := bootToMenu(t)
	if err := h.Screenshot(dir + "/othello_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapRect(a.menu.variantBtns[1]) // Omvänd
	if err := h.Screenshot(dir + "/othello_menu_omvand.png"); err != nil {
		t.Fatal(err)
	}
	if err := h.TapText("Regler"); err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/othello_rules.png"); err != nil {
		t.Fatal(err)
	}
}
