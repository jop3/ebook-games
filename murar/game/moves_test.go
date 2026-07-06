package game

import (
	"image"
	"sort"
	"testing"
)

// ptsEqual reports whether two point slices contain the same points,
// ignoring order.
func ptsEqual(a, b []image.Point) bool {
	if len(a) != len(b) {
		return false
	}
	sortPts := func(s []image.Point) []image.Point {
		out := append([]image.Point(nil), s...)
		sort.Slice(out, func(i, j int) bool {
			if out[i].Y != out[j].Y {
				return out[i].Y < out[j].Y
			}
			return out[i].X < out[j].X
		})
		return out
	}
	as, bs := sortPts(a), sortPts(b)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

func contains(pts []image.Point, p image.Point) bool {
	for _, q := range pts {
		if q == p {
			return true
		}
	}
	return false
}

func TestLegalPawnMovesOpenBoardStart(t *testing.T) {
	b := NewBoard() // P1 at (4,8), P2 at (4,0), far apart
	got := LegalPawnMoves(&b, P1)
	want := []image.Point{{X: 4, Y: 7}, {X: 3, Y: 8}, {X: 5, Y: 8}} // up/left/right; down is off-board
	if !ptsEqual(got, want) {
		t.Fatalf("LegalPawnMoves(P1) = %v, want %v", got, want)
	}
}

func TestLegalPawnMovesBlockedByWall(t *testing.T) {
	b := NewBoard()
	// Vertical wall at (4,7) blocks the (4,8)-(5,8) edge, removing the right move.
	b.place(Wall{X: 4, Y: 7, Orient: Vertical})
	got := LegalPawnMoves(&b, P1)
	if contains(got, image.Pt(5, 8)) {
		t.Fatalf("right move should be blocked by the wall, got %v", got)
	}
	if !contains(got, image.Pt(3, 8)) || !contains(got, image.Pt(4, 7)) {
		t.Fatalf("the other two moves should remain legal, got %v", got)
	}
}

// --- Jump / diagonal exception cases (the trickiest rule) -------------------

func TestStraightJumpWhenClear(t *testing.T) {
	var b Board
	b.Pawns[P1] = image.Pt(4, 4)
	b.Pawns[P2] = image.Pt(4, 3) // adjacent above P1
	got := LegalPawnMoves(&b, P1)
	if !contains(got, image.Pt(4, 2)) {
		t.Fatalf("straight jump over the opponent should be legal, got %v", got)
	}
	if contains(got, image.Pt(3, 3)) || contains(got, image.Pt(5, 3)) {
		t.Fatalf("diagonal cells must NOT be offered when the straight jump is clear, got %v", got)
	}
	// The other 3 directions remain ordinary moves (down/left/right, all empty).
	for _, p := range []image.Point{{4, 5}, {3, 4}, {5, 4}} {
		if !contains(got, p) {
			t.Fatalf("ordinary move to %v missing from %v", p, got)
		}
	}
}

func TestDiagonalJumpWhenStraightLandingWallBlocked(t *testing.T) {
	var b Board
	b.Pawns[P1] = image.Pt(4, 4)
	b.Pawns[P2] = image.Pt(4, 3)
	// Block the edge (4,2)-(4,3) so the straight jump landing is unreachable.
	b.place(Wall{X: 4, Y: 2, Orient: Horizontal})
	got := LegalPawnMoves(&b, P1)
	if contains(got, image.Pt(4, 2)) {
		t.Fatalf("straight jump should be unavailable when its landing is wall-blocked, got %v", got)
	}
	if !contains(got, image.Pt(3, 3)) || !contains(got, image.Pt(5, 3)) {
		t.Fatalf("both diagonal jump cells should be legal, got %v", got)
	}
}

func TestDiagonalJumpWhenStraightLandingOffBoard(t *testing.T) {
	var b Board
	b.Pawns[P1] = image.Pt(4, 1)
	b.Pawns[P2] = image.Pt(4, 0) // opponent pawn against the far (top) edge
	got := LegalPawnMoves(&b, P1)
	if contains(got, image.Pt(4, -1)) {
		t.Fatal("an off-board destination must never be offered")
	}
	if !contains(got, image.Pt(3, 0)) || !contains(got, image.Pt(5, 0)) {
		t.Fatalf("both diagonal jump cells should be legal when the beyond-cell is off-board, got %v", got)
	}
}

func TestDiagonalJumpOnlyOneSideOpen(t *testing.T) {
	var b Board
	b.Pawns[P1] = image.Pt(4, 4)
	b.Pawns[P2] = image.Pt(4, 3)
	b.place(Wall{X: 4, Y: 2, Orient: Horizontal}) // straight landing blocked
	// Additionally block the (4,3)-(3,3) diagonal approach. Anchored at
	// Y=2 (not Y=3) so it blocks rows 2/3 only, not row 4 — otherwise it
	// would incidentally also block P1's ordinary left step at (3,4).
	b.place(Wall{X: 3, Y: 2, Orient: Vertical})
	got := LegalPawnMoves(&b, P1)
	if contains(got, image.Pt(3, 3)) {
		t.Fatalf("the wall-blocked diagonal must not be offered, got %v", got)
	}
	if !contains(got, image.Pt(5, 3)) {
		t.Fatalf("the still-open diagonal must be offered, got %v", got)
	}
	if !contains(got, image.Pt(3, 4)) {
		t.Fatalf("P1's ordinary left step must remain legal (only the diagonal jump is blocked), got %v", got)
	}
}

func TestNoExtraMovesWhenOpponentNotAdjacent(t *testing.T) {
	var b Board
	b.Pawns[P1] = image.Pt(4, 4)
	b.Pawns[P2] = image.Pt(0, 0) // far away, not adjacent in any direction
	got := LegalPawnMoves(&b, P1)
	want := []image.Point{{4, 3}, {4, 5}, {3, 4}, {5, 4}}
	if !ptsEqual(got, want) {
		t.Fatalf("LegalPawnMoves = %v, want the 4 plain orthogonal steps %v", got, want)
	}
}

func TestDiagonalJumpBothSidesBlockedYieldsNoJumpMoves(t *testing.T) {
	var b Board
	b.Pawns[P1] = image.Pt(4, 4)
	b.Pawns[P2] = image.Pt(4, 3)
	b.place(Wall{X: 4, Y: 2, Orient: Horizontal}) // straight landing blocked
	// Both diagonal-blocking walls anchored at Y=2 so they only affect rows
	// 2/3 (the diagonal approaches), not row 4 (P1's own ordinary moves).
	b.place(Wall{X: 3, Y: 2, Orient: Vertical}) // left diagonal blocked
	b.place(Wall{X: 4, Y: 2, Orient: Vertical}) // right diagonal blocked
	got := LegalPawnMoves(&b, P1)
	if contains(got, image.Pt(4, 2)) || contains(got, image.Pt(3, 3)) || contains(got, image.Pt(5, 3)) {
		t.Fatalf("no jump-related destination should be legal when all are blocked, got %v", got)
	}
	// The 3 ordinary moves in the other directions remain.
	for _, p := range []image.Point{{4, 5}, {3, 4}, {5, 4}} {
		if !contains(got, p) {
			t.Fatalf("ordinary move to %v missing from %v", p, got)
		}
	}
}

func TestIsLegalPawnMove(t *testing.T) {
	b := NewBoard()
	if !IsLegalPawnMove(&b, P1, image.Pt(4, 7)) {
		t.Fatal("stepping forward should be legal")
	}
	if IsLegalPawnMove(&b, P1, image.Pt(4, 6)) {
		t.Fatal("a 2-cell plain move must not be legal")
	}
	if IsLegalPawnMove(&b, P1, image.Pt(4, 8)) {
		t.Fatal("staying in place must not be legal")
	}
}
