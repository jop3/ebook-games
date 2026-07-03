// Command jotto is a Swedish word-guessing game (Jotto / Wordle style) for the
// PocketBook Verse Pro (PB634), built on the dennwc/inkview SDK.
//
// The computer picks a secret 5-letter Swedish word. The player types 5-letter
// guesses on an on-screen keyboard; each guess must be a real dictionary word.
// Per-letter feedback is shown as e-ink symbols (greyscale, not colour):
//   - right letter, right place -> filled black tile
//   - right letter, wrong place -> hollow ringed tile
//   - letter not in the word     -> light/empty tile
// Duplicate letters follow Wordle rules. Six guesses are allowed.
//
// Pure game logic (dictionary, secret selection, Evaluate, guess state) lives in
// the jotto/game package with no SDK dependency and is unit-tested; this file
// and ui.go handle rendering and input.
package main

import (
	"image"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	ink "github.com/dennwc/inkview"

	"jotto/game"
)

type screen int

const (
	screenSplash screen = iota // shown on launch; tap -> menu
	screenMenu
	screenGame
	screenRules
)

type app struct {
	fonts  *Fonts
	screen screen
	menu   *Menu
	rng    *rand.Rand

	gs      *game.GameState
	layout  Layout
	keys    []Key
	buttons []Button
	updates int
	msg     string // transient status message (e.g. "Inte ett ord")

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
	a.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
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
		DrawSplash(screenSize, a.fonts, "Jotto", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Jotto", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawGrid(&a.layout, a.gs, a.fonts)
	a.keys = DrawKeyboard(&a.layout, a.gs, a.fonts)
	a.buttons = DrawButtonBar(&a.layout, a.buttonLabels(), a.fonts)

	if a.gs.Over || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) buttonLabels() []string {
	if a.gs.Over {
		return []string{"Nytt spel", "Meny"}
	}
	return []string{"Gissa", "Sudda", "Meny"}
}

func (a *app) statusText() string {
	s := a.gs
	if a.msg != "" {
		return a.msg
	}
	if s.Won {
		return "Rätt! Löst på " + itoa(len(s.Guesses)) + " gissningar"
	}
	if s.Over {
		return "Facit: " + upperStr(s.Secret())
	}
	return "Gissning " + itoa(len(s.Guesses)+1) + " av " + itoa(game.MaxGuesses)
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
	if p.In(a.menu.PlayButton()) {
		a.startGame()
		return true
	}
	return false
}

func (a *app) startGame() {
	a.gs = game.NewGame(a.rng)
	a.screen = screenGame
	a.updates = 0
	a.msg = ""
	ink.Repaint()
}

func (a *app) tapGame(p image.Point) bool {
	for _, b := range a.buttons {
		if b.Hit(p) {
			return a.handleButton(b.Label)
		}
	}
	if a.gs.Over {
		return false
	}
	for _, k := range a.keys {
		if k.Hit(p) {
			if a.gs.AppendLetter(k.Letter) {
				a.msg = ""
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
		switch a.gs.Submit() {
		case game.SubmitOK:
			a.msg = ""
			a.updates = 0 // full redraw so the grid row settles cleanly
			ink.Repaint()
		case game.SubmitIncomplete:
			a.msg = "Fyll i 5 bokstäver"
			ink.Repaint()
		case game.SubmitNotWord:
			a.msg = "Inte ett giltigt ord"
			ink.Repaint()
		}
		return true
	case "Sudda":
		if a.gs.Backspace() {
			a.msg = ""
			a.updates++
			ink.Repaint()
		}
		return true
	case "Nytt spel":
		a.startGame()
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
