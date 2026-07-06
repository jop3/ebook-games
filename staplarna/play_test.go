//go:build playtest

package main

// Headless PLAYTHROUGH tests for Staplarna (TZAAR). They drive the real touch
// path against the rules as written (see rulesParagraphs in ui.go): Black
// places first during the setup phase (30 pieces per side: 6 Tzaar, 9
// Tzarra, 15 Tott, alternating turns, any empty cell); once all 60 are down,
// stacks move EXACTLY as far as they are tall along one of the 6 hex
// directions, never through an occupied cell; landing on an empty cell is a
// plain move, on one's own stack a merge, on an enemy stack no taller than
// the mover a whole-stack capture (including any buried pieces); a capturing
// move is never mandatory; the game ends when a side has zero pieces of any
// ONE type left, or no legal move. Runs under the pure-Go inkview emulator
// (playtest/play.sh).

import (
	"math/rand"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"staplarna/game"
)

// --- helpers -----------------------------------------------------------

func bootToMenu(t *testing.T) (*ink.Harness, *app) {
	t.Helper()
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700) // dismiss splash
	if a.screen != screenMenu {
		t.Fatalf("splash tap did not open menu, screen=%v", a.screen)
	}
	return h, a
}

// startOpponent picks the first menu row for the given opponent (and, for the
// AI, the given search depth), taps it, and enters the game.
func startOpponent(t *testing.T, h *ink.Harness, a *app, opp game.Opponent, depth int) {
	t.Helper()
	for _, row := range a.menu.rows {
		if row.choice.opponent == opp && (opp == game.OpponentHotseat || row.choice.aiDepth == depth) {
			h.TapRect(row.rect)
			if a.screen != screenGame || a.gs == nil || a.gs.Opponent != opp {
				t.Fatalf("did not start opponent %v (screen=%v)", opp, a.screen)
			}
			return
		}
	}
	t.Fatalf("no menu row for opponent %v depth %d; visible: %v", opp, depth, texts(h))
}

// tapPoint taps the screen location of board point p.
func tapPoint(h *ink.Harness, a *app, p game.Point) bool {
	c := a.layout.Center(p)
	return h.TapXY(c.X, c.Y)
}

// tapMove drives a full Staplarna move through the real UI: tap the origin
// stack (selecting it), then tap the destination.
func tapMove(h *ink.Harness, a *app, m game.Move) bool {
	if !tapPoint(h, a, m.From) {
		return false
	}
	return tapPoint(h, a, m.To)
}

// selectType taps the setup-bar chip for typ, if one is currently shown
// (chips only appear for types the side to move still has left to place).
func selectType(h *ink.Harness, a *app, typ game.PieceType) bool {
	for _, c := range a.setupChips {
		if c.Type == typ {
			return h.TapRect(c.Rect)
		}
	}
	return false
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// setStack places a stack directly on the board (bypassing setup/move
// legality), used to build specific test positions — the game package
// exports Board.Stacks as a plain map, exactly like every other hex game's
// play_test.go pokes its board directly (see ringar's clearBoard/Rings).
func setStack(b *game.Board, p game.Point, owner game.Side, top game.PieceType, comp [3]int) {
	b.Stacks[p] = game.Stack{
		Owner:  owner,
		Type:   top,
		Height: comp[0] + comp[1] + comp[2],
		Comp:   comp,
	}
}

func clearBoard(gs *game.GameState) {
	gs.Board.Stacks = map[game.Point]game.Stack{}
}

// lineFrom returns n points starting at from and stepping by d each time
// (all guaranteed to still be valid board cells for the small n used here) —
// used to build simple straight-line test positions.
func lineFrom(from, d game.Point, n int) []game.Point {
	pts := make([]game.Point, n)
	p := from
	for i := 0; i < n; i++ {
		pts[i] = p
		p = p.Add(d)
	}
	return pts
}

// --- RULE: setup-phase placement via real taps, chips, and turn order ------

func TestPlayStaplarnaSetupPlacement(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)

	if a.gs.Phase != game.PhaseSetup || a.gs.Turn != game.Black {
		t.Fatalf("game should start in setup, Black first; got phase=%v turn=%v", a.gs.Phase, a.gs.Turn)
	}

	// Place a handful of pieces, alternating sides, verifying the chip
	// selection actually drives which type gets placed and that the
	// remaining count decrements.
	wantTurn := game.Black
	pts := game.AllPoints()
	for i := 0; i < 8; i++ {
		if a.gs.Turn != wantTurn {
			t.Fatalf("placement %d: turn = %v, want %v", i, a.gs.Turn, wantTurn)
		}
		typ := game.Tott
		before := a.gs.RemainingCount(wantTurn, typ)
		if !selectType(h, a, typ) {
			t.Fatalf("placement %d: no Tott chip (remaining=%d)", i, before)
		}
		if a.setupType != typ {
			t.Fatalf("placement %d: tapping the Tott chip should select it, got %v", i, a.setupType)
		}
		if !tapPoint(h, a, pts[i]) {
			t.Fatalf("placement %d at %v should be accepted", i, pts[i])
		}
		if a.gs.RemainingCount(wantTurn, typ) != before-1 {
			t.Fatalf("placement %d: remaining Tott for %v = %d, want %d", i, wantTurn, a.gs.RemainingCount(wantTurn, typ), before-1)
		}
		wantTurn = wantTurn.Opponent()
	}
	if a.gs.Phase != game.PhaseSetup {
		t.Fatalf("should still be in setup after 8/60 placements, got %v", a.gs.Phase)
	}

	// Tapping an already-occupied cell must be rejected (no piece
	// duplicated/lost, turn unchanged).
	blackTott := a.gs.RemainingCount(game.Black, game.Tott)
	if tapPoint(h, a, pts[0]) && a.gs.RemainingCount(game.Black, game.Tott) != blackTott {
		t.Fatal("tapping an occupied cell must not place a piece")
	}
}

