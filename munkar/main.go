// Command munkar is Munkar, a placement/capture board game for the
// PocketBook Verse Pro (PB634), built on the dennwc/inkview SDK. Based on the
// board game Donuts (Funforge), reimplemented here with original art and a
// neutral name.
//
// The 6x6 board is built from four 3x3 tiles; every cell carries a line
// glyph (horizontal, vertical, or one of the two diagonals). Players
// alternate placing a ring of their color on an empty cell: the line glyph
// of the cell just filled dictates the row/column/diagonal the opponent must
// play on next (or anywhere, if that line is already full). Placing a ring
// also runs a custodial capture check along all 4 geometric axes through it:
// if the mover's own run (including the new ring) sits bounded immediately
// on both ends by an opponent ring, both those enemy bookends flip to the
// mover's color. Getting 5 of your own rings in a line wins outright; a full
// board with no five-in-a-row is decided by the largest orthogonally-
// connected group of rings (equal size is a draw).
//
// Pure game logic (board, direction-forcing, capture, win conditions, AI)
// lives in the munkar/game package with no SDK dependency and is
// unit-tested; this file and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"munkar/game"
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

	msg string // transient status-line hint (e.g. after a rejected tap)

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
		DrawSplash(screenSize, a.fonts, "Munkar", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn, compute its move AFTER this frame is shown
		// so the player sees their own move land first, then trigger a
		// redraw.
		if a.aiPend {
			a.aiPend = false
			if a.gs.StepAI() {
				a.updates++
				ink.Repaint()
			}
		}
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Munkar", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawBoard(&a.layout, a.gs, a.fonts)
	a.buttons = DrawButtonBar(&a.layout, []string{"Ny", "Meny"}, a.fonts)

	if a.gs.Phase == game.PhaseDone || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) statusText() string {
	s := a.gs
	if a.msg != "" {
		return a.msg
	}
	if s.Phase == game.PhaseDone {
		switch s.Winner() {
		case game.Black:
			return "Svart vinner!"
		case game.White:
			return "Vit vinner!"
		default:
			return "Oavgjort!"
		}
	}
	turn := "Svart"
	if s.Turn == game.White {
		turn = "Vit"
	}
	if s.AITurn() {
		turn = "Vit (dator)"
	}
	return turn + " placerar"
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
		a.startGame(choice.mode, choice.aiLevel)
		return true
	}
	return false
}

func (a *app) startGame(mode game.Mode, aiLevel int) {
	a.gs = game.NewGame(mode, aiLevel)
	a.screen = screenGame
	a.updates = 0
	a.aiPend = false
	a.msg = ""
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
	x, y, ok := a.layout.ScreenToCell(p)
	if !ok {
		return false
	}
	if a.gs.Play(image.Pt(x, y)) {
		a.msg = ""
		a.updates++
		if a.gs.AITurn() {
			a.aiPend = true
		}
		ink.Repaint()
		return true
	}
	// Illegal tap (occupied cell, or off the currently forced line): reject
	// it (report unhandled, same as othello/hasami), but still surface a
	// hint on the status line so a mis-tap isn't confusing.
	a.msg = "Ogiltigt drag – spela på den markerade linjen"
	ink.Repaint()
	return false
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Ny":
		a.startGame(a.gs.Mode, a.gs.AILevel)
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
