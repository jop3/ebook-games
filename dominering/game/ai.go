package game

import (
	"sort"
	"time"
)

// ai.go: alpha-beta (negamax) search for Domineering, driven by ITERATIVE
// DEEPENING under a wall-clock time budget rather than a bare fixed depth.
//
// The board shrinks by exactly 2 empty cells every move and branching
// strictly decreases turn over turn — unlike most partisan games, so a
// depth that would be hopelessly slow from the opening position (dozens of
// legal moves) becomes cheap once only a handful of cells remain. Iterative
// deepening exploits this directly: it searches depth 1, then 2, then 3...
// up to the difficulty's target depth, stopping as soon as the time budget
// runs out and falling back to the last FULLY completed depth's move. Early
// in the game this typically only reaches a few plies within the budget;
// late in the game — where the spec calls for "search deep, even to the
// endgame" — the same budget reaches all the way to a true, exact solve,
// because there is so little left to search. Each iteration also re-orders
// moves with the previous iteration's best move searched first, which is
// what makes alpha-beta pruning effective at the deeper iterations.
//
// Evaluation is dominated by mobility (own legal moves minus the opponent's
// legal moves — the standard strong heuristic for this game family), with a
// small positional term that prefers interior placements over edge/corner
// ones as a tie-breaker.
//
// Difficulty selects both the target depth cap and the time budget: Lätt
// (easy) = DepthEasy, Medel (medium) = DepthMedium, Svår (hard) = DepthHard.
const (
	DepthEasy   = 3
	DepthMedium = 5
	DepthHard   = 8
)

// budgetFor maps a difficulty's target depth to how long BestMove may spend
// searching before it must return the best move found so far. Difficulties
// not in the table (shouldn't happen in practice — callers only ever pass
// the Depth* constants) fall back to the Hard budget.
func budgetFor(depth int) time.Duration {
	switch depth {
	case DepthEasy:
		return 150 * time.Millisecond
	case DepthMedium:
		return 600 * time.Millisecond
	default:
		return 2500 * time.Millisecond
	}
}

const (
	negInf   = -1 << 30
	posInf   = 1 << 30
	winScore = 1 << 20
)

// endgameExtensionFactor scales how many empty cells ahead of the requested
// target depth still count as "small enough to aim for an exact solve":
// once EmptyCount() <= depth * endgameExtensionFactor, the iterative
// deepening loop's target extends to the true end of the game instead of
// stopping at depth. The time budget (not just the depth cap) is the real
// backstop against runaway search, so this only shifts the TARGET the loop
// climbs toward — it can never itself cause an unbounded search.
const endgameExtensionFactor = 3

// cellWeight is a small positional bias: cells nearer the board's center are
// worth slightly more than edge/corner cells, since a domino placed near an
// edge is more easily pinched off by the boundary. Purely a search-ordering
// and evaluation tie-breaker, never affects legality.
func cellWeight(size int, x, y int) int {
	cx, cy := (size-1)/2, (size-1)/2
	return -(iabs(x-cx) + iabs(y-cy))
}

func iabs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// mobilityBias sums a tiny positional weight over every legal move side has
// available, favoring central over edge placements — a secondary term added
// to the dominant mobility-count evaluation below.
func mobilityBias(b *Board, side Side) int {
	score := 0
	for _, m := range b.LegalMoves(side) {
		score += cellWeight(b.Size, m.A.X, m.A.Y) + cellWeight(b.Size, m.B.X, m.B.Y)
	}
	return score
}

// evaluate scores the board from toMove's perspective (higher = better for
// toMove). The own-legal-moves-minus-opponent's-legal-moves difference
// dominates; mobilityBias is a small tie-break.
func evaluate(b *Board, toMove Side) int {
	opp := toMove.Opponent()
	mobility := len(b.LegalMoves(toMove)) - len(b.LegalMoves(opp))
	bias := mobilityBias(b, toMove) - mobilityBias(b, opp)
	return mobility*100 + bias
}

// orderedMoves ranks side's legal moves for search: for each candidate,
// apply it and score by the resulting (side's mobility - opponent's
// mobility), highest first. Good move ordering is what makes alpha-beta
// pruning effective, especially early in the game before any move is
// obviously forced.
func orderedMoves(b *Board, side Side) []Move {
	moves := b.LegalMoves(side)
	opp := side.Opponent()
	type scored struct {
		m Move
		s int
	}
	list := make([]scored, len(moves))
	for i, m := range moves {
		nb := b.Apply(m)
		list[i] = scored{m, len(nb.LegalMoves(side)) - len(nb.LegalMoves(opp))}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].s > list[j].s })
	out := make([]Move, len(list))
	for i, sc := range list {
		out[i] = sc.m
	}
	return out
}

