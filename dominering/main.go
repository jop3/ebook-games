// Command dominering is Domineering (Conway) for the PocketBook Verse Pro
// (PB634), built on the dennwc/inkview SDK.
//
// Two players share a rectangular board (8x8, or a smaller 6x6 "Lätt"
// option) of empty cells. Player V may only place a 1x2 domino VERTICALLY;
// player H may only place one HORIZONTALLY. Turns alternate — V always
// starts. Whoever cannot legally place their domino on their turn loses
// (normal play convention). Play hot-seat against a friend, or against a
// built-in alpha-beta AI (human plays V, AI plays H).
//
// Pure game logic (board, legal moves, win condition, AI) lives in the
// dominering/game package with no SDK dependency and is unit-tested; this
// file and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"dominering/game"
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

	hasSelection bool
	selected     image.Point

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

// --- ink.App -----------------------------------------------------------------

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
		DrawSplash(screenSize, a.fonts, "Dominering", drawSplashMotif)
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
				if a.gs.AITurn() {
					a.aiPend = true
				}
				ink.Repaint()
			}
		}
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Dominering", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize, a.gs.Board.Size)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawBoard(&a.layout, a)
	a.buttons = DrawButtonBar(&a.layout, []string{"Ny", "Meny"}, a.fonts)

	if a.gs.Phase == game.PhaseDone || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) statusText() string {
	s := a.gs
	left := s.Board.EmptyCount()
	if s.Phase == game.PhaseDone {
		loser := sideName(s.Turn)
		winner := sideName(s.Winner())
		return loser + " kan inte lägga — " + winner + " vinner!"
	}
	turn := sideName(s.Turn)
	if s.AITurn() {
		turn += " (dator)"
	}
	return turn + " drar   ·   " + itoa(left) + " rutor kvar"
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
		a.startGame(choice.opponent, choice.aiDepth, a.menu.size)
		return true
	}
	return false
}

func (a *app) startGame(opp game.Opponent, aiDepth int, size int) {
	a.gs = game.NewGame(opp, size, aiDepth)
	a.screen = screenGame
	a.updates = 0
	a.aiPend = false
	a.hasSelection = false
	if a.gs.AITurn() {
		a.aiPend = true
	}
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
	cell := image.Pt(x, y)

	if a.hasSelection {
		if cell == a.selected {
			a.hasSelection = false // tap the selected cell again to deselect
			ink.Repaint()
			return true
		}
		for _, partner := range a.gs.Board.PartnersFrom(a.gs.Turn, a.selected) {
			if partner == cell {
				m := game.MakeMove(a.selected, cell)
				if a.gs.Play(m) {
					a.hasSelection = false
					a.updates++
					if a.gs.AITurn() {
						a.aiPend = true
					}
					ink.Repaint()
					return true
				}
				return false
			}
		}
		// Not the selected cell, not a legal partner: if it's a usable new
		// anchor (empty, with at least one legal partner), switch selection
		// to it — mirrors "tap another own man to switch" in other games.
		if a.gs.Board.Empty(x, y) && len(a.gs.Board.PartnersFrom(a.gs.Turn, cell)) > 0 {
			a.selected = cell
			ink.Repaint()
			return true
		}
		return false
	}

	if a.gs.Board.Empty(x, y) && len(a.gs.Board.PartnersFrom(a.gs.Turn, cell)) > 0 {
		a.selected = cell
		a.hasSelection = true
		ink.Repaint()
		return true
	}
	return false
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Ny":
		a.startGame(a.gs.Opponent, a.gs.AIDepth, a.gs.Board.Size)
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
