package game

import (
	"image"
	"testing"
)

// --- GOTCHA: clone leaves the source occupied; jump vacates it -------------

func TestApplyCloneLeavesSourceOccupied(t *testing.T) {
	var b Board
	b.set(3, 3, Black)
	m := Move{From: image.Pt(3, 3), To: image.Pt(4, 3)} // distance 1: clone
	if !m.IsClone() || m.IsJump() {
		t.Fatalf("Move %v should be classified as a clone, not a jump", m)
	}
	nb, _ := b.Apply(m)
	if nb.At(3, 3) != Black {
		t.Fatal("a clone must leave the source cell occupied by the mover")
	}
	if nb.At(4, 3) != Black {
		t.Fatal("a clone must place a new man at the destination")
	}
	if nb.Count(Black) != 2 {
		t.Fatalf("Black count after a clone = %d, want 2 (net gain of one man)", nb.Count(Black))
	}
}

func TestApplyJumpVacatesSource(t *testing.T) {
	var b Board
	b.set(3, 3, Black)
	m := Move{From: image.Pt(3, 3), To: image.Pt(5, 3)} // distance 2: jump
	if !m.IsJump() || m.IsClone() {
		t.Fatalf("Move %v should be classified as a jump, not a clone", m)
	}
	nb, _ := b.Apply(m)
	if nb.At(3, 3) != Empty {
		t.Fatal("a jump must vacate the source cell")
	}
	if nb.At(5, 3) != Black {
		t.Fatal("a jump must place the man at the destination")
	}
	if nb.Count(Black) != 1 {
		t.Fatalf("Black count after a jump = %d, want 1 (the man only relocated)", nb.Count(Black))
	}
}

// TestApplyConflationBug guards directly against the classic bug of treating
// clone and jump as the same "plain move": applying both a clone and a jump
// from the same starting board must produce different resulting piece
// counts (a clone is a net +1 for the mover; a jump is unchanged).
func TestApplyConflationBug(t *testing.T) {
	var b Board
	b.set(3, 3, Black)
	clone, _ := b.Apply(Move{From: image.Pt(3, 3), To: image.Pt(4, 3)})
	jump, _ := b.Apply(Move{From: image.Pt(3, 3), To: image.Pt(5, 3)})
	if clone.Count(Black) == jump.Count(Black) {
		t.Fatalf("clone and jump must not produce the same Black count (clone=%d, jump=%d): they are different moves",
			clone.Count(Black), jump.Count(Black))
	}
	if clone.At(3, 3) == Empty {
		t.Fatal("clone conflated with jump: source wrongly vacated")
	}
	if jump.At(3, 3) != Empty {
		t.Fatal("jump conflated with clone: source wrongly left occupied")
	}
}

// --- GOTCHA: flip must scan all 8 neighbors of the destination --------------

func TestApplyFlipsAllEightNeighbors(t *testing.T) {
	var b Board
	// Surround (3,3) with White men on all 8 sides; Black clones into (3,3)
	// from further away is not possible in one move (too far), so instead
	// Black already sits adjacent and jumps in from range 2 to land exactly
	// on (3,3), which itself must be empty beforehand.
	b.set(1, 3, Black) // 2 away horizontally: legal jump source
	for _, d := range neighbor8 {
		b.set(3+d.X, 3+d.Y, White)
	}
	m := Move{From: image.Pt(1, 3), To: image.Pt(3, 3)}
	if !m.IsJump() {
		t.Fatalf("setup error: %v should be a jump", m)
	}
	nb, flipped := b.Apply(m)
	if len(flipped) != 8 {
		t.Fatalf("flipped %d cells, want all 8 neighbors: %v", len(flipped), flipped)
	}
	for _, d := range neighbor8 {
		x, y := 3+d.X, 3+d.Y
		if nb.At(x, y) != Black {
			t.Fatalf("neighbor (%d,%d) should have flipped to Black, still %v", x, y, nb.At(x, y))
		}
	}
	if nb.Count(White) != 0 {
		t.Fatalf("White count after the flip = %d, want 0", nb.Count(White))
	}
	if nb.Count(Black) != 9 { // the mover + its 8 flipped neighbors
		t.Fatalf("Black count after the flip = %d, want 9", nb.Count(Black))
	}
}

// TestApplyFlipDiagonalOnly guards specifically against a 4-neighbor
// (orthogonal-only) flip scan: place enemy men ONLY on the 4 diagonal
// neighbors of the destination and confirm they still flip.
func TestApplyFlipDiagonalOnly(t *testing.T) {
	var b Board
	b.set(3, 1, Black) // 2 away vertically: legal jump source to (3,3)
	b.set(2, 2, White) // NW diagonal neighbor of (3,3)
	b.set(4, 2, White) // NE diagonal neighbor of (3,3)
	b.set(2, 4, White) // SW diagonal neighbor of (3,3)
	b.set(4, 4, White) // SE diagonal neighbor of (3,3)

	m := Move{From: image.Pt(3, 1), To: image.Pt(3, 3)}
	nb, flipped := b.Apply(m)
	if len(flipped) != 4 {
		t.Fatalf("flipped %d cells, want the 4 diagonal-only enemies: %v", len(flipped), flipped)
	}
	diag := [4]image.Point{{X: 2, Y: 2}, {X: 4, Y: 2}, {X: 2, Y: 4}, {X: 4, Y: 4}}
	for _, p := range diag {
		if nb.At(p.X, p.Y) != Black {
			t.Fatalf("diagonal neighbor %v should have flipped to Black (a 4-neighbor-only scan would miss this)", p)
		}
	}
}

func TestApplyDoesNotFlipNonAdjacentEnemies(t *testing.T) {
	var b Board
	b.set(3, 3, Black)
	b.set(6, 6, White) // far away, not adjacent to any destination used below
	nb, flipped := b.Apply(Move{From: image.Pt(3, 3), To: image.Pt(4, 4)})
	if len(flipped) != 0 {
		t.Fatalf("flipped %v, want none: the White man is not adjacent to the destination", flipped)
	}
	if nb.At(6, 6) != White {
		t.Fatal("a non-adjacent enemy man must never flip")
	}
}

func TestApplyFlipOnlyEnemyNeighborsNotOwnColor(t *testing.T) {
	var b Board
	b.set(1, 3, Black)
	b.set(3, 2, Black) // own-color neighbor of destination: must not "flip" (already the mover's color)
	b.set(4, 3, White) // enemy neighbor of destination: must flip
	nb, flipped := b.Apply(Move{From: image.Pt(1, 3), To: image.Pt(3, 3)})
	if len(flipped) != 1 || flipped[0] != (image.Point{X: 4, Y: 3}) {
		t.Fatalf("flipped = %v, want exactly [(4,3)]", flipped)
	}
	if nb.At(3, 2) != Black {
		t.Fatal("an own-color neighbor must remain unaffected")
	}
}
