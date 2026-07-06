package game

import (
	"image"
	"testing"
)

func emptyBoard() Board {
	var b Board
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			b.Present[y][x] = true
		}
	}
	b.BlackPawn = image.Pt(-1, -1)
	b.WhitePawn = image.Pt(-1, -1)
	return b
}

func TestNewBoardStartingPosition(t *testing.T) {
	b := NewBoard()
	if b.TotalPresent() != Size*Size {
		t.Fatalf("TotalPresent = %d, want %d (every tile present)", b.TotalPresent(), Size*Size)
	}
	if b.PawnPos(Black) == b.PawnPos(White) {
		t.Fatal("Black and White must not start on the same square")
	}
	if b.PawnPos(Black).Y != Size-1 {
		t.Fatalf("Black should start on the bottom edge, got %v", b.PawnPos(Black))
	}
	if b.PawnPos(White).Y != 0 {
		t.Fatalf("White should start on the top edge, got %v", b.PawnPos(White))
	}
}

func TestQueenMoveAllEightDirections(t *testing.T) {
	b := emptyBoard()
	b.BlackPawn = image.Pt(4, 4)
	dests := b.DestinationsFrom(image.Pt(4, 4))
	// From an otherwise empty board, a queen at (4,4) (0-indexed, Size=8)
	// reaches: 4 left + 3 right (row) + 4 up + 3 down (col) + all 4 diagonal
	// rays to the board edge.
	want := map[image.Point]bool{}
	for x := 0; x < Size; x++ {
		if x != 4 {
			want[image.Pt(x, 4)] = true
		}
	}
	for y := 0; y < Size; y++ {
		if y != 4 {
			want[image.Pt(4, y)] = true
		}
	}
	for _, d := range [4]image.Point{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
		x, y := 4+d.X, 4+d.Y
		for inBounds(x, y) {
			want[image.Pt(x, y)] = true
			x += d.X
			y += d.Y
		}
	}
	if len(dests) != len(want) {
		t.Fatalf("got %d destinations, want %d: %v", len(dests), len(want), dests)
	}
	for _, d := range dests {
		if !want[d] {
			t.Fatalf("unexpected destination %v", d)
		}
	}
}

// --- GOTCHA: line-of-sight stops dead at the first missing tile; no jumping.

func TestQueenCannotJumpOverMissingTile(t *testing.T) {
	b := emptyBoard()
	b.BlackPawn = image.Pt(0, 4)
	b.Present[4][3] = false // gap at (3,4), directly in the path
	dests := b.DestinationsFrom(image.Pt(0, 4))
	for _, d := range dests {
		if d.Y == 4 && d.X >= 3 {
			t.Fatalf("must not reach past the gap at (3,4): got destination %v", d)
		}
	}
	found2 := false
	for _, d := range dests {
		if d == image.Pt(2, 4) {
			found2 = true
		}
	}
	if !found2 {
		t.Fatal("(2,4), the cell just before the gap, should be reachable")
	}
}

// --- GOTCHA: line-of-sight stops dead at the opponent's pawn; no jumping.

func TestQueenCannotJumpOverOpponentPawn(t *testing.T) {
	b := emptyBoard()
	b.BlackPawn = image.Pt(0, 4)
	b.WhitePawn = image.Pt(3, 4)
	dests := b.DestinationsFrom(image.Pt(0, 4))
	for _, d := range dests {
		if d.Y == 4 && d.X >= 3 {
			t.Fatalf("must not reach onto or past the opponent's pawn at (3,4): got %v", d)
		}
	}
}

func TestCannotLandOnMissingTile(t *testing.T) {
	b := emptyBoard()
	b.BlackPawn = image.Pt(0, 0)
	b.Present[0][3] = false
	if b.IsLegalPawnMove(Black, image.Pt(3, 0)) {
		t.Fatal("landing on a missing tile must be illegal")
	}
}

func TestCannotLandOnOpponentPawn(t *testing.T) {
	b := emptyBoard()
	b.BlackPawn = image.Pt(0, 0)
	b.WhitePawn = image.Pt(3, 0)
	if b.IsLegalPawnMove(Black, image.Pt(3, 0)) {
		t.Fatal("landing on the opponent's pawn must be illegal")
	}
}

