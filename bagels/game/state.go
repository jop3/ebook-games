package game

import (
	"math/rand"
	"time"
)

// Preset defines a difficulty by code length and the guess allowance.
type Preset struct {
	Name    string
	Length  int
	MaxTurn int // guesses allowed before the game is lost
}

// Presets offered on the menu.
var Presets = []Preset{
	{"Lätt – 3 siffror", 3, 10},
	{"Klassisk – 4 siffror", 4, 12},
	{"Svår – 5 siffror", 5, 14},
}

// Guess is a recorded guess together with its score.
type Guess struct {
	Code  Code
	Score Score
}

// Feedback returns the sorted word feedback for this guess.
func (g Guess) Feedback() []string { return g.Score.Feedback() }

// GameState is a full playable Bagels game.
type GameState struct {
	Cfg     Preset
	Secret  Code
	Guesses []Guess
	Entry   Code // digits the player is currently typing
	Solved  bool
	Lost    bool
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

// Over reports whether the game has ended (won or lost).
func (s *GameState) Over() bool { return s.Solved || s.Lost }

// TurnsLeft returns how many guesses remain.
func (s *GameState) TurnsLeft() int {
	left := s.Cfg.MaxTurn - len(s.Guesses)
	if left < 0 {
		left = 0
	}
	return left
}

// AppendDigit adds a digit to the current entry if there is room and the digit
// is not already present (distinct-digit rule). Returns true if it changed.
func (s *GameState) AppendDigit(d int) bool {
	if s.Over() || len(s.Entry) >= s.Cfg.Length {
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
// true if a guess was submitted. Winning sets Solved; using the last allowed
// guess without winning sets Lost.
func (s *GameState) Submit() bool {
	if s.Over() || !s.EntryComplete() {
		return false
	}
	guess := make(Code, len(s.Entry))
	copy(guess, s.Entry)
	sc := Evaluate(s.Secret, guess)
	s.Guesses = append(s.Guesses, Guess{Code: guess, Score: sc})
	s.Entry = s.Entry[:0]
	if sc.Solved(s.Cfg.Length) {
		s.Solved = true
	} else if len(s.Guesses) >= s.Cfg.MaxTurn {
		s.Lost = true
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
