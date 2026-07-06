package game

import (
	"image"
	"testing"
)

// clear returns an empty board, for tests that want to hand-place tiles
// rather than use the standard starting position.
func clear() Board {
	return Board{}
}

func put(b *Board, x, y int, typ TileType, side Side, f Face) {
	b.set(x, y, &Tile{Type: typ, Side: side, Face: f})
}

func hasDest(actions []Action, kind ActionKind, to image.Point) bool {
	for _, a := range actions {
		if a.Kind == kind && a.To == to {
			return true
		}
	}
	return false
}

func destSet(actions []Action) map[image.Point]ActionKind {
	m := make(map[image.Point]ActionKind, len(actions))
	for _, a := range actions {
		m[a.To] = a.Kind
	}
	return m
}

// --- starting position sanity ------------------------------------------------

func TestNewBoardStartingPosition(t *testing.T) {
	b := NewBoard()
	for _, side := range [2]Side{Black, White} {
		y := homeRow(side)
		if tl := b.At(startDukeX, y); tl == nil || tl.Type != Duke || tl.Side != side || tl.Face != FaceA {
			t.Fatalf("side %v: Duke missing/wrong at (%d,%d): %+v", side, startDukeX, y, tl)
		}
		if tl := b.At(startFootmanX, y); tl == nil || tl.Type != Footman || tl.Side != side {
			t.Fatalf("side %v: Footman missing/wrong at (%d,%d): %+v", side, startFootmanX, y, tl)
		}
		if tl := b.At(startChampX, y); tl == nil || tl.Type != Champion || tl.Side != side {
			t.Fatalf("side %v: Champion missing/wrong at (%d,%d): %+v", side, startChampX, y, tl)
		}
	}
	if b.Count(Black) != 3 || b.Count(White) != 3 {
		t.Fatalf("expected 3 tiles per side on board, got B=%d W=%d", b.Count(Black), b.Count(White))
	}
}

func TestNewReserveHasFourTypes(t *testing.T) {
	r := NewReserve()
	want := map[TileType]bool{Knight: true, Rider: true, DiagGuard: true, Catapult: true}
	for _, t2 := range TroopTypes {
		got := r.Has(t2)
		if got != want[t2] {
			t.Errorf("reserve.Has(%v) = %v, want %v", t2.Name(), got, want[t2])
		}
	}
}

// --- Fotknekt (Footman): short-range slider, both faces ---------------------

func TestFootmanFaceAOrthogonalAdjacent(t *testing.T) {
	b := clear()
	put(&b, 3, 3, Footman, Black, FaceA)
	acts := b.ActionsFrom(image.Pt(3, 3))
	want := []image.Point{{X: 4, Y: 3}, {X: 2, Y: 3}, {X: 3, Y: 4}, {X: 3, Y: 2}}
	if len(acts) != 4 {
		t.Fatalf("FaceA Footman: got %d actions, want 4: %+v", len(acts), acts)
	}
	for _, w := range want {
		if !hasDest(acts, ActRelocate, w) {
			t.Errorf("FaceA Footman missing destination %v; got %+v", w, acts)
		}
	}
}

func TestFootmanFaceBDiagonalAdjacent(t *testing.T) {
	b := clear()
	put(&b, 3, 3, Footman, Black, FaceB)
	acts := b.ActionsFrom(image.Pt(3, 3))
	want := []image.Point{{X: 4, Y: 4}, {X: 4, Y: 2}, {X: 2, Y: 4}, {X: 2, Y: 2}}
	if len(acts) != 4 {
		t.Fatalf("FaceB Footman: got %d actions, want 4: %+v", len(acts), acts)
	}
	for _, w := range want {
		if !hasDest(acts, ActRelocate, w) {
			t.Errorf("FaceB Footman missing destination %v; got %+v", w, acts)
		}
	}
}

