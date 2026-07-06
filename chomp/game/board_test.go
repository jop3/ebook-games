package game

import "testing"

func TestNewState(t *testing.T) {
	s := NewState(5, 6)
	if s.NumRows() != 5 {
		t.Fatalf("NumRows() = %d, want 5", s.NumRows())
	}
	for r := 0; r < 5; r++ {
		if s.RowLen(r) != 6 {
			t.Fatalf("RowLen(%d) = %d, want 6", r, s.RowLen(r))
		}
	}
	if s.RowLen(-1) != 0 || s.RowLen(5) != 0 {
		t.Fatal("RowLen out of range should return 0")
	}
	if s.Total() != 30 {
		t.Fatalf("Total() = %d, want 30", s.Total())
	}
	if s.Empty() {
		t.Fatal("a fresh rectangle must not be Empty")
	}
	if !s.Has(0, 0) {
		t.Fatal("the poisoned cell (0,0) must be present on a fresh board")
	}
	if !s.Has(4, 5) || s.Has(4, 6) || s.Has(5, 0) {
		t.Fatal("Has boundary check wrong for a 5x6 board")
	}
}

// TestApplyRemovedRegionShape hand-checks the exact removed-region shape for
// a few picked moves — the classic off-by-one risk is getting the row>=/
// col>= direction (and which corner is poisoned) backwards.
func TestApplyRemovedRegionShape(t *testing.T) {
	cases := []struct {
		name string
		move Move
		want State
	}{
		// Eating (1,2) on a 4x4 board: row 0 is untouched (row < 1); rows
		// 1..3 each keep only columns 0..1 (col < 2).
		{"mid-move clamps rows at/below to col", Move{Row: 1, Col: 2}, State{4, 2, 2, 2}},
		// Eating in row 0 clamps EVERY row (row >= 0 is everything).
		{"row-0 move clamps the whole board", Move{Row: 0, Col: 2}, State{2, 2, 2, 2}},
		// Eating col 0 at row 2 wipes rows 2..3 completely (col >= 0 is
		// every remaining column) but leaves rows above untouched.
		{"col-0 move clears rows below to nothing", Move{Row: 2, Col: 0}, State{4, 4, 0, 0}},
		// Eating the poisoned cell (0,0) clears the entire board.
		{"poison move clears everything", Move{Row: 0, Col: 0}, State{0, 0, 0, 0}},
		// A move on the last row/col only removes that single cell.
		{"corner move removes just one cell", Move{Row: 3, Col: 3}, State{4, 4, 4, 3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := NewState(4, 4)
			got := s.Apply(c.move)
			if !equalState(got, c.want) {
				t.Fatalf("Apply(%v) = %v, want %v", c.move, got, c.want)
			}
			// The original must be untouched (Apply must not mutate the
			// receiver — GameState relies on this to keep old references
			// like "before" snapshots valid in tests).
			if !equalState(s, NewState(4, 4)) {
				t.Fatalf("Apply mutated the receiver: %v", s)
			}
		})
	}
}

// TestApplyChainedMoveKeepsMonotonic checks that repeated Apply calls keep
// producing a valid (non-increasing) staircase, and hand-verifies the exact
// shape after a second move on an already-eaten board.
func TestApplyChainedMoveKeepsMonotonic(t *testing.T) {
	s := NewState(4, 4).Apply(Move{Row: 1, Col: 2}) // -> {4,2,2,2}
	if !equalState(s, State{4, 2, 2, 2}) {
		t.Fatalf("setup: got %v, want {4,2,2,2}", s)
	}
	// Eating (1,1): rows 1..3 clamp to col 1 (only column 0 survives there).
	s2 := s.Apply(Move{Row: 1, Col: 1})
	if !equalState(s2, State{4, 1, 1, 1}) {
		t.Fatalf("Apply({1,1}) on {4,2,2,2} = %v, want {4,1,1,1}", s2)
	}
	assertNonIncreasing(t, s2)
}

func TestApplyIllegalMoveIsNoop(t *testing.T) {
	s := NewState(3, 3)
	// (1,3) is out of range for row 1 (only columns 0..2 exist).
	got := s.Apply(Move{Row: 1, Col: 3})
	if !equalState(got, s) {
		t.Fatalf("Apply of an illegal move must be a no-op, got %v want %v", got, s)
	}
	// Row out of range entirely.
	got2 := s.Apply(Move{Row: 9, Col: 0})
	if !equalState(got2, s) {
		t.Fatalf("Apply of an out-of-range row must be a no-op, got %v", got2)
	}
}

func TestIsLegalAndHasAgree(t *testing.T) {
	s := State{4, 2, 2, 2}
	legalCases := []struct {
		m  Move
		ok bool
	}{
		{Move{0, 0}, true},
		{Move{0, 3}, true},
		{Move{0, 4}, false}, // row 0 only has 4 cells (cols 0..3)
		{Move{1, 1}, true},
		{Move{1, 2}, false}, // row 1 only has 2 cells (cols 0..1)
		{Move{3, 1}, true},
		{Move{4, 0}, false}, // row out of range
		{Move{-1, 0}, false},
	}
	for _, c := range legalCases {
		if s.IsLegal(c.m) != c.ok {
			t.Errorf("IsLegal(%v) = %v, want %v", c.m, s.IsLegal(c.m), c.ok)
		}
		if s.Has(c.m.Row, c.m.Col) != c.ok {
			t.Errorf("Has(%d,%d) = %v, want %v", c.m.Row, c.m.Col, s.Has(c.m.Row, c.m.Col), c.ok)
		}
	}
}

func TestLegalMovesCountsEveryRemainingCell(t *testing.T) {
	s := State{4, 2, 2, 2}
	moves := s.LegalMoves()
	if len(moves) != s.Total() {
		t.Fatalf("LegalMoves count = %d, want Total() = %d", len(moves), s.Total())
	}
	for _, m := range moves {
		if !s.Has(m.Row, m.Col) {
			t.Errorf("LegalMoves produced a move %v not actually on the board", m)
		}
	}
}

func TestMoveIsPoison(t *testing.T) {
	if !(Move{Row: 0, Col: 0}).IsPoison() {
		t.Fatal("(0,0) must be the poisoned cell")
	}
	cases := []Move{{0, 1}, {1, 0}, {1, 1}, {2, 3}}
	for _, m := range cases {
		if m.IsPoison() {
			t.Errorf("Move %v must NOT be considered poison", m)
		}
	}
}

func TestEmptyAfterEatingEverything(t *testing.T) {
	s := NewState(3, 3)
	s = s.Apply(Move{Row: 0, Col: 0})
	if !s.Empty() {
		t.Fatal("eating the poisoned cell must clear the whole board")
	}
	if s.Total() != 0 {
		t.Fatalf("Total() after clearing = %d, want 0", s.Total())
	}
	if len(s.LegalMoves()) != 0 {
		t.Fatal("an empty board must have no legal moves left")
	}
}

// --- test helpers ------------------------------------------------------

func equalState(a, b State) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func assertNonIncreasing(t *testing.T, s State) {
	t.Helper()
	for i := 1; i < len(s); i++ {
		if s[i] > s[i-1] {
			t.Fatalf("state %v is not a valid non-increasing staircase at index %d", s, i)
		}
	}
}
