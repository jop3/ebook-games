package game

import (
	"image"
	"testing"
)

// --- GOTCHA: capture-before-suicide ordering ---------------------------------
//
// Setup on a 9x9 board:
//
//	White lone stone at (0,0), with Black already at (1,0).
//	White stones at (1,1) and (0,2), each still holding outside liberties.
//
// Black now plays at (0,1). This simultaneously:
//   - fills White(0,0)'s last liberty (neighbors (1,0)=Black, (0,1)=Black-new)
//     -> White(0,0) must be captured.
//   - looks, BEFORE that capture is resolved, like suicide: Black's new stone
//     at (0,1) has neighbors (0,0)=White, (1,1)=White, (0,2)=White — zero
//     liberties if you check suicide first.
//
// The correct engine order (remove captures, THEN check suicide) makes this
// legal: once White(0,0) is removed, Black(0,1) gains a liberty at (0,0).
// Checking suicide before resolving the capture would wrongly reject this
// legal, capturing move — the classic Go-engine bug this test guards against.
func TestCaptureBeforeSuicideOrdering(t *testing.T) {
	b := NewBoard(9)
	b.Set(image.Pt(0, 0), White)
	b.Set(image.Pt(1, 0), Black)
	b.Set(image.Pt(1, 1), White)
	b.Set(image.Pt(0, 2), White)

	// Sanity: White(1,1) and White(0,2) must each retain outside liberties so
	// only the isolated White(0,0) is captured by this move.
	if libs := Liberties(b, Group(b, image.Pt(1, 1))); len(libs) == 0 {
		t.Fatal("test setup bug: White(1,1) has no liberties before the move")
	}
	if libs := Liberties(b, Group(b, image.Pt(0, 2))); len(libs) == 0 {
		t.Fatal("test setup bug: White(0,2) has no liberties before the move")
	}

	if !Legal(b, image.Pt(0, 1), Black, nil) {
		t.Fatal("a move that captures should be legal even though the placed " +
			"stone would be self-atari/suicide in isolation")
	}

	nb, captured, ok := Place(b, image.Pt(0, 1), Black)
	if !ok {
		t.Fatal("Place should apply the capturing move")
	}
	if len(captured) != 1 || captured[0] != image.Pt(0, 0) {
		t.Fatalf("expected exactly White(0,0) captured, got %v", captured)
	}
	if nb.At(image.Pt(0, 0)) != Empty {
		t.Fatal("captured White stone should be removed from the board")
	}
	if nb.At(image.Pt(0, 1)) != Black {
		t.Fatal("Black's new stone should remain on the board")
	}
	// Black's stone now has exactly the 1 liberty freed up by the capture.
	libs := Liberties(nb, Group(nb, image.Pt(0, 1)))
	if len(libs) != 1 || libs[0] != image.Pt(0, 0) {
		t.Fatalf("expected Black(0,1)'s only liberty to be the freed (0,0), got %v", libs)
	}
	// The other White stones must be untouched (this wasn't a big sweep).
	if nb.At(image.Pt(1, 1)) != White || nb.At(image.Pt(0, 2)) != White {
		t.Fatal("unrelated White stones must survive")
	}
}

// --- GOTCHA: suicide rejection when no capture happens -----------------------
func TestSuicideRejectedWithoutCapture(t *testing.T) {
	b := NewBoard(9)
	// White surrounds (0,0) with liberties elsewhere, so Black playing (0,0)
	// captures nothing and is pure self-atari/suicide.
	b.Set(image.Pt(1, 0), White)
	b.Set(image.Pt(0, 1), White)
	// Give both White stones outside liberties so neither is itself captured.
	if libs := Liberties(b, Group(b, image.Pt(1, 0))); len(libs) < 2 {
		t.Fatal("test setup bug: White(1,0) should have outside liberties")
	}

	if Legal(b, image.Pt(0, 0), Black, nil) {
		t.Fatal("playing into a fully enemy-surrounded point with no capture must be illegal (suicide)")
	}
	_, _, ok := Place(b, image.Pt(0, 0), Black)
	if ok {
		t.Fatal("Place must reject the suicide move")
	}
}

