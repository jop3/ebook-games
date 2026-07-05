package game

import "image"

// axisDirs gives the two opposite unit directions for each of the 4
// geometric axes capture is checked along. These are independent of what
// glyph happens to be drawn in any particular cell — capture always looks
// along all 4 axes through the just-placed ring, unlike direction-forcing
// (ForcedCells) which depends on the specific glyph of one cell.
var axisDirs = map[Orient][2]image.Point{
	OrientH:  {{X: -1, Y: 0}, {X: 1, Y: 0}},
	OrientV:  {{X: 0, Y: -1}, {X: 0, Y: 1}},
	OrientD1: {{X: 1, Y: -1}, {X: -1, Y: 1}}, // "╱": NE / SW
	OrientD2: {{X: 1, Y: 1}, {X: -1, Y: -1}}, // "╲": SE / NW
}

var axes = [4]Orient{OrientH, OrientV, OrientD1, OrientD2}

// captureScan finds every ENEMY ring that flips as a result of mover having
// just placed at "at". This is the rule's custodial capture, and its
// direction is the opposite of Othello's: Othello flips an ENEMY run that
// the mover brackets between two of their own rings; Munkar instead flips
// the two ENEMY rings that bracket the MOVER's own run (which includes the
// ring just placed at "at").
//
// For each of the 4 axes, extend the contiguous run of mover-colored rings
// through "at" as far as it goes in both directions (the run may already
// include older rings from earlier in the game, not just the new one). If
// the single cell immediately beyond BOTH ends of that run holds an
// opponent ring, both of those bookend rings are captured (flip to mover's
// color) — "E Y...Y E" in the spec's shorthand. If only one end (or
// neither) is bounded by an enemy ring, nothing flips on that axis: a
// placement with no bounding enemy on both sides captures nothing, even if
// it sits directly next to a lone enemy ring.
func captureScan(b *Board, at image.Point, mover Cell) []image.Point {
	var flips []image.Point
	enemy := mover.Opponent()

	for _, ax := range axes {
		d := axisDirs[ax]

		left := at
		for {
			n := image.Pt(left.X+d[0].X, left.Y+d[0].Y)
			if inBounds(n.X, n.Y) && b.At(n.X, n.Y) == mover {
				left = n
				continue
			}
			break
		}
		right := at
		for {
			n := image.Pt(right.X+d[1].X, right.Y+d[1].Y)
			if inBounds(n.X, n.Y) && b.At(n.X, n.Y) == mover {
				right = n
				continue
			}
			break
		}

		leftBookend := image.Pt(left.X+d[0].X, left.Y+d[0].Y)
		rightBookend := image.Pt(right.X+d[1].X, right.Y+d[1].Y)
		if inBounds(leftBookend.X, leftBookend.Y) && inBounds(rightBookend.X, rightBookend.Y) &&
			b.At(leftBookend.X, leftBookend.Y) == enemy && b.At(rightBookend.X, rightBookend.Y) == enemy {
			flips = append(flips, leftBookend, rightBookend)
		}
	}
	return flips
}

// Place plays mover's ring at p (assumed legal — callers must check
// LegalMoves/ForcedCells first) and resolves custodial capture from p. It
// returns the resulting board and the list of enemy cells that flipped
// (nil if none), for the UI to briefly flash.
func Place(b Board, p image.Point, mover Cell) (Board, []image.Point) {
	nb := b
	nb.Ring[p.Y][p.X] = mover
	flips := captureScan(&nb, p, mover)
	for _, f := range flips {
		nb.Ring[f.Y][f.X] = mover
	}
	return nb, flips
}
