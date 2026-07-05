package game

import "image"

// AreaScore computes the area (Chinese) score of board b: each side's score
// is its stones on the board plus every empty region whose entire border
// touches only that color, then komi is added to White. dead marks stones
// that were tapped as dead in the end-of-game marking phase; those points
// are treated as empty (belonging to whichever color's territory now
// surrounds them) before scoring.
//
// A region bordering both colors (or, degenerately, no stones at all — e.g.
// the empty board) counts for neither side: it is a neutral/dame point, not
// anyone's area. This matters for seki-like shapes and single dame points
// that touch both colors.
func AreaScore(b Board, dead map[image.Point]bool, komi float64) (black, white float64) {
	sb := b.Clone()
	size := sb.Size()
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			p := image.Pt(x, y)
			if dead[p] {
				sb.Set(p, Empty)
			}
		}
	}

	visited := make([][]bool, size)
	for i := range visited {
		visited[i] = make([]bool, size)
	}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			p := image.Pt(x, y)
			switch sb.At(p) {
			case Black:
				black++
			case White:
				white++
			default:
				if visited[y][x] {
					continue
				}
				region := Group(sb, p)
				for _, q := range region {
					visited[q.Y][q.X] = true
				}
				borders := BorderColors(sb, region)
				if len(borders) == 1 {
					n := float64(len(region))
					if borders[Black] {
						black += n
					} else if borders[White] {
						white += n
					}
				}
			}
		}
	}

	white += komi
	return black, white
}
