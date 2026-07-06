package game

import (
	"image"
	"sort"
)

// ai.go: alpha-beta search over a single fixed evaluation function chosen by
// which side the AI is playing. The board is tiny (7x7, 13 pieces total) so
// search depth is computationally cheap; the real engineering cost, per the
// spec, is that the two sides value completely different things and so need
// two genuinely distinct evaluation functions, not a shared symmetric one:
//
//   - Attackers value the king's distance to the nearest corner (bigger is
//     safer for them) and containment/encirclement of the king (how many of
//     the king's own 4 neighbors are already attackers).
//   - Defenders value the king's own escape-route count (its current
//     mobility) and how many orthogonal lines from the king run clear all
//     the way to the board edge (open lines toward a corner).
//
// BestMove always maximizes the CHOSEN side's evaluation and treats the
// opponent as minimizing that same function — the standard single-sided
// minimax approximation for asymmetric games, rather than hasami's
// symmetric negamax (score, -score), which does not fit here because the two
// sides' notions of "good" are not mirror images of each other.
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

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// kingDistanceToNearestCorner is the Manhattan distance from kp to the
// closest of the 4 corners — a cheap proxy for how close the king is to
// winning by escape.
func kingDistanceToNearestCorner(kp image.Point) int {
	best := 1 << 30
	for _, c := range cornerCells {
		d := absInt(kp.X-c.X) + absInt(kp.Y-c.Y)
		if d < best {
			best = d
		}
	}
	return best
}

// containment counts how many of the king's 4 orthogonal neighbors are
// already occupied by an attacker — the attackers' encirclement metric.
func containment(b *Board, kp image.Point) int {
	n := 0
	for _, d := range dirs4 {
		if b.At(kp.X+d.X, kp.Y+d.Y) == Attacker {
			n++
		}
	}
	return n
}

// kingEscapeRouteCount is the king's own current legal-move count — the
// defenders' mobility metric for the king specifically.
func kingEscapeRouteCount(b *Board, kp image.Point) int {
	return len(b.DestinationsFrom(kp, true))
}

// openLinesToCorner counts how many of the 4 orthogonal directions from the
// king run completely clear of any piece all the way to the board edge — a
// proxy for open lanes the king could use toward a corner in the near
// future, even though the piece in the way might not sit exactly on a
// corner-bound path this move.
func openLinesToCorner(b *Board, kp image.Point) int {
	n := 0
	for _, d := range dirs4 {
		x, y := kp.X+d.X, kp.Y+d.Y
		clear := true
		for inBounds(x, y) {
			if b.At(x, y) != Empty {
				clear = false
				break
			}
			x += d.X
			y += d.Y
		}
		if clear {
			n++
		}
	}
	return n
}

// evalForAttacker scores b favorably for the attacking side: the king should
// be far from any corner and boxed in, and material losses/gains matter too.
func evalForAttacker(b *Board) int {
	kp, alive := b.KingPos()
	if !alive {
		return winScore
	}
	material := (StartDefenders+1-b.DefenderSideCount())*40 - (StartAttackers-b.Count(Attacker))*40
	mobility := len(b.LegalMoves(SideAttacker)) - len(b.LegalMoves(SideDefender))
	return kingDistanceToNearestCorner(kp)*12 + containment(b, kp)*25 + material + mobility
}

// evalForDefender scores b favorably for the defending side: the king should
// be close to a corner, with many open escape lines and high mobility.
func evalForDefender(b *Board) int {
	kp, alive := b.KingPos()
	if !alive {
		return -winScore
	}
	material := (b.DefenderSideCount()-(StartDefenders+1))*40 + (StartAttackers-b.Count(Attacker))*40
	mobility := len(b.LegalMoves(SideDefender)) - len(b.LegalMoves(SideAttacker))
	return -kingDistanceToNearestCorner(kp)*8 + kingEscapeRouteCount(b, kp)*15 + openLinesToCorner(b, kp)*20 + material + mobility
}

// evalFor returns the evaluation function to use when the AI is playing
// side.
func evalFor(side Side) func(*Board) int {
	if side == SideAttacker {
		return evalForAttacker
	}
	return evalForDefender
}

