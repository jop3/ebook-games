package game

import "math"

// Pattern scores from §9. We evaluate the board after a candidate move and
// reward the AI's own potential lines while penalising the opponent's, so a
// single evaluator yields both offence and defence. "Blocking" emerges
// naturally: a move that removes an opponent threat lowers the opponent term.
const (
	scoreWin       = 1000000 // completed line of WinLength
	scoreOpenFour  = 100000  // WinLength-1 in a row, extendable (win next)
	scoreOpenThree = 1000
	scoreOpenTwo   = 50
)

// opponentWeight makes the AI value denying the opponent slightly less than
// advancing itself for equal-length threats, except for immediate losing
// threats which are weighted to dominate (handled via the open-four tier).
const opponentWeight = 0.9

// BestMove picks the AI's move for the current phase using the heuristic.
// It is deterministic: among equal-scoring moves the first enumerated wins,
// which keeps behaviour reproducible for testing.
func BestMove(b Board, ai Player, phase Phase) (Move, bool) {
	moves := b.ValidMoves(ai, phase)
	if len(moves) == 0 {
		return Move{}, false
	}

	best := moves[0]
	bestScore := math.Inf(-1)
	for _, m := range moves {
		next := b.Apply(m, ai)
		// Immediate win short-circuits everything.
		if next.CheckWin(m.To) == ai {
			return m, true
		}
		score := evaluate(&next, ai)
		if score > bestScore {
			best, bestScore = m, score
		}
	}
	return best, true
}

// evaluate scores a board from ai's perspective: own line potential minus a
// weighted opponent potential.
func evaluate(b *Board, ai Player) float64 {
	opp := ai.Other()
	return lineScore(b, ai) - opponentWeight*lineScore(b, opp)
}

// lineScore sums threat values for every maximal run of who's stones that
// still has room to grow to WinLength. Runs that cannot possibly reach
// WinLength (walled in by edges, blocked cells, or opponent stones on both
// ends with insufficient space) contribute nothing.
//
// To avoid double-counting a run we only start scoring at a run's leading
// cell — i.e. the cell before it (in direction d) is not the same player.
func lineScore(b *Board, who Player) float64 {
	var total float64
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			i := b.Idx(x, y)
			if b.Cells[i] != who {
				continue
			}
			for _, d := range directions {
				if isRunStart(b, x, y, d, who) {
					total += runValue(b, x, y, d, who)
				}
			}
		}
	}
	return total
}

// isRunStart reports whether (x,y) is the first cell of a run in direction d
// (the previous cell is not who).
func isRunStart(b *Board, x, y int, d [2]int, who Player) bool {
	px, py := x-d[0], y-d[1]
	if !b.InBounds(px, py) {
		return true
	}
	return b.Cells[b.Idx(px, py)] != who
}

// runValue measures the run starting at (x,y) in direction d and returns its
// threat score, accounting for whether the surrounding space allows the run
// to still reach WinLength and how open its ends are.
func runValue(b *Board, x, y int, d [2]int, who Player) float64 {
	// Length of the solid run.
	length := 0
	cx, cy := x, y
	for b.InBounds(cx, cy) && b.Cells[b.Idx(cx, cy)] == who {
		length++
		cx += d[0]
		cy += d[1]
	}

	// Open ends: count empty extendable space on each side.
	before := openSpace(b, x-d[0], y-d[1], -d[0], -d[1], who)
	after := openSpace(b, cx, cy, d[0], d[1], who)

	// If even with all reachable space the run cannot make WinLength, it is
	// dead and worthless.
	if length+before+after < b.WinLength {
		return 0
	}

	openEnds := 0
	if before > 0 {
		openEnds++
	}
	if after > 0 {
		openEnds++
	}

	switch {
	case length >= b.WinLength:
		return scoreWin
	case length == b.WinLength-1:
		// One short of winning. Two open ends is unstoppable; one open end
		// is a direct win threat.
		if openEnds == 2 {
			return scoreOpenFour * 2
		}
		return scoreOpenFour
	case length == b.WinLength-2:
		if openEnds == 2 {
			return scoreOpenThree * 2
		}
		return scoreOpenThree
	default:
		// Shorter runs: reward proportionally, doubling for fully open runs.
		v := float64(scoreOpenTwo) * float64(length)
		if openEnds == 2 {
			v *= 2
		}
		return v
	}
}

// openSpace counts consecutive empty, unblocked cells starting at (x,y) and
// walking in direction (dx,dy). Stops at edges, blocked cells, or any stone.
// Cells occupied by who do not extend the count here (they belong to an
// adjacent run) but they don't cap the line either — for simplicity we treat
// any non-empty cell as the boundary of growable space, which is a safe
// underestimate for threat scoring.
func openSpace(b *Board, x, y, dx, dy int, who Player) int {
	space := 0
	for b.InBounds(x, y) {
		i := b.Idx(x, y)
		if b.Blocked[i] || b.Cells[i] != PlayerNone {
			break
		}
		space++
		x += dx
		y += dy
	}
	return space
}
