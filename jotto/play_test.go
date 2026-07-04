//go:build playtest

package main

// Headless PLAYTHROUGH tests for Jotto (a Wordle-style Swedish word game). They
// drive the real touch path and check the gameplay against the rules as written
// (see rulesParagraphs in main.go): guess the hidden five-letter word in at most
// six tries; each guess must be a real dictionary word; tiles score correct
// (right place), present (wrong place) or absent, with Wordle-style duplicate
// handling. Covers win, loss, feedback correctness (vs an independent scorer),
// invalid-word rejection, and quit/replay/rules. Runs under the pure-Go inkview
// emulator (playtest/play.sh).

import (
	"math/rand"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"jotto/game"
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

func startGame(t *testing.T, h *ink.Harness, a *app) string {
	t.Helper()
	h.TapRect(a.menu.PlayButton())
	if a.screen != screenGame || a.gs == nil {
		t.Fatalf("did not enter game, screen=%v", a.screen)
	}
	return a.gs.Secret()
}

func typeWord(t *testing.T, h *ink.Harness, a *app, word string) {
	t.Helper()
	for _, r := range word {
		found := false
		for _, k := range a.keys {
			if k.Letter == r {
				h.TapRect(k.Rect)
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("no key for %q", string(r))
		}
	}
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

func guessWord(t *testing.T, h *ink.Harness, a *app, word string) {
	t.Helper()
	typeWord(t, h, a, word)
	tapButton(t, h, a, "Gissa")
}

// refStatuses scores a guess Wordle-style, independently of game.Evaluate.
func refStatuses(guess, secret string) []game.Status {
	g, s := []rune(guess), []rune(secret)
	res := make([]game.Status, len(g))
	used := make([]bool, len(s))
	for i := range g {
		if i < len(s) && g[i] == s[i] {
			res[i] = game.Correct
			used[i] = true
		}
	}
	for i := range g {
		if res[i] == game.Correct {
			continue
		}
		for j := range s {
			if !used[j] && g[i] == s[j] {
				res[i] = game.Present
				used[j] = true
				break
			}
		}
	}
	return res
}

// validWordsExcept returns n distinct dictionary words that are not `secret`.
func validWordsExcept(secret string, n int) []string {
	rng := rand.New(rand.NewSource(12345))
	seen := map[string]bool{secret: true}
	var out []string
	for len(out) < n {
		w := game.PickSecret(rng)
		if seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- WIN: guess the secret; all tiles score correct -------------------------

func TestPlayJottoWin(t *testing.T) {
	h, a := bootToMenu(t)
	secret := startGame(t, h, a)

	guessWord(t, h, a, secret)
	if !a.gs.Won || !a.gs.Over {
		t.Fatalf("guessing the secret %q did not win (won=%v)", secret, a.gs.Won)
	}
	last := a.gs.Guesses[len(a.gs.Guesses)-1]
	if !game.AllCorrect(last.Statuses) {
		t.Fatalf("winning guess not all-correct: %v", last.Statuses)
	}
	if _, ok := h.FindTextContains("Rätt"); !ok {
		t.Fatalf("win banner missing; visible: %v", texts(h))
	}
}

// --- Feedback matches the rules (Wordle duplicate handling) -----------------

func TestPlayJottoFeedbackMatchesRules(t *testing.T) {
	h, a := bootToMenu(t)
	secret := startGame(t, h, a)

	for _, w := range validWordsExcept(secret, 3) {
		before := len(a.gs.Guesses)
		guessWord(t, h, a, w)
		if len(a.gs.Guesses) != before+1 {
			t.Fatalf("valid word %q was not accepted", w)
		}
		got := a.gs.Guesses[before].Statuses
		want := refStatuses(w, secret)
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("guess %q vs %q: tile %d is %v, rules say %v", w, secret, i, got[i], want[i])
			}
		}
		if a.gs.Over {
			break
		}
	}
}

// --- LOSS after six wrong guesses -------------------------------------------

func TestPlayJottoLoss(t *testing.T) {
	h, a := bootToMenu(t)
	secret := startGame(t, h, a)

	for _, w := range validWordsExcept(secret, game.MaxGuesses) {
		if a.gs.Over {
			t.Fatal("game ended before all guesses were used")
		}
		guessWord(t, h, a, w)
	}
	if a.gs.Won || !a.gs.Over {
		t.Fatalf("expected a loss (won=%v over=%v)", a.gs.Won, a.gs.Over)
	}
	if _, ok := h.FindTextContains("Facit"); !ok {
		t.Fatalf("loss banner (reveal) missing; visible: %v", texts(h))
	}
}

// --- RULE: a non-dictionary guess is rejected -------------------------------

func TestPlayJottoInvalidWordRejected(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)

	// "xxxxx" is not a Swedish word.
	if game.IsWord("xxxxx") {
		t.Skip("dictionary unexpectedly contains xxxxx")
	}
	guessWord(t, h, a, "xxxxx")
	if len(a.gs.Guesses) != 0 {
		t.Fatal("a non-word guess was recorded")
	}
	if _, ok := h.FindTextContains("giltigt"); !ok {
		t.Fatalf("no 'invalid word' message; visible: %v", texts(h))
	}
}

// --- Backspace / replay / quit / rules --------------------------------------

func TestPlayJottoBackspaceReplayQuitRules(t *testing.T) {
	h, a := bootToMenu(t)
	secret := startGame(t, h, a)

	// Type two letters, backspace one.
	typeWord(t, h, a, string([]rune(secret)[:2]))
	if len(a.gs.Entry) != 2 {
		t.Fatalf("expected 2 letters entered, got %d", len(a.gs.Entry))
	}
	tapButton(t, h, a, "Sudda")
	if len(a.gs.Entry) != 1 {
		t.Fatalf("Sudda should leave 1 letter, got %d", len(a.gs.Entry))
	}

	// Win, then "Nytt spel" restarts.
	tapButton(t, h, a, "Sudda") // clear the stray letter
	guessWord(t, h, a, secret)
	if !a.gs.Won {
		t.Fatal("setup win failed")
	}
	tapButton(t, h, a, "Nytt spel")
	if a.gs.Over || len(a.gs.Guesses) != 0 {
		t.Fatalf("Nytt spel did not reset (over=%v guesses=%d)", a.gs.Over, len(a.gs.Guesses))
	}

	tapButton(t, h, a, "Meny")
	if a.screen != screenMenu {
		t.Fatalf("Meny did not return to menu, screen=%v", a.screen)
	}

	h.TapRect(a.menu.RulesButton())
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("fembokstavsordet"); !ok {
		t.Fatalf("rules text missing; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave rules, screen=%v", a.screen)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayJottoScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	h, a := bootToMenu(t)
	secret := startGame(t, h, a)
	for _, w := range validWordsExcept(secret, 2) {
		guessWord(t, h, a, w)
	}
	guessWord(t, h, a, secret)
	if err := h.Screenshot(dir + "/jotto_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
