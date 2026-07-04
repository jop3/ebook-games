package game

import "testing"

func TestNewGameSeededSetsUpFactories(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 42)
	for i, f := range gs.Factories {
		if len(f) != FactoryFill {
			t.Errorf("factory %d has %d tiles, want %d", i, len(f), FactoryFill)
		}
	}
	if !gs.CenterHasStart {
		t.Error("the start marker should begin in the center")
	}
	if len(gs.Center) != 0 {
		t.Error("the center should start empty of tiles")
	}
	if gs.Turn != 0 || gs.Phase != PhasePlaying {
		t.Fatalf("fresh game should have player 0 to move in PhasePlaying, got turn=%d phase=%v", gs.Turn, gs.Phase)
	}
}

func TestLegalMovesFloorAlwaysOffered(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 7)
	gs.Factories[0] = []Color{ColorSolid, ColorSolid, ColorRing, ColorDot}
	moves := LegalMoves(gs, 0)
	sawFloorFor := map[Color]bool{}
	for _, m := range moves {
		if m.Source == 0 && m.TargetLine == -1 {
			sawFloorFor[m.Color] = true
		}
	}
	for _, c := range []Color{ColorSolid, ColorRing, ColorDot} {
		if !sawFloorFor[c] {
			t.Errorf("floor should always be a legal target for color %d from factory 0", c)
		}
	}
}

// GOTCHA (overflow): taking more tiles than a pattern line has room for
// routes the extras to the floor.
func TestApplyOverflowToFloor(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 8)
	gs.Factories[0] = []Color{ColorSolid, ColorSolid, ColorSolid, ColorRing}
	gs.Turn = 0
	if !gs.Play(Move{Source: 0, Color: ColorSolid, TargetLine: 0}) { // line 0 has cap 1
		t.Fatal("taking all 3 Solid tiles onto line 0 should be legal (overflow allowed)")
	}
	b := &gs.Boards[0]
	if len(b.Lines[0]) != 1 {
		t.Fatalf("line 0 should hold exactly 1 tile (its capacity), got %d", len(b.Lines[0]))
	}
	if len(b.Floor) != 2 {
		t.Fatalf("the other 2 Solid tiles should have overflowed to the floor, got %d", len(b.Floor))
	}
	// The Ring tile (not the chosen color) must have gone to the center, not
	// vanished or landed on this player's board.
	if countColor(gs.Center, ColorRing) != 1 {
		t.Fatalf("the un-drafted Ring tile should be in the center, center=%v", gs.Center)
	}
}

// GOTCHA (start marker): the marker lands on the floor and is penalized,
// even when the drafted tiles themselves fit cleanly on a line.
func TestStartMarkerLandsOnFloorAndPenalizes(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 9)
	gs.Center = []Color{ColorRing, ColorRing}
	gs.CenterHasStart = true
	gs.Turn = 0
	if !gs.Play(Move{Source: -1, Color: ColorRing, TargetLine: 2}) { // line 2 has cap 3, room for 2
		t.Fatal("taking 2 Ring tiles from the center onto line 2 should be legal")
	}
	b := &gs.Boards[0]
	if len(b.Lines[2]) != 2 {
		t.Fatalf("both Ring tiles should have fit on line 2, got %d", len(b.Lines[2]))
	}
	if !b.HasMarker {
		t.Fatal("the start marker should now be on player 0's floor line")
	}
	if len(b.Floor) != 0 {
		t.Fatalf("no actual tiles should be on the floor (only the marker), got %v", b.Floor)
	}
	if gs.CenterHasStart {
		t.Fatal("the marker must be claimed (removed from the center)")
	}
	if gs.StartNext != 0 {
		t.Fatalf("player 0 claimed the marker, so they should start next round, got %d", gs.StartNext)
	}
	// floorCount must count the marker even with zero actual tiles.
	if b.floorCount() != 1 {
		t.Fatalf("floorCount = %d, want 1 (marker only)", b.floorCount())
	}
}

