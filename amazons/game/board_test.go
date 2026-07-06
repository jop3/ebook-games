package game

import (
	"image"
	"testing"
)

func emptyBoard() Board {
	return Board{}
}

func TestNewBoardStartPositions(t *testing.T) {
	b := NewBoard()
	want := map[image.Point]Cell{
		{X: 3, Y: 0}: QueenBlack,
		{X: 6, Y: 0}: QueenBlack,
		{X: 0, Y: 3}: QueenBlack,
		{X: 9, Y: 3}: QueenBlack,
		{X: 3, Y: 9}: QueenWhite,
		{X: 6, Y: 9}: QueenWhite,
		{X: 0, Y: 6}: QueenWhite,
		{X: 9, Y: 6}: QueenWhite,
	}
	for p, c := range want {
		if got := b.At(p.X, p.Y); got != c {
			t.Errorf("At(%d,%d) = %v, want %v", p.X, p.Y, got, c)
		}
	}
	if got := nonEmptyCount(&b); got != 8 {
		t.Fatalf("expected exactly 8 queens on the board, got %d", got)
	}
	if len(b.QueenPositions(Black)) != 4 || len(b.QueenPositions(White)) != 4 {
		t.Fatalf("each side should have exactly 4 queens: black=%d white=%d",
			len(b.QueenPositions(Black)), len(b.QueenPositions(White)))
	}
}

// nonEmptyCount is a small test helper: total non-empty cells (queens only,
// since a fresh board has no burned squares).
func nonEmptyCount(b *Board) int {
	n := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) != Empty {
				n++
			}
		}
	}
	return n
}

func TestDestinationsFromOpenBoardCenter(t *testing.T) {
	b := emptyBoard()
	// From (4,4) on a fully open 10x10 board: distances to each edge in the 8
	// directions are 4 (up/left... to 0) ... let's just count total reachable
	// squares directly instead of hand-deriving each ray length.
	dests := b.DestinationsFrom(image.Pt(4, 4))
	// Right: x=5..9 (5), Left: x=3..0 (4), Down: y=5..9(5), Up: y=3..0(4)
	// Diag down-right: (5,5)(6,6)(7,7)(8,8)(9,9) = 5
	// Diag up-left: (3,3)(2,2)(1,1)(0,0) = 4
	// Diag down-left: (3,5)(2,6)(1,7)(0,8) = 4
	// Diag up-right: (5,3)(6,2)(7,1)(8,0) = 4
	want := 5 + 4 + 5 + 4 + 5 + 4 + 4 + 4
	if len(dests) != want {
		t.Fatalf("DestinationsFrom((4,4)) on open board = %d, want %d", len(dests), want)
	}
}

func TestDestinationsFromBlockedByQueen(t *testing.T) {
	b := emptyBoard()
	b.set(5, 4, QueenWhite) // blocks the rightward ray from (4,4) at x=5
	dests := b.DestinationsFrom(image.Pt(4, 4))
	for _, d := range dests {
		if d.Y == 4 && d.X >= 5 {
			t.Fatalf("ray should stop before the occupying queen at (5,4), got destination %v", d)
		}
	}
	// (5,4) itself must not be offered (it's occupied) and nothing past it
	// on that ray either.
	for _, d := range dests {
		if d == (image.Point{X: 5, Y: 4}) {
			t.Fatal("occupied square must not be a legal destination")
		}
	}
}

func TestDestinationsFromBlockedByBurned(t *testing.T) {
	b := emptyBoard()
	b.set(5, 4, Burned)
	dests := b.DestinationsFrom(image.Pt(4, 4))
	for _, d := range dests {
		if d.Y == 4 && d.X >= 5 {
			t.Fatalf("ray should stop before the burned square at (5,4), got destination %v", d)
		}
	}
}

func TestIsLegalQueenMove(t *testing.T) {
	b := NewBoard()
	// A Black queen at (3,0) can move down the column to (3,8) (blocked at
	// (3,9) by... nothing, (3,9) holds a White queen so it stops at (3,8)).
	if !b.IsLegalQueenMove(Black, QueenMove{From: image.Pt(3, 0), To: image.Pt(3, 5)}) {
		t.Fatal("expected a clear vertical move to be legal")
	}
	if b.IsLegalQueenMove(Black, QueenMove{From: image.Pt(3, 0), To: image.Pt(3, 9)}) {
		t.Fatal("move onto an occupied square (White's queen) must be illegal")
	}
	if b.IsLegalQueenMove(Black, QueenMove{From: image.Pt(3, 0), To: image.Pt(4, 2)}) {
		t.Fatal("a non-straight, non-diagonal move must be illegal")
	}
	if b.IsLegalQueenMove(White, QueenMove{From: image.Pt(3, 0), To: image.Pt(3, 5)}) {
		t.Fatal("moving a square that isn't side's own queen must be illegal")
	}
	if b.IsLegalQueenMove(Black, QueenMove{From: image.Pt(3, 0), To: image.Pt(3, 0)}) {
		t.Fatal("a zero-length move must be illegal")
	}
}

func TestIsLegalQueenMoveDiagonal(t *testing.T) {
	b := emptyBoard()
	b.set(2, 2, QueenBlack)
	if !b.IsLegalQueenMove(Black, QueenMove{From: image.Pt(2, 2), To: image.Pt(7, 7)}) {
		t.Fatal("a clear diagonal move must be legal (queen moves like a bishop too)")
	}
	b.set(5, 5, QueenWhite) // blocks the diagonal partway
	if b.IsLegalQueenMove(Black, QueenMove{From: image.Pt(2, 2), To: image.Pt(7, 7)}) {
		t.Fatal("a diagonal move must be blocked by an intervening queen")
	}
	if !b.IsLegalQueenMove(Black, QueenMove{From: image.Pt(2, 2), To: image.Pt(4, 4)}) {
		t.Fatal("the diagonal up to (but not through) the blocker should remain legal")
	}
}

