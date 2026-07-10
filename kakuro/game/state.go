// Package game implements Kakuro (sum-crossword) logic with no dependency on
// the inkview SDK, so it is unit-tested cgo-free.
//
// A puzzle is a fixed grid shape (which cells are black/blocked vs white
// entry cells), into which valid digit runs are filled procedurally each
// game; entry-cell values are 1-9 with no repeats within a run, and each run
// must sum to its clue.
package game

import (
	"math/rand"
	"time"
)

// CellKind distinguishes block cells (with optional clues) from entry cells.
type CellKind int8

const (
	KindBlock CellKind = iota
	KindEntry
)

// Cell is one grid position. For KindBlock, DownClue/RightClue hold the sum
// for the run starting immediately below/right (0 = no run that direction).
// For KindEntry, Value holds the player's current digit (0 = empty).
type Cell struct {
	Kind      CellKind
	DownClue  int
	RightClue int
	Value     int
	Solution  int // the generator's answer, used for win-checking
}

// Run is an ordered list of entry-cell coordinates (row,col) that must sum to
// Target with no repeated digit.
type Run struct {
	Cells  [][2]int
	Target int
}

// Preset is a difficulty: which fixed shape to use.
type Preset struct {
	Name  string
	Shape int // index into shapes
}

var Presets = []Preset{
	{"Lätt 6×6", 0},
	{"Medel 8×8", 1},
	{"Svår 10×10", 2},
}

// Puzzle is a generated Kakuro board: the shape's block layout plus derived
// clues and the hidden solution.
type Puzzle struct {
	W, H int
	Grid [][]Cell // [row][col]
	Runs []Run
}

// GameState is a puzzle plus done-tracking.
type GameState struct {
	Cfg  Preset
	Puz  *Puzzle
	Done bool
}

// NewGame generates a fresh puzzle for the preset.
func NewGame(p Preset) *GameState {
	return NewGameSeeded(p, time.Now().UnixNano())
}

// NewGameSeeded generates a puzzle with a deterministic seed (for tests).
func NewGameSeeded(p Preset, seed int64) *GameState {
	rng := rand.New(rand.NewSource(seed))
	puz := Generate(p, rng)
	return &GameState{Cfg: p, Puz: puz}
}

// SetDigit sets an entry cell's value (1-9, or 0 to clear) and re-checks Done.
func (s *GameState) SetDigit(row, col, v int) bool {
	if row < 0 || row >= s.Puz.H || col < 0 || col >= s.Puz.W {
		return false
	}
	c := &s.Puz.Grid[row][col]
	if c.Kind != KindEntry || s.Done {
		return false
	}
	c.Value = v
	s.checkDone()
	return true
}

func (s *GameState) checkDone() {
	for _, row := range s.Puz.Grid {
		for _, c := range row {
			if c.Kind == KindEntry && c.Value != c.Solution {
				s.Done = false
				return
			}
		}
	}
	s.Done = true
}

// Reset clears all player-entered digits.
func (s *GameState) Reset() {
	for r := range s.Puz.Grid {
		for c := range s.Puz.Grid[r] {
			if s.Puz.Grid[r][c].Kind == KindEntry {
				s.Puz.Grid[r][c].Value = 0
			}
		}
	}
	s.Done = false
}

// RunOK reports whether a run's currently-filled cells have no repeated
// digit and (if complete) sum to the target; a partial run that already
// exceeds the target or repeats a digit is also flagged not-OK.
func RunOK(puz *Puzzle, run Run) bool {
	seen := map[int]bool{}
	sum := 0
	filled := 0
	for _, rc := range run.Cells {
		v := puz.Grid[rc[0]][rc[1]].Value
		if v == 0 {
			continue
		}
		filled++
		if seen[v] {
			return false
		}
		seen[v] = true
		sum += v
	}
	if sum > run.Target {
		return false
	}
	if filled == len(run.Cells) && sum != run.Target {
		return false
	}
	return true
}
