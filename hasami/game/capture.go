package game

import "image"

// corners lists the board's four corner cells.
var corners = [4]image.Point{
	{X: 0, Y: 0}, {X: Size - 1, Y: 0}, {X: 0, Y: Size - 1}, {X: Size - 1, Y: Size - 1},
}

// cornerAdjacent returns the two cells orthogonally adjacent to corner c.
func cornerAdjacent(c image.Point) [2]image.Point {
	dx := 1
	if c.X == Size-1 {
		dx = -1
	}
	dy := 1
	if c.Y == Size-1 {
		dy = -1
	}
	return [2]image.Point{{X: c.X + dx, Y: c.Y}, {X: c.X, Y: c.Y + dy}}
}

// captureScan finds every enemy man captured by side having just moved to at.
// Custodial capture: walk each of the 4 orthogonal directions from at; a
// contiguous run of one or more enemy men that ends (in-bounds) at one of
// side's own men is captured in full. Corner capture: an enemy man sitting in
// a board corner is captured whenever side occupies both cells orthogonally
// adjacent to that corner. Only side's move can capture — the man that just
// moved is never itself considered "sandwiched" (moving into the gap between
// two enemies is always safe), because capture only ever looks outward from
// at for runs of the OPPONENT's men.
func captureScan(b *Board, at image.Point, side Side) []image.Point {
	enemy := side.Opponent()
	var captured []image.Point

	for _, d := range dirs4 {
		var run []image.Point
		x, y := at.X+d.X, at.Y+d.Y
		for inBounds(x, y) && b.At(x, y) == enemy {
			run = append(run, image.Pt(x, y))
			x += d.X
			y += d.Y
		}
		if len(run) > 0 && inBounds(x, y) && b.At(x, y) == side {
			captured = append(captured, run...)
		}
	}

	for _, c := range corners {
		if b.At(c.X, c.Y) != enemy {
			continue
		}
		adj := cornerAdjacent(c)
		if b.At(adj[0].X, adj[0].Y) == side && b.At(adj[1].X, adj[1].Y) == side {
			captured = append(captured, c)
		}
	}

	return captured
}

// Apply plays move m (assumed legal — callers should check IsLegalMove first)
// and returns the resulting board plus the list of enemy cells captured by
// it. The moved man is relocated first, then capture resolution runs from its
// new square for the mover's side only.
func (b Board) Apply(m Move) (Board, []image.Point) {
	mover := b.At(m.From.X, m.From.Y)
	nb := b
	nb.set(m.From.X, m.From.Y, Empty)
	nb.set(m.To.X, m.To.Y, mover)

	captured := captureScan(&nb, m.To, mover)
	for _, p := range captured {
		nb.set(p.X, p.Y, Empty)
	}
	return nb, captured
}
