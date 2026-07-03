// Command nurikabe is the sea/island painting logic puzzle (Nurikabe) for
// the PocketBook Verse Pro (PB634), built on the dennwc/inkview SDK.
//
// A grid carries numbered seed cells; the player paints every other cell as
// sea or island so each numbered island reaches exactly its size, islands
// never touch, all sea is connected, and no 2x2 block is entirely sea.
// Tapping a non-seed cell cycles it Unknown -> Sea -> Island -> Unknown.
//
// Pure game logic (validators, generator, win-check) lives in the
// nurikabe/game package with no SDK dependency and is unit-tested; this file
// and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"nurikabe/game"
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
		DrawSplash(screenSize, a.fonts, "Nurikabe", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Nurikabe", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize, a.gs.Puz)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawGrid(&a.layout, a.gs, a.fonts)
	a.buttons = DrawButtonBar(&a.layout, a.buttonLabels(), a.fonts)

	if a.gs.Done || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) buttonLabels() []string {
	if a.gs.Done {
		return []string{"Nytt pussel", "Meny"}
	}
	return []string{"Rensa", "Meny"}
}

func (a *app) statusText() string {
	if a.gs.Done {
		return "Löst! Öar och hav stämmer."
	}
	if game.IsSea2x2Violation(a.gs) {
		return "Ett 2×2-block med hav upptäckt — det är inte tillåtet"
	}
	return "Tryck: tom → hav → ö-markering → tom"
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
	if i := a.menu.HandleTouch(p); i >= 0 {
		a.startGame(game.Presets[i])
		return true
	}
	return false
}

func (a *app) startGame(p game.Preset) {
	a.gs = game.NewGame(p)
	a.screen = screenGame
	a.updates = 0
	ink.Repaint()
}

func (a *app) tapGame(p image.Point) bool {
	for _, b := range a.buttons {
		if b.Hit(p) {
			return a.handleButton(b.Label)
		}
	}
	if a.gs.Done {
		return false
	}
	if x, y, ok := a.layout.ScreenToCell(p); ok {
		if a.gs.Toggle(x, y) {
			a.updates++
			ink.Repaint()
			return true
		}
	}
	return false
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Nytt pussel":
		a.startGame(a.gs.Cfg)
		return true
	case "Rensa":
		a.gs.Reset()
		a.updates = 0
		ink.Repaint()
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
