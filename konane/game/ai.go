package game

import "sort"

// ai.go: a compact negamax (alpha-beta) AI. Since jumps (captures) are the
// only action in Konane, evaluation is dominated by material — the side with
// more stones remaining is, all else equal, always ahead — with a mobility
// term (count of legal first-jumps) as a small tie-breaker. Because a side
// with zero legal jumps on its turn loses immediately, that terminal case is
// checked directly in the search rather than folded into the material score.
//
// Difficulty is passed directly as search depth, matching the other games'
// AIs (e.g. hasami):
//
//	Lätt (easy) = DepthEasy, Medel (medium) = DepthMedium, Svår (hard) = DepthHard.
//
// Each ply of the search is one full turn — i.e. one candidate entry from
// GenerateChains, which may itself be a chain of several jumps — so depth
// here means "turns ahead", not "jumps ahead".
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

// evaluate scores the board from toMove's perspective (higher = better for
// toMove). Material (stone count difference) dominates; mobility (count of
// legal first-jumps) is a small tie-breaking term.
func evaluate(b *Board, toMove Cell) int {
	opp := toMove.Opponent()
	material := (b.Count(toMove) - b.Count(opp)) * 1000
	mobility := len(b.LegalJumpsAnywhere(toMove)) - len(b.LegalJumpsAnywhere(opp))
	return material + 10*mobility
}

// orderedChains returns every legal move (chain) for side, longest first —
// a cheap move-ordering heuristic (a longer chain captures more stones, and
// material dominates evaluation) that makes alpha-beta pruning effective.
func orderedChains(b Board, side Cell) [][]Jump {
	chains := GenerateChains(b, side)
	sort.SliceStable(chains, func(i, j int) bool { return len(chains[i]) > len(chains[j]) })
	return chains
}

// BestMove returns the AI's chosen move (a complete jump chain, of whatever
// length the search judges best) for side at the given search depth. ok is
// false only if side has no legal jump at all.
func BestMove(b Board, side Cell, depth int) ([]Jump, bool) {
	chains := orderedChains(b, side)
	if len(chains) == 0 {
		return nil, false
	}
	if depth < 1 {
		depth = 1
	}
	opp := side.Opponent()

	best := negInf
	var bestChain []Jump
	found := false
	alpha, beta := negInf, posInf
	for _, c := range chains {
		nb := ApplyChain(b, side, c)
		score := -negamax(nb, opp, depth-1, -beta, -alpha)
		if !found || score > best {
			best, bestChain, found = score, c, true
		}
		if best > alpha {
			alpha = best
		}
	}
	return bestChain, found
}

// negamax searches to the given depth from toMove's perspective. A position
// where toMove has no legal jump at all is an immediate loss for toMove (deeper
// losses scored slightly better than shallower ones, so the search prefers
// delaying an unavoidable loss and hastening a forced win).
func negamax(b Board, toMove Cell, depth, alpha, beta int) int {
	opp := toMove.Opponent()
	if !b.HasAnyJump(toMove) {
		return -winScore - depth
	}
	if depth == 0 {
		return evaluate(&b, toMove)
	}
	chains := orderedChains(b, toMove)
	best := negInf
	for _, c := range chains {
		nb := ApplyChain(b, toMove, c)
		score := -negamax(nb, opp, depth-1, -beta, -alpha)
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
