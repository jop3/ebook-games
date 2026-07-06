package game

import (
	"image"
	"math/rand"
)

// ai.go: a deliberately weak, exploratory Amazons AI — NOT a deep search.
//
// The branching factor for a full Amazons turn (move a queen, then shoot an
// arrow from its new square) is enormous: at midgame a side can have on the
// order of a few dozen queen-move destinations across its 4 queens, each
// followed by another few dozen arrow destinations, i.e. low thousands of
// full (move, shoot) turns per ply. Naive minimax/alpha-beta even 2 turns
// deep would mean evaluating thousands-squared positions — not realistic on
// this device's 32-bit ARM chip. Real Amazons engines' standard answer is a
// "territory" heuristic (per empty square, compare which side's queen(s) can
// reach it in fewer queen-moves — a multi-source flood-fill/BFS from each
// side's queens, counting queen-move hops rather than tile steps) evaluated
// at shallow depth. That is exactly what this file does: it enumerates every
// legal turn ONCE (depth 1 — the move+shoot pair counted as a single ply,
// not a 2-ply search with a further reply lookahead, which would square the
// branching factor and blow the time budget) and picks the turn that leaves
// the best territory/mobility score for the mover. This is a real but
// honestly weak/exploratory opponent — see the rules screen and menu label
// ("Mot dator, svag/experimentell") — never advertised as strong play. Same
// honesty policy as the goban module's Go AI.
const distInf = 1 << 20

// queenDistances runs a multi-source BFS from every one of side's queens,
// where a single "hop" is a whole queen-line ray (not one tile) — matching
// how a queen actually reaches a square. Because every hop costs exactly 1
// and all sources start simultaneously at distance 0, the queue is
// processed in non-decreasing distance order exactly like a normal
// single-source BFS, even though one node can fan out to many next-layer
// nodes at once (a whole ray of them) — so each square's distance is
// finalized the first time it's discovered.
func queenDistances(b *Board, side Side) [Size][Size]int {
	var dist [Size][Size]int
	for y := range dist {
		for x := range dist[y] {
			dist[y][x] = distInf
		}
	}
	queens := b.QueenPositions(side)
	queue := make([]image.Point, 0, len(queens)*4)
	for _, q := range queens {
		dist[q.Y][q.X] = 0
		queue = append(queue, q)
	}
	for i := 0; i < len(queue); i++ {
		p := queue[i]
		d := dist[p.Y][p.X]
		for _, dir := range dirs8 {
			x, y := p.X+dir.X, p.Y+dir.Y
			for inBounds(x, y) && b.At(x, y) == Empty {
				if d+1 < dist[y][x] {
					dist[y][x] = d + 1
					queue = append(queue, image.Pt(x, y))
				}
				x += dir.X
				y += dir.Y
			}
		}
	}
	return dist
}

// evaluate scores board b from side's perspective (higher = better for
// side): territory (empty squares side's queens can reach in strictly fewer
// queen-move hops than the opponent's, minus the reverse) dominates, with a
// small immediate-mobility tie-break layered on top.
func evaluate(b *Board, side Side) int {
	opp := side.Opponent()
	dSide := queenDistances(b, side)
	dOpp := queenDistances(b, opp)

	territory := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) != Empty {
				continue // only contestable (empty, non-burned) squares count
			}
			switch {
			case dSide[y][x] < dOpp[y][x]:
				territory++
			case dOpp[y][x] < dSide[y][x]:
				territory--
			}
		}
	}

	mobility := 0
	for _, q := range b.QueenPositions(side) {
		mobility += len(b.DestinationsFrom(q))
	}
	for _, q := range b.QueenPositions(opp) {
		mobility -= len(b.DestinationsFrom(q))
	}

	return territory*10 + mobility
}

// BestTurn returns the AI's chosen full turn for side: every legal (move,
// shoot) pair is enumerated once and scored by evaluate on the resulting
// board (depth 1, per this file's header comment — no further lookahead).
// ok is false only if side has no legal move at all.
func BestTurn(b Board, side Side) (Turn, bool) {
	found := false
	var best Turn
	bestScore := negInf

	for _, from := range b.QueenPositions(side) {
		for _, to := range b.DestinationsFrom(from) {
			afterMove := b.MoveQueen(QueenMove{From: from, To: to})
			for _, shot := range afterMove.DestinationsFrom(to) {
				afterShot := afterMove.Shoot(shot)
				score := evaluate(&afterShot, side)
				score += rand.Intn(3) // small jitter: avoid deterministic staleness
				if !found || score > bestScore {
					found = true
					bestScore = score
					best = Turn{Move: QueenMove{From: from, To: to}, Shot: shot}
				}
			}
		}
	}
	return best, found
}

const negInf = -1 << 30
