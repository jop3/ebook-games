package game

import (
	"image"
	"sort"
)

// ai.go: alpha-beta search using the classic Isola/Isolation evaluation —
// mobility difference (the mover's legal-move count minus the opponent's).
// A "move" in this search is a full turn (pawn move + tile removal), so the
// branching factor is destinations x remaining-tiles — far larger than a
// single-decision game like hasami (often 1000+ full turns early in the
// game). Exhaustively scoring that many candidates at every node of a
// multi-ply search is too slow, so:
//
//   - The ROOT (BestMove's own top-level choice, computed once per AI turn)
//     enumerates and scores every legal full turn exactly (orderedMoves /
//     fullMoves) for maximum move quality.
//   - Every node BELOW the root (searchMoves) uses a cheap, bounded
//     candidate generator instead: rank destinations by a 1-ply mobility
//     score (touching only the ~20 destinations, not the destination x
//     removal cross-product), keep the best few, and for each kept
//     destination rank removals by whether they take away a tile the
//     opponent could otherwise reach (a direct, O(1)-per-tile mobility hit)
//     rather than a full board Apply+evaluate per candidate. This mirrors
//     the "prune the huge candidate set down to the most promising options"
//     mitigation the Quoridor wall-move design note in
//     SPEC_STRATEGY_CANDIDATES.md describes for its own (even larger) wall-
//     placement branching factor.
//
// Difficulty is passed as search depth, matching hasami/othello's pattern.
// Because each ply here already bundles a move+removal (twice the
// information of a hasami ply), these depths search a comparable total game
// tree to hasami's DepthEasy/Medium/Hard despite the smaller numbers.
const (
	DepthEasy   = 1
	DepthMedium = 2
	DepthHard   = 3
)

// candidateDests/candidateRemovals bound searchMoves' branching at internal
// search nodes (see the package doc comment above).
const (
	candidateDests    = 6
	candidateRemovals = 10
)

const (
	negInf   = -1 << 30
	posInf   = 1 << 30
	winScore = 1 << 20
)

// evaluate scores the board from toMove's perspective (higher = better for
// toMove): the number of legal moves toMove has, minus the number the
// opponent has.
func evaluate(b *Board, toMove Side) int {
	return len(b.LegalMoves(toMove)) - len(b.LegalMoves(toMove.Opponent()))
}

// fullMoves enumerates every legal (pawn move, tile removal) pair for side —
// a full Isola turn. Used for the search root and by tests; too expensive to
// call at every internal search node (see searchMoves).
func fullMoves(b *Board, side Side) []Move {
	from := b.PawnPos(side)
	dests := b.DestinationsFrom(from)
	var moves []Move
	for _, d := range dests {
		nb := *b
		nb.setPawnPos(side, d)
		for _, r := range nb.LegalTileRemovals(d) {
			moves = append(moves, Move{Side: side, From: from, To: d, Remove: r})
		}
	}
	return moves
}

// scoredMove pairs a move with a precomputed ordering score, so sorting
// doesn't re-evaluate the resulting board on every comparison.
type scoredMove struct {
	m Move
	s int
}

// orderedMoves returns side's full-turn moves sorted best-first by a quick
// 1-ply mobility-diff score (evaluated from side's own perspective on the
// board that results from actually playing the move). Used only at the
// search root, where it runs exactly once per AI turn.
func orderedMoves(b *Board, side Side) []Move {
	moves := fullMoves(b, side)
	list := make([]scoredMove, len(moves))
	for i, m := range moves {
		nb := b.Apply(m)
		list[i] = scoredMove{m, evaluate(&nb, side)}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].s > list[j].s })
	out := make([]Move, len(list))
	for i, sc := range list {
		out[i] = sc.m
	}
	return out
}

// searchMoves returns a small, cheaply-ranked candidate list of full turns
// for side, used at every internal search node (everywhere below the root).
// It ranks the (typically ~5-25) pawn destinations by a one-touch mobility
// score and keeps only the best candidateDests, then for each kept
// destination ranks tile removals by whether they take away a tile the
// opponent could otherwise reach this turn (a direct, cheap proxy for "hurts
// their mobility") and keeps only the best candidateRemovals. This trades a
// little search accuracy for the large constant-factor speedup needed to
// search more than one ply deep.
func searchMoves(b *Board, side Side) []Move {
	from := b.PawnPos(side)
	dests := b.DestinationsFrom(from)
	if len(dests) == 0 {
		return nil
	}
	opp := side.Opponent()
	oppMoves := b.LegalMoves(opp)
	oppSet := make(map[image.Point]bool, len(oppMoves))
	for _, p := range oppMoves {
		oppSet[p] = true
	}

	type destScore struct {
		d image.Point
		s int
	}
	dscored := make([]destScore, len(dests))
	for i, d := range dests {
		nb := *b
		nb.setPawnPos(side, d)
		dscored[i] = destScore{d, len(nb.LegalMoves(side)) - len(nb.LegalMoves(opp))}
	}
	sort.SliceStable(dscored, func(i, j int) bool { return dscored[i].s > dscored[j].s })
	if len(dscored) > candidateDests {
		dscored = dscored[:candidateDests]
	}

	type remScore struct {
		r image.Point
		s int
	}
	var moves []Move
	for _, ds := range dscored {
		nb := *b
		nb.setPawnPos(side, ds.d)
		removals := nb.LegalTileRemovals(ds.d)
		rscored := make([]remScore, len(removals))
		for i, r := range removals {
			s := 0
			if oppSet[r] {
				s = 100 // removing a tile the opponent could reach right now directly shrinks their mobility
			}
			rscored[i] = remScore{r, s}
		}
		sort.SliceStable(rscored, func(i, j int) bool { return rscored[i].s > rscored[j].s })
		if len(rscored) > candidateRemovals {
			rscored = rscored[:candidateRemovals]
		}
		for _, rs := range rscored {
			moves = append(moves, Move{Side: side, From: from, To: ds.d, Remove: rs.r})
		}
	}
	return moves
}

// BestMove returns the AI's chosen full turn (move+removal) for side at the
// given search depth. ok is false only if side has no legal pawn move at all
// (side has already lost).
func BestMove(b Board, side Side, depth int) (Move, bool) {
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

// negamax searches to the given depth from toMove's perspective. Reaching a
// position where toMove has zero legal moves is scored as an immediate loss
// (deeper losses scored slightly better than shallower ones, so the search
// prefers to delay an unavoidable loss and hasten a forced win).
func negamax(b *Board, toMove Side, depth, alpha, beta int) int {
	// GameOver is a cheap check (just toMove's own destinations) — used here
	// to avoid building any move list at all when the game has already
	// ended at this node.
	if GameOver(b, toMove) {
		return -winScore - depth
	}
	if depth == 0 {
		return evaluate(b, toMove)
	}
	moves := searchMoves(b, toMove)
	opp := toMove.Opponent()
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
