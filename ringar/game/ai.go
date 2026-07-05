package game

import "sort"

// ai.go: a compact negamax (alpha-beta) search for ring moves, plus simple
// deterministic heuristics for the two other decisions a player makes
// (placement, and — when a move completes a row — which ring to give up).
//
// YINSH's branching factor is much larger than the other games in this
// batch (up to 5 rings, each often with several sliding+jumping
// destinations, easily 40-100+ legal moves in the middle game), so per the
// spec this AI is deliberately shallow (AIDepth = 2) and shipped as an
// explicitly CASUAL "Mot dator" opponent — hot-seat two-player is Ringar's
// primary experience. See ai_test.go for measured wall-clock timing.

const (
	negInf   = -1 << 30
	posInf   = 1 << 30
	winScore = 1 << 20
)

type ringMove struct{ From, To Point }

// genMoves lists side's legal ring moves, in a fixed deterministic order (so
// alpha-beta tie-breaks are reproducible across runs).
func genMoves(b *Board, side Side) []ringMove {
	var froms []Point
	for p, s := range b.Rings {
		if s == side {
			froms = append(froms, p)
		}
	}
	sort.Slice(froms, func(i, j int) bool { return lessPoint(froms[i], froms[j]) })
	var moves []ringMove
	for _, f := range froms {
		dests := RingMoves(b, f)
		sort.Slice(dests, func(i, j int) bool { return lessPoint(dests[i], dests[j]) })
		for _, d := range dests {
			moves = append(moves, ringMove{f, d})
		}
	}
	return moves
}

func lessPoint(a, b Point) bool {
	if a.X != b.X {
		return a.X < b.X
	}
	if a.Y != b.Y {
		return a.Y < b.Y
	}
	return a.Z < b.Z
}

// searchState is the AI's own compact copy of the position: a board plus a
// rings-removed tally, independent of the UI-facing GameState so the search
// can simulate row resolutions freely.
type searchState struct {
	b       *Board
	removed [3]int // indexed by Side; None unused
}

// resolveAllRows simulates claiming every row `mover`'s move just created —
// mover's own rows first, then (defensively, see rows.go) the opponent's —
// using the simplest legal choice at each step: the leftmost 5-window of a
// long run, and the mover's currently least mobile ring (keeping the more
// useful rings on the board). This mirrors AIPickWindow/AIPickRingToRemove,
// which drive the same decisions in the real (non-simulated) game.
func resolveAllRows(ss *searchState, mover Side) {
	for _, side := range [2]Side{mover, mover.Opponent()} {
		for {
			rows := FindRows(ss.b, side)
			if len(rows) == 0 {
				break
			}
			win := rows[0].Points
			if len(win) > 5 {
				win = win[:5]
			}
			RemoveRow(ss.b, win)
			ring := AIPickRingToRemove(ss.b, side)
			RemoveRing(ss.b, ring)
			ss.removed[side]++
			if ss.removed[side] >= 3 {
				return
			}
		}
	}
}

// eval scores ss from side's perspective (higher is better for side).
// Rings-removed difference dominates (it is literally the win condition);
// near-complete rows, marker majority and ring mobility are much smaller
// tie-breaking terms.
func eval(ss *searchState, side Side) int {
	opp := side.Opponent()
	score := 100000 * (ss.removed[side] - ss.removed[opp])
	score += 6 * (nearRowScore(ss.b, side) - nearRowScore(ss.b, opp))
	score += ss.b.MarkerCount(side) - ss.b.MarkerCount(opp)
	score += 2 * (mobility(ss.b, side) - mobility(ss.b, opp))
	return score
}

// nearRowScore rewards runs of 2-4 markers of side's color (the seeds of a
// future row), weighted steeply toward runs closer to completion.
func nearRowScore(b *Board, side Side) int {
	total := 0
	for axis := AxisA; axis <= AxisC; axis++ {
		for _, line := range Lines(axis) {
			i := 0
			for i < len(line) {
				if b.Markers[line[i]] != side {
					i++
					continue
				}
				j := i
				for j < len(line) && b.Markers[line[j]] == side {
					j++
				}
				switch L := j - i; {
				case L == 4:
					total += 8
				case L == 3:
					total += 3
				case L == 2:
					total += 1
				}
				i = j
			}
		}
	}
	return total
}