// --- GOTCHA: burned squares block BOTH movement and further arrows --------
//
// This is the classic bug the spec calls out explicitly: a burned square
// must behave exactly like a permanently occupied cell, both for queen
// movement and for a later arrow shot along the same line — never treated
// as a merely decorative marker.

func TestBurnedBlocksQueenMovement(t *testing.T) {
	b := emptyBoard()
	b.set(2, 4, QueenBlack)
	b.set(5, 4, Burned)
	if b.IsLegalQueenMove(Black, QueenMove{From: image.Pt(2, 4), To: image.Pt(8, 4)}) {
		t.Fatal("a queen move must not pass through a burned square")
	}
	if b.IsLegalQueenMove(Black, QueenMove{From: image.Pt(2, 4), To: image.Pt(5, 4)}) {
		t.Fatal("a queen must not be able to land ON a burned square")
	}
	if !b.IsLegalQueenMove(Black, QueenMove{From: image.Pt(2, 4), To: image.Pt(4, 4)}) {
		t.Fatal("the queen should still be able to move up to (but not onto/through) the burned square")
	}
}

func TestBurnedBlocksArrowShots(t *testing.T) {
	b := emptyBoard()
	b.set(2, 4, QueenBlack) // the queen that "just landed" here for this test
	b.set(5, 4, Burned)
	if b.IsLegalShot(image.Pt(2, 4), image.Pt(8, 4)) {
		t.Fatal("an arrow must not fly through a burned square")
	}
	if b.IsLegalShot(image.Pt(2, 4), image.Pt(5, 4)) {
		t.Fatal("an arrow must not be able to land ON an already-burned square")
	}
	if !b.IsLegalShot(image.Pt(2, 4), image.Pt(4, 4)) {
		t.Fatal("the arrow should still be able to land up to (but not onto/through) the burned square")
	}
}

// --- GOTCHA: an arrow may legally shoot back through the queen's own ------
// vacated starting square, since that square becomes Empty the instant the
// queen leaves it.

func TestShotCanPassThroughVacatedOrigin(t *testing.T) {
	b := emptyBoard()
	b.set(2, 4, QueenBlack)
	// Nothing else on the board: move the queen from (2,4) to (6,4).
	from, to := image.Pt(2, 4), image.Pt(6, 4)
	if !b.IsLegalQueenMove(Black, QueenMove{From: from, To: to}) {
		t.Fatal("setup: the move should be legal")
	}
	nb := b.MoveQueen(QueenMove{From: from, To: to})
	if nb.At(from.X, from.Y) != Empty {
		t.Fatal("the vacated origin square must be Empty after the queen moves away")
	}
	// The queen at (6,4) shoots an arrow back along the same line, past its
	// own former square, all the way to (0,4) — legal, since (2,4) and every
	// square between are now Empty.
	if !nb.IsLegalShot(to, image.Pt(0, 4)) {
		t.Fatal("an arrow shot back through the queen's own vacated origin square should be legal")
	}
	if !nb.IsLegalShot(to, from) {
		t.Fatal("an arrow shot landing exactly on the queen's own vacated origin square should be legal")
	}
}

func TestIsLegalShotRequiresAQueenAtFrom(t *testing.T) {
	b := emptyBoard()
	// Nothing at (2,2): a shot cannot be attempted from an empty or burned
	// square, only from wherever a queen (of either side) actually stands.
	if b.IsLegalShot(image.Pt(2, 2), image.Pt(2, 5)) {
		t.Fatal("a shot must require a queen presently standing at 'from'")
	}
	b.set(2, 2, Burned)
	if b.IsLegalShot(image.Pt(2, 2), image.Pt(2, 5)) {
		t.Fatal("a shot must not be allowed to originate from a burned square")
	}
}

func TestLegalQueenMovesAndTurnsOnStartingBoard(t *testing.T) {
	b := NewBoard()
	moves := b.LegalQueenMoves(Black)
	if len(moves) == 0 {
		t.Fatal("Black should have legal moves from the starting position")
	}
	for _, m := range moves {
		if !b.IsLegalQueenMove(Black, m) {
			t.Fatalf("LegalQueenMoves returned an illegal move: %v", m)
		}
	}
	turns := b.LegalTurns(Black)
	if len(turns) == 0 {
		t.Fatal("Black should have at least one legal full turn from the starting position")
	}
	for _, tn := range turns {
		if !b.IsLegalQueenMove(Black, tn.Move) {
			t.Fatalf("LegalTurns returned a turn with an illegal move: %v", tn)
		}
		afterMove := b.MoveQueen(tn.Move)
		if !afterMove.IsLegalShot(tn.Move.To, tn.Shot) {
			t.Fatalf("LegalTurns returned a turn with an illegal shot: %v", tn)
		}
	}
}

func TestApplyMovesQueenAndBurnsShotSquare(t *testing.T) {
	b := NewBoard()
	turn := Turn{Move: QueenMove{From: image.Pt(3, 0), To: image.Pt(3, 4)}, Shot: image.Pt(3, 0)}
	nb := b.Apply(turn)
	if nb.At(3, 4) != QueenBlack {
		t.Fatal("the mover's queen should now be at its destination")
	}
	if nb.At(3, 0) != Burned {
		t.Fatal("the shot square should now be permanently Burned")
	}
	if b.At(3, 0) != QueenBlack {
		t.Fatal("Apply must not mutate the original board (value semantics)")
	}
}
