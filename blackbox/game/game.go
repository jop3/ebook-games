// Package blackbox implements the classic Black Box ray-tracing deduction
// game. This file contains the pure game logic (grid, atoms, ray simulation,
// scoring) with NO dependency on the inkview SDK, so it can be unit-tested as
// ordinary Go on any machine.
package game

import (
	"math/rand"
)

// --- Directions ------------------------------------------------------------

// dir is a unit step on the grid. Rays always travel axis-aligned in one of
// four directions.
type dir struct{ dx, dy int }

var (
	dirRight = dir{1, 0}
	dirLeft  = dir{-1, 0}
	dirUp    = dir{0, -1}
	dirDown  = dir{0, 1}
)

// left returns the direction obtained by rotating d 90° counter-clockwise
// (a "turn to the left" from the ray's point of view). right rotates
// clockwise. On a y-down screen coordinate system, CCW is (dx,dy)->(dy,-dx).
func (d dir) left() dir  { return dir{d.dy, -d.dx} }
func (d dir) right() dir { return dir{-d.dy, d.dx} }
func (d dir) reverse() dir {
	return dir{-d.dx, -d.dy}
}

// --- Grid & atoms ----------------------------------------------------------

// Grid holds the hidden atom layout. Coordinates are (x,y) with x in [0,W),
// y in [0,H); (0,0) is the top-left cell.
type Grid struct {
	W, H  int
	atoms [][]bool // atoms[y][x]
}

// NewGrid returns an empty W×H grid.
func NewGrid(w, h int) *Grid {
	a := make([][]bool, h)
	for y := range a {
		a[y] = make([]bool, w)
	}
	return &Grid{W: w, H: h, atoms: a}
}

// inBounds reports whether (x,y) lies within the grid.
func (g *Grid) inBounds(x, y int) bool {
	return x >= 0 && x < g.W && y >= 0 && y < g.H
}

// HasAtom reports whether a cell contains an atom. Out-of-bounds cells never
// contain an atom (there is nothing beyond the edge).
func (g *Grid) HasAtom(x, y int) bool {
	if !g.inBounds(x, y) {
		return false
	}
	return g.atoms[y][x]
}

// SetAtom places or clears an atom at (x,y).
func (g *Grid) SetAtom(x, y int, on bool) {
	if g.inBounds(x, y) {
		g.atoms[y][x] = on
	}
}

// AtomCount returns the number of atoms currently on the grid.
func (g *Grid) AtomCount() int {
	n := 0
	for y := range g.atoms {
		for x := range g.atoms[y] {
			if g.atoms[y][x] {
				n++
			}
		}
	}
	return n
}

// Atoms returns the list of atom coordinates, for scoring/answer checking.
func (g *Grid) Atoms() []Cell {
	var out []Cell
	for y := range g.atoms {
		for x := range g.atoms[y] {
			if g.atoms[y][x] {
				out = append(out, Cell{x, y})
			}
		}
	}
	return out
}

// Cell is a grid coordinate.
type Cell struct{ X, Y int }

// --- Edge points -----------------------------------------------------------

// An edge point is a location on the perimeter from which a ray may be fired.
// It is identified by an index 0..2*(W+H)-1, ordered clockwise starting at the
// top edge. Each edge point knows the cell just inside the grid and the
// inward-travel direction of a ray fired from it.

// EdgePoint describes one perimeter firing position.
type EdgePoint struct {
	Index  int
	entryX int // cell just inside the grid where the ray starts
	entryY int
	inDir  dir // direction the ray travels when entering
}

// Side identifies which edge of the grid an edge point sits on. Used by the UI
// to place the marker slot outside the correct grid border.
type Side int

const (
	SideTop Side = iota
	SideRight
	SideBottom
	SideLeft
)

// EntryCell returns the interior cell (just inside the grid) associated with
// this edge point.
func (e EdgePoint) EntryCell() (x, y int) { return e.entryX, e.entryY }

// Side reports which grid border this edge point lies on, derived from its
// inward travel direction.
func (e EdgePoint) Side() Side {
	switch e.inDir {
	case dirDown:
		return SideTop
	case dirUp:
		return SideBottom
	case dirLeft:
		return SideRight
	default: // dirRight
		return SideLeft
	}
}

// EdgePoints returns all firing positions for a grid, in a stable clockwise
// order: top row left→right, right column top→bottom, bottom row right→left,
// left column bottom→top.
func (g *Grid) EdgePoints() []EdgePoint {
	var pts []EdgePoint
	idx := 0
	add := func(ex, ey int, d dir) {
		pts = append(pts, EdgePoint{Index: idx, entryX: ex, entryY: ey, inDir: d})
		idx++
	}
	// Top edge: enter going down.
	for x := 0; x < g.W; x++ {
		add(x, 0, dirDown)
	}
	// Right edge: enter going left.
	for y := 0; y < g.H; y++ {
		add(g.W-1, y, dirLeft)
	}
	// Bottom edge: enter going up.
	for x := g.W - 1; x >= 0; x-- {
		add(x, g.H-1, dirUp)
	}
	// Left edge: enter going right.
	for y := g.H - 1; y >= 0; y-- {
		add(0, y, dirRight)
	}
	return pts
}

// edgePointAt returns the EdgePoint whose interior cell is (x,y) and whose
// inward direction is d. Used to map a ray's exit back to an edge index.
func (g *Grid) edgePointAt(x, y int, d dir) (EdgePoint, bool) {
	for _, ep := range g.EdgePoints() {
		if ep.entryX == x && ep.entryY == y && ep.inDir == d {
			return ep, true
		}
	}
	return EdgePoint{}, false
}

