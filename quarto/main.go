// Command quarto is the Quarto! strategy game for the PocketBook Verse Pro
// (PB634), built on the dennwc/inkview SDK.
//
// Two players share one pool of 16 pieces on a 4x4 board. On your turn you
// place the piece your opponent handed you, then choose a piece from the
// pool and hand it to your opponent. Whoever completes a row, column, or
// diagonal of four pieces that share an attribute wins. Play hot-seat
// against a friend or against a built-in minimax AI.
//
// Pure game logic (board, win detection, turn state, AI) lives in the
// quarto/game package with no SDK dependency and is unit-tested; this file
// and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"quarto/game"
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
	poolBtn []PoolButton
	updates int
	aiPend  bool // an AI action is queued to run on the next Draw

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
		DrawSplash(screenSize, a.fonts, "Quarto!", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn, compute its action AFTER this frame is shown
		// so the player sees their own move land first, then trigger a redraw.
		if a.aiPend {
			a.aiPend = false
			if a.gs.StepAI() {
				a.updates++
				if a.gs.AITurn() {
					// AI has another action to take (place, then give).
					a.aiPend = true
				}
				ink.Repaint()
			}
		}
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Quarto!", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawBoard(&a.layout, a.gs, a.fonts)
	a.poolBtn = DrawPool(&a.layout, a.gs, a.fonts)
	a.buttons = DrawButtonBar(&a.layout, a.buttonLabels(), a.fonts)

	if a.gs.Phase != game.PhasePlaying || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) buttonLabels() []string {
	if a.gs.Phase != game.PhasePlaying {
		return []string{"Spela igen", "Meny"}
	}
	return []string{"Meny"}
}

func playerLabel(n int, ai bool) string {
	if n == 0 {
		return "Spelare 1"
	}
	if ai {
		return "Datorn"
	}
	return "Spelare 2"
}

func (a *app) statusText() string {
	s := a.gs
	aiP1 := s.Mode == game.ModeAI
	if s.Phase != game.PhasePlaying {
		switch s.Phase {
		case game.PhaseWon:
			return playerLabel(s.Winner(), aiP1 && s.Winner() == 1) + " vinner!"
		default:
			return "Oavgjort!"
		}
	}
	who := playerLabel(s.Turn, aiP1 && s.Turn == 1)
	if s.Step == game.StepPlace {
		return who + " placerar den tilldelade brickan"
	}
	return who + " väljer en bricka att ge bort"
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
	switch a.gs.Step {
	case game.StepPlace:
		if x, y, ok := a.layout.ScreenToCell(p); ok {
			if a.gs.PlacePiece(x, y) {
				a.updates++
				if a.gs.AITurn() {
					a.aiPend = true
				}
				ink.Repaint()
				return true
			}
		}
	case game.StepGive:
		for _, pb := range a.poolBtn {
			if pb.Hit(p) {
				if a.gs.GivePiece(pb.Piece) {
					a.updates++
					if a.gs.AITurn() {
						a.aiPend = true
					}
					ink.Repaint()
					return true
				}
			}
		}
	}
	return false
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Spela igen":
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
