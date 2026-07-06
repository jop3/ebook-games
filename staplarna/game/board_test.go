package game

import "testing"

// TestExactDistanceNotUpTo is THE core movement gotcha per the spec: a stack
// moves EXACTLY N cells (N = its height), never "up to N" like a queen/rook.
// A height-2 stack must be able to reach distance-2 destinations along a
// clear line, and must NOT be able to stop early at distance 1 (nor overshoot
// to distance 3).
func TestExactDistanceNotUpTo(t *testing.T) {
	b := NewBoard()
	from := Point{0, 0, 0}
	b.PlaceNew(Black, Tzarra, from)
	b.Stacks[from] = Stack{Owner: Black, Type: Tzarra, Height: 2, Comp: [3]int{Tzarra: 2}}

	d := Directions[0]
	oneAway := from.Add(d)
	twoAway := from.Add(d).Add(d)
	threeAway := from.Add(d).Add(d).Add(d)

	moves := LegalMoves(b, Black)
	found := map[Point]bool{}
	for _, m := range moves {
		if m.From == from {
			found[m.To] = true
		}
	}
	if found[oneAway] {
		t.Errorf("a height-2 stack must NOT be able to land at distance 1 (%v)", oneAway)
	}
	if !found[twoAway] {
		t.Errorf("a height-2 stack must be able to land at EXACTLY distance 2 (%v); got %v", twoAway, found)
	}
	if found[threeAway] {
		t.Errorf("a height-2 stack must NOT be able to overshoot to distance 3 (%v)", threeAway)
	}

	// A lone piece (height 1) can only ever move exactly 1 cell.
	b2 := NewBoard()
	lone := Point{0, 0, 0}
	b2.PlaceNew(White, Tott, lone)
	dests := DestinationsFrom(b2, lone)
	for _, dest := range dests {
		if Distance(lone, dest) != 1 {
			t.Errorf("a lone piece's destination %v is at distance %d, want exactly 1", dest, Distance(lone, dest))
		}
	}
}

// TestCannotPassThroughOccupiedCell: a move may never jump over any
// occupied cell (friend or foe) partway along its exact-distance ray.
func TestCannotPassThroughOccupiedCell(t *testing.T) {
	b := NewBoard()
	from := Point{-2, 1, 1}
	d := Directions[0] // {1,-1,0}
	mid := from.Add(d)
	dest := mid.Add(d).Add(d) // distance 3 from `from`

	b.Stacks[from] = Stack{Owner: Black, Type: Tott, Height: 3, Comp: [3]int{Tott: 3}}
	b.Stacks[mid] = Stack{Owner: White, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}

	if IsLegalMove(b, Black, Move{From: from, To: dest}) {
		t.Fatalf("a move whose ray passes through an occupied cell at %v must be illegal", mid)
	}

	// The same stack must also be unable to capture the blocking piece
	// itself, since it sits at distance 1, not the stack's required exact
	// distance 3.
	if IsLegalMove(b, Black, Move{From: from, To: mid}) {
		t.Fatal("a height-3 stack must not be able to land at distance 1 even to capture")
	}
}

// TestMergeWithOwnStack: landing on your own stack merges heights and
// combines Comp (physical piece composition), and always succeeds regardless
// of the destination stack's height (no "too tall to merge" concept).
func TestMergeWithOwnStack(t *testing.T) {
	b := NewBoard()
	from := Point{0, 0, 0}
	to := from.Add(Directions[0])
	b.Stacks[from] = Stack{Owner: Black, Type: Tzaar, Height: 1, Comp: [3]int{Tzaar: 1}}
	b.Stacks[to] = Stack{Owner: Black, Type: Tott, Height: 5, Comp: [3]int{Tott: 5}}

	if !IsLegalMove(b, Black, Move{From: from, To: to}) {
		t.Fatal("moving onto one's own (much taller) stack should be a legal merge")
	}
	captured := b.Apply(Move{From: from, To: to})
	if captured {
		t.Fatal("a merge with one's own stack must never report a capture")
	}
	merged, ok := b.At(to)
	if !ok {
		t.Fatal("merged stack missing at destination")
	}
	if merged.Height != 6 {
		t.Fatalf("merged height = %d, want 6", merged.Height)
	}
	if merged.Type != Tzaar {
		t.Fatalf("merged stack's visible type = %v, want Tzaar (the mover's, since it lands on top)", merged.Type)
	}
	if merged.Comp[Tzaar] != 1 || merged.Comp[Tott] != 5 {
		t.Fatalf("merged Comp = %v, want Tzaar:1 Tott:5 (both original pieces still physically present)", merged.Comp)
	}
	if _, stillThere := b.At(from); stillThere {
		t.Fatal("the origin cell must be empty after the move")
	}
}

