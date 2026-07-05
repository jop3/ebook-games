package game

import (
	"image"
	"testing"
	"time"
)

// --- AI takes an immediately winning King capture ---------------------------

func TestBestMoveTakesImmediateKingCapture(t *testing.T) {
	b := emptyBoard()
	// White to move; sliding to (1,1) captures Black's King outright.
	b[2][2] = &Piece{Kind: Triangle, Side: White}
	b[1][1] = &Piece{Kind: King, Side: Black}
	b[5][3] = &Piece{Kind: King, Side: White}
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		m, ok := BestMove(b, White, depth)
		if !ok {
			t.Fatalf("depth %d: BestMove found no move", depth)
		}
		if m.From != (image.Point{X: 2, Y: 2}) || m.To != (image.Point{X: 1, Y: 1}) {
			t.Fatalf("depth %d: BestMove = %v, want the immediate King-capturing move (2,2)->(1,1)", depth, m)
		}
		nb, captured := b.Apply(m)
		if captured == nil || captured.Kind != King {
			t.Fatalf("depth %d: move did not actually capture the King", depth)
		}
		if w, ok := Winner(&nb); !ok || w != White {
			t.Fatalf("depth %d: move did not actually win the game", depth)
		}
	}
}

// --- AI takes an immediate win by walking its King to the far edge ---------

func TestBestMoveTakesImmediateGoalRankWin(t *testing.T) {
	b := emptyBoard()
	b[4][1] = &Piece{Kind: King, Side: Black} // (x=1,y=4): one diagonal step from the goal rank y=5
	b[2][3] = &Piece{Kind: King, Side: White} // neutral row, not on anyone's goal rank
	// Give Black an alternative, tempting-looking capture elsewhere so the
	// AI genuinely has to prefer the outright win over a capture.
	b[4][2] = &Piece{Kind: Triangle, Side: Black}
	b[3][2] = &Piece{Kind: Square, Side: White}
	m, ok := BestMove(b, Black, DepthHard)
	if !ok {
		t.Fatal("BestMove found no move")
	}
	if m.From != (image.Point{X: 1, Y: 4}) || m.To != (image.Point{X: 2, Y: 5}) {
		t.Fatalf("BestMove = %v, want the immediate goal-rank win (1,4)->(2,5)", m)
	}
}

// --- AI avoids leaving its own King hanging when a safe alternative exists --

func TestBestMoveAvoidsHangingTheKing(t *testing.T) {
	// Black's King sits where a White X could capture it next move UNLESS
	// Black moves the King to safety this turn; Black also has an unrelated,
	// harmless piece it could move instead. A good Black move must not leave
	// the King capturable next turn when moving it to safety is available.
	b := emptyBoard()
	b[2][1] = &Piece{Kind: King, Side: Black, Ortho: true} // (x=1,y=2): orthogonal mode
	b[2][3] = &Piece{Kind: Ex, Side: White, Moved: true}   // (x=3,y=2): long (distance 2); (3,2)->(1,2) captures the King
	b[0][0] = &Piece{Kind: Triangle, Side: Black}          // (x=0,y=0): an unrelated, irrelevant Black piece
	b[5][3] = &Piece{Kind: King, Side: White}

	m, ok := BestMove(b, Black, DepthHard)
	if !ok {
		t.Fatal("BestMove found no move")
	}
	afterBlack, _ := b.Apply(m)
	if _, stillThere := afterBlack.KingPos(Black); !stillThere {
		t.Fatal("Black's own move should never capture its own King")
	}
	whiteMove, whiteOK := BestMove(afterBlack, White, DepthHard)
	if whiteOK {
		afterWhite, captured := afterBlack.Apply(whiteMove)
		if captured != nil && captured.Kind == King {
			t.Fatalf("Black's move %v left the King capturable by %v", m, whiteMove)
		}
		_ = afterWhite
	}
}

// --- AI performance: DepthHard must stay fast from a realistic mid-game ----

func TestBestMoveHardDepthIsFastEnough(t *testing.T) {
	b := NewBoard()
	// Play out a few opening moves both ways to reach a more typical
	// mid-game position with mixed short/long pieces, rather than timing the
	// (unusually forced) starting position.
	for _, mv := range []Move{
		{From: image.Pt(1, 0), To: image.Pt(0, 1)},
		{From: image.Pt(1, 5), To: image.Pt(0, 4)},
		{From: image.Pt(2, 0), To: image.Pt(2, 1)},
		{From: image.Pt(2, 5), To: image.Pt(2, 4)},
	} {
		nb, _ := b.Apply(mv)
		b = nb
	}
	start := time.Now()
	m, ok := BestMove(b, White, DepthHard)
	elapsed := time.Since(start)
	if !ok {
		t.Fatal("BestMove found no move from the mid-game position")
	}
	t.Logf("BestMove at DepthHard=%d took %v (move %v)", DepthHard, elapsed, m)
	if elapsed > 2*time.Second {
		t.Fatalf("BestMove at DepthHard took %v, want well under 2s", elapsed)
	}
}
