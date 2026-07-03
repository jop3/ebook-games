package game

import (
	"math/rand"
	"testing"
)

func TestRoundBasics(t *testing.T) {
	d := newTestDict(t)
	rng := rand.New(rand.NewSource(7))
	r := NewRound(d, rng)
	if len(r.Letters()) != 7 {
		t.Fatalf("expected 7 letters, got %d", len(r.Letters()))
	}
	if !d.IsValidWord(r.BaseWord()) {
		t.Fatalf("base word %q invalid", r.BaseWord())
	}
	if r.Score() != 0 || len(r.Found()) != 0 {
		t.Fatal("new round should start empty")
	}
	if r.TotalSolutions() < 10 {
		t.Fatalf("round should have solutions, got %d", r.TotalSolutions())
	}
}

// Build the base word itself by tapping its letters and submit it: must be
// accepted and scored by its length.
func TestSubmitBaseWord(t *testing.T) {
	d := newTestDict(t)
	rng := rand.New(rand.NewSource(3))
	r := NewRound(d, rng)

	typeWord(t, r, r.BaseWord())
	res, w := r.Submit()
	if res != ResultAccepted {
		t.Fatalf("submitting base word %q gave result %v", r.BaseWord(), res)
	}
	if w != r.BaseWord() {
		t.Fatalf("accepted word %q != base %q", w, r.BaseWord())
	}
	if r.Score() != len([]rune(r.BaseWord())) {
		t.Fatalf("score %d != base length %d", r.Score(), len([]rune(r.BaseWord())))
	}
	if len(r.Found()) != 1 {
		t.Fatalf("expected 1 found word, got %d", len(r.Found()))
	}
	if r.Input() != "" {
		t.Fatal("input should be cleared after accept")
	}
}

// Re-submitting an already-found word must be rejected as a duplicate and not
// score again.
func TestDuplicateRejected(t *testing.T) {
	d := newTestDict(t)
	rng := rand.New(rand.NewSource(3))
	r := NewRound(d, rng)

	typeWord(t, r, r.BaseWord())
	r.Submit()
	scoreAfterFirst := r.Score()

	typeWord(t, r, r.BaseWord())
	res, _ := r.Submit()
	if res != ResultAlreadyFound {
		t.Fatalf("expected ResultAlreadyFound, got %v", res)
	}
	if r.Score() != scoreAfterFirst {
		t.Fatal("duplicate must not change the score")
	}
}

func TestTooShortRejected(t *testing.T) {
	d := newTestDict(t)
	rng := rand.New(rand.NewSource(5))
	r := NewRound(d, rng)
	// tap first two letters only
	r.Tap(indexOfUnused(r))
	r.Tap(indexOfUnused(r))
	res, _ := r.Submit()
	if res != ResultTooShort {
		t.Fatalf("expected ResultTooShort, got %v", res)
	}
}

func TestClearAndBackspace(t *testing.T) {
	d := newTestDict(t)
	rng := rand.New(rand.NewSource(9))
	r := NewRound(d, rng)
	r.Tap(0)
	r.Tap(1)
	r.Tap(2)
	if len([]rune(r.Input())) != 3 {
		t.Fatal("expected 3 input letters")
	}
	r.Backspace()
	if len([]rune(r.Input())) != 2 {
		t.Fatal("backspace should remove one letter")
	}
	// slot 2 should now be free again
	if r.LetterUsed(2) {
		t.Fatal("slot 2 should be free after backspace")
	}
	r.Clear()
	if r.Input() != "" {
		t.Fatal("clear should empty input")
	}
	for i := range r.Letters() {
		if r.LetterUsed(i) {
			t.Fatalf("slot %d still marked used after clear", i)
		}
	}
}

func TestCannotReuseLetterSlot(t *testing.T) {
	d := newTestDict(t)
	rng := rand.New(rand.NewSource(11))
	r := NewRound(d, rng)
	r.Tap(0)
	before := r.Input()
	r.Tap(0) // same slot again -> ignored
	if r.Input() != before {
		t.Fatal("tapping the same slot twice must be ignored")
	}
}

func TestShuffleKeepsSameMultiset(t *testing.T) {
	d := newTestDict(t)
	rng := rand.New(rand.NewSource(13))
	r := NewRound(d, rng)
	before := runeHist(r.Letters())
	r.Shuffle()
	after := runeHist(r.Letters())
	if !histEqual(before, after) {
		t.Fatal("shuffle must preserve the letter multiset")
	}
}

func TestMissedWords(t *testing.T) {
	d := newTestDict(t)
	rng := rand.New(rand.NewSource(3))
	r := NewRound(d, rng)
	// find just the base word
	typeWord(t, r, r.BaseWord())
	r.Submit()
	missed := r.MissedWords(5)
	if len(missed) == 0 {
		t.Fatal("expected some missed words")
	}
	for _, m := range missed {
		if m == r.BaseWord() {
			t.Fatal("found word should not appear in missed list")
		}
	}
}

// --- helpers ---

// typeWord taps letter slots to spell out word using available letters.
func typeWord(t *testing.T, r *Round, word string) {
	t.Helper()
	r.Clear()
	for _, want := range word {
		found := false
		for i, ch := range r.Letters() {
			if r.LetterUsed(i) {
				continue
			}
			if ch == want {
				r.Tap(i)
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("cannot type %q: letter %c unavailable", word, want)
		}
	}
}

func indexOfUnused(r *Round) int {
	for i := range r.Letters() {
		if !r.LetterUsed(i) {
			return i
		}
	}
	return -1
}

func runeHist(rs []rune) map[rune]int {
	m := map[rune]int{}
	for _, r := range rs {
		m[r]++
	}
	return m
}

func histEqual(a, b map[rune]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
