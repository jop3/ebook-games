package game

import (
	"math/rand"
	"time"
)

// Preset defines a difficulty by code length and whether digits may repeat.
type Preset struct {
	Name   string
	Length int
}

// Presets offered on the menu.
var Presets = []Preset{
	{"Lätt – 3 siffror", 3},
	{"Klassisk – 4 siffror", 4},
	{"Svår – 5 siffror", 5},
}

// Guess is a recorded guess and its score.
type Guess struct {
	Code  Code
	Score Score
}

// GameState is a full playable Bulls & Cows game.
type GameState struct {
	Cfg     Preset
	Secret  Code
	Guesses []Guess
	Entry   Code // digits the player is currently typing
	Solved  bool
}

// NewGame starts a game with a random secret.
func NewGame(p Preset) *GameState {
	return NewGameSeeded(p, time.Now().UnixNano())
}

// NewGameSeeded starts a game with a deterministic seed (for tests).
func NewGameSeeded(p Preset, seed int64) *GameState {
	rng := rand.New(rand.NewSource(seed))
	return &GameState{
		Cfg:    p,
		Secret: RandomCode(p.Length, rng),
		Entry:  Code{},
	}
}

// RandomCode returns a code of length distinct digits (0-9). length must be
// <= 10.
func RandomCode(length int, rng *rand.Rand) Code {
	if length > 10 {
		length = 10
	}
	digits := rng.Perm(10) // 0..9 shuffled
	out := make(Code, length)
	copy(out, digits[:length])
	return out
}

// AppendDigit adds a digit to the current entry if there is room and the digit
// is not already present (distinct-digit rule). Returns true if it changed.
func (s *GameState) AppendDigit(d int) bool {
	if s.Solved || len(s.Entry) >= s.Cfg.Length {
		return false
	}
	for _, e := range s.Entry {
		if e == d {
			return false // no repeats
		}
	}
	s.Entry = append(s.Entry, d)
	return true
}

// Backspace removes the last entered digit. Returns true if it changed.
func (s *GameState) Backspace() bool {
	if len(s.Entry) == 0 {
		return false
	}
	s.Entry = s.Entry[:len(s.Entry)-1]
	return true
}

// EntryComplete reports whether the entry is full and ready to submit.
func (s *GameState) EntryComplete() bool {
	return len(s.Entry) == s.Cfg.Length
}

// Submit scores the current entry, records it, and clears the entry. Returns
// true if a guess was submitted.
func (s *GameState) Submit() bool {
	if s.Solved || !s.EntryComplete() {
		return false
	}
	guess := make(Code, len(s.Entry))
	copy(guess, s.Entry)
	sc := Evaluate(s.Secret, guess)
	s.Guesses = append(s.Guesses, Guess{Code: guess, Score: sc})
	s.Entry = s.Entry[:0]
	if sc.Solved(s.Cfg.Length) {
		s.Solved = true
	}
	return true
}

// DigitAvailable reports whether digit d can still be added to the entry
// (used to grey out keys that are already placed).
func (s *GameState) DigitAvailable(d int) bool {
	if len(s.Entry) >= s.Cfg.Length {
		return false
	}
	for _, e := range s.Entry {
		if e == d {
			return false
		}
	}
	return true
}