// GOTCHA: the round ends only once every factory AND the center are empty —
// verified by driving a full round via real Play() calls until doTiling
// fires.
func TestRoundEndsWhenFactoriesAndCenterEmpty(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 11)
	for i := 0; i < 500 && gs.Phase == PhasePlaying; i++ {
		moves := gs.LegalMoves()
		if len(moves) == 0 {
			t.Fatal("no legal moves while still PhasePlaying")
		}
		if !gs.Play(moves[0]) {
			t.Fatal("a move from LegalMoves() was rejected")
		}
	}
	if gs.Phase != PhaseTiling {
		t.Fatalf("Phase = %v, want PhaseTiling once drafting empties out", gs.Phase)
	}
	if len(gs.Center) != 0 {
		t.Fatal("center should be empty at round end")
	}
	for i, f := range gs.Factories {
		if len(f) != 0 {
			t.Fatalf("factory %d should be empty at round end, got %v", i, f)
		}
	}
}

// GOTCHA: the game-end trigger (a completed wall row) ends the game only
// AFTER that round finishes tiling — not the instant the row is filled — and
// end bonuses are applied exactly once.
func TestRoundEndThenGameEndSequencing(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 13)

	completerColor := Color(2)
	completerCol := wallColOf(4, completerColor)
	for c := 0; c < WallSize; c++ {
		if c != completerCol {
			gs.Boards[0].Wall[4][c] = true
		}
	}
	gs.Boards[0].Lines[4] = []Color{completerColor, completerColor, completerColor, completerColor, completerColor}

	for i := range gs.Factories {
		gs.Factories[i] = nil
	}
	gs.Center = nil
	gs.CenterHasStart = false

	gs.doTiling()
	if gs.Phase != PhaseTiling {
		t.Fatalf("Phase = %v, want PhaseTiling immediately after the round's last draft action", gs.Phase)
	}
	if !gameOver(&gs.Boards[0].Wall) {
		t.Fatal("setup bug: wall row 4 should be complete after tiling")
	}
	if gs.Bonuses[0].Total != 0 {
		t.Fatal("end bonuses must NOT be applied before Continue() runs — tiling and game-end are separate steps")
	}

	scoreAfterTiling := gs.Boards[0].Score
	if !gs.Continue() {
		t.Fatal("Continue() should advance out of PhaseTiling")
	}
	if gs.Phase != PhaseBonus {
		t.Fatalf("Phase = %v, want PhaseBonus (a row completed this round)", gs.Phase)
	}
	_, _, _, wantBonus := endBonusesDetailed(&gs.Boards[0].Wall)
	if gs.Bonuses[0].Total != wantBonus {
		t.Fatalf("recorded bonus = %d, want %d", gs.Bonuses[0].Total, wantBonus)
	}
	if gs.Boards[0].Score != scoreAfterTiling+wantBonus {
		t.Fatalf("score after bonus = %d, want %d", gs.Boards[0].Score, scoreAfterTiling+wantBonus)
	}

	scoreAfterBonus := gs.Boards[0].Score
	if !gs.Continue() {
		t.Fatal("Continue() should advance out of PhaseBonus")
	}
	if gs.Phase != PhaseDone {
		t.Fatalf("Phase = %v, want PhaseDone", gs.Phase)
	}
	if gs.Boards[0].Score != scoreAfterBonus {
		t.Fatal("end bonuses must be applied exactly once — the second Continue() must not add them again")
	}
}

func TestRoundEndWithoutGameEndStartsNextRound(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 17)
	gs.Boards[0].Lines[0] = []Color{ColorSolid} // will tile to 1 point, no row completed
	for i := range gs.Factories {
		gs.Factories[i] = nil
	}
	gs.Center = nil
	gs.CenterHasStart = false

	gs.doTiling()
	if gs.Phase != PhaseTiling {
		t.Fatal("expected PhaseTiling")
	}
	if !gs.Continue() {
		t.Fatal("Continue() should advance out of PhaseTiling")
	}
	if gs.Phase != PhasePlaying {
		t.Fatalf("Phase = %v, want PhasePlaying (no row completed -> next round starts)", gs.Phase)
	}
	if gs.RoundNum != 1 {
		t.Fatalf("RoundNum = %d, want 1", gs.RoundNum)
	}
	for i, f := range gs.Factories {
		if len(f) != FactoryFill {
			t.Fatalf("factory %d not refilled: %v", i, f)
		}
	}
	if !gs.CenterHasStart {
		t.Fatal("the marker should be back in the center for the new round")
	}
}

