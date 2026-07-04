//go:build playtest

package main

// Headless PLAYTHROUGH tests for Mastermind. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// main.go): guess the hidden code; each guess scores black pegs (right colour,
// right place) and white pegs (right colour, wrong place); you win on all-black
// and lose after MaxGuesses. The "Enheten gissar" mode has the Knuth solver
// guess YOUR code from truthful feedback. Covers win, loss, feedback
// correctness (vs an independent scorer), the solver, and quit/rules. Runs under
// the pure-Go inkview emulator (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"
)

// --- helpers ----------------------------------------------------------------

func bootToMenu(t *testing.T) (*ink.Harness, *App) {
	t.Helper()
	a := newApp()
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if a.scr != screenMenu {
		t.Fatalf("splash tap did not open menu, scr=%v", a.scr)
	}
	return h, a
}

func startPreset(t *testing.T, h *ink.Harness, a *App, i int) {
	t.Helper()
	if i >= len(a.menuButtons) {
		t.Fatalf("preset %d out of range (%d)", i, len(a.menuButtons))
	}
	h.TapRect(a.menuButtons[i].rect)
	if a.scr != screenGame || a.game == nil {
		t.Fatalf("did not enter game for preset %d, scr=%v", i, a.scr)
	}
}

// enterGuess sets every peg to the given colours (select colour, tap peg).
func enterGuess(t *testing.T, h *ink.Harness, a *App, g Guess) {
	t.Helper()
	for i, c := range g {
		h.TapRect(a.paletteRect[int(c)])
		if a.sel != int(c) {
			t.Fatalf("colour %d not selected (sel=%d)", c, a.sel)
		}
		h.TapRect(a.pegRects[i])
	}
	if !a.draftFull() {
		t.Fatalf("draft not full after entering %v: %v", g, a.draft)
	}
}

func submit(h *ink.Harness, a *App) { h.TapRect(a.confirmBtn.rect) }

