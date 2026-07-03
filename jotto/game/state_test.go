package game

import (
	"math/rand"
	"testing"
)

func TestWordlistLoaded(t *testing.T) {
	if WordCount() < 800 {
		t.Fatalf("dictionary too small: %d words", WordCount())
	}
	// Every loaded word must be exactly WordLen runes.
	for _, w := range words {
		if n := len([]rune(w)); n != WordLen {
			t.Fatalf("word %q has %d runes, want %d", w, n, WordLen)
		}
	}
}

func TestIsWord(t *testing.T) {
	// Pick a real word from the list and confirm membership.
	sample := words[0]
	if !IsWord(sample) {
		t.Errorf("IsWord(%q) = false, want true", sample)
	}
	if !IsWord(upper(sample)) {
		t.Errorf("IsWord should be case-insensitive for %q", sample)
	}
	if IsWord("zzzzz") {
		t.Errorf("IsWord(zzzzz) = true, want false")
	}
	if IsWord("abc") {
		t.Errorf("IsWord(abc) = true, want false (wrong length)")
	}
}

func upper(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'a' && r <= 'z' {
			out[i] = r - 32
		}
	}
	return string(out)
}

func TestPickSecretDeterministic(t *testing.T) {
	r1 := rand.New(rand.NewSource(42))
	r2 := rand.New(rand.NewSource(42))
	a := PickSecret(r1)
	b := PickSecret(r2)
	if a != b {
		t.Errorf("same seed gave different secrets: %q vs %q", a, b)
	}
	if !IsWord(a) {
		t.Errorf("secret %q is not a dictionary word", a)
	}
}

func TestGameFlowWin(t *testing.T) {
	const secret = "spela" // a real Swedish dictionary word
	if !IsWord(secret) {
		t.Fatalf("test secret %q missing from dictionary", secret)
	}
	g := NewGameWithSecret(secret)
	// Type a wrong (but valid-shaped) word not in dict -> rejected.
	for _, r := range "zzzzz" {
		g.AppendLetter(r)
	}
	if res := g.Submit(); res != SubmitNotWord {
		t.Fatalf("expected SubmitNotWord, got %v", res)
	}
	// Clear the bad entry.
	for len(g.Entry) > 0 {
		g.Backspace()
	}
	// Now guess the secret exactly.
	for _, r := range secret {
		g.AppendLetter(r)
	}
	if res := g.Submit(); res != SubmitOK {
		t.Fatalf("expected SubmitOK, got %v", res)
	}
	if !g.Won || !g.Over {
		t.Fatalf("expected win: Won=%v Over=%v", g.Won, g.Over)
	}
}

func TestGameFlowLose(t *testing.T) {
	// Use a real dictionary word as the secret and a different real word as a
	// repeated wrong guess so MaxGuesses is exhausted.
	secret := words[0]
	wrong := ""
	for _, w := range words {
		if w != secret {
			wrong = w
			break
		}
	}
	g := NewGameWithSecret(secret)
	for i := 0; i < MaxGuesses; i++ {
		for len(g.Entry) > 0 {
			g.Backspace()
		}
		for _, r := range wrong {
			g.AppendLetter(r)
		}
		g.Submit()
	}
	if !g.Over || g.Won {
		t.Fatalf("expected loss after %d guesses: Over=%v Won=%v", MaxGuesses, g.Over, g.Won)
	}
	if g.GuessesLeft() != 0 {
		t.Fatalf("expected 0 guesses left, got %d", g.GuessesLeft())
	}
}

func TestLetterHints(t *testing.T) {
	secret := "spela"
	if !IsWord(secret) {
		t.Fatalf("secret %q missing from dictionary", secret)
	}
	g := NewGameWithSecret(secret)
	// Guess a valid word sharing letters. Find any dict word to guess.
	guess := ""
	for _, w := range words {
		if w != secret {
			guess = w
			break
		}
	}
	for _, r := range guess {
		g.AppendLetter(r)
	}
	g.Submit()
	// Every letter of the guess should now be "seen".
	for _, r := range guess {
		if _, seen := g.LetterStatus(r); !seen {
			t.Errorf("letter %q should be seen after guess", string(r))
		}
	}
	// A letter never guessed should be unseen.
	if _, seen := g.LetterStatus('q'); seen && !contains(guess, 'q') {
		t.Errorf("letter q should be unseen")
	}
}

func contains(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
