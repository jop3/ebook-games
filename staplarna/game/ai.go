package game

import (
	"sort"
	"time"
)

// ai.go: a compact negamax (alpha-beta) AI. Evaluation is dominated by
// material, weighted by piece-type SCARCITY — losing your last Tzaar (only 6
// per side) ends the game exactly as surely as losing your last Tzarra (9)
// or Tott (15), but Tzaars are worth far more to hang onto along the way, so
// they're weighted heavier and get a steep extra penalty at exactly 1
// remaining ("one more capture from losing outright"). Mobility and total
// stack height are small tie-breaking terms on top.
//
// Difficulty is passed directly as search depth, matching hasami/othello:
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

// typeWeight scores one physical piece of typ for material purposes. Weights
// are inversely related to StartCount — scarcer pieces matter more — scaled
// so a lone Tzaar is worth roughly as much as several Totts. TZAAR has no
// official point values; this is this port's own tuned heuristic.
func typeWeight(t PieceType) int {
	switch t {
	case Tzaar:
		return 60
	case Tzarra:
		return 25
	default:
		return 10
	}
}

// dangerPenalty adds an extra heuristic penalty when a side is down to
// EXACTLY one piece of a type — one more capture away from an immediate
// loss, a risk plain per-piece material alone under-weights.
func dangerPenalty(t PieceType) int {
	switch t {
	case Tzaar:
		return 400
	case Tzarra:
		return 150
	default:
		return 40
	}
}

// material scores side's holdings: weighted piece count, minus a danger
// penalty for any type sitting at exactly 1 remaining.
func material(b *Board, side Side) int {
	total := 0
	for _, t := range AllTypes {
		n := b.TypeCount(side, t)
		total += typeWeight(t) * n
		if n == 1 {
			total -= dangerPenalty(t)
		}
	}
	return total
}

// heightScore rewards side for owning taller stacks (more capturing reach)
// and more of them (more mobility potential).
func heightScore(b *Board, side Side) int {
	total := 0
	for _, s := range b.Stacks {
		if s.Owner == side {
			total += s.Height
		}
	}
	return total
}

// evaluate scores the board from toMove's perspective (higher = better for
// toMove).
func evaluate(b *Board, toMove Side) int {
	opp := toMove.Opponent()
	mat := material(b, toMove) - material(b, opp)
	mobility := len(LegalMoves(b, toMove)) - len(LegalMoves(b, opp))
	height := heightScore(b, toMove) - heightScore(b, opp)
	return mat + 3*mobility + 2*height
}

// captureValue estimates the material value of capturing whatever sits at
// m.To (0 if m is not a capture at all — an empty landing or a merge), used
// only to rank moves for search ordering.
func captureValue(b *Board, m Move) int {
	tgt, ok := b.Stacks[m.To]
	if !ok {
		return 0
	}
	mover := b.Stacks[m.From]
	if tgt.Owner == mover.Owner {
		return 0 // a merge, not a capture
	}
	v := 0
	for _, t := range AllTypes {
		v += typeWeight(t) * tgt.Comp[t]
	}
	return v
}

// orderedMoves returns side's legal moves ranked for search: biggest
// captures first, ties broken arbitrarily (map iteration order) — good move
// ordering is what makes alpha-beta pruning effective.
func orderedMoves(b *Board, side Side) []Move {
	moves := LegalMoves(b, side)
	type scored struct {
		m Move
		s int
	}
	list := make([]scored, len(moves))
	for i, m := range moves {
		list[i] = scored{m, captureValue(b, m)}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].s > list[j].s })
	out := make([]Move, len(list))
	for i, sc := range list {
		out[i] = sc.m
	}
	return out
}

// timeBudget bounds how long BestMove is allowed to spend searching at the
// given nominal depth. A full 60-piece TZAAR board (30 stacks/side, up to 6
// directions each) has a branching factor deep-search can't afford at fixed
// depth on slow e-ink hardware — depth 4 measured over 11s on a fast dev
// machine, well past acceptable UI latency. Rather than just lowering the
// depth constants (which caps how far the AI can ever see even on an easy,
// nearly-empty board), BestMove/negamax below use iterative deepening under
// this wall-clock budget instead: try depth 1, then 2, then 3... up to the
// requested max depth, keeping the best move found so far, and stopping as
// soon as the budget runs out — bounded latency regardless of branching,
// same fix already applied to dominering's AI for the same reason.
func timeBudget(depth int) time.Duration {
	switch {
	case depth <= DepthEasy:
		return 400 * time.Millisecond
	case depth == DepthMedium:
		return 1200 * time.Millisecond
	default:
		return 3 * time.Second
	}
}

