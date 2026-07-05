package game

import "testing"

func TestConnectedTrivialCases(t *testing.T) {
	if !Connected(map[Hex]Side{}) {
		t.Fatal("an empty map should be trivially connected")
	}
	if !Connected(map[Hex]Side{{0, 0, 0}: Black}) {
		t.Fatal("a single tile should be trivially connected")
	}
}

func TestConnectedChainAndSplit(t *testing.T) {
	chain := map[Hex]Side{
		{0, 0, 0}:  Black,
		{1, -1, 0}: White,
		{2, -2, 0}: Black,
	}
	if !Connected(chain) {
		t.Fatal("3 tiles in an adjacent chain should be connected")
	}

	split := map[Hex]Side{
		{0, 0, 0}:  Black,
		{5, -5, 0}: White, // far away, not adjacent to anything
	}
	if Connected(split) {
		t.Fatal("2 tiles with no adjacency should not be reported connected")
	}
}

func TestComponents(t *testing.T) {
	tiles := map[Hex]Side{
		{0, 0, 0}:   Black,
		{1, -1, 0}:  White,
		{5, -5, 0}:  Black,
		{5, -4, -1}: White,
	}
	comps := Components(tiles)
	if len(comps) != 2 {
		t.Fatalf("expected 2 components, got %d: %v", len(comps), comps)
	}
	sizes := map[int]bool{len(comps[0]): true, len(comps[1]): true}
	if !sizes[2] {
		t.Fatalf("expected both components to have size 2, got sizes %v", comps)
	}
}

func TestPlaceMovesEmptyBoardIsWholeBoard(t *testing.T) {
	moves := PlaceMoves(map[Hex]Side{})
	if len(moves) != len(AllPoints()) {
		t.Fatalf("an empty board should allow placing anywhere, got %d want %d", len(moves), len(AllPoints()))
	}
}

// GOTCHA: placement adjacency — once tiles exist, only cells edge-adjacent
// to the cluster are legal, and the cluster stays connected by construction
// (every legal placement, by definition, touches an existing tile).
func TestPlaceMovesAdjacencyRule(t *testing.T) {
	tiles := map[Hex]Side{{0, 0, 0}: Black}
	moves := PlaceMoves(tiles)
	want := Neighbors(Hex{0, 0, 0})
	if len(moves) != len(want) {
		t.Fatalf("with 1 tile down, expected exactly its %d neighbours as legal placements, got %d: %v", len(want), len(moves), moves)
	}
	for _, m := range moves {
		adjacent := false
		for _, n := range want {
			if m == n {
				adjacent = true
			}
		}
		if !adjacent {
			t.Fatalf("placement %v is not adjacent to the sole existing tile", m)
		}
	}
	// A non-adjacent empty cell must NOT be in the legal placement list.
	farAway := Hex{Radius, -Radius, 0}
	for _, m := range moves {
		if m == farAway {
			t.Fatalf("far-away cell %v should not be a legal placement", farAway)
		}
	}
	// Placing atop an occupied cell must never be offered.
	for _, m := range moves {
		if m == (Hex{0, 0, 0}) {
			t.Fatal("an occupied cell must never be offered as a legal placement")
		}
	}
}

// GOTCHA: a move that would disconnect the cluster must be rejected in
// standard (non-advanced) play. Position: a straight chain of 3 tiles,
// A-B-C, where B is the only link between A and C (B's only common
// neighbour of both A and C is itself, verified below), so EVERY move of B
// to anywhere else must be rejected — not merely "any offered move happens
// to be safe" (which would pass vacuously if zero moves were offered).
func TestMoveMovesRejectsDisconnectingMoveInStandardPlay(t *testing.T) {
	a := Hex{0, 0, 0}
	b := Hex{1, -1, 0}
	c := Hex{2, -2, 0}
	tiles := map[Hex]Side{a: Black, b: Black, c: White}

	// Confirm the test setup: no cell other than b is adjacent to both a
	// and c (so moving b anywhere else genuinely cannot avoid a split).
	for _, n := range Neighbors(a) {
		if n == b {
			continue
		}
		for _, n2 := range Neighbors(c) {
			if n == n2 {
				t.Fatalf("test setup error: %v is adjacent to both a and c besides b", n)
			}
		}
	}

	moves := MoveMoves(tiles, Black, false)
	for _, m := range moves {
		if m.From == b {
			t.Fatalf("standard mode must reject every move of b — b is the only cell keeping a/c connected — but offered b->%v", m.To)
		}
	}
}

// GOTCHA (advanced mode): the same disconnecting move IS offered when the
// advanced rule is enabled.
func TestMoveMovesAllowsDisconnectInAdvancedMode(t *testing.T) {
	a := Hex{0, 0, 0}
	b := Hex{1, -1, 0}
	c := Hex{2, -2, 0}
	tiles := map[Hex]Side{a: Black, b: Black, c: White}

	// A destination far from both a and c, but still adjacent to the
	// remaining cluster (rest = {a, c}, which are NOT adjacent to each
	// other, so PlaceMoves(rest) includes neighbours of a AND of c).
	far := Neighbors(a)[0]
	for _, n := range Neighbors(a) {
		if n != b && n != c {
			far = n
			break
		}
	}

	found := false
	for _, m := range MoveMoves(tiles, Black, true) {
		if m.From == b && m.To == far {
			found = true
		}
	}
	if !found {
		t.Fatalf("advanced mode should offer the disconnecting move b(%v)->%v", b, far)
	}

	// And confirm applying it truly disconnects (a alone vs c alone).
	candidate := map[Hex]Side{a: Black, c: White, far: Black}
	if Connected(candidate) {
		t.Fatal("test setup error: the candidate move should actually disconnect")
	}
}

// A move that reconnects two already-split components (by landing on a
// bridge cell touching both) must be legal even in standard mode.
func TestMoveMovesBridgingReconnectIsLegalInStandardPlay(t *testing.T) {
	// Two separate 1-tile groups plus a mover tile positioned so that its
	// only legal destination happens to touch both.
	g1 := Hex{0, 0, 0}
	g2 := Hex{2, 0, -2}
	// bridge is adjacent to both g1 and g2.
	bridge := Hex{1, 0, -1}
	mover := Hex{-1, 1, 0} // adjacent to g1 only, so it CAN move away from g1
	tiles := map[Hex]Side{g1: Black, g2: Black, mover: Black}
	if Connected(tiles) {
		t.Fatal("test setup error: g1/g2/mover should NOT start connected (g2 is isolated)")
	}

	found := false
	for _, m := range MoveMoves(tiles, Black, false) {
		if m.From == mover && m.To == bridge {
			found = true
		}
	}
	if !found {
		t.Fatal("a move that bridges two components back together should be legal even in standard mode")
	}
}
