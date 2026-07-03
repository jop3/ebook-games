package game

import (
	"math/rand"
	"time"
)

// Preset is a difficulty configuration, in the same spirit as the I rad
// presets: a named grid size + atom count.
type Preset struct {
	Name             string
	W, H             int
	Atoms            int
	WrongAtomPenalty int // score penalty added per incorrectly-guessed atom
}

// Presets are the built-in difficulty levels.
var Presets = []Preset{
	{Name: "Lätt (6x6, 3 atomer)", W: 6, H: 6, Atoms: 3, WrongAtomPenalty: 5},
	{Name: "Klassisk (8x8, 4 atomer)", W: 8, H: 8, Atoms: 4, WrongAtomPenalty: 5},
	{Name: "Svår (10x10, 5 atomer)", W: 10, H: 10, Atoms: 5, WrongAtomPenalty: 5},
}

// Phase is the current stage of a game.
type Phase int

const (
	// PhaseProbing: the player is firing rays to gather information.
	PhaseProbing Phase = iota
	// PhaseGuessing: the player is marking suspected atom cells.
	PhaseGuessing
	// PhaseDone: the answer has been submitted and revealed.
	PhaseDone
)

// FiredRay records one fired ray and the marker number assigned to it. Markers
// let the UI label the entry (and, for detours, the exit) edge points.
type FiredRay struct {
	Result RayResult
	Marker int // 1-based label shown at the edge point(s)
}

// GameState holds a full game in progress: the hidden grid, the player's fired
// rays, their guessed atom positions, and scoring.
type GameState struct {
	Cfg   Preset
	Grid  *Grid // hidden solution
	Phase Phase

	Fired      []FiredRay
	firedIndex map[int]int // entry edge index -> position in Fired (dedupe)

	Guesses  map[Cell]bool // player-marked suspected atom cells
	nextMark int

	// Filled in when the answer is submitted.
	Score        int
	CorrectAtoms int
	WrongAtoms   int
	MissedAtoms  int
}

// NewGame starts a new game from a preset, placing atoms randomly.
func NewGame(p Preset) *GameState {
	return NewGameSeeded(p, time.Now().UnixNano())
}

// NewGameSeeded starts a new game with a deterministic seed (useful in tests).
func NewGameSeeded(p Preset, seed int64) *GameState {
	g := NewGrid(p.W, p.H)
	rng := rand.New(rand.NewSource(seed))
	g.PlaceRandomAtoms(p.Atoms, rng)
	return &GameState{
		Cfg:        p,
		Grid:       g,
		Phase:      PhaseProbing,
		firedIndex: map[int]int{},
		Guesses:    map[Cell]bool{},
		nextMark:   1,
	}
}

// FireAt fires a ray from the edge point with the given index, records it, and
// returns the result plus whether it was newly fired. Re-firing the same edge
// point returns the existing record without changing state.
func (s *GameState) FireAt(edgeIndex int) (FiredRay, bool) {
	if s.Phase != PhaseProbing {
		return FiredRay{}, false
	}
	if pos, ok := s.firedIndex[edgeIndex]; ok {
		return s.Fired[pos], false
	}
	eps := s.Grid.EdgePoints()
	if edgeIndex < 0 || edgeIndex >= len(eps) {
		return FiredRay{}, false
	}
	res := s.Grid.Fire(eps[edgeIndex])

	fr := FiredRay{Result: res, Marker: s.nextMark}
	s.nextMark++
	s.Fired = append(s.Fired, fr)
	s.firedIndex[edgeIndex] = len(s.Fired) - 1
	// A detour occupies BOTH its entry and exit edge points; record the exit
	// index too so re-tapping the paired point shows the same result.
	if res.Outcome == OutcomeDetour {
		s.firedIndex[res.ExitIndex] = len(s.Fired) - 1
	}
	return fr, true
}

// RaysFired returns how many rays the player has fired (the primary score
// component).
func (s *GameState) RaysFired() int { return len(s.Fired) }

// ToggleGuess marks or unmarks a suspected atom cell (guessing phase only).
func (s *GameState) ToggleGuess(x, y int) {
	if s.Phase == PhaseDone {
		return
	}
	if !s.Grid.inBounds(x, y) {
		return
	}
	c := Cell{x, y}
	if s.Guesses[c] {
		delete(s.Guesses, c)
	} else {
		s.Guesses[c] = true
	}
}

// GuessCount returns the number of currently-marked cells.
func (s *GameState) GuessCount() int { return len(s.Guesses) }

// EnterGuessing moves from probing to guessing.
func (s *GameState) EnterGuessing() {
	if s.Phase == PhaseProbing {
		s.Phase = PhaseGuessing
	}
}

// Submit finalizes the game: compares the player's guesses to the hidden atoms,
// computes the score, and moves to PhaseDone. Score = rays fired + penalty per
// wrong guess. Lower is better.
func (s *GameState) Submit() {
	if s.Phase == PhaseDone {
		return
	}
	correct, wrong := 0, 0
	for c := range s.Guesses {
		if s.Grid.HasAtom(c.X, c.Y) {
			correct++
		} else {
			wrong++
		}
	}
	missed := s.Grid.AtomCount() - correct

	s.CorrectAtoms = correct
	s.WrongAtoms = wrong
	s.MissedAtoms = missed
	s.Score = s.RaysFired() + wrong*s.Cfg.WrongAtomPenalty
	s.Phase = PhaseDone
}

// Solved reports whether every atom was correctly identified with no wrong or
// missing guesses.
func (s *GameState) Solved() bool {
	return s.WrongAtoms == 0 && s.MissedAtoms == 0 && s.CorrectAtoms == s.Grid.AtomCount()
}
