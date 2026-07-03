package main

import "math/rand"

// Color is a peg color index in the range 0..Colors-1.
type Color int

// Secret is the hidden sequence the player tries to guess. Length = Pegs.
type Secret []Color

// Guess is a player guess. Length = Pegs.
type Guess []Color

// Feedback is the response to a guess.
type Feedback struct {
	Black int // right color, right place
	White int // right color, wrong place
}

// Config describes a difficulty configuration.
type Config struct {
	Name        string
	Pegs        int  // classic 4
	Colors      int  // classic 6
	MaxGuesses  int  // classic 10-12
	AllowRepeat bool // classic true
}

// Status is the overall game state.
type Status int

const (
	Playing Status = iota
	Won
	Lost
)

// HistoryEntry is one row of the guess history.
type HistoryEntry struct {
	Guess    Guess
	Feedback Feedback
}

// GameState holds a full game.
type GameState struct {
	Cfg     Config
	Secret  Secret
	History []HistoryEntry
	Status  Status
}

// Presets from design doc §4.5.
var Presets = []Config{
	{Name: "Klassisk", Pegs: 4, Colors: 6, MaxGuesses: 10, AllowRepeat: true},
	{Name: "Lätt", Pegs: 4, Colors: 4, MaxGuesses: 12, AllowRepeat: true},
	{Name: "Svår", Pegs: 5, Colors: 8, MaxGuesses: 10, AllowRepeat: true},
	{Name: "Expert", Pegs: 5, Colors: 8, MaxGuesses: 8, AllowRepeat: false},
}

// Evaluate implements the two-pass feedback algorithm from §4.3. It correctly
// handles repeated colors: a position already counted as Black is never
// re-counted as White.
func Evaluate(secret Secret, guess Guess) Feedback {
	secretCount := map[Color]int{}
	guessCount := map[Color]int{}
	black := 0

	// Pass 1: count exact hits, tally the rest per color.
	for i := range secret {
		if i < len(guess) && secret[i] == guess[i] {
			black++
		} else {
			secretCount[secret[i]]++
			if i < len(guess) {
				guessCount[guess[i]]++
			}
		}
	}

	// Pass 2: for each color, white = min(count in secret, count in guess)
	// among the positions that were not exact hits.
	white := 0
	for color, n := range guessCount {
		if m := secretCount[color]; m < n {
			white += m
		} else {
			white += n
		}
	}

	return Feedback{Black: black, White: white}
}

// NewSecret generates a random secret according to cfg.
func NewSecret(cfg Config, rng *rand.Rand) Secret {
	s := make(Secret, cfg.Pegs)
	if cfg.AllowRepeat {
		for i := range s {
			s[i] = Color(rng.Intn(cfg.Colors))
		}
		return s
	}
	// No repeats: pick a random permutation prefix of the color set.
	perm := rng.Perm(cfg.Colors)
	for i := 0; i < cfg.Pegs; i++ {
		s[i] = Color(perm[i])
	}
	return s
}

// NewGame creates a fresh game for cfg with a random secret.
func NewGame(cfg Config, rng *rand.Rand) *GameState {
	return &GameState{
		Cfg:     cfg,
		Secret:  NewSecret(cfg, rng),
		History: nil,
		Status:  Playing,
	}
}

// Submit records a guess, computes feedback, and updates status. It returns the
// computed feedback. Guesses submitted after the game is over are ignored.
func (g *GameState) Submit(guess Guess) Feedback {
	if g.Status != Playing {
		return Feedback{}
	}
	fb := Evaluate(g.Secret, guess)
	g.History = append(g.History, HistoryEntry{Guess: append(Guess(nil), guess...), Feedback: fb})
	switch {
	case fb.Black == g.Cfg.Pegs:
		g.Status = Won
	case len(g.History) >= g.Cfg.MaxGuesses:
		g.Status = Lost
	}
	return fb
}
