//go:build playtest

package main

// Headless PLAYTHROUGH tests for Bulls & Cows. These drive the real touch path
// (splash -> menu -> game) and check the gameplay against the rules as written
// (see rulesParagraphs in ui.go): distinct digits, Bull = right digit right
// place, Cow = right digit wrong place, unlimited guesses (no loss), erase with
// Sudda, and menu/replay/quit navigation. Runs under the pure-Go inkview
// emulator (playtest/play.sh); gated by the playtest build tag so device builds
// ignore it.

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"bullscows/game"
)

// --- helpers ----------------------------------------------------------------

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

// bootToMenu boots the app and dismisses the splash.
func bootToMenu(t *testing.T) (*ink.Harness, *app) {
	t.Helper()
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := h.FindText("Bulls & Cows"); !ok {
		t.Fatalf("splash title missing; visible: %v", texts(h))
	}
	h.TapXY(500, 700)
	if a.screen != screenMenu {
		t.Fatalf("tap on splash did not open menu, screen=%v", a.screen)
	}
	return h, a
}

// startPreset picks difficulty i off the menu and returns a copy of the secret.
func startPreset(t *testing.T, h *ink.Harness, a *app, i int) game.Code {
	t.Helper()
	if err := h.TapText(game.Presets[i].Name); err != nil {
		t.Fatalf("could not start preset %d (%s): %v", i, game.Presets[i].Name, err)
	}
	if a.screen != screenGame {
		t.Fatalf("did not enter game for preset %d, screen=%v", i, a.screen)
	}
	if a.gs.Cfg.Length != game.Presets[i].Length {
		t.Fatalf("preset %d: expected length %d, got %d", i, game.Presets[i].Length, a.gs.Cfg.Length)
	}
	return append(game.Code{}, a.gs.Secret...)
}

