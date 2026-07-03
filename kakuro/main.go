// Command kakuro is the sum-crossword logic puzzle (Kakuro) for the
// PocketBook Verse Pro (PB634), built on the dennwc/inkview SDK.
//
// A grid of block (clue) and entry (digit 1-9) cells forms horizontal and
// vertical "runs" that must sum to their clue with no repeated digit within a
// run. Tap an entry cell to select it, then tap a digit on the keypad to set
// its value.
//
// Pure game logic (grid shapes, runs, generator, win-check) lives in the
// kakuro/game package with no SDK dependency and is unit-tested; this file
// and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"kakuro/game"
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

	selRow, selCol int
	hasSel         bool

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
		DrawSplash(screenSize, a.fonts, "Kakuro", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Kakuro", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize, a.gs.Puz)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawGrid(&a.layout, a.gs, a.fonts, a.selRow, a.selCol, a.hasSel)
	used := map[int]bool{}
	if a.hasSel {
		used = a.usedDigitsForSelection()
	}
	a.buttons = DrawKeypad(&a.layout, a.fonts, used)

	if a.gs.Done || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

// usedDigitsForSelection returns digits already present in any run the
// selected cell belongs to, so the keypad can grey them out.
func (a *app) usedDigitsForSelection() map[int]bool {
	used := map[int]bool{}
	for _, run := range a.gs.Puz.Runs {
		inRun := false
		for _, rc := range run.Cells {
			if rc[0] == a.selRow && rc[1] == a.selCol {
				inRun = true
				break
			}
		}
		if !inRun {
			continue
		}
		for _, rc := range run.Cells {
			if rc[0] == a.selRow && rc[1] == a.selCol {
				continue
			}
			if v := a.gs.Puz.Grid[rc[0]][rc[1]].Value; v != 0 {
				used[v] = true
			}
		}
	}
	return used
}

func (a *app) statusText() string {
	if a.gs.Done {
		return "Löst! Alla summor stämmer."
	}
	return "Välj en ruta, tryck sedan på en siffra"
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
	if row, col, ok := a.layout.ScreenToCell(p); ok {
		if a.gs.Puz.Grid[row][col].Kind == game.KindEntry {
			a.selRow, a.selCol = row, col
			a.hasSel = true
			ink.Repaint()
			return true
		}
	}
	return false
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Meny":
		a.screen = screenMenu
		ink.Repaint()
		return true
	case "Sudda":
		if a.hasSel {
			a.gs.SetDigit(a.selRow, a.selCol, 0)
			ink.Repaint()
		}
		return true
	}
	if len(label) == 1 && label[0] >= '1' && label[0] <= '9' {
		if a.hasSel {
			v := int(label[0] - '0')
			if a.gs.SetDigit(a.selRow, a.selCol, v) {
				a.updates++
			}
			ink.Repaint()
		}
		return true
	}
	return false
}

func (a *app) Orientation(o ink.Orientation) bool {
	ink.Repaint()
	return true
}
