package game

import (
	"image"
	"testing"
)

// --- GOTCHA: the capture direction is the OPPOSITE of Othello ---------------
//
// In Othello, the MOVER brackets an ENEMY run between two of the mover's own
// discs, and the enemy run flips to the mover. In Munkar it is inverted: the
// MOVER's own run (the color that was just placed) is what must sit bounded
// between two ENEMY rings, and it is those two ENEMY bookends that flip to
// the mover's color. Every test below places Black and asserts that WHITE
// (the enemy bookends) flips to BLACK (the mover) — never the other way
// around — and separately confirms Black's own newly-placed run is left
// exactly as placed (it does not itself get "captured" by anything).

// TestCaptureInvertedDirection_SingleGap is the spec's own "O_O -> OXO =>
// XXX" example, spelled out with our colors: two White rings 2 apart with an
// empty gap; Black fills the gap; BOTH White bookends must flip to Black
// (not the reverse — Black's new ring must not itself flip, and White must
// not somehow capture Black).
func TestCaptureInvertedDirection_SingleGap(t *testing.T) {
	var b Board
	b.Ring[0][1] = White // left bookend
	b.Ring[0][3] = White // right bookend
	// (0,2) empty: Black plays there.
	nb, flips := Place(b, image.Pt(2, 0), Black)

	if nb.At(2, 0) != Black {
		t.Fatal("the newly-placed ring must remain Black, not flip")
	}
	if nb.At(1, 0) != Black || nb.At(3, 0) != Black {
		t.Fatalf("both White bookends must flip to Black; got left=%v right=%v", nb.At(1, 0), nb.At(3, 0))
	}
	if len(flips) != 2 {
		t.Fatalf("expected exactly 2 flips (both White bookends), got %v", flips)
	}
	for _, f := range flips {
		if f != (image.Point{X: 1, Y: 0}) && f != (image.Point{X: 3, Y: 0}) {
			t.Errorf("unexpected flip at %v", f)
		}
	}
}

// TestCaptureInvertedDirection_RunOfTwo is the spec's "OXX_O -> OXXXO =>
// XXXXX" example: Black already has a run of 2 with a White ring to its
// left and an empty gap + White ring to its right; filling the gap extends
// Black's run to 3, bounded by White on both ends, so BOTH White rings flip
// to Black (never the reverse).
func TestCaptureInvertedDirection_RunOfTwo(t *testing.T) {
	var b Board
	b.Ring[0][0] = White // left bookend
	b.Ring[0][1] = Black
	b.Ring[0][2] = Black
	// (0,3) empty
	b.Ring[0][4] = White // right bookend
	nb, flips := Place(b, image.Pt(3, 0), Black)

	for x := 1; x <= 3; x++ {
		if nb.At(x, 0) != Black {
			t.Fatalf("Black's run at (%d,0) should remain/become Black, got %v", x, nb.At(x, 0))
		}
	}
	if nb.At(0, 0) != Black || nb.At(4, 0) != Black {
		t.Fatalf("both White bookends must flip to Black; got left=%v right=%v", nb.At(0, 0), nb.At(4, 0))
	}
	if len(flips) != 2 {
		t.Fatalf("expected exactly 2 flips, got %v", flips)
	}
}

// TestCaptureNoFlipWithoutBothBookends confirms a placement next to a lone
// enemy ring (only ONE side bounded, the other side open board/empty)
// captures nothing at all — the rule requires enemy on BOTH ends.
func TestCaptureNoFlipWithoutBothBookends(t *testing.T) {
	var b Board
	b.Ring[0][1] = White // one neighbor is an enemy ring...
	// ...but (0,3) and beyond are empty: no bookend on the right at all.
	_, flips := Place(b, image.Pt(2, 0), Black)
	if len(flips) != 0 {
		t.Fatalf("no flips expected with only one side bounded by an enemy, got %v", flips)
	}

	// Also: enemy on one side, mover's own color already on the other side
	// (not an enemy) — still no capture, since the far bookend isn't an
	// enemy ring.
	var b2 Board
	b2.Ring[0][1] = White
	b2.Ring[0][3] = Black
	_, flips2 := Place(b2, image.Pt(2, 0), Black)
	if len(flips2) != 0 {
		t.Fatalf("no flips expected when the far side is the mover's own color, got %v", flips2)
	}
}

