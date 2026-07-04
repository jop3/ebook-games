//go:build playtest

package main

// Headless PLAYTHROUGH test: boots the real app and plays a whole game through
// the real touch path (splash -> menu -> game -> guesses -> win), asserting the
// gameplay actually works and the on-screen feedback is correct. Runs on a PC
// under the pure-Go inkview emulator (see playtest/play.sh); it is a _test.go
// file so the device Docker build ignores it.

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"bullscows/game"
)

// tapDigitKey finds the live keypad key for digit d and taps it. It re-reads
// a.keys each call because the app rebuilds the keypad on every Draw.
func tapDigitKey(t *testing.T, h *ink.Harness, a *app, d int) {
	t.Helper()
	for _, k := range a.keys {
		if k.Digit == d {
			h.TapRect(k.Rect)
			return
		}
	}
	t.Fatalf("digit key %d not available on keypad (entry=%v)", d, a.gs.Entry)
}

// tapButton finds a button by label and taps it.
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
		t.Fatalf("entry not complete after typing %v: got %v", code, a.gs.Entry)
	}
	tapButton(t, h, a, "Gissa")
}

func TestPlayBullsCowsToWin(t *testing.T) {
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Splash screen is up and shows the title.
	if _, ok := h.FindText("Bulls & Cows"); !ok {
		t.Fatalf("splash title missing; visible: %v", texts(h))
	}
	// Tapping the splash advances to the menu.
	h.Tap(image.Pt(500, 700))
	if a.screen != screenMenu {
		t.Fatalf("tap on splash did not open menu, screen=%v", a.screen)
	}

	// 2. Menu shows the three difficulties. Pick the classic 4-digit game.
	if err := h.TapText(game.Presets[1].Name); err != nil {
		t.Fatalf("could not start classic game: %v", err)
	}
	if a.screen != screenGame {
		t.Fatalf("did not enter game screen, screen=%v", a.screen)
	}
	if a.gs.Cfg.Length != 4 {
		t.Fatalf("expected 4-digit game, got %d", a.gs.Cfg.Length)
	}
	secret := append(game.Code{}, a.gs.Secret...)

	// 3. A wrong guess (secret rotated by one) has no fixed points: it must score
	//    0 bulls and (length) cows. This checks the whole input->scoring->display
	//    chain, not just the pure logic.
	wrong := rotate(secret)
	enterAndGuess(t, h, a, wrong)
	if len(a.gs.Guesses) != 1 {
		t.Fatalf("guess not recorded, have %d", len(a.gs.Guesses))
	}
	got := a.gs.Guesses[0].Score
	want := game.Evaluate(secret, wrong)
	if got != want {
		t.Fatalf("scored %+v, expected %+v for guess %v vs secret %v", got, want, wrong, secret)
	}
	if got.Bulls != 0 || got.Cows != len(secret) {
		t.Fatalf("rotated guess should be all cows: got %+v", got)
	}
	if a.gs.Solved {
		t.Fatal("game solved on a deliberately wrong guess")
	}

	// 4. Before the entry is complete there must be no "Gissa" button — you can't
	//    submit a half-typed code.
	tapDigitKey(t, h, a, secret[0])
	if _, ok := h.FindText("Gissa"); ok {
		t.Fatal("Gissa offered with an incomplete entry")
	}
	// Clear the stray digit so we can type the real answer cleanly.
	tapButton(t, h, a, "Sudda")

	// 5. Enter the actual secret and win.
	enterAndGuess(t, h, a, secret)
	if !a.gs.Solved {
		t.Fatalf("typing the secret %v did not win; last score %+v", secret, a.gs.Guesses[len(a.gs.Guesses)-1].Score)
	}
	if _, ok := h.FindTextContains("Löst"); !ok {
		t.Fatalf("win status not shown; visible: %v", texts(h))
	}

	// 6. Snapshot the winning board so a human can confirm it "makes sense".
	if dir := os.Getenv("PLAYTEST_SHOTS"); dir != "" {
		if err := h.Screenshot(dir + "/bullscows_win.png"); err != nil {
			t.Fatalf("screenshot: %v", err)
		}
	}
}

func rotate(c game.Code) game.Code {
	if len(c) < 2 {
		return c
	}
	out := append(game.Code{}, c[1:]...)
	return append(out, c[0])
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}
