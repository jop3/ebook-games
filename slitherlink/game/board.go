// Package game implements Slitherlink logic with no dependency on the inkview
// SDK, so it is unit-tested cgo-free.
//
// A puzzle is a W x H grid of cells (dots at (x,y) for x in 0..W, y in 0..H).
// Some cells carry a clue 0..3: the number of ON edges that must surround it.
// The player toggles edges (segments between adjacent dots) on/off/marked-off
// until the ON edges form a single closed loop that satisfies every clue.
//
// Edges are stored as two grids:
//
//	HEdge[y][x] — horizontal edge between dot (x,y) and (x+1,y); y in 0..H, x in 0..W-1
//	VEdge[y][x] — vertical edge between dot (x,y) and (x,y+1);   y in 0..H-1, x in 0..W
package game

// EdgeState is what the player has marked an edge as.
type EdgeState int8

const (
	EdgeUnknown EdgeState = iota // untouched
	EdgeOn                       // part of the loop
	EdgeOff                      // player marks definitely not part of the loop (X)
)

// Preset is a difficulty: grid size.
type Preset struct {
	Name string
	W, H int
}

// Presets offered on the menu. Kept modest so the solver stays fast and clue
// grids fit the e-ink screen.
var Presets = []Preset{
	{"Lätt 5×5", 5, 5},
	{"Medel 7×7", 7, 7},
	{"Svår 10×10", 10, 10},
}

// Puzzle is a generated, uniquely-solvable slitherlink board.
type Puzzle struct {
	W, H int
	// Clue[y][x] is the clue for cell (x,y), or -1 if the cell has no clue.
	Clue [][]int
	// SolutionH/SolutionV are the loop edges of the generated solution (for
	// reference / debugging; not shown to the player).
	SolutionH [][]bool // [y][x], y in 0..H, x in 0..W-1
	SolutionV [][]bool // [y][x], y in 0..H-1, x in 0..W
}

// Board is a puzzle plus the player's working edge grids.
type Board struct {
	W, H int
	Clue [][]int // [y][x], -1 = no clue

	HEdge [][]EdgeState // [y][x], y in 0..H, x in 0..W-1
	VEdge [][]EdgeState // [y][x], y in 0..H-1, x in 0..W
}

// NewBoard creates an empty player board for the given puzzle.
func NewBoard(p *Puzzle) *Board {
	b := &Board{W: p.W, H: p.H}
	b.Clue = make([][]int, p.H)
	for y := range b.Clue {
		b.Clue[y] = make([]int, p.W)
		copy(b.Clue[y], p.Clue[y])
	}
	b.HEdge = make([][]EdgeState, p.H+1)
	for y := range b.HEdge {
		b.HEdge[y] = make([]EdgeState, p.W)
	}
	b.VEdge = make([][]EdgeState, p.H)
	for y := range b.VEdge {
		b.VEdge[y] = make([]EdgeState, p.W+1)
	}
	return b
}

// ToggleH cycles a horizontal edge Unknown -> On -> Off -> Unknown.
func (b *Board) ToggleH(x, y int) bool {
	if y < 0 || y > b.H || x < 0 || x >= b.W {
		return false
	}
	b.HEdge[y][x] = nextState(b.HEdge[y][x])
	return true
}

// ToggleV cycles a vertical edge Unknown -> On -> Off -> Unknown.
func (b *Board) ToggleV(x, y int) bool {
	if y < 0 || y >= b.H || x < 0 || x > b.W {
		return false
	}
	b.VEdge[y][x] = nextState(b.VEdge[y][x])
	return true
}

func nextState(s EdgeState) EdgeState {
	switch s {
	case EdgeUnknown:
		return EdgeOn
	case EdgeOn:
		return EdgeOff
	default:
		return EdgeUnknown
	}
}

// Reset clears the player's edges.
func (b *Board) Reset() {
	for y := range b.HEdge {
		for x := range b.HEdge[y] {
			b.HEdge[y][x] = EdgeUnknown
		}
	}
	for y := range b.VEdge {
		for x := range b.VEdge[y] {
			b.VEdge[y][x] = EdgeUnknown
		}
	}
}

// CellCount returns the number of ON edges surrounding cell (x,y).
func (b *Board) CellCount(x, y int) int {
	n := 0
	if b.HEdge[y][x] == EdgeOn {
		n++
	}
	if b.HEdge[y+1][x] == EdgeOn {
		n++
	}
	if b.VEdge[y][x] == EdgeOn {
		n++
	}
	if b.VEdge[y][x+1] == EdgeOn {
		n++
	}
	return n
}

// AllCluesSatisfied reports whether every clued cell has exactly its number of
// ON edges.
func (b *Board) AllCluesSatisfied() bool {
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			if b.Clue[y][x] >= 0 && b.CellCount(x, y) != b.Clue[y][x] {
				return false
			}
		}
	}
	return true
}

