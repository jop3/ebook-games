package game

import "sort"

// ai.go: a compact negamax (alpha-beta) AI. The board is tiny (4x6 = 24
// squares, at most 4 pieces per side), so a search that on a larger board
// would be far too slow is cheap here — this is the one game in the set
// where a genuinely strong AI is easy. Evaluation blends King safety (is the
// King currently attacked?), the King's race distance to the goal rank
// (reaching it wins outright), material, and mobility. Terminal states
// (King captured, or a King already sitting on its goal rank) are scored as
// immediate wins/losses inside the search itself.
//
// Difficulty is passed directly as search depth, matching this codebase's
// other 2-player+AI games (othello, hasami):
//
//	Lätt (easy) = DepthEasy, Medel (medium) = DepthMedium, Svår (hard) = DepthHard.
//
// Depth choice: with move ordering (captures — especially of the King —
// and King advances searched first), DepthHard=8 plies runs well under a
// second per move from realistic mid-game positions on this 24-square
// board (see BenchmarkBestMoveHard / the timings noted in ai_test.go), so
// the full 6-8 range the spec allows for Svår is affordable; 8 is shipped
// for the strongest practical Svår.
const (
	DepthEasy   = 2
	DepthMedium = 4
	DepthHard   = 8
)

const (
	negInf   = -1 << 30
	posInf   = 1 << 30
	winScore = 1 << 20
)

// pieceValue is a small positional weight used only for move ordering and
// the material term; King is excluded (its presence/goal-rank status is
// handled as a terminal condition, not a material count).
func pieceValue(k Kind) int {
	switch k {
	case Ex:
		return 5
	case Square:
		return 4
	case Triangle:
		return 4
	}
	return 0
}

// materialScore sums side's non-King piece values.
func materialScore(b *Board, side Side) int {
	score := 0
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			if p := b[y][x]; p != nil && p.Side == side {
				score += pieceValue(p.Kind)
			}
		}
	}
	return score
}

// kingRaceScore is higher the closer side's King is to its goal rank
// (reaching it outright wins the game), so the AI is rewarded for racing
// its King forward whenever that's not otherwise unsafe.
func kingRaceScore(b *Board, side Side) int {
	pos, ok := b.KingPos(side)
	if !ok {
		return 0 // handled as a terminal condition elsewhere
	}
	dist := absInt(goalRank(side) - pos.Y)
	return (Rows - 1) - dist
}

// kingSafety returns a penalty (<=0) if side's King is currently attacked by
// the opponent (i.e. some legal opponent move lands on the King's square).
// This is a cheap approximation of "in check", not a full lookahead.
func kingSafety(b *Board, side Side) int {
	pos, ok := b.KingPos(side)
	if !ok {
		return 0 // handled as a terminal condition elsewhere
	}
	opp := side.Opponent()
	for _, m := range b.LegalMoves(opp) {
		if m.To == pos {
			return -1
		}
	}
	return 0
}

// evaluate scores the board from toMove's perspective (higher = better for
// toMove). King safety and the King race are weighted heavily since either
// one can decide the game outright; material and mobility are smaller
// tie-breaking terms.
func evaluate(b *Board, toMove Side) int {
	opp := toMove.Opponent()
	material := materialScore(b, toMove) - materialScore(b, opp)
	mobility := len(b.LegalMoves(toMove)) - len(b.LegalMoves(opp))
	race := kingRaceScore(b, toMove) - kingRaceScore(b, opp)
	safety := kingSafety(b, toMove) - kingSafety(b, opp)
	return material*10 + mobility*3 + race*15 + safety*60
}

// orderedMoves returns side's legal moves ranked for search: capturing the
// enemy King first (an immediate win), other captures next (bigger piece
// value first), then King advances toward the goal rank, then everything
// else. Good move ordering is what makes alpha-beta pruning effective on
// this search.
func orderedMoves(b *Board, side Side) []Move {
	moves := b.LegalMoves(side)
	type scored struct {
		m Move
		s int
	}
	list := make([]scored, len(moves))
	goal := goalRank(side)
	for i, m := range moves {
		s := 0
		if target := b.At(m.To.X, m.To.Y); target != nil {
			if target.Kind == King {
				s += 100000
			} else {
				s += 1000 + pieceValue(target.Kind)*10
			}
		}
		if mover := b.At(m.From.X, m.From.Y); mover != nil && mover.Kind == King {
			before, after := absInt(goal-m.From.Y), absInt(goal-m.To.Y)
			s += (before - after) * 20
		}
		list[i] = scored{m, s}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].s > list[j].s })
	out := make([]Move, len(list))
	for i, sc := range list {
		out[i] = sc.m
	}
	return out
}

// terminalScore reports the score for toMove if the position is already
// decided (a King captured, or a King already sitting on its goal rank),
// and whether the position is in fact terminal. Deeper wins/losses score
// slightly better/worse than shallower ones so the search prefers the
// fastest win and the slowest loss.
func terminalScore(b *Board, toMove Side, depth int) (int, bool) {
	opp := toMove.Opponent()
	if _, ok := b.KingPos(toMove); !ok {
		return -winScore - depth, true
	}
	if _, ok := b.KingPos(opp); !ok {
		return winScore + depth, true
	}
	if kp, ok := b.KingPos(toMove); ok && kp.Y == goalRank(toMove) {
		return winScore + depth, true
	}
	if kp, ok := b.KingPos(opp); ok && kp.Y == goalRank(opp) {
		return -winScore - depth, true
	}
	return 0, false
}

// BestMove returns the AI's chosen move for side at the given search depth.
// ok is false only if side has no legal move at all.
func BestMove(b Board, side Side, depth int) (Move, bool) {
	moves := orderedMoves(&b, side)
	if len(moves) == 0 {
		return Move{}, false
	}
	if depth < 1 {
		depth = 1
	}
	opp := side.Opponent()

	// Fast path: if any move immediately wins (captures the enemy King, or
	// walks this side's own King onto its goal rank), take it without
	// searching deeper.
	for _, m := range moves {
		nb, _ := b.Apply(m)
		if score, ok := terminalScore(&nb, opp, depth); ok && score < 0 {
			return m, true
		}
	}

	best := negInf
	var bestMove Move
	found := false
	alpha, beta := negInf, posInf
	for _, m := range moves {
		nb, _ := b.Apply(m)
		score := -negamax(&nb, opp, depth-1, -beta, -alpha)
		if !found || score > best {
			best, bestMove, found = score, m, true
		}
		if best > alpha {
			alpha = best
		}
	}
	return bestMove, found
}

// negamax searches to the given depth from toMove's perspective.
func negamax(b *Board, toMove Side, depth, alpha, beta int) int {
	if score, ok := terminalScore(b, toMove, depth); ok {
		return score
	}
	moves := orderedMoves(b, toMove)
	if len(moves) == 0 {
		// No legal move at all: Shong defines no pass, so this is a loss for
		// toMove (same defensive rule as GameState.advance).
		return -winScore - depth
	}
	if depth == 0 {
		return evaluate(b, toMove)
	}
	opp := toMove.Opponent()
	best := negInf
	for _, m := range moves {
		nb, _ := b.Apply(m)
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
