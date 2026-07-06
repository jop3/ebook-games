package game

import (
	"image"
	"testing"
)

func TestBestMoveTakesImmediateWin(t *testing.T) {
	b := NewBoard()
	b.Pawns[P2] = image.Pt(4, Size-2) // one step from P2's goal row
	act, ok := BestMove(b, P2, DepthEasy)
	if !ok {
		t.Fatal("BestMove should find a move")
	}
	if act.IsWall {
		t.Fatal("an immediate winning pawn move must be preferred over any wall")
	}
	if act.To.Y != Size-1 {
		t.Fatalf("BestMove should take the winning step, got %v", act.To)
	}
}

func TestBestMoveNeverCrashesFromStart(t *testing.T) {
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		b := NewBoard()
		act, ok := BestMove(b, P1, depth)
		if !ok {
			t.Fatalf("depth %d: BestMove should return a move from the start position", depth)
		}
		if !act.IsWall && !IsLegalPawnMove(&b, P1, act.To) {
			t.Fatalf("depth %d: BestMove returned an illegal pawn move %v", depth, act.To)
		}
		if act.IsWall && !CanPlaceWall(&b, act.Wall) {
			t.Fatalf("depth %d: BestMove returned an illegal wall %v", depth, act.Wall)
		}
	}
}

func TestCandidateWallsAreAllLegal(t *testing.T) {
	b := NewBoard()
	walls := candidateWalls(&b, P1)
	if len(walls) == 0 {
		t.Fatal("expected some candidate walls near the opponent's clear starting path")
	}
	for _, w := range walls {
		if !CanPlaceWall(&b, w) {
			t.Fatalf("candidate wall %v should already be filtered to only legal placements", w)
		}
	}
}

func TestCandidateWallsEmptyWithoutWallsLeft(t *testing.T) {
	b := NewBoard()
	b.WallsLeft[P1] = 0
	if walls := candidateWalls(&b, P1); len(walls) != 0 {
		t.Fatalf("candidateWalls should be empty once a side has no walls left, got %v", walls)
	}
}

func TestAIVsAIFullGameTerminates(t *testing.T) {
	b := NewBoard()
	turn := P1
	for ply := 0; ; ply++ {
		if ply > 300 {
			t.Fatal("AI vs AI game did not terminate")
		}
		if w, ok := Winner(&b); ok {
			if w != P1 && w != P2 {
				t.Fatalf("unexpected winner value %v", w)
			}
			return
		}
		act, ok := BestMove(b, turn, DepthMedium)
		if !ok {
			t.Fatalf("side %v has no legal action at ply %d (should be unreachable)", turn, ply)
		}
		b = applyAction(&b, turn, act)
		turn = turn.Opponent()
	}
}

func TestOrderActionsPreservesSet(t *testing.T) {
	b := NewBoard()
	acts := candidateActions(&b, P1)
	ordered := orderActions(&b, P1, acts)
	if len(ordered) != len(acts) {
		t.Fatalf("orderActions changed the candidate count: %d vs %d", len(ordered), len(acts))
	}
}
