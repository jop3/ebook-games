// Package game implements Bagels (Pico-Fermi-Bagels), a code-breaking game,
// with no dependency on the inkview SDK so it can be unit-tested cgo-free.
//
// The host holds a secret code of N distinct digits. Each guess is scored per
// digit and reported as sorted WORDS (never revealing which position matched):
//
//	Fermi  — a digit correct in both value AND position.
//	Pico   — a digit present in the code but in the wrong position.
//	Bagels — printed once when NO digit is in the code at all.
//
// The player wins when every digit is a Fermi. A limited number of guesses is
// allowed; running out is a loss.
package game

import "sort"

// Code is a sequence of digits (each 0-9). Digits are distinct in the classic
// rules; the generator enforces this.
type Code []int

// Score is the raw feedback for one guess.
type Score struct {
	Fermi int // right digit, right place
	Pico  int // right digit, wrong place
}

// Evaluate compares a guess against the secret and returns the fermi/pico
// counts. Both codes must be the same length. A guessed digit contributes to at
// most one fermi or pico, counted via digit frequency so the general case
// (should digits ever repeat) stays correct.
func Evaluate(secret, guess Code) Score {
	var sc Score
	if len(secret) != len(guess) {
		return sc
	}
	var secretFreq [10]int
	var guessFreq [10]int
	for i := range secret {
		if secret[i] == guess[i] {
			sc.Fermi++
		} else {
			secretFreq[secret[i]]++
			guessFreq[guess[i]]++
		}
	}
	// Pico = for each digit, the overlap of the non-fermi occurrences.
	for d := 0; d < 10; d++ {
		if secretFreq[d] < guessFreq[d] {
			sc.Pico += secretFreq[d]
		} else {
			sc.Pico += guessFreq[d]
		}
	}
	return sc
}

// Feedback returns the sorted word feedback for a score. Ordering the words
// (alphabetically: Fermi before Pico) is deliberate so the sequence never hints
// at which position produced which word. When nothing matches at all, the
// single word "Bagels" is returned.
func (s Score) Feedback() []string {
	if s.Fermi == 0 && s.Pico == 0 {
		return []string{"Bagels"}
	}
	words := make([]string, 0, s.Fermi+s.Pico)
	for i := 0; i < s.Fermi; i++ {
		words = append(words, "Fermi")
	}
	for i := 0; i < s.Pico; i++ {
		words = append(words, "Pico")
	}
	sort.Strings(words) // Fermi < Pico, hides positional information
	return words
}

// Solved reports whether a score is a full win for a code of the given length.
func (s Score) Solved(length int) bool {
	return s.Fermi == length
}

// Equal compares two codes.
func Equal(a, b Code) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
