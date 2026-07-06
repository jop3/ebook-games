package game

import (
	"image"
	"testing"
	"time"
)

func TestBestMoveNoLegalMove(t *testing.T) {
	var b Board // empty board, no pawns of either color
	if _, ok := BestMove(b, Black, DepthEasy); ok {
		t.Fatal("BestMove should report ok=false when side has no pawns at all")
	}
}

// BestMove must take an immediately winning move (reaching the goal rank)
// over any other option, even a tempting capture elsewhere.
func TestBestMoveTakesImmediateWinByReachingGoal(t *testing.T) {
	var b Board
	b.set(3, 1, Black) // one step from row 0: Black wins by playing (3,1)->(3,0)
	b.set(0, 4, Black) // a second Black pawn with other options
	b.set(1, 3, White) // capturable by the second pawn, but not decisive...
	b.set(7, 4, White) // ...a second White pawn so that capture doesn't ALSO win outright
	m, ok := BestMove(b, Black, DepthMedium)
	if !ok {
		t.Fatal("BestMove should find a move")
	}
	if m.From != image.Pt(3, 1) || m.To != image.Pt(3, 0) {
		t.Fatalf("BestMove = %v, want the immediate winning advance to (3,0)", m)
	}
}

// BestMove must take an immediately winning capture (the opponent's last
// pawn) over a merely-good positional move.
func TestBestMoveTakesImmediateWinByCapturingLastPawn(t *testing.T) {
	var b Board
	b.set(3, 3, Black)
	b.set(2, 2, White) // White's only pawn, capturable diagonally
	m, ok := BestMove(b, Black, DepthMedium)
	if !ok {
		t.Fatal("BestMove should find a move")
	}
	if m.To != image.Pt(2, 2) || !m.Capture {
		t.Fatalf("BestMove = %v, want the capture of White's last pawn", m)
	}
}

// A simple hanging-capture scenario: the AI should prefer taking a free
// pawn over an unrelated quiet move, at every difficulty depth.
func TestBestMovePrefersFreeCapture(t *testing.T) {
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		var b Board
		b.set(3, 3, Black)
		b.set(2, 2, White) // free capture (nothing defends it)
		b.set(0, 0, White) // an unrelated White pawn far away, irrelevant to Black's choice
		m, ok := BestMove(b, Black, depth)
		if !ok {
			t.Fatalf("depth %d: BestMove should find a move", depth)
		}
		if !m.Capture {
			t.Errorf("depth %d: BestMove = %v, want the free capture", depth, m)
		}
	}
}

// The AI should decline a quiet advance that walks straight into a square
// where an enemy pawn can diagonally capture it for free next turn, when a
// safe alternative move exists elsewhere.
func TestBestMoveAvoidsHangingAPawnForFree(t *testing.T) {
	var b Board
	b.set(3, 2, Black) // its only move, (3,2)->(3,1), walks into...
	b.set(2, 0, White) // ...this pawn's diagonal capture at (3,1)
	b.set(6, 3, Black) // a second Black pawn with a plain, uncontested advance
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		m, ok := BestMove(b, Black, depth)
		if !ok {
			t.Fatalf("depth %d: BestMove should find a move", depth)
		}
		if m.From == image.Pt(3, 2) {
			t.Errorf("depth %d: BestMove = %v, should have avoided hanging the (3,2) pawn for free", depth, m)
		}
	}
}

// --- isUnopposed / evaluate sanity: an unopposed advanced pawn should score
// higher than an equally advanced but opposed one.

func TestIsUnopposed(t *testing.T) {
	var b Board
	b.set(3, 2, Black) // nothing ahead in columns 2,3,4 toward row 0
	if !isUnopposed(&b, Black, 3, 2) {
		t.Fatal("a pawn with a clear cone to its goal should be unopposed")
	}
	b.set(3, 0, White) // directly ahead, in the cone
	if isUnopposed(&b, Black, 3, 2) {
		t.Fatal("an enemy pawn in the cone should make the pawn opposed")
	}
}

func TestIsUnopposedAdjacentFileBlocks(t *testing.T) {
	var b Board
	b.set(3, 2, Black)
	b.set(2, 1, White) // adjacent file, ahead — still inside the 3-wide cone
	if isUnopposed(&b, Black, 3, 2) {
		t.Fatal("an enemy pawn in an adjacent file within the cone should make the pawn opposed")
	}
}

func TestEvaluateRewardsUnopposedAdvancedPawnMoreThanOpposed(t *testing.T) {
	// Same material both sides (1 Black pawn, 1 White pawn) in each board;
	// the only difference is whether the White pawn sits inside Black's
	// advancement cone (opposed) or well off to the side (unopposed).
	var free Board
	free.set(3, 1, Black) // one step from goal, cone = columns 2-4 at row 0
	free.set(7, 1, White) // far column: outside the cone, doesn't oppose it
	scoreFree := evaluate(&free, Black)

	var opposed Board
	opposed.set(3, 1, Black)
	opposed.set(3, 0, White) // squarely inside the cone: now opposed
	scoreOpposed := evaluate(&opposed, Black)

	if scoreFree <= scoreOpposed {
		t.Fatalf("unopposed advanced pawn scored %d, opposed scored %d; want unopposed clearly higher",
			scoreFree, scoreOpposed)
	}
}

// --- Direction sanity: BestMove must never move a pawn backward for either
// side (mirrors the board-level direction tests, one level up).

func TestBestMoveNeverMovesBackward(t *testing.T) {
	b := NewBoard()
	m, ok := BestMove(b, Black, DepthEasy)
	if !ok {
		t.Fatal("BestMove should find an opening move for Black")
	}
	if m.To.Y >= m.From.Y {
		t.Fatalf("Black move %v moved backward or sideways in y", m)
	}
	m2, ok := BestMove(b, White, DepthEasy)
	if !ok {
		t.Fatal("BestMove should find an opening move for White")
	}
	if m2.To.Y <= m2.From.Y {
		t.Fatalf("White move %v moved backward or sideways in y", m2)
	}
}

// --- Performance sanity: depth 7-8 on this small board must stay fast
// enough for a comfortable e-reader UI (well under a second per move).

func TestBestMoveHardDepthIsFastEnough(t *testing.T) {
	b := NewBoard()
	start := time.Now()
	if _, ok := BestMove(b, White, DepthHard); !ok {
		t.Fatal("BestMove should find an opening move")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("DepthHard search took %v from the opening position, too slow", elapsed)
	}
}

// --- A full AI vs. AI game must terminate cleanly with a real winner.

func TestFullAIvsAIGameTerminates(t *testing.T) {
	s := NewGame(OpponentAI, DepthMedium)
	for ply := 0; s.Phase == PhasePlaying; ply++ {
		if ply > 300 {
			t.Fatal("game did not terminate")
		}
		var mv Move
		var ok bool
		if s.Turn == Black {
			mv, ok = BestMove(s.Board, Black, DepthEasy)
		} else {
			mv, ok = BestMove(s.Board, White, DepthEasy)
		}
		if !ok {
			t.Fatalf("side %v to move but BestMove found nothing at ply %d", s.Turn, ply)
		}
		if !s.Play(mv.From, mv.To) {
			t.Fatalf("BestMove %v was rejected by Play at ply %d", mv, ply)
		}
	}
	if w := s.Winner(); w != Black && w != White {
		t.Fatalf("game ended with no clear winner: %v", w)
	}
}
