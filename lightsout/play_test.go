//go:build playtest

package main

// Headless PLAYTHROUGH test: boots the real app, starts a scrambled puzzle, then
// SOLVES it by tapping the grid through the real touch path and asserts the app
// reaches its win state. This proves the generated puzzle is actually solvable
// via the UI and that win detection fires. Runs under the pure-Go inkview
// emulator (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"
)

// tapCell taps the centre of grid cell (r,c) using the geometry the app cached
// on its last Draw.
func tapCell(h *ink.Harness, a *app, r, c int) {
	x := a.gridX + c*a.cell + a.cell/2
	y := a.gridY + r*a.cell + a.cell/2
	h.Tap(image.Pt(x, y))
}

func TestPlayLightsOutSolve(t *testing.T) {
	a := newApp()
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}

	// Splash -> menu.
	h.Tap(image.Pt(500, 700))
	if a.scr != screenMenu {
		t.Fatalf("splash tap did not open menu, scr=%v", a.scr)
	}
	if len(a.menuBtns) == 0 {
		t.Fatalf("no difficulty buttons on menu; visible: %v", texts(h))
	}

	// Start the first offered puzzle.
	h.TapRect(a.menuBtns[0].rect)
	if a.scr != screenPlay {
		t.Fatalf("did not enter play screen, scr=%v", a.scr)
	}
	if a.board == nil || a.board.Solved() {
		t.Fatalf("expected a scrambled, unsolved board; solved=%v", a.board.Solved())
	}
	startLit := a.board.Count()
	if startLit == 0 {
		t.Fatal("fresh puzzle has no lit cells")
	}

	// Ask the game's own solver which cells to press, then press them through the
	// touch grid. Presses commute, so applying the whole solution wins regardless
	// of order.
	solution, ok := a.board.Solve()
	if !ok {
		t.Fatal("generated puzzle is unsolvable (solver returned false)")
	}
	presses := 0
	for r := 0; r < a.size; r++ {
		for c := 0; c < a.size; c++ {
			if solution[r][c] {
				tapCell(h, a, r, c)
				presses++
			}
		}
	}

	if !a.won {
		t.Fatalf("board not won after applying the solution (%d presses); %d lights still on",
			presses, a.board.Count())
	}
	if !a.board.Solved() {
		t.Fatalf("app.won set but board not actually solved (%d lit)", a.board.Count())
	}
	if a.board.Moves != presses {
		t.Fatalf("move counter %d != presses made %d", a.board.Moves, presses)
	}

	if dir := os.Getenv("PLAYTEST_SHOTS"); dir != "" {
		if err := h.Screenshot(dir + "/lightsout_win.png"); err != nil {
			t.Fatalf("screenshot: %v", err)
		}
	}
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}