// --- Ray simulation --------------------------------------------------------

// RayOutcome classifies the result of firing a ray.
type RayOutcome int

const (
	// OutcomeHit: the ray was absorbed by an atom and never emerged.
	OutcomeHit RayOutcome = iota
	// OutcomeReflection: the ray emerged at the same edge point it entered.
	OutcomeReflection
	// OutcomeDetour: the ray emerged at a different edge point (ExitIndex set).
	OutcomeDetour
)

// RayResult is the outcome of firing a ray from a given edge point.
type RayResult struct {
	EntryIndex int
	Outcome    RayOutcome
	ExitIndex  int // valid only when Outcome == OutcomeDetour
}

// Fire simulates a ray entering at the given edge point and returns the
// result. The simulation follows the classic Black Box rules:
//
//   - Look at the cell directly in front and the two diagonally-forward cells.
//   - HIT: an atom directly in front → absorbed.
//   - Both diagonals occupied → reflect straight back.
//   - One diagonal occupied → deflect 90° away from that atom.
//   - Otherwise advance one cell.
//
// The edge special case (an atom diagonally adjacent to the very entry point,
// which would immediately bounce the ray back out) is handled by running the
// same interaction check from a "virtual" cell one step outside the grid before
// the first real step; if that check reflects, the ray never enters.
func (g *Grid) Fire(ep EdgePoint) RayResult {
	// Start position: one cell OUTSIDE the grid, so the very first interaction
	// check happens as the ray is about to enter. This is what produces the
	// edge-reflection special case naturally.
	pos := Cell{ep.entryX - ep.inDir.dx, ep.entryY - ep.inDir.dy}
	d := ep.inDir

	// Guard against pathological infinite loops (should never trigger with
	// valid physics, but bounds the simulation defensively).
	maxSteps := (g.W + g.H) * 4 * 8
	for step := 0; step < maxSteps; step++ {
		frontX, frontY := pos.X+d.dx, pos.Y+d.dy

		// If the cell directly in front holds an atom → hit/absorbed. This can
		// only happen once the ray is inside (an atom can never be outside the
		// grid), so the entry step never registers a hit.
		if g.HasAtom(frontX, frontY) {
			return RayResult{EntryIndex: ep.Index, Outcome: OutcomeHit}
		}

		// Diagonally-forward cells: front cell offset by ±perpendicular.
		lp := d.left()  // "left-forward" diagonal offset direction
		rp := d.right() // "right-forward" diagonal offset direction
		atomLeft := g.HasAtom(frontX+lp.dx, frontY+lp.dy)
		atomRight := g.HasAtom(frontX+rp.dx, frontY+rp.dy)

		switch {
		case atomLeft && atomRight:
			// Both diagonals: reflect straight back.
			d = d.reverse()
		case atomLeft:
			// Atom on the left-forward diagonal → turn away (to the right).
			d = d.right()
		case atomRight:
			// Atom on the right-forward diagonal → turn away (to the left).
			d = d.left()
		default:
			// No interaction: advance one cell.
			pos = Cell{frontX, frontY}
		}

		// After updating, check whether the ray has left the grid. It has left
		// if its current cell is outside the grid AND it is heading further out
		// (i.e. it emerged at an edge). We detect emergence by looking at the
		// cell the ray now occupies: if it is outside bounds, resolve the exit.
		if !g.inBounds(pos.X, pos.Y) {
			return g.resolveExit(ep, pos, d)
		}
	}
	// Defensive fallback: treat as reflection.
	return RayResult{EntryIndex: ep.Index, Outcome: OutcomeReflection}
}

// resolveExit maps a ray that has stepped outside the grid to an outcome.
// pos is the (out-of-bounds) cell the ray now occupies and d its direction.
// The exit edge point is the interior cell it just came from, paired with the
// reverse of the outward direction (i.e. the inward direction of that edge).
func (g *Grid) resolveExit(entry EdgePoint, pos Cell, d dir) RayResult {
	// The interior cell adjacent to the exit is one step back from pos.
	interiorX := pos.X - d.dx
	interiorY := pos.Y - d.dy
	// The inward direction of the exit edge point equals the ray's outward
	// direction (a ray leaving downward exits at a bottom edge point whose
	// inward direction is up... no: the edge point's inDir is INTO the grid,
	// which is the reverse of the outward travel d).
	exitInDir := d.reverse()
	exitEP, ok := g.edgePointAt(interiorX, interiorY, exitInDir)
	if !ok {
		// Corner/edge ambiguity fallback: nearest matching by interior cell.
		return RayResult{EntryIndex: entry.Index, Outcome: OutcomeReflection}
	}
	if exitEP.Index == entry.Index {
		return RayResult{EntryIndex: entry.Index, Outcome: OutcomeReflection}
	}
	return RayResult{
		EntryIndex: entry.Index,
		Outcome:    OutcomeDetour,
		ExitIndex:  exitEP.Index,
	}
}

// --- Random layout ---------------------------------------------------------

// PlaceRandomAtoms clears the grid and scatters n atoms at distinct random
// positions using the supplied source. If n exceeds the number of cells it is
// capped.
func (g *Grid) PlaceRandomAtoms(n int, rng *rand.Rand) {
	for y := range g.atoms {
		for x := range g.atoms[y] {
			g.atoms[y][x] = false
		}
	}
	total := g.W * g.H
	if n > total {
		n = total
	}
	perm := rng.Perm(total)
	for i := 0; i < n; i++ {
		c := perm[i]
		g.SetAtom(c%g.W, c/g.W, true)
	}
}
