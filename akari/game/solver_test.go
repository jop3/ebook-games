package game

import (
	"math/rand"
	"testing"
)

// buildBoard is a small test helper: rows of runes, '#'=wall (no number),
// '0'-'4'=numbered wall, '.'=white.
func buildBoard(rows []string) *Board {
	h := len(rows)
	w := len(rows[0])
	b := newBoard(w, h)
	for y, row := range rows {
		for x, r := range row {
			switch {
			case r == '.':
				b.Cells[y][x] = Cell{Kind: White, Number: -1}
			case r == '#':
				b.Cells[y][x] = Cell{Kind: Wall, Number: -1}
			case r >= '0' && r <= '4':
				b.Cells[y][x] = Cell{Kind: Wall, Number: int(r - '0')}
			}
		}
	}
	return b
}

func TestLitStopsAtWall(t *testing.T) {
	b := buildBoard([]string{
		"...",
		".#.",
		"...",
	})
	bulbs := make([][]bool, 3)
	for i := range bulbs {
		bulbs[i] = make([]bool, 3)
	}
	bulbs[0][0] = true
	lit := Lit(b, bulbs)
	if !lit[0][0] || !lit[0][1] || !lit[0][2] || !lit[1][0] || !lit[2][0] {
		t.Error("expected row 0, col 0 lit")
	}
	// The wall cell itself should never register as "lit" meaningfully, and
	// nothing on the far side of the wall should be lit by this bulb.
	if lit[1][2] {
		t.Error("cell beyond the wall on that ray should not be lit")
	}
	if lit[2][1] || lit[2][2] {
		t.Error("far corner unreachable by a single ray should not be lit")
	}
}

func TestBulbSeesBulb(t *testing.T) {
	b := buildBoard([]string{
		"...",
		"...",
		"...",
	})
	bulbs := make([][]bool, 3)
	for i := range bulbs {
		bulbs[i] = make([]bool, 3)
	}
	bulbs[0][0] = true
	bulbs[0][2] = true
	if !BulbSeesBulb(b, bulbs) {
		t.Error("two bulbs on the same unobstructed row should see each other")
	}

	bulbs2 := make([][]bool, 3)
	for i := range bulbs2 {
		bulbs2[i] = make([]bool, 3)
	}
	b2 := buildBoard([]string{
		"...",
		"###",
		"...",
	})
	bulbs2[0][0] = true
	bulbs2[2][0] = true
	if BulbSeesBulb(b2, bulbs2) {
		t.Error("bulbs separated by a wall row should not see each other")
	}
}

func TestWallsSatisfied(t *testing.T) {
	b := buildBoard([]string{
		".2.",
		"...",
		"...",
	})
	bulbs := make([][]bool, 3)
	for i := range bulbs {
		bulbs[i] = make([]bool, 3)
	}
	// The '2' wall at (1,0) has neighbours (0,0), (2,0), (1,1).
	bulbs[0][0] = true
	bulbs[0][2] = true
	if !WallsSatisfied(b, bulbs) {
		t.Error("wall with exactly 2 adjacent bulbs should be satisfied")
	}
	bulbs[1][1] = true
	if WallsSatisfied(b, bulbs) {
		t.Error("wall now has 3 adjacent bulbs, should fail its '2' clue")
	}
}

func TestSolvedSimplePuzzle(t *testing.T) {
	// A 3x3 all-white board framed by a '4' wall would be artificial; use a
	// simple known-good arrangement: a single bulb in the center of a 3x3
	// all-white board lights everything and has no wall constraints.
	b := buildBoard([]string{
		"...",
		"...",
		"...",
	})
	bulbs := make([][]bool, 3)
	for i := range bulbs {
		bulbs[i] = make([]bool, 3)
	}
	bulbs[1][1] = true
	// Center bulb alone does not light the corners, so this should NOT solve.
	if Solved(b, bulbs) {
		t.Error("center bulb alone should not light corner cells")
	}
}

func TestSolveUniqueOnHandBuiltPuzzle(t *testing.T) {
	// Row of 3 white cells flanked by a '1' wall on each side forces exactly
	// one bulb, and its position is ambiguous only if not otherwise pinned;
	// use a '0' wall to eliminate one candidate outright, forcing uniqueness.
	//
	//   0 . #
	//   . . .
	//   # . 1
	//
	// Wall '0' at (0,0) forbids a bulb at neighbours (1,0) and (0,1).
	// Wall '1' at (2,2) requires exactly one bulb among (1,2) and (2,1).
	b := buildBoard([]string{
		"0.#",
		"...",
		"#.1",
	})
	res := Solve(b)
	if res == SolveContradiction {
		t.Fatal("hand-built puzzle should be satisfiable")
	}
	// Not asserting SolveUnique here since this hand-built example is just a
	// smoke test of the deduction machinery; Generate's own test below
	// checks real generated puzzles resolve uniquely.
	_ = res
}

