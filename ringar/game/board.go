package game

// Side is a player color. The zero value (None) means "empty" for both the
// ring map and the marker map, so a plain map lookup with no "ok" needed
// reads as "nothing there".
type Side int

const (
	None Side = iota
	Black
	White
)

// Opponent returns the other side (None maps to None).
func (s Side) Opponent() Side {
	switch s {
	case Black:
		return White
	case White:
		return Black
	default:
		return None
	}
}

// Board holds the mutable position: which points carry a ring of which
// color, and which carry a marker of which color. A point never carries
// both at once.
type Board struct {
	Rings   map[Point]Side
	Markers map[Point]Side
}

// NewBoard returns an empty board (no rings, no markers placed yet).
func NewBoard() *Board {
	return &Board{Rings: map[Point]Side{}, Markers: map[Point]Side{}}
}

// Clone returns a deep copy, used by the AI search so it can try moves
// without disturbing the real game state.
func (b *Board) Clone() *Board {
	nb := &Board{
		Rings:   make(map[Point]Side, len(b.Rings)),
		Markers: make(map[Point]Side, len(b.Markers)),
	}
	for p, s := range b.Rings {
		nb.Rings[p] = s
	}
	for p, s := range b.Markers {
		nb.Markers[p] = s
	}
	return nb
}

// HasRing/HasMarker report occupancy at p.
func (b *Board) HasRing(p Point) bool   { return b.Rings[p] != None }
func (b *Board) HasMarker(p Point) bool { return b.Markers[p] != None }

// RingCount/MarkerCount count a side's pieces on the board.
func (b *Board) RingCount(side Side) int {
	n := 0
	for _, s := range b.Rings {
		if s == side {
			n++
		}
	}
	return n
}

func (b *Board) MarkerCount(side Side) int {
	n := 0
	for _, s := range b.Markers {
		if s == side {
			n++
		}
	}
	return n
}

// RingMoves returns the legal destination points for the ring at `from`
// (from must currently hold a ring). Rays are walked independently along
// each of the 6 neighbour directions:
//
//   - While the next point is empty, it is a legal destination and the ray
//     keeps sliding (an ordinary rook-like slide over empty points).
//   - Once a marker is encountered, the ray enters "jumping": it keeps
//     consuming consecutive markers (none of them are legal destinations
//     themselves) until it reaches a non-marker point. If that point is
//     empty, it is the single legal destination for the jump and the ray
//     stops there (a ring may never jump more than one contiguous run of
//     markers in a single move). If it is a ring, or off the board, the jump
//     fails and the ray stops with no additional destination.
//   - A ring (of either color) blocks the ray outright: the ray stops and
//     that point is never itself a destination.
func RingMoves(b *Board, from Point) []Point {
	if !b.HasRing(from) {
		return nil
	}
	var out []Point
	for _, d := range Directions {
		out = append(out, rayDestinations(b, from, d)...)
	}
	return out
}

func rayDestinations(b *Board, from, d Point) []Point {
	var out []Point
	jumped := false
	p := from
	for {
		p = p.Add(d)
		if !Valid(p) {
			return out
		}
		if b.HasRing(p) {
			return out
		}
		if b.HasMarker(p) {
			jumped = true
			continue
		}
		// p is empty.
		out = append(out, p)
		if jumped {
			return out // must stop immediately after the jumped run
		}
		// still in the initial empty stretch: keep sliding
	}
}

// direction finds the axis direction and step count connecting from to to
// (from and to must be colinear along one of the 6 directions), used to walk
// the intermediate points of a move for flipping.
func direction(from, to Point) (d Point, steps int, ok bool) {
	for _, dir := range Directions {
		p := from
		n := 0
		for {
			p = p.Add(dir)
			n++
			if !Valid(p) {
				break
			}
			if p == to {
				return dir, n, true
			}
			if n > 2*hexRadius+2 {
				break
			}
		}
	}
	return Point{}, 0, false
}

// IsLegalRingMove reports whether moving the ring at from to to is legal
// right now (from holds a ring belonging to side, and to is one of its
// RingMoves destinations).
func IsLegalRingMove(b *Board, side Side, from, to Point) bool {
	if b.Rings[from] != side {
		return false
	}
	for _, dest := range RingMoves(b, from) {
		if dest == to {
			return true
		}
	}
	return false
}

// ApplyRingMove executes an already-legal ring move: it drops a marker of
// the mover's color at `from`, flips every marker strictly between `from`
// and `to` (all of which are, by construction of RingMoves, markers) to the
// mover's color, and slides the ring itself to `to`. Returns the list of
// flipped points (for UI highlighting).
func ApplyRingMove(b *Board, from, to Point) []Point {
	mover := b.Rings[from]
	d, steps, ok := direction(from, to)
	if !ok {
		return nil
	}
	b.Markers[from] = mover
	var flipped []Point
	p := from
	for i := 1; i < steps; i++ {
		p = p.Add(d)
		b.Markers[p] = b.Markers[p].Opponent()
		flipped = append(flipped, p)
	}
	delete(b.Rings, from)
	b.Rings[to] = mover
	return flipped
}

// RemoveRow clears every marker in pts (the 5 markers of a completed row).
func RemoveRow(b *Board, pts []Point) {
	for _, p := range pts {
		delete(b.Markers, p)
	}
}

// RemoveRing removes the ring at p (the scoring action: p must hold a ring
// of the side claiming the row).
func RemoveRing(b *Board, p Point) {
	delete(b.Rings, p)
}