func TestFootmanCapturesByLanding(t *testing.T) {
	b := clear()
	put(&b, 3, 3, Footman, Black, FaceA)
	put(&b, 4, 3, Footman, White, FaceA) // enemy directly east
	acts := b.ActionsFrom(image.Pt(3, 3))
	if !hasDest(acts, ActRelocate, image.Pt(4, 3)) {
		t.Fatalf("MoveOrStrike should offer a capturing relocate onto the enemy square: %+v", acts)
	}
	nb, captured := b.Apply(Action{Kind: ActRelocate, From: image.Pt(3, 3), To: image.Pt(4, 3)})
	if len(captured) != 1 || captured[0] != (image.Point{X: 4, Y: 3}) {
		t.Fatalf("captured = %v, want exactly [(4,3)]", captured)
	}
	if tl := nb.At(4, 3); tl == nil || tl.Side != Black || tl.Type != Footman {
		t.Fatalf("mover should now occupy (4,3): %+v", tl)
	}
	if tl := nb.At(4, 3); tl.Face != FaceB {
		t.Fatalf("mover should have flipped to FaceB after acting, got %v", tl.Face)
	}
	if nb.At(3, 3) != nil {
		t.Fatal("origin square should now be empty")
	}
}

func TestFootmanNeverLandsOnOwnTile(t *testing.T) {
	b := clear()
	put(&b, 3, 3, Footman, Black, FaceA)
	put(&b, 4, 3, Footman, Black, FaceA) // own tile east
	acts := b.ActionsFrom(image.Pt(3, 3))
	if hasDest(acts, ActRelocate, image.Pt(4, 3)) {
		t.Fatalf("must not offer a relocate onto an own tile: %+v", acts)
	}
}

// --- Diagonalvakt: diagonal-only on BOTH faces, and FaceB is MoveOnly -------

func TestDiagGuardNeverHasOrthogonalOffset(t *testing.T) {
	for _, f := range []Face{FaceA, FaceB} {
		for _, e := range Patterns(DiagGuard, f) {
			if e.Dx == 0 || e.Dy == 0 {
				t.Fatalf("DiagGuard face %v has a non-diagonal offset (%d,%d)", f, e.Dx, e.Dy)
			}
		}
	}
}

func TestDiagGuardFaceBCannotCapture(t *testing.T) {
	b := clear()
	put(&b, 3, 3, DiagGuard, Black, FaceB)
	put(&b, 5, 5, DiagGuard, White, FaceA) // enemy 2 diagonal squares away
	acts := b.ActionsFrom(image.Pt(3, 3))
	if hasDest(acts, ActRelocate, image.Pt(5, 5)) || hasDest(acts, ActStrike, image.Pt(5, 5)) {
		t.Fatalf("FaceB (MoveOnly) must not be able to act onto an occupied square: %+v", acts)
	}
	// But an empty distance-2 diagonal square is reachable.
	if !hasDest(acts, ActRelocate, image.Pt(1, 1)) {
		t.Fatalf("FaceB should reach the empty diagonal square (1,1): %+v", acts)
	}
}

func TestDiagGuardFaceAStrikesAdjacentDiagonal(t *testing.T) {
	b := clear()
	put(&b, 3, 3, DiagGuard, Black, FaceA)
	put(&b, 4, 4, DiagGuard, White, FaceA)
	acts := b.ActionsFrom(image.Pt(3, 3))
	if !hasDest(acts, ActRelocate, image.Pt(4, 4)) {
		t.Fatalf("FaceA MoveOrStrike should capture-by-landing on the adjacent enemy: %+v", acts)
	}
}

func TestDiagGuardFaceBBlockedByInterveningPiece(t *testing.T) {
	b := clear()
	put(&b, 3, 3, DiagGuard, Black, FaceB)
	put(&b, 4, 4, DiagGuard, White, FaceA) // sits in the path to (5,5)
	acts := b.ActionsFrom(image.Pt(3, 3))
	if hasDest(acts, ActRelocate, image.Pt(5, 5)) {
		t.Fatalf("MoveOnly must be blocked by an intervening piece: %+v", acts)
	}
}

