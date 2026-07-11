package game

import (
	"math/rand"
	"time"
)

// generate.go builds a random single closed loop on the dot grid, derives
// each cell's clue from it, then strips clues (retaining only those needed
// for a unique solution) — the "solution-first, dig holes" approach used by
// PocketPuzzles' loopy.c (see guide).

// GameState is a puzzle plus the player's working board.
type GameState struct {
	Cfg  Preset
	Puz  *Puzzle
	Bd   *Board
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
	return &GameState{Cfg: p, Puz: puz, Bd: NewBoard(puz)}
}

// Toggle cycles the horizontal or vertical edge nearest a logical edge
// address. Convenience wrapper kept parallel to the UI's edge addressing.
func (g *GameState) ToggleH(x, y int) bool {
	if g.Done {
		return false
	}
	ok := g.Bd.ToggleH(x, y)
	if ok {
		g.Done = g.Bd.Solved()
	}
	return ok
}

func (g *GameState) ToggleV(x, y int) bool {
	if g.Done {
		return false
	}
	ok := g.Bd.ToggleV(x, y)
	if ok {
		g.Done = g.Bd.Solved()
	}
	return ok
}

func (g *GameState) Reset() {
	g.Bd.Reset()
	g.Done = false
}

const (
	maxLoopAttempts     = 60
	maxStripOuterRounds = 6
)

// Generate builds a uniquely-solvable Slitherlink puzzle: grow a random
// single loop covering a good fraction of the grid, derive full clues, then
// remove clues (in shuffled order, rolling back if uniqueness breaks) so the
// final puzzle has as few givens as it can while the solver still proves it
// unique. Falls back to the fullest-clue version if stripping/loop generation
// struggles, so the app always starts.
func Generate(p Preset, rng *rand.Rand) *Puzzle {
	var best *Puzzle
	for attempt := 0; attempt < maxLoopAttempts; attempt++ {
		loopH, loopV, ok := randomLoop(p.W, p.H, rng)
		if !ok {
			continue
		}
		fullClue := deriveClues(p.W, p.H, loopH, loopV)
		puz := &Puzzle{W: p.W, H: p.H, Clue: fullClue, SolutionH: loopH, SolutionV: loopV}
		if Solve(puz) != SolveUnique {
			// A full-clue board (every cell given) is virtually always
			// unique, but guard anyway.
			continue
		}
		best = puz
		stripped := stripClues(puz, rng)
		return stripped
	}
	if best != nil {
		return best
	}
	// Absolute fallback: a simple rectangular loop around the border so the
	// app always starts even if random generation somehow fails repeatedly.
	loopH, loopV := borderLoop(p.W, p.H)
	full := deriveClues(p.W, p.H, loopH, loopV)
	return &Puzzle{W: p.W, H: p.H, Clue: full, SolutionH: loopH, SolutionV: loopV}
}

// deriveClues computes each cell's ON-edge count from a solved loop.
func deriveClues(W, H int, hEdge, vEdge [][]bool) [][]int {
	clue := make([][]int, H)
	for y := 0; y < H; y++ {
		clue[y] = make([]int, W)
		for x := 0; x < W; x++ {
			n := 0
			if hEdge[y][x] {
				n++
			}
			if hEdge[y+1][x] {
				n++
			}
			if vEdge[y][x] {
				n++
			}
			if vEdge[y][x+1] {
				n++
			}
			clue[y][x] = n
		}
	}
	return clue
}

// maxStripDuration bounds the total wall-clock time stripClues may spend
// trying to remove clues. On larger grids (10x10), later removals leave the
// board sparsely clued, and each uniqueness check can then burn its full
// per-solve search-node budget (maxSearchNodes) before concluding "still
// unique" or "no longer unique" — repeated over the remaining cells this adds
// up to tens of seconds or more. Rather than let stripping run unbounded, we
// cap total effort and gracefully stop early, keeping whatever clues have
// been removed so far: the puzzle in hand was already certified SolveUnique
// before this function started removing anything further, so bailing out
// mid-way still yields a valid, uniquely-solvable (if slightly more clued
// than ideal) puzzle. Mirrors the spec's "bounded retry + fallback so the app
// always starts" pattern used elsewhere in this codebase.
const maxStripDuration = 1500 * time.Millisecond

