package game

import "testing"

// --- Placement phase: adjacency, turn order, and the transition to moving --

func TestPlaceTileAdjacencyAndTurnOrder(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy, false)
	if s.Phase != PhasePlacement || s.Turn != Black {
		t.Fatalf("new game should start in placement, Black first; got phase=%v turn=%v", s.Phase, s.Turn)
	}

	// The very first tile may land anywhere.
	first := Hex{2, -1, -1}
	if !s.PlaceTile(first) {
		t.Fatal("the very first placement should be legal anywhere on the board")
	}
	if s.Turn != White {
		t.Fatalf("turn should pass to White after Black's placement, got %v", s.Turn)
	}

	// GOTCHA: a non-adjacent placement must be rejected.
	farAway := Hex{-Radius, Radius, 0}
	if s.PlaceTile(farAway) {
		t.Fatal("a placement not touching the existing cluster must be rejected")
	}
	if s.Turn != White {
		t.Fatal("a rejected placement must not consume the turn")
	}

	// An adjacent placement is legal.
	adj := Neighbors(first)[0]
	if !s.PlaceTile(adj) {
		t.Fatalf("placement adjacent to the cluster (%v) should be legal", adj)
	}
	if s.Turn != Black {
		t.Fatalf("turn should pass back to Black, got %v", s.Turn)
	}

	// Placing on an already-occupied cell must be rejected.
	if s.PlaceTile(first) {
		t.Fatal("placing on an occupied cell must be rejected")
	}
}

func TestPlacementPhaseTransitionsToMoving(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy, false)
	for i := 0; i < 2*TilesPerSide; i++ {
		moves := PlaceMoves(s.Board.Tiles)
		if len(moves) == 0 {
			t.Fatalf("placement %d: no legal placements left", i)
		}
		if !s.PlaceTile(moves[0]) {
			t.Fatalf("placement %d: expected the offered move %v to be legal", i, moves[0])
		}
		if s.Phase == PhaseDone {
			// A shape can legitimately complete mid-placement; the test's
			// own PlaceMoves-driven play is not designed to avoid that,
			// so just stop early rather than fail.
			return
		}
	}
	if s.Phase != PhaseMoving {
		t.Fatalf("after all %d tiles are placed with no winner, phase should be Moving, got %v", 2*TilesPerSide, s.Phase)
	}
	if s.Remaining[Black] != 0 || s.Remaining[White] != 0 {
		t.Fatalf("both supplies should be empty, got Black=%d White=%d", s.Remaining[Black], s.Remaining[White])
	}
}

// --- Movement phase: disconnect rejection (standard) ------------------------

func TestMoveTileRejectsDisconnectInStandardMode(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy, false)
	s.Phase = PhaseMoving
	s.Remaining[Black], s.Remaining[White] = 0, 0
	a, b, c := Hex{0, 0, 0}, Hex{1, -1, 0}, Hex{2, -2, 0}
	s.Board.Tiles = map[Hex]Side{a: Black, b: Black, c: White}
	s.Turn = Black

	far := Hex{-4, 4, 0} // not adjacent to a or c
	if s.MoveTile(b, far) {
		t.Fatal("a move that disconnects the board must be rejected in standard mode")
	}
	if s.Board.Tiles[b] != Black || s.Turn != Black {
		t.Fatal("a rejected move must leave the board and turn unchanged")
	}
}

// --- Movement phase: advanced-mode disconnect, strand, and the <=5 loss ----

// TestMoveTileAdvancedSplitStrandsAndChecksFiveTileLoss builds a position
// where White has 8 tiles on the board: 5 attached to a Black "hub" tile
// directly, and 3 attached ONLY via a second Black tile ("bridge") that sits
// one hop further out. When bridge moves away (advanced mode) to a
// destination that reattaches to the hub side only, the 3-tile White branch
// is left in a component that does NOT contain the moved tile and must be
// stranded and removed outright — leaving White with exactly 5 tiles left
// (Remaining 0 + 5 on board), which must immediately lose under the
// advanced rule's "<=5 tiles left" condition.
func TestMoveTileAdvancedSplitStrandsAndChecksFiveTileLoss(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy, true)
	s.Phase = PhaseMoving
	// Black keeps a comfortable supply so this test isolates White's tile
	// count as the only one near the <=5 boundary (this minimal position
	// only puts 2 Black tiles on the board, which must not itself trip the
	// same check).
	s.Remaining[Black], s.Remaining[White] = 10, 0

	hub := Hex{0, 0, 0}
	bridge := Hex{1, -1, 0}
	stay := []Hex{{-1, 1, 0}, {-2, 2, 0}, {-3, 3, 0}, {-4, 4, 0}, {-5, 5, 0}}
	goBranch := []Hex{{2, -2, 0}, {3, -3, 0}, {4, -4, 0}}

	tiles := map[Hex]Side{hub: Black, bridge: Black}
	for _, p := range stay {
		tiles[p] = White
	}
	for _, p := range goBranch {
		tiles[p] = White
	}
	s.Board.Tiles = tiles
	s.Turn = Black

	if !Connected(s.Board.Tiles) {
		t.Fatal("test setup error: initial position should be one connected cluster")
	}

	to := Hex{0, 1, -1} // adjacent to hub only, not to the goBranch chain
	if s.MoveTile(bridge, to) != true {
		t.Fatalf("the advanced-mode disconnecting move bridge(%v)->%v should be accepted", bridge, to)
	}

	for _, p := range goBranch {
		if s.Board.HasTile(p) {
			t.Fatalf("stranded tile at %v should have been removed from the board", p)
		}
	}
	for _, p := range stay {
		if s.Board.Tiles[p] != White {
			t.Fatalf("tile at %v should still be on the board (it stayed connected via hub)", p)
		}
	}
	if s.Board.Count(White) != 5 {
		t.Fatalf("White should have exactly 5 tiles left on the board, got %d", s.Board.Count(White))
	}
	if s.TilesLeft(White) != 5 {
		t.Fatalf("TilesLeft(White) = %d, want 5", s.TilesLeft(White))
	}
	if s.Phase != PhaseDone {
		t.Fatalf("White dropping to exactly 5 tiles should end the game, got phase=%v", s.Phase)
	}
	if s.WinSide != Black {
		t.Fatalf("Black should win when White is reduced to <=5 tiles, got winner=%v", s.WinSide)
	}
}