// GOTCHA: bag exhaustion mid-round reshuffles the lid into the bag.
func TestBagExhaustionAndReshuffle(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 19)
	gs.Bag = []Color{ColorSolid, ColorRing}
	gs.Lid = []Color{ColorDot, ColorDot, ColorDot, ColorCross, ColorCross, ColorStripe}

	drawn := gs.draw(5)
	if len(drawn) != 5 {
		t.Fatalf("draw(5) returned %d tiles, want 5 (2 from the bag + reshuffled lid)", len(drawn))
	}
	if len(gs.Lid) != 0 {
		t.Fatalf("the lid should have been fully reshuffled into the bag, got %d left", len(gs.Lid))
	}
	if len(gs.Bag) != 3 { // 2+6=8 available, drew 5, 3 should remain
		t.Fatalf("bag should have 3 tiles left after the reshuffle+draw, got %d", len(gs.Bag))
	}
}

func TestDrawStopsWhenSupplyTrulyExhausted(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 23)
	gs.Bag = []Color{ColorSolid}
	gs.Lid = nil
	drawn := gs.draw(4)
	if len(drawn) != 1 {
		t.Fatalf("draw should return only the 1 tile actually available, got %d", len(drawn))
	}
	if len(gs.Bag) != 0 || len(gs.Lid) != 0 {
		t.Fatal("bag and lid should both be empty once the 100-tile supply is exhausted")
	}
}

func TestPlayRejectsIllegalMove(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 29)
	gs.Factories[0] = []Color{ColorSolid, ColorSolid, ColorRing, ColorDot}
	// Factory 0 has no Cross tiles: this move must be rejected.
	before := *gs
	if gs.Play(Move{Source: 0, Color: ColorCross, TargetLine: 0}) {
		t.Fatal("a move drafting a color absent from the source must be rejected")
	}
	if gs.Turn != before.Turn {
		t.Fatal("a rejected move must not change the turn")
	}
}

func TestPlayAdvancesTurn(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 31)
	moves := gs.LegalMoves()
	if len(moves) == 0 {
		t.Fatal("no legal moves at game start")
	}
	if !gs.Play(moves[0]) {
		t.Fatal("a legal move from LegalMoves() should be accepted")
	}
	if gs.Turn != 1 {
		t.Fatalf("turn should now be player 1, got %d", gs.Turn)
	}
}

func TestWinnerTieBreakOnRows(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 37)
	gs.Boards[0].Score = 40
	gs.Boards[1].Score = 40
	for c := 0; c < WallSize; c++ {
		gs.Boards[0].Wall[0][c] = true
	}
	if w := gs.Winner(); w != 0 {
		t.Fatalf("Winner() = %d, want 0 (more complete rows on equal score)", w)
	}
}

func TestWinnerSharedWhenFullyTied(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 41)
	gs.Boards[0].Score = 10
	gs.Boards[1].Score = 10
	if w := gs.Winner(); w != -1 {
		t.Fatalf("Winner() = %d, want -1 (shared)", w)
	}
}

func TestAITurn(t *testing.T) {
	gs := NewGameSeeded(ModeAI, DepthEasy, 43)
	if gs.AITurn() {
		t.Fatal("player 0 (human) moves first, AITurn() should be false")
	}
	gs.Turn = 1
	if !gs.AITurn() {
		t.Fatal("player 1 (AI) to move in PhasePlaying should report AITurn() true")
	}
	gs.Phase = PhaseTiling
	if gs.AITurn() {
		t.Fatal("AITurn() should be false outside PhasePlaying")
	}
}

func TestCloneIsIndependent(t *testing.T) {
	gs := NewGameSeeded(ModeHotseat, 0, 47)
	clone := gs.Clone()
	clone.Boards[0].Score = 999
	clone.Factories[0] = append(clone.Factories[0], ColorSolid)
	if gs.Boards[0].Score == 999 {
		t.Fatal("mutating the clone's board must not affect the original")
	}
	if len(gs.Factories[0]) == len(clone.Factories[0]) {
		t.Fatal("mutating the clone's factory must not affect the original")
	}
}
