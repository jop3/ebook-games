package game

import "sort"

// ai.go implements Mosaik's AI as a difficulty-layered greedy search. Each
// legal move is evaluated by estimating (a) how many wall points it will
// eventually be worth once end-of-round tiling runs, (b) the floor penalty
// it incurs right now, and — at Medel and Svår — (c) a "denial" term
// estimating how good a moment this leaves for the opponent's very next
// turn.
//
// Lätt (DepthEasy) is pure greedy: baseValue only, no opponent modeling.
// Medel (DepthMedium) looks one ply further, subtracting the opponent's best
// immediate baseValue after our move (mediumValue). Svår (DepthHard) models
// that opponent reply using mediumValue itself — i.e. Svår assumes the
// opponent plays at Medel strength — which makes its choices account for a
// materially smarter reply without an explicit deeper minimax tree.
//
// The search never looks past the current round: a move that ends the round
// (Phase leaves PhasePlaying) is evaluated as a leaf right there, and no
// recursion ever crosses into the next round's bag draws (see
// GameState.Clone's doc comment for why this keeps the search
// randomness-free).
//
// To keep the recursive opponent-reply estimates cheap (this is what turns
// what would otherwise be a cubic blow-up into something fast — see ai_test.go
// for the measured wall-clock time), the INNER (opponent-simulation) search
// widths are capped at searchWidth candidates, pre-filtered by a cheap,
// clone-free heuristic (quickScore). The AI's own top-level move choice
// still considers every legal candidate.
const searchWidth = 10

const (
	denialWeightMedium = 1
	denialWeightHard   = 1
)

// BestMove returns the AI's chosen move for `side` at the given difficulty
// (DepthEasy/DepthMedium/DepthHard). Assumes gs.Turn == side (always true
// when called from GameState.StepAI). ok is false only if there are no
// legal moves.
func BestMove(gs *GameState, side int, diff int) (Move, bool) {
	candidates := LegalMoves(gs, side)
	if len(candidates) == 0 {
		return Move{}, false
	}

	var value func(Move) int
	switch {
	case diff <= DepthEasy:
		value = func(m Move) int { return baseValue(gs, m) }
	case diff == DepthMedium:
		value = func(m Move) int { return mediumValue(gs, m) }
	default:
		value = func(m Move) int { return hardValue(gs, m) }
	}

	best := candidates[0]
	bestV := value(best)
	for _, m := range candidates[1:] {
		if v := value(m); v > bestV {
			best, bestV = m, v
		}
	}
	return best, true
}

// baseValue estimates the immediate merit of move m for the side currently
// to move in gs (gs.Turn): the wall points it would score if wall-tiling ran
// right now (crediting every currently-full pattern line, so multi-line
// opportunities are visible), minus the newly-incurred floor penalty, plus a
// small progress term rewarding moves that build toward a row/column/color
// bonus.
func baseValue(gs *GameState, m Move) int {
	side := gs.Turn
	beforeFloor := gs.Boards[side].floorCount()
	ns := gs.Clone()
	ns.applyMove(m)
	b := &ns.Boards[side]

	gain := potentialWallGain(b)
	penalty := floorPenalty(b.floorCount()) - floorPenalty(beforeFloor)
	progress := progressTerm(b)

	return gain*3 - penalty*2 + progress
}

// potentialWallGain sums the hypothetical scorePlacement value of every
// currently-full pattern line, as if wall-tiling ran right now (against a
// throwaway copy of the wall array — b itself is not mutated).
func potentialWallGain(b *Board) int {
	wall := b.Wall // array value copy
	total := 0
	for i := 0; i < WallSize; i++ {
		if len(b.Lines[i]) != i+1 {
			continue
		}
		c := b.Lines[i][0]
		col := wallColOf(i, c)
		if wall[i][col] {
			continue
		}
		wall[i][col] = true
		total += scorePlacement(&wall, i, col)
	}
	return total
}

// progressTerm gives partial credit for building toward the three end-game
// bonuses (rows/cols/colors), even before a line is full.
func progressTerm(b *Board) int {
	total := 0
	for i := 0; i < WallSize; i++ {
		total += len(b.Lines[i])
	}
	_, _, colors, _ := endBonusesDetailed(&b.Wall)
	return total + colors*2
}

// mediumValue looks one ply further than baseValue: after simulating m, it
// subtracts a weighted estimate of the best baseValue the opponent could get
// on their very next turn (if the round didn't just end) — a "does this
// leave an obvious big take on the table" denial term.
func mediumValue(gs *GameState, m Move) int {
	side := gs.Turn
	v := baseValue(gs, m)
	ns := gs.Clone()
	ns.applyMove(m)
	if ns.Phase != PhasePlaying || ns.Turn == side {
		return v
	}
	best := 0
	for _, om := range topByQuickScore(ns, LegalMoves(ns, ns.Turn), searchWidth) {
		if bv := baseValue(ns, om); bv > best {
			best = bv
		}
	}
	return v - denialWeightMedium*best
}

// hardValue is mediumValue's move, but the opponent's best reply is chosen
// using mediumValue's own one-ply-smarter evaluator instead of plain
// baseValue — Svår assumes the opponent plays at Medel strength.
func hardValue(gs *GameState, m Move) int {
	side := gs.Turn
	v := baseValue(gs, m)
	ns := gs.Clone()
	ns.applyMove(m)
	if ns.Phase != PhasePlaying || ns.Turn == side {
		return v
	}
	best := 0
	for _, om := range topByQuickScore(ns, LegalMoves(ns, ns.Turn), searchWidth) {
		if bv := mediumValue(ns, om); bv > best {
			best = bv
		}
	}
	return v - denialWeightHard*best
}

// quickScore is a cheap, clone-free heuristic used only to shortlist
// candidates before the (much more expensive) recursive opponent-reply
// estimate: it ranks a move by how many tiles it lands on a pattern line and
// whether that completes the line, without simulating the move at all.
func quickScore(gs *GameState, m Move) int {
	if m.TargetLine == -1 {
		return -1 // dumping to the floor is rarely the opponent's best reply
	}
	b := &gs.Boards[gs.Turn]
	i := m.TargetLine
	n := countColor(sourceTiles(gs, m.Source), m.Color)
	room := (i + 1) - len(b.Lines[i])
	if room < 0 {
		room = 0
	}
	filled := n
	if filled > room {
		filled = room
	}
	score := filled
	if len(b.Lines[i])+filled == i+1 {
		score += 100 // completes the line
	}
	return score
}

// sourceTiles returns the tiles currently sitting in the given move source
// (a factory index, or -1 for the center).
func sourceTiles(gs *GameState, source int) []Color {
	if source == -1 {
		return gs.Center
	}
	return gs.Factories[source]
}

// topByQuickScore returns the k candidates with the highest quickScore
// (stable on ties), or all of them if there are k or fewer.
func topByQuickScore(gs *GameState, candidates []Move, k int) []Move {
	if len(candidates) <= k {
		return candidates
	}
	type scored struct {
		m Move
		s int
	}
	list := make([]scored, len(candidates))
	for i, m := range candidates {
		list[i] = scored{m, quickScore(gs, m)}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].s > list[j].s })
	out := make([]Move, k)
	for i := 0; i < k; i++ {
		out[i] = list[i].m
	}
	return out
}