// --- Katapult: strike-only, never relocates ---------------------------------

func TestCatapultNeverHasAMoveEntry(t *testing.T) {
	for _, f := range []Face{FaceA, FaceB} {
		for _, e := range Patterns(Catapult, f) {
			if e.Kind != Strike {
				t.Fatalf("Catapult face %v has a non-Strike entry: %+v", f, e)
			}
		}
	}
}

func TestCatapultStrikesWithoutRelocating(t *testing.T) {
	b := clear()
	put(&b, 3, 3, Catapult, Black, FaceA)
	put(&b, 5, 3, Catapult, White, FaceA) // 2 squares east, empty square between
	acts := b.ActionsFrom(image.Pt(3, 3))
	if !hasDest(acts, ActStrike, image.Pt(5, 3)) {
		t.Fatalf("Catapult FaceA should strike 2 squares east: %+v", acts)
	}
	nb, captured := b.Apply(Action{Kind: ActStrike, From: image.Pt(3, 3), To: image.Pt(5, 3)})
	if len(captured) != 1 || captured[0] != (image.Point{X: 5, Y: 3}) {
		t.Fatalf("captured = %v, want [(5,3)]", captured)
	}
	if nb.At(5, 3) != nil {
		t.Fatal("struck square should now be empty")
	}
	if tl := nb.At(3, 3); tl == nil || tl.Type != Catapult || tl.Side != Black {
		t.Fatalf("the striking Catapult must remain at its own square: %+v", tl)
	}
	if tl := nb.At(3, 3); tl.Face != FaceB {
		t.Fatalf("Catapult should flip to FaceB after striking, got %v", tl.Face)
	}
}

func TestCatapultStrikeBlockedByInterveningPiece(t *testing.T) {
	b := clear()
	put(&b, 3, 3, Catapult, Black, FaceA)
	put(&b, 4, 3, Footman, Black, FaceA) // own tile blocks the line of sight
	put(&b, 5, 3, Catapult, White, FaceA)
	acts := b.ActionsFrom(image.Pt(3, 3))
	if hasDest(acts, ActStrike, image.Pt(5, 3)) {
		t.Fatalf("Strike must require a clear line of sight: %+v", acts)
	}
}

func TestCatapultNoTargetMeansNoActions(t *testing.T) {
	b := clear()
	put(&b, 3, 3, Catapult, Black, FaceA)
	acts := b.ActionsFrom(image.Pt(3, 3))
	if len(acts) != 0 {
		t.Fatalf("with nothing at any of its 4 strike offsets, Catapult should have 0 actions: %+v", acts)
	}
}

// --- Riddare (Knight): the jumper -------------------------------------------

func TestKnightFaceAClassicLeapIgnoresBlockers(t *testing.T) {
	b := clear()
	put(&b, 2, 2, Knight, Black, FaceA)
	// Surround it on all 4 orthogonal/diagonal adjacent squares with own
	// tiles; a slider would be fully blocked, a knight leap ignores it.
	for _, d := range [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
		put(&b, 2+d[0], 2+d[1], Footman, Black, FaceA)
	}
	acts := b.ActionsFrom(image.Pt(2, 2))
	if !hasDest(acts, ActRelocate, image.Pt(4, 3)) {
		t.Fatalf("knight leap (2,1) should ignore the surrounding blockers: %+v", acts)
	}
	if len(acts) != 8 {
		t.Fatalf("Knight FaceA should have all 8 knight leaps available (in-bounds), got %d: %+v", len(acts), acts)
	}
}

