package game

// Winner reports the winning side for the given board, considering only the
// two board-position win conditions (a pawn on the opponent's back rank, or
// the opponent has zero pawns), or Empty if neither holds (yet). It is a
// pure board scan and does not need to know whose turn it is; it never wins
// on the strength of both conditions together (see the doc comment on each
// case). The third win condition — the side to move has no legal move — is
// turn-dependent and lives in GameState.advance, not here.
func Winner(b *Board) Cell {
	if rowHasPawn(b, GoalRow(Black), Black) {
		return Black
	}
	if rowHasPawn(b, GoalRow(White), White) {
		return White
	}
	if b.Count(White) == 0 {
		return Black
	}
	if b.Count(Black) == 0 {
		return White
	}
	return Empty
}

func rowHasPawn(b *Board, y int, side Cell) bool {
	for x := 0; x < Cols; x++ {
		if b.At(x, y) == side {
			return true
		}
	}
	return false
}