// refScore scores a guess straight from the written rules (distinct digits):
// a Bull is a digit in the right place; a Cow is a guessed digit that appears in
// the secret but in a different place. Independent of game.Evaluate on purpose.
func refScore(secret, guess game.Code) (bulls, cows int) {
	for i := range guess {
		switch {
		case guess[i] == secret[i]:
			bulls++
		case contains(secret, guess[i]):
			cows++
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

// rotate returns the secret shifted by one — a guaranteed derangement (0 bulls,
// all cows) using the same distinct digits.
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

// --- WIN, every difficulty (all "sides") ------------------------------------

func TestPlayBullsCowsWinAllPresets(t *testing.T) {
	for i := range game.Presets {
		i := i
		t.Run(game.Presets[i].Name, func(t *testing.T) {
			h, a := bootToMenu(t)
			secret := startPreset(t, h, a, i)

			// A wrong guess first: the secret rotated by one has no digit in place,
			// so by the rules it must be 0 bulls and (length) cows.
			wrong := rotate(secret)
			enterAndGuess(t, h, a, wrong)
			gb, gc := a.gs.Guesses[0].Score.Bulls, a.gs.Guesses[0].Score.Cows
			if gb != 0 || gc != len(secret) {
				t.Fatalf("rotated guess should be 0 bulls / %d cows, got %d/%d", len(secret), gb, gc)
			}
			if a.gs.Solved {
				t.Fatal("solved on a deliberately wrong guess")
			}

			// Now enter the real secret and win.
			enterAndGuess(t, h, a, secret)
			if !a.gs.Solved {
				t.Fatalf("typing the secret %v did not win", secret)
			}
			if _, ok := h.FindTextContains("Löst"); !ok {
				t.Fatalf("win banner missing; visible: %v", texts(h))
			}
			// A solved game offers replay + menu, and the keypad is fully greyed.
			for _, want := range []string{"Spela igen", "Meny"} {
				if _, ok := h.FindText(want); !ok {
					t.Fatalf("won game missing %q button; visible: %v", want, texts(h))
				}
			}
			if len(a.keys) != 0 {
				t.Fatalf("keypad still tappable after win (%d live keys)", len(a.keys))
			}
		})
	}
}

// --- SCORING matches the rules across many guesses --------------------------

func TestPlayBullsCowsScoringMatchesRules(t *testing.T) {
	h, a := bootToMenu(t)
	secret := startPreset(t, h, a, 1) // classic 4-digit

	// Drive a spread of deterministic distinct-digit guesses through the UI and
	// check each recorded score against the independent rules reference.
	guesses := []game.Code{
		{0, 1, 2, 3}, {9, 8, 7, 6}, {1, 0, 3, 2},
		{secret[0], 9, 8, 7}, // at least one bull unless secret starts with 9/8/7
		rotate(secret),
	}
	for _, g := range guesses {
		if !distinct(g) {
			continue // keypad forbids repeats; skip malformed fixtures
		}
		before := len(a.gs.Guesses)
		enterAndGuess(t, h, a, g)
		rec := a.gs.Guesses[before].Score
		wb, wc := refScore(secret, g)
		if rec.Bulls != wb || rec.Cows != wc {
			t.Fatalf("guess %v vs secret %v: UI scored %d/%d, rules say %d/%d",
				g, secret, rec.Bulls, rec.Cows, wb, wc)
		}
		if a.gs.Solved { // if a fixture happened to equal the secret, stop
			break
		}
	}
}

func distinct(c game.Code) bool {
	seen := map[int]bool{}
	for _, d := range c {
		if seen[d] {
			return false
		}
		seen[d] = true
	}
	return true
}

// --- RULE: distinct digits; RULE: no submit until complete ------------------

func TestPlayBullsCowsDistinctDigitRule(t *testing.T) {
	h, a := bootToMenu(t)
	startPreset(t, h, a, 1)

	tapDigitKey(t, h, a, 5)
	if len(a.gs.Entry) != 1 {
		t.Fatalf("entry should hold one digit, got %v", a.gs.Entry)
	}
	// The rules forbid repeats: the just-used key must be greyed out (absent from
	// the live keys), and DigitAvailable must report false.
	if a.gs.DigitAvailable(5) {
		t.Fatal("digit 5 still reported available after being placed")
	}
	for _, k := range a.keys {
		if k.Digit == 5 {
			t.Fatal("digit 5 still tappable after being placed (repeat allowed)")
		}
	}

	// You cannot submit a half-typed code: no Gissa button until the entry fills.
	if _, ok := h.FindText("Gissa"); ok {
		t.Fatal("Gissa offered with an incomplete entry")
	}
}

// --- RULE: Sudda erases the last digit --------------------------------------

func TestPlayBullsCowsBackspace(t *testing.T) {
	h, a := bootToMenu(t)
	startPreset(t, h, a, 1)

	tapDigitKey(t, h, a, 1)
	tapDigitKey(t, h, a, 2)
	if len(a.gs.Entry) != 2 {
		t.Fatalf("expected 2 digits, got %v", a.gs.Entry)
	}
	tapButton(t, h, a, "Sudda")
	if len(a.gs.Entry) != 1 || a.gs.Entry[0] != 1 {
		t.Fatalf("Sudda should leave [1], got %v", a.gs.Entry)
	}
	// The erased digit becomes available again.
	if !a.gs.DigitAvailable(2) {
		t.Fatal("digit 2 not available again after erasing it")
	}
}

// --- No loss condition: you can keep guessing forever -----------------------

func TestPlayBullsCowsNoLossManyGuesses(t *testing.T) {
	h, a := bootToMenu(t)
	secret := startPreset(t, h, a, 1)

	// Fire ten wrong-but-legal guesses. Per the rules there is no guess limit, so
	// the game must stay playable and unsolved the whole time.
	wrong := rotate(secret)
	for i := 0; i < 10; i++ {
		enterAndGuess(t, h, a, wrong)
		if a.gs.Solved {
			t.Fatalf("rotated (wrong) guess reported solved on try %d", i)
		}
	}
	if len(a.gs.Guesses) != 10 {
		t.Fatalf("expected 10 recorded guesses, got %d", len(a.gs.Guesses))
	}
	if len(a.keys) == 0 {
		t.Fatal("keypad went dead though the game is not solved")
	}
}

// --- Quit mid-game (Back key AND Meny button) then restart ------------------

func TestPlayBullsCowsQuitMidGame(t *testing.T) {
	h, a := bootToMenu(t)
	startPreset(t, h, a, 1)
	tapDigitKey(t, h, a, 7) // a move in progress

	// Hardware Back abandons the game and returns to the menu.
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}

	// Start again, then quit with the on-screen Meny button instead.
	startPreset(t, h, a, 0)
	tapButton(t, h, a, "Meny")
	if a.screen != screenMenu {
		t.Fatalf("Meny button did not return to menu, screen=%v", a.screen)
	}

	// The menu is fully functional again after quitting.
	if err := h.TapText(game.Presets[2].Name); err != nil {
		t.Fatalf("menu not usable after quitting: %v", err)
	}
	if a.screen != screenGame {
		t.Fatalf("could not restart a game after quitting, screen=%v", a.screen)
	}
}

// --- Replay a finished game -------------------------------------------------

func TestPlayBullsCowsReplayAfterWin(t *testing.T) {
	h, a := bootToMenu(t)
	secret := startPreset(t, h, a, 1)
	enterAndGuess(t, h, a, secret)
	if !a.gs.Solved {
		t.Fatalf("did not win with secret %v", secret)
	}
	tapButton(t, h, a, "Spela igen")
	if a.screen != screenGame || a.gs.Solved {
		t.Fatalf("Spela igen did not start a fresh game (solved=%v)", a.gs.Solved)
	}
	if len(a.gs.Guesses) != 0 || len(a.gs.Entry) != 0 {
		t.Fatalf("replayed game not reset: %d guesses, entry %v", len(a.gs.Guesses), a.gs.Entry)
	}
}

// --- Rules screen: open from the menu, verify content, and return -----------

func TestPlayBullsCowsRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)

	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button on menu: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open the rules screen, screen=%v", a.screen)
	}
	// The rules text must actually explain Bulls and Cows.
	for _, want := range []string{"Bull", "Cow"} {
		if _, ok := h.FindTextContains(want); !ok {
			t.Fatalf("rules screen missing %q; visible: %v", want, texts(h))
		}
	}
	if err := h.TapText("Tillbaka"); err != nil {
		t.Fatalf("no Tillbaka button on rules screen: %v", err)
	}
	if a.screen != screenMenu {
		t.Fatalf("Tillbaka did not return to menu, screen=%v", a.screen)
	}
}

// --- Screenshot of a won board for eyeballing -------------------------------

func TestPlayBullsCowsWinScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}

	// Splash first (before dismissing it).
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if e := h.Screenshot(dir + "/bullscows_splash.png"); e != nil {
		t.Fatalf("splash screenshot: %v", e)
	}

	// Menu.
	h.TapXY(500, 700)
	if a.screen != screenMenu {
		t.Fatalf("tap on splash did not open menu, screen=%v", a.screen)
	}
	_ = h.Screenshot(dir + "/bullscows_menu.png")

	// Rules screen, then back to the menu.
	if err := h.TapText("Regler"); err == nil && a.screen == screenRules {
		_ = h.Screenshot(dir + "/bullscows_rules.png")
		_ = h.TapText("Tillbaka")
	}

	// In-progress board: classic 4-digit preset, one scored (wrong) guess.
	secret := startPreset(t, h, a, 1)
	enterAndGuess(t, h, a, rotate(secret))
	_ = h.Screenshot(dir + "/bullscows_board.png")

	// Won board.
	enterAndGuess(t, h, a, secret)
	if err := h.Screenshot(dir + "/bullscows_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
