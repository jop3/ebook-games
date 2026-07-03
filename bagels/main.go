// Command bagels is the Pico-Fermi-Bagels code-breaking game for the
// PocketBook Verse Pro (PB634), built on the dennwc/inkview SDK.
//
// The game holds a secret code of distinct digits. The player taps digits to
// build a guess and submits it; feedback is given as sorted WORDS — "Fermi"
// (right digit, right place), "Pico" (right digit, wrong place), or "Bagels"
// (no digit matches at all). The words are sorted so their order never reveals
// which position produced which. Crack the code within a limited number of
// guesses.
//
// Pure game logic (scoring, feedback, secret generation, entry state) lives in
// the bagels/game package with no SDK dependency and is unit-tested; this file
// and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"bagels/game"
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
	keys    []Key
	buttons []Button
	updates int

	rulesBack image.Rectangle
}

func newApp() *app { return &app{} }

func main() {
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}
	if err := ink.Run(newApp()); err != nil {
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
	screenSize.Y = usableH // real drawable height is ~1340, not the reported 1448 (guide §5)
	switch a.screen {
	case screenSplash:
		DrawSplash(screenSize, a.fonts, "Bagels", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Bagels", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawHistory(&a.layout, a.gs, a.fonts)
	DrawEntry(&a.layout, a.gs, a.fonts)
	a.keys = DrawKeypad(&a.layout, a.gs, a.fonts)
	a.buttons = DrawButtonBar(&a.layout, a.buttonLabels(), a.fonts)

	if a.gs.Over() || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) buttonLabels() []string {
	if a.gs.Over() {
		return []string{"Spela igen", "Meny"}
	}
	labels := []string{"Sudda", "Meny"}
	if a.gs.EntryComplete() {
		labels = []string{"Gissa", "Sudda", "Meny"}
	}
	return labels
}

func (a *app) statusText() string {
	s := a.gs
	if s.Solved {
		return "Löst på " + itoa(len(s.Guesses)) + " gissningar!"
	}
	if s.Lost {
		return "Slut! Koden var " + codeString(s.Secret)
	}
	return "Gissningar kvar: " + itoa(s.TurnsLeft()) + "   ·   " + itoa(s.Cfg.Length) + " olika siffror"
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
	if a.gs.Over() {
		return false
	}
	for _, k := range a.keys {
		if k.Hit(p) {
			if a.gs.AppendDigit(k.Digit) {
				a.updates++
				ink.Repaint()
			}
			return true
		}
	}
	return false
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Gissa":
		if a.gs.Submit() {
			a.updates = 0 // full redraw so history grows cleanly
			ink.Repaint()
		}
		return true
	case "Sudda":
		if a.gs.Backspace() {
			a.updates++
			ink.Repaint()
		}
		return true
	case "Spela igen":
		a.startGame(a.gs.Cfg)
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
