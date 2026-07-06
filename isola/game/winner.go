package game

// GameOver reports whether toMove has already lost: Isola's only win
// condition is that the side to move has zero legal pawn moves on its own
// turn (no straight-line queen move to any present, unoccupied tile).
func GameOver(b *Board, toMove Side) bool {
	return len(b.LegalMoves(toMove)) == 0
}

// Winner returns the winning side given that toMove is the side about to
// play, or Empty if the game isn't over yet. toMove loses (its opponent
// wins) exactly when it has no legal move at all.
func Winner(b *Board, toMove Side) Side {
	if GameOver(b, toMove) {
		return toMove.Opponent()
	}
	return Empty
}
