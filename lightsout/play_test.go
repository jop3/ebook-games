//go:build playtest

package main

// Headless PLAYTHROUGH tests for Lights Out. They drive the real touch path and
// check the gameplay against the rules as written (see rulesParagraphs in
// ui.go): pressing a square toggles it AND its four orthogonal neighbours (fewer
// at the edges), every board is solvable, "Losning" marks exactly the cells to
// press, "Ny" reshuffles, and the goal is to turn every light off. There is no
// loss condition. Runs under the pure-Go inkview emulator (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"
)

// --- helpers ----------------------------------------------------------------

func bootToMenu(t *testing.T) (*ink.Harness, *app) {
	t.Helper()
	a := newApp()
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700) // dismiss splash
	if a.scr != screenMenu {
		t.Fatalf("splash tap did not open menu, scr=%v", a.scr)
	}
	if len(a.menuBtns) == 0 {
		t.Fatalf("no difficulty buttons on menu; visible: %v", texts(h))
	}
	return h, a
}

// startSize taps the menu button for board size n and returns once in play.
func startSize(t *testing.T, h *ink.Harness, a *app, n int) {
	t.Helper()
	for _, b := range a.menuBtns {
		if b.size == n {
			h.TapRect(b.rect)
			if a.scr != screenPlay {
				t.Fatalf("size %d button did not enter play, scr=%v", n, a.scr)
			}
			if a.size != n {
				t.Fatalf("started size %d, wanted %d", a.size, n)
			}
			return
		}
	}
	t.Fatalf("no %dx%d button on menu", n, n)
}

// tapCell taps the centre of grid cell (row r, col c).
func tapCell(h *ink.Harness, a *app, r, c int) {
	x := a.gridX + c*a.cell + a.cell/2
	y := a.gridY + r*a.cell + a.cell/2
	h.Tap(image.Pt(x, y))
}

