package game

// Winner reports the winning color for the given (final) board, or Empty for
// a tie. It is a pure piece-count comparison; the game's rule for WHEN to
// call it (board full, or the side to move has no legal move / no pieces) is
// enforced by GameState.advance in state.go, not here.
//
// Unlike custodial-capture games, running out of legal moves in Ataxx is not
// itself a loss: whichever side simply has more men when the game ends wins,
// and a tie is possible (e.g. the game ends on a stuck position, before the
// board fills, with equal counts on both sides).
func Winner(b *Board) Cell {
	bc, wc := b.Count(Black), b.Count(White)
	switch {
	case bc > wc:
		return Black
	case wc > bc:
		return White
	default:
		return Empty
	}
}
