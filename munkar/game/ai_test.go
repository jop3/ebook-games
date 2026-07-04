package game

import (
	"image"
	"math/rand"
	"testing"
	"time"
)

func TestBestMoveTakesImmediateWin(t *testing.T) {
	var b Board
	for x := 0; x < 4; x++ {
		b.Ring[2][x] = White
	}
	candidates := []image.Point{{X: 4, Y: 2}, {X: 5, Y: 5}, {X: 0, Y: 0}}
	mv, ok := BestMove(b, White, candidates, DepthMedium)
	if !ok {
		t.Fatal("BestMove should find a move")
	}
	if mv != (image.Point{X: 4, Y: 2}) {
		t.Fatalf("BestMove = %v, want the immediate winning move (4,2)", mv)
	}
}

func TestBestMoveOnlyConsidersGivenCandidates(t *testing.T) {
	// Even if the board has other empty cells, BestMove must only ever
	// return one of the candidates it was given (the caller has already
	// applied the forced-direction constraint).
	var b Board
	candidates := []image.Point{{X: 2, Y: 2}}
	mv, ok := BestMove(b, Black, candidates, DepthEasy)
	if !ok || mv != (image.Point{X: 2, Y: 2}) {
		t.Fatalf("BestMove(single candidate) = %v,%v, want (2,2),true", mv, ok)
	}
}

func TestBestMoveNoCandidatesFails(t *testing.T) {
	var b Board
	if _, ok := BestMove(b, Black, nil, DepthEasy); ok {
		t.Fatal("BestMove with no candidates should report ok=false")
	}
}

func TestBestMovePrefersACaptureOverANonCapture(t *testing.T) {
	// Black to move: playing (2,0) captures both White bookends immediately
	// (material swing of 2), a clearly better move than an isolated
	// placement far away with no other effect. At shallow depth this should
	// be the AI's obvious pick.
	var b Board
	b.Ring[0][1] = White
	b.Ring[0][3] = White
	candidates := []image.Point{{X: 2, Y: 0}, {X: 5, Y: 5}}
	mv, ok := BestMove(b, Black, candidates, DepthEasy)
	if !ok || mv != (image.Point{X: 2, Y: 0}) {
		t.Fatalf("BestMove = %v,%v, want the capturing move (2,0)", mv, ok)
	}
}

// TestAIDoesNotPanicFromAFreshGame plays a handful of AI-vs-itself plies
// from a real freshly-shuffled board (exercising the direction-forcing
// fallback and general move generation across many random layouts) at every
// difficulty, just to be sure nothing panics or returns an illegal move.
func TestAIDoesNotPanicFromAFreshGame(t *testing.T) {
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		for seed := int64(1); seed <= 3; seed++ {
			rng := rand.New(rand.NewSource(seed))
			b := NewBoard(rng)
			turn := Black
			hasLast, last := false, image.Point{}
			for ply := 0; ply < 10; ply++ {
				candidates := LegalMoves(b, last, hasLast)
				if len(candidates) == 0 {
					break
				}
				mv, ok := BestMove(b, turn, candidates, depth)
				if !ok {
					t.Fatalf("depth=%d seed=%d ply=%d: BestMove failed with %d candidates", depth, seed, ply, len(candidates))
				}
				nb, _ := Place(b, mv, turn)
				b = nb
				last, hasLast = mv, true
				turn = turn.Opponent()
			}
		}
	}
}

// TestAIHardDepthTiming measures wall-clock for a single BestMove call at
// DepthHard from a realistic mid-game position (board partially filled,
// forced-direction narrowing branching like a real game). This is the
// number reported in the task write-up; keep the position "realistic" (not
// an empty board, not a near-empty forced set) so the measurement reflects
// real play.
func TestAIHardDepthTiming(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	b := NewBoard(rng)
	turn := Black
	hasLast, last := false, image.Point{}
	// Play out ~16 plies with a cheap deterministic policy to reach a
	// representative mid-game position (~16 of 36 cells filled).
	for ply := 0; ply < 16; ply++ {
		candidates := LegalMoves(b, last, hasLast)
		if len(candidates) == 0 {
			break
		}
		mv, _ := BestMove(b, turn, candidates, DepthEasy)
		nb, _ := Place(b, mv, turn)
		b = nb
		last, hasLast = mv, true
		turn = turn.Opponent()
	}
	candidates := LegalMoves(b, last, hasLast)
	if len(candidates) == 0 {
		t.Skip("ran out of legal moves building the mid-game position")
	}

	start := time.Now()
	mv, ok := BestMove(b, turn, candidates, DepthHard)
	elapsed := time.Since(start)
	if !ok {
		t.Fatal("BestMove at DepthHard should find a move")
	}
	t.Logf("BestMove at DepthHard (search depth %d) from a %d-empty-cell mid-game position took %v, chose %v",
		DepthHard, len(emptyCells(b)), elapsed, mv)
	if elapsed > 2*time.Second {
		t.Fatalf("DepthHard search took %v, want well under 2s", elapsed)
	}
}
