package game

import "image"

// Place plays c at p on board b. It returns the resulting board, the points
// captured by the move, and whether the move was legal.
//
// Order of operations matters here (the classic Go-engine gotcha): captured
// enemy groups are removed FIRST, and only THEN is the mover's own group
// checked for liberties. A move that captures is legal even if the placed
// stone would have zero liberties in isolation — removing the dead enemy
// stones first is what gives it a liberty. Checking suicide before resolving
// captures would wrongly reject legal capturing moves.
func Place(b Board, p image.Point, c Color) (Board, []image.Point, bool) {
	if !b.InBounds(p) || b.At(p) != Empty {
		return b, nil, false
	}
	nb := b.Clone()
	nb.Set(p, c)

	// 1. Remove any enemy group adjacent to p that is now out of liberties.
	opp := c.Opponent()
	var captured []image.Point
	seenGroup := map[image.Point]bool{}
	for _, n := range nb.Neighbors(p) {
		if nb.At(n) != opp || seenGroup[n] {
			continue
		}
		grp := Group(nb, n)
		for _, q := range grp {
			seenGroup[q] = true
		}
		if len(Liberties(nb, grp)) == 0 {
			for _, q := range grp {
				nb.Set(q, Empty)
			}
			captured = append(captured, grp...)
		}
	}

	// 2. Only now check the mover's own group for suicide.
	ownGroup := Group(nb, p)
	if len(Liberties(nb, ownGroup)) == 0 {
		return b, nil, false // illegal: self-capture with no resulting capture
	}

	return nb, captured, true
}

// Legal reports whether c may legally play at p on board b. koPrev, if
// non-nil, is the board position immediately before the opponent's last
// move; a move that would recreate that exact position is forbidden (simple
// positional ko rule). This only ever blocks the single very-next reply —
// once any other move intervenes, koPrev advances and the position is no
// longer an exact repeat, so the ko lifts naturally.
func Legal(b Board, p image.Point, c Color, koPrev *Board) bool {
	if !b.InBounds(p) || b.At(p) != Empty {
		return false
	}
	nb, _, ok := Place(b, p, c)
	if !ok {
		return false
	}
	if koPrev != nil && Equal(nb, *koPrev) {
		return false
	}
	return true
}

// LegalMoves returns every legal placement for c on board b.
func LegalMoves(b Board, c Color, koPrev *Board) []image.Point {
	var moves []image.Point
	size := b.Size()
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			p := image.Pt(x, y)
			if b.At(p) == Empty && Legal(b, p, c, koPrev) {
				moves = append(moves, p)
			}
		}
	}
	return moves
}
