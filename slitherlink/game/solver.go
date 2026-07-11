package game

// solver.go implements a constraint-propagation + bounded-backtracking solver
// used to certify that a generated puzzle has exactly one solution. It
// mutates a single shared edge grid in place and undoes changes on
// backtrack (via a change log), which is far cheaper than cloning the whole
// grid at every search node — important since stripClues calls this once per
// candidate clue removal.

// SolveResult reports how many solutions a puzzle has (capped at 2, mirroring
// the einstein-style "0/1/>1, early abort" approach).
type SolveResult int

const (
	SolveNone SolveResult = iota
	SolveUnique
	SolveMultiple
)

type edgeRef struct {
	isH  bool
	x, y int
}

// solverState holds the propagation working set for one solve attempt. All
// edges live in a single flat slice indexed via edgeIndex so undo logging is
// a simple (index,oldValue) pair.
type solverState struct {
	W, H  int
	clue  [][]int
	hEdge []EdgeState // flattened [y*W+x], y in 0..H, x in 0..W-1
	vEdge []EdgeState // flattened [y*(W+1)+x], y in 0..H-1, x in 0..W

	nodes int // shared search-node counter for this solve
}

func newSolverState(p *Puzzle) *solverState {
	s := &solverState{W: p.W, H: p.H, clue: p.Clue}
	s.hEdge = make([]EdgeState, (p.H+1)*p.W)
	s.vEdge = make([]EdgeState, p.H*(p.W+1))
	return s
}

func (s *solverState) hIdx(x, y int) int { return y*s.W + x }
func (s *solverState) vIdx(x, y int) int { return y*(s.W+1) + x }

func (s *solverState) get(isH bool, x, y int) EdgeState {
	if isH {
		return s.hEdge[s.hIdx(x, y)]
	}
	return s.vEdge[s.vIdx(x, y)]
}

func (s *solverState) set(isH bool, x, y int, v EdgeState) {
	if isH {
		s.hEdge[s.hIdx(x, y)] = v
	} else {
		s.vEdge[s.vIdx(x, y)] = v
	}
}

// undoEntry records a single edge assignment so it can be reverted.
type undoEntry struct {
	isH  bool
	x, y int
	old  EdgeState
}

// setLogged assigns an edge and appends an undo entry, but only if the edge
// is currently Unknown (propagation never overwrites a decided edge; a
// mismatch is a contradiction the caller already checks via propagate's bool
// returns at the call sites that matter).
func (s *solverState) setLogged(e edgeRef, v EdgeState, log *[]undoEntry) {
	old := s.get(e.isH, e.x, e.y)
	*log = append(*log, undoEntry{e.isH, e.x, e.y, old})
	s.set(e.isH, e.x, e.y, v)
}

func (s *solverState) undo(log []undoEntry) {
	for i := len(log) - 1; i >= 0; i-- {
		e := log[i]
		s.set(e.isH, e.x, e.y, e.old)
	}
}

// cellEdges returns the 4 edges bordering cell (x,y).
func cellEdges(x, y int) [4]edgeRef {
	return [4]edgeRef{{true, x, y}, {true, x, y + 1}, {false, x, y}, {false, x + 1, y}}
}

// dotEdges returns the up-to-4 edges touching dot (x,y).
func dotEdges(W, H, x, y int) []edgeRef {
	var refs []edgeRef
	if x > 0 {
		refs = append(refs, edgeRef{true, x - 1, y})
	}
	if x < W {
		refs = append(refs, edgeRef{true, x, y})
	}
	if y > 0 {
		refs = append(refs, edgeRef{false, x, y - 1})
	}
	if y < H {
		refs = append(refs, edgeRef{false, x, y})
	}
	return refs
}

// propagate applies local deduction rules to a fixed point, logging every
// edge it sets so the caller can undo them all in one shot on backtrack.
// Returns false if a contradiction is found (caller must still undo using
// the returned log up to the point of failure — propagate always appends
// what it changed before detecting the conflict on a *later* edge, so a
// partial log is safe to undo).
func (s *solverState) propagate(log *[]undoEntry) bool {
	changed := true
	for changed {
		changed = false

		// Cell clue rules.
		for y := 0; y < s.H; y++ {
			for x := 0; x < s.W; x++ {
				c := s.clue[y][x]
				if c < 0 {
					continue
				}
				edges := cellEdges(x, y)
				on, off, unk := 0, 0, 0
				var unkRefs [4]edgeRef
				nUnk := 0
				for _, e := range edges {
					switch s.get(e.isH, e.x, e.y) {
					case EdgeOn:
						on++
					case EdgeOff:
						off++
					default:
						unkRefs[nUnk] = e
						nUnk++
						unk++
					}
				}
				if on > c || off > 4-c {
					return false
				}
				if on == c && unk > 0 {
					for i := 0; i < nUnk; i++ {
						s.setLogged(unkRefs[i], EdgeOff, log)
					}
					changed = true
				} else if 4-off == c && unk > 0 {
					for i := 0; i < nUnk; i++ {
						s.setLogged(unkRefs[i], EdgeOn, log)
					}
					changed = true
				}
			}
		}

		// Dot degree rules: every dot has ON-degree 0 or 2.
		for y := 0; y <= s.H; y++ {
			for x := 0; x <= s.W; x++ {
				refs := dotEdges(s.W, s.H, x, y)
				on, unk := 0, 0
				var unkRefs [4]edgeRef
				nUnk := 0
				for _, e := range refs {
					switch s.get(e.isH, e.x, e.y) {
					case EdgeOn:
						on++
					case EdgeUnknown:
						unkRefs[nUnk] = e
						nUnk++
						unk++
					}
				}
				if on > 2 {
					return false
				}
				if on == 2 && unk > 0 {
					for i := 0; i < nUnk; i++ {
						s.setLogged(unkRefs[i], EdgeOff, log)
					}
					changed = true
				} else if on == 1 && unk == 1 {
					for i := 0; i < nUnk; i++ {
						s.setLogged(unkRefs[i], EdgeOn, log)
					}
					changed = true
				} else if on == 0 && unk == 1 {
					for i := 0; i < nUnk; i++ {
						s.setLogged(unkRefs[i], EdgeOff, log)
					}
					changed = true
				}
			}
		}
	}
	return true
}

