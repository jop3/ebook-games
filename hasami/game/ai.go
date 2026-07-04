package game

import (
	"image"
	"sort"
)

// ai.go: a compact negamax (alpha-beta) AI. Evaluation is dominated by
// material (piece count difference), with small mobility, center/advancement
// and "hanging man" terms layered on top. Terminal states (either side down
// to a single man) are scored as immediate wins/losses inside the search
// itself, so a move that hangs a decisive capture is already avoided by
// ordinary minimax lookahead — no separate special case is needed for that.
//
// Difficulty is passed directly as search depth, matching othello's AI:
//
//	Lätt (easy) = DepthEasy (2), Medel (medium) = DepthMedium (3),
//	Svår (hard) = DepthHard (4).
const (
	DepthEasy   = 2
	DepthMedium = 3
	DepthHard   = 4
)

const (
	negInf   = -1 << 30
	posInf   = 1 << 30
	winScore = 1 << 20
)

// centerScore rewards side's men for being advanced off their home rank and
// for being near the board's center — a light positional nudge, dominated by
// material in the overall evaluation.
func centerScore(b *Board, side Cell) int {
	home := homeRank(side)
	score := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) != side {
				continue
			}
			adv := y - home
			if adv < 0 {
				adv = -adv
			}
			score += adv
			score += (4 - absInt(x-4)) + (4 - absInt(y-4))
		}
	}
	return score
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// countVulnerable counts side's men that are one enemy rook-move away from
// being custodially captured: an enemy man on one side along an axis, with
// the opposite cell on that same axis empty (so a single enemy move landing
// there would complete the sandwich). This is a cheap approximation — it does
// not verify an enemy man can actually reach that empty cell this move — used
// only as a small evaluation penalty, not a rules check.
func countVulnerable(b *Board, side Cell) int {
	enemy := side.Opponent()
	n := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) != side {
				continue
			}
			if (b.At(x-1, y) == enemy && b.At(x+1, y) == Empty) ||
				(b.At(x+1, y) == enemy && b.At(x-1, y) == Empty) ||
				(b.At(x, y-1) == enemy && b.At(x, y+1) == Empty) ||
				(b.At(x, y+1) == enemy && b.At(x, y-1) == Empty) {
				n++
			}
		}
	}
	return n
}

// evaluate scores the board from toMove's perspective (higher = better for
// toMove). Material dominates; mobility, center/advancement and vulnerability
// are small tie-breaking terms.
func evaluate(b *Board, toMove Cell) int {
	opp := toMove.Opponent()
	material := (b.Count(toMove) - b.Count(opp)) * 1000
	mobility := len(b.LegalMoves(toMove)) - len(b.LegalMoves(opp))
	center := centerScore(b, toMove) - centerScore(b, opp)
	vulnerable := countVulnerable(b, toMove) - countVulnerable(b, opp)
	return material + 5*mobility + center - 20*vulnerable
}

// quickCaptureCount reports how many enemy men move m would capture, computed
// directly against b (treating m.From as vacated and m.To as occupied by
// side) without copying the board — used only to rank moves for search
// ordering, so an approximation that skips the actual board mutation is fine.
func quickCaptureCount(b *Board, m Move, side Cell) int {
	enemy := side.Opponent()
	at := func(x, y int) Cell {
		switch {
		case x == m.From.X && y == m.From.Y:
			return Empty
		case x == m.To.X && y == m.To.Y:
			return side
		default:
			return b.At(x, y)
		}
	}
	n := 0
	for _, d := range dirs4 {
		run := 0
		x, y := m.To.X+d.X, m.To.Y+d.Y
		for inBounds(x, y) && at(x, y) == enemy {
			run++
			x += d.X
			y += d.Y
		}
		if run > 0 && inBounds(x, y) && at(x, y) == side {
			n += run
		}
	}
	for _, c := range corners {
		if at(c.X, c.Y) != enemy {
			continue
		}
		adj := cornerAdjacent(c)
		if at(adj[0].X, adj[0].Y) == side && at(adj[1].X, adj[1].Y) == side {
			n++
		}
	}
	return n
}

// moveValue is a cheap per-square positional value (advancement off the home
// rank + closeness to center) used only to break ties between non-capturing
// moves for search ordering — the same idea as centerScore, evaluated for a
// single square without scanning the whole board.
func moveValue(p image.Point, home int) int {
	adv := p.Y - home
	if adv < 0 {
		adv = -adv
	}
	return adv + (4 - absInt(p.X-4)) + (4 - absInt(p.Y-4))
}

// orderedMoves returns side's legal moves ranked for search: captures first
// (biggest first), with non-capturing moves broken by a cheap positional
// delta. Good move ordering is what makes alpha-beta pruning effective — in
// the opening, where no captures are available at all, the positional
// tiebreak is what keeps the search from degrading to an unordered (and much
// slower) full-width minimax.
func orderedMoves(b *Board, side Cell) []Move {
	moves := b.LegalMoves(side)
	home := homeRank(side)
	type scored struct {
		m Move
		s int
	}
	list := make([]scored, len(moves))
	for i, m := range moves {
		s := quickCaptureCount(b, m, side)*1000 + moveValue(m.To, home) - moveValue(m.From, home)
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

	// Fast path: if any move immediately wins (reduces the opponent to a
	// single man), take it without searching deeper.
	for _, m := range moves {
		nb, _ := b.Apply(m)
		if nb.Count(opp) <= 1 && nb.Count(side) > 1 {
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

// negamax searches to the given depth from toMove's perspective. Reaching a
// position where toMove already has <=1 man (i.e. toMove has already lost, a
// result of the opponent's prior move) or the opponent has <=1 man (toMove
// has already won) is scored as an immediate terminal result, deeper wins/
// losses scored slightly better/worse than shallower ones so the search
// prefers the fastest win and the slowest loss. A side with zero legal moves
// is treated as having lost (no pass is defined for Hasami; this is a
// defensive rule for an essentially unreachable edge case on an open board).
func negamax(b *Board, toMove Cell, depth, alpha, beta int) int {
	opp := toMove.Opponent()
	if b.Count(toMove) <= 1 {
		return -winScore - depth
	}
	if b.Count(opp) <= 1 {
		return winScore + depth
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
