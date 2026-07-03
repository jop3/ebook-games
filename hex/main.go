// Command hex is the classic connection game Hex for the PocketBook Verse Pro
// (PB634), built on the dennwc/inkview SDK.
//
// Two players place stones on an NxN rhombus of hexagons. Black links the top
// and bottom edges; White links the left and right edges. First to connect
// their two edges wins — in Hex a draw is impossible. Play hot-seat or against
// a built-in heuristic AI.
//
// Pure game logic (board, connectivity, AI) lives in the hex/game package with
// no SDK dependency and is unit-tested; this file and ui.go handle rendering
// and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"hex/game"
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
	aiPend  bool

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
		DrawSplash(screenSize, a.fonts, "Hex", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		if a.aiPend {
			a.aiPend = false
			if a.gs.StepAI() {
				a.updates++
				ink.Repaint()
			}
		}
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Hex", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize, a.gs.Board.N)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawBoard(&a.layout, a.gs, a.fonts)
	a.buttons = DrawButtonBar(&a.layout, a.buttonLabels(), a.fonts)

	if a.gs.Phase == game.PhaseDone || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) buttonLabels() []string {
	if a.gs.Phase == game.PhaseDone {
		return []string{"Spela igen", "Meny"}
	}
	return []string{"Meny"}
}

func (a *app) statusText() string {
	s := a.gs
	if s.Phase == game.PhaseDone {
		if s.Win == game.Black {
			return "Svart vinner!  (topp–botten)"
		}
		return "Vit vinner!  (vänster–höger)"
	}
	if s.Turn == game.Black {
		return "Svart drar  ·  koppla topp–botten"
	}
	if s.AITurn() {
		return "Vit (dator) tänker …"
	}
	return "Vit drar  ·  koppla vänster–höger"
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
		a.startGame(choice.n, choice.mode)
		return true
	}
	return false
}

func (a *app) startGame(n int, mode game.Mode) {
	a.gs = game.NewGame(n, mode)
	a.screen = screenGame
	a.updates = 0
	a.aiPend = false
	ink.Repaint()
}

func (a *app) tapGame(p image.Point) bool {
	for _, b := range a.buttons {
		if b.Hit(p) {
			return a.handleButton(b.Label)
		}
	}
	if a.gs.Phase != game.PhasePlaying || a.gs.AITurn() {
		return false
	}
	if x, y, ok := a.layout.CellAt(p); ok {
		if a.gs.Play(x, y) {
			a.updates++
			if a.gs.AITurn() {
				a.aiPend = true
			}
			ink.Repaint()
			return true
		}
	}
	return false
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Spela igen":
		a.startGame(a.gs.Board.N, a.gs.Mode)
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
