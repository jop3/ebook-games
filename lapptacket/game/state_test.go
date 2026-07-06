package game

import (
	"image"
	"testing"
)

func newTestPatches(n int, timeCost int) []Patch {
	out := make([]Patch, n)
	for i := 0; i < n; i++ {
		out[i] = Patch{ID: 100 + i, Name: "test", Cells: shMono, Cost: 0, TimeCost: timeCost, Income: 0}
	}
	return out
}

func TestBuyPatchAppliesCostPlacementIncomeAndAdvancesToken(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Queue = newTestPatches(5, 1)
	s.TokenIdx = 0
	s.Buttons[0] = 5

	// Buy offset 1 (patch ID 101). Legal orientation 0 of a monomino is
	// just {(0,0)}, so anchor (0,0) is always a valid single-cell spot on
	// an empty board.
	if !s.BuyPatch(0, 1, 0, image.Pt(0, 0)) {
		t.Fatal("expected BuyPatch to succeed")
	}
	if s.Buttons[0] != 5 { // cost 0
		t.Fatalf("Buttons[0] = %d, want 5 (cost was 0)", s.Buttons[0])
	}
	if !s.Boards[0].Filled[0][0] {
		t.Fatal("expected cell (0,0) to be filled after buying")
	}
	if s.LastBuyPatchID != 101 {
		t.Fatalf("LastBuyPatchID = %d, want 101", s.LastBuyPatchID)
	}
	if s.Marker[0] != 1 {
		t.Fatalf("Marker[0] = %d, want 1 (TimeCost)", s.Marker[0])
	}

	// The queue shed the bought patch (101), and the token now points to
	// where it used to be — which, after the shift, holds the NEXT patch
	// (102) — so NextThree skips 100 (now "behind" the token) entirely.
	wantIDs := []int{102, 103, 104}
	for i, p := range s.Queue {
		wantID := []int{100, 102, 103, 104}[i]
		if p.ID != wantID {
			t.Fatalf("Queue[%d].ID = %d, want %d (queue=%v)", i, p.ID, wantID, s.Queue)
		}
	}
	three := s.NextThree()
	if len(three) != 3 {
		t.Fatalf("NextThree() len = %d, want 3", len(three))
	}
	for i, p := range three {
		if p.ID != wantIDs[i] {
			t.Fatalf("NextThree()[%d].ID = %d, want %d", i, p.ID, wantIDs[i])
		}
	}
}

func TestBuyPatchRejectsInsufficientButtons(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Queue = []Patch{{ID: 1, Cells: shMono, Cost: 10, TimeCost: 1, Income: 0}}
	s.Buttons[0] = 3
	if s.BuyPatch(0, 0, 0, image.Pt(0, 0)) {
		t.Fatal("expected BuyPatch to fail: cost 10 > 3 buttons")
	}
	if s.Boards[0].FilledCount() != 0 {
		t.Fatal("board should be untouched after a rejected buy")
	}
}

func TestBuyPatchRejectsIllegalPlacement(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Queue = []Patch{{ID: 1, Cells: shMono, Cost: 0, TimeCost: 1, Income: 0}}
	s.Buttons[0] = 5
	s.Boards[0].Filled[0][0] = true // occupy the only cell we'll try to use

	if s.BuyPatch(0, 0, 0, image.Pt(0, 0)) {
		t.Fatal("expected BuyPatch onto an occupied cell to fail")
	}
	// Off-board anchor.
	if s.BuyPatch(0, 0, 0, image.Pt(50, 50)) {
		t.Fatal("expected BuyPatch off-board to fail")
	}
}

func TestBuyPatchRejectsWrongActor(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Queue = []Patch{{ID: 1, Cells: shMono, Cost: 0, TimeCost: 1, Income: 0}}
	s.Buttons[0] = 5
	s.Buttons[1] = 5
	s.Marker[0] = 5
	s.Marker[1] = 0 // player 1 is behind -> must act next

	if s.BuyPatch(0, 0, 0, image.Pt(0, 0)) {
		t.Fatal("expected BuyPatch by player 0 to be rejected when player 1 is the trailing actor")
	}
	if !s.BuyPatch(1, 0, 0, image.Pt(0, 0)) {
		t.Fatal("expected BuyPatch by the correct trailing actor (player 1) to succeed")
	}
}

func TestAdvanceMovesToJustPastOpponentAndPaysButtons(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Marker[0] = 3
	s.Marker[1] = 10
	before := s.Buttons[0]

	if !s.Advance(0) {
		t.Fatal("expected Advance(0) to succeed (player 0 is trailing)")
	}
	if s.Marker[0] != 11 {
		t.Fatalf("Marker[0] = %d, want 11 (just past opponent's 10)", s.Marker[0])
	}
	if got, want := s.Buttons[0]-before, 8; got != want {
		t.Fatalf("Advance paid %d buttons, want %d (11-3)", got, want)
	}
}

func TestAdvanceCrossesIncomeSquareAndPaysBoardIncome(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Income[0] = 3 // pretend player 0 already owns patches worth 3 income
	s.Marker[0] = 2
	s.Marker[1] = 8 // Advance(0) will move 2 -> 9, crossing IncomeSquares[0]=4... but
	// IncomeSquares are {4,9,15,...}; moving 2->9 lands exactly on AND passes 4.
	before := s.Buttons[0]

	if !s.Advance(0) {
		t.Fatal("expected Advance(0) to succeed")
	}
	// Crosses both 4 and 9 (landing exactly on 9): 2 income squares x 3
	// income = 6 bonus buttons, plus 7 buttons for the 7 squares advanced.
	wantDelta := 7 + 2*3
	if got := s.Buttons[0] - before; got != wantDelta {
		t.Fatalf("Buttons delta = %d, want %d", got, wantDelta)
	}
}

