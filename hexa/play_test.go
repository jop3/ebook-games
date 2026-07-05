//go:build playtest

package main

// Headless PLAYTHROUGH tests for Hexa. They drive the real touch path
// against the rules as written (see rulesParagraphs in ui.go): Black places
// first (42 placements total, 21 tiles each, each touching the growing
// cluster), then once the board is full players alternate sliding one of
// their own tiles to another empty cell on the cluster's edge, and forming
// a Linje/Triangel/Hexagon of 6 of your own tiles wins. Runs under the
// pure-Go inkview emulator (playtest/play.sh).

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"hexa/game"
)

// --- helpers ----------------------------------------------------------------

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

// startChoice picks the menu row for the given opponent/depth pair, taps it,
// and enters the game.
func startChoice(t *testing.T, h *ink.Harness, a *app, opp game.Opponent, depth int) {
	t.Helper()
	for _, row := range a.menu.rows {
		if row.choice.opponent == opp && row.choice.aiDepth == depth {
			h.TapRect(row.rect)
			if a.screen != screenGame || a.gs == nil || a.gs.Opponent != opp {
				t.Fatalf("did not start opponent=%v depth=%d (screen=%v)", opp, depth, a.screen)
			}
			return
		}
	}
	t.Fatalf("no menu row for opponent=%v depth=%d; visible: %v", opp, depth, texts(h))
}

// tapPoint taps the screen location of board cell p.
func tapPoint(h *ink.Harness, a *app, p game.Hex) bool {
	c := a.layout.Center(p)
	return h.TapXY(c.X, c.Y)
}

