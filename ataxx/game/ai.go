package game

import "sort"

// ai.go: a compact negamax (alpha-beta) AI. Evaluation is material (piece
// count difference) dominant with a small mobility term layered on top, per
// the spec ("alpha-beta with material + mobility heuristic"). The board is
// only 49 cells, so this search is comfortably strong at modest depths.
//
// Difficulty is passed directly as search depth, matching the other games in
// this repo (e.g. hasami, othello):
//
//	Lätt (easy) = DepthEasy, Medel (medium) = DepthMedium, Svår (hard) = DepthHard.
const (
	DepthEasy   = 1
	DepthMedium = 2
	DepthHard   = 3
)

const (
	negInf = -1 << 30
	posInf = 1 << 30
)

// evaluate scores the board from toMove's perspective (higher = better for
// toMove). Material dominates; mobility (the count of each side's own legal
// moves) is a small tie-breaking term. This same formula doubles as the
// "terminal" score when the game has actually ended (board full, or toMove
// has no legal move): mobility is naturally 0 in the no-legal-move case, so
// no separate win/loss special-casing is needed the way Hasami needs one —
// Ataxx's ending is just "compare piece counts", which is exactly what
// evaluate already computes.
func evaluate(b *Board, toMove Cell) int {
	opp := toMove.Opponent()
	material := (b.Count(toMove) - b.Count(opp)) * 100
	mobility := len(b.LegalMoves(toMove)) - len(b.LegalMoves(opp))
	return material + 3*mobility
}

// quickFlipCount reports how many enemy men move m would flip, computed
// directly against b (the board BEFORE the move) without mutating it: only
// the 8 neighbors of m.To matter, and since m.To is empty before the move,
// b.At on those neighbors already reflects the correct pre-move state (m.From
// can only coincide with one of them for a clone, and a clone's source holds
// side's own color, never the enemy's, so it's never miscounted). Used only
// to rank moves for search ordering.
func quickFlipCount(b *Board, m Move, side Cell) int {
	enemy := side.Opponent()
	n := 0
	for _, d := range neighbor8 {
		x, y := m.To.X+d.X, m.To.Y+d.Y
		if b.At(x, y) == enemy {
			n++
		}
	}
	return n
}

// orderedMoves returns side's legal moves ranked for search: moves that flip
// more enemy men first. Good move ordering is what makes alpha-beta pruning
// effective, especially in the opening where flips are rare.
func orderedMoves(b *Board, side Cell) []Move {
	moves := b.LegalMoves(side)
	type scored struct {
		m Move
		s int
	}
	list := make([]scored, len(moves))
	for i, m := range moves {
		s := quickFlipCount(b, m, side) * 10
		if m.IsClone() {
			s++ // mild tiebreak: prefer keeping the source occupied when equally good
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

	best := negInf
	var bestMove Move
	alpha, beta := negInf, posInf
	for _, m := range moves {
		nb, _ := b.Apply(m)
		score := -negamax(&nb, opp, depth-1, -beta, -alpha)
		if score > best {
			best, bestMove = score, m
		}
		if best > alpha {
			alpha = best
		}
	}
	return bestMove, true
}

// negamax searches to the given depth from toMove's perspective. If toMove
// has no legal move, the game has actually ended right here (see the
// GameState.advance doc comment: Ataxx as specified has no pass), so that is
// scored the same way a depth-0 cutoff would be — a plain evaluate() call —
// rather than as a forced win or loss for either side.
func negamax(b *Board, toMove Cell, depth, alpha, beta int) int {
	moves := orderedMoves(b, toMove)
	if len(moves) == 0 || depth == 0 {
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
