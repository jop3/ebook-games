package game

import (
	"math/rand"
	"time"
)

// CellState is what the player has marked in a cell.
type CellState int8

const (
	StateUnknown CellState = iota // untouched
	StateFilled                   // player says filled
	StateMarked                   // player says definitely blank (X)
)

// Preset is a difficulty: grid size and target fill density.
type Preset struct {
	Name    string
	W, H    int
	Density float64 // fraction of cells filled in the generated picture
}

// Presets offered on the menu. Kept modest so the line solver stays fast and
// clues fit the e-ink screen.
var Presets = []Preset{
	{"Lätt 5×5", 5, 5, 0.55},
	{"Medel 10×10", 10, 10, 0.55},
	{"Svår 15×15", 15, 15, 0.55},
}

// Puzzle is a generated, uniquely-solvable nonogram.
type Puzzle struct {
	W, H     int
	Solution [][]bool // the hidden picture, [y][x]
	RowClues []Clue
	ColClues []Clue
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
	cells := make([][]CellState, puz.H)
	for y := range cells {
		cells[y] = make([]CellState, puz.W)
	}
	return &GameState{Cfg: p, Puz: puz, Cells: cells}
}

// Generate builds a uniquely line-solvable puzzle by trial: draw a random
// picture at the target density, derive clues, and accept it only if the line
// solver resolves it uniquely. Retries with fresh pictures otherwise.
func Generate(p Preset, rng *rand.Rand) *Puzzle {
	for attempt := 0; attempt < 400; attempt++ {
		sol := randomPicture(p, rng)
		rowClues := make([]Clue, p.H)
		for y := 0; y < p.H; y++ {
			rowClues[y] = LineClue(sol[y])
		}
		colClues := make([]Clue, p.W)
		for x := 0; x < p.W; x++ {
			col := make([]bool, p.H)
			for y := 0; y < p.H; y++ {
				col[y] = sol[y][x]
			}
			colClues[x] = LineClue(col)
		}
		if LineSolvable(rowClues, colClues) == SolveUnique {
			return &Puzzle{W: p.W, H: p.H, Solution: sol, RowClues: rowClues, ColClues: colClues}
		}
	}
	// Fallback: return the last picture even if not uniquely line-solvable so
	// the game always starts. (Extremely unlikely at these sizes/densities.)
	sol := randomPicture(p, rng)
	rowClues := make([]Clue, p.H)
	for y := 0; y < p.H; y++ {
		rowClues[y] = LineClue(sol[y])
	}
	colClues := make([]Clue, p.W)
	for x := 0; x < p.W; x++ {
		col := make([]bool, p.H)
		for y := 0; y < p.H; y++ {
			col[y] = sol[y][x]
		}
		colClues[x] = LineClue(col)
	}
	return &Puzzle{W: p.W, H: p.H, Solution: sol, RowClues: rowClues, ColClues: colClues}
}

// randomPicture draws a random filled picture by independent per-cell
// sampling at the preset's density. An all-blank row or column is possible
// and fine — it just renders as a "0" clue.
func randomPicture(p Preset, rng *rand.Rand) [][]bool {
	sol := make([][]bool, p.H)
	for y := range sol {
		sol[y] = make([]bool, p.W)
		for x := range sol[y] {
			sol[y][x] = rng.Float64() < p.Density
		}
	}
	return sol
}

// Toggle cycles a cell Unknown -> Filled -> Marked -> Unknown and re-checks the
// win condition. Returns true if the cell changed.
func (s *GameState) Toggle(x, y int) bool {
	if s.Done || y < 0 || y >= s.Puz.H || x < 0 || x >= s.Puz.W {
		return false
	}
	switch s.Cells[y][x] {
	case StateUnknown:
		s.Cells[y][x] = StateFilled
	case StateFilled:
		s.Cells[y][x] = StateMarked
	default:
		s.Cells[y][x] = StateUnknown
	}
	s.checkDone()
	return true
}

// checkDone sets Done when the filled cells exactly match the solution. Marked
// (X) cells are treated as blank; only filled placement matters.
func (s *GameState) checkDone() {
	for y := 0; y < s.Puz.H; y++ {
		for x := 0; x < s.Puz.W; x++ {
			filled := s.Cells[y][x] == StateFilled
			if filled != s.Puz.Solution[y][x] {
				s.Done = false
				return
			}
		}
	}
	s.Done = true
}

// Reset clears the player's grid.
func (s *GameState) Reset() {
	for y := range s.Cells {
		for x := range s.Cells[y] {
			s.Cells[y][x] = StateUnknown
		}
	}
	s.Done = false
}
