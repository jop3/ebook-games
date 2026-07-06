package game

import "image"

// BoardSize is the side length of each player's quilt board.
const BoardSize = 9

// Board is one player's 9x9 quilt: which cells are filled by a patch, and
// (purely cosmetic, for the UI) which of those were placed as a free 1x1
// "special patch" rather than bought — so the board can render them
// distinctly.
type Board struct {
	Filled [BoardSize][BoardSize]bool
	Bonus  [BoardSize][BoardSize]bool
}

// Place marks cells as filled. Callers are expected to have already checked
// legality (canPlaceAt / LegalPlacementsForOrientation).
func (b *Board) Place(cells []image.Point) {
	for _, c := range cells {
		b.Filled[c.Y][c.X] = true
	}
}

// FilledCount returns the number of occupied cells.
func (b *Board) FilledCount() int {
	n := 0
	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			if b.Filled[y][x] {
				n++
			}
		}
	}
	return n
}

// EmptyCount returns the number of unoccupied cells.
func (b *Board) EmptyCount() int { return BoardSize*BoardSize - b.FilledCount() }

// SevenBySeven reports whether any 7x7 window of the board is completely
// filled (there are 3x3=9 candidate window positions on a 9x9 board).
func (b *Board) SevenBySeven() bool {
	const win = 7
	for oy := 0; oy+win <= BoardSize; oy++ {
		for ox := 0; ox+win <= BoardSize; ox++ {
			full := true
			for y := oy; y < oy+win && full; y++ {
				for x := ox; x < ox+win; x++ {
					if !b.Filled[y][x] {
						full = false
						break
					}
				}
			}
			if full {
				return true
			}
		}
	}
	return false
}
