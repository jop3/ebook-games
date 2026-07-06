// Command breakthrough is Breakthrough for the PocketBook Verse Pro (PB634),
// built on the dennwc/inkview SDK.
//
// Two players face off on an 8x6 board with 2 ranks of pawns each. A pawn
// moves one step straight forward onto an empty square (never a capture),
// or one step diagonally forward onto a square held by an enemy pawn
// (always a capture) — the reverse of chess pawns. There is no double-step,
// no en passant, no promotion. Play hot-seat against a friend or against a
// built-in alpha-beta AI. Win by getting a pawn to the opponent's back rank,
// by eliminating every opposing pawn, or if the opponent has no legal move.
//
// Pure game logic (board, moves, win conditions, AI) lives in the
// breakthrough/game package with no SDK dependency and is unit-tested; this
// file and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"breakthrough/game"
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

// --- ink.App -----------------------------------------------------------

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
		DrawSplash(screenSize, a.fonts, "Genombrott", drawSplashMotif)
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
		a.rulesBack = DrawRules(screenSize, a.fonts, "Genombrott", rulesParagraphs)
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
	bl, wh := s.Board.Count(game.Black), s.Board.Count(game.White)
	score := "Svart " + itoa(bl) + " – Vit " + itoa(wh)
	if s.Phase == game.PhaseDone {
		switch s.Winner() {
		case game.Black:
			return "Svart vann!  " + score
		case game.White:
			return "Vit vann!  " + score
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
	return turn + " drar   ·   " + score
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
		a.startGame(choice.opponent, choice.aiDepth)
		return true
	}
	return false
}

func (a *app) startGame(opp game.Opponent, aiDepth int) {
	a.gs = game.NewGame(opp, aiDepth)
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

	if a.hasSelection {
		if cell == a.selected {
			a.hasSelection = false // tap the selected pawn again to deselect
			ink.Repaint()
			return true
		}
		if a.gs.Board.At(x, y) == a.gs.Turn {
			a.selected = cell // switch selection to another own pawn
			ink.Repaint()
			return true
		}
		if a.gs.Play(a.selected, cell) {
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

	if a.gs.Board.At(x, y) == a.gs.Turn {
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
		a.startGame(a.gs.Opponent, a.gs.AIDepth)
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
