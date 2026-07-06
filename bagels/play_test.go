//go:build playtest

package main

// Headless PLAYTHROUGH tests for Bagels. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): the secret is distinct digits; each guess scores Fermi (right digit,
// right place) and Pico (right digit, wrong place), or "Bagels" for none; you
// win by matching the code and LOSE when the guesses run out. Covers win, loss,
// feedback correctness (vs an independent scorer), the distinct-digit rule, and
// quit/replay/rules. Runs under the pure-Go inkview emulator (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"bagels/game"
)

// --- helpers ----------------------------------------------------------------

func bootToMenu(t *testing.T) (*ink.Harness, *app) {
	t.Helper()
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if a.screen != screenMenu {
		t.Fatalf("splash tap did not open menu, screen=%v", a.screen)
	}
	return h, a
}

func startPreset(t *testing.T, h *ink.Harness, a *app, i int) game.Code {
	t.Helper()
	if err := h.TapText(game.Presets[i].Name); err != nil {
		t.Fatalf("could not start %q: %v", game.Presets[i].Name, err)
	}
	if a.screen != screenGame {
		t.Fatalf("did not enter game for preset %d, screen=%v", i, a.screen)
	}
	return append(game.Code{}, a.gs.Secret...)
}

func tapDigitKey(t *testing.T, h *ink.Harness, a *app, d int) {
	t.Helper()
	for _, k := range a.keys {
		if k.Digit == d {
			h.TapRect(k.Rect)
			return
		}
	}
	t.Fatalf("digit key %d not available (entry=%v)", d, a.gs.Entry)
}

func tapButton(t *testing.T, h *ink.Harness, a *app, label string) {
	t.Helper()
	for _, b := range a.buttons {
		if b.Label == label {
			h.TapRect(b.Rect)
			return
		}
	}
	t.Fatalf("button %q not present (have %v)", label, a.buttonLabels())
}

func enterAndGuess(t *testing.T, h *ink.Harness, a *app, code game.Code) {
	t.Helper()
	for _, d := range code {
		tapDigitKey(t, h, a, d)
	}
	if !a.gs.EntryComplete() {
		t.Fatalf("entry incomplete after %v: %v", code, a.gs.Entry)
	}
	tapButton(t, h, a, "Gissa")
}

func rotate(c game.Code) game.Code {
	if len(c) < 2 {
		return c
	}
	out := append(game.Code{}, c[1:]...)
	return append(out, c[0])
}

// refScore scores from the rules (distinct digits): Fermi = right place, Pico =
// present in the secret but a different place.
func refScore(secret, guess game.Code) (fermi, pico int) {
	for i := range guess {
		switch {
		case guess[i] == secret[i]:
			fermi++
		case contains(secret, guess[i]):
			pico++
		}
	}
	return
}