// A stranding move that leaves the affected side with MORE than 5 tiles must
// NOT end the game.
func TestMoveTileAdvancedSplitAboveFiveContinues(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy, true)
	s.Phase = PhaseMoving
	// Black keeps a comfortable supply (well above the <=5 threshold) so
	// this test isolates White's tile count as the only thing near the
	// boundary — Black itself must not be the one accidentally tripping
	// the <=5 check just because this minimal test position only gives it
	// 2 tiles on the board.
	s.Remaining[Black], s.Remaining[White] = 10, 0

	hub := Hex{0, 0, 0}
	bridge := Hex{1, -1, 0}
	// 6 tiles stay attached to hub (well above the 5-tile threshold even
	// after the single tile in goBranch is stranded).
	stay := []Hex{{-1, 1, 0}, {-2, 2, 0}, {-3, 3, 0}, {-4, 4, 0}, {-5, 5, 0}, {-1, 0, 1}}
	goBranch := []Hex{{2, -2, 0}}

	tiles := map[Hex]Side{hub: Black, bridge: Black}
	for _, p := range stay {
		tiles[p] = White
	}
	for _, p := range goBranch {
		tiles[p] = White
	}
	s.Board.Tiles = tiles
	s.Turn = Black

	to := Hex{0, 1, -1}
	if !s.MoveTile(bridge, to) {
		t.Fatal("the advanced-mode disconnecting move should be accepted")
	}
	if s.Board.Count(White) != 6 {
		t.Fatalf("White should have 6 tiles left (only the 1-tile branch stranded), got %d", s.Board.Count(White))
	}
	if s.Phase == PhaseDone {
		t.Fatal("White still having 6 tiles left should NOT end the game")
	}
	if s.Turn != White {
		t.Fatalf("turn should pass to White, got %v", s.Turn)
	}
}

// --- A genuine shape win, end to end, via the real state machine ------------

func TestPlaceTileTriggersLineWin(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy, false)
	s.Phase = PhasePlacement
	line := []Hex{{-3, 3, 0}, {-2, 2, 0}, {-1, 1, 0}, {0, 0, 0}, {1, -1, 0}, {2, -2, 0}}
	for _, p := range line[:5] {
		s.Board.Tiles[p] = Black
	}
	s.Remaining[Black] = TilesPerSide - 5
	s.Turn = Black

	if !s.PlaceTile(line[5]) {
		t.Fatal("the 6th placement completing the line should be legal")
	}
	if s.Phase != PhaseDone || s.WinSide != Black || s.WinKind != ShapeLine {
		t.Fatalf("expected a completed Black line win, got phase=%v winner=%v kind=%v", s.Phase, s.WinSide, s.WinKind)
	}
	if !hexSetEqual(s.WinCells, line) {
		t.Fatalf("WinCells = %v, want %v", s.WinCells, line)
	}
}

func TestMoveTileTriggersHexRingWin(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy, false)
	s.Phase = PhaseMoving
	s.Remaining[Black], s.Remaining[White] = 0, 0

	center := Hex{0, 0, 0}
	ring := Neighbors(center)
	// 5 of the 6 ring cells already Black; the 6th ring cell instead holds
	// a Black tile SOMEWHERE ELSE (one hop further out from ring[0], still
	// attached to the other 5 via ring[0]) that will slide into the final
	// ring cell.
	parked := ring[0].Add(Directions[0])
	tiles := map[Hex]Side{}
	for _, p := range ring[:5] {
		tiles[p] = Black
	}
	tiles[parked] = Black
	s.Board.Tiles = tiles
	s.Turn = Black

	if !Connected(s.Board.Tiles) {
		t.Fatal("test setup error: position should start connected")
	}
	if !s.MoveTile(parked, ring[5]) {
		t.Fatalf("sliding the parked tile %v into the final ring cell %v should be legal", parked, ring[5])
	}
	if s.Phase != PhaseDone || s.WinSide != Black || s.WinKind != ShapeHexRing {
		t.Fatalf("expected a completed Black hex-ring win, got phase=%v winner=%v kind=%v", s.Phase, s.WinSide, s.WinKind)
	}
}

// --- TilesLeft bookkeeping ---------------------------------------------------

func TestTilesLeftAccounting(t *testing.T) {
	s := NewGame(OpponentHotseat, DepthEasy, false)
	if s.TilesLeft(Black) != TilesPerSide || s.TilesLeft(White) != TilesPerSide {
		t.Fatalf("a fresh game should have %d tiles left per side", TilesPerSide)
	}
	s.PlaceTile(Hex{0, 0, 0})
	if s.TilesLeft(Black) != TilesPerSide {
		t.Fatal("TilesLeft should be invariant across an ordinary placement (hand -1, board +1)")
	}
}
