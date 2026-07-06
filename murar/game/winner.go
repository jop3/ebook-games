package game

// Winner reports the winning side (whichever pawn sits on its own goal row),
// or ok=false if neither has won yet. It is a pure board scan and does not
// need to know whose turn it is.
func Winner(b *Board) (Side, bool) {
	if b.Pawns[P1].Y == GoalRow(P1) {
		return P1, true
	}
	if b.Pawns[P2].Y == GoalRow(P2) {
		return P2, true
	}
	return P1, false
}
