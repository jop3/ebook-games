// Command chomp is Chomp — the classic mathematical "poisoned chocolate bar"
// game — for the PocketBook Verse Pro (PB634), built on the dennwc/inkview
// SDK.
//
// A rectangular grid of cells (a chocolate bar) has its top-left cell
// poisoned. On a turn, a player eats any remaining cell; that cell and every
// remaining cell with row >= its row AND column >= its column are removed
// too. Whoever is forced to eat the poisoned cell loses. Play hot-seat
// against a friend, or against a built-in AI that exhaustively solves the
// tiny "staircase" state space by minimax — an UNBEATABLE, perfect-play
// opponent, exactly like nim's Sprague-Grundy AI.
//
// Pure game logic (board, moves, win/loss, AI) lives in the chomp/game
// package with no SDK dependency and is unit-tested; this file and ui.go
// handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"chomp/game"
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
		DrawSplash(screenSize, a.fonts, "Chomp", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn, compute its move AFTER this frame is shown
		// so the player sees their own move land first, then trigger a
		// redraw (guide §6b "aiPend"-after-paint pattern).
		if a.aiPend {
			a.aiPend = false
			if a.gs.StepAI() {
				a.updates++
				ink.Repaint()
			}
		}
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Chomp", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize, a.gs.Rows, a.gs.Cols)
	ink.ClearScreen()
	DrawStatus(&a.layout, a)
	DrawBoard(&a.layout, a.gs)
	a.buttons = DrawButtonBar(&a.layout, []string{"Ny", "Meny"}, a.fonts)

	if a.gs.Phase == game.PhaseDone || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
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
	if a.menu.TapSizeToggle(p) {
		ink.Repaint()
		return true
	}
	if choice, ok := a.menu.HandleTouch(p); ok {
		sz := game.Sizes[a.menu.SizeIdx]
		a.startGame(sz.Rows, sz.Cols, choice.opponent)
		return true
	}
	return false
}

func (a *app) startGame(rows, cols int, opp game.Opponent) {
	a.gs = game.NewGame(rows, cols, opp)
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
	r, c, ok := a.layout.ScreenToCell(p)
	if !ok {
		return false
	}
	m := game.Move{Row: r, Col: c}
	if !a.gs.Play(m) {
		return false
	}
	a.updates++
	if a.gs.AITurn() {
		a.aiPend = true
	}
	ink.Repaint()
	return true
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Ny":
		a.startGame(a.gs.Rows, a.gs.Cols, a.gs.Opponent)
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
