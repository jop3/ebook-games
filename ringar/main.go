// Command ringar is Ringar (based on YINSH, Kris Burm / the GIPF project) for
// the PocketBook Verse Pro (PB634), built on the dennwc/inkview SDK.
//
// Two players first place 5 rings each on a hexagonal 85-point board, then
// take turns moving a ring in a straight line: dropping a marker of their
// color where the ring departs and sliding it to an empty point, flipping
// every marker jumped along the way to their own color — Othello-style, but
// as the direct result of a ring's move rather than a bracketing placement.
// A row of 5 markers of one color along any of the board's 3 axes lets that
// color's owner remove the row and one of their own rings. Removing 3 rings
// wins. Play hot-seat against a friend (the primary experience) or against a
// small, explicitly casual built-in AI.
//
// Pure game logic (board geometry, ring moves, flips, row detection, ring
// removal, AI) lives in the ringar/game package with no SDK dependency and is
// unit-tested; this file and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"ringar/game"
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
	forceFull bool // force a FullUpdate on the next drawGame (clears ghosting after a flip)

	hasSelection bool
	selected     game.Point

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
		DrawSplash(screenSize, a.fonts, "Ringar", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn (placement, a move, or resolving one of its
		// own pending rows), act AFTER this frame is shown so the player
		// sees their own move land first, then trigger a redraw.
		if a.aiPend {
			a.aiPend = false
			if a.gs.StepAI() {
				a.updates++
				a.forceFull = true // clear ghosting from any flips/removals the AI just made
				if a.gs.AITurn() {
					a.aiPend = true
				}
				ink.Repaint()
			}
		}
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Ringar", rulesParagraphs)
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
	score := "Svart " + itoa(s.Removed[game.Black]) + "/3  ·  Vit " + itoa(s.Removed[game.White]) + "/3"
	if s.Phase == game.PhaseDone {
		switch s.Winner() {
		case game.Black:
			return "Svart vann!  " + score
		case game.White:
			return "Vit vann!  " + score
		}
	}
	turn := sideName(s.CurrentActor())
	if s.AITurn() {
		turn += " (dator)"
	}
	switch s.Phase {
	case game.PhasePlacement:
		return turn + " placerar   ·   " + score
	case game.PhaseRowPending:
		if s.PendingWindow == nil {
			return turn + ": välj vilka 5 som tas   ·   " + score
		}
		return turn + ": välj en ring att ta bort   ·   " + score
	default:
		return turn + " drar   ·   " + score
	}
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
	a.hasSelection = false
	a.aiPend = a.gs.AITurn() // White places/moves first, and White is the AI in OpponentAI mode
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
		if a.gs.PlaceRing(pt) {
			a.updates++
			a.checkAIPend()
			ink.Repaint()
			return true
		}
		return false
	case game.PhaseRowPending:
		return a.tapRowPending(pt)
	case game.PhasePlaying:
		return a.tapMove(pt)
	}
	return false
}

func (a *app) tapMove(pt game.Point) bool {
	if a.hasSelection {
		if pt == a.selected {
			a.hasSelection = false // tap the selected ring again to deselect
			ink.Repaint()
			return true
		}
		if a.gs.Board.Rings[pt] == a.gs.Turn {
			a.selected = pt // switch selection to another own ring
			ink.Repaint()
			return true
		}
		if a.gs.Play(a.selected, pt) {
			a.hasSelection = false
			a.updates++
			a.forceFull = true // clear ghosting from the marker drop + flips
			a.checkAIPend()
			ink.Repaint()
			return true
		}
		return false
	}
	if a.gs.Board.Rings[pt] == a.gs.Turn {
		a.selected = pt
		a.hasSelection = true
		ink.Repaint()
		return true
	}
	return false
}

func (a *app) tapRowPending(pt game.Point) bool {
	if a.gs.PendingWindow == nil {
		if a.gs.ChooseWindow(pt) {
			ink.Repaint()
			return true
		}
		return false
	}
	if a.gs.RemoveRingChoice(pt) {
		a.updates++
		a.hasSelection = false
		a.forceFull = true // clear ghosting from the removed row + ring
		a.checkAIPend()
		ink.Repaint()
		return true
	}
	return false
}

// checkAIPend queues an AI step if it is now the AI's turn (used after every
// human action that might hand control to the AI: a placement, a move, or a
// row resolution — any of which can, per the spec's own tie-break, chain
// straight into another pending row before the turn truly passes).
func (a *app) checkAIPend() {
	if a.gs.AITurn() {
		a.aiPend = true
	}
}

func (a *app) handleButton(label string) bool {
	switch label {
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