// dotDegreeOn returns how many ON edges touch dot (x,y).
func dotDegreeOn(hEdge, vEdge [][]EdgeState, W, H, x, y int) int {
	n := 0
	if x > 0 && hEdge[y][x-1] == EdgeOn {
		n++
	}
	if x < W && hEdge[y][x] == EdgeOn {
		n++
	}
	if y > 0 && vEdge[y-1][x] == EdgeOn {
		n++
	}
	if y < H && vEdge[y][x] == EdgeOn {
		n++
	}
	return n
}

// IsSingleLoop reports whether the ON edges in hEdge/vEdge form exactly one
// closed loop: every dot touched by an ON edge has degree exactly 2, and
// walking the loop from any ON edge visits every ON edge and returns to start
// (rules out a satisfied-clue board that is actually two or more separate
// loops, or a loop plus a stray branch).
func IsSingleLoop(hEdge, vEdge [][]EdgeState, W, H int) bool {
	totalOn := 0
	for y := range hEdge {
		for x := range hEdge[y] {
			if hEdge[y][x] == EdgeOn {
				totalOn++
			}
		}
	}
	for y := range vEdge {
		for x := range vEdge[y] {
			if vEdge[y][x] == EdgeOn {
				totalOn++
			}
		}
	}
	if totalOn == 0 {
		return false // no loop at all
	}
	for y := 0; y <= H; y++ {
		for x := 0; x <= W; x++ {
			d := dotDegreeOn(hEdge, vEdge, W, H, x, y)
			if d != 0 && d != 2 {
				return false
			}
		}
	}
	// Walk from the first ON edge found, following degree-2 dots, and count
	// how many ON edges get visited.
	var startX, startY int
	found := false
	for y := 0; y <= H && !found; y++ {
		for x := 0; x < W && !found; x++ {
			if hEdge[y][x] == EdgeOn {
				startX, startY = x, y
				found = true
			}
		}
	}
	if !found {
		for y := 0; y < H && !found; y++ {
			for x := 0; x <= W && !found; x++ {
				if vEdge[y][x] == EdgeOn {
					startX, startY = x, y
					found = true
				}
			}
		}
	}
	visitedH := make(map[[2]int]bool)
	visitedV := make(map[[2]int]bool)

	// Walk the loop starting at dot (startX,startY) along its first ON edge.
	curX, curY := startX, startY
	prevX, prevY := -1, -1
	visited := 0
	for {
		nx, ny, isH, ex, ey, ok := nextLoopStep(hEdge, vEdge, W, H, curX, curY, prevX, prevY)
		if !ok {
			return false
		}
		if isH {
			if visitedH[[2]int{ex, ey}] {
				break
			}
			visitedH[[2]int{ex, ey}] = true
		} else {
			if visitedV[[2]int{ex, ey}] {
				break
			}
			visitedV[[2]int{ex, ey}] = true
		}
		visited++
		prevX, prevY = curX, curY
		curX, curY = nx, ny
		if curX == startX && curY == startY {
			break
		}
		if visited > totalOn+1 {
			return false // safety: shouldn't happen given degree constraint
		}
	}
	return visited == totalOn && curX == startX && curY == startY
}

// nextLoopStep finds the ON edge leaving dot (x,y) that is not the edge we
// arrived on from (prevX,prevY), and returns the dot at its other end plus
// which edge (h or v) and its grid indices.
func nextLoopStep(hEdge, vEdge [][]EdgeState, W, H, x, y, prevX, prevY int) (nx, ny int, isH bool, ex, ey int, ok bool) {
	type cand struct {
		nx, ny int
		isH    bool
		ex, ey int
	}
	var cands []cand
	if x > 0 && hEdge[y][x-1] == EdgeOn {
		cands = append(cands, cand{x - 1, y, true, x - 1, y})
	}
	if x < W && hEdge[y][x] == EdgeOn {
		cands = append(cands, cand{x + 1, y, true, x, y})
	}
	if y > 0 && vEdge[y-1][x] == EdgeOn {
		cands = append(cands, cand{x, y - 1, false, x, y - 1})
	}
	if y < H && vEdge[y][x] == EdgeOn {
		cands = append(cands, cand{x, y + 1, false, x, y})
	}
	for _, c := range cands {
		if c.nx == prevX && c.ny == prevY {
			continue
		}
		return c.nx, c.ny, c.isH, c.ex, c.ey, true
	}
	return 0, 0, false, 0, 0, false
}

// Solved reports whether the board is a winning state: every clue satisfied
// and the ON edges form a single closed loop.
func (b *Board) Solved() bool {
	if !b.AllCluesSatisfied() {
		return false
	}
	return IsSingleLoop(b.HEdge, b.VEdge, b.W, b.H)
}