func TestKnightJumpCapturesByLanding(t *testing.T) {
	b := clear()
	put(&b, 2, 2, Knight, Black, FaceA)
	put(&b, 4, 3, Knight, White, FaceA) // sits at a knight-leap destination
	acts := b.ActionsFrom(image.Pt(2, 2))
	if !hasDest(acts, ActRelocate, image.Pt(4, 3)) {
		t.Fatalf("jump should be able to capture by landing on an enemy: %+v", acts)
	}
	nb, captured := b.Apply(Action{Kind: ActRelocate, From: image.Pt(2, 2), To: image.Pt(4, 3)})
	if len(captured) != 1 {
		t.Fatalf("captured = %v, want 1 cell", captured)
	}
	if tl := nb.At(4, 3); tl == nil || tl.Side != Black || tl.Type != Knight {
		t.Fatalf("Knight should now occupy (4,3): %+v", tl)
	}
}

func TestKnightFaceBOrthogonalHopIgnoresBlockers(t *testing.T) {
	b := clear()
	put(&b, 2, 2, Knight, Black, FaceB)
	put(&b, 3, 2, Footman, Black, FaceA) // directly in the path to (4,2)
	acts := b.ActionsFrom(image.Pt(2, 2))
	if !hasDest(acts, ActRelocate, image.Pt(4, 2)) {
		t.Fatalf("FaceB orthogonal hop should ignore the intervening piece: %+v", acts)
	}
}

// --- Ryttare (Rider): long-range slider --------------------------------------

func TestRiderFaceARookRayBlockedByFirstPiece(t *testing.T) {
	b := clear()
	put(&b, 0, 3, Rider, Black, FaceA)
	put(&b, 3, 3, Footman, White, FaceA) // enemy at distance 3 east
	acts := b.ActionsFrom(image.Pt(0, 3))
	dests := destSet(acts)
	for x := 1; x <= 2; x++ {
		if k, ok := dests[image.Pt(x, 3)]; !ok || k != ActRelocate {
			t.Errorf("expected an empty-square relocate at (%d,3): dests=%v", x, dests)
		}
	}
	if k, ok := dests[image.Pt(3, 3)]; !ok || k != ActRelocate {
		t.Errorf("expected a capturing relocate onto the enemy at (3,3): dests=%v", dests)
	}
	if _, ok := dests[image.Pt(4, 3)]; ok {
		t.Errorf("ray must stop at the first occupied square, (4,3) should be unreachable: dests=%v", dests)
	}
}

func TestRiderFaceBBishopRay(t *testing.T) {
	b := clear()
	put(&b, 0, 0, Rider, Black, FaceB)
	acts := b.ActionsFrom(image.Pt(0, 0))
	// Only one diagonal direction is even in-bounds from the corner.
	for n := 1; n <= 5; n++ {
		if !hasDest(acts, ActRelocate, image.Pt(n, n)) {
			t.Errorf("bishop ray should reach (%d,%d): %+v", n, n, acts)
		}
	}
}

// --- Kämpe (Champion): mixes MoveOrStrike + Jump on the same face ----------

func TestChampionFaceACombinesSlideAndJump(t *testing.T) {
	b := clear()
	put(&b, 2, 2, Champion, Black, FaceA)
	acts := b.ActionsFrom(image.Pt(2, 2))
	// Adjacent orthogonal MoveOrStrike.
	if !hasDest(acts, ActRelocate, image.Pt(3, 2)) {
		t.Fatalf("Champion FaceA missing adjacent orthogonal destination: %+v", acts)
	}
	// 2-square diagonal Jump.
	if !hasDest(acts, ActRelocate, image.Pt(4, 4)) {
		t.Fatalf("Champion FaceA missing 2-square diagonal jump: %+v", acts)
	}
}

func TestChampionJumpIgnoresBlockerButSlideDoesNot(t *testing.T) {
	b := clear()
	put(&b, 2, 2, Champion, Black, FaceA)
	put(&b, 3, 3, Footman, White, FaceA) // sits between (2,2) and (4,4)
	acts := b.ActionsFrom(image.Pt(2, 2))
	if !hasDest(acts, ActRelocate, image.Pt(4, 4)) {
		t.Fatalf("the diagonal Jump entry must ignore the intervening enemy: %+v", acts)
	}
	// The orthogonal adjacent MoveOrStrike toward (3,2) is unaffected by
	// that diagonal blocker (different offset entirely) and must still work.
	if !hasDest(acts, ActRelocate, image.Pt(3, 2)) {
		t.Fatalf("adjacent orthogonal MoveOrStrike should be unaffected: %+v", acts)
	}
}

