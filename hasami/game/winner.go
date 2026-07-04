package game

// WinMode selects which win condition governs a game.
type WinMode int

const (
	// ModeCapture ("Fångst"): reduce the opponent to a single man.
	ModeCapture WinMode = iota
	// ModeFiveInRow ("Fem i rad"): first to an unbroken line of 5 of their
	// men, anywhere outside their own starting rank, wins.
	ModeFiveInRow
)

// homeRank returns the row index of side's starting rank (Black's is the
// bottom row, White's is the top row).
func homeRank(side Cell) int {
	if side == Black {
		return Size - 1
	}
	return 0
}

// Winner reports the winning side for the given board and mode, or Empty if
// nobody has won (yet). It is a pure board scan and does not need to know
// whose turn it is.
func Winner(b *Board, mode WinMode) Cell {
	switch mode {
	case ModeFiveInRow:
		if hasFiveInRow(b, Black) {
			return Black
		}
		if hasFiveInRow(b, White) {
			return White
		}
		return Empty
	default: // ModeCapture
		bc, wc := b.Count(Black), b.Count(White)
		switch {
		case bc <= 1 && wc > 1:
			return White
		case wc <= 1 && bc > 1:
			return Black
		default:
			return Empty
		}
	}
}

// hasFiveInRow reports whether side has an unbroken line of 5+ men, entirely
// outside side's own home rank, either horizontally or vertically.
//
// "Outside their own starting rank" is read literally here: a qualifying line
// must not include ANY cell of side's home rank (not merely avoid being
// wholly contained in it). Since the home rank is always a board edge (row 0
// or row Size-1), this only ever trims one end of a vertical run — it can
// never split a run in the middle — so a run that reaches into the home rank
// simply cannot count its home-rank cell(s) toward the 5.
func hasFiveInRow(b *Board, side Cell) bool {
	home := homeRank(side)

	// Horizontal lines: the home rank itself is entirely excluded.
	for y := 0; y < Size; y++ {
		if y == home {
			continue
		}
		run := 0
		for x := 0; x < Size; x++ {
			if b.At(x, y) == side {
				run++
				if run >= 5 {
					return true
				}
			} else {
				run = 0
			}
		}
	}

	// Vertical lines: a cell on the home rank breaks the run (can't be
	// counted), but since it's a board edge it can only ever be the run's
	// would-be extreme end, never a mid-run gap.
	for x := 0; x < Size; x++ {
		run := 0
		for y := 0; y < Size; y++ {
			if y == home {
				run = 0
				continue
			}
			if b.At(x, y) == side {
				run++
				if run >= 5 {
					return true
				}
			} else {
				run = 0
			}
		}
	}
	return false
}
