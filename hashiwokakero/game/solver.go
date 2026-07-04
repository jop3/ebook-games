package game

// solver.go: bounded backtracking with degree pruning, used both to certify
// unique solvability during generation and to validate puzzles. A naive
// "try 0/1/2 for every neighbour pair" blows up combinatorially on larger
// boards, so we (a) branch pair-by-pair with immediate degree-bound pruning
// (an island can never exceed its Need, and never fall short of Need given
// remaining undecided pairs), and (b) cap total recursive calls so generation
// always terminates quickly even on a pathological layout.

// SolveResult reports the outcome of solving.
type SolveResult int

const (
	SolveUnique SolveResult = iota
	SolveStuck
	SolveContradiction
)

type solvePair struct {
	i, j int
}

type solveState struct {
	p       *Puzzle
	n       int
	pairs   []solvePair
	count   []int // parallel to pairs: current assigned bridge count, -1 = undecided
	degree  []int // current degree per island (sum of decided pair counts)
	calls   int
	callCap int
}

func newSolveState(p *Puzzle) *solveState {
	n := len(p.Islands)
	seen := map[[2]int]bool{}
	var pairs []solvePair
	for i := range p.Islands {
		for _, j := range p.NeighbourList(i) {
			k := pairKey(i, j)
			if !seen[k] {
				seen[k] = true
				pairs = append(pairs, solvePair{k[0], k[1]})
			}
		}
	}
	cnt := make([]int, len(pairs))
	return &solveState{p: p, n: n, pairs: pairs, count: cnt, degree: make([]int, n), callCap: 400000}
}

func (s *solveState) connectedFinal() bool {
	if s.n == 0 {
		return true
	}
	adj := make([][]int, s.n)
	for idx, c := range s.count {
		if c == 0 {
			continue
		}
		pr := s.pairs[idx]
		adj[pr.i] = append(adj[pr.i], pr.j)
		adj[pr.j] = append(adj[pr.j], pr.i)
	}
	seen := make([]bool, s.n)
	stack := []int{0}
	seen[0] = true
	total := 1
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, nb := range adj[cur] {
			if !seen[nb] {
				seen[nb] = true
				total++
				stack = append(stack, nb)
			}
		}
	}
	return total == s.n
}

// remainingCapAt returns the max additional bridges island i could still get
// from undecided pairs (idx >= from), i.e. pairs not yet visited.
func (s *solveState) remainingCapAt(i, from int) int {
	r := 0
	for k := from; k < len(s.pairs); k++ {
		pr := s.pairs[k]
		if pr.i == i || pr.j == i {
			r += 2
		}
	}
	return r
}

// rec assigns a bridge count to pairs[idx..] in order, pruning on degree
// bounds and crossings, counting full solutions up to limit.
func (s *solveState) rec(idx, limit int) int {
	s.calls++
	if s.calls > s.callCap {
		return limit // bail out treating as "too many" so caller sees Stuck, not a false Unique
	}
	if idx == len(s.pairs) {
		for i, isl := range s.p.Islands {
			if s.degree[i] != isl.Need {
				return 0
			}
		}
		if !s.connectedFinal() {
			return 0
		}
		return 1
	}
	pr := s.pairs[idx]
	total := 0
	for c := 0; c <= 2; c++ {
		if c > 0 && s.crossesAny2(idx, c) {
			continue
		}
		ni := s.degree[pr.i] + c
		nj := s.degree[pr.j] + c
		if ni > s.p.Islands[pr.i].Need || nj > s.p.Islands[pr.j].Need {
			continue
		}
		if ni+s.remainingCapAt(pr.i, idx+1) < s.p.Islands[pr.i].Need {
			continue
		}
		if nj+s.remainingCapAt(pr.j, idx+1) < s.p.Islands[pr.j].Need {
			continue
		}
		s.count[idx] = c
		s.degree[pr.i] += c
		s.degree[pr.j] += c
		total += s.rec(idx+1, limit-total)
		s.degree[pr.i] -= c
		s.degree[pr.j] -= c
		s.count[idx] = 0
		if total >= limit {
			return total
		}
	}
	return total
}

// crossesAny2 checks pair idx (about to be set to a nonzero count) against
// all OTHER pairs with index < idx that already carry a nonzero count.
func (s *solveState) crossesAny2(idx, c int) bool {
	pr := s.pairs[idx]
	for k := 0; k < idx; k++ {
		if s.count[k] == 0 {
			continue
		}
		other := s.pairs[k]
		if crosses(s.p, pr.i, pr.j, other.i, other.j) {
			return true
		}
	}
	return false
}

// CountSolutions solves via bounded, pruned backtracking, returning up to
// `limit` solutions found (we only ever need 0, 1, or >=2 to distinguish
// unique from ambiguous).
func CountSolutions(p *Puzzle, limit int) int {
	s := newSolveState(p)
	return s.rec(0, limit)
}

// SolveBridges returns a solution's bridge counts, keyed by pairKey(i,j) ->
// count (1 or 2), for use in revealing/verifying the intended solution. The
// second return is false if no solution is found within the call cap. It uses
// the same degree/crossing pruning as the counter, stopping at the first full
// solution.
func SolveBridges(p *Puzzle) (map[[2]int]int, bool) {
	s := newSolveState(p)
	if !s.findFirst(0) {
		return nil, false
	}
	out := map[[2]int]int{}
	for idx, pr := range s.pairs {
		if s.count[idx] > 0 {
			out[pairKey(pr.i, pr.j)] = s.count[idx]
		}
	}
	return out, true
}

// findFirst is rec's early-exit twin: it stops at the first complete, connected
// solution and leaves s.count holding that assignment.
func (s *solveState) findFirst(idx int) bool {
	s.calls++
	if s.calls > s.callCap {
		return false
	}
	if idx == len(s.pairs) {
		for i, isl := range s.p.Islands {
			if s.degree[i] != isl.Need {
				return false
			}
		}
		return s.connectedFinal()
	}
	pr := s.pairs[idx]
	for c := 0; c <= 2; c++ {
		if c > 0 && s.crossesAny2(idx, c) {
			continue
		}
		ni := s.degree[pr.i] + c
		nj := s.degree[pr.j] + c
		if ni > s.p.Islands[pr.i].Need || nj > s.p.Islands[pr.j].Need {
			continue
		}
		if ni+s.remainingCapAt(pr.i, idx+1) < s.p.Islands[pr.i].Need {
			continue
		}
		if nj+s.remainingCapAt(pr.j, idx+1) < s.p.Islands[pr.j].Need {
			continue
		}
		s.count[idx] = c
		s.degree[pr.i] += c
		s.degree[pr.j] += c
		if s.findFirst(idx + 1) {
			return true // leave s.count intact so the caller can read the solution
		}
		s.degree[pr.i] -= c
		s.degree[pr.j] -= c
		s.count[idx] = 0
	}
	return false
}

// Solve reports SolveUnique/SolveStuck/SolveContradiction.
func Solve(p *Puzzle) SolveResult {
	n := CountSolutions(p, 2)
	switch n {
	case 0:
		return SolveContradiction
	case 1:
		return SolveUnique
	default:
		return SolveStuck
	}
}