// --- Duke: alternates orthogonal/diagonal adjacency -------------------------

func TestDukeAlternatesFaces(t *testing.T) {
	b := clear()
	put(&b, 2, 2, Duke, Black, FaceA)
	actsA := b.ActionsFrom(image.Pt(2, 2))
	if !hasDest(actsA, ActRelocate, image.Pt(3, 2)) || hasDest(actsA, ActRelocate, image.Pt(3, 3)) {
		t.Fatalf("Duke FaceA should be orthogonal-only: %+v", actsA)
	}
	nb, _ := b.Apply(Action{Kind: ActRelocate, From: image.Pt(2, 2), To: image.Pt(3, 2)})
	if tl := nb.At(3, 2); tl.Face != FaceB {
		t.Fatalf("Duke should flip to FaceB after moving, got %v", tl.Face)
	}
	actsB := nb.ActionsFrom(image.Pt(3, 2))
	if !hasDest(actsB, ActRelocate, image.Pt(4, 3)) || hasDest(actsB, ActRelocate, image.Pt(4, 2)) {
		t.Fatalf("Duke FaceB should be diagonal-only: %+v", actsB)
	}
}

// --- flip-after-every-act, generically --------------------------------------

func TestFlipHappensAfterEveryRelocateAndStrike(t *testing.T) {
	for _, typ := range append([]TileType{Duke}, TroopTypes...) {
		b := clear()
		put(&b, 2, 2, typ, Black, FaceA)
		acts := b.ActionsFrom(image.Pt(2, 2))
		if len(acts) == 0 {
			// Catapult needs a target; give it one and retry.
			put(&b, 4, 2, typ, White, FaceA)
			acts = b.ActionsFrom(image.Pt(2, 2))
		}
		if len(acts) == 0 {
			t.Fatalf("%v: no action available to test flip with", typ.Name())
		}
		a := acts[0]
		nb, _ := b.Apply(a)
		var landed *Tile
		if a.Kind == ActStrike {
			landed = nb.At(a.From.X, a.From.Y)
		} else {
			landed = nb.At(a.To.X, a.To.Y)
		}
		if landed == nil || landed.Face != FaceB {
			t.Errorf("%v: expected FaceB after acting, got %+v", typ.Name(), landed)
		}
	}
}

// --- Recruit legality --------------------------------------------------------

func TestRecruitOnlyAdjacentToDukeAndOnlyEmptySquares(t *testing.T) {
	b := clear()
	put(&b, 2, 2, Duke, Black, FaceA)
	put(&b, 3, 2, Footman, Black, FaceA) // occupies one adjacent square
	reserve := NewReserve()
	acts := b.RecruitActions(Black, reserve)
	if hasDest(acts, ActRecruit, image.Pt(3, 2)) {
		t.Fatalf("must not offer recruiting onto an occupied square: %+v", acts)
	}
	wantEmpty := []image.Point{{X: 1, Y: 2}, {X: 2, Y: 3}, {X: 2, Y: 1}}
	for _, sq := range wantEmpty {
		if !hasDest(acts, ActRecruit, sq) {
			t.Errorf("missing recruit option at empty adjacent square %v: %+v", sq, acts)
		}
	}
	// Every recruit-able type should appear at each empty square.
	count := 0
	for _, a := range acts {
		if a.To == wantEmpty[0] {
			count++
		}
	}
	if count != len(reserve.Types()) {
		t.Errorf("expected one recruit action per reserve type at each empty square, got %d want %d", count, len(reserve.Types()))
	}
}

