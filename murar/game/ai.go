package game

import (
	"image"
	"sort"
)

// ai.go: a shallow minimax AI over a PRUNED move set.
//
// Naive full-width search is not viable for Quoridor: each turn offers up to
// ~5 pawn destinations plus up to WallGrid*WallGrid*2 = 128 raw wall slots.
// Instead:
//
//  1. Positions are evaluated with BFS shortest-path distance to each side's
//     goal row (opponent distance minus own distance, weighted) — this alone
//     gives a competent-feeling opponent even at depth 1.
//  2. The wall candidates considered are restricted to walls that touch the
//     OPPONENT's current shortest path (recomputed fresh, via BFS, on every
//     hypothetical board the search visits — so the search always reasons
//     about the path as it actually is after each move, not a stale one),
//     rather than enumerating all ~120 slots.
//  3. A shallow (depth 1-2) minimax runs over {pawn moves} ∪ {pruned wall
//     candidates}.
//
// This plays a reasonable, sometimes-tricky game — it is NOT a strong,
// exhaustive Quoridor engine, and the UI/rules screen say so explicitly
// ("spelar okej, inte perfekt"), the same honesty policy already used for
// this repo's goban (Go) AI.
const (
	DepthEasy   = 1
	DepthMedium = 2
	DepthHard   = 2
)

const (
	negInf   = -1 << 30
	posInf   = 1 << 30
	winScore = 1 << 20
)

// Action is one turn's worth of play: either a pawn move to To, or placing
// Wall.
type Action struct {
	IsWall bool
	To     image.Point
	Wall   Wall
}

// applyAction returns the board that results from side taking act against b.
// It does not validate legality — callers must only pass legal actions.
func applyAction(b *Board, side Side, act Action) Board {
	nb := *b
	if act.IsWall {
		nb.place(act.Wall)
		nb.WallsLeft[side]--
	} else {
		nb.Pawns[side] = act.To
	}
	return nb
}

// candidateWalls returns a pruned set of legal wall placements worth
// considering for side: every wall that would block an edge of the
// opponent's CURRENT shortest path to their goal (plus, for each such edge,
// the other wall slot that also covers that edge, since two different wall
// anchors can each block the same edge). Each candidate is verified legal
// (no overlap/cross, leaves both players a path) before inclusion.
func candidateWalls(b *Board, side Side) []Wall {
	if b.WallsLeft[side] <= 0 {
		return nil
	}
	opp := side.Opponent()
	path, ok := ShortestPath(b, b.Pawns[opp], GoalRow(opp))
	if !ok || len(path) < 2 {
		return nil
	}
	seen := make(map[Wall]bool)
	var out []Wall
	add := func(w Wall) {
		if w.X < 0 || w.X >= WallGrid || w.Y < 0 || w.Y >= WallGrid {
			return
		}
		if seen[w] {
			return
		}
		seen[w] = true
		if CanPlaceWall(b, w) {
			out = append(out, w)
		}
	}
	for i := 0; i+1 < len(path); i++ {
		a, c := path[i], path[i+1]
		switch {
		case c.X == a.X+1: // stepped right: edge blocked by a vertical wall
			add(Wall{X: a.X, Y: a.Y, Orient: Vertical})
			add(Wall{X: a.X, Y: a.Y - 1, Orient: Vertical})
		case c.X == a.X-1: // stepped left
			add(Wall{X: c.X, Y: c.Y, Orient: Vertical})
			add(Wall{X: c.X, Y: c.Y - 1, Orient: Vertical})
		case c.Y == a.Y+1: // stepped down: edge blocked by a horizontal wall
			add(Wall{X: a.X, Y: a.Y, Orient: Horizontal})
			add(Wall{X: a.X - 1, Y: a.Y, Orient: Horizontal})
		case c.Y == a.Y-1: // stepped up
			add(Wall{X: c.X, Y: c.Y, Orient: Horizontal})
			add(Wall{X: c.X - 1, Y: c.Y, Orient: Horizontal})
		}
	}
	return out
}

// candidateActions returns every pawn move plus the pruned wall candidates
// for side to consider.
func candidateActions(b *Board, side Side) []Action {
	moves := LegalPawnMoves(b, side)
	acts := make([]Action, 0, len(moves)+8)
	for _, m := range moves {
		acts = append(acts, Action{To: m})
	}
	for _, w := range candidateWalls(b, side) {
		acts = append(acts, Action{IsWall: true, Wall: w})
	}
	return acts
}

// evaluate scores board b from toMove's perspective (higher = better for
// toMove): primarily the difference in BFS shortest-path distance to goal
// (toMove closer to their goal is good, opponent closer to theirs is bad),
// with remaining wall count as a small tie-break.
func evaluate(b *Board, toMove Side) int {
	opp := toMove.Opponent()
	dSelf, okSelf := BFSDistance(b, b.Pawns[toMove], GoalRow(toMove))
	if !okSelf {
		dSelf = Size * Size // should not happen; CanPlaceWall forbids it
	}
	dOpp, okOpp := BFSDistance(b, b.Pawns[opp], GoalRow(opp))
	if !okOpp {
		dOpp = Size * Size
	}
	score := (dOpp - dSelf) * 10
	score += (b.WallsLeft[toMove] - b.WallsLeft[opp]) * 2
	return score
}

// orderActions ranks candidate actions by a shallow (one-ply) evaluation, so
// alpha-beta pruning is effective even though the search itself is shallow.
func orderActions(b *Board, side Side, acts []Action) []Action {
	type scored struct {
		a Action
		s int
	}
	list := make([]scored, len(acts))
	for i, act := range acts {
		nb := applyAction(b, side, act)
		list[i] = scored{act, evaluate(&nb, side)}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].s > list[j].s })
	out := make([]Action, len(list))
	for i, sc := range list {
		out[i] = sc.a
	}
	return out
}

// BestMove returns the AI's chosen action for side at the given search depth.
// ok is false only if side has no legal action at all (should not happen in a
// reachable Murar position).
func BestMove(b Board, side Side, depth int) (Action, bool) {
	acts := candidateActions(&b, side)
	if len(acts) == 0 {
		return Action{}, false
	}
	if depth < 1 {
		depth = 1
	}
	opp := side.Opponent()

	// Fast path: take any move that wins immediately.
	for _, act := range acts {
		if act.IsWall {
			continue
		}
		nb := applyAction(&b, side, act)
		if w, ok := Winner(&nb); ok && w == side {
			return act, true
		}
	}

	ordered := orderActions(&b, side, acts)
	best := negInf
	var bestAct Action
	found := false
	alpha, beta := negInf, posInf
	for _, act := range ordered {
		nb := applyAction(&b, side, act)
		score := -negamax(&nb, opp, depth-1, -beta, -alpha)
		if !found || score > best {
			best, bestAct, found = score, act, true
		}
		if best > alpha {
			alpha = best
		}
	}
	return bestAct, found
}

// negamax searches to the given depth from toMove's perspective over the
// pruned candidate set, alpha-beta pruned.
func negamax(b *Board, toMove Side, depth, alpha, beta int) int {
	if w, ok := Winner(b); ok {
		if w == toMove {
			return winScore + depth
		}
		return -winScore - depth
	}
	if depth <= 0 {
		return evaluate(b, toMove)
	}
	acts := candidateActions(b, toMove)
	if len(acts) == 0 {
		return evaluate(b, toMove)
	}
	opp := toMove.Opponent()
	best := negInf
	for _, act := range orderActions(b, toMove, acts) {
		nb := applyAction(b, toMove, act)
		score := -negamax(&nb, opp, depth-1, -beta, -alpha)
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
