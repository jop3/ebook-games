// Command othello is the classic Othello (Reversi) strategy game for the
// PocketBook Verse Pro (PB634), built on the dennwc/inkview SDK.
//
// Two players place discs on an 8x8 board; each move must bracket a line of the
// opponent's discs, which then flip to the mover's color. Play hot-seat against
// a friend or against a built-in minimax AI. The player with the most discs
// when neither side can move wins.
//
// Pure game logic (board, flips, legal moves, AI) lives in the othello/game
// package with no SDK dependency and is unit-tested; this file and ui.go handle
// rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"othello/game"
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
		DrawSplash(screenSize, a.fonts, "Othello", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn, compute its move AFTER this frame is shown so
		// the player sees their own move land first, then trigger a redraw.
		if a.aiPend {
			a.aiPend = false
			if a.gs.StepAI() {
				a.updates++
				ink.Repaint()
			}
		}
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Othello", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
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
	bl, wh := s.Board.Count(game.Black), s.Board.Count(game.White)
	score := "Svart " + itoa(bl) + " – Vit " + itoa(wh)
	if s.Phase == game.PhaseDone {
		switch s.Winner() {
		case game.Black:
			return "Svart vinner!  " + score
		case game.White:
			return "Vit vinner!  " + score
		default:
			return "Oavgjort!  " + score
		}
	}
	turn := "Svart"
	if s.Turn == game.White {
		turn = "Vit"
	}
	if s.AITurn() {
		turn = "Vit (dator)"
	}
	pre := ""
	if s.LastPass {
		pre = "Pass! "
	}
	return pre + turn + " drar   ·   " + score
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
	if x, y, ok := a.layout.ScreenToCell(p); ok {
		if a.gs.Play(x, y) {
			a.updates++
			// If the move handed the turn to the AI, queue its reply.
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