// --- GOTCHA: classic ko shape -------------------------------------------------
//
// Setup (see comment in the test body for the diagram): Black surrounds a
// lone White stone on 3 sides, White recaptures a lone Black stone, creating
// the textbook ko shape where an immediate recapture would exactly restore
// the prior position.
func TestKoClassicShape(t *testing.T) {
	// P0: Black at (2,0),(3,1),(2,2),(1,1); White at (1,0),(0,1),(1,2).
	// (2,1) is the ko point, empty.
	//
	//      (1,0)W (2,0)B
	// (0,1)W (1,1)B (2,1).  (3,1)B
	//      (1,2)W (2,2)B
	p0 := NewBoard(9)
	p0.Set(image.Pt(2, 0), Black)
	p0.Set(image.Pt(3, 1), Black)
	p0.Set(image.Pt(2, 2), Black)
	p0.Set(image.Pt(1, 1), Black)
	p0.Set(image.Pt(1, 0), White)
	p0.Set(image.Pt(0, 1), White)
	p0.Set(image.Pt(1, 2), White)

	// Sanity: Black(1,1) is in atari with its only liberty at the ko point.
	blackLibs := Liberties(p0, Group(p0, image.Pt(1, 1)))
	if len(blackLibs) != 1 || blackLibs[0] != image.Pt(2, 1) {
		t.Fatalf("test setup bug: expected Black(1,1) in atari at (2,1), got libs %v", blackLibs)
	}

	// White plays the ko point, capturing the lone Black(1,1) stone.
	p1, captured, ok := Place(p0, image.Pt(2, 1), White)
	if !ok || len(captured) != 1 || captured[0] != image.Pt(1, 1) {
		t.Fatalf("White's capturing move should succeed capturing (1,1), got ok=%v captured=%v", ok, captured)
	}
	// White's new stone must itself now be a lone stone in atari, its only
	// liberty being the point it just vacated.
	whiteLibs := Liberties(p1, Group(p1, image.Pt(2, 1)))
	if len(whiteLibs) != 1 || whiteLibs[0] != image.Pt(1, 1) {
		t.Fatalf("expected White(2,1) in atari at (1,1), got libs %v", whiteLibs)
	}

	// Black's immediate recapture at (1,1) would exactly restore p0 — this
	// must be forbidden by the ko rule.
	if Legal(p1, image.Pt(1, 1), Black, &p0) {
		t.Fatal("immediate ko recapture must be illegal")
	}
	// Without the ko-check (koPrev == nil), the same move is a perfectly
	// ordinary legal capture — confirming the rejection above is specifically
	// the ko rule, not some other bug.
	if !Legal(p1, image.Pt(1, 1), Black, nil) {
		t.Fatal("the same move must be legal on its own merits once ko is not being checked")
	}

	// The ko lifts once a move intervenes elsewhere: Black plays elsewhere,
	// White plays elsewhere, and THEN Black's recapture at (1,1) is legal
	// again, because the resulting full-board position is no longer an exact
	// repeat of p0 (it now also has the two intervening stones).
	pAfterElsewhere, _, ok := Place(p1, image.Pt(7, 7), Black)
	if !ok {
		t.Fatal("Black's elsewhere move should be legal")
	}
	pAfterElsewhere2, _, ok := Place(pAfterElsewhere, image.Pt(7, 8), White)
	if !ok {
		t.Fatal("White's elsewhere move should be legal")
	}
	// koPrev for this hypothetical next move would be pAfterElsewhere (the
	// position immediately before White's elsewhere move) — not p0 — so the
	// ko no longer applies.
	if !Legal(pAfterElsewhere2, image.Pt(1, 1), Black, &pAfterElsewhere) {
		t.Fatal("ko should have lifted after an intervening move elsewhere")
	}
	nb, recaptured, ok := Place(pAfterElsewhere2, image.Pt(1, 1), Black)
	if !ok || len(recaptured) != 1 || recaptured[0] != image.Pt(2, 1) {
		t.Fatalf("the now-legal recapture should still actually capture White(2,1), got ok=%v captured=%v", ok, recaptured)
	}
	_ = nb
}

// --- LegalMoves sanity --------------------------------------------------------
func TestLegalMovesExcludesOccupiedAndSuicide(t *testing.T) {
	b := NewBoard(9)
	b.Set(image.Pt(4, 4), Black)
	b.Set(image.Pt(1, 0), White)
	b.Set(image.Pt(0, 1), White)
	moves := LegalMoves(b, Black, nil)
	for _, m := range moves {
		if m == image.Pt(4, 4) {
			t.Fatal("LegalMoves must not include an occupied point")
		}
		if m == image.Pt(0, 0) {
			t.Fatal("LegalMoves must not include a suicide point")
		}
	}
	want := 9*9 - 3 /* occupied */ - 1 /* suicide at (0,0) */
	if len(moves) != want {
		t.Fatalf("expected %d legal points (81 - 3 occupied - 1 suicide), got %d", want, len(moves))
	}
}
