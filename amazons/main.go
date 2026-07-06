// Command amazons is the Amazons abstract strategy game for the PocketBook
// Verse Pro (PB634), built on the dennwc/inkview SDK.
//
// Two players each have 4 queens on a 10x10 board. On your turn you move one
// of your queens like a chess queen (any distance, straight or diagonal, no
// jumping), then — from its new square — shoot an arrow (the same kind of
// move) onto another empty square, which becomes permanently burned: closed
// to every future queen move and arrow, for both sides, for the rest of the
// game. There are no captures; the board just slowly burns away until one
// side has no legal move at all and loses. Play hot-seat against a friend
// (the recommended, primary mode) or against a deliberately weak/exploratory
// territory-heuristic AI — see game/ai.go and the in-app rules screen for why
// a strong AI isn't attempted here.
//
// Pure game logic (board, queen/arrow rays, win condition, AI) lives in the
// amazons/game package with no SDK dependency and is unit-tested; this file
// and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"amazons/game"
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
	aiPend  bool // an AI half-turn is queued to run on the next Draw

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
		DrawSplash(screenSize, a.fonts, "Amazons", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn, compute its next half-action AFTER this
		// frame is shown, so the player sees the previous action land first,
		// then trigger a redraw. A full AI turn is two halves (move, then
		// shoot) played across two separate Draw frames — see GameState.StepAI.
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
		a.rulesBack = DrawRules(screenSize, a.fonts, "Amazons", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
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
	burned := "  ·  " + itoa(s.Board.CountBurned()) + " brända rutor"
	if s.Phase == game.PhaseDone {
		switch s.Winner() {
		case game.QueenBlack:
			return "Svart vinner!" + burned
		case game.QueenWhite:
			return "Vit vinner!" + burned
		default:
			return "Spelet är slut." + burned
		}
	}
	turn := "Svart"
	if s.Turn == game.White {
		turn = "Vit"
	}
	if s.AITurn() {
		turn = "Vit (dator)"
	}
	if s.Step == game.StepMove {
		return turn + " flyttar en drottning" + burned
	}
	return turn + " skjuter en pil" + burned
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
		a.startGame(choice.opponent)
		return true
	}
	return false
}

func (a *app) startGame(opp game.Opponent) {
	a.gs = game.NewGame(opp)
	a.screen = screenGame
	a.updates = 0
	a.aiPend = false
	a.hasSelection = false
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

	switch a.gs.Step {
	case game.StepMove:
		return a.tapMove(cell)
	case game.StepShoot:
		if a.gs.Shoot(cell) {
			a.updates++
			if a.gs.AITurn() {
				a.aiPend = true
			}
			ink.Repaint()
			return true
		}
		return false
	}
	return false
}

// tapMove handles a tap during the move half of a turn: select one of the
// side-to-move's own queens, then tap a highlighted destination to move it
// there (which advances the turn to the shoot half). Tapping the already-
// selected queen again deselects it; tapping a different own queen switches
// the selection.
func (a *app) tapMove(cell image.Point) bool {
	s := a.gs
	if a.hasSelection {
		if cell == a.selected {
			a.hasSelection = false
			ink.Repaint()
			return true
		}
		if s.Board.At(cell.X, cell.Y) == s.Turn.Queen() {
			a.selected = cell
			ink.Repaint()
			return true
		}
		if s.MoveQueen(a.selected, cell) {
			a.hasSelection = false
			a.updates++
			ink.Repaint()
			return true
		}
		return false
	}
	if s.Board.At(cell.X, cell.Y) == s.Turn.Queen() {
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
		a.startGame(a.gs.Opponent)
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
