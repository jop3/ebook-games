// Package game implements Nurikabe logic with no dependency on the inkview
// SDK, so it is unit-tested cgo-free.
//
// A puzzle is a grid with numbered seed cells. The player paints each cell as
// sea (black) or leaves it as island (white); the four win constraints are:
// each numbered island has exactly its number of cells and exactly one seed,
// islands never touch orthogonally, all sea is one connected region, and no
// 2x2 block is entirely sea.
package game

import (
	"math/rand"
	"time"
)

// CellState is the player's current paint on a cell.
type CellState int8

const (
	StateUnknown CellState = iota // undecided
	StateSea                      // painted black
	StateIsland                   // marked white (island) explicitly
)

// Preset is a difficulty: grid size.
type Preset struct {
	Name string
	W, H int
}

var Presets = []Preset{
	{"Lätt 5×5", 5, 5},
	{"Medel 7×7", 7, 7},
	{"Svår 9×9", 9, 9},
}

// Puzzle is a generated Nurikabe board: seed positions/sizes plus the hidden
// solution (true = sea) used for win-checking.
type Puzzle struct {
	W, H     int
	Seeds    map[[2]int]int // position -> island size
	Solution [][]bool       // [y][x], true = sea
}

// GameState is a puzzle plus the player's working grid.
type GameState struct {
	Cfg   Preset
	Puz   *Puzzle
	Cells [][]CellState // [y][x]
	Done  bool
}

// NewGame generates a fresh puzzle for the preset.
func NewGame(p Preset) *GameState {
	return NewGameSeeded(p, time.Now().UnixNano())
}

// NewGameSeeded generates a puzzle with a deterministic seed (for tests).
func NewGameSeeded(p Preset, seed int64) *GameState {
	rng := rand.New(rand.NewSource(seed))
	puz := Generate(p, rng)
	cells := make([][]CellState, p.H)
	for y := range cells {
		cells[y] = make([]CellState, p.W)
	}
	// Seed cells always start as known-island (they can't be painted sea).
	for pos := range puz.Seeds {
		cells[pos[1]][pos[0]] = StateIsland
	}
	return &GameState{Cfg: p, Puz: puz, Cells: cells}
}

// Toggle cycles a non-seed cell Unknown -> Sea -> Island -> Unknown.
func (s *GameState) Toggle(x, y int) bool {
	if s.Done || y < 0 || y >= s.Puz.H || x < 0 || x >= s.Puz.W {
		return false
	}
	if _, isSeed := s.Puz.Seeds[[2]int{x, y}]; isSeed {
		return false // seeds are fixed
	}
	switch s.Cells[y][x] {
	case StateUnknown:
		s.Cells[y][x] = StateSea
	case StateSea:
		s.Cells[y][x] = StateIsland
	default:
		s.Cells[y][x] = StateUnknown
	}
	s.checkDone()
	return true
}

// checkDone sets Done when the board is fully painted AND satisfies all four
// Nurikabe constraints. It deliberately does NOT compare against the stored
// generator fill: the generator doesn't certify uniqueness, so a puzzle can
// have several fully valid solutions — a player who finds a different one
// must still win, not be stuck staring at a correct board that never fires
// "Löst!".
func (s *GameState) checkDone() {
	sea := make([][]bool, s.Puz.H)
	for y := 0; y < s.Puz.H; y++ {
		sea[y] = make([]bool, s.Puz.W)
		for x := 0; x < s.Puz.W; x++ {
			switch s.Cells[y][x] {
			case StateUnknown:
				s.Done = false
				return
			case StateSea:
				sea[y][x] = true
			}
		}
	}
	s.Done = ValidateSolution(s.Puz.W, s.Puz.H, sea, s.Puz.Seeds)
}

// Reset clears all non-seed cells back to Unknown.
func (s *GameState) Reset() {
	for y := range s.Cells {
		for x := range s.Cells[y] {
			if _, isSeed := s.Puz.Seeds[[2]int{x, y}]; isSeed {
				continue
			}
			s.Cells[y][x] = StateUnknown
		}
	}
	s.Done = false
}

// IsSea2x2Violation reports whether any 2x2 block of currently-painted-Sea
// cells exists (Unknown cells don't count, so this only flags a definite
// mistake, not a pending deduction).
func IsSea2x2Violation(s *GameState) bool {
	for y := 0; y+1 < s.Puz.H; y++ {
		for x := 0; x+1 < s.Puz.W; x++ {
			if s.Cells[y][x] == StateSea && s.Cells[y][x+1] == StateSea &&
				s.Cells[y+1][x] == StateSea && s.Cells[y+1][x+1] == StateSea {
				return true
			}
		}
	}
	return false
}
