// Command hashiwokakero is the bridge-building logic puzzle (Hashiwokakero /
// Bridges) for the PocketBook Verse Pro (PB634), built on the dennwc/inkview
// SDK.
//
// A grid of numbered islands must be connected with horizontal/vertical
// bridges (at most 2 between any pair, never crossing) so every island's
// bridge count matches its number and the whole network is connected. Tap an
// island to select it, then tap a neighbour to cycle the bridge between them
// 0 -> 1 -> 2 -> 0.
//
// Pure game logic (islands, bridges, crossing/connectivity checks, solver,
// generator) lives in the hashiwokakero/game package with no SDK dependency
// and is unit-tested; this file and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"hashiwokakero/game"
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

	gs       *game.GameState
	layout   Layout
	buttons  []Button
	updates  int
	selected int
	hasSel   bool

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
		DrawSplash(screenSize, a.fonts, "Hashiwokakero", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Hashiwokakero", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize, a.gs.Puz)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawBoard(&a.layout, a.gs, a.fonts, a.selected, a.hasSel)
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
		return "Löst! Alla öar är förbundna."
	}
	if a.hasSel {
		return "Ö vald — tryck på en granne för att lägga/ta bort en bro"
	}
	return "Tryck på en ö, sedan på en granne för att bygga en bro"
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
	a.hasSel = false
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
	idx, ok := HitIsland(&a.layout, a.gs.Puz, p)
	if !ok {
		a.hasSel = false
		ink.Repaint()
		return true
	}
	if !a.hasSel {
		a.selected = idx
		a.hasSel = true
		ink.Repaint()
		return true
	}
	if idx == a.selected {
		a.hasSel = false
		ink.Repaint()
		return true
	}
	if a.gs.Cycle(a.selected, idx) {
		a.updates++
	}
	a.hasSel = false
	ink.Repaint()
	return true
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Nytt pussel":
		a.startGame(a.gs.Cfg)
		return true
	case "Rensa":
		a.gs.Reset()
		a.updates = 0
		a.hasSel = false
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