// orderedMovesWithPreferred is orderedMoves but with a previously-found best
// move (typically the winner of the prior, shallower iterative-deepening
// pass) moved to the front — searching the likely-best move first is what
// lets alpha-beta prune hardest at the next depth.
func orderedMovesWithPreferred(b *Board, side Side, preferred Move, havePreferred bool) []Move {
	moves := orderedMoves(b, side)
	if !havePreferred {
		return moves
	}
	for i, m := range moves {
		if m == preferred {
			moves[0], moves[i] = moves[i], moves[0]
			break
		}
	}
	return moves
}

// search carries the wall-clock deadline for one BestMove call across the
// whole iterative-deepening / alpha-beta recursion. Time is only actually
// read every timeCheckInterval nodes (a time.Now() syscall on every node
// would itself be a meaningful slowdown at high node counts).
type search struct {
	deadline time.Time
	nodes    int
	aborted  bool
}

const timeCheckInterval = 2048

// timeUp reports whether the deadline has passed, checking the wall clock
// only periodically. Once aborted becomes true it stays true for the rest of
// this search — every remaining node returns immediately, so an in-progress
// (necessarily incomplete) iterative-deepening depth unwinds quickly instead
// of continuing to grind.
func (s *search) timeUp() bool {
	if s.aborted {
		return true
	}
	s.nodes++
	if s.nodes%timeCheckInterval == 0 && time.Now().After(s.deadline) {
		s.aborted = true
	}
	return s.aborted
}

// BestMove returns the AI's chosen move for side, searching iteratively
// deeper (1, 2, 3, ...) up to a target depth set by depth (see
// DepthEasy/Medium/Hard) — extended further when the endgame is close
// enough to solve exactly — and returning the move found by the deepest
// FULLY completed iteration once the difficulty's time budget runs out. ok
// is false only if side has no legal move at all (the game is already over
// for side to move).
func BestMove(b Board, side Side, depth int) (Move, bool) {
	moves := orderedMoves(&b, side)
	if len(moves) == 0 {
		return Move{}, false
	}
	if depth < 1 {
		depth = 1
	}
	target := depth
	if empty := b.EmptyCount(); empty <= depth*endgameExtensionFactor && empty > depth {
		target = empty
	}

	s := &search{deadline: time.Now().Add(budgetFor(depth))}
	best := moves[0] // a sane fallback even if depth-1 search somehow can't complete
	havePreferred := false
	for d := 1; d <= target; d++ {
		mv, ok := searchRoot(s, &b, side, d, best, havePreferred)
		if s.aborted {
			break // this depth's result is incomplete; keep the prior depth's move
		}
		if ok {
			best, havePreferred = mv, true
		}
	}
	return best, true
}

// searchRoot runs one full alpha-beta pass at the given depth from the root
// position, returning the best move found (ok is false only if side has no
// legal move, which BestMove already excludes before calling this).
func searchRoot(s *search, b *Board, side Side, depth int, preferred Move, havePreferred bool) (Move, bool) {
	moves := orderedMovesWithPreferred(b, side, preferred, havePreferred)
	opp := side.Opponent()
	best := negInf
	var bestMove Move
	found := false
	alpha, beta := negInf, posInf
	for _, m := range moves {
		if s.aborted {
			break
		}
		nb := b.Apply(m)
		score := -negamax(s, &nb, opp, depth-1, -beta, -alpha)
		if s.aborted {
			break
		}
		if !found || score > best {
			best, bestMove, found = score, m, true
		}
		if best > alpha {
			alpha = best
		}
	}
	return bestMove, found
}

// negamax searches to the given depth from toMove's perspective. A position
// where toMove has zero legal moves is a genuine, immediate loss for toMove
// under normal play convention — checked explicitly here, not assumed from
// "ran out of depth" or "last move wins" — scored so that a loss found
// sooner (larger remaining depth budget) is worse than one found only deep
// in the tree, and symmetrically for wins, so the search prefers the
// fastest win and the slowest loss.
func negamax(s *search, b *Board, toMove Side, depth, alpha, beta int) int {
	if s.timeUp() {
		return 0
	}
	if !b.HasMove(toMove) {
		return -winScore - depth
	}
	if depth == 0 {
		return evaluate(b, toMove)
	}
	opp := toMove.Opponent()
	moves := orderedMoves(b, toMove)
	best := negInf
	for _, m := range moves {
		if s.aborted {
			break
		}
		nb := b.Apply(m)
		score := -negamax(s, &nb, opp, depth-1, -beta, -alpha)
		if score > best {
			best = score
		}
		if best > alpha {
			alpha = best
		}
		if alpha >= beta {
			break
		}
	}
	return best
}
