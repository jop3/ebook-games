package game

import (
	"image"
	"testing"
)

// --- AI takes an immediately winning capture --------------------------------

func TestBestMoveTakesImmediateWinningCapture(t *testing.T) {
	b := emptyBoard()
	// White to move; sliding to (4,4) captures Black's last-but-one man at
	// (3,4), reducing Black to a single man and winning outright.
	b.set(4, 0, White)
	b.set(3, 4, Black) // Black's only other man besides the lone survivor
	b.set(2, 4, White)
	b.set(0, 0, Black) // Black's lone surviving man, far away
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		m, ok := BestMove(b, White, depth)
		if !ok {
			t.Fatalf("depth %d: BestMove found no move", depth)
		}
		if m.From != (image.Point{X: 4, Y: 0}) || m.To != (image.Point{X: 4, Y: 4}) {
			t.Fatalf("depth %d: BestMove = %v, want the immediate winning capture (4,0)->(4,4)", depth, m)
		}
		nb, captured := b.Apply(m)
		if len(captured) != 1 || nb.Count(Black) != 1 {
			t.Fatalf("depth %d: move did not actually win (captured=%v, black=%d)", depth, captured, nb.Count(Black))
		}
	}
}

// --- AI avoids hanging a decisive capture -----------------------------------

func TestBestMoveAvoidsHangingADecisiveCapture(t *testing.T) {
	// Black has only 2 men left: man A at (4,4), already bracketed on its
	// left by a White man at (3,4); White also has a second man at (5,0) that
	// can slide straight down to (5,4) this turn, completing the bracket and
	// capturing A — which would reduce Black to a single man and lose the
	// game outright. Black's other man, B at (0,8), is safe and irrelevant.
	// A good Black move must not leave A hanging like that (either move A to
	// safety, or otherwise prevent the capture) when a safe alternative move
	// of B is available but decisive.
	b := emptyBoard()
	b.set(3, 4, White)
	b.set(4, 4, Black) // man A
	b.set(5, 0, White)
	b.set(0, 8, Black) // man B
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		m, ok := BestMove(b, Black, depth)
		if !ok {
			t.Fatalf("depth %d: no move found", depth)
		}
		// Black must NOT move man A off of a square that's already safe into
		// one that hangs it, and must not leave it sitting one White move away
		// from a decisive (down-to-1) capture if a safe alternative exists.
		afterBlack, _ := b.Apply(m)
		whiteMove, whiteOK := BestMove(afterBlack, White, DepthHard)
		if whiteOK {
			afterWhite, _ := afterBlack.Apply(whiteMove)
			if afterWhite.Count(Black) <= 1 {
				t.Fatalf("depth %d: BestMove %v hangs a decisive capture (White replies %v and wins)", depth, m, whiteMove)
			}
		}
	}
}

// --- AI runs at all three difficulties without error, always moving --------

func TestBestMoveAllDifficultiesFromStartingPosition(t *testing.T) {
	b := NewBoard()
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		m, ok := BestMove(b, White, depth)
		if !ok {
			t.Fatalf("depth %d: expected a move from the starting position", depth)
		}
		if !b.IsLegalMove(White, m) {
			t.Fatalf("depth %d: BestMove returned an illegal move %v", depth, m)
		}
	}
}

func TestBestMoveNoMovesReturnsFalse(t *testing.T) {
	b := emptyBoard()
	b.set(4, 4, Black) // completely boxed in by White on all 4 sides
	b.set(3, 4, White)
	b.set(5, 4, White)
	b.set(4, 3, White)
	b.set(4, 5, White)
	if _, ok := BestMove(b, Black, DepthEasy); ok {
		t.Fatal("BestMove should report no move when the side is completely boxed in")
	}
}
