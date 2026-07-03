// Package game implements Bulls and Cows, a symmetric digit-deduction game,
// with no dependency on the inkview SDK so it can be unit-tested cgo-free.
//
// The host holds a secret code of N distinct digits. Each guess scores:
//   Bulls — digits correct in both value AND position.
//   Cows  — digits present in the code but in the wrong position.
// The player wins when all N digits are bulls. Fewer guesses is better.
package game

// Code is a sequence of digits (each 0-9). Digits are distinct in the classic
// rules; the generator enforces this.
type Code []int

// Score is the feedback for one guess.
type Score struct {
	Bulls int
	Cows  int
}

// Evaluate compares a guess against the secret and returns bulls and cows.
// Both codes must be the same length. It correctly handles the general case
// even if digits were to repeat: a guessed digit contributes to at most one
// bull or cow, counted via digit frequency (this is the classic trap).
func Evaluate(secret, guess Code) Score {
	var sc Score
	if len(secret) != len(guess) {
		return sc
	}
	var secretFreq [10]int
	var guessFreq [10]int
	for i := range secret {
		if secret[i] == guess[i] {
			sc.Bulls++
		} else {
			secretFreq[secret[i]]++
			guessFreq[guess[i]]++
		}
	}
	// Cows = for each digit, the overlap of the non-bull occurrences.
	for d := 0; d < 10; d++ {
		if secretFreq[d] < guessFreq[d] {
			sc.Cows += secretFreq[d]
		} else {
			sc.Cows += guessFreq[d]
		}
	}
	return sc
}

// Solved reports whether a score is a full win for a code of the given length.
func (s Score) Solved(length int) bool {
	return s.Bulls == length
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
