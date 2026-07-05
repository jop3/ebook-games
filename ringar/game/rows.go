package game

// RowGroup is one maximal monochrome run of length >= 5 found along a
// straight line. Points are ordered along the line. A run longer than 5
// (only possible on the board's longest lines, up to 11 points) is reported
// whole, length 6+: see Window's doc for how the player then picks which
// 5-marker slice of it to claim.
type RowGroup struct {
	Side   Side
	Axis   Axis
	Points []Point
}

// FindRows scans every line on all 3 axes for maximal runs of side's
// markers of length >= 5.
func FindRows(b *Board, side Side) []RowGroup {
	if side == None {
		return nil
	}
	var out []RowGroup
	for axis := AxisA; axis <= AxisC; axis++ {
		for _, line := range Lines(axis) {
			i := 0
			for i < len(line) {
				if b.Markers[line[i]] != side {
					i++
					continue
				}
				j := i
				for j < len(line) && b.Markers[line[j]] == side {
					j++
				}
				if j-i >= 5 {
					pts := make([]Point, j-i)
					copy(pts, line[i:j])
					out = append(out, RowGroup{Side: side, Axis: axis, Points: pts})
				}
				i = j
			}
		}
	}
	return out
}

// Window picks the 5-marker slice of a (possibly longer) completed run that
// the player claims, given a tap on tapped (which must be one of run's
// points). Real YINSH lets the owner of a 6-or-longer run choose which 5
// contiguous markers to remove, leaving the rest on the board (still in
// play, and possibly the seed of a future row) — Ringar exposes that choice
// with a single tap on the run: whichever marker the player taps becomes
// (as closely as possible) the first marker of the removed window, clamped
// so the 5-window stays inside the run.
func Window(run []Point, tapped Point) ([]Point, bool) {
	idx := -1
	for i, p := range run {
		if p == tapped {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, false
	}
	L := len(run)
	start := idx
	if start > L-5 {
		start = L - 5
	}
	if start < 0 {
		start = 0
	}
	win := make([]Point, 5)
	copy(win, run[start:start+5])
	return win, true
}
