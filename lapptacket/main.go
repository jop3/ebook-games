// Command lapptacket is Lapptäcket ("The Quilt"), a 2-player quilt-building
// economy race for the PocketBook Verse Pro (PB634), built on the
// dennwc/inkview SDK. Loosely based on Patchwork (Uwe Rosenberg) — this
// port uses an original, invented roster of 33 patches (shapes, costs, time
// costs, and button-income values), not the real game's exact tile list.
//
// Both players race to fill their own 9x9 quilt board by buying polyomino
// patches off a shared queue (rendered as a horizontal strip, not an actual
// circle — much easier to tap-hit-test and render on e-ink) using buttons
// and time. Turn order is NOT strict alternation: whichever player's marker
// on the linear 0-53 time track is furthest behind always acts next. Play
// hot-seat against a friend or against a built-in greedy-heuristic AI (the
// game is perfect information — nothing is hidden from either player).
//
// Pure game logic (the patch queue/neutral token, turn order, the 9x9
// board + polyomino placement, income/bonus crossing detection, scoring,
// AI) lives in the lapptacket/game package with no SDK dependency and is
// unit-tested; this file and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"lapptacket/game"
)

type screen int

const (
	screenSplash screen = iota
	screenMenu
	screenGame
	screenRules
)

type app struct {
	fonts  *Fonts
	screen screen
	menu   *Menu

	gs      *game.GameState
	layout  Layout
	buttons []Button
	updates int
	aiPend  bool // an AI move is queued to run on the next Draw

	selOffset int // -1 = no patch selected, else 0..2 (index into NextThree)
	orientIdx int
	showOpp   bool // "Visa motståndare": show the OTHER board full-size, read-only
	msg       string

	rulesBack image.Rectangle
}

func main() {
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}
	if err := ink.Run(&app{}); err != nil {
		panic(err)
	}
}

// --- ink.App -----------------------------------------------------------

func (a *app) Init() error {
	a.fonts = InitFonts()
	a.menu = NewMenu()
	a.screen = screenSplash
	a.selOffset = -1
	ink.Repaint()
	return nil
}

func (a *app) Close() error {
	if a.fonts != nil {
		a.fonts.Close()
	}
	return nil
}

const fullUpdateEvery = 6

