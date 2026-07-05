package game

// Winner reports the winning side, and whether the game is over at all. A
// side wins the instant either becomes true: its King has captured the
// enemy King (the opposing King is simply absent from the board), or its own
// King sits on the opponent's back rank (goalRank). Both are checked by a
// single pure board scan; it does not need to know whose turn it is.
func Winner(b *Board) (Side, bool) {
	blackKing, whiteKing := false, false
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			p := b[y][x]
			if p == nil || p.Kind != King {
				continue
			}
			switch p.Side {
			case Black:
				blackKing = true
				if y == goalRank(Black) {
					return Black, true
				}
			case White:
				whiteKing = true
				if y == goalRank(White) {
					return White, true
				}
			}
		}
	}
	if !blackKing {
		return White, true
	}
	if !whiteKing {
		return Black, true
	}
	return Black, false
}
