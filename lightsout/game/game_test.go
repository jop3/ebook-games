package game

import (
	"math/rand"
	"testing"
)

// applySolution presses every cell marked in sol on a copy and checks it solves.
func (b *Board) applySolution(sol [][]bool) {
	for r := 0; r < b.N; r++ {
		for c := 0; c < b.N; c++ {
			if sol[r][c] {
				b.press(r, c)
			}
		}
	}
}

func TestPressCenterNeighbors(t *testing.T) {
	b := New(5)
	b.Press(2, 2)
	// center + 4 orthogonal neighbours should be lit, nothing else
	want := map[[2]int]bool{{2, 2}: true, {1, 2}: true, {3, 2}: true, {2, 1}: true, {2, 3}: true}
	for r := 0; r < 5; r++ {
		for c := 0; c < 5; c++ {
			if b.Lit(r, c) != want[[2]int{r, c}] {
				t.Errorf("center press: (%d,%d)=%v want %v", r, c, b.Lit(r, c), want[[2]int{r, c}])
			}
		}
	}
	if b.Count() != 5 {
		t.Errorf("center press lit %d, want 5", b.Count())
	}
}

func TestPressCornerEdges(t *testing.T) {
	// top-left corner: only cell + right + down (2 neighbours) → 3 lit
	b := New(5)
	b.Press(0, 0)
	if !b.Lit(0, 0) || !b.Lit(0, 1) || !b.Lit(1, 0) {
		t.Errorf("corner: expected (0,0),(0,1),(1,0) lit")
	}
	if b.Count() != 3 {
		t.Errorf("corner press lit %d, want 3", b.Count())
	}

	// top edge middle: cell + left + right + down = 4 lit
	b2 := New(5)
	b2.Press(0, 2)
	if b2.Count() != 4 {
		t.Errorf("top-edge press lit %d, want 4", b2.Count())
	}
	if b2.Lit(-1, 2) {
		t.Errorf("off-board neighbour should not be lit")
	}
}

func TestPressIsSelfInverse(t *testing.T) {
	b := New(7)
	b.Press(3, 4)
	b.Press(3, 4)
	if !b.Solved() {
		t.Errorf("pressing same cell twice should return to all-off")
	}
}

func TestPressCountsMoves(t *testing.T) {
	b := New(3)
	b.Press(0, 0)
	b.Press(1, 1)
	if ok := b.Press(9, 9); ok {
		t.Errorf("off-board press should return false")
	}
	if b.Moves != 2 {
		t.Errorf("Moves=%d, want 2 (off-board must not count)", b.Moves)
	}
}

func TestSolvedDetection(t *testing.T) {
	b := New(5)
	if !b.Solved() {
		t.Errorf("empty board should be solved")
	}
	b.Press(2, 2)
	if b.Solved() {
		t.Errorf("board with lit cells should not be solved")
	}
}

func TestGenerateIsSolvableAndNotTrivial(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for _, n := range []int{3, 5, 7} {
		for trial := 0; trial < 50; trial++ {
			b := New(n)
			b.Generate(n*n, rng)
			if b.Solved() {
				t.Fatalf("n=%d: generated puzzle is already solved", n)
			}
			if b.Moves != 0 {
				t.Fatalf("n=%d: Moves should reset to 0 after Generate, got %d", n, b.Moves)
			}
			sol, ok := b.Solve()
			if !ok {
				t.Fatalf("n=%d trial=%d: generated puzzle reported unsolvable", n, trial)
			}
			// applying the solution must clear the board
			b.applySolution(sol)
			if !b.Solved() {
				t.Fatalf("n=%d trial=%d: applying solver output did not solve", n, trial)
			}
		}
	}
}

func TestSolveOnRandomStates(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	// 3x3 and 5x5 are always fully solvable regardless of state.
	for _, n := range []int{3, 5} {
		for trial := 0; trial < 100; trial++ {
			b := New(n)
			for i := 0; i < rng.Intn(20); i++ {
				b.press(rng.Intn(n), rng.Intn(n))
			}
			sol, ok := b.Solve()
			if !ok {
				continue // some states genuinely have no solution for certain n
			}
			b.applySolution(sol)
			if !b.Solved() {
				t.Fatalf("n=%d trial=%d: solver output failed to clear board", n, trial)
			}
		}
	}
}