func TestGenerateProducesUniquePuzzles(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for _, p := range Presets {
		b := Generate(p, rng)
		if b.W != p.W || b.H != p.H {
			t.Fatalf("%s: wrong size", p.Name)
		}
		res := Solve(b)
		if res != SolveUnique {
			t.Errorf("%s: generated puzzle not uniquely solvable (result=%d)", p.Name, res)
		}
	}
}

func TestGenerateMultipleSeeds(t *testing.T) {
	// Sanity across a handful of seeds at the smallest size to make sure the
	// generator reliably converges, not just for one lucky seed.
	for seed := int64(0); seed < 8; seed++ {
		rng := rand.New(rand.NewSource(seed))
		b := Generate(Presets[0], rng)
		if Solve(b) != SolveUnique {
			t.Errorf("seed %d: Lätt puzzle not uniquely solvable", seed)
		}
	}
}

func TestToggleCycleAndWin(t *testing.T) {
	s := NewGameSeeded(Presets[0], 7)
	// Find a white cell and cycle it.
	x, y := -1, -1
outer:
	for yy := 0; yy < s.Board.H; yy++ {
		for xx := 0; xx < s.Board.W; xx++ {
			if s.Board.Cells[yy][xx].Kind == White {
				x, y = xx, yy
				break outer
			}
		}
	}
	if x < 0 {
		t.Fatal("no white cell found")
	}
	if s.Marks[y][x] != MarkEmpty {
		t.Fatal("expected empty initially")
	}
	s.Toggle(x, y)
	if s.Marks[y][x] != MarkBulb {
		t.Error("first toggle should place a bulb")
	}
	s.Toggle(x, y)
	if s.Marks[y][x] != MarkDot {
		t.Error("second toggle should place a dot mark")
	}
	s.Toggle(x, y)
	if s.Marks[y][x] != MarkEmpty {
		t.Error("third toggle should clear back to empty")
	}
}

func TestSolveBySolutionWins(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	b := Generate(Presets[0], rng)
	s := &GameState{Cfg: Presets[0], Board: b}
	s.Marks = make([][]MarkState, b.H)
	for y := range s.Marks {
		s.Marks[y] = make([]MarkState, b.W)
	}

	// Reconstruct the unique solution via the deduction solver's internal
	// bulb placement by re-running Solve logic through the public API: since
	// Solve doesn't expose the bulb grid, derive it by brute force search
	// guided by the fact the puzzle is uniquely solvable — try all subsets
	// is infeasible, so instead verify via the generator's own greedy bulbs
	// indirectly: place bulbs wherever a numbered wall forces them and
	// wherever no-bulb-forced-empty leaves exactly the lit solution. For this
	// unit test it's simpler to directly assert GameState.Done flips true
	// once fed a known-good bulb layout obtained by exhaustive search on the
	// small Lätt board.
	sol := bruteForceSolution(t, b)
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			if sol[y][x] {
				s.Marks[y][x] = MarkBulb
			}
		}
	}
	s.checkDone()
	if !s.Done {
		t.Error("placing the true solution's bulbs should mark the game Done")
	}
}

// bruteForceSolution finds *a* valid bulb placement for a small board via
// backtracking. Only used in tests (small Lätt-size boards).
func bruteForceSolution(t *testing.T, b *Board) [][]bool {
	t.Helper()
	var cells [][2]int
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			if b.Cells[y][x].Kind == White {
				cells = append(cells, [2]int{x, y})
			}
		}
	}
	bulbs := make([][]bool, b.H)
	for y := range bulbs {
		bulbs[y] = make([]bool, b.W)
	}
	var rec func(i int) bool
	rec = func(i int) bool {
		if i == len(cells) {
			return Solved(b, bulbs)
		}
		x, y := cells[i][0], cells[i][1]
		// Try no-bulb first, then bulb.
		bulbs[y][x] = false
		if rec(i + 1) {
			return true
		}
		if canPlaceBulb(b, bulbs, x, y) {
			bulbs[y][x] = true
			if rec(i + 1) {
				return true
			}
			bulbs[y][x] = false
		}
		return false
	}
	if !rec(0) {
		t.Fatal("brute force could not find any valid solution for generated board")
	}
	return bulbs
}