func contains(c game.Code, d int) bool {
	for _, x := range c {
		if x == d {
			return true
		}
	}
	return false
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- WIN on every preset + feedback matches the rules -----------------------

func TestPlayBagelsWinAllPresets(t *testing.T) {
	for i := range game.Presets {
		i := i
		t.Run(game.Presets[i].Name, func(t *testing.T) {
			h, a := bootToMenu(t)
			secret := startPreset(t, h, a, i)

			// A rotated guess is a derangement: 0 Fermi, all Pico.
			wrong := rotate(secret)
			enterAndGuess(t, h, a, wrong)
			sc := a.gs.Guesses[0].Score
			wf, wp := refScore(secret, wrong)
			if sc.Fermi != wf || sc.Pico != wp {
				t.Fatalf("score Fermi %d/Pico %d, rules say %d/%d", sc.Fermi, sc.Pico, wf, wp)
			}
			if sc.Fermi != 0 || sc.Pico != len(secret) {
				t.Fatalf("rotated guess should be all Pico: %+v", sc)
			}
			if a.gs.Over() {
				t.Fatal("game over on the first (wrong) guess")
			}

			enterAndGuess(t, h, a, secret)
			if !a.gs.Solved {
				t.Fatalf("guessing the secret %v did not win", secret)
			}
			if _, ok := h.FindTextContains("Löst"); !ok {
				t.Fatalf("win banner missing; visible: %v", texts(h))
			}
		})
	}
}

// --- LOSS when the guesses run out ------------------------------------------

func TestPlayBagelsLoss(t *testing.T) {
	h, a := bootToMenu(t)
	secret := startPreset(t, h, a, 0) // Lätt: 3 digits, 10 guesses
	wrong := rotate(secret)           // never the secret

	max := a.gs.Cfg.MaxTurn
	for n := 0; n < max; n++ {
		if a.gs.Over() {
			t.Fatalf("game ended early after %d guesses", n)
		}
		enterAndGuess(t, h, a, wrong)
	}
	if !a.gs.Lost {
		t.Fatalf("expected a loss after %d wrong guesses (solved=%v lost=%v)", max, a.gs.Solved, a.gs.Lost)
	}
	if _, ok := h.FindTextContains("Slut"); !ok {
		t.Fatalf("loss banner missing; visible: %v", texts(h))
	}
	// The keypad is dead once the game is over.
	if len(a.keys) != 0 {
		t.Fatalf("keypad still live after a loss (%d keys)", len(a.keys))
	}
}

// --- RULE: distinct digits; no Gissa until the entry is complete ------------

func TestPlayBagelsDistinctDigitRule(t *testing.T) {
	h, a := bootToMenu(t)
	startPreset(t, h, a, 1) // classic 4-digit

	tapDigitKey(t, h, a, 6)
	if a.gs.DigitAvailable(6) {
		t.Fatal("digit 6 still available after being placed")
	}
	for _, k := range a.keys {
		if k.Digit == 6 {
			t.Fatal("digit 6 still tappable (repeat allowed)")
		}
	}
	if _, ok := h.FindText("Gissa"); ok {
		t.Fatal("Gissa offered with an incomplete entry")
	}
	// Sudda frees the digit again.
	tapButton(t, h, a, "Sudda")
	if !a.gs.DigitAvailable(6) {
		t.Fatal("digit 6 not available again after Sudda")
	}
}

// --- Replay / quit / rules --------------------------------------------------

func TestPlayBagelsReplayQuitRules(t *testing.T) {
	h, a := bootToMenu(t)
	secret := startPreset(t, h, a, 0)
	enterAndGuess(t, h, a, secret)
	if !a.gs.Solved {
		t.Fatal("setup win failed")
	}
	tapButton(t, h, a, "Spela igen")
	if a.gs.Over() || len(a.gs.Guesses) != 0 {
		t.Fatalf("Spela igen did not reset (over=%v guesses=%d)", a.gs.Over(), len(a.gs.Guesses))
	}

	tapButton(t, h, a, "Meny")
	if a.screen != screenMenu {
		t.Fatalf("Meny did not return to menu, screen=%v", a.screen)
	}

	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	for _, want := range []string{"Fermi", "Pico"} {
		if _, ok := h.FindTextContains(want); !ok {
			t.Fatalf("rules missing %q; visible: %v", want, texts(h))
		}
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave rules, screen=%v", a.screen)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayBagelsScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if e := h.Screenshot(dir + "/bagels_splash.png"); e != nil {
		t.Fatal(e)
	}
	h.TapXY(500, 700) // dismiss splash -> menu
	_ = h.Screenshot(dir + "/bagels_menu.png")
	if _, ok := h.FindText("Regler"); ok {
		_ = h.TapText("Regler")
		_ = h.Screenshot(dir + "/bagels_rules.png")
		h.Back()
	}

	secret := startPreset(t, h, a, 1)
	// One wrong (all-Pico) guess in progress, showing the Fermi/Pico board.
	enterAndGuess(t, h, a, rotate(secret))
	_ = h.Screenshot(dir + "/bagels_board.png")
	// Then guess the code for the win banner.
	enterAndGuess(t, h, a, secret)
	if err := h.Screenshot(dir + "/bagels_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