func (a *app) Draw() {
	screenSize := ink.ScreenSize()
	switch a.screen {
	case screenSplash:
		DrawSplash(screenSize, a.fonts, "Lapptacket", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn, compute its move AFTER this frame is shown
		// so the player sees their own move land first, then trigger a
		// redraw (same pattern as this repo's other AI games).
		if a.aiPend {
			a.aiPend = false
			if a.gs.StepAI() {
				a.updates++
				if a.gs.AITurn() {
					a.aiPend = true
				}
				ink.Repaint()
			}
		}
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Lapptacket", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()

	DrawStatus(&a.layout, a.gs, a.fonts, a.msg)
	DrawTimeTrack(&a.layout, a.gs, a.fonts)
	DrawPatchStrip(&a.layout, a.gs, a.selOffset, a.orientIdx, a.fonts)

	fullSide := a.fullBoardSide()
	compactSide := 1 - fullSide
	DrawActiveBoard(&a.layout, a.gs, fullSide, a.legalAnchors(fullSide))
	DrawOpponentStrip(&a.layout, a.gs, compactSide, a.showOpp, a.fonts)

	a.buttons = DrawButtonBar(&a.layout, []string{"Rotera", "Avancera", "Ny", "Meny"}, a.fonts)

	if a.gs.Phase == game.PhaseDone || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

// fullBoardSide is whichever board is currently shown full-size: normally
// the player who must act next, or — while "Visa motståndare" is toggled
// on — the other one (read-only).
func (a *app) fullBoardSide() int {
	active := a.gs.ActingPlayer()
	if a.showOpp {
		return 1 - active
	}
	return active
}

// legalAnchors returns, for the board currently shown full-size and
// interactive, the set of cells a tap would legally act on right now: every
// empty cell if a free patch is owed, or every legal anchor of the
// currently selected patch+orientation, or nil if nothing is selected /
// it's read-only / it's the AI's turn.
func (a *app) legalAnchors(side int) map[image.Point]bool {
	if a.showOpp || a.gs.AITurn() || a.gs.Phase != game.PhasePlaying {
		return nil
	}
	if a.gs.Pending[side] > 0 {
		out := map[image.Point]bool{}
		b := &a.gs.Boards[side]
		for y := 0; y < game.BoardSize; y++ {
			for x := 0; x < game.BoardSize; x++ {
				if !b.Filled[y][x] {
					out[image.Pt(x, y)] = true
				}
			}
		}
		return out
	}
	if a.selOffset < 0 {
		return nil
	}
	three := a.gs.NextThree()
	if a.selOffset >= len(three) {
		return nil
	}
	patch := three[a.selOffset]
	placements := game.LegalPlacementsForOrientation(&a.gs.Boards[side], patch, a.orientIdx)
	out := map[image.Point]bool{}
	for _, p := range placements {
		out[p.Anchor] = true
	}
	return out
}

func (a *app) Key(e ink.KeyEvent) bool {
	if e.State == ink.KeyStateUp && e.Key == ink.KeyBack {
		if a.screen == screenGame || a.screen == screenRules {
			a.screen = screenMenu
			ink.Repaint()
			return true
		}
	}
	return false
}

func (a *app) Pointer(e ink.PointerEvent) bool {
	if e.State != ink.PointerUp {
		return false
	}
	return a.handleTap(e.Point)
}

func (a *app) Touch(e ink.TouchEvent) bool {
	if e.State != ink.TouchUp {
		return false
	}
	return a.handleTap(e.Point)
}

func (a *app) handleTap(p image.Point) bool {
	switch a.screen {
	case screenSplash:
		a.screen = screenMenu
		ink.Repaint()
		return true
	case screenMenu:
		return a.tapMenu(p)
	case screenGame:
		return a.tapGame(p)
	case screenRules:
		if p.In(a.rulesBack) {
			a.screen = screenMenu
			ink.Repaint()
			return true
		}
	}
	return false
}

func (a *app) tapMenu(p image.Point) bool {
	if p.In(a.menu.RulesButton()) {
		a.screen = screenRules
		ink.Repaint()
		return true
	}
	if choice, ok := a.menu.HandleTouch(p); ok {
		a.startGame(choice.opponent)
		return true
	}
	return false
}

func (a *app) startGame(opp game.Opponent) {
	a.gs = game.NewGame(opp)
	a.screen = screenGame
	a.updates = 0
	a.aiPend = false
	a.selOffset = -1
	a.orientIdx = 0
	a.showOpp = false
	a.msg = ""
	ink.Repaint()
}

func (a *app) tapGame(p image.Point) bool {
	for _, b := range a.buttons {
		if b.Hit(p) {
			return a.handleButton(b.Label)
		}
	}
	if p.In(a.layout.ShowOppBtn) {
		a.showOpp = !a.showOpp
		a.selOffset = -1
		ink.Repaint()
		return true
	}
	if a.gs.Phase != game.PhasePlaying || a.showOpp || a.gs.AITurn() {
		return false
	}

	active := a.gs.ActingPlayer()

	// A free patch is owed: any tap on an empty board cell places it.
	if a.gs.Pending[active] > 0 {
		x, y, ok := a.layout.ScreenToCell(p)
		if !ok {
			return false
		}
		if a.gs.PlaceFreePatch(active, image.Pt(x, y)) {
			a.msg = ""
			a.updates++
			if a.gs.AITurn() {
				a.aiPend = true
			}
			ink.Repaint()
			return true
		}
		return false
	}

	// Tap one of the (up to 3) buyable patch tiles to select/deselect it.
	for i, r := range a.layout.PatchRects {
		if i >= patchTilesBuyable {
			break
		}
		if p.In(r) {
			if a.selOffset == i {
				a.selOffset = -1
			} else {
				a.selOffset = i
				a.orientIdx = 0
			}
			a.msg = ""
			ink.Repaint()
			return true
		}
	}

	// Tap a board cell to commit the selected patch there.
	if a.selOffset >= 0 {
		x, y, ok := a.layout.ScreenToCell(p)
		if !ok {
			return false
		}
		if a.gs.BuyPatch(active, a.selOffset, a.orientIdx, image.Pt(x, y)) {
			a.selOffset = -1
			a.orientIdx = 0
			a.msg = ""
			a.updates++
			if a.gs.AITurn() {
				a.aiPend = true
			}
			ink.Repaint()
			return true
		}
		a.msg = "Får inte plats där"
		ink.Repaint()
		return false
	}
	return false
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Rotera":
		if a.gs.Phase == game.PhasePlaying && a.selOffset >= 0 {
			three := a.gs.NextThree()
			if a.selOffset < len(three) {
				n := len(game.Orientations(three[a.selOffset].Cells))
				a.orientIdx = (a.orientIdx + 1) % n
				ink.Repaint()
			}
		}
		return true
	case "Avancera":
		if a.gs.Phase != game.PhasePlaying || a.showOpp || a.gs.AITurn() {
			return true
		}
		active := a.gs.ActingPlayer()
		if a.gs.Pending[active] > 0 {
			return true
		}
		if a.gs.Advance(active) {
			a.selOffset = -1
			a.orientIdx = 0
			a.msg = ""
			a.updates++
			if a.gs.AITurn() {
				a.aiPend = true
			}
			ink.Repaint()
		}
		return true
	case "Ny":
		a.startGame(a.gs.Opponent)
		return true
	case "Meny":
		a.screen = screenMenu
		ink.Repaint()
		return true
	}
	return false
}

func (a *app) Orientation(o ink.Orientation) bool {
	ink.Repaint()
	return true
}
