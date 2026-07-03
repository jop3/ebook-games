package game

import (
	"math/rand"
	"strings"
)

// MinWordLen is the shortest word the player may submit.
const MinWordLen = 3

// SubmitResult describes the outcome of submitting a word.
type SubmitResult int

const (
	// ResultAccepted: valid new word, scored and added to the found list.
	ResultAccepted SubmitResult = iota
	// ResultTooShort: word shorter than MinWordLen.
	ResultTooShort
	// ResultNotFormable: uses letters (or counts) not available in the round.
	ResultNotFormable
	// ResultNotAWord: formable but not in the dictionary.
	ResultNotAWord
	// ResultAlreadyFound: valid but the player already found it.
	ResultNotFormableEmpty // empty input
	ResultAlreadyFound
)

// Round is a single game round: a base word, its shuffled letters, the letters
// the player has picked into the input row, the found words and the score.
type Round struct {
	dict      *Dictionary
	baseWord  string
	letters   []rune       // the shuffled display letters (the base word's runes)
	avail     map[rune]int // multiset of letters for validation
	input     []int        // indices into letters currently selected, in order
	used      []bool       // which letter slots are consumed by the input
	found     []string     // accepted words, in the order found
	foundSet  map[string]bool
	score     int
	rng       *rand.Rand
	totalWord int // total number of solutions available (target hint)
}

// NewRound builds a round from the dictionary using rng for base-word choice
// and letter shuffling. Injecting rng makes rounds deterministic in tests.
func NewRound(d *Dictionary, rng *rand.Rand) *Round {
	base := d.PickBaseWord(rng)
	r := &Round{
		dict:     d,
		baseWord: base,
		avail:    letterCount(base),
		foundSet: make(map[string]bool),
		rng:      rng,
	}
	r.letters = []rune(base)
	r.used = make([]bool, len(r.letters))
	r.Shuffle()
	r.totalWord = len(d.AllSolutions(base, MinWordLen))
	return r
}

// BaseWord returns the round's hidden base word (lowercase).
func (r *Round) BaseWord() string { return r.baseWord }

// Letters returns the current (shuffled) display letters.
func (r *Round) Letters() []rune { return r.letters }

// LetterUsed reports whether the letter slot i is currently in the input row.
func (r *Round) LetterUsed(i int) bool {
	if i < 0 || i >= len(r.used) {
		return false
	}
	return r.used[i]
}

// Input returns the current input word being assembled.
func (r *Round) Input() string {
	var b strings.Builder
	for _, idx := range r.input {
		b.WriteRune(r.letters[idx])
	}
	return b.String()
}

// Found returns the accepted words in order found.
func (r *Round) Found() []string { return r.found }

// Score returns the current score (sum of found word lengths).
func (r *Round) Score() int { return r.score }

// TotalSolutions returns how many valid words exist for this round.
func (r *Round) TotalSolutions() int { return r.totalWord }

// Tap picks the letter slot i into the input row (if not already used).
func (r *Round) Tap(i int) {
	if i < 0 || i >= len(r.letters) || r.used[i] {
		return
	}
	r.used[i] = true
	r.input = append(r.input, i)
}

// Backspace removes the last picked letter from the input row.
func (r *Round) Backspace() {
	if len(r.input) == 0 {
		return
	}
	last := r.input[len(r.input)-1]
	r.used[last] = false
	r.input = r.input[:len(r.input)-1]
}

// Clear empties the input row, freeing all letters.
func (r *Round) Clear() {
	for _, idx := range r.input {
		r.used[idx] = false
	}
	r.input = r.input[:0]
}

// Shuffle randomises the order of the display letters. Any in-progress input is
// cleared because the slot indices change.
func (r *Round) Shuffle() {
	r.Clear()
	r.rng.Shuffle(len(r.letters), func(i, j int) {
		r.letters[i], r.letters[j] = r.letters[j], r.letters[i]
	})
}

// Submit validates the current input word, and on success scores it, records it
// and clears the input. It returns the outcome and (on acceptance) the word.
func (r *Round) Submit() (SubmitResult, string) {
	word := r.Input()
	if word == "" {
		return ResultNotFormableEmpty, ""
	}
	if len([]rune(word)) < MinWordLen {
		return ResultTooShort, word
	}
	// The input is by construction formable from the letters (it is built from
	// them), but validate defensively against the multiset anyway.
	if !CanForm(r.avail, word) {
		return ResultNotFormable, word
	}
	if r.foundSet[word] {
		return ResultAlreadyFound, word
	}
	if !r.dict.IsValidWord(word) {
		return ResultNotAWord, word
	}
	r.foundSet[word] = true
	r.found = append(r.found, word)
	r.score += len([]rune(word))
	r.Clear()
	return ResultAccepted, word
}

// MissedWords returns up to n valid solutions the player has NOT found yet,
// preferring longer words (useful for a "give up" reveal).
func (r *Round) MissedWords(n int) []string {
	all := r.dict.AllSolutions(r.baseWord, MinWordLen)
	// longest first
	sortWordsLongFirst(all)
	var out []string
	for _, w := range all {
		if r.foundSet[w] {
			continue
		}
		out = append(out, w)
		if len(out) >= n {
			break
		}
	}
	return out
}

func sortWordsLongFirst(ws []string) {
	// simple insertion by length desc then alpha
	for i := 1; i < len(ws); i++ {
		for j := i; j > 0; j-- {
			a, b := ws[j-1], ws[j]
			la, lb := len([]rune(a)), len([]rune(b))
			swap := la < lb || (la == lb && a > b)
			if !swap {
				break
			}
			ws[j-1], ws[j] = ws[j], ws[j-1]
		}
	}
}
