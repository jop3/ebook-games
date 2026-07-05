package game

import "testing"

func lineOf(axis Axis, length int) []Point {
	for _, line := range Lines(axis) {
		if len(line) >= length {
			return line[:length]
		}
	}
	return nil
}

// TestFindRowsAllThreeAxes: a monochrome run of 5 is detected on each of the
// 3 axes independently.
func TestFindRowsAllThreeAxes(t *testing.T) {
	for _, axis := range []Axis{AxisA, AxisB, AxisC} {
		b := NewBoard()
		line := lineOf(axis, 5)
		if line == nil {
			t.Fatalf("axis %v: no line of length >= 5 found", axis)
		}
		for _, p := range line {
			b.Markers[p] = Black
		}
		rows := FindRows(b, Black)
		if len(rows) != 1 || len(rows[0].Points) != 5 {
			t.Fatalf("axis %v: FindRows = %v, want exactly one 5-run", axis, rows)
		}
		if rows[0].Axis != axis {
			t.Fatalf("axis %v: row reported wrong axis %v", axis, rows[0].Axis)
		}
		// The opposing color must not see a row here.
		if rows := FindRows(b, White); len(rows) != 0 {
			t.Fatalf("axis %v: White incorrectly sees a row: %v", axis, rows)
		}
	}
}

// TestFindRowsRequiresFive: a run of only 4 is not a row.
func TestFindRowsRequiresFive(t *testing.T) {
	b := NewBoard()
	line := lineOf(AxisA, 4)
	for _, p := range line {
		b.Markers[p] = Black
	}
	if rows := FindRows(b, Black); len(rows) != 0 {
		t.Fatalf("a run of 4 must not count as a row: %v", rows)
	}
}

// TestFindRowsBrokenByOpponentMarker: an interruption breaks the run.
func TestFindRowsBrokenByOpponentMarker(t *testing.T) {
	b := NewBoard()
	line := lineOf(AxisA, 6)
	for i, p := range line {
		if i == 2 {
			b.Markers[p] = White
		} else {
			b.Markers[p] = Black
		}
	}
	if rows := FindRows(b, Black); len(rows) != 0 {
		t.Fatalf("an interrupted run must not count as a row: %v", rows)
	}
}

// TestSixInARowWindowChoice: our documented policy for runs longer than 5 —
// the owner picks which 5-window to claim by tapping any marker in the run;
// the tapped marker's window is chosen (clamped to stay inside the run), and
// the untaken 6th marker remains on the board afterward.
func TestSixInARowWindowChoice(t *testing.T) {
	line := lineOf(AxisA, 6)
	b := NewBoard()
	for _, p := range line {
		b.Markers[p] = Black
	}
	rows := FindRows(b, Black)
	if len(rows) != 1 || len(rows[0].Points) != 6 {
		t.Fatalf("expected one 6-run, got %v", rows)
	}
	run := rows[0].Points

	// Tapping the very first marker selects the leftmost window [0:5].
	win, ok := Window(run, run[0])
	if !ok || win[0] != run[0] || win[4] != run[4] {
		t.Fatalf("tapping the first marker should select window [0:5], got %v", win)
	}
	// Tapping the very last marker selects the rightmost window [1:6]
	// (clamped so the window still fits inside the run).
	win2, ok := Window(run, run[5])
	if !ok || win2[0] != run[1] || win2[4] != run[5] {
		t.Fatalf("tapping the last marker should select window [1:6], got %v", win2)
	}

	// Applying the chosen window leaves exactly the untaken marker behind.
	RemoveRow(b, win2)
	if b.Markers[run[0]] != Black {
		t.Fatalf("the marker outside the chosen window must remain on the board")
	}
	for _, p := range win2 {
		if b.HasMarker(p) {
			t.Fatalf("marker %v should have been removed", p)
		}
	}
}

// TestMultipleRowsFromOneMoveAcrossAxes: a single move flips one marker that
// happens to complete rows on two DIFFERENT axes at once (both axes pass
// through that one shared point) — the resolution loop must offer both, one
// at a time.
func TestMultipleRowsFromOneMoveAcrossAxes(t *testing.T) {
	// Pick an interior point with a full 6-neighbour ring so we can build a
	// 4-run on axis A and a 4-run on axis B, both missing exactly the shared
	// center point, then drop that one marker to complete both simultaneously.
	center := Point{0, 0, 0}
	b := NewBoard()

	lineA := axisLineThrough(AxisA, center)
	lineB := axisLineThrough(AxisB, center)
	idxA := indexOfPoint(lineA, center)
	idxB := indexOfPoint(lineB, center)

	// 4 consecutive Black markers ending right before center on axis A.
	for i := idxA - 4; i < idxA; i++ {
		b.Markers[lineA[i]] = Black
	}
	// 4 consecutive Black markers ending right before center on axis B.
	for i := idxB - 4; i < idxB; i++ {
		b.Markers[lineB[i]] = Black
	}
	b.Markers[center] = Black // completes both runs at once

	rows := FindRows(b, Black)
	if len(rows) != 2 {
		t.Fatalf("expected 2 distinct rows (one per axis) sharing the center point, got %d: %v", len(rows), rows)
	}
	seenAxes := map[Axis]bool{}
	for _, r := range rows {
		seenAxes[r.Axis] = true
	}
	if !seenAxes[AxisA] || !seenAxes[AxisB] {
		t.Fatalf("expected rows on both AxisA and AxisB, got %v", rows)
	}
}

func axisLineThrough(axis Axis, p Point) []Point {
	for _, line := range Lines(axis) {
		for _, q := range line {
			if q == p {
				return line
			}
		}
	}
	return nil
}

func indexOfPoint(line []Point, p Point) int {
	for i, q := range line {
		if q == p {
			return i
		}
	}
	return -1
}
