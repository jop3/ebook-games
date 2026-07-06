package game

import "sort"

// ai.go: a compact negamax (alpha-beta) AI, structured the same way as
// hasami's and shong's. Evaluation is material (weighted per troop type,
// since a jumper or long-range slider is worth more than a basic footman)
// plus a heavy penalty for enemy tiles that threaten — or merely stand next
// to — the evaluating side's own Duke, since losing the Duke ends the game
// outright regardless of material elsewhere.
//
// Difficulty is passed directly as search depth, matching the rest of the
// repo's rule-based games:
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

// tileValue weights material by troop type — a rough, hand-tuned read of
// each type's usefulness (reach and flexibility), not a claim of exact
// balance. The Duke itself is never scored as material: losing it is a
// terminal loss, handled separately in negamax/BestMove, not a point
// deduction.
func tileValue(t TileType) int {
	switch t {
	case Footman:
		return 100
	case DiagGuard:
		return 110
	case Rider:
		return 150
	case Knight:
		return 130
	case Catapult:
		return 120
	case Champion:
		return 160
	default:
		return 0
	}
}

// material sums side's tile values currently on the board.
func material(b *Board, side Side) int {
	total := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if t := b.At(x, y); t != nil && t.Side == side {
				total += tileValue(t.Type)
			}
		}
	}
	return total
}

// dukeThreatPenalty scores how exposed side's Duke currently is: an enemy
// action that could land on (or strike) the Duke's square RIGHT NOW is an
// immediate mating threat (heavily penalized); an enemy tile merely sitting
// orthogonally adjacent to the Duke is a lighter, precautionary penalty
// (adjacency is what recruiting also cares about, and it's usually the
// precondition for next turn's threat). Both terms are deliberately much
// larger than typical material deltas, per the spec's instruction to
// "heavily penalize Duke-adjacent enemy threats."
func dukeThreatPenalty(b *Board, side Side) int {
	dukePos, ok := b.DukePos(side)
	if !ok {
		return winScore // already gone; BestMove/negamax special-case this before it matters
	}
	enemy := side.Opponent()
	penalty := 0
	for _, a := range b.LegalActions(enemy, 0) { // reserve irrelevant: recruiting can't threaten a capture
		if (a.Kind == ActRelocate || a.Kind == ActStrike) && a.To == dukePos {
			penalty += 600
		}
	}
	for _, d := range dukeAdjacent {
		if t := b.At(dukePos.X+d[0], dukePos.Y+d[1]); t != nil && t.Side == enemy {
			penalty += 60
		}
	}
	return penalty
}

// mobility is a light tie-breaking term: how many actions side currently
// has available (reserve omitted — recruit-count swings would otherwise
// dwarf the genuinely useful on-board mobility signal).
func mobility(b *Board, side Side) int {
	return len(b.LegalActions(side, 0))
}

// evaluate scores the board from toMove's perspective (higher = better for
// toMove). Material and Duke safety dominate; mobility is a minor
// tiebreaker.
func evaluate(b *Board, reserve [2]ReserveMask, toMove Side) int {
	opp := toMove.Opponent()
	m := material(b, toMove) - material(b, opp)
	safety := dukeThreatPenalty(b, opp) - dukeThreatPenalty(b, toMove)
	mob := mobility(b, toMove) - mobility(b, opp)
	_ = reserve // reserve size is already reflected via recruit actions when they're actually searched
	return m + safety + 3*mob
}

// quickCaptureValue estimates how valuable an action is for move ordering:
// the value of whatever it captures (if anything), without mutating the
// board — good move ordering is what makes alpha-beta pruning effective.
func quickCaptureValue(b *Board, a Action) int {
	if a.Kind == ActRecruit {
		return 0
	}
	if t := b.At(a.To.X, a.To.Y); t != nil {
		return tileValue(t.Type) + 1 // +1 nudges captures ahead of recruits/moves of equal material
	}
	return 0
}

// orderedActions returns side's legal actions ranked for search: biggest
// captures first, everything else after.
func orderedActions(b *Board, reserve [2]ReserveMask, side Side) []Action {
	actions := b.LegalActions(side, reserve[side])
	type scored struct {
		a Action
		s int
	}
	list := make([]scored, len(actions))
	for i, a := range actions {
		list[i] = scored{a, quickCaptureValue(b, a)}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].s > list[j].s })
	out := make([]Action, len(list))
	for i, sc := range list {
		out[i] = sc.a
	}
	return out
}

// applyAI applies action a for side against (b, reserve), returning the
// resulting board and reserve pair — a GameState-free helper so the search
// doesn't need to construct a full GameState per node.
func applyAI(b Board, reserve [2]ReserveMask, side Side, a Action) (Board, [2]ReserveMask) {
	if a.Kind == ActRecruit {
		nb := b
		nb.set(a.To.X, a.To.Y, &Tile{Type: a.Recruit, Side: side, Face: FaceA})
		nr := reserve
		nr[side] = nr[side].Remove(a.Recruit)
		return nb, nr
	}
	nb, _ := b.Apply(a)
	return nb, reserve
}

// BestMove returns the AI's chosen action for side at the given search
// depth, given the live board and both sides' reserves. ok is false only if
// side has no legal action at all.
func BestMove(b Board, reserve [2]ReserveMask, side Side, depth int) (Action, bool) {
	actions := orderedActions(&b, reserve, side)
	if len(actions) == 0 {
		return Action{}, false
	}
	if depth < 1 {
		depth = 1
	}
	opp := side.Opponent()

	// Fast path: take an immediate Duke-capturing action without searching
	// deeper.
	for _, a := range actions {
		if a.Kind == ActRecruit {
			continue
		}
		if t := b.At(a.To.X, a.To.Y); t != nil && t.Type == Duke && t.Side == opp {
			return a, true
		}
	}

	best := negInf
	var bestAction Action
	found := false
	alpha, beta := negInf, posInf
	for _, a := range actions {
		nb, nr := applyAI(b, reserve, side, a)
		score := -negamax(nb, nr, opp, depth-1, -beta, -alpha)
		if !found || score > best {
			best, bestAction, found = score, a, true
		}
		if best > alpha {
			alpha = best
		}
	}
	return bestAction, found
}

// negamax searches to the given depth from toMove's perspective. A position
// where toMove's Duke is already gone (the opponent's prior action just
// captured it) is an immediate loss for toMove; the opponent's Duke already
// gone is an immediate win. Deeper wins/losses are scored slightly
// better/worse than shallower ones so the search prefers the fastest win and
// the slowest loss, same convention as hasami's negamax.
func negamax(b Board, reserve [2]ReserveMask, toMove Side, depth, alpha, beta int) int {
	opp := toMove.Opponent()
	if _, ok := b.DukePos(toMove); !ok {
		return -winScore - depth
	}
	if _, ok := b.DukePos(opp); !ok {
		return winScore + depth
	}
	actions := orderedActions(&b, reserve, toMove)
	if len(actions) == 0 {
		return -winScore - depth // no legal action: forfeit, same as GameState's stalemate rule
	}
	if depth == 0 {
		return evaluate(&b, reserve, toMove)
	}
	best := negInf
	for _, a := range actions {
		nb, nr := applyAI(b, reserve, toMove, a)
		score := -negamax(nb, nr, opp, depth-1, -beta, -alpha)
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