// TestCaptureNoSelfFlip places a ring into a gap flanked by the mover's OWN
// color on both sides (Black _ Black -> Black Black Black): nothing is an
// enemy here, so nothing flips — confirms captures only ever target enemy
// bookends, never re-color the mover's own already-placed rings.
func TestCaptureNoSelfFlip(t *testing.T) {
	var b Board
	b.Ring[0][1] = Black
	b.Ring[0][3] = Black
	nb, flips := Place(b, image.Pt(2, 0), Black)
	if len(flips) != 0 {
		t.Fatalf("no enemy present, expected no flips, got %v", flips)
	}
	if nb.At(1, 0) != Black || nb.At(2, 0) != Black || nb.At(3, 0) != Black {
		t.Fatal("all three Black rings should remain Black")
	}
}

// TestCaptureVertical and TestCaptureDiagonals confirm the inverted-capture
// direction holds on every one of the 4 axes, not just the horizontal case
// tested above.
func TestCaptureVertical(t *testing.T) {
	var b Board
	b.Ring[1][3] = White
	b.Ring[3][3] = White
	_, flips := Place(b, image.Pt(3, 2), Black)
	want := map[image.Point]bool{{X: 3, Y: 1}: true, {X: 3, Y: 3}: true}
	if len(flips) != 2 {
		t.Fatalf("vertical capture: got %v, want 2 flips", flips)
	}
	for _, f := range flips {
		if !want[f] {
			t.Errorf("unexpected vertical flip at %v", f)
		}
	}
}

func TestCaptureDiagonalRising(t *testing.T) {
	// "╱" axis (x+y varies by (1,-1)/(−1,1)): White at (1,3) and (3,1),
	// Black fills the gap at (2,2).
	var b Board
	b.Ring[3][1] = White
	b.Ring[1][3] = White
	_, flips := Place(b, image.Pt(2, 2), Black)
	want := map[image.Point]bool{{X: 1, Y: 3}: true, {X: 3, Y: 1}: true}
	if len(flips) != 2 {
		t.Fatalf("╱ diagonal capture: got %v, want 2 flips", flips)
	}
	for _, f := range flips {
		if !want[f] {
			t.Errorf("unexpected ╱ diagonal flip at %v", f)
		}
	}
}

func TestCaptureDiagonalFalling(t *testing.T) {
	// "╲" axis: White at (1,1) and (3,3), Black fills the gap at (2,2).
	var b Board
	b.Ring[1][1] = White
	b.Ring[3][3] = White
	_, flips := Place(b, image.Pt(2, 2), Black)
	want := map[image.Point]bool{{X: 1, Y: 1}: true, {X: 3, Y: 3}: true}
	if len(flips) != 2 {
		t.Fatalf("╲ diagonal capture: got %v, want 2 flips", flips)
	}
	for _, f := range flips {
		if !want[f] {
			t.Errorf("unexpected ╲ diagonal flip at %v", f)
		}
	}
}

// TestCaptureMultiAxis confirms up to 2 flips per axis, and that multiple
// axes can each independently capture off a single placement.
func TestCaptureMultiAxis(t *testing.T) {
	var b Board
	// Horizontal bracket around (2,2).
	b.Ring[2][1] = White
	b.Ring[2][3] = White
	// Vertical bracket around (2,2).
	b.Ring[1][2] = White
	b.Ring[3][2] = White
	_, flips := Place(b, image.Pt(2, 2), Black)
	if len(flips) != 4 {
		t.Fatalf("expected 4 flips (2 axes x 2 bookends), got %v", flips)
	}
}

// TestCaptureFromWhitesPerspective runs the same inverted-direction check
// with White as the mover, to make sure the rule isn't accidentally
// hardcoded to only work for Black: Black bookends must flip to White.
func TestCaptureFromWhitesPerspective(t *testing.T) {
	var b Board
	b.Ring[0][1] = Black
	b.Ring[0][3] = Black
	nb, flips := Place(b, image.Pt(2, 0), White)
	if nb.At(1, 0) != White || nb.At(3, 0) != White {
		t.Fatalf("both Black bookends must flip to White; got left=%v right=%v", nb.At(1, 0), nb.At(3, 0))
	}
	if len(flips) != 2 {
		t.Fatalf("expected exactly 2 flips, got %v", flips)
	}
}
