package game

import (
	"image"
	"testing"
	"time"
)

func TestBestMoveReturnsLegalMove(t *testing.T) {
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		b := NewBoard()
		fm, ok := BestMove(b, White, depth)
		if !ok {
			t.Fatalf("depth %d: BestMove should find a move on the starting board", depth)
		}
		if !IsLegalLPlacement(b, White, fm.L) {
			t.Fatalf("depth %d: BestMove's L placement %v is not legal", depth, fm.L)
		}
		afterL := ApplyLPlacement(b, White, fm.L)
		if fm.HasNeutral && !IsLegalNeutralMove(afterL, fm.Neutral) {
			t.Fatalf("depth %d: BestMove's neutral move %v is not legal", depth, fm.Neutral)
		}
	}
}

// TestAIPerformanceBudget is not a correctness test - it's a guardrail that
// records how long each difficulty's search takes from the (maximally
// branchy) starting position, so a future change that blows up the search
// doesn't silently make the on-device AI unusably slow. It fails only on
// clearly-unreasonable slowness (>5s on this dev machine); actual on-device
// ARM hardware will be slower, so the depth constants were chosen with
// margin — see the AI performance note in the final report.
func TestAIPerformanceBudget(t *testing.T) {
	b := NewBoard()
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		start := time.Now()
		fm, ok := BestMove(b, White, depth)
		elapsed := time.Since(start)
		if !ok {
			t.Fatalf("depth %d: no move found", depth)
		}
		t.Logf("depth %d (full turns): BestMove took %v, chose L-placement %v (hasNeutral=%v)",
			depth, elapsed, fm.L, fm.HasNeutral)
		if elapsed > 5*time.Second {
			t.Errorf("depth %d took %v, which is too slow for a tap-driven UI (>5s)", depth, elapsed)
		}
	}
}

func TestEvaluateSymmetric(t *testing.T) {
	b := NewBoard()
	// evaluate(b, Black) and evaluate(b, White) should be exact negatives of
	// each other by construction (mobility difference is antisymmetric).
	if evaluate(b, Black) != -evaluate(b, White) {
		t.Errorf("evaluate(Black)=%d, evaluate(White)=%d; should be negatives of each other",
			evaluate(b, Black), evaluate(b, White))
	}
}

func TestGenerateFullMovesCountsSkipAndEachNeutralOption(t *testing.T) {
	b := NewBoard()
	lMoves := LegalLPlacements(b, Black)
	full := GenerateFullMoves(b, Black)
	// Every L placement contributes 1 (skip) + len(LegalNeutralMoves(afterL))
	// full moves; check the total matches that sum exactly.
	want := 0
	for _, pl := range lMoves {
		afterL := ApplyLPlacement(b, Black, pl)
		want += 1 + len(LegalNeutralMoves(afterL))
	}
	if len(full) != want {
		t.Fatalf("GenerateFullMoves returned %d moves, want %d", len(full), want)
	}
}

func TestNegamaxNoLegalMoveIsTerminalLoss(t *testing.T) {
	// A position where toMove (White) has zero legal L-placements must be
	// scored as a loss for White regardless of depth.
	var b Board
	placeShape(&b, Black, 1, image.Pt(0, 0))
	placeShape(&b, White, 0, image.Pt(1, 2))
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) == Empty {
				b.set(x, y, Black)
			}
		}
	}
	if len(LegalLPlacements(b, White)) != 0 {
		t.Fatal("test setup bug: White should have zero legal placements")
	}
	score := negamax(b, White, 2, negInf, posInf, fullNeutralPlies)
	if score >= -winScore {
		t.Errorf("negamax score for a lost position = %d, want <= -winScore(%d)", score, -winScore)
	}
}

// TestBestMoveNeverLeavesSelfWithNoMovesIfAvoidable plays several AI-vs-AI
// full games at DepthEasy (fast) purely as a smoke/robustness check: every
// move BestMove returns must always be legal, and the game must terminate
// (someone eventually runs out of legal L-placements) within a generous ply
// budget - it should never spin forever or panic on a real, reachable board.
func TestBestMoveNeverLeavesSelfWithNoMovesIfAvoidable(t *testing.T) {
	s := NewGame(OpponentHotseat, 0)
	for ply := 0; ply < 200; ply++ {
		fm, ok := BestMove(s.Board, s.Turn, DepthEasy)
		if !ok {
			// s.Turn has no legal L placement: game over, that side lost.
			return
		}
		if !s.PlaceL(fm.L) {
			t.Fatalf("ply %d: BestMove's own L placement %v was rejected by PlaceL", ply, fm.L)
		}
		if fm.HasNeutral {
			if !s.MoveNeutral(fm.Neutral) {
				t.Fatalf("ply %d: BestMove's own neutral move %v was rejected", ply, fm.Neutral)
			}
		} else {
			s.SkipNeutral()
		}
		if s.Phase == PhaseDone {
			return
		}
	}
	t.Fatal("game did not terminate within 200 plies")
}