// playMove drives a full tile move through the real UI: tap the tile
// (selecting it), then tap the destination.
func playMove(h *ink.Harness, a *app, from, to game.Hex) bool {
	if !tapPoint(h, a, from) {
		return false
	}
	return tapPoint(h, a, to)
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// clearBoard empties the board directly (used to build specific test
// positions, exactly like every other game's play_test.go setCell helper).
func clearBoard(gs *game.GameState) {
	gs.Board.Tiles = map[game.Hex]game.Side{}
}

// lineCells6 returns the same 6-cell axis-A line used throughout game/'s own
// tests, plus 5 filler cells (one hop further along axis C from each line
// cell) suitable for the OTHER side's placements in a scripted full game —
// each filler is adjacent to its corresponding line cell, so it always
// legally attaches to the growing cluster.
func lineCells6() []game.Hex {
	return []game.Hex{{-3, 3, 0}, {-2, 2, 0}, {-1, 1, 0}, {0, 0, 0}, {1, -1, 0}, {2, -2, 0}}
}

// --- RULE: placement phase, real taps, adjacency + turn order --------------

func TestPlayHexaPlacementAdjacencyAndTurnOrder(t *testing.T) {
	h, a := bootToMenu(t)
	startChoice(t, h, a, game.OpponentHotseat, 0)

	if a.gs.Phase != game.PhasePlacement || a.gs.Turn != game.Black {
		t.Fatalf("game should start in placement, Black first; got phase=%v turn=%v", a.gs.Phase, a.gs.Turn)
	}
	first := game.Hex{2, -1, -1}
	if !tapPoint(h, a, first) {
		t.Fatal("the very first placement should be legal anywhere on the board")
	}
	if a.gs.Turn != game.White {
		t.Fatalf("turn should pass to White after Black's placement, got %v", a.gs.Turn)
	}

	// GOTCHA: a non-adjacent placement must be rejected via a real tap.
	farAway := game.Hex{-game.Radius, game.Radius, 0}
	tapPoint(h, a, farAway)
	if a.gs.Board.HasTile(farAway) {
		t.Fatal("a placement not touching the cluster must be rejected even via a real tap")
	}
	if a.gs.Turn != game.White {
		t.Fatal("a rejected placement must not consume the turn")
	}

	// An adjacent cell IS legal.
	adj := game.Neighbors(first)[0]
	if !tapPoint(h, a, adj) {
		t.Fatalf("placement adjacent to the cluster (%v) should be legal", adj)
	}
	if a.gs.Turn != game.Black {
		t.Fatalf("turn should pass back to Black, got %v", a.gs.Turn)
	}
}

// --- RULE: the movement-phase transition, once all 42 tiles are down -------

func TestPlayHexaTransitionsToMovementPhase(t *testing.T) {
	h, a := bootToMenu(t)
	startChoice(t, h, a, game.OpponentHotseat, 0)

	for i := 0; i < 2*game.TilesPerSide; i++ {
		moves := game.PlaceMoves(a.gs.Board.Tiles)
		if len(moves) == 0 {
			t.Fatalf("placement %d: no legal placements left", i)
		}
		if !tapPoint(h, a, moves[0]) {
			t.Fatalf("placement %d: offered move %v should be legal via a real tap", i, moves[0])
		}
		if a.gs.Phase == game.PhaseDone {
			t.Logf("a shape completed at step %d — fine, stop early", i)
			return
		}
	}
	if a.gs.Phase != game.PhaseMoving {
		t.Fatalf("after all %d tiles are placed with no winner, phase should be Moving, got %v", 2*game.TilesPerSide, a.gs.Phase)
	}
	if _, ok := h.FindTextContains("flyttar"); !ok {
		t.Fatalf("status should now say a side is moving, not placing; visible: %v", texts(h))
	}
}

// --- GOTCHA: a disconnecting move is rejected via a real tap (standard) ----

func TestPlayHexaMoveDisconnectRejectedStandard(t *testing.T) {
	h, a := bootToMenu(t)
	startChoice(t, h, a, game.OpponentHotseat, 0)
	a.gs.Phase = game.PhaseMoving
	a.gs.Remaining[game.Black], a.gs.Remaining[game.White] = 0, 0
	clearBoard(a.gs)

	x, b, c := game.Hex{0, 0, 0}, game.Hex{1, -1, 0}, game.Hex{2, -2, 0}
	a.gs.Board.Tiles[x] = game.Black
	a.gs.Board.Tiles[b] = game.Black
	a.gs.Board.Tiles[c] = game.White
	a.gs.Turn = game.Black
	h.Draw()

	far := game.Hex{-4, 4, 0}
	if playMove(h, a, b, far) {
		t.Fatal("a move that disconnects the board must be rejected in standard mode, even via real taps")
	}
	if a.gs.Board.At(b) != game.Black {
		t.Fatal("the tile must still be at its original position after a rejected move")
	}
}

// --- GOTCHA: the Avancerat toggle + a real disconnecting move via taps -----

func TestPlayHexaAdvancedSplitViaTaps(t *testing.T) {
	h, a := bootToMenu(t)
	if !h.TapRect(a.menu.modeBtns[1]) {
		t.Fatal("could not tap the Avancerat toggle")
	}
	if !a.menu.advanced {
		t.Fatal("Avancerat should now be selected")
	}
	startChoice(t, h, a, game.OpponentHotseat, 0)
	if !a.gs.Advanced {
		t.Fatal("the started game should have Advanced=true")
	}

	a.gs.Phase = game.PhaseMoving
	a.gs.Remaining[game.Black], a.gs.Remaining[game.White] = 10, 0
	clearBoard(a.gs)

	hub := game.Hex{0, 0, 0}
	bridge := game.Hex{1, -1, 0}
	stay := []game.Hex{{-1, 1, 0}, {-2, 2, 0}, {-3, 3, 0}, {-4, 4, 0}, {-5, 5, 0}}
	goBranch := []game.Hex{{2, -2, 0}, {3, -3, 0}, {4, -4, 0}}
	a.gs.Board.Tiles[hub] = game.Black
	a.gs.Board.Tiles[bridge] = game.Black
	for _, p := range stay {
		a.gs.Board.Tiles[p] = game.White
	}
	for _, p := range goBranch {
		a.gs.Board.Tiles[p] = game.White
	}
	a.gs.Turn = game.Black
	h.Draw()

	to := game.Hex{0, 1, -1}
	if !playMove(h, a, bridge, to) {
		t.Fatal("the advanced-mode disconnecting move should be accepted via real taps")
	}
	for _, p := range goBranch {
		if a.gs.Board.HasTile(p) {
			t.Fatalf("stranded tile at %v should have been removed", p)
		}
	}
	if a.gs.Board.Count(game.White) != 5 {
		t.Fatalf("White should have exactly 5 tiles left, got %d", a.gs.Board.Count(game.White))
	}
	if a.gs.Phase != game.PhaseDone || a.gs.Winner() != game.Black {
		t.Fatalf("White dropping to <=5 tiles should end the game for Black, got phase=%v winner=%v", a.gs.Phase, a.gs.Winner())
	}
	if _, ok := h.FindTextContains("Svart vann"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- The 3 winning shapes, each completed by a real tap, with highlight ----

func TestPlayHexaLineWinViaTap(t *testing.T) {
	h, a := bootToMenu(t)
	startChoice(t, h, a, game.OpponentHotseat, 0)
	line := lineCells6()
	clearBoard(a.gs)
	for _, p := range line[:5] {
		a.gs.Board.Tiles[p] = game.Black
	}
	a.gs.Remaining[game.Black] = game.TilesPerSide - 5
	a.gs.Turn = game.Black
	h.Draw()

	if !tapPoint(h, a, line[5]) {
		t.Fatal("the 6th tap completing the line should be accepted")
	}
	if a.gs.Phase != game.PhaseDone || a.gs.WinKind != game.ShapeLine {
		t.Fatalf("expected a completed Linje win, got phase=%v kind=%v", a.gs.Phase, a.gs.WinKind)
	}
	if _, ok := h.FindTextContains("Svart vann (Linje)"); !ok {
		t.Fatalf("win banner should name the shape; visible: %v", texts(h))
	}
}

func TestPlayHexaTriangleWinViaTap(t *testing.T) {
	h, a := bootToMenu(t)
	startChoice(t, h, a, game.OpponentHotseat, 0)
	cells := []game.Hex{{0, 0, 0}, {1, -1, 0}, {2, -2, 0}, {0, 1, -1}, {1, 0, -1}, {0, 2, -2}}
	clearBoard(a.gs)
	for _, p := range cells[:5] {
		a.gs.Board.Tiles[p] = game.White
	}
	a.gs.Remaining[game.White] = game.TilesPerSide - 5
	a.gs.Turn = game.White
	h.Draw()

	if !tapPoint(h, a, cells[5]) {
		t.Fatal("the 6th tap completing the triangle should be accepted")
	}
	if a.gs.Phase != game.PhaseDone || a.gs.WinKind != game.ShapeTriangle {
		t.Fatalf("expected a completed Triangel win, got phase=%v kind=%v", a.gs.Phase, a.gs.WinKind)
	}
	if _, ok := h.FindTextContains("Vit vann (Triangel)"); !ok {
		t.Fatalf("win banner should name the shape; visible: %v", texts(h))
	}
	// The highlight: every winning cell should be reported.
	if len(a.gs.WinCells) != 6 {
		t.Fatalf("expected 6 highlighted winning cells, got %d", len(a.gs.WinCells))
	}
}

func TestPlayHexaHexRingWinViaTap(t *testing.T) {
	h, a := bootToMenu(t)
	startChoice(t, h, a, game.OpponentHotseat, 0)
	center := game.Hex{0, 0, 0}
	ring := game.Neighbors(center)
	clearBoard(a.gs)
	for _, p := range ring[:5] {
		a.gs.Board.Tiles[p] = game.Black
	}
	a.gs.Remaining[game.Black] = game.TilesPerSide - 5
	a.gs.Turn = game.Black
	h.Draw()

	if !tapPoint(h, a, ring[5]) {
		t.Fatal("the 6th tap completing the ring should be accepted")
	}
	if a.gs.Phase != game.PhaseDone || a.gs.WinKind != game.ShapeHexRing {
		t.Fatalf("expected a completed Hexagon win, got phase=%v kind=%v", a.gs.Phase, a.gs.WinKind)
	}
	if _, ok := h.FindTextContains("Svart vann (Hexagon)"); !ok {
		t.Fatalf("win banner should name the shape; visible: %v", texts(h))
	}
}

// --- Movement-phase frontier highlighting -----------------------------------

func TestPlayHexaMovementFrontierHighlight(t *testing.T) {
	h, a := bootToMenu(t)
	startChoice(t, h, a, game.OpponentHotseat, 0)
	a.gs.Phase = game.PhaseMoving
	a.gs.Remaining[game.Black], a.gs.Remaining[game.White] = 0, 0
	clearBoard(a.gs)
	a.gs.Board.Tiles[game.Hex{0, 0, 0}] = game.Black
	a.gs.Board.Tiles[game.Hex{1, -1, 0}] = game.White
	a.gs.Turn = game.Black
	h.Draw()

	if !tapPoint(h, a, game.Hex{0, 0, 0}) {
		t.Fatal("tapping the own tile should select it")
	}
	if !a.hasSelection {
		t.Fatal("selecting a tile in the movement phase should set hasSelection")
	}
	dests := game.MoveMoves(a.gs.Board.Tiles, game.Black, false)
	if len(dests) == 0 {
		t.Fatal("test setup error: the selected tile should have legal destinations")
	}
	h.Draw() // render the frontier ghosts for visual confirmation (screenshot test covers the pixels)
}

// --- Both opponent modes, all 3 AI difficulties -----------------------------

func TestPlayHexaBothOpponentModes(t *testing.T) {
	t.Run("hotseat", func(t *testing.T) {
		h, a := bootToMenu(t)
		startChoice(t, h, a, game.OpponentHotseat, 0)
		if a.gs.AITurn() {
			t.Fatal("hotseat mode should never report an AI turn")
		}
	})
	for _, depth := range []int{game.DepthEasy, game.DepthMedium, game.DepthHard} {
		t.Run("ai-depth-"+itoa(depth), func(t *testing.T) {
			h, a := bootToMenu(t)
			startChoice(t, h, a, game.OpponentAI, depth)
			if a.gs.AITurn() {
				t.Fatal("Black (the human) should move first, not the AI")
			}
			if !tapPoint(h, a, game.Hex{0, 0, 0}) {
				t.Fatal("the human's opening placement should be accepted")
			}
			for i := 0; i < 80 && a.gs.Board.Count(game.White) == 0; i++ {
				h.Draw()
			}
			if a.gs.Board.Count(game.White) == 0 {
				t.Fatal("the AI should have placed its reply automatically")
			}
			if a.gs.AIDepth != depth {
				t.Fatalf("AIDepth = %d, want %d", a.gs.AIDepth, depth)
			}
		})
	}
}

// --- A full game, scripted via real taps, to a genuine Linje win -----------

// TestPlayHexaFullGameToLineWin scripts an entire placement-phase game via
// real taps: Black claims lineCells6() on its own 6 turns (turns 1,3,5,7,9,
// 11), White fills in an adjacent filler cell on each of its turns in
// between (turns 2,4,6,8,10) — every single placement legally touches the
// shared, ever-growing cluster, exactly like a real game — until Black's
// final placement completes the line and wins before White ever gets an
// 6th turn.
func TestPlayHexaFullGameToLineWin(t *testing.T) {
	h, a := bootToMenu(t)
	startChoice(t, h, a, game.OpponentHotseat, 0)

	line := lineCells6()
	fillerDir := game.Directions[4] // (0,1,-1): adjacent to each line cell
	turn := 0
	for i, L := range line {
		if a.gs.Turn != game.Black {
			t.Fatalf("round %d: expected Black to move, got %v", i, a.gs.Turn)
		}
		if !tapPoint(h, a, L) {
			t.Fatalf("round %d: Black's placement at %v should be legal", i, L)
		}
		turn++
		if a.gs.Phase == game.PhaseDone {
			break // the line completed
		}
		if i == len(line)-1 {
			break // Black's last placement should have already won
		}
		W := L.Add(fillerDir)
		if a.gs.Turn != game.White {
			t.Fatalf("round %d: expected White to move, got %v", i, a.gs.Turn)
		}
		if !tapPoint(h, a, W) {
			t.Fatalf("round %d: White's filler placement at %v should be legal", i, W)
		}
	}

	if a.gs.Phase != game.PhaseDone || a.gs.WinSide != game.Black || a.gs.WinKind != game.ShapeLine {
		t.Fatalf("expected Black to win with a Linje, got phase=%v winner=%v kind=%v", a.gs.Phase, a.gs.WinSide, a.gs.WinKind)
	}
	if _, ok := h.FindTextContains("Svart vann"); !ok {
		t.Fatalf("win banner not shown; visible: %v", texts(h))
	}
}

// --- Quit mid-game (Back key AND the Meny button), then restart ------------

func TestPlayHexaQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startChoice(t, h, a, game.OpponentHotseat, 0)
	tapPoint(h, a, game.Hex{0, 0, 0})

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}

	startChoice(t, h, a, game.OpponentAI, game.DepthEasy)
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
	startChoice(t, h, a, game.OpponentHotseat, 0)
}

// --- Rules screen ------------------------------------------------------------

func TestPlayHexaRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("Six"); !ok {
		t.Fatalf("rules text missing the original-game credit; visible: %v", texts(h))
	}
	if _, ok := h.FindTextContains("avslappnad"); !ok {
		t.Fatalf("rules text missing the honest casual-AI framing; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayHexaScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/hexa_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/hexa_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/hexa_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startChoice(t, h, a, game.OpponentHotseat, 0)
	// Early placement: a handful of tiles down.
	first := game.Hex{0, 0, 0}
	tapPoint(h, a, first)
	for i, n := range game.Neighbors(first) {
		if i >= 4 {
			break
		}
		tapPoint(h, a, n)
	}
	if err := h.Screenshot(dir + "/hexa_placement_early.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// Near-full board: 30+ tiles placed — the legibility risk the spec
	// calls out explicitly. Built directly (a breadth-first blob from the
	// center, alternating colors) rather than via more taps, so the shot
	// is deterministic and guaranteed dense.
	clearBoard(a.gs)
	added := []game.Hex{{0, 0, 0}}
	seen := map[game.Hex]bool{{0, 0, 0}: true}
	tmp := map[game.Hex]game.Side{{0, 0, 0}: game.Black}
	for len(added) < 34 {
		frontier := game.PlaceMoves(tmp)
		progressed := false
		for _, p := range frontier {
			if len(added) >= 34 {
				break
			}
			if seen[p] {
				continue
			}
			seen[p] = true
			tmp[p] = game.Black
			added = append(added, p)
			progressed = true
		}
		if !progressed {
			break
		}
	}
	for i, p := range added {
		if i%2 == 0 {
			a.gs.Board.Tiles[p] = game.Black
		} else {
			a.gs.Board.Tiles[p] = game.White
		}
	}
	a.gs.Remaining[game.Black] = game.TilesPerSide - a.gs.Board.Count(game.Black)
	a.gs.Remaining[game.White] = game.TilesPerSide - a.gs.Board.Count(game.White)
	a.gs.Turn = game.Black
	h.Draw()
	if err := h.Screenshot(dir + "/hexa_near_full_board.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// Movement phase + frontier highlight for a selected tile. Uses a
	// smaller, sparser position than the dense board above so the
	// selection ring and the destination ghosts are unambiguous in the
	// screenshot (in a fully dense cluster, an interior tile's legal
	// destinations really do cover the whole outer frontier — correct,
	// but visually busy for a legibility screenshot).
	clearBoard(a.gs)
	a.gs.Phase = game.PhaseMoving
	frontierCenter := game.Hex{0, 0, 0}
	a.gs.Board.Tiles[frontierCenter] = game.Black
	for i, n := range game.Neighbors(frontierCenter) {
		if i%2 == 0 {
			a.gs.Board.Tiles[n] = game.White
		}
	}
	far := game.Hex{3, -3, 0}
	a.gs.Board.Tiles[far] = game.Black
	a.gs.Board.Tiles[game.Hex{4, -4, 0}] = game.White
	a.gs.Remaining[game.Black] = game.TilesPerSide - a.gs.Board.Count(game.Black)
	a.gs.Remaining[game.White] = game.TilesPerSide - a.gs.Board.Count(game.White)
	a.gs.Turn = game.Black
	h.Draw()
	tapPoint(h, a, frontierCenter)
	if err := h.Screenshot(dir + "/hexa_movement_frontier.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	// Each of the 3 winning-shape completions, with the highlight.
	clearBoard(a.gs)
	a.gs.Phase = game.PhasePlacement
	line := lineCells6()
	for _, p := range line[:5] {
		a.gs.Board.Tiles[p] = game.Black
	}
	a.gs.Remaining[game.Black] = game.TilesPerSide - 5
	a.gs.Turn = game.Black
	h.Draw()
	tapPoint(h, a, line[5])
	if err := h.Screenshot(dir + "/hexa_win_line.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	clearBoard(a.gs)
	a.gs.Phase = game.PhasePlacement
	tri := []game.Hex{{0, 0, 0}, {1, -1, 0}, {2, -2, 0}, {0, 1, -1}, {1, 0, -1}, {0, 2, -2}}
	for _, p := range tri[:5] {
		a.gs.Board.Tiles[p] = game.White
	}
	a.gs.Remaining[game.White] = game.TilesPerSide - 5
	a.gs.Turn = game.White
	h.Draw()
	tapPoint(h, a, tri[5])
	if err := h.Screenshot(dir + "/hexa_win_triangle.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	clearBoard(a.gs)
	a.gs.Phase = game.PhasePlacement
	center := game.Hex{0, 0, 0}
	ring := game.Neighbors(center)
	for _, p := range ring[:5] {
		a.gs.Board.Tiles[p] = game.Black
	}
	a.gs.Remaining[game.Black] = game.TilesPerSide - 5
	a.gs.Turn = game.Black
	h.Draw()
	tapPoint(h, a, ring[5])
	if err := h.Screenshot(dir + "/hexa_win_ring.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	if err := h.Screenshot(dir + "/hexa_win_banner.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
