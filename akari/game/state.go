// Package game implements Akari (Light Up) puzzle logic with no dependency on
// the inkview SDK, so it is unit-tested cgo-free.
//
// A puzzle is a W x H grid. Each cell is either a black wall (optionally
// carrying a number 0-4 that constrains how many bulbs touch it orthogonally)
// or a white cell. The player places bulbs in white cells. A bulb lights its
// own cell and shines in the four orthogonal directions until it hits a wall
// or the grid edge. The player wins when every white cell is lit and no two
// bulbs shine on each other, and every numbered wall's adjacent-bulb count
// matches its number.
package game

import (
	"math/rand"
	"time"
)

// CellKind is the static board terrain (fixed at generation time).
type CellKind int8

const (
	White CellKind = iota
	Wall           // black wall, no number (Number == -1)
)

// Cell is one static board square.
type Cell struct {
	Kind   CellKind
	Number int // 0-4 if the wall carries a clue; -1 if it's a plain/unnumbered wall
}

// Board is the static puzzle layout (walls + numbers). It never changes once
// generated; player state (bulbs, marks) lives separately in GameState.
type Board struct {
	W, H  int
	Cells [][]Cell // [y][x]
}

func newBoard(w, h int) *Board {
	b := &Board{W: w, H: h}
	b.Cells = make([][]Cell, h)
	for y := range b.Cells {
		b.Cells[y] = make([]Cell, w)
	}
	return b
}

func (b *Board) inBounds(x, y int) bool {
	return x >= 0 && x < b.W && y >= 0 && y < b.H
}

func (b *Board) at(x, y int) Cell { return b.Cells[y][x] }

// dirs is the four orthogonal directions used for ray-casting and adjacency.
var dirs = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// Lit computes the set of cells illuminated by the given bulbs. Ray-casting
// stops AT a wall (the wall cell itself is never "lit"); a bulb's own cell is
// always lit.
func Lit(b *Board, bulbs [][]bool) [][]bool {
	lit := make([][]bool, b.H)
	for y := range lit {
		lit[y] = make([]bool, b.W)
	}
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			if !bulbs[y][x] {
				continue
			}
			lit[y][x] = true
			for _, d := range dirs {
				cx, cy := x+d[0], y+d[1]
				for b.inBounds(cx, cy) && b.at(cx, cy).Kind != Wall {
					lit[cy][cx] = true
					cx += d[0]
					cy += d[1]
				}
			}
		}
	}
	return lit
}

// BulbSeesBulb reports whether any two bulbs shine on each other (share a ray
// segment with no wall between them).
func BulbSeesBulb(b *Board, bulbs [][]bool) bool {
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			if !bulbs[y][x] {
				continue
			}
			for _, d := range dirs {
				cx, cy := x+d[0], y+d[1]
				for b.inBounds(cx, cy) && b.at(cx, cy).Kind != Wall {
					if bulbs[cy][cx] {
						return true
					}
					cx += d[0]
					cy += d[1]
				}
			}
		}
	}
	return false
}

// WallsSatisfied reports whether every numbered wall has exactly its required
// count of orthogonally adjacent bulbs.
func WallsSatisfied(b *Board, bulbs [][]bool) bool {
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			c := b.at(x, y)
			if c.Kind != Wall || c.Number < 0 {
				continue
			}
			n := 0
			for _, d := range dirs {
				cx, cy := x+d[0], y+d[1]
				if b.inBounds(cx, cy) && bulbs[cy][cx] {
					n++
				}
			}
			if n != c.Number {
				return false
			}
		}
	}
	return true
}

// AllWhiteLit reports whether every white cell is lit.
func AllWhiteLit(b *Board, lit [][]bool) bool {
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			if b.at(x, y).Kind == White && !lit[y][x] {
				return false
			}
		}
	}
	return true
}

// Solved reports whether the given bulb placement is a valid solution.
func Solved(b *Board, bulbs [][]bool) bool {
	if BulbSeesBulb(b, bulbs) {
		return false
	}
	if !WallsSatisfied(b, bulbs) {
		return false
	}
	lit := Lit(b, bulbs)
	return AllWhiteLit(b, lit)
}

// --- Player-facing game state ------------------------------------------------

// MarkState is what the player has placed in a white cell.
type MarkState int8