// refFeedback scores a guess straight from the rules (handles repeated colours):
// black = right colour and place; white = remaining colour matches by frequency.
func refFeedback(secret Secret, guess Guess) (black, white int) {
	var sc, gc [64]int
	for i := range secret {
		if secret[i] == guess[i] {
			black++
		} else {
			sc[secret[i]]++
			gc[guess[i]]++
		}
	}
	for c := range sc {
		if sc[c] < gc[c] {
			white += sc[c]
		} else {
			white += gc[c]
		}
	}
	return
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- WIN on every preset + feedback matches the rules -----------------------

func TestPlayMastermindWinAllPresets(t *testing.T) {
	for i := range Presets {
		i := i
		t.Run(Presets[i].Name, func(t *testing.T) {
			h, a := bootToMenu(t)
			startPreset(t, h, a, i)
			secret := append(Secret(nil), a.game.Secret...)

			// A wrong guess first (secret with peg 0 bumped): its recorded feedback
			// must match an independent scorer.
			wrong := bump(secret, a.game.Cfg.Colors)
			enterGuess(t, h, a, Guess(wrong))
			submit(h, a)
			gotFb := a.game.History[0].Feedback
			wb, ww := refFeedback(secret, Guess(wrong))
			if gotFb.Black != wb || gotFb.White != ww {
				t.Fatalf("feedback %v/%v, rules say %d/%d for %v vs %v",
					gotFb.Black, gotFb.White, wb, ww, wrong, secret)
			}
			if a.game.Status != Playing {
				t.Fatal("game ended on a wrong guess")
			}

			// Now guess the secret and win.
			enterGuess(t, h, a, Guess(secret))
			submit(h, a)
			if a.game.Status != Won {
				t.Fatalf("guessing the secret %v did not win (status=%v)", secret, a.game.Status)
			}
			if _, ok := h.FindTextContains("Du vann"); !ok {
				t.Fatalf("win banner missing; visible: %v", texts(h))
			}
		})
	}
}

func bump(s Secret, colors int) Secret {
	out := append(Secret(nil), s...)
	// change peg 0 to a different colour within the palette (wraps, stays in range)
	out[0] = Color((int(out[0]) + 1) % colors)
	return out
}

// --- LOSS after MaxGuesses --------------------------------------------------

func TestPlayMastermindLoss(t *testing.T) {
	h, a := bootToMenu(t)
	startPreset(t, h, a, 0) // Klassisk, 10 guesses
	secret := append(Secret(nil), a.game.Secret...)
	wrong := bump(secret, a.game.Cfg.Colors) // differs in peg 0, so never all-black

	max := a.game.Cfg.MaxGuesses
	for n := 0; n < max; n++ {
		if a.game.Status != Playing {
			t.Fatalf("game ended early after %d guesses (status=%v)", n, a.game.Status)
		}
		enterGuess(t, h, a, Guess(wrong))
		submit(h, a)
	}
	if a.game.Status != Lost {
		t.Fatalf("expected a loss after %d wrong guesses, got %v", max, a.game.Status)
	}
	if _, ok := h.FindTextContains("Förlust"); !ok {
		t.Fatalf("loss banner missing; visible: %v", texts(h))
	}
}

// --- The Knuth solver ("Enheten gissar") cracks the player's code -----------

func TestPlayMastermindKnuthSolves(t *testing.T) {
	h, a := bootToMenu(t)
	h.TapRect(a.knuthBtn.rect)
	if a.scr != screenKnuth || a.knuth == nil {
		t.Fatalf("did not enter Knuth mode, scr=%v", a.scr)
	}
	cfg := knuthCfg
	secret := Secret{0, 1, 2, 3} // a valid code for the 4-peg/6-colour config

	for turn := 0; ; turn++ {
		if a.knuth.Solved() {
			break
		}
		if turn > cfg.MaxGuesses {
			t.Fatalf("solver did not crack the code in %d guesses", cfg.MaxGuesses)
		}
		guess := a.knuth.CurrentGuess()
		b, w := refFeedback(secret, guess)
		// Dial the feedback with the steppers, then confirm.
		for k := 0; k < b; k++ {
			h.TapRect(a.kBlackPlus.rect)
		}
		for k := 0; k < w; k++ {
			h.TapRect(a.kWhitePlus.rect)
		}
		if a.kfbBlack != b || a.kfbWhite != w {
			t.Fatalf("feedback steppers set %d/%d, wanted %d/%d", a.kfbBlack, a.kfbWhite, b, w)
		}
		h.TapRect(a.kConfirm.rect)
	}
	if !a.knuth.Solved() {
		t.Fatal("solver reported not solved")
	}
	// The device's final guess must equal the secret.
	final := a.knuth.CurrentGuess()
	for i := range secret {
		if final[i] != secret[i] {
			t.Fatalf("solver 'solved' with wrong code %v (secret %v)", final, secret)
		}
	}
}

// --- Replay, quit, rules ----------------------------------------------------

func TestPlayMastermindReplayQuitRules(t *testing.T) {
	h, a := bootToMenu(t)
	startPreset(t, h, a, 0)
	secret := append(Secret(nil), a.game.Secret...)

	// Win, then "Nytt spel" (newBtn) restarts.
	enterGuess(t, h, a, Guess(secret))
	submit(h, a)
	if a.game.Status != Won {
		t.Fatal("setup win failed")
	}
	h.TapRect(a.newBtn.rect)
	if a.game.Status != Playing || len(a.game.History) != 0 {
		t.Fatalf("new game not reset (status=%v history=%d)", a.game.Status, len(a.game.History))
	}

	// Back to menu.
	h.TapRect(a.backBtn.rect)
	if a.scr != screenMenu {
		t.Fatalf("Back did not return to menu, scr=%v", a.scr)
	}

	// Rules.
	h.TapRect(a.rulesBtn.rect)
	if a.scr != screenRules {
		t.Fatalf("Regler did not open rules, scr=%v", a.scr)
	}
	if _, ok := h.FindTextContains("hemliga koden"); !ok {
		t.Fatalf("rules text missing; visible: %v", texts(h))
	}
	h.Back()
	if a.scr != screenMenu {
		t.Fatalf("Back did not leave rules, scr=%v", a.scr)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayMastermindScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	h, a := bootToMenu(t)
	startPreset(t, h, a, 0)
	secret := append(Secret(nil), a.game.Secret...)
	enterGuess(t, h, a, Guess(bump(secret, a.game.Cfg.Colors)))
	submit(h, a)
	enterGuess(t, h, a, Guess(secret))
	submit(h, a)
	if err := h.Screenshot(dir + "/mastermind_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
