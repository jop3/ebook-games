package game

import "image"

// LegalPawnMoves returns every cell side's pawn may legally move to this
// turn: the up-to-4 orthogonal steps (filtered by board edges and walls),
// with the standard Quoridor jump/diagonal exception applied whenever the
// opponent's pawn occupies an adjacent cell:
//
//   - If the cell directly beyond the opponent (same direction, 2 cells away)
//     is on the board and not wall-blocked, the side may jump straight over
//     to it.
//   - Otherwise (that beyond-cell is off the board, or a wall blocks the
//     approach to it), the side may instead step diagonally to either of the
//     two cells adjacent to the opponent's pawn — whichever of those two are
//     on the board and not wall-blocked.
func LegalPawnMoves(b *Board, side Side) []image.Point {
	pos := b.Pawns[side]
	opp := side.Opponent()
	oppPos := b.Pawns[opp]

	var out []image.Point
	for _, d := range dirs4 {
		adj := pos.Add(d)
		if !inBounds(adj) || b.wallBetween(pos, adj) {
			continue
		}
		if adj != oppPos {
			// Empty (the only two pawns on the board are side's and opp's,
			// and opp isn't here), so this is a plain legal step.
			out = append(out, adj)
			continue
		}

		// The opponent's pawn sits adjacent in this direction.
		beyond := adj.Add(d)
		if inBounds(beyond) && !b.wallBetween(adj, beyond) {
			out = append(out, beyond)
			continue
		}
		// Straight jump unavailable (off-board or wall-blocked landing):
		// the diagonal exception. Try both perpendicular cells adjacent to
		// the opponent's pawn.
		for _, pd := range perpendiculars(d) {
			diag := adj.Add(pd)
			if !inBounds(diag) || b.wallBetween(adj, diag) {
				continue
			}
			out = append(out, diag)
		}
	}
	return out
}

// IsLegalPawnMove reports whether moving side's pawn to "to" is one of its
// currently legal destinations.
func IsLegalPawnMove(b *Board, side Side, to image.Point) bool {
	for _, m := range LegalPawnMoves(b, side) {
		if m == to {
			return true
		}
	}
	return false
}
