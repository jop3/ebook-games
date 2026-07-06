package game

import "testing"

// TestBoardHas61Points is the make-or-break geometry check: the spec's own
// figure (a 61-cell radius-5 hexagon, 5 cells per edge) must actually hold
// for our enumeration, not just on paper. See the coords.go doc comment for
// why this file's cube-coordinate hexRadius is 4, not 5 — the two "radius"
// conventions (edge length vs. cube-coordinate radius) differ by one.
func TestBoardHas61Points(t *testing.T) {
	pts := AllPoints()
	if len(pts) != 61 {
		t.Fatalf("AllPoints() returned %d points, want exactly 61", len(pts))
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
		if !Valid(p) {
			t.Fatalf("AllPoints() produced invalid point %v", p)
		}
	}
}

// TestNoCornerTruncation confirms this board, unlike ringar's YINSH board,
// is a PLAIN hexagon: every point at the maximum radius (the 6 true corners
// included) is valid — nothing is snipped off.
func TestNoCornerTruncation(t *testing.T) {
	corners := []Point{
		{4, -4, 0}, {4, 0, -4}, {0, 4, -4},
		{-4, 4, 0}, {-4, 0, 4}, {0, -4, 4},
	}
	for _, c := range corners {
		if !Valid(c) {
			t.Errorf("corner %v should be a valid Staplarna board point (no truncation)", c)
		}
	}
}

// TestNeighbourCounts checks every point's neighbour count and that
// adjacency is symmetric. For a plain hexagon of cube-radius R, the 6 true
// corners have 3 neighbours, the rest of the boundary ring has 4, and every
// interior point has 6.
func TestNeighbourCounts(t *testing.T) {
	pts := AllPoints()
	hist := map[int]int{}
	for _, p := range pts {
		n := Neighbors(p)
		hist[len(n)]++
		if len(n) < 3 || len(n) > 6 {
			t.Errorf("point %v has an out-of-range neighbour count %d", p, len(n))
		}
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
	// 6 corners (3 neighbours), 24 other boundary points (4 neighbours: the
	// ring at radius 4 has 6*4=24 points total, 6 of which are corners),
	// and 30 interior points (6 neighbours) — 6+24+30 = 60... plus the
	// center itself also has 6, giving 61 total. Checked as a histogram
	// rather than assumed, exactly like ringar's equivalent test.
	want := map[int]int{3: 6, 4: 18, 6: 37}
	for k, v := range want {
		if hist[k] != v {
			t.Errorf("neighbour-count histogram[%d] = %d, want %d (full histogram: %v)", k, hist[k], v, hist)
		}
	}
}

func TestDistance(t *testing.T) {
	center := Point{0, 0, 0}
	for _, p := range AllPoints() {
		d := Distance(center, p)
		if d < 0 || d > hexRadius {
			t.Errorf("Distance(center, %v) = %d, out of range [0,%d]", p, d, hexRadius)
		}
	}
	// Neighbours are always exactly distance 1 apart.
	for _, p := range AllPoints() {
		for _, n := range Neighbors(p) {
			if d := Distance(p, n); d != 1 {
				t.Errorf("Distance(%v, neighbour %v) = %d, want 1", p, n, d)
			}
		}
	}
	// A point walked N steps along one direction is exactly distance N away.
	p := Point{0, 0, 0}
	d := Directions[0]
	q := p
	for n := 1; n <= 3; n++ {
		q = q.Add(d)
		if got := Distance(p, q); got != n {
			t.Errorf("Distance after %d steps = %d, want %d", n, got, n)
		}
	}
}