func TestAdvanceRejectsWrongActor(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Marker[0] = 3
	s.Marker[1] = 10
	if s.Advance(1) {
		t.Fatal("expected Advance(1) to fail: player 1 is leading, not trailing")
	}
}

func TestPendingBlocksNormalActionsUntilPlaced(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Pending[0] = 1

	if s.Advance(0) {
		t.Fatal("expected Advance to be rejected while a free patch is pending")
	}
	s.Queue = []Patch{{ID: 1, Cells: shMono, Cost: 0, TimeCost: 1}}
	if s.BuyPatch(0, 0, 0, image.Pt(1, 1)) {
		t.Fatal("expected BuyPatch to be rejected while a free patch is pending")
	}

	if s.PlaceFreePatch(0, image.Pt(1, 1)) == false {
		t.Fatal("expected PlaceFreePatch to succeed")
	}
	if s.Pending[0] != 0 {
		t.Fatalf("Pending[0] = %d, want 0 after placing", s.Pending[0])
	}
	if !s.Boards[0].Filled[1][1] {
		t.Fatal("expected the free patch to occupy (1,1)")
	}
	if s.Income[0] != FreePatch.Income {
		t.Fatalf("Income[0] = %d, want %d", s.Income[0], FreePatch.Income)
	}

	// Now that pending is cleared, normal actions work again.
	if !s.BuyPatch(0, 0, 0, image.Pt(2, 2)) {
		t.Fatal("expected BuyPatch to succeed once pending is resolved")
	}
}

func TestPlaceFreePatchRejectsOccupiedOrEmpty(t *testing.T) {
	s := NewGame(OpponentHotseat)
	if s.PlaceFreePatch(0, image.Pt(0, 0)) {
		t.Fatal("expected PlaceFreePatch to fail with none pending")
	}
	s.Pending[0] = 1
	s.Boards[0].Filled[2][2] = true
	if s.PlaceFreePatch(0, image.Pt(2, 2)) {
		t.Fatal("expected PlaceFreePatch onto an occupied cell to fail")
	}
	if s.Pending[0] != 1 {
		t.Fatal("Pending must not be consumed by a failed placement")
	}
}

func TestSevenBySevenBonusClaimedOnce(t *testing.T) {
	s := NewGame(OpponentHotseat)
	for y := 0; y < 7; y++ {
		for x := 0; x < 7; x++ {
			s.Boards[0].Filled[y][x] = true
			s.Boards[1].Filled[y][x] = true
		}
	}
	s.checkSevenBySeven(0)
	if s.BonusOwner != 0 {
		t.Fatalf("BonusOwner = %d, want 0", s.BonusOwner)
	}
	// Player 1 also completed a 7x7, but the bonus is claimable once total.
	s.checkSevenBySeven(1)
	if s.BonusOwner != 0 {
		t.Fatalf("BonusOwner changed to %d after a second claim attempt, want it to stay 0", s.BonusOwner)
	}
}

func TestFinalScoreFormula(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Buttons[0] = 20
	// Fill 81-9 = 72 cells, leaving 9 empty.
	n := 0
	for y := 0; y < BoardSize && n < 72; y++ {
		for x := 0; x < BoardSize && n < 72; x++ {
			s.Boards[0].Filled[y][x] = true
			n++
		}
	}
	got := s.FinalScore(0)
	want := 20 - 2*9 // 2
	if got != want {
		t.Fatalf("FinalScore = %d, want %d", got, want)
	}
	s.BonusOwner = 0
	if got := s.FinalScore(0); got != want+7 {
		t.Fatalf("FinalScore with bonus = %d, want %d", got, want+7)
	}
}

func TestGameOverRequiresBothMarkersAtEndAndNoPending(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Marker[0] = TrackEnd
	s.Marker[1] = TrackEnd - 1
	if s.GameOver() {
		t.Fatal("expected GameOver=false: player 1 has not reached the end")
	}
	s.Marker[1] = TrackEnd
	s.Pending[0] = 1
	if s.GameOver() {
		t.Fatal("expected GameOver=false while a free patch is still pending")
	}
	s.Pending[0] = 0
	if !s.GameOver() {
		t.Fatal("expected GameOver=true: both markers at TrackEnd, nothing pending")
	}
}

func TestWinnerPicksHigherFinalScoreOrTie(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Buttons[0], s.Buttons[1] = 10, 4
	if w := s.Winner(); w != 0 {
		t.Fatalf("Winner() = %d, want 0", w)
	}
	s.Buttons[1] = 10
	if w := s.Winner(); w != -1 {
		t.Fatalf("Winner() = %d, want -1 (tie)", w)
	}
}

func TestActingPlayerPrioritizesPendingOverTrailingRule(t *testing.T) {
	s := NewGame(OpponentHotseat)
	s.Marker[0] = 0
	s.Marker[1] = 10 // ordinarily player 0 (trailing) would act next
	s.Pending[1] = 1 // but player 1 owes a free-patch placement first
	if got := s.ActingPlayer(); got != 1 {
		t.Fatalf("ActingPlayer() = %d, want 1 (pending placement takes priority)", got)
	}
}