// snapshot copies the current lit grid.
func snapshot(a *app) [][]bool {
	g := make([][]bool, a.size)
	for r := 0; r < a.size; r++ {
		g[r] = make([]bool, a.size)
		for c := 0; c < a.size; c++ {
			g[r][c] = a.board.Lit(r, c)
		}
	}
	return g
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- WIN, every size (all "sides") ------------------------------------------

func TestPlayLightsOutSolveAllSizes(t *testing.T) {
	for _, n := range []int{3, 5, 7} {
		n := n
		t.Run(sizeName(n), func(t *testing.T) {
			h, a := bootToMenu(t)
			startSize(t, h, a, n)
			if a.board == nil || a.board.Solved() {
				t.Fatalf("fresh %dx%d puzzle should be unsolved", n, n)
			}

			// The game's own GF(2) solver says which cells to press; press them
			// through the grid. Presses commute, so order does not matter.
			solution, ok := a.board.Solve()
			if !ok {
				t.Fatalf("%dx%d puzzle reported unsolvable (rules say all are solvable)", n, n)
			}
			presses := 0
			for r := 0; r < n; r++ {
				for c := 0; c < n; c++ {
					if solution[r][c] {
						tapCell(h, a, r, c)
						presses++
					}
				}
			}
			if !a.won || !a.board.Solved() {
				t.Fatalf("%dx%d not solved after %d presses; %d lights still on",
					n, n, presses, a.board.Count())
			}
			if a.board.Moves != presses {
				t.Fatalf("move counter %d != presses %d", a.board.Moves, presses)
			}
			if _, ok := h.FindTextContains("tryck"); !ok {
				t.Fatalf("win banner missing; visible: %v", texts(h))
			}
		})
	}
}

func sizeName(n int) string { return string(rune('0'+n)) + "x" + string(rune('0'+n)) }

// --- RULE: pressing toggles the plus-shape (fewer at edges) ------------------

func TestPlayLightsOutToggleRule(t *testing.T) {
	h, a := bootToMenu(t)
	startSize(t, h, a, 3)

	// Interior press toggles all five of the plus.
	assertToggles(t, h, a, 1, 1, [][2]int{{1, 1}, {0, 1}, {2, 1}, {1, 0}, {1, 2}})
	// Corner press toggles only three (the two off-board neighbours don't count).
	assertToggles(t, h, a, 0, 0, [][2]int{{0, 0}, {1, 0}, {0, 1}})
	// Edge (non-corner) press toggles four.
	assertToggles(t, h, a, 0, 1, [][2]int{{0, 1}, {0, 0}, {0, 2}, {1, 1}})
}

// assertToggles taps (r,c) and checks exactly the expected cells flipped.
func assertToggles(t *testing.T, h *ink.Harness, a *app, r, c int, want [][2]int) {
	t.Helper()
	before := snapshot(a)
	tapCell(h, a, r, c)

	flipped := map[[2]int]bool{}
	for rr := 0; rr < a.size; rr++ {
		for cc := 0; cc < a.size; cc++ {
			if a.board.Lit(rr, cc) != before[rr][cc] {
				flipped[[2]int{rr, cc}] = true
			}
		}
	}
	wantSet := map[[2]int]bool{}
	for _, p := range want {
		wantSet[p] = true
	}
	if len(flipped) != len(wantSet) {
		t.Fatalf("press (%d,%d) flipped %v, expected %v", r, c, keys(flipped), want)
	}
	for p := range wantSet {
		if !flipped[p] {
			t.Fatalf("press (%d,%d) did not flip %v (flipped %v)", r, c, p, keys(flipped))
		}
	}
}

func keys(m map[[2]int]bool) [][2]int {
	var out [][2]int
	for k := range m {
		out = append(out, k)
	}
	return out
}

// --- RULE: "Losning" marks exactly the cells the solver would press ----------

func TestPlayLightsOutHintMatchesSolver(t *testing.T) {
	h, a := bootToMenu(t)
	startSize(t, h, a, 5)

	want, ok := a.board.Solve()
	if !ok {
		t.Fatal("puzzle unsolvable")
	}
	h.TapRect(a.btnHint) // show "Losning"
	if !a.solved || a.solution == nil {
		t.Fatal("hint did not turn on")
	}
	for r := 0; r < a.size; r++ {
		for c := 0; c < a.size; c++ {
			if a.solution[r][c] != want[r][c] {
				t.Fatalf("hint overlay disagrees with solver at (%d,%d)", r, c)
			}
		}
	}
	h.TapRect(a.btnHint) // toggle it back off
	if a.solved || a.solution != nil {
		t.Fatal("second tap did not hide the hint")
	}
}

// --- "Ny" reshuffles a fresh solvable puzzle of the same size ----------------

func TestPlayLightsOutNewPuzzle(t *testing.T) {
	h, a := bootToMenu(t)
	startSize(t, h, a, 5)

	// Make a move so we can see the counter reset.
	tapCell(h, a, 2, 2)
	if a.board.Moves == 0 {
		t.Fatal("a grid tap did not register a move")
	}

	h.TapRect(a.btnNew)
	if a.scr != screenPlay || a.size != 5 {
		t.Fatalf("Ny left play or changed size (scr=%v size=%d)", a.scr, a.size)
	}
	if a.won || a.board.Solved() {
		t.Fatal("Ny produced an already-solved/won puzzle")
	}
	if a.board.Moves != 0 {
		t.Fatalf("Ny did not reset the move counter (%d)", a.board.Moves)
	}
	if _, ok := a.board.Solve(); !ok {
		t.Fatal("Ny produced an unsolvable puzzle")
	}
}

// --- No input after a win; taps outside the grid are ignored -----------------

func TestPlayLightsOutInputGuards(t *testing.T) {
	h, a := bootToMenu(t)
	startSize(t, h, a, 3)

	// Tap far outside the grid — nothing should change.
	movesBefore := a.board.Moves
	h.TapXY(5, 5)
	if a.board.Moves != movesBefore {
		t.Fatal("a tap outside the grid registered a move")
	}

	// Solve it, then confirm the solved board is frozen.
	sol, _ := a.board.Solve()
	for r := 0; r < a.size; r++ {
		for c := 0; c < a.size; c++ {
			if sol[r][c] {
				tapCell(h, a, r, c)
			}
		}
	}
	if !a.won {
		t.Fatal("did not win")
	}
	frozen := a.board.Moves
	tapCell(h, a, 0, 0)
	if a.board.Moves != frozen || !a.board.Solved() {
		t.Fatal("board accepted a tap after the win")
	}
}

// --- Quit mid-game (Back key AND Meny button), then restart -----------------

func TestPlayLightsOutQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startSize(t, h, a, 5)
	tapCell(h, a, 1, 1) // a move in progress

	h.Back()
	if a.scr != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, scr=%v", a.scr)
	}

	startSize(t, h, a, 3)
	h.TapRect(a.btnMenu)
	if a.scr != screenMenu {
		t.Fatalf("Meny button did not return to menu, scr=%v", a.scr)
	}

	// Menu still works afterwards.
	startSize(t, h, a, 7)
}

// --- Rules screen -----------------------------------------------------------

func TestPlayLightsOutRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)

	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.scr != screenRules {
		t.Fatalf("Regler did not open rules, scr=%v", a.scr)
	}
	if _, ok := h.FindTextContains("grannar"); !ok { // "neighbours" — the core rule
		t.Fatalf("rules text does not explain the neighbour toggle; visible: %v", texts(h))
	}
	h.Back()
	if a.scr != screenMenu {
		t.Fatalf("Back did not leave the rules screen, scr=%v", a.scr)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayLightsOutWinScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	h, a := bootToMenu(t)
	startSize(t, h, a, 5)
	sol, _ := a.board.Solve()
	for r := 0; r < a.size; r++ {
		for c := 0; c < a.size; c++ {
			if sol[r][c] {
				tapCell(h, a, r, c)
			}
		}
	}
	if err := h.Screenshot(dir + "/lightsout_win.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