// maxSearchNodes bounds the total number of backtracking branch points a
// single Solve call may explore. Without a bound, a loosely-clued large grid
// (few givens left after stripping) can blow up combinatorially, since
// propagation alone doesn't fully resolve sparse regions. When the budget is
// exhausted we treat the result as "at least cap" — the safe direction for a
// uniqueness certifier: reject rather than wrongly accept a puzzle that we
// simply couldn't finish checking.
const maxSearchNodes = 15000

// countSolutions performs bounded backtracking search with propagation,
// counting solutions up to a cap of 2 (then aborting early). It mutates s in
// place and fully undoes its own changes before returning, so the caller's
// state is unchanged regardless of the result.
func (s *solverState) countSolutions(cap int) int {
	var log []undoEntry
	ok := s.propagate(&log)
	if !ok {
		s.undo(log)
		return 0
	}
	s.nodes++
	if s.nodes > maxSearchNodes {
		s.undo(log)
		return cap // budget exhausted: treat as non-unique, the safe default
	}
	bx, by, bIsH, found := s.pickBranchEdge()
	if !found {
		result := 0
		if IsSingleLoopFlat(s) && s.boardClueOK() {
			result = 1
		}
		s.undo(log)
		return result
	}

	total := 0
	for _, try := range [2]EdgeState{EdgeOn, EdgeOff} {
		s.set(bIsH, bx, by, try)
		total += s.countSolutions(cap - total)
		s.set(bIsH, bx, by, EdgeUnknown)
		if total >= cap {
			break
		}
	}
	s.undo(log)
	return total
}

// pickBranchEdge chooses the next unknown edge to branch on: prefer an edge
// bordering a clued cell (propagation prunes those fastest, keeping the
// search tree far shallower than raster-order branching on sparsely-clued
// grids); fall back to the first unknown edge in raster order.
func (s *solverState) pickBranchEdge() (bx, by int, bIsH bool, found bool) {
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
			if s.clue[y][x] < 0 {
				continue
			}
			for _, e := range cellEdges(x, y) {
				if s.get(e.isH, e.x, e.y) == EdgeUnknown {
					return e.x, e.y, e.isH, true
				}
			}
		}
	}
	for y := 0; y <= s.H; y++ {
		for x := 0; x < s.W; x++ {
			if s.hEdge[s.hIdx(x, y)] == EdgeUnknown {
				return x, y, true, true
			}
		}
	}
	for y := 0; y < s.H; y++ {
		for x := 0; x <= s.W; x++ {
			if s.vEdge[s.vIdx(x, y)] == EdgeUnknown {
				return x, y, false, true
			}
		}
	}
	return 0, 0, false, false
}

func (s *solverState) boardClueOK() bool {
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
			c := s.clue[y][x]
			if c < 0 {
				continue
			}
			n := 0
			for _, e := range cellEdges(x, y) {
				if s.get(e.isH, e.x, e.y) == EdgeOn {
					n++
				}
			}
			if n != c {
				return false
			}
		}
	}
	return true
}

// IsSingleLoopFlat adapts the flat solverState edges to the [][]EdgeState
// shape IsSingleLoop expects. Used only at full-assignment leaves, so the
// conversion cost is negligible relative to the search itself.
func IsSingleLoopFlat(s *solverState) bool {
	hEdge := make([][]EdgeState, s.H+1)
	for y := range hEdge {
		hEdge[y] = make([]EdgeState, s.W)
		copy(hEdge[y], s.hEdge[y*s.W:(y+1)*s.W])
	}
	vEdge := make([][]EdgeState, s.H)
	for y := range vEdge {
		vEdge[y] = make([]EdgeState, s.W+1)
		copy(vEdge[y], s.vEdge[y*(s.W+1):(y+1)*(s.W+1)])
	}
	return IsSingleLoop(hEdge, vEdge, s.W, s.H)
}

// Solve returns whether the puzzle (as given, with only its clues fixed and
// all edges unknown) has zero, exactly one, or multiple solutions.
func Solve(p *Puzzle) SolveResult {
	s := newSolverState(p)
	n := s.countSolutions(2)
	switch n {
	case 0:
		return SolveNone
	case 1:
		return SolveUnique
	default:
		return SolveMultiple
	}
}
