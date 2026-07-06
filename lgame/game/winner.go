package game

// Winner reports whether the game is over given that sideToMove is next to
// act: the L-Game's entire win condition is "if, on your turn, you have no
// legal placement for your L piece, you lose" — evaluated against the
// CURRENT board, before sideToMove would move. If sideToMove has zero legal
// placements, they lose immediately and this returns (the opponent, true).
// Otherwise it returns (Empty, false): the game continues.
func Winner(b Board, sideToMove Side) (winner Side, over bool) {
	if len(LegalLPlacements(b, sideToMove)) == 0 {
		return sideToMove.Opponent(), true
	}
	return Empty, false
}
