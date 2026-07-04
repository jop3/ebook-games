package game

import "testing"

// TestBoardHas85Points is the make-or-break geometry check: the spec's own
// arithmetic hint (a radius-5 hexagon has 91 points; YINSH's board is that
// hexagon with its 6 true corners removed, 91-6=85) must actually hold for
// our enumeration, not just on paper.
func TestBoardHas85Points(t *testing.T) {
	pts := AllPoints()
	if len(pts) != 85 {
		t.Fatalf("AllPoints() returned %d points, want exactly 85", len(pts))
	}
	seen := map[Point]bool{}
	for _, p := range pts {
		if p.X+p.Y+p.Z != 0 {
			t.Fatalf("point %v does not satisfy x+y+z=0", p)
		}
		if seen[p] {
			t.Fatalf("duplicate point %v", p)
		}
		seen[p] = true
	}
}

// TestNeighbourCounts checks every point's neighbour count against the
// distribution independently computed in Python before writing this Go
// model (55 points with 6 neighbours, 6 with 5, 12 with 4, 12 with 3 — see
// the task notes). No point should ever end up with 0, 1, 2 or >6
// neighbours, and adjacency must be symmetric.
//
// Note the 6 points with only 5 neighbours are NOT the points on the
// board's outer boundary (max(|x|,|y|,|z|)==5) — they are one step *in*
// from a truncated corner (e.g. (-4,4,0), whose would-be 6th neighbour
// (-5,5,0) is exactly a removed corner). So "interior" in the naive
// max<hexRadius sense does not by itself imply 6 neighbours; the real board
// shape is subtler than a simple radius cutoff, which is exactly why this
// is checked computationally rather than assumed.
func TestNeighbourCounts(t *testing.T) {
	pts := AllPoints()
	hist := map[int]int{}
	for _, p := range pts {
		n := Neighbors(p)
		hist[len(n)]++
		if len(n) < 3 || len(n) > 6 {
			t.Errorf("point %v has an out-of-range neighbour count %d", p, len(n))
		}
		// Every neighbour must itself be a valid board point and list p back
		// (adjacency must be symmetric).
		for _, q := range n {
			if !Valid(q) {
				t.Errorf("Neighbors(%v) returned invalid point %v", p, q)
			}
			back := false
			for _, r := range Neighbors(q) {
				if r == p {
					back = true
				}
			}
			if !back {
				t.Errorf("adjacency not symmetric: %v -> %v but not back", p, q)
			}
		}
	}
	want := map[int]int{3: 12, 4: 12, 5: 6, 6: 55}
	for k, v := range want {
		if hist[k] != v {
			t.Errorf("neighbour-count histogram[%d] = %d, want %d (full histogram: %v)", k, hist[k], v, hist)
		}
	}
}

// TestNoTrueCornersRemain makes sure the 6 excluded corner points really are
// gone, and that InHex would have included them (i.e. they were excluded
// specifically by the corner rule, not by some other accident).
func TestNoTrueCornersRemain(t *testing.T) {
	corners := []Point{
		{5, -5, 0}, {5, 0, -5}, {0, 5, -5},
		{-5, 5, 0}, {-5, 0, 5}, {0, -5, 5},
	}
	for _, c := range corners {
		if !InHex(c) {
			t.Fatalf("corner %v should satisfy InHex", c)
		}
		if Valid(c) {
			t.Fatalf("corner %v should NOT be a valid board point", c)
		}
	}
}

// TestLinesCoverBoardAndAreContiguous verifies the axis-line precomputation
// used by row detection: every point appears on exactly one line per axis,
// every line's points are actually mutually adjacent along the axis (no
// gaps), and the longest line is 11 points (the board's long diagonals) —
// this is what lets FindRows treat "index i and i+1 in a line" as real board
// neighbours without re-checking adjacency. (11 was hand-guessed in the task
// notes before this test was run; the actual longest line, confirmed by an
// independent Python enumeration, is 10 — see the task report.)
func TestLinesCoverBoardAndAreContiguous(t *testing.T) {
	all := AllPoints()
	for axis := AxisA; axis <= AxisC; axis++ {
		lines := Lines(axis)
		total := 0
		longest := 0
		for _, line := range lines {
			total += len(line)
			if len(line) > longest {
				longest = len(line)
			}
			for i := 1; i < len(line); i++ {
				adjacent := false
				for _, n := range Neighbors(line[i-1]) {
					if n == line[i] {
						adjacent = true
					}
				}
				if !adjacent {
					t.Errorf("axis %v: line points %v and %v are not adjacent (gap in line)", axis, line[i-1], line[i])
				}
			}
		}
		if total != len(all) {
			t.Errorf("axis %v: lines cover %d points, want %d", axis, total, len(all))
		}
		if longest != 10 {
			t.Errorf("axis %v: longest line is %d points, want 10", axis, longest)
		}
	}
}