// BestMove returns the AI's chosen move for side, searching iteratively
// deeper up to depth (or until its time budget runs out, whichever is
// first). ok is false only if side has no legal move at all.
func BestMove(b *Board, side Side, depth int) (Move, bool) {
	moves := orderedMoves(b, side)
	if len(moves) == 0 {
		return Move{}, false
	}
	if depth < 1 {
		depth = 1
	}
	opp := side.Opponent()

	// Fast path: if any move wins outright right now (eliminates a type of
	// the opponent's), take it without searching deeper.
	for _, m := range moves {
		nb := b.Clone()
		nb.Apply(m)
		if EliminatedSide(nb) == opp {
			return m, true
		}
	}

	deadline := time.Now().Add(timeBudget(depth))
	var bestMove Move
	found := false
	for d := 1; d <= depth; d++ {
		best := negInf
		var curMove Move
		curFound := false
		alpha, beta := negInf, posInf
		for _, m := range moves {
			nb := b.Clone()
			nb.Apply(m)
			score := -negamax(nb, opp, d-1, -beta, -alpha, deadline)
			if !curFound || score > best {
				best, curMove, curFound = score, m, true
			}
			if best > alpha {
				alpha = best
			}
		}
		if curFound {
			bestMove, found = curMove, true
			// Reorder root moves so the next (deeper) iteration searches
			// this iteration's best move first — better alpha-beta pruning.
			for i, m := range moves {
				if m == curMove {
					moves[0], moves[i] = moves[i], moves[0]
					break
				}
			}
		}
		if time.Now().After(deadline) {
			break
		}
	}
	return bestMove, found
}

// negamax searches to the given depth from toMove's perspective, bailing out
// to a static evaluation early if the deadline has passed (bounding total
// search time regardless of how deep `depth` nominally asks for). A position
// where some side has already been eliminated on a type, or toMove has no
// legal move at all, is scored as an immediate terminal result — deeper
// wins/losses scored slightly better/worse than shallower ones so the search
// prefers the fastest win and the slowest loss.
func negamax(b *Board, toMove Side, depth, alpha, beta int, deadline time.Time) int {
	opp := toMove.Opponent()
	if loser := EliminatedSide(b); loser != None {
		if loser == toMove {
			return -winScore - depth
		}
		return winScore + depth
	}
	if depth == 0 || time.Now().After(deadline) {
		return evaluate(b, toMove)
	}
	moves := orderedMoves(b, toMove)
	if len(moves) == 0 {
		return -winScore - depth // toMove has no legal move: toMove loses
	}
	best := negInf
	for _, m := range moves {
		nb := b.Clone()
		nb.Apply(m)
		score := -negamax(nb, opp, depth-1, -beta, -alpha, deadline)
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

// AIPlacement chooses a setup-phase placement for side: it places its most
// expendable remaining type first (Tott, then Tzarra, saving the scarce
// Tzaar for last, once the board has some structure to tuck it into), at the
// empty cell closest to the board's center (maximizing a freshly placed lone
// piece's future reach). A deliberately CASUAL heuristic — TZAAR opening
// theory is deep and this AI does not attempt it, matching this house's
// established "honest, not master-strength" framing for hex-board AIs (see
// ringar's identical disclaimer).
func AIPlacement(b *Board, remaining map[PieceType]int) (PieceType, Point) {
	order := [3]PieceType{Tott, Tzarra, Tzaar}
	typ := Tott
	for _, t := range order {
		if remaining[t] > 0 {
			typ = t
			break
		}
	}

	center := Point{0, 0, 0}
	var best Point
	bestD := -1
	for _, p := range AllPoints() {
		if _, occ := b.At(p); occ {
			continue
		}
		d := Distance(p, center)
		if bestD == -1 || d < bestD {
			bestD, best = d, p
		}
	}
	return typ, best
}
