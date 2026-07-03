// Package game implements Hashiwokakero (Bridges) logic with no dependency on
// the inkview SDK, so it is unit-tested cgo-free.
//
// A puzzle is a set of numbered islands on a grid. The player connects islands
// with horizontal/vertical bridges (at most 2 between any pair) so each
// island's bridge count matches its number, bridges never cross, and the
// whole network ends up connected.
package game

import (
	"math/rand"
	"time"
)

// Island is a numbered node at (X,Y) requiring exactly Need bridges.
type Island struct {
	X, Y int
	Need int
}

// Preset is a difficulty: grid size and island count.
type Preset struct {
	Name       string
	W, H       int
	NumIslands int
}

// Presets offered on the menu.
var Presets = []Preset{
	{"Lätt 7×7", 7, 7, 8},
	{"Medel 10×10", 10, 10, 14},
	{"Svår 13×13", 13, 13, 20},
}

// Puzzle is a generated, uniquely-solvable Hashiwokakero board.
type Puzzle struct {
	W, H    int
	Islands []Island
	indexAt map[[2]int]int // grid pos -> island index, for neighbour lookup
}

// GameState is a puzzle plus the player's working bridge counts.
type GameState struct {
	Cfg     Preset
	Puz     *Puzzle
	Bridges map[[2]int]int // key {min(i,j)*n+max(i,j)} -> count 0,1,2; only for neighbour pairs
	Done    bool
}

func pairKey(i, j int) [2]int {
	if i > j {
		i, j = j, i
	}
	return [2]int{i, j}
}

// NewGame generates a fresh puzzle for the preset.
func NewGame(p Preset) *GameState {
	return NewGameSeeded(p, time.Now().UnixNano())
}

// NewGameSeeded generates a puzzle with a deterministic seed (for tests).
func NewGameSeeded(p Preset, seed int64) *GameState {
	rng := rand.New(rand.NewSource(seed))
	puz := Generate(p, rng)
	return &GameState{Cfg: p, Puz: puz, Bridges: map[[2]int]int{}}
}

// neighbourIslands returns, for island i, the map of direction -> neighbour
// island index reachable in a straight line with no island in between.
func (p *Puzzle) neighbours(i int) map[int]int {
	src := p.Islands[i]
	out := map[int]int{}
	// 0=up,1=down,2=left,3=right
	dirs := [4][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
	for d, dv := range dirs {
		x, y := src.X, src.Y
		for {
			x += dv[0]
			y += dv[1]
			if x < 0 || x >= p.W || y < 0 || y >= p.H {
				break
			}
			if j, ok := p.indexAt[[2]int{x, y}]; ok {
				out[d] = j
				break
			}
		}
	}
	return out
}

// NeighbourList returns the sorted list of neighbour island indices for i
// (any direction), used by the UI to know which islands are tappable.
func (p *Puzzle) NeighbourList(i int) []int {
	nb := p.neighbours(i)
	seen := map[int]bool{}
	var out []int
	for _, j := range nb {
		if !seen[j] {
			seen[j] = true
			out = append(out, j)
		}
	}
	return out
}

// crosses reports whether a horizontal bridge (islands a,b, same Y) and a
// vertical bridge (islands c,d, same X) would physically cross.
func crosses(p *Puzzle, a, b, c, d int) bool {
	ia, ib, ic, id := p.Islands[a], p.Islands[b], p.Islands[c], p.Islands[d]
	horiz := ia.Y == ib.Y
	vert := ic.X == id.X
	if !horiz || !vert {
		// try swapped
		if ic.Y == id.Y && ia.X == ib.X {
			return crosses(p, c, d, a, b)
		}
		return false
	}
	y := ia.Y
	x0, x1 := ia.X, ib.X
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	x := ic.X
	y0, y1 := ic.Y, id.Y
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	return x > x0 && x < x1 && y > y0 && y < y1
}

// CanAdd reports whether a bridge can be added between neighbour islands i,j
// (not exceeding 2, and not crossing any existing bridge).
func (s *GameState) CanAdd(i, j int) bool {
	if s.Bridges[pairKey(i, j)] >= 2 {
		return false
	}
	for k, cnt := range s.Bridges {
		if cnt == 0 {
			continue
		}
		a, b := k[0], k[1]
		if (a == i && b == j) || (a == j && b == i) {
			continue
		}
		if crosses(s.Puz, i, j, a, b) {
			return false
		}
	}
	return true
}

// Cycle toggles the bridge count between neighbour islands i,j: 0->1->2->0.
// Returns false if i,j aren't neighbours or the increment is illegal.
func (s *GameState) Cycle(i, j int) bool {
	if i == j {
		return false
	}
	if !isNeighbour(s.Puz, i, j) {
		return false
	}
	key := pairKey(i, j)
	cur := s.Bridges[key]
	if cur >= 2 {
		delete(s.Bridges, key)
		s.checkDone()
		return true
	}
	if !s.CanAdd(i, j) {
		// still allow cycling back to 0 to unstick, else reject
		return false
	}
	s.Bridges[key] = cur + 1
	s.checkDone()
	return true
}

func isNeighbour(p *Puzzle, i, j int) bool {
	for _, k := range p.NeighbourList(i) {
		if k == j {
			return true
		}
	}
	return false
}

// Degree returns island i's current total bridge count.
func (s *GameState) Degree(i int) int {
	d := 0
	for k, cnt := range s.Bridges {
		if k[0] == i || k[1] == i {
			d += cnt
		}
	}
	return d
}

// Connected reports whether all islands with at least one bridge form a
// single connected component including every island (islands with Need==0
// don't occur, so every island must be reachable).
func (s *GameState) Connected() bool {
	n := len(s.Puz.Islands)
	if n == 0 {
		return true
	}
	adj := make([][]int, n)
	for k, cnt := range s.Bridges {
		if cnt == 0 {
			continue
		}
		adj[k[0]] = append(adj[k[0]], k[1])
		adj[k[1]] = append(adj[k[1]], k[0])
	}
	seen := make([]bool, n)
	stack := []int{0}
	seen[0] = true
	count := 1
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, nb := range adj[cur] {
			if !seen[nb] {
				seen[nb] = true
				count++
				stack = append(stack, nb)
			}
		}
	}
	return count == n
}

// checkDone sets Done when every island's degree matches its Need and the
// network is fully connected.
func (s *GameState) checkDone() {
	for i, isl := range s.Puz.Islands {
		if s.Degree(i) != isl.Need {
			s.Done = false
			return
		}
	}
	s.Done = s.Connected()
}

// Reset clears all player bridges.
func (s *GameState) Reset() {
	s.Bridges = map[[2]int]int{}
	s.Done = false
}
