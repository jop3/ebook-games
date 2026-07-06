package game

import "image"

// isBracket reports whether (x,y) is a square that can serve as the far
// anchor of a custodial capture for side: either one of side's own pieces
// (for SideDefender this includes both ordinary Defenders and the King — the
// king may help capture attackers), or the EMPTY throne, which is hostile
// terrain to BOTH sides (the single most commonly-misremembered rule in fan
// implementations of this game — it is not attacker-only).
//
// Per the spec text, only the throne carries this "hostile when empty"
// status; the four corners are movement-restricted terrain and the king's
// escape squares, but are not separately called out as capture-hostile, so
// we do not extend the hostile-square rule to them (a deliberate, literal
// reading of the spec rather than an invented extra rule).
func isBracket(b *Board, x, y int, side Side) bool {
	if !inBounds(x, y) {
		return false
	}
	c := b.At(x, y)
	if c == Empty {
		return IsThrone(x, y)
	}
	return Owner(c) == side
}

// captureScan finds every ORDINARY enemy piece (Attacker or Defender, never
// the King) captured by side having just moved to at. Custodial capture: walk
// each of the 4 orthogonal directions from at; a contiguous run of one or
// more enemy pieces that ends (in-bounds) at a bracketing square for side
// (see isBracket) is captured in full.
//
// The King is deliberately excluded from ever being swept up in this scan,
// even though it is owned by SideDefender — the king has its own, separate
// capture rule (see isKingSurrounded) requiring 3 or 4 attacking sides
// depending on throne adjacency, never a simple 2-piece sandwich. Only side's
// own move can capture — the piece that just moved is never itself
// considered "sandwiched" (moving into the gap between two enemies is always
// safe), because capture only ever looks outward from at for runs of the
// opponent's ordinary pieces.
func captureScan(b *Board, at image.Point, side Side) []image.Point {
	enemy := side.Opponent()
	var captured []image.Point

	for _, d := range dirs4 {
		var run []image.Point
		x, y := at.X+d.X, at.Y+d.Y
		for inBounds(x, y) {
			c := b.At(x, y)
			if c != Empty && c != King && Owner(c) == enemy {
				run = append(run, image.Pt(x, y))
				x += d.X
				y += d.Y
				continue
			}
			break
		}
		if len(run) > 0 && isBracket(b, x, y, side) {
			captured = append(captured, run...)
		}
	}

	return captured
}

// isKingSurrounded reports whether the king at kp is captured right now: all
// four orthogonal neighbors must be "hostile to the king" — either an
// Attacker piece, or the EMPTY throne. This single rule naturally produces
// all three cases from the spec without hardcoding a side count:
//
//   - King on an open square, nowhere near the throne: none of its neighbors
//     is the (necessarily distant) throne, so all 4 must hold attackers.
//   - King orthogonally adjacent to the (empty) throne: one neighbor IS the
//     throne, automatically hostile, so only the other 3 need attackers.
//   - King actually on the throne: none of ITS neighbors is the throne
//     itself, so — same as the open-square case — all 4 must hold attackers.
//
// A neighbor that is off the board does not count as hostile (the spec only
// grants hostile status to attackers and the empty throne, never the board
// edge), so a king touching the board edge can never be captured on that
// side — a deliberate, spec-literal judgment call.
func isKingSurrounded(b *Board, kp image.Point) bool {
	for _, d := range dirs4 {
		nx, ny := kp.X+d.X, kp.Y+d.Y
		if !inBounds(nx, ny) {
			return false
		}
		c := b.At(nx, ny)
		hostile := c == Attacker || (c == Empty && IsThrone(nx, ny))
		if !hostile {
			return false
		}
	}
	return true
}

// ApplyResult reports what a move captured.
type ApplyResult struct {
	// Captured holds the ordinary (non-king) enemy pieces removed by
	// custodial capture, if any.
	Captured []image.Point
	// KingCaptured is true if this move surrounded and captured the king.
	// Only an attacker's move can ever set this (see Apply).
	KingCaptured bool
	// KingCell is the (now-empty) cell the king occupied, valid only when
	// KingCaptured is true — so the UI can mark it.
	KingCell image.Point
}

// Apply plays move m (assumed legal — callers should check IsLegalMove
// first) and returns the resulting board plus what it captured. The moved
// piece is relocated first, then ordinary custodial capture resolves from
// its new square for the mover's side; finally, if (and only if) the mover is
// an attacker, the king's surround condition is checked (a non-attacker move
// can never capture the king — there is no such thing as marching your own
// king into self-capture).
func (b Board) Apply(m Move) (Board, ApplyResult) {
	mover := b.At(m.From.X, m.From.Y)
	nb := b
	nb.set(m.From.X, m.From.Y, Empty)
	nb.set(m.To.X, m.To.Y, mover)

	side := Owner(mover)
	captured := captureScan(&nb, m.To, side)
	for _, p := range captured {
		nb.set(p.X, p.Y, Empty)
	}

	res := ApplyResult{Captured: captured}
	if side == SideAttacker {
		if kp, ok := nb.KingPos(); ok && isKingSurrounded(&nb, kp) {
			nb.set(kp.X, kp.Y, Empty)
			res.KingCaptured = true
			res.KingCell = kp
		}
	}
	return nb, res
}