func mobility(b *Board, side Side) int {
	n := 0
	for p, s := range b.Rings {
		if s == side {
			n += len(RingMoves(b, p))
		}
	}
	return n
}

// negamax searches to the given depth from toMove's perspective.
func negamax(ss *searchState, toMove Side, depth, alpha, beta int) int {
	if ss.removed[toMove] >= 3 {
		return winScore + depth
	}
	if ss.removed[toMove.Opponent()] >= 3 {
		return -winScore - depth
	}
	moves := genMoves(ss.b, toMove)
	if len(moves) == 0 {
		// No legal ring move at all is an essentially unreachable edge case
		// (it requires every one of toMove's rings to be fully boxed in on
		// all 6 directions); treated defensively as a loss for toMove rather
		// than crashing the search.
		return -winScore - depth
	}
	if depth == 0 {
		return eval(ss, toMove)
	}
	best := negInf
	for _, m := range moves {
		nb := ss.b.Clone()
		nss := &searchState{b: nb, removed: ss.removed}
		ApplyRingMove(nss.b, m.From, m.To)
		resolveAllRows(nss, toMove)
		score := -negamax(nss, toMove.Opponent(), depth-1, -beta, -alpha)
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

// BestMove returns the AI's chosen ring move for side. ok is false only if
// side has no legal ring move at all.
func BestMove(b *Board, side Side, depth int) (from, to Point, ok bool) {
	moves := genMoves(b, side)
	if len(moves) == 0 {
		return Point{}, Point{}, false
	}
	if depth < 1 {
		depth = 1
	}
	opp := side.Opponent()
	best := negInf
	var bestMove ringMove
	alpha, beta := negInf, posInf
	for _, m := range moves {
		nb := b.Clone()
		ss := &searchState{b: nb}
		ApplyRingMove(ss.b, m.From, m.To)
		resolveAllRows(ss, side)
		score := -negamax(ss, opp, depth-1, -beta, -alpha)
		if score > best {
			best, bestMove = score, m
		}
		if best > alpha {
			alpha = best
		}
	}
	return bestMove.From, bestMove.To, true
}

// AIPlacement chooses where the AI places its next ring during
// PhasePlacement: the most central empty point (minimizing the cube-radius
// max(|x|,|y|,|z|)), which is a reasonable opening heuristic — central rings
// reach more of the board's lines. Ties broken by AllPoints' fixed order.
func AIPlacement(b *Board) Point {
	best := Point{}
	bestScore := 1 << 30
	for _, p := range AllPoints() {
		if b.HasRing(p) {
			continue
		}
		score := maxi(absi(p.X), absi(p.Y), absi(p.Z))
		if score < bestScore {
			bestScore, best = score, p
		}
	}
	return best
}

// AIPickWindow picks which 5-marker slice of a completed run (len > 5) the
// AI claims: simply the leftmost (lowest-index) window, a deterministic,
// cheap default consistent with resolveAllRows' search-tree approximation.
func AIPickWindow(run []Point) []Point {
	if len(run) <= 5 {
		win := make([]Point, len(run))
		copy(win, run)
		return win
	}
	win, _ := Window(run, run[0])
	return win
}

// AIPickRingToRemove chooses which of side's own rings to sacrifice when
// claiming a row: the one with the fewest current legal moves, so the
// AI keeps its more mobile (more useful) rings on the board.
func AIPickRingToRemove(b *Board, side Side) Point {
	var best Point
	bestMob := 1 << 30
	found := false
	var rings []Point
	for p, s := range b.Rings {
		if s == side {
			rings = append(rings, p)
		}
	}
	sort.Slice(rings, func(i, j int) bool { return lessPoint(rings[i], rings[j]) })
	for _, p := range rings {
		m := len(RingMoves(b, p))
		if !found || m < bestMob {
			best, bestMob, found = p, m, true
		}
	}
	return best
}