func TestRecruitNoneWhenReserveEmpty(t *testing.T) {
	b := clear()
	put(&b, 2, 2, Duke, Black, FaceA)
	acts := b.RecruitActions(Black, 0)
	if len(acts) != 0 {
		t.Fatalf("empty reserve should yield no recruit actions: %+v", acts)
	}
}

func TestRecruitNoneWhenDukeCaptured(t *testing.T) {
	b := clear()
	acts := b.RecruitActions(Black, NewReserve())
	if len(acts) != 0 {
		t.Fatalf("no Duke on board should yield no recruit actions: %+v", acts)
	}
}

func TestRecruitPlacesOnFaceAAndRemovesFromReserve(t *testing.T) {
	b := clear()
	put(&b, 2, 2, Duke, Black, FaceA)
	reserve := NewReserve()
	a := Action{Kind: ActRecruit, To: image.Pt(3, 2), Recruit: Knight}
	if !b.IsLegalAction(Black, reserve, a) {
		t.Fatal("recruiting a Knight onto an empty adjacent square should be legal")
	}
	// Board.Apply intentionally does not implement ActRecruit (reserve is
	// owned by GameState); use GameState to exercise the full path here.
	s := &GameState{Board: b, Reserve: [2]ReserveMask{Black: reserve, White: NewReserve()}, Turn: Black, Phase: PhasePlaying}
	if !s.Play(a) {
		t.Fatal("Play(recruit) should succeed")
	}
	tl := s.Board.At(3, 2)
	if tl == nil || tl.Type != Knight || tl.Side != Black || tl.Face != FaceA {
		t.Fatalf("recruited tile wrong: %+v", tl)
	}
	if s.Reserve[Black].Has(Knight) {
		t.Fatal("Knight should have been removed from reserve after recruiting")
	}
}

// --- Duke capture ends the game ----------------------------------------------

func TestDukeCaptureEndsGame(t *testing.T) {
	b := clear()
	put(&b, 2, 2, Duke, White, FaceA)
	put(&b, 3, 2, Footman, Black, FaceA)
	put(&b, 0, 5, Duke, Black, FaceA) // Black's own Duke must also be on board
	s := &GameState{
		Board:    b,
		Reserve:  [2]ReserveMask{Black: 0, White: 0},
		Turn:     Black,
		Opponent: OpponentHotseat,
		Phase:    PhasePlaying,
	}
	a := Action{Kind: ActRelocate, From: image.Pt(3, 2), To: image.Pt(2, 2)}
	if !s.Play(a) {
		t.Fatal("capturing the enemy Duke should be a legal move")
	}
	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done once the White Duke is captured")
	}
	winner, ok := s.Winner()
	if !ok || winner != Black {
		t.Fatalf("Winner() = (%v,%v), want (Black,true)", winner, ok)
	}
}

func TestDukeCaptureByStrikeEndsGame(t *testing.T) {
	b := clear()
	put(&b, 5, 3, Duke, White, FaceA)
	put(&b, 3, 3, Catapult, Black, FaceA) // strikes at distance 2 orthogonal
	put(&b, 0, 5, Duke, Black, FaceA)     // Black's own Duke must also be on board
	s := &GameState{
		Board:    b,
		Reserve:  [2]ReserveMask{Black: 0, White: 0},
		Turn:     Black,
		Opponent: OpponentHotseat,
		Phase:    PhasePlaying,
	}
	a := Action{Kind: ActStrike, From: image.Pt(3, 3), To: image.Pt(5, 3)}
	if !s.Play(a) {
		t.Fatal("striking the enemy Duke should be a legal move")
	}
	if s.Phase != PhaseDone {
		t.Fatal("Phase should be Done once the White Duke is struck down")
	}
	winner, ok := s.Winner()
	if !ok || winner != Black {
		t.Fatalf("Winner() = (%v,%v), want (Black,true)", winner, ok)
	}
}
