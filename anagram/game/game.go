// Package game holds the pure, ink-free logic for the Swedish anagram word
// game "Ordskrav". It embeds a Swedish word list, picks a 7-letter base word
// per round, and validates player words against the letter multiset. It has no
// cgo / inkview dependency so it unit-tests on any machine.
package game

import (
	_ "embed"
	"math/rand"
	"sort"
	"strings"
)

//go:embed words.txt
var wordsData string

//go:embed basewords.txt
var baseWordsData string

// Dictionary holds the valid Swedish words (lowercase, 2-9 letters) and the
// curated pool of base words. Build one with NewDictionary().
type Dictionary struct {
	valid     map[string]bool // all valid words for lookup
	baseWords []string        // 7-letter words known to yield many sub-words
}

// NewDictionary parses the embedded word data. It is cheap enough to call once
// at start-up.
func NewDictionary() *Dictionary {
	d := &Dictionary{valid: make(map[string]bool, 128*1024)}
	for _, w := range strings.Split(wordsData, "\n") {
		w = strings.TrimSpace(w)
		if w != "" {
			d.valid[w] = true
		}
	}
	for _, w := range strings.Split(baseWordsData, "\n") {
		w = strings.TrimSpace(w)
		if w != "" {
			d.baseWords = append(d.baseWords, w)
		}
	}
	return d
}

// Size reports how many valid words are loaded.
func (d *Dictionary) Size() int { return len(d.valid) }

// BaseWordCount reports how many candidate base words are available.
func (d *Dictionary) BaseWordCount() int { return len(d.baseWords) }

// IsValidWord reports whether w is in the dictionary (case-insensitive).
func (d *Dictionary) IsValidWord(w string) bool {
	return d.valid[strings.ToLower(strings.TrimSpace(w))]
}

// letterCount returns the multiset of runes in s.
func letterCount(s string) map[rune]int {
	m := make(map[rune]int, len(s))
	for _, r := range s {
		m[r]++
	}
	return m
}

// CanForm reports whether target can be built from the letters of the multiset
// available (each letter used no more times than it appears in available).
func CanForm(available map[rune]int, target string) bool {
	need := make(map[rune]int, len(target))
	for _, r := range target {
		need[r]++
		if need[r] > available[r] {
			return false
		}
	}
	return true
}

// AllSolutions returns every valid dictionary word (length >= minLen) that can
// be formed from the base word's letters, sorted by length then alphabetically.
func (d *Dictionary) AllSolutions(base string, minLen int) []string {
	avail := letterCount(base)
	var out []string
	for w := range d.valid {
		if len([]rune(w)) < minLen {
			continue
		}
		if CanForm(avail, w) {
			out = append(out, w)
		}
	}
	sortWords(out)
	return out
}

func sortWords(ws []string) {
	sort.Slice(ws, func(i, j int) bool {
		li, lj := len([]rune(ws[i])), len([]rune(ws[j]))
		if li != lj {
			return li < lj
		}
		return ws[i] < ws[j]
	})
}

// PickBaseWord chooses a random base word from the curated pool using the
// supplied rand source (inject one for deterministic tests). It returns the
// base word in lowercase.
func (d *Dictionary) PickBaseWord(rng *rand.Rand) string {
	if len(d.baseWords) == 0 {
		return ""
	}
	return d.baseWords[rng.Intn(len(d.baseWords))]
}