// --- GOTCHA: move distance is EXACT, never "up to" --------------------------

func TestPlayStaplarnaMoveExactDistance(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	a.gs.Phase = game.PhasePlaying
	a.gs.Turn = game.Black
	clearBoard(a.gs)

	line := lineFrom(game.Point{X: 0, Y: 0, Z: 0}, game.Point{X: 1, Y: -1, Z: 0}, 5)
	setStack(a.gs.Board, line[0], game.Black, game.Tzarra, [3]int{0, 3, 0}) // height 3
	h.Draw()

	// Select the stack ONCE — a rejected destination tap leaves the
	// selection standing (see tapMove in main.go), so the same selected
	// stack can be tried against several wrong distances in a row before
	// finally landing on the correct one.
	if !tapPoint(h, a, line[0]) {
		t.Fatal("selecting the height-3 stack should succeed")
	}
	// Distance 1 and 2 (short of height 3) must be rejected.
	if tapPoint(h, a, line[1]) {
		t.Fatal("a distance-1 move by a height-3 stack should be illegal")
	}
	if _, occ := a.gs.Board.At(line[1]); occ {
		t.Fatal("the rejected short move must not have actually landed")
	}
	if tapPoint(h, a, line[2]) {
		t.Fatal("a distance-2 move by a height-3 stack should be illegal")
	}
	// Distance 4 (past height 3) must also be rejected.
	if tapPoint(h, a, line[4]) {
		t.Fatal("a distance-4 move by a height-3 stack should be illegal")
	}
	// The exact distance (3) must be accepted — the selection from above is
	// still standing, so this is just the landing tap.
	if !tapPoint(h, a, line[3]) {
		t.Fatal("the exact-distance-3 move should be legal")
	}
	if st, occ := a.gs.Board.At(line[3]); !occ || st.Height != 3 {
		t.Fatalf("the stack should have landed at %v with height 3, got %v/%v", line[3], st, occ)
	}
	if _, occ := a.gs.Board.At(line[0]); occ {
		t.Fatal("the origin cell should be empty after the move")
	}
}

// --- GOTCHA: capturing a stack removes buried pieces too -------------------