const (
	MarkEmpty MarkState = iota
	MarkBulb
	MarkDot // "known empty" memory mark
)

// Preset is a difficulty: grid size and target wall density.
type Preset struct {
	Name string
	W, H int
}

// Presets offered on the menu.
var Presets = []Preset{
	{"Lätt 7×7", 7, 7},
	{"Medel 10×10", 10, 10},
	{"Svår 14×14", 14, 14},
}

// GameState is a generated board plus the player's working marks.
type GameState struct {
	Cfg   Preset
	Board *Board
	Marks [][]MarkState // [y][x], meaningful only on White cells
	Done  bool
}

// NewGame generates a fresh puzzle for the preset.
func NewGame(p Preset) *GameState {
	return NewGameSeeded(p, time.Now().UnixNano())
}

// NewGameSeeded generates a puzzle with a deterministic seed (for tests).
func NewGameSeeded(p Preset, seed int64) *GameState {
	rng := rand.New(rand.NewSource(seed))
	b := Generate(p, rng)
	marks := make([][]MarkState, b.H)
	for y := range marks {
		marks[y] = make([]MarkState, b.W)
	}
	return &GameState{Cfg: p, Board: b, Marks: marks}
}

// bulbsFromMarks extracts the boolean bulb grid from the player's marks.
func (s *GameState) bulbsFromMarks() [][]bool {
	bulbs := make([][]bool, s.Board.H)
	for y := range bulbs {
		bulbs[y] = make([]bool, s.Board.W)
		for x := range bulbs[y] {
			bulbs[y][x] = s.Marks[y][x] == MarkBulb
		}
	}
	return bulbs
}

// Toggle cycles a white cell empty -> bulb -> dot -> empty. Returns true if
// the cell changed (no-op on wall cells or once solved).
func (s *GameState) Toggle(x, y int) bool {
	if s.Done || !s.Board.inBounds(x, y) || s.Board.at(x, y).Kind != White {
		return false
	}
	switch s.Marks[y][x] {
	case MarkEmpty:
		s.Marks[y][x] = MarkBulb
	case MarkBulb:
		s.Marks[y][x] = MarkDot
	default:
		s.Marks[y][x] = MarkEmpty
	}
	s.checkDone()
	return true
}

// LitGrid returns the current lit set given the player's bulbs.
func (s *GameState) LitGrid() [][]bool {
	return Lit(s.Board, s.bulbsFromMarks())
}

// ConflictBulbs returns, for each cell, whether that bulb sees another bulb
// (used to highlight rule violations).
func (s *GameState) ConflictBulbs() [][]bool {
	bulbs := s.bulbsFromMarks()
	conflict := make([][]bool, s.Board.H)
	for y := range conflict {
		conflict[y] = make([]bool, s.Board.W)
	}
	for y := 0; y < s.Board.H; y++ {
		for x := 0; x < s.Board.W; x++ {
			if !bulbs[y][x] {
				continue
			}
			for _, d := range dirs {
				cx, cy := x+d[0], y+d[1]
				for s.Board.inBounds(cx, cy) && s.Board.at(cx, cy).Kind != Wall {
					if bulbs[cy][cx] {
						conflict[y][x] = true
						conflict[cy][cx] = true
					}
					cx += d[0]
					cy += d[1]
				}
			}
		}
	}
	return conflict
}

// WallOK reports, for a numbered wall cell, whether its adjacent bulb count
// currently matches its number exactly.
func (s *GameState) WallOK(x, y int) bool {
	c := s.Board.at(x, y)
	if c.Kind != Wall || c.Number < 0 {
		return true
	}
	bulbs := s.bulbsFromMarks()
	n := 0
	for _, d := range dirs {
		cx, cy := x+d[0], y+d[1]
		if s.Board.inBounds(cx, cy) && bulbs[cy][cx] {
			n++
		}
	}
	return n == c.Number
}

func (s *GameState) checkDone() {
	s.Done = Solved(s.Board, s.bulbsFromMarks())
}

// Reset clears the player's marks.
func (s *GameState) Reset() {
	for y := range s.Marks {
		for x := range s.Marks[y] {
			s.Marks[y][x] = MarkEmpty
		}
	}
	s.Done = false
}
