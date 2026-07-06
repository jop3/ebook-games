package game

import (
	"image"
	"testing"
)

func TestEvaluateMobilityDifference(t *testing.T) {
	b := NewBoard()
	// From the fresh starting position both pawns are symmetric, so the
	// mobility difference should be exactly 0 for either side.
	if e := evaluate(&b, Black); e != 0 {
		t.Fatalf("evaluate(Black) on the symmetric start = %d, want 0", e)
	}
	if e := evaluate(&b, White); e != 0 {
		t.Fatalf("evaluate(White) on the symmetric start = %d, want 0", e)
	}

	// Trap White down to a single destination while leaving Black wide open:
	// evaluate from Black's perspective should now be strongly positive.
	b2, _ := winnerTestBoard() // Black stuck, White free — use from White's view
	if e := evaluate(&b2, White); e <= 0 {
		t.Fatalf("evaluate(White) with Black stuck = %d, want > 0 (White should look better off)", e)
	}
}

func TestFullMovesEnumeratesMoveTimesRemoval(t *testing.T) {
	b := NewBoard()
	moves := fullMoves(&b, Black)
	dests := b.LegalMoves(Black)
	if len(dests) == 0 {
		t.Fatal("setup: Black should have legal moves")
	}
	// Every generated move must be individually legal: a legal pawn
	// destination, paired with a legal removal given that destination.
	seen := map[image.Point]int{}
	for _, m := range moves {
		if !b.IsLegalPawnMove(Black, m.To) {
			t.Fatalf("fullMoves produced an illegal destination %v", m.To)
		}
		if !b.IsLegalRemoval(m.To, m.Remove) {
			t.Fatalf("fullMoves produced an illegal removal %v for destination %v", m.Remove, m.To)
		}
		seen[m.To]++
	}
	for _, d := range dests {
		// Each destination should offer (present tiles - 1) removal choices.
		want := b.TotalPresent() - 1
		if seen[d] != want {
			t.Fatalf("destination %v has %d removal choices, want %d", d, seen[d], want)
		}
	}
}

func TestBestMoveReturnsLegalFullMove(t *testing.T) {
	b := NewBoard()
	m, ok := BestMove(b, Black, DepthEasy)
	if !ok {
		t.Fatal("BestMove should find a move from the starting position")
	}
	if !b.IsLegalPawnMove(Black, m.To) {
		t.Fatalf("BestMove destination %v is illegal", m.To)
	}
	nb := b
	nb.setPawnPos(Black, m.To)
	if !nb.IsLegalRemoval(m.To, m.Remove) {
		t.Fatalf("BestMove removal %v is illegal for destination %v", m.Remove, m.To)
	}
}

func TestBestMoveNoLegalMoveWhenTrapped(t *testing.T) {
	b, _ := winnerTestBoard()
	_, ok := BestMove(b, Black, DepthEasy)
	if ok {
		t.Fatal("BestMove should report no legal move for a side with zero legal pawn moves")
	}
}

// TestBestMovePrefersImmediateWin sets up a position where White has a move
// that instantly traps Black (Black has exactly one destination left, and
// that destination is also removable). White's move should be found by the
// depth-1 search (a fully-informed 1-ply search must find any 1-move win).
func TestBestMovePrefersImmediateWin(t *testing.T) {
	var b Board
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			b.Present[y][x] = true
		}
	}
	b.BlackPawn = image.Pt(0, 0)
	b.WhitePawn = image.Pt(7, 0)
	// Wall off Black's other two directions from the corner entirely, and
	// truncate the third (straight down column 0) to a single step, so
	// Black has exactly one destination left: (0,1).
	b.Present[0][1] = false // (1,0) missing: blocks the "right" ray entirely
	b.Present[1][1] = false // (1,1) missing: blocks the diagonal ray entirely
	b.Present[2][0] = false // (0,2) missing: truncates the "down" ray to just (0,1)
	// Black's only destination left is (0,1).
	if dests := b.LegalMoves(Black); len(dests) != 1 || dests[0] != image.Pt(0, 1) {
		t.Fatalf("setup: Black should have exactly one destination (0,1), got %v", dests)
	}

	mv, ok := BestMove(b, White, DepthEasy)
	if !ok {
		t.Fatal("White should have a legal move")
	}
	nb := b.Apply(mv)
	if !GameOver(&nb, Black) {
		t.Fatalf("White's chosen move %+v should immediately trap Black (mobility-maximizing AI should take the win)", mv)
	}
}
