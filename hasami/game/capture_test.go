package game

import (
	"image"
	"sort"
	"testing"
)

// sortPts gives a deterministic order for comparing captured-cell slices.
func sortPts(pts []image.Point) []image.Point {
	out := append([]image.Point(nil), pts...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].X < out[j].X
	})
	return out
}

func ptsEqual(a, b []image.Point) bool {
	a, b = sortPts(a), sortPts(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- Basic custodial capture (single enemy man, one direction) -------------

func TestCaptureSingleManOneDirection(t *testing.T) {
	b := emptyBoard()
	b.set(2, 4, Black) // mover, about to slide right to (4,4)
	b.set(5, 4, White) // sandwiched
	b.set(6, 4, Black) // the other bracket, already in place
	nb, captured := b.Apply(Move{From: image.Pt(2, 4), To: image.Pt(4, 4)})
	// Independently reasoned expectation: Black now at 4 and 6, with White at
	// 5 bracketed on both sides by Black -> captured.
	if !ptsEqual(captured, []image.Point{{X: 5, Y: 4}}) {
		t.Fatalf("captured = %v, want [(5,4)]", captured)
	}
	if nb.At(5, 4) != Empty {
		t.Fatalf("(5,4) should be empty after capture, got %v", nb.At(5, 4))
	}
	if nb.At(4, 4) != Black || nb.At(6, 4) != Black {
		t.Fatal("both bracketing Black men should remain")
	}
}

// --- Multi-direction capture: one move captures in several directions ------

func TestCaptureMultipleDirectionsAtOnce(t *testing.T) {
	b := emptyBoard()
	// Mover slides from (4,0) down to (4,4), the center. Prepare White men
	// bracketed on three sides (left, right, below) with Black anchors beyond
	// each, so all three capture simultaneously off the single move.
	b.set(4, 0, Black) // mover, moving down the center column to (4,4)
	// Left: White at (3,4), Black anchor at (2,4).
	b.set(3, 4, White)
	b.set(2, 4, Black)
	// Right: White at (5,4), Black anchor at (6,4).
	b.set(5, 4, White)
	b.set(6, 4, Black)
	// Below: White at (4,5), Black anchor at (4,6).
	b.set(4, 5, White)
	b.set(4, 6, Black)
	nb, captured := b.Apply(Move{From: image.Pt(4, 0), To: image.Pt(4, 4)})
	want := []image.Point{{X: 3, Y: 4}, {X: 5, Y: 4}, {X: 4, Y: 5}}
	if !ptsEqual(captured, want) {
		t.Fatalf("captured = %v, want %v (three directions at once)", captured, want)
	}
	for _, p := range want {
		if nb.At(p.X, p.Y) != Empty {
			t.Fatalf("(%d,%d) should be captured/empty", p.X, p.Y)
		}
	}
	if nb.At(4, 4) != Black {
		t.Fatal("the mover should now be at (4,4)")
	}
}

// --- Multi-length run capture: more than one man in a bracketed line -------

func TestCaptureRunOfMultipleMen(t *testing.T) {
	// A run of 3 White men bracketed between a Black man already in place at
	// the far end (4,4) and the mover completing the near end at (0,4).
	b := emptyBoard()
	b.set(1, 4, White)
	b.set(2, 4, White)
	b.set(3, 4, White)
	b.set(4, 4, Black) // far bracket
	b.set(0, 0, Black) // mover, will land at (0,4): the near bracket
	nb, captured := b.Apply(Move{From: image.Pt(0, 0), To: image.Pt(0, 4)})
	want := []image.Point{{X: 1, Y: 4}, {X: 2, Y: 4}, {X: 3, Y: 4}}
	if !ptsEqual(captured, want) {
		t.Fatalf("captured = %v, want %v (whole 3-man run)", captured, want)
	}
	for _, p := range want {
		if nb.At(p.X, p.Y) != Empty {
			t.Fatalf("(%d,%d) should be captured", p.X, p.Y)
		}
	}
}

// --- GOTCHA: safe entry — moving into the gap between two enemies ----------

func TestSafeEntryOrthogonalIsNotSelfCapture(t *testing.T) {
	b := emptyBoard()
	b.set(2, 4, White)
	b.set(4, 4, White)
	b.set(3, 0, Black) // mover, slides down column 3 straight to (3,4)
	nb, captured := b.Apply(Move{From: image.Pt(3, 0), To: image.Pt(3, 4)})
	if len(captured) != 0 {
		t.Fatalf("moving into the gap between two enemies must NOT self-capture, got %v", captured)
	}
	if nb.At(3, 4) != Black {
		t.Fatal("the mover's man must remain on the board")
	}
	if nb.At(2, 4) != White || nb.At(4, 4) != White {
		t.Fatal("the flanking White men must remain (safe entry, not a capture)")
	}
}

func TestSafeEntryAtCorner(t *testing.T) {
	// Moving Black's own man onto the corner cell itself, flanked by White men
	// on both orthogonal neighbors of the corner, must not self-capture: the
	// corner-capture rule only ever removes an ENEMY man sitting in the
	// corner, never the mover's own man that just moved there.
	b := emptyBoard()
	b.set(1, 0, White)
	b.set(0, 1, White)
	b.set(0, 5, Black) // mover, slides up column 0 to the corner (0,0)
	nb, captured := b.Apply(Move{From: image.Pt(0, 5), To: image.Pt(0, 0)})
	if len(captured) != 0 {
		t.Fatalf("mover's own man landing in a flanked corner must not self-capture, got %v", captured)
	}
	if nb.At(0, 0) != Black {
		t.Fatal("the mover must remain at the corner")
	}
	if nb.At(1, 0) != White || nb.At(0, 1) != White {
		t.Fatal("the flanking White men must remain")
	}
}

// --- GOTCHA: corner capture is a separate code path — test all 4 corners ---

func TestCornerCaptureAllFourCorners(t *testing.T) {
	type tc struct {
		name       string
		corner     image.Point
		adjA, adjB image.Point
		moverFrom  image.Point
		moverTo    image.Point // completes the second adjacency
	}
	cases := []tc{
		{
			name: "top_left", corner: image.Pt(0, 0),
			adjA: image.Pt(1, 0), adjB: image.Pt(0, 1),
			moverFrom: image.Pt(0, 5), moverTo: image.Pt(0, 1),
		},
		{
			name: "top_right", corner: image.Pt(8, 0),
			adjA: image.Pt(7, 0), adjB: image.Pt(8, 1),
			moverFrom: image.Pt(8, 5), moverTo: image.Pt(8, 1),
		},
		{
			name: "bottom_left", corner: image.Pt(0, 8),
			adjA: image.Pt(1, 8), adjB: image.Pt(0, 7),
			moverFrom: image.Pt(0, 3), moverTo: image.Pt(0, 7),
		},
		{
			name: "bottom_right", corner: image.Pt(8, 8),
			adjA: image.Pt(7, 8), adjB: image.Pt(8, 7),
			moverFrom: image.Pt(8, 3), moverTo: image.Pt(8, 7),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := emptyBoard()
			b.set(c.corner.X, c.corner.Y, White) // enemy man sitting in the corner
			b.set(c.adjA.X, c.adjA.Y, Black)     // one adjacency already in place
			b.set(c.moverFrom.X, c.moverFrom.Y, Black)
			nb, captured := b.Apply(Move{From: c.moverFrom, To: c.moverTo})
			if !ptsEqual(captured, []image.Point{c.corner}) {
				t.Fatalf("%s: captured = %v, want corner %v captured", c.name, captured, c.corner)
			}
			if nb.At(c.corner.X, c.corner.Y) != Empty {
				t.Fatalf("%s: corner should be emptied by capture", c.name)
			}
			if nb.At(c.adjA.X, c.adjA.Y) != Black || nb.At(c.adjB.X, c.adjB.Y) != Black {
				t.Fatalf("%s: both adjacent Black men should remain", c.name)
			}
		})
	}
}

func TestCornerNotCapturedWithOnlyOneAdjacency(t *testing.T) {
	b := emptyBoard()
	b.set(0, 0, White)
	b.set(1, 0, Black) // only one of the two adjacencies
	b.set(5, 5, Black)
	_, captured := b.Apply(Move{From: image.Pt(5, 5), To: image.Pt(5, 0)})
	if len(captured) != 0 {
		t.Fatalf("a corner man flanked on only one side must not be captured, got %v", captured)
	}
}

// --- GOTCHA: multiple directions resolved before Winner is ever checked ----

func TestMultiDirectionCaptureThenWinnerReflectsIt(t *testing.T) {
	b := emptyBoard()
	// White will be reduced to exactly 1 man by a single multi-direction
	// capturing move, which should immediately end a Fångst game.
	b.set(4, 0, Black)
	b.set(3, 4, White)
	b.set(2, 4, Black)
	b.set(5, 4, White)
	b.set(6, 4, Black)
	b.set(4, 8, White) // the last surviving White man, elsewhere on the board
	nb, captured := b.Apply(Move{From: image.Pt(4, 0), To: image.Pt(4, 4)})
	if len(captured) != 2 {
		t.Fatalf("expected both White men bracketed to be captured, got %v", captured)
	}
	if nb.Count(White) != 1 {
		t.Fatalf("White should be down to 1 man, has %d", nb.Count(White))
	}
	if w := Winner(&nb, ModeCapture); w != Black {
		t.Fatalf("Winner() = %v, want Black once White is reduced to 1 man", w)
	}
}