func TestPlayStaplarnaCaptureRemovesBuriedPieces(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	a.gs.Phase = game.PhasePlaying
	a.gs.Turn = game.Black
	clearBoard(a.gs)

	line := lineFrom(game.Point{X: 0, Y: 0, Z: 0}, game.Point{X: 1, Y: -1, Z: 0}, 3)
	setStack(a.gs.Board, line[0], game.Black, game.Tzarra, [3]int{0, 2, 0}) // height 2 mover
	// White's stack at line[2]: topped by a Tott, but with a Tzaar BURIED
	// underneath — the classic TZAAR defensive tactic. Height 2 <= mover's 2,
	// so it's capturable.
	setStack(a.gs.Board, line[2], game.White, game.Tott, [3]int{1, 0, 1})
	if n := a.gs.Board.TypeCount(game.White, game.Tzaar); n != 1 {
		t.Fatalf("setup sanity: White should show 1 buried Tzaar, got %d", n)
	}
	h.Draw()

	if !tapMove(h, a, game.Move{From: line[0], To: line[2]}) {
		t.Fatal("the capturing move should be legal")
	}
	if n := a.gs.Board.TypeCount(game.White, game.Tzaar); n != 0 {
		t.Fatalf("capturing the whole stack should remove the buried Tzaar too, White Tzaar count = %d", n)
	}
	if n := a.gs.Board.TypeCount(game.White, game.Tott); n != 0 {
		t.Fatalf("the captured stack's Tott should also be gone, count = %d", n)
	}
	if st, occ := a.gs.Board.At(line[2]); !occ || st.Owner != game.Black || st.Height != 2 {
		t.Fatalf("Black's mover should now occupy %v with height 2, got %v/%v", line[2], st, occ)
	}
	if !a.gs.LastCaptured {
		t.Fatal("LastCaptured should be true after a capturing move")
	}
}

// --- GOTCHA: capture is NOT mandatory ---------------------------------------

func TestPlayStaplarnaCaptureNotMandatory(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	a.gs.Phase = game.PhasePlaying
	a.gs.Turn = game.Black
	clearBoard(a.gs)

	center := game.Point{X: 0, Y: 0, Z: 0}
	capturable := game.Point{X: 1, Y: -1, Z: 0}  // one direction: a capturable enemy
	plainEmpty := game.Point{X: -1, Y: 1, Z: 0}  // opposite direction: empty
	setStack(a.gs.Board, center, game.Black, game.Tott, [3]int{0, 0, 1})
	setStack(a.gs.Board, capturable, game.White, game.Tott, [3]int{0, 0, 1})
	h.Draw()

	// The center cell has all 6 neighbours on-board: 1 capture (toward the
	// lone White stack) plus 5 plain moves to the other empty neighbours.
	legal := game.LegalMoves(a.gs.Board, game.Black)
	if len(legal) != 6 {
		t.Fatalf("expected exactly 6 legal moves (1 capture + 5 plain), got %v", legal)
	}
	captures := 0
	for _, m := range legal {
		if m.To == capturable {
			captures++
		}
	}
	if captures != 1 {
		t.Fatalf("expected exactly 1 capturing move (to %v), got %d among %v", capturable, captures, legal)
	}

	// Choosing the NON-capturing move must still be accepted, even though a
	// capture is available elsewhere on the board.
	if !tapMove(h, a, game.Move{From: center, To: plainEmpty}) {
		t.Fatal("the non-capturing move should be legal even though a capture exists")
	}
	if st, occ := a.gs.Board.At(capturable); !occ || st.Owner != game.White {
		t.Fatal("the un-captured White stack must still be on the board")
	}
	if st, occ := a.gs.Board.At(plainEmpty); !occ || st.Owner != game.Black {
		t.Fatal("Black's stack should have made the plain move")
	}
	if a.gs.LastCaptured {
		t.Fatal("a plain (non-capturing) move must not report LastCaptured")
	}
}

// --- WIN: type elimination (not total piece count) --------------------------

