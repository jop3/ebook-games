package game

import (
	_ "embed"
	"math/rand"
	"strings"
)

// WordLen is the fixed length of every secret and guess.
const WordLen = 5

//go:embed words.txt
var rawWords string

// words is the loaded, validated dictionary (lowercase, exactly WordLen runes,
// only a-ö). It is used both to pick secrets and to validate guesses.
var words []string

// wordSet mirrors words for O(1) validity checks. Keyed by the lowercase word.
var wordSet map[string]struct{}

func init() {
	words = make([]string, 0, 8192)
	wordSet = make(map[string]struct{}, 8192)
	for _, line := range strings.Split(rawWords, "\n") {
		w := strings.TrimSpace(strings.ToLower(line))
		if !isValidShape(w) {
			continue
		}
		if _, seen := wordSet[w]; seen {
			continue
		}
		wordSet[w] = struct{}{}
		words = append(words, w)
	}
}

// isValidShape reports whether w is exactly WordLen runes and only contains the
// Swedish lowercase letters a-z plus å, ä, ö.
func isValidShape(w string) bool {
	n := 0
	for _, r := range w {
		n++
		if n > WordLen {
			return false
		}
		if (r >= 'a' && r <= 'z') || r == 'å' || r == 'ä' || r == 'ö' {
			continue
		}
		return false
	}
	return n == WordLen
}

// WordCount returns the number of words in the loaded dictionary.
func WordCount() int { return len(words) }

// IsWord reports whether guess is a valid dictionary word. The guess is
// compared case-insensitively.
func IsWord(guess string) bool {
	_, ok := wordSet[strings.ToLower(guess)]
	return ok
}

// PickSecret returns a random word from the dictionary using r. Injecting r
// makes secret selection deterministic in tests. If the dictionary is empty it
// returns the empty string.
func PickSecret(r *rand.Rand) string {
	if len(words) == 0 {
		return ""
	}
	return words[r.Intn(len(words))]
}