// TestCaptureRemovesWholeStackIncludingBuriedPieces: capturing an enemy
// stack removes ALL of its physical pieces (Comp), even ones buried under a
// different visible top type — this is the mechanic that makes "burying your
// last Tzaar under other pieces" a real (risky) tactic rather than a free
// hiding spot.
func TestCaptureRemovesWholeStackIncludingBuriedPieces(t *testing.T) {
	b := NewBoard()
	from := Point{0, 0, 0}
	// A height-2 stack moves EXACTLY 2 steps, so the target must be 2 cells
	// away along the ray, not 1 (a stack can never land short of its height).
	to := from.Add(Directions[0]).Add(Directions[0])
	// White's stack shows Tott on top, but has a buried Tzaar underneath —
	// White's very last Tzaar.
	b.Stacks[to] = Stack{Owner: White, Type: Tott, Height: 2, Comp: [3]int{Tzaar: 1, Tott: 1}}
	b.Stacks[from] = Stack{Owner: Black, Type: Tzarra, Height: 2, Comp: [3]int{Tzarra: 2}}

	if !IsLegalMove(b, Black, Move{From: from, To: to}) {
		t.Fatal("landing on an enemy stack of equal height should be a legal capture")
	}
	captured := b.Apply(Move{From: from, To: to})
	if !captured {
		t.Fatal("Apply should report a capture")
	}
	if b.TypeCount(White, Tzaar) != 0 {
		t.Fatalf("White's buried Tzaar should have been eliminated along with the rest of the captured stack, TypeCount=%d", b.TypeCount(White, Tzaar))
	}
	winnerSide, ok := b.At(to)
	if !ok || winnerSide.Owner != Black || winnerSide.Height != 2 {
		t.Fatalf("the mover should now occupy %v with its own height-2 stack, got %+v ok=%v", to, winnerSide, ok)
	}
}

// TestCannotCaptureTallerStack: landing on an enemy stack STRICTLY taller
// than the mover is illegal (not merely "not a capture" — it must not appear
// among the legal moves at all).
func TestCannotCaptureTallerStack(t *testing.T) {
	b := NewBoard()
	from := Point{0, 0, 0}
	to := from.Add(Directions[0])
	b.Stacks[from] = Stack{Owner: Black, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}
	b.Stacks[to] = Stack{Owner: White, Type: Tott, Height: 2, Comp: [3]int{Tott: 2}}

	if IsLegalMove(b, Black, Move{From: from, To: to}) {
		t.Fatal("landing on a taller enemy stack must be illegal")
	}
	for _, m := range LegalMoves(b, Black) {
		if m.From == from && m.To == to {
			t.Fatal("LegalMoves must not include a move onto a taller enemy stack")
		}
	}

	// Equal height IS capturable ("<=", not "<").
	b.Stacks[to] = Stack{Owner: White, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}
	if !IsLegalMove(b, Black, Move{From: from, To: to}) {
		t.Fatal("landing on an EQUAL-height enemy stack should be a legal capture")
	}
}

// TestCapturesAreOptional: the documented design decision — a non-capturing
// move remains legal even when a capturing move is available elsewhere.
func TestCapturesAreOptional(t *testing.T) {
	b := NewBoard()
	from := Point{0, 0, 0}
	capTarget := from.Add(Directions[0])
	emptyTarget := from.Add(Directions[2])
	b.Stacks[from] = Stack{Owner: Black, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}
	b.Stacks[capTarget] = Stack{Owner: White, Type: Tott, Height: 1, Comp: [3]int{Tott: 1}}
	// emptyTarget is a plain empty landing at the same distance-1.

	moves := LegalMoves(b, Black)
	sawCapture, sawPlain := false, false
	for _, m := range moves {
		if m.From != from {
			continue
		}
		if m.To == capTarget {
			sawCapture = true
		}
		if m.To == emptyTarget {
			sawPlain = true
		}
	}
	if !sawCapture {
		t.Fatal("the capturing move should be legal")
	}
	if !sawPlain {
		t.Fatal("a plain non-capturing move must remain legal even though a capture is available elsewhere (captures are not mandatory)")
	}
}

// TestPlaceNewRejectsOccupiedOrInvalid.
func TestPlaceNewRejectsOccupiedOrInvalid(t *testing.T) {
	b := NewBoard()
	p := Point{0, 0, 0}
	if !b.PlaceNew(Black, Tzaar, p) {
		t.Fatal("first placement on an empty valid cell should succeed")
	}
	if b.PlaceNew(White, Tott, p) {
		t.Fatal("placing on an already-occupied cell must fail")
	}
	if b.PlaceNew(Black, Tzaar, Point{100, -100, 0}) {
		t.Fatal("placing off-board must fail")
	}
	s, ok := b.At(p)
	if !ok || s.Owner != Black || s.Type != Tzaar || s.Height != 1 || s.Comp[Tzaar] != 1 {
		t.Fatalf("placed stack wrong: %+v", s)
	}
}

// TestBoardCloneIsIndependent: the AI search relies on Clone() producing a
// fully independent copy.
func TestBoardCloneIsIndependent(t *testing.T) {
	b := NewBoard()
	p := Point{0, 0, 0}
	b.PlaceNew(Black, Tzaar, p)
	nb := b.Clone()
	nb.Apply(Move{From: p, To: p.Add(Directions[0])})
	if _, stillThere := b.At(p); !stillThere {
		t.Fatal("mutating the clone must not affect the original board")
	}
}