func TestPlayStaplarnaTypeEliminationWin(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	a.gs.Phase = game.PhasePlaying
	a.gs.Turn = game.Black
	clearBoard(a.gs)

	line := lineFrom(game.Point{X: 0, Y: 0, Z: 0}, game.Point{X: 1, Y: -1, Z: 0}, 3)
	// Black holds at least one of every type elsewhere on the board so this
	// capture cannot accidentally read as Black's own elimination.
	setStack(a.gs.Board, game.Point{X: 4, Y: -4, Z: 0}, game.Black, game.Tzaar, [3]int{1, 0, 0})
	setStack(a.gs.Board, game.Point{X: -4, Y: 4, Z: 0}, game.Black, game.Tott, [3]int{0, 0, 1})
	setStack(a.gs.Board, line[0], game.Black, game.Tzarra, [3]int{0, 2, 0}) // mover, height 2

	// White's ONLY Tzaar sits right in the mover's capturing path; White also
	// holds other types elsewhere so this is a genuine single-type wipeout,
	// not merely an empty board.
	setStack(a.gs.Board, line[2], game.White, game.Tzaar, [3]int{1, 0, 0})
	setStack(a.gs.Board, game.Point{X: 0, Y: 4, Z: -4}, game.White, game.Tzarra, [3]int{0, 1, 0})
	setStack(a.gs.Board, game.Point{X: 0, Y: -4, Z: 4}, game.White, game.Tott, [3]int{0, 0, 1})
	h.Draw()

	if !tapMove(h, a, game.Move{From: line[0], To: line[2]}) {
		t.Fatal("the type-eliminating capture should be legal")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatalf("Phase should be Done once White's Tzaar hits 0, got %v", a.gs.Phase)
	}
	if a.gs.Winner() != game.Black {
		t.Fatalf("Winner() = %v, want Black", a.gs.Winner())
	}
	if _, ok := h.FindTextContains("Svart vann!"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- All 3 AI difficulties actually reply -----------------------------------

func TestPlayStaplarnaAllDifficultiesReply(t *testing.T) {
	for _, depth := range []int{game.DepthEasy, game.DepthMedium, game.DepthHard} {
		depth := depth
		t.Run(itoa(depth), func(t *testing.T) {
			h, a := bootToMenu(t)
			startOpponent(t, h, a, game.OpponentAI, depth)
			if a.gs.AIDepth != depth {
				t.Fatalf("AIDepth = %d, want %d", a.gs.AIDepth, depth)
			}
			a.gs.QuickRandomSetup(rand.New(rand.NewSource(1)))
			if a.gs.Phase != game.PhasePlaying {
				t.Fatalf("QuickRandomSetup should reach PhasePlaying, got %v", a.gs.Phase)
			}
			legal := game.LegalMoves(a.gs.Board, game.Black)
			if len(legal) == 0 {
				t.Fatal("Black should have a legal move at the start of play")
			}
			if !tapMove(h, a, legal[0]) {
				t.Fatal("Black's opening move should be legal")
			}
			if a.gs.AITurn() {
				t.Fatal("control returned on the AI's turn (deferred reply not drained)")
			}
		})
	}
}

// --- Full game, including the setup phase, driven entirely by taps --------

// bestStaplarnaMove picks a legal move for side, preferring the highest-value
// capture available (biggest scarcity-weighted haul first) so a scripted
// playthrough converges to a real type-elimination or no-legal-move result in
// a bounded number of plies, instead of wandering forever — the same
// deterministic "always take the best capture, else just move" policy
// hasami/play_test.go uses for its own full-game test.
func bestStaplarnaMove(b *game.Board, side game.Side) (game.Move, bool) {
	moves := game.LegalMoves(b, side)
	if len(moves) == 0 {
		return game.Move{}, false
	}
	weight := func(t game.PieceType) int {
		switch t {
		case game.Tzaar:
			return 3
		case game.Tzarra:
			return 2
		default:
			return 1
		}
	}
	best := moves[0]
	bestScore := -1
	for _, m := range moves {
		mover, _ := b.At(m.From)
		tgt, occ := b.At(m.To)
		if !occ || tgt.Owner == mover.Owner {
			continue // not a capture
		}
		score := 0
		for _, t := range game.AllTypes {
			score += weight(t) * tgt.Comp[t]
		}
		if score > bestScore {
			bestScore, best = score, m
		}
	}
	return best, true
}

func TestPlayStaplarnaFullGameToWin(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentAI, game.DepthEasy)

	// --- setup phase: alternate placements via real taps (chip select, then
	// tap an empty cell) until all 60 pieces are down. White is the AI and
	// places itself automatically once its turn comes (drained inside the
	// preceding Tap call), so this loop only ever acts on Black's turn.
	for i := 0; a.gs.Phase == game.PhaseSetup; i++ {
		if i > 70 {
			t.Fatal("setup phase did not complete in a reasonable number of placements")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's setup turn (deferred reply not drained)")
		}
		avail := a.gs.AvailableTypes(a.gs.Turn)
		if len(avail) == 0 {
			t.Fatalf("no available type to place for %v", a.gs.Turn)
		}
		typ := avail[0]
		if !selectType(h, a, typ) {
			t.Fatalf("no setup chip for type %v", typ)
		}
		var target game.Point
		found := false
		for _, p := range game.AllPoints() {
			if _, occ := a.gs.Board.At(p); !occ {
				target, found = p, true
				break
			}
		}
		if !found {
			t.Fatal("no empty cell left to place on")
		}
		if !tapPoint(h, a, target) {
			t.Fatalf("setup placement at %v (type %v) was rejected", target, typ)
		}
	}
	if a.gs.PlacedCount() != 2*game.TotalPerSide {
		t.Fatalf("setup should place all %d pieces, got %d", 2*game.TotalPerSide, a.gs.PlacedCount())
	}
	if a.gs.Board.StackCount(game.Black) == 0 || a.gs.Board.StackCount(game.White) == 0 {
		t.Fatal("both sides should have stacks on the board after setup")
	}

	// --- play phase: drive Black via a capture-seeking heuristic through
	// the real tap path; White replies via the built-in AI automatically.
	for ply := 0; a.gs.Phase == game.PhasePlaying; ply++ {
		if ply > 2000 {
			t.Fatal("game did not terminate in a reasonable number of plies")
		}
		if a.gs.AITurn() {
			t.Fatal("control returned on the AI's turn (deferred reply not drained)")
		}
		m, ok := bestStaplarnaMove(a.gs.Board, a.gs.Turn)
		if !ok {
			t.Fatalf("human to move but no legal move at ply %d", ply)
		}
		if !tapMove(h, a, m) {
			t.Fatalf("legal move %v at ply %d was rejected", m, ply)
		}
	}

	if a.gs.Phase != game.PhaseDone {
		t.Fatalf("Phase should be Done at the end of the game, got %v", a.gs.Phase)
	}
	want := "Svart vann!"
	if a.gs.Winner() == game.White {
		want = "Vit vann!"
	}
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("end banner %q not shown; visible: %v", want, texts(h))
	}
}

