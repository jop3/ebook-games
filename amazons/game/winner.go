package game

// Winner reports the winning side's queen Cell (QueenBlack or QueenWhite) if
// sideToMove has no legal move at all — the last player able to move wins —
// or Empty if sideToMove still has at least one legal turn available. It is
// a pure board scan and does not need any other game state.
func Winner(b *Board, sideToMove Side) Cell {
	if !b.SideHasMove(sideToMove) {
		return sideToMove.Opponent().Queen()
	}
	return Empty
}
