package game

import "sort"

// ai.go: a full-turn negamax (alpha-beta) AI. The L-Game's total state space
// is tiny — a 4x4 board with only 10 of 16 cells ever occupied at once — so
// a full-depth search over complete turns (mandatory L-placement, optional
// neutral move) is cheap and gives a strong, close-to-optimal opponent
// without needing a heuristic evaluation tuned by hand; the evaluation used
// at the search horizon is simply L-placement mobility, which is exactly
// what the win condition is about.
//
// Difficulty is passed directly as search depth, counted in full turns (one
// player's mandatory-L-plus-optional-neutral move = one ply), matching the
// convention used by hasami/othello's AI:
const (
	DepthEasy   = 1
	DepthMedium = 3
	DepthHard   = 6
)

const (
	negInf   = -1 << 30
	posInf   = 1 << 30
	winScore = 1 << 20
)

// FullMove is one complete turn: the mandatory L-placement, and, if
// HasNeutral, the optional neutral-piece move that follows it. Result is the
// board after both parts are applied (or after just the L-placement, if
// HasNeutral is false).
type FullMove struct {
	L          Placement
	HasNeutral bool
	Neutral    NeutralMove
	Result     Board
}

// GenerateFullMoves returns every complete turn side could take from board
// b: every legal L-placement, each paired with either skipping the neutral
// step or every legal neutral-piece relocation afterward.
func GenerateFullMoves(b Board, side Side) []FullMove {
	return generateFullMoves(b, side, true)
}

// GenerateFullMovesLite returns only the "skip the neutral move" option for
// each legal L-placement — used to keep deeper search plies affordable (see
// generateFullMoves).
func GenerateFullMovesLite(b Board, side Side) []FullMove {
	return generateFullMoves(b, side, false)
}

// generateFullMoves is GenerateFullMoves's implementation; includeNeutral
// controls whether every legal neutral relocation is enumerated (true) or
// only the "skip" option (false).
//
// Full fidelity (side's L-placement x every neutral option) has real
// branching — on a fairly open board, ~10 legal L-placements x ~13 neutral
// options (2 pieces x up to ~6 empty cells, plus skip) is ~130 full moves
// per node, and a 3-ply search over that is too slow for a tap-driven UI on
// slow ARM e-ink hardware (measured ~3.5s on a fast dev machine at depth 3
// from the opening position — see TestAIPerformanceBudget). Since the
// neutral move is the secondary, "nudge" part of a turn, BestMove only
// spends that full branching budget on the immediate decision and the
// opponent's immediate reply (see fullNeutralPlies in BestMove/negamax);
// deeper plies use the lite generator (L-placement choices only, always
// treated as skipping the neutral step), which cuts branching roughly
// tenfold per additional ply while still fully modeling the one rule that
// actually decides the game — L-placement mobility.
func generateFullMoves(b Board, side Side, includeNeutral bool) []FullMove {
	var out []FullMove
	for _, pl := range LegalLPlacements(b, side) {
		afterL := ApplyLPlacement(b, side, pl)
		out = append(out, FullMove{L: pl, Result: afterL})
		if !includeNeutral {
			continue
		}
		for _, nm := range LegalNeutralMoves(afterL) {
			out = append(out, FullMove{
				L: pl, HasNeutral: true, Neutral: nm,
				Result: ApplyNeutralMove(afterL, nm),
			})
		}
	}
	return out
}

// evaluate scores board b from toMove's perspective (higher = better for
// toMove) when the search horizon is reached without a decided game: the
// difference in L-placement mobility, since running out of legal
// placements is precisely how the game is won or lost.
func evaluate(b Board, toMove Cell) int {
	opp := toMove.Opponent()
	return len(LegalLPlacements(b, toMove)) - len(LegalLPlacements(b, opp))
}

// orderedFullMoves ranks side's full moves for search: those leaving the
// opponent with the fewest legal L-placements first. Good move ordering is
// what makes alpha-beta pruning effective. full selects the full or lite
// move generator (see generateFullMoves).
func orderedFullMoves(b Board, side Side, full bool) []FullMove {
	opp := side.Opponent()
	moves := generateFullMoves(b, side, full)
	type scored struct {
		m FullMove
		s int
	}
	list := make([]scored, len(moves))
	for i, m := range moves {
		list[i] = scored{m, len(LegalLPlacements(m.Result, opp))}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].s < list[j].s })
	out := make([]FullMove, len(list))
	for i, sc := range list {
		out[i] = sc.m
	}
	return out
}

// fullNeutralPlies is how many plies from the root get full neutral-move
// fidelity (the root decision itself, plus this many additional plies of
// lookahead) before the search switches to the cheaper lite generator for
// the rest of its depth. See generateFullMoves for why.
const fullNeutralPlies = 1

// BestMove returns the AI's chosen full turn for side at the given search
// depth (in full turns). ok is false only if side has no legal L-placement
// at all (the game should already be over in that case).
func BestMove(b Board, side Side, depth int) (FullMove, bool) {
	moves := orderedFullMoves(b, side, true)
	if len(moves) == 0 {
		return FullMove{}, false
	}
	if depth < 1 {
		depth = 1
	}
	opp := side.Opponent()

	// Fast path: if any move immediately wins (leaves the opponent with zero
	// legal L-placements), take it without searching deeper.
	for _, m := range moves {
		if len(LegalLPlacements(m.Result, opp)) == 0 {
			return m, true
		}
	}

	best := negInf
	var bestMove FullMove
	found := false
	alpha, beta := negInf, posInf
	for _, m := range moves {
		score := -negamax(m.Result, opp, depth-1, -beta, -alpha, fullNeutralPlies)
		if !found || score > best {
			best, bestMove, found = score, m, true
		}
		if best > alpha {
			alpha = best
		}
	}
	return bestMove, found
}

// negamax searches to the given depth (in full turns) from toMove's
// perspective. A position where toMove has zero legal L-placements is a
// terminal loss for toMove — scored so that a loss found further down the
// tree (larger remaining depth consumed, i.e. delayed) is preferred over an
// immediate one, and symmetrically a win is preferred sooner. fullPliesLeft
// counts down how many more plies get full neutral-move fidelity before
// falling back to the lite (L-placement only) generator.
func negamax(b Board, toMove Cell, depth, alpha, beta, fullPliesLeft int) int {
	moves := orderedFullMoves(b, toMove, fullPliesLeft > 0)
	if len(moves) == 0 {
		return -winScore - depth
	}
	if depth == 0 {
		return evaluate(b, toMove)
	}
	opp := toMove.Opponent()
	best := negInf
	for _, m := range moves {
		score := -negamax(m.Result, opp, depth-1, -beta, -alpha, fullPliesLeft-1)
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
