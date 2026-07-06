package game

import "sort"

// ai.go: a compact negamax (alpha-beta) AI. Breakthrough is small and
// well-studied, so a genuinely strong opponent is cheap here: evaluation is
// material plus an advancement term that rewards pawns closer to their goal
// rank, with a much larger bonus for pawns that are "unopposed" — no enemy
// pawn occupies a file (or an adjacent file) anywhere between the pawn and
// its goal, meaning no enemy pawn can ever step in front of or diagonally
// capture it on the way there. An unopposed pawn near its goal is close to an
// unstoppable win, so it should dominate the evaluation of that position —
// exactly the "much more valuable" emphasis the spec calls for.
//
// Difficulty is passed directly as search depth, matching hasami/othello:
//
//	Lätt (easy) = DepthEasy (3), Medel (medium) = DepthMedium (5),
//	Svår (hard) = DepthHard (7). The small 8x6 board and simple 3-destination
//	move rule keep even 7-8 ply comfortably fast.
const (
	DepthEasy   = 3
	DepthMedium = 5
	DepthHard   = 7
)

const (
	negInf   = -1 << 30
	posInf   = 1 << 30
	winScore = 1 << 20
)

const (
	materialWeight = 1000
	advanceWeight  = 15
	mobilityWeight = 2
)

// isUnopposed reports whether no enemy pawn stands anywhere in the "cone" a
// pawn of side at (x,y) must cross to reach its goal row: every cell in
// columns x-1..x+1 (clipped to the board) from one step ahead of y through
// the goal row inclusive. If the cone is clear, no enemy pawn can ever move
// in front of this pawn (blocking its straight advance) or land beside it to
// capture it diagonally, so the pawn is unstoppable barring the opponent
// bringing a piece in from the side — which this heuristic, matching the
// spec's "no enemy pawn able to intercept" framing, treats as already ruled
// out once the cone is empty.
func isUnopposed(b *Board, side Cell, x, y int) bool {
	enemy := side.Opponent()
	dy := ForwardDY(side)
	goal := GoalRow(side)
	yy := y
	for yy != goal {
		yy += dy
		for xx := x - 1; xx <= x+1; xx++ {
			if xx < 0 || xx >= Cols {
				continue
			}
			if b.At(xx, yy) == enemy {
				return false
			}
		}
	}
	return true
}

// pawnScore values one pawn of side at (x,y): a linear reward for distance
// already advanced off the home rank, plus — only if the pawn is unopposed —
// a quadratically growing bonus so that an unopposed pawn deep in enemy
// territory dwarfs ordinary material/positional terms, reflecting how close
// it is to an unstoppable win.
func pawnScore(b *Board, side Cell, x, y int) int {
	adv := (Rows - 1) - absInt(GoalRow(side)-y)
	s := advanceWeight * adv
	if isUnopposed(b, side, x, y) {
		s += 30 * adv * adv
	}
	return s
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// advanceScore sums pawnScore over every pawn of side on the board.
func advanceScore(b *Board, side Cell) int {
	total := 0
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			if b.At(x, y) == side {
				total += pawnScore(b, side, x, y)
			}
		}
	}
	return total
}

// evaluate scores the board from toMove's perspective (higher = better for
// toMove). Material dominates ordinary trades; the advancement/unopposed
// terms are what make the AI actually race pawns forward instead of just
// trading material evenly.
func evaluate(b *Board, toMove Cell) int {
	opp := toMove.Opponent()
	material := (b.Count(toMove) - b.Count(opp)) * materialWeight
	adv := advanceScore(b, toMove) - advanceScore(b, opp)
	mobility := len(b.LegalMoves(toMove)) - len(b.LegalMoves(opp))
	return material + adv + mobilityWeight*mobility
}

// orderedMoves returns side's legal moves ranked for search: captures first,
// with all moves further broken by how much closer to the goal the
// destination is than the origin — good move ordering is what makes
// alpha-beta pruning effective, and in Breakthrough's opening (no captures
// available at all) the advancement tiebreak is what keeps the search from
// degrading to an unordered, much slower full-width minimax.
func orderedMoves(b *Board, side Cell) []Move {
	moves := b.LegalMoves(side)
	goal := GoalRow(side)
	type scored struct {
		m Move
		s int
	}
	list := make([]scored, len(moves))
	for i, m := range moves {
		s := 0
		if m.Capture {
			s += 10000
		}
		fromDist := absInt(goal - m.From.Y)
		toDist := absInt(goal - m.To.Y)
		s += fromDist - toDist // positive: moved closer to goal
		list[i] = scored{m, s}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].s > list[j].s })
	out := make([]Move, len(list))
	for i, sc := range list {
		out[i] = sc.m
	}
	return out
}

// BestMove returns the AI's chosen move for side at the given search depth.
// ok is false only if side has no legal move at all.
func BestMove(b Board, side Cell, depth int) (Move, bool) {
	moves := orderedMoves(&b, side)
	if len(moves) == 0 {
		return Move{}, false
	}
	if depth < 1 {
		depth = 1
	}
	opp := side.Opponent()

	// Fast path: if any move wins immediately (reaches the goal rank, or
	// captures the opponent's last pawn), take it without searching deeper.
	for _, m := range moves {
		nb := b.Apply(m)
		if Winner(&nb) == side {
			return m, true
		}
	}

	best := negInf
	var bestMove Move
	found := false
	alpha, beta := negInf, posInf
	for _, m := range moves {
		nb := b.Apply(m)
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

// negamax searches to the given depth from toMove's perspective. A position
// already won or lost (per Winner, evaluated before generating moves) is
// scored as an immediate terminal result, with deeper wins/losses scored
// slightly better/worse than shallower ones so the search prefers the
// fastest win and the slowest loss. A side with zero legal moves also loses
// immediately (Breakthrough defines no pass).
func negamax(b *Board, toMove Cell, depth, alpha, beta int) int {
	opp := toMove.Opponent()
	if w := Winner(b); w != Empty {
		if w == toMove {
			return winScore + depth
		}
		return -winScore - depth
	}
	moves := orderedMoves(b, toMove)
	if len(moves) == 0 {
		return -winScore - depth
	}
	if depth == 0 {
		return evaluate(b, toMove)
	}
	best := negInf
	for _, m := range moves {
		nb := b.Apply(m)
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