// quickCaptureCount is a cheap search-ordering heuristic: how many ordinary
// pieces (never the king, which needs the separate surround rule) move m
// would capture, computed directly against b without copying the board. The
// mover's side is inferred from the board itself (whatever piece sits at
// m.From), so callers don't need to pass it separately.
func quickCaptureCount(b *Board, m Move) int {
	mover := b.At(m.From.X, m.From.Y)
	at := func(x, y int) Cell {
		switch {
		case x == m.From.X && y == m.From.Y:
			return Empty
		case x == m.To.X && y == m.To.Y:
			return mover
		default:
			return b.At(x, y)
		}
	}
	sd := Owner(mover)
	enemy := sd.Opponent()
	n := 0
	for _, d := range dirs4 {
		run := 0
		x, y := m.To.X+d.X, m.To.Y+d.Y
		for inBounds(x, y) {
			c := at(x, y)
			if c != Empty && c != King && Owner(c) == enemy {
				run++
				x += d.X
				y += d.Y
				continue
			}
			break
		}
		if run > 0 {
			bracketC := at(x, y)
			bracket := (bracketC == Empty && IsThrone(x, y)) || (bracketC != Empty && Owner(bracketC) == sd)
			if inBounds(x, y) && bracket {
				n += run
			}
		}
	}
	return n
}

// orderedMoves ranks side's legal moves for search: capturing moves first
// (biggest capture count first), everything else after. Good move ordering
// is what makes alpha-beta pruning effective on the very first few plies,
// where the tiny board otherwise gives no other signal.
func orderedMoves(b *Board, side Side) []Move {
	moves := b.LegalMoves(side)
	type scored struct {
		m Move
		s int
	}
	list := make([]scored, len(moves))
	for i, m := range moves {
		list[i] = scored{m, quickCaptureCount(b, m)}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].s > list[j].s })
	out := make([]Move, len(list))
	for i, sc := range list {
		out[i] = sc.m
	}
	return out
}

// BestMove returns the AI's chosen move for side at the given search depth,
// using the evaluation function appropriate to that side. ok is false only
// if side has no legal move at all.
func BestMove(b Board, side Side, depth int) (Move, bool) {
	moves := orderedMoves(&b, side)
	if len(moves) == 0 {
		return Move{}, false
	}
	if depth < 1 {
		depth = 1
	}

	// Fast path: take an immediately winning move (king escape for
	// defenders, king capture for attackers) without searching deeper.
	for _, m := range moves {
		nb, _ := b.Apply(m)
		if w, _, ok := Winner(&nb); ok && w == side {
			return m, true
		}
	}

	eval := evalFor(side)
	opp := side.Opponent()
	best := negInf
	var bestMove Move
	found := false
	alpha, beta := negInf, posInf
	for _, m := range moves {
		nb, _ := b.Apply(m)
		score := minimaxValue(&nb, opp, side, eval, depth-1, alpha, beta)
		if !found || score > best {
			best, bestMove, found = score, m, true
		}
		if best > alpha {
			alpha = best
		}
	}
	return bestMove, found
}

// minimaxValue searches to the given depth, always from aiSide's evaluation
// function: nodes where it is aiSide's own turn maximize the score, nodes
// where it is the opponent's turn minimize it (the opponent is assumed to
// play adversarially against aiSide's goal — the standard single-eval
// approximation for a game whose two sides are not evaluated symmetrically).
func minimaxValue(b *Board, toMove, aiSide Side, eval func(*Board) int, depth, alpha, beta int) int {
	if w, _, ok := Winner(b); ok {
		if w == aiSide {
			return winScore + depth
		}
		return -winScore - depth
	}
	moves := b.LegalMoves(toMove)
	if len(moves) == 0 {
		// toMove has no legal move: toMove's opponent wins immediately.
		if toMove.Opponent() == aiSide {
			return winScore + depth
		}
		return -winScore - depth
	}
	if depth == 0 {
		return eval(b)
	}
	maximizing := toMove == aiSide
	if maximizing {
		best := negInf
		for _, m := range orderedMoves(b, toMove) {
			nb, _ := b.Apply(m)
			v := minimaxValue(&nb, toMove.Opponent(), aiSide, eval, depth-1, alpha, beta)
			if v > best {
				best = v
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
	best := posInf
	for _, m := range orderedMoves(b, toMove) {
		nb, _ := b.Apply(m)
		v := minimaxValue(&nb, toMove.Opponent(), aiSide, eval, depth-1, alpha, beta)
		if v < best {
			best = v
		}
		if best < beta {
			beta = best
		}
		if alpha >= beta {
			break
		}
	}
	return best
}
