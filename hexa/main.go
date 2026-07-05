// Command hexa is Hexa (based on "Six" by Steffen Spiele / Steffen
// Mühlhäuser) for the PocketBook Verse Pro (PB634), built on the
// dennwc/inkview SDK.
//
// Two players place hex tiles of their own color edge-to-edge into a single
// growing, always-connected mosaic, then — once all 42 tiles (21 each) are
// down — take turns sliding one of their own tiles to another empty spot on
// the cluster's edge. First to arrange 6 of their own tiles into a straight
// line, a size-3 triangle, or a ring around one central cell wins. Play
// hot-seat against a friend (the primary experience) or against a small,
// explicitly casual built-in AI.
//
// Pure game logic (hex geometry, placement/move legality, connectivity, the
// 3 winning-shape checks, AI) lives in the hexa/game package with no SDK
// dependency and is unit-tested; this file and ui.go handle rendering and
// input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"hexa/game"
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

	gs        *game.GameState
	layout    Layout
	buttons   []Button
	updates   int
	aiPend    bool // an AI action is queued to run on the next Draw
	forceFull bool // force a FullUpdate on the next drawGame (clears ghosting after a placement/move)

	hasSelection bool
	selected     game.Hex

	rulesBack image.Rectangle // back-button rect on the rules screen
}

func main() {
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}
	if err := ink.Run(&app{}); err != nil {
		panic(err)
	}
}

// --- ink.App ---------------------------------------------------------------

func (a *app) Init() error {
	a.fonts = InitFonts()
	a.menu = NewMenu()
	a.screen = screenSplash
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
		DrawSplash(screenSize, a.fonts, "Hexa", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn, compute its move AFTER this frame is shown
		// so the player sees their own move land first, then trigger a
		// redraw — this house's established aiPend-after-paint pattern.
		if a.aiPend {
			a.aiPend = false
			if a.gs.StepAI() {
				a.updates++
				a.forceFull = true // clear ghosting from the AI's placement/move
				if a.gs.AITurn() {
					a.aiPend = true
				}
				ink.Repaint()
			}
		}
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Hexa", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawBoard(&a.layout, a)
	a.buttons = DrawButtonBar(&a.layout, []string{"Ny", "Meny"}, a.fonts)

	force := a.forceFull
	a.forceFull = false
	if force || a.gs.Phase == game.PhaseDone || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) statusText() string {
	s := a.gs
	score := "Svart " + itoa(s.Board.Count(game.Black)) + "/" + itoa(game.TilesPerSide) +
		"  ·  Vit " + itoa(s.Board.Count(game.White)) + "/" + itoa(game.TilesPerSide)
	if s.Phase == game.PhaseDone {
		switch s.Winner() {
		case game.Black:
			return "Svart vann (" + s.WinKind.String() + ")!  " + score
		case game.White:
			return "Vit vann (" + s.WinKind.String() + ")!  " + score
		default:
			return "Spelet slut.  " + score
		}
	}
	turn := sideName(s.Turn)
	if s.AITurn() {
		turn += " (dator)"
	}
	phase := "placerar"
	if s.Phase == game.PhaseMoving {
		phase = "flyttar"
	}
	edge := ""
	if s.EdgeReached {
		edge = "  ·  (kant nådd)"
	}
	return turn + " " + phase + "   ·   " + score + edge
}

func sideName(s game.Side) string {
	if s == game.White {
		return "Vit"
	}
	return "Svart"
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
	if a.menu.TapAdvancedToggle(p) {
		ink.Repaint()
		return true
	}
	if choice, ok := a.menu.HandleTouch(p); ok {
		a.startGame(choice.opponent, choice.aiDepth, a.menu.advanced)
		return true
	}
	return false
}

func (a *app) startGame(opp game.Opponent, aiDepth int, advanced bool) {
	a.gs = game.NewGame(opp, aiDepth, advanced)
	a.screen = screenGame
	a.updates = 0
	a.aiPend = a.gs.AITurn()
	a.hasSelection = false
	ink.Repaint()
}

func (a *app) tapGame(p image.Point) bool {
	for _, b := range a.buttons {
		if b.Hit(p) {
			return a.handleButton(b.Label)
		}
	}
	if a.gs.Phase == game.PhaseDone || a.gs.AITurn() {
		return false
	}
	pt, ok := a.layout.PointAt(p)
	if !ok {
		return false
	}

	switch a.gs.Phase {
	case game.PhasePlacement:
		if a.gs.PlaceTile(pt) {
			a.updates++
			a.forceFull = true
			a.checkAIPend()
			ink.Repaint()
			return true
		}
		return false
	case game.PhaseMoving:
		return a.tapMove(pt)
	}
	return false
}

func (a *app) tapMove(pt game.Hex) bool {
	if a.hasSelection {
		if pt == a.selected {
			a.hasSelection = false // tap the selected tile again to deselect
			ink.Repaint()
			return true
		}
		if a.gs.Board.At(pt) == a.gs.Turn {
			a.selected = pt // switch selection to another own tile
			ink.Repaint()
			return true
		}
		if a.gs.MoveTile(a.selected, pt) {
			a.hasSelection = false
			a.updates++
			a.forceFull = true // clear ghosting from the move (and any strand)
			a.checkAIPend()
			ink.Repaint()
			return true
		}
		return false
	}
	if a.gs.Board.At(pt) == a.gs.Turn {
		a.selected = pt
		a.hasSelection = true
		ink.Repaint()
		return true
	}
	return false
}

// checkAIPend queues an AI step if it is now the AI's turn (used after
// every human action that might hand control to the AI: a placement or a
// move).
func (a *app) checkAIPend() {
	if a.gs.AITurn() {
		a.aiPend = true
	}
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Ny":
		a.startGame(a.gs.Opponent, a.gs.AIDepth, a.gs.Advanced)
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