func TestDiagonalAndOrthogonalBothLegal(t *testing.T) {
	b := emptyBoard()
	b.BlackPawn = image.Pt(0, 0)
	if !b.IsLegalPawnMove(Black, image.Pt(5, 5)) {
		t.Fatal("a diagonal queen move should be legal")
	}
	if !b.IsLegalPawnMove(Black, image.Pt(0, 6)) {
		t.Fatal("a vertical queen move should be legal")
	}
	if !b.IsLegalPawnMove(Black, image.Pt(6, 0)) {
		t.Fatal("a horizontal queen move should be legal")
	}
}

func TestKnightLikeMoveIsIllegal(t *testing.T) {
	b := emptyBoard()
	b.BlackPawn = image.Pt(0, 0)
	if b.IsLegalPawnMove(Black, image.Pt(1, 2)) {
		t.Fatal("a non-queen-line move must be illegal")
	}
}

// --- GOTCHA: removing the just-vacated cell IS allowed; the new cell is NOT.

func TestRemovalExcludesOnlyTheNewPosition(t *testing.T) {
	b := NewBoard()
	from := b.PawnPos(Black)
	to := image.Pt(from.X, from.Y-1) // one step "up" toward the board center
	if !b.IsLegalPawnMove(Black, to) {
		t.Fatalf("setup: expected %v -> %v to be legal", from, to)
	}
	nb := b
	nb.setPawnPos(Black, to)

	if nb.IsLegalRemoval(to, to) {
		t.Fatal("removing the cell the mover just landed on must be illegal")
	}
	if !nb.IsLegalRemoval(to, from) {
		t.Fatal("removing the cell the mover just vacated must be legal")
	}
	removals := nb.LegalTileRemovals(to)
	for _, r := range removals {
		if r == to {
			t.Fatal("LegalTileRemovals must never include the mover's new position")
		}
	}
	foundOld := false
	for _, r := range removals {
		if r == from {
			foundOld = true
		}
	}
	if !foundOld {
		t.Fatal("LegalTileRemovals must include the mover's just-vacated old position")
	}
	// Every present tile except the new position should be offered.
	if len(removals) != nb.TotalPresent()-1 {
		t.Fatalf("len(removals) = %d, want %d (every present tile but the new one)", len(removals), nb.TotalPresent()-1)
	}
}

func TestCannotRemoveAMissingTileTwice(t *testing.T) {
	b := NewBoard()
	b.Present[3][3] = false
	if b.IsLegalRemoval(image.Pt(0, 0), image.Pt(3, 3)) {
		t.Fatal("removing an already-missing tile must be illegal")
	}
}

func TestApplyMovesPawnAndRemovesTile(t *testing.T) {
	b := NewBoard()
	from := b.PawnPos(Black)
	to := image.Pt(from.X, from.Y-1)
	m := Move{Side: Black, From: from, To: to, Remove: from}
	nb := b.Apply(m)
	if nb.PawnPos(Black) != to {
		t.Fatalf("pawn should be at %v, got %v", to, nb.PawnPos(Black))
	}
	if nb.IsPresent(from.X, from.Y) {
		t.Fatal("the removed (vacated) tile should now be missing")
	}
	if nb.TotalPresent() != b.TotalPresent()-1 {
		t.Fatal("exactly one tile should have been removed")
	}
	// The original board must be unmodified (value semantics).
	if !b.IsPresent(from.X, from.Y) {
		t.Fatal("Apply must not mutate the receiver")
	}
}

func TestLegalMovesFromStartingPosition(t *testing.T) {
	b := NewBoard()
	moves := b.LegalMoves(Black)
	if len(moves) == 0 {
		t.Fatal("Black should have legal moves at the start")
	}
	for _, m := range moves {
		if !b.IsLegalPawnMove(Black, m) {
			t.Fatalf("LegalMoves produced a destination IsLegalPawnMove rejects: %v", m)
		}
	}
}