// stripClues removes as many clues as possible while the solver still proves
// the puzzle uniquely solvable, trying cells in random order and rolling back
// any removal that breaks uniqueness (mirrors loopy.c's remove_clues). Bails
// out early (keeping the puzzle solvable, just less sparse) if it runs past
// maxStripDuration — see that constant's comment for why this is needed on
// larger grids.
func stripClues(p *Puzzle, rng *rand.Rand) *Puzzle {
	deadline := time.Now().Add(maxStripDuration)
	order := rng.Perm(p.W * p.H)
	for _, idx := range order {
		if time.Now().After(deadline) {
			break
		}
		y, x := idx/p.W, idx%p.W
		saved := p.Clue[y][x]
		if saved < 0 {
			continue
		}
		p.Clue[y][x] = -1
		if Solve(p) != SolveUnique {
			p.Clue[y][x] = saved
		}
	}
	return p
}

// randomLoop grows a random single closed loop on the W x H dot grid using a
// self-avoiding random walk that closes back on itself, retried a bounded
// number of times. Returns ok=false if it fails to close within the attempt
// budget (caller retries with a new random loop).
func randomLoop(W, H int, rng *rand.Rand) (hEdge, vEdge [][]bool, ok bool) {
	hEdge = make([][]bool, H+1)
	for y := range hEdge {
		hEdge[y] = make([]bool, W)
	}
	vEdge = make([][]bool, H)
	for y := range vEdge {
		vEdge[y] = make([]bool, W+1)
	}

	type pt struct{ x, y int }
	// visited dots for this walk attempt.
	visited := map[pt]bool{}

	start := pt{rng.Intn(W + 1), rng.Intn(H + 1)}
	path := []pt{start}
	visited[start] = true

	minLen := (W + H) // require a reasonably sized loop, not a tiny 4-edge box
	maxSteps := (W + 1) * (H + 1) * 4

	cur := start
	for step := 0; step < maxSteps; step++ {
		// Gather candidate moves: to an unvisited neighbour dot, OR back to
		// start if the path is long enough to close the loop.
		type move struct {
			nx, ny  int
			isH     bool
			ex, ey  int
			toStart bool
		}
		var moves []move
		dirs := []struct{ dx, dy int }{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
		for _, d := range dirs {
			nx, ny := cur.x+d.dx, cur.y+d.dy
			if nx < 0 || nx > W || ny < 0 || ny > H {
				continue
			}
			var isH bool
			var ex, ey int
			if d.dy == 0 {
				isH = true
				ex = min(cur.x, nx)
				ey = cur.y
			} else {
				isH = false
				ex = cur.x
				ey = min(cur.y, ny)
			}
			if isH && hEdge[ey][ex] {
				continue
			}
			if !isH && vEdge[ey][ex] {
				continue
			}
			np := pt{nx, ny}
			if np == start && len(path) >= minLen {
				moves = append(moves, move{nx, ny, isH, ex, ey, true})
				continue
			}
			if visited[np] {
				continue
			}
			moves = append(moves, move{nx, ny, isH, ex, ey, false})
		}
		if len(moves) == 0 {
			return nil, nil, false // dead end; caller retries
		}
		// Prefer closing the loop with some probability once long enough, to
		// avoid overly long walks; otherwise pick uniformly among options.
		var chosen move
		closable := -1
		for i, m := range moves {
			if m.toStart {
				closable = i
			}
		}
		if closable >= 0 && (len(path) > minLen+rng.Intn(minLen+1) || rng.Intn(4) == 0) {
			chosen = moves[closable]
		} else {
			chosen = moves[rng.Intn(len(moves))]
		}
		if chosen.isH {
			hEdge[chosen.ey][chosen.ex] = true
		} else {
			vEdge[chosen.ey][chosen.ex] = true
		}
		if chosen.toStart {
			return hEdge, vEdge, true
		}
		cur = pt{chosen.nx, chosen.ny}
		path = append(path, cur)
		visited[cur] = true
	}
	return nil, nil, false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// borderLoop is the simplest possible loop: the outer rectangle of the grid.
// Used only as an absolute last-resort fallback.
func borderLoop(W, H int) (hEdge, vEdge [][]bool) {
	hEdge = make([][]bool, H+1)
	for y := range hEdge {
		hEdge[y] = make([]bool, W)
		if y == 0 || y == H {
			for x := 0; x < W; x++ {
				hEdge[y][x] = true
			}
		}
	}
	vEdge = make([][]bool, H)
	for y := range vEdge {
		vEdge[y] = make([]bool, W+1)
		vEdge[y][0] = true
		vEdge[y][W] = true
	}
	return hEdge, vEdge
}
