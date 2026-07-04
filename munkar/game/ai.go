package game

import (
	"image"
	"sort"
)

// ai.go: a compact negamax (alpha-beta) AI over legal placements, respecting
// the direction-forcing constraint in move generation (the legal-move list
// at every node is exactly GameState.LegalMoves()'s rule: the forced cells
// implied by the previous placement, or every empty cell if unconstrained).
//
// Depth is passed directly as search depth, matching othello/hasami's AI:
//
//	Lätt (easy) = DepthEasy (2), Medel (medium) = DepthMedium (3),
//	Svår (hard) = DepthHard (4).
//
// See ai_test.go for the measured wall-clock time at DepthHard from a
// realistic mid-game position.
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

// windowWeight scores a live (not blocked by an enemy ring) 5-cell window by
// how many of the mover's own rings already sit in it — an exponential
// reward for building toward a 5-in-a-row.
var windowWeight = [6]int{0, 1, 4, 16, 64, 256}

// linePotential sums windowWeight over every live 5-cell window on every
// line (row/column/diagonal) on the board — the "own longest-line
// potential" term from the spec's evaluation.
func linePotential(b *Board, player Cell) int {
	enemy := player.Opponent()
	score := 0
	for _, line := range precomputedLines {
		for i := 0; i+5 <= len(line); i++ {
			own, blocked := 0, false
			for _, p := range line[i : i+5] {
				switch b.At(p.X, p.Y) {
				case player:
					own++
				case enemy:
					blocked = true
				}
			}
			if !blocked {
				score += windowWeight[own]
			}
		}
	}
	return score
}

// exposure counts how many rings of player would flip, summed over every
// empty cell the opponent could hypothetically place at right now — an
// exact (not approximated) measure of "how much capture is on the table
// against player", using the same captureScan the real rules use. It
// ignores whether the opponent's forced-direction constraint would actually
// let them reach that cell next turn, which makes this a (safe)
// overestimate of real one-move danger — an acceptable simplification for a
// shallow-search heuristic term.
func exposure(b *Board, player Cell) int {
	opp := player.Opponent()
	total := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) != Empty {
				continue
			}
			nb := *b
			nb.Ring[y][x] = opp
			total += len(captureScan(&nb, image.Pt(x, y), opp))
		}
	}
	return total
}

// evaluate scores the board from toMove's perspective (higher = better for
// toMove): own vs. opponent longest-line potential, material (ring count
// difference — the net effect of captures gained over the game), exposure
// to being captured, and largest-group size.
func evaluate(b *Board, toMove Cell) int {
	opp := toMove.Opponent()
	line := linePotential(b, toMove) - linePotential(b, opp)
	material := (b.Count(toMove) - b.Count(opp)) * 10
	exp := exposure(b, toMove) - exposure(b, opp)
	group := LargestGroup(b, toMove) - LargestGroup(b, opp)
	return line + material - 6*exp + 5*group
}

// orderMoves ranks candidates for search: moves that capture the most
// immediately come first (real captureScan counts, not an approximation),
// tied-broken by board order. Good move ordering is what makes alpha-beta
// pruning effective, especially important here since direction-forcing
// already narrows the branching factor a lot — the few candidates that
// remain are worth ordering well.
func orderMoves(b Board, mover Cell, candidates []image.Point) []image.Point {
	type scored struct {
		p image.Point
		s int
	}
	list := make([]scored, len(candidates))
	for i, p := range candidates {
		nb := b
		nb.Ring[p.Y][p.X] = mover
		list[i] = scored{p, len(captureScan(&nb, p, mover))}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].s > list[j].s })
	out := make([]image.Point, len(list))
	for i, sc := range list {
		out[i] = sc.p
	}
	return out
}

// BestMove returns the AI's chosen move for player at the given search
// depth, choosing among the given legal candidates (the caller — GameState —
// already computed these respecting the forced-direction constraint). ok is
// false only if candidates is empty.
func BestMove(b Board, player Cell, candidates []image.Point, depth int) (image.Point, bool) {
	if len(candidates) == 0 {
		return image.Point{}, false
	}
	if depth < 1 {
		depth = 1
	}

	// Fast path: take an immediate 5-in-a-row win without searching deeper.
	for _, m := range candidates {
		nb, _ := Place(b, m, player)
		if Five(&nb, player) {
			return m, true
		}
	}

	opp := player.Opponent()
	moves := orderMoves(b, player, candidates)
	best := negInf
	var bestMove image.Point
	found := false
	alpha, beta := negInf, posInf
	for _, m := range moves {
		nb, _ := Place(b, m, player)
		score := -negamax(nb, opp, m, depth-1, -beta, -alpha)
		if !found || score > best {
			best, bestMove, found = score, m, true
		}
		if best > alpha {
			alpha = best
		}
	}
	return bestMove, found
}

// negamax searches to the given depth from toMove's perspective. b already
// reflects the opponent's (toMove.Opponent()'s) placement at lastPlaced, so
// that must be checked for an immediate win/loss before anything else.
func negamax(b Board, toMove Cell, lastPlaced image.Point, depth, alpha, beta int) int {
	opp := toMove.Opponent()
	if Five(&b, opp) {
		return -(winScore + depth) // toMove just lost; prefer the slowest loss
	}
	if boardFull(&b) {
		switch tiebreakWinner(&b) {
		case toMove:
			return winScore
		case opp:
			return -winScore
		default:
			return 0
		}
	}
	if depth == 0 {
		return evaluate(&b, toMove)
	}

	candidates := LegalMoves(b, lastPlaced, true)
	moves := orderMoves(b, toMove, candidates)
	best := negInf
	for _, m := range moves {
		nb, _ := Place(b, m, toMove)
		score := -negamax(nb, opp, m, depth-1, -beta, -alpha)
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
