package game

// ai.go: a compact minimax search over Quarto's two-phase turn (choose where
// to place the handed piece, then choose which piece to hand back). Quarto's
// branching factor is large (up to 16 empty cells x 15 remaining pieces), so
// full-depth search is reserved for the endgame; depth is capped for the
// early/mid game to stay snappy on the device's CPU. AILevel scales the depth
// budget (in plies, where placing counts as one ply and giving counts as
// another).

const negInf = -1 << 30
const posInf = 1 << 30

// emptyCells returns the (x,y) of every empty board cell.
func emptyCells(b *Board) [][2]int {
	var out [][2]int
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.Empty(x, y) {
				out = append(out, [2]int{x, y})
			}
		}
	}
	return out
}

// BestPlacement chooses where to place s.ActivePiece for the AI (player to
// move at StepPlace). It always takes an immediate win if one exists;
// otherwise it searches to find the placement that leaves the opponent in
// the worst position, accounting for the give that follows.
func BestPlacement(s *GameState, level int) (x, y int) {
	cells := emptyCells(&s.Board)
	if len(cells) == 0 {
		return 0, 0
	}
	piece := s.ActivePiece
	depth := searchDepth(level, len(s.Pool))

	best := negInf
	bx, by := cells[0][0], cells[0][1]
	for _, c := range cells {
		b2 := s.Board
		b2.Place(c[0], c[1], piece)
		var score int
		if b2.HasWin() {
			score = posInf - 1 // immediate win: best possible, prefer over deeper search
		} else {
			score = -giveMinimax(&b2, s.Pool, depth-1)
		}
		if score > best {
			best = score
			bx, by = c[0], c[1]
		}
	}
	return bx, by
}

// BestGive chooses which pool piece the AI hands to the opponent. It avoids
// handing a piece that lets the opponent win immediately whenever a safe
// alternative exists, and otherwise searches for the piece that leaves the
// opponent (who will place it) worst off.
func BestGive(s *GameState, level int) Piece {
	if len(s.Pool) == 0 {
		return NoPiece
	}
	depth := searchDepth(level, len(s.Pool))

	best := posInf // we want to MINIMIZE the opponent's resulting score
	bestPiece := s.Pool[0]
	for _, p := range s.Pool {
		remaining := removePiece(s.Pool, p)
		score := placeMinimax(&s.Board, p, remaining, depth-1)
		if score < best {
			best = score
			bestPiece = p
		}
	}
	return bestPiece
}

// searchDepth scales the ply budget with level and the size of the
// remaining pool. Quarto's branching factor (empty cells x remaining pieces)
// is largest early in the game, so early-game depth must be capped hard to
// stay fast; as the pool shrinks the same depth costs far less, so the
// endgame can afford — and gets — much deeper search "for free".
func searchDepth(level int, poolSize int) int {
	d := level
	if d < 1 {
		d = 1
	}
	// Cap the base depth itself by how large the branching factor still is,
	// independent of the requested level — a "Svår" search must not be
	// allowed to request a wide-open full-width search at the opening.
	switch {
	case poolSize > 12: // opening: up to 16 cells x 16 pieces per ply
		if d > 2 {
			d = 2
		}
	case poolSize > 8:
		if d > 3 {
			d = 3
		}
	case poolSize > 6:
		if d > 4 {
			d = 4
		}
	}
	// Endgame: few pieces left, search can afford much more depth.
	if poolSize <= 6 {
		d += 4
	} else if poolSize <= 10 {
		d += 1
	}
	if d > poolSize+1 {
		d = poolSize + 1
	}
	return d
}

func removePiece(pool []Piece, p Piece) []Piece {
	out := make([]Piece, 0, len(pool)-1)
	for _, q := range pool {
		if q != p {
			out = append(out, q)
		}
	}
	return out
}

// giveMinimax evaluates the position "to move: choose a piece to give" from
// the perspective of the player who is ABOUT to give (i.e. it returns the
// best score that player can guarantee, where a HIGHER score is better for
// them). Returns 0 (neutral) at depth 0 or when pool is empty (draw-ish).
func giveMinimax(b *Board, pool []Piece, depth int) int {
	if b.Full() {
		return 0 // draw
	}
	if len(pool) == 0 || depth <= 0 {
		return 0
	}
	// The giver wants to minimize what the receiver can score when placing.
	best := posInf
	for _, p := range pool {
		remaining := removePiece(pool, p)
		score := placeMinimax(b, p, remaining, depth-1)
		if score < best {
			best = score
		}
	}
	return -best // negamax sign flip: from the giver's own perspective
}

// placeMinimax evaluates "to move: place piece p on b, then give", returning
// the best score the placer can guarantee (higher = better for the placer).
func placeMinimax(b *Board, p Piece, pool []Piece, depth int) int {
	cells := emptyCells(b)
	if len(cells) == 0 {
		return 0
	}
	best := negInf
	for _, c := range cells {
		b2 := *b
		b2.Place(c[0], c[1], p)
		var score int
		if b2.HasWin() {
			score = posInf - 1
		} else if b2.Full() {
			score = 0
		} else if depth <= 0 {
			score = 0
		} else {
			score = -giveMinimax(&b2, pool, depth-1)
		}
		if score > best {
			best = score
		}
	}
	return best
}
