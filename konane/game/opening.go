package game

import "image"

// CenterRemovalOptions returns the two center stones Black may remove to
// open the game — the traditional Konane opening. On a fresh board these are
// always Black's own stones (their parity matches Black's checkerboard
// coloring), and every one of their 4 orthogonal neighbors is, by
// construction, a White stone — which is exactly what makes White's
// follow-up removal ("any stone orthogonally adjacent to the gap") always
// have at least one legal option.
func CenterRemovalOptions() [2]image.Point {
	return [2]image.Point{
		{X: Size/2 - 1, Y: Size/2 - 1},
		{X: Size / 2, Y: Size / 2},
	}
}

// isCenterRemovalOption reports whether p is one of the two opening options.
func isCenterRemovalOption(p image.Point) bool {
	for _, o := range CenterRemovalOptions() {
		if o == p {
			return true
		}
	}
	return false
}
