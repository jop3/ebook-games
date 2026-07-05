package game

import (
	"testing"
	"time"
)

func TestBestMoveReturnsLegalMove(t *testing.T) {
	for _, diff := range []int{DepthEasy, DepthMedium, DepthHard} {
		gs := NewGameSeeded(ModeAI, diff, 100+int64(diff))
		gs.Turn = 1
		legal := LegalMoves(gs, 1)
		mv, ok := BestMove(gs, 1, diff)
		if !ok {
			t.Fatalf("diff %d: BestMove found no move despite %d legal candidates", diff, len(legal))
		}
		found := false
		for _, m := range legal {
			if m == mv {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("diff %d: BestMove returned %v, not among the %d legal moves", diff, mv, len(legal))
		}
	}
}

func TestBestMoveNoLegalMoves(t *testing.T) {
	gs := NewGameSeeded(ModeAI, DepthEasy, 101)
	gs.Turn = 1
	for i := range gs.Factories {
		gs.Factories[i] = nil
	}
	gs.Center = nil
	if _, ok := BestMove(gs, 1, DepthEasy); ok {
		t.Fatal("BestMove should report no move when there are no legal candidates")
	}
}

// GOTCHA: BestMove must actually prefer completing a line that scores now
// over one that scores nothing, at every difficulty — a basic sanity check
// that the evaluator isn't inverted.
func TestBestMovePrefersScoringMove(t *testing.T) {
	gs := NewGameSeeded(ModeAI, DepthEasy, 103)
	gs.Turn = 1
	for i := range gs.Factories {
		gs.Factories[i] = nil
	}
	gs.Center = nil
	// Factory 0: 4 Solid tiles. Board 1's pattern line 0 (cap 1) is empty and
	// its wall cell is open, so taking Solid onto line 0 completes it
	// (scoring 1 point at tiling) — Move A. Move B (same tiles, to the
	// floor) only incurs penalty. A rational AI must prefer A.
	gs.Factories[0] = []Color{ColorSolid, ColorSolid, ColorSolid, ColorSolid}
	for _, diff := range []int{DepthEasy, DepthMedium, DepthHard} {
		mv, ok := BestMove(gs, 1, diff)
		if !ok {
			t.Fatalf("diff %d: no move found", diff)
		}
		if mv.TargetLine == -1 {
			t.Errorf("diff %d: AI chose the floor (%v) over a scoring line placement", diff, mv)
		}
	}
}

// Measures wall-clock time for BestMove at DepthHard from a realistic
// mid-round position (all 5 factories full, several distinct colors, both
// boards partially built) — the deepest/most expensive difficulty. Must
// stay comfortably under 2s; see ai.go's doc comment for how the search
// width is bounded.
func TestBestMoveHardPerformance(t *testing.T) {
	gs := NewGameSeeded(ModeAI, DepthHard, 107)
	gs.Turn = 1
	gs.Factories[0] = []Color{ColorSolid, ColorRing, ColorCross, ColorStripe}
	gs.Factories[1] = []Color{ColorDot, ColorDot, ColorSolid, ColorRing}
	gs.Factories[2] = []Color{ColorCross, ColorCross, ColorStripe, ColorDot}
	gs.Factories[3] = []Color{ColorSolid, ColorSolid, ColorRing, ColorRing}
	gs.Factories[4] = []Color{ColorDot, ColorCross, ColorStripe, ColorStripe}
	gs.Center = []Color{ColorSolid, ColorDot, ColorCross}
	gs.CenterHasStart = true

	gs.Boards[0].Lines[1] = []Color{ColorRing}
	gs.Boards[0].Lines[3] = []Color{ColorDot, ColorDot}
	gs.Boards[0].Wall[0][2] = true
	gs.Boards[1].Lines[2] = []Color{ColorCross, ColorCross}
	gs.Boards[1].Wall[1][0] = true

	start := time.Now()
	mv, ok := BestMove(gs, 1, DepthHard)
	elapsed := time.Since(start)
	t.Logf("BestMove(DepthHard) from a realistic mid-round position took %s (move=%v)", elapsed, mv)
	if !ok {
		t.Fatal("expected a legal move")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("BestMove(DepthHard) took %s, want comfortably under 2s", elapsed)
	}
}

// Difficulty must actually differ in strength: the denial term (Medel and
// Svår only) must be able to flip a preference away from the AI's own
// highest-baseValue move once it sees that move leaves the opponent an even
// bigger score on the table. This directly exercises mediumValue's "does
// this leave an obvious big take" logic rather than depending on every
// other candidate move's exact ranking.
func TestDenialTermPrefersDenyingTheOpponent(t *testing.T) {
	gs := NewGameSeeded(ModeAI, DepthHard, 109)
	gs.Turn = 1
	for i := range gs.Factories {
		gs.Factories[i] = nil
	}
	gs.Center = nil

	// AI (board 1): taking 2 Solid tiles onto line 1 (cap 2) completes it
	// against 2 pre-placed wall neighbors, scoring hLen=2+vLen=2=4 —
	// clearly better *for the AI itself* than taking the 1 Cross tile onto
	// line 0 (cap 1, isolated, scores 1).
	gs.Factories[0] = []Color{ColorSolid, ColorSolid, ColorRing, ColorDot}
	gs.Boards[1].Wall[1][3] = true // left neighbor of Solid's row-1 cell (col 4)
	gs.Boards[1].Wall[0][4] = true // up neighbor of Solid's row-1 cell

	// Factory 1 holds the only Cross tile anywhere. The opponent (board 0)
	// already has 3 of the 4 Cross tiles needed on their line 3 (cap 4), and
	// their wall around that cell is built up so completing it scores
	// hLen=3+vLen=2=5 — a much bigger reply than anything else available.
	// If the AI leaves this Factory untouched, the opponent grabs that 1
	// Cross tile next turn and cashes in; if the AI takes it instead, the
	// opponent has nothing comparable left.
	gs.Factories[1] = []Color{ColorCross, ColorStripe, ColorStripe, ColorDot}
	gs.Boards[0].Lines[3] = []Color{ColorCross, ColorCross, ColorCross}
	gs.Boards[0].Wall[3][3] = true
	gs.Boards[0].Wall[3][2] = true
	gs.Boards[0].Wall[4][4] = true

	solidMove := Move{Source: 0, Color: ColorSolid, TargetLine: 1}
	crossMove := Move{Source: 1, Color: ColorCross, TargetLine: 0}
	legal := LegalMoves(gs, 1)
	if !containsMove(legal, solidMove) || !containsMove(legal, crossMove) {
		t.Fatalf("setup bug: both %v and %v must be legal, legal=%v", solidMove, crossMove, legal)
	}

	bvSolid, bvCross := baseValue(gs, solidMove), baseValue(gs, crossMove)
	t.Logf("baseValue: solid=%d cross=%d", bvSolid, bvCross)
	if bvSolid <= bvCross {
		t.Fatalf("setup bug: solidMove should have the higher immediate baseValue (solid=%d, cross=%d)", bvSolid, bvCross)
	}

	mvSolid, mvCross := mediumValue(gs, solidMove), mediumValue(gs, crossMove)
	t.Logf("mediumValue: solid=%d cross=%d", mvSolid, mvCross)
	if mvCross <= mvSolid {
		t.Fatalf("mediumValue should flip the preference once the opponent's big Cross-line reply is priced in: solid=%d, cross=%d", mvSolid, mvCross)
	}
}

func containsMove(moves []Move, m Move) bool {
	for _, cand := range moves {
		if cand == m {
			return true
		}
	}
	return false
}
