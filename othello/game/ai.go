package game

// ai.go: a compact minimax (negamax with alpha-beta) AI. Evaluation blends
// positional weights (corners are gold, squares next to corners are poison)
// with disc differential and mobility. Depth is small enough to stay snappy on
// the device's CPU; the positional table alone already produces a tough
// opponent for casual play.

// weights is the classic Othello positional value table. Corners are highly
// valued; the X-squares and C-squares next to them are penalized because taking
// them hands the corner to the opponent.
var weights = [Size * Size]int{
	120, -20, 20, 5, 5, 20, -20, 120,
	-20, -40, -5, -5, -5, -5, -40, -20,
	20, -5, 15, 3, 3, 15, -5, 20,
	5, -5, 3, 3, 3, 3, -5, 5,
	5, -5, 3, 3, 3, 3, -5, 5,
	20, -5, 15, 3, 3, 15, -5, 20,
	-20, -40, -5, -5, -5, -5, -40, -20,
	120, -20, 20, 5, 5, 20, -20, 120,
}

const negInf = -1 << 30
const posInf = 1 << 30

// evaluate scores the board from player's perspective (higher = better).
func evaluate(b *Board, player Cell) int {
	opp := player.Opponent()
	pos := 0
	for i, c := range b {
		switch c {
		case player:
			pos += weights[i]
		case opp:
			pos -= weights[i]
		}
	}
	// Mobility: number of legal moves available to each side.
	mob := len(b.LegalMoves(player)) - len(b.LegalMoves(opp))
	return pos + 8*mob
}

// BestMove returns the AI's chosen move for player at the given search depth.
// ok is false only if player has no legal moves. In VariantAnti ("Omvänd
// Othello") the leaf evaluation is sign-flipped (see evalSign), so the same
// negamax search that normally maximises the mover's advantage instead
// maximises their disadvantage — the AI plays toward giving discs away and
// avoiding corners/mobility, the cheap approximation of "fewest discs wins".
func BestMove(b *Board, player Cell, depth int, variant Variant) (move [2]int, ok bool) {
	moves := b.LegalMoves(player)
	if len(moves) == 0 {
		return move, false
	}
	if depth < 1 {
		depth = 1
	}
	sign := evalSign(variant)
	best := negInf
	for _, m := range moves {
		child := *b
		child.Apply(m[0], m[1], player)
		score := -negamax(&child, player.Opponent(), player, depth-1, negInf, posInf, sign)
		if score > best {
			best = score
			move = m
			ok = true
		}
	}
	return move, ok
}

// evalSign returns -1 for VariantAnti (flip the leaf evaluation so the search
// pursues fewer discs instead of more) and +1 for normal Othello.
func evalSign(variant Variant) int {
	if variant == VariantAnti {
		return -1
	}
	return 1
}

// negamax searches to the given depth. toMove is the side to move; root is the
// player we're scoring for. Passing is handled by recursing with the same board
// for the opponent; if neither can move, we score the terminal position.
func negamax(b *Board, toMove, root Cell, depth, alpha, beta, sign int) int {
	if depth == 0 {
		return signedEval(b, toMove, root, sign)
	}
	moves := b.LegalMoves(toMove)
	if len(moves) == 0 {
		if !b.HasMove(toMove.Opponent()) {
			// Terminal: no one can move.
			return signedEval(b, toMove, root, sign)
		}
		// Pass: opponent moves, no depth spent double (still costs a ply).
		return -negamax(b, toMove.Opponent(), root, depth-1, -beta, -alpha, sign)
	}
	best := negInf
	for _, m := range moves {
		child := *b
		child.Apply(m[0], m[1], toMove)
		score := -negamax(&child, toMove.Opponent(), root, depth-1, -beta, -alpha, sign)
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

// signedEval returns evaluate() from toMove's perspective (negamax
// convention), scaled by sign (-1 in VariantAnti to pursue fewer discs).
func signedEval(b *Board, toMove, root Cell, sign int) int {
	return sign * evaluate(b, toMove)
}