// --- Quit mid-game (Back key AND the Meny button), then restart ------------

func TestPlayStaplarnaQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	tapPoint(h, a, game.AllPoints()[0]) // a placement in progress

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}

	startOpponent(t, h, a, game.OpponentAI, game.DepthEasy)
	tappedMeny := false
	for _, b := range a.buttons {
		if b.Label == "Meny" {
			h.TapRect(b.Rect)
			tappedMeny = true
		}
	}
	if !tappedMeny {
		t.Fatalf("no Meny button in game; visible: %v", texts(h))
	}
	if a.screen != screenMenu {
		t.Fatalf("Meny button did not return to menu, screen=%v", a.screen)
	}
	// Menu still usable afterwards.
	startOpponent(t, h, a, game.OpponentHotseat, 0)
}

// --- "Ny" restarts the current configuration mid-game -----------------------

func TestPlayStaplarnaNyRestarts(t *testing.T) {
	h, a := bootToMenu(t)
	startOpponent(t, h, a, game.OpponentHotseat, 0)
	tapPoint(h, a, game.AllPoints()[0])
	if a.gs.Turn != game.White || a.gs.PlacedCount() != 1 {
		t.Fatal("setup: expected exactly one placement to have been made")
	}
	tappedNy := false
	for _, b := range a.buttons {
		if b.Label == "Ny" {
			h.TapRect(b.Rect)
			tappedNy = true
		}
	}
	if !tappedNy {
		t.Fatalf("no Ny button in game; visible: %v", texts(h))
	}
	if a.gs.Turn != game.Black || a.gs.Phase != game.PhaseSetup || a.gs.PlacedCount() != 0 {
		t.Fatal("Ny should reset to a fresh setup-phase game")
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlayStaplarnaRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("TZAAR"); !ok {
		t.Fatalf("rules text missing the TZAAR credit; visible: %v", texts(h))
	}
	if _, ok := h.FindTextContains("INTE obligatoriskt"); !ok {
		t.Fatalf("rules text missing the capture-not-mandatory rule; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayStaplarnaScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/staplarna_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/staplarna_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/staplarna_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startOpponent(t, h, a, game.OpponentHotseat, 0)
	pts := game.AllPoints()
	for i := 0; i < 10; i++ {
		tapPoint(h, a, pts[i])
	}
	if err := h.Screenshot(dir + "/staplarna_setup.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	a.gs.QuickRandomSetup(rand.New(rand.NewSource(2)))
	h.Draw()
	if err := h.Screenshot(dir + "/staplarna_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	clearBoard(a.gs)
	line := lineFrom(game.Point{X: 0, Y: 0, Z: 0}, game.Point{X: 1, Y: -1, Z: 0}, 3)
	setStack(a.gs.Board, line[0], game.Black, game.Tzaar, [3]int{2, 0, 0})
	setStack(a.gs.Board, line[2], game.White, game.Tott, [3]int{0, 0, 1})
	a.gs.Phase = game.PhaseDone
	h.Draw()
	if err := h.Screenshot(dir + "/staplarna_end.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
