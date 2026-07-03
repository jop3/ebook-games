package game

import (
	"math/rand"
	"strings"
)

// MaxGuesses is the number of attempts the player has.
const MaxGuesses = 6

// alphabet is the on-screen keyboard order: the Swedish alphabet A-Ö.
var alphabet = []rune("abcdefghijklmnopqrstuvwxyzåäö")

// Alphabet returns the Swedish letters used by the keyboard, in order.
func Alphabet() []rune {
	out := make([]rune, len(alphabet))
	copy(out, alphabet)
	return out
}

// Guess is a submitted guess together with its per-position feedback.
type Guess struct {
	Word    string
	Statuses []Status
}

// GameState holds a single Jotto game in progress.
type GameState struct {
	secret  string
	Guesses []Guess
	Entry   []rune // current in-progress guess (0..WordLen runes)
	Won     bool
	Over    bool // won or out of guesses

	// letterBest tracks the best status seen for each letter across all
	// submitted guesses, used to grey out / mark keyboard keys.
	letterBest map[rune]Status
	letterSeen map[rune]bool
}

// NewGame starts a new game with a secret chosen randomly from the dictionary
// using r (inject for deterministic tests).
func NewGame(r *rand.Rand) *GameState {
	return NewGameWithSecret(PickSecret(r))
}

// NewGameWithSecret starts a game with an explicit secret (used in tests).
func NewGameWithSecret(secret string) *GameState {
	return &GameState{
		secret:     strings.ToLower(secret),
		Entry:      make([]rune, 0, WordLen),
		letterBest: make(map[rune]Status),
		letterSeen: make(map[rune]bool),
	}
}

// Secret returns the secret word (exposed for the reveal-on-loss UI).
func (g *GameState) Secret() string { return g.secret }

// EntryString returns the current entry as a string.
func (g *GameState) EntryString() string { return string(g.Entry) }

// EntryComplete reports whether the entry has a full WordLen runes.
func (g *GameState) EntryComplete() bool { return len(g.Entry) == WordLen }

// GuessesLeft returns how many guesses remain.
func (g *GameState) GuessesLeft() int { return MaxGuesses - len(g.Guesses) }

// AppendLetter adds a letter to the current entry if there is room and the game
// is still active. Returns true if the entry changed.
func (g *GameState) AppendLetter(r rune) bool {
	if g.Over || len(g.Entry) >= WordLen {
		return false
	}
	g.Entry = append(g.Entry, r)
	return true
}

// Backspace removes the last entered letter. Returns true if it changed.
func (g *GameState) Backspace() bool {
	if g.Over || len(g.Entry) == 0 {
		return false
	}
	g.Entry = g.Entry[:len(g.Entry)-1]
	return true
}

// SubmitResult describes the outcome of a submit attempt.
type SubmitResult int

const (
	SubmitOK          SubmitResult = iota // accepted and scored
	SubmitIncomplete                      // entry not yet WordLen letters
	SubmitNotWord                         // entry is not in the dictionary
	SubmitGameOver                        // game already finished
)

// Submit validates and scores the current entry. On success the entry is
// cleared, the guess recorded, keyboard hints updated, and win/over flags set.
func (g *GameState) Submit() SubmitResult {
	if g.Over {
		return SubmitGameOver
	}
	if len(g.Entry) != WordLen {
		return SubmitIncomplete
	}
	word := string(g.Entry)
	if !IsWord(word) {
		return SubmitNotWord
	}
	st := Evaluate(word, g.secret)
	g.Guesses = append(g.Guesses, Guess{Word: word, Statuses: st})
	g.updateLetterHints(g.Entry, st)
	g.Entry = g.Entry[:0]

	if AllCorrect(st) {
		g.Won = true
		g.Over = true
	} else if len(g.Guesses) >= MaxGuesses {
		g.Over = true
	}
	return SubmitOK
}

// updateLetterHints records, per letter, the best feedback status seen so far.
func (g *GameState) updateLetterHints(entry []rune, st []Status) {
	for i, r := range entry {
		g.letterSeen[r] = true
		if !g.has(r) || st[i] > g.letterBest[r] {
			g.letterBest[r] = st[i]
		}
	}
}

func (g *GameState) has(r rune) bool {
	_, ok := g.letterBest[r]
	return ok
}

// LetterStatus returns the best-known feedback for a keyboard letter and
// whether it has been guessed at all. Unguessed letters return (Absent, false).
func (g *GameState) LetterStatus(r rune) (Status, bool) {
	if !g.letterSeen[r] {
		return Absent, false
	}
	return g.letterBest[r], true
}
