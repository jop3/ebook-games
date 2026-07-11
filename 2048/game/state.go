package game

import (
	"math/rand"
	"strconv"
	"strings"
)

// Status is the high-level state of a game.
type Status int

const (
	StatusPlaying Status = iota
	StatusWon            // reached the target tile; player may keep playing
	StatusOver           // no move possible
)

// GameState is a full playable 2048 game.
type GameState struct {
	Board   Board
	Score   int
	Best    int
	Target  int // winning tile value, e.g. 2048
	Status  Status
	WonSeen bool // the win banner has been shown once (continuing play suppresses it)
	rng     *rand.Rand
}

// NewGame starts a fresh game with two initial tiles, seeded from rng (pass
// rand.New(rand.NewSource(seed)) so callers control determinism in tests; the
// app seeds from real entropy at startup).
func NewGame(target int, best int, rng *rand.Rand) *GameState {
	s := &GameState{Target: target, Best: best, Status: StatusPlaying, rng: rng}
	s.Board = Spawn(s.Board, rng)
	s.Board = Spawn(s.Board, rng)
	return s
}

// Move applies a swipe. Returns true if the board changed (and thus a new
// tile spawned and status was re-evaluated); false for a no-op swipe.
func (s *GameState) Move(dir Dir) bool {
	if s.Status == StatusOver {
		return false
	}
	out, gained, moved := Slide(s.Board, dir)
	if !moved {
		return false
	}
	s.Board = Spawn(out, s.rng)
	s.Score += gained
	if s.Score > s.Best {
		s.Best = s.Score
	}
	s.Status = statusFor(s.Board, s.Target, s.WonSeen)
	return true
}

// statusFor is the pure win/game-over evaluation Move applies after every
// spawn; split out so it unit-tests against a hand-built board without
// needing to route a specific spawn through the RNG.
func statusFor(b Board, target int, wonSeen bool) Status {
	switch {
	case Won(b, target) && !wonSeen:
		return StatusWon
	case !CanMove(b):
		return StatusOver
	default:
		return StatusPlaying
	}
}

// Continue dismisses the win banner and keeps playing past the target tile.
// The status is re-derived rather than blindly set to Playing: the winning
// move may ALSO have filled the board with no merges left (Won has priority
// over Over in statusFor), and continuing such a game would silently freeze —
// no banner, no legal move, forever.
func (s *GameState) Continue() {
	s.WonSeen = true
	if s.Status == StatusWon {
		s.Status = statusFor(s.Board, s.Target, s.WonSeen)
	}
}

// --- Best-score persistence --------------------------------------------------
//
// Stored as the decimal ASCII text of the integer, in a single small file.
// A missing or corrupt file is treated as best=0 (never fatal).

// ParseBest decodes a best-score file's contents. Invalid/empty content
// yields 0, never an error — the caller always gets a usable value.
func ParseBest(data []byte) int {
	s := strings.TrimSpace(string(data))
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// FormatBest encodes a best score for writing to the persistence file.
func FormatBest(best int) []byte {
	return []byte(strconv.Itoa(best))
}
