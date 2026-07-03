package game

import (
	"math/rand"
	"testing"
)

func newTestDict(t *testing.T) *Dictionary {
	t.Helper()
	d := NewDictionary()
	if d.Size() < 20000 {
		t.Fatalf("dictionary too small: %d words", d.Size())
	}
	if d.BaseWordCount() < 100 {
		t.Fatalf("too few base words: %d", d.BaseWordCount())
	}
	return d
}

func TestDictionaryLoads(t *testing.T) {
	d := newTestDict(t)
	t.Logf("dictionary: %d words, %d base words", d.Size(), d.BaseWordCount())
	// A few known common Swedish words must be present.
	for _, w := range []string{"katt", "hund", "sol", "bok", "äta", "öga", "häst"} {
		if !d.IsValidWord(w) {
			t.Errorf("expected %q to be a valid word", w)
		}
	}
	// Junk must be rejected.
	for _, w := range []string{"", "x", "qwzx", "123", "hello!"} {
		if d.IsValidWord(w) {
			t.Errorf("did not expect %q to be valid", w)
		}
	}
}

func TestIsValidWordCaseInsensitive(t *testing.T) {
	d := newTestDict(t)
	if !d.IsValidWord("KATT") || !d.IsValidWord("  Katt ") {
		t.Error("IsValidWord should be case/space insensitive")
	}
}

func TestCanFormMultiset(t *testing.T) {
	avail := letterCount("banan") // b,a,a,a,n,n  -> a:3 wait: banan = b a n a n => a:2, n:2, b:1
	// recompute: b,a,n,a,n
	// Word using two a's and two n's is allowed:
	if !CanForm(avail, "annan") {
		// annan = a,n,n,a,n needs n:3 -> should be REJECTED actually
	}
	// Use precise expectations:
	av := letterCount("test") // t:2, e:1, s:1
	if !CanForm(av, "test") {
		t.Error("test should be formable from its own letters")
	}
	if !CanForm(av, "set") {
		t.Error("set formable from test")
	}
	if CanForm(av, "tests") { // needs s:2, only s:1
		t.Error("tests must NOT be formable (needs two s)")
	}
	if CanForm(av, "teee") { // needs e:3, only e:1
		t.Error("teee must NOT be formable (too many e)")
	}
	if CanForm(av, "tex") { // x not available
		t.Error("tex must NOT be formable (x missing)")
	}
}

// Every curated base word must itself be a valid dictionary word.
func TestBaseWordsAreValid(t *testing.T) {
	d := newTestDict(t)
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 200; i++ {
		base := d.PickBaseWord(rng)
		if base == "" {
			t.Fatal("empty base word")
		}
		if len([]rune(base)) != 7 {
			t.Errorf("base word %q is not 7 letters", base)
		}
		if !d.IsValidWord(base) {
			t.Errorf("base word %q is not a valid dictionary word", base)
		}
	}
}

// Every curated base word must yield a solid number of solutions so rounds are
// playable.
func TestBaseWordsAreSolvable(t *testing.T) {
	d := newTestDict(t)
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 30; i++ {
		base := d.PickBaseWord(rng)
		sols := d.AllSolutions(base, MinWordLen)
		if len(sols) < 10 {
			t.Errorf("base word %q has too few solutions: %d", base, len(sols))
		}
		// Every solution must be formable from the base and be a valid word.
		avail := letterCount(base)
		for _, s := range sols {
			if !CanForm(avail, s) {
				t.Errorf("solution %q not formable from base %q", s, base)
			}
			if !d.IsValidWord(s) {
				t.Errorf("solution %q not a valid word", s)
			}
		}
	}
}
