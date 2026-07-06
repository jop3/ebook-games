// Command lgame is L-spelet (Edward de Bono's L-Game) for the PocketBook
// Verse Pro (PB634), built on the dennwc/inkview SDK.
//
// Two players face off on a fixed 4x4 board. Each has one L-tetromino piece
// (placeable in any of its 8 rotation/reflection orientations) and shares two
// neutral single-cell pieces with the opponent. On your turn you MUST lift
// your own L-piece and place it somewhere new; you may then optionally
// nudge one of the neutral pieces. If, on your turn, you have no legal
// placement for your L-piece at all, you lose immediately.
//
// Pure game logic (board, the 8 orientations, legal placements, win
// condition, AI) lives in the lgame/game package with no SDK dependency and
// is unit-tested; this file and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"lgame/game"
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

	selectedOrient  int             // -1 = no orientation chosen yet this turn
	selectedNeutral *image.Point    // nil = no neutral piece chosen yet
	anchorTargets   []anchorTarget  // legal L-placement tap targets, this frame
	neutralTargets  []neutralTarget // legal neutral tap targets, this frame
	orientRects     [8]image.Rectangle

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
	a.selectedOrient = -1
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
		DrawSplash(screenSize, a.fonts, "L-spelet", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn, compute its move AFTER this frame is shown
		// so the player sees their own move land first, then trigger a
		// redraw (the aiPend-after-paint pattern).
		if a.aiPend {
			a.aiPend = false
			if a.gs.StepAI() {
				a.updates++
				a.selectedOrient = -1
				a.selectedNeutral = nil
				if a.gs.AITurn() {
					a.aiPend = true
				}
				ink.Repaint()
			}
		}
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "L-spelet", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawBoard(&a.layout, a)

	if a.gs.Phase == game.PhasePlaying && !a.gs.AITurn() {
		switch a.gs.Step {
		case game.StepPlaceL:
			a.orientRects = DrawOrientationPicker(a.layout.PickerArea, a.fonts, a.gs, a.selectedOrient)
		case game.StepNeutralOptional:
			DrawNeutralHint(a.layout.PickerArea, a.fonts, a.selectedNeutral != nil)
		}
	} else {
		a.orientRects = [8]image.Rectangle{}
	}

	labels := []string{"Ny", "Meny"}
	if a.gs.Phase == game.PhasePlaying && a.gs.Step == game.StepNeutralOptional && !a.gs.AITurn() {
		labels = []string{"Klar", "Ny", "Meny"}
	}
	a.buttons = DrawButtonBar(&a.layout, labels, a.fonts)

	if a.gs.Phase == game.PhaseDone || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) statusText() string {
	s := a.gs
	if s.Phase == game.PhaseDone {
		switch s.Winner() {
		case game.Black:
			return "Svart vann!"
		case game.White:
			return "Vit vann!"
		}
	}
	turn := "Svart"
	if s.Turn == game.White {
		turn = "Vit"
	}
	if s.AITurn() {
		return turn + " (dator) tänker …"
	}
	if s.Step == game.StepNeutralOptional {
		return turn + " drar — valfri neutral flytt"
	}
	return turn + " drar — välj vridning för din L-bit"
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
	a.selectedOrient = -1
	a.selectedNeutral = nil
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
	case game.StepPlaceL:
		return a.tapPlaceL(p)
	case game.StepNeutralOptional:
		return a.tapNeutral(p)
	}
	return false
}

// tapPlaceL handles a tap during the mandatory L-placement step: either the
// orientation picker, or (once an orientation is chosen) a legal anchor cell.
func (a *app) tapPlaceL(p image.Point) bool {
	for i, r := range a.orientRects {
		if p.In(r) {
			a.selectedOrient = i
			ink.Repaint()
			return true
		}
	}
	if a.selectedOrient < 0 {
		return false
	}
	for _, at := range a.anchorTargets {
		if p.In(at.Rect) {
			if !a.gs.PlaceL(at.Pl) {
				return false
			}
			a.selectedOrient = -1
			ink.Repaint()
			return true
		}
	}
	return false
}

// tapNeutral handles a tap during the optional neutral-move step: selecting
// one of the two neutral pieces, then a destination cell for it (or tapping
// the same piece again to deselect, or the other piece to switch selection).
func (a *app) tapNeutral(p image.Point) bool {
	x, y, ok := a.layout.ScreenToCell(p)
	if !ok {
		return false
	}
	cell := image.Pt(x, y)

	if a.selectedNeutral == nil {
		if a.gs.Board.At(x, y) == game.Neutral {
			a.selectedNeutral = &cell
			ink.Repaint()
			return true
		}
		return false
	}

	if cell == *a.selectedNeutral {
		a.selectedNeutral = nil // tap the same piece again to deselect
		ink.Repaint()
		return true
	}
	if a.gs.Board.At(x, y) == game.Neutral {
		a.selectedNeutral = &cell // switch selection to the other neutral piece
		ink.Repaint()
		return true
	}
	if a.gs.MoveNeutral(game.NeutralMove{From: *a.selectedNeutral, To: cell}) {
		a.selectedNeutral = nil
		a.finishTurnBookkeeping()
		ink.Repaint()
		return true
	}
	return false
}

// finishTurnBookkeeping runs after a neutral move (or a skip) ends the
// current side's turn: reset per-turn UI selection state and, if the game
// isn't over and it's now the AI's turn, queue its move for the next Draw.
func (a *app) finishTurnBookkeeping() {
	a.updates++
	a.selectedOrient = -1
	a.selectedNeutral = nil
	if a.gs.AITurn() {
		a.aiPend = true
	}
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Klar":
		if a.gs.SkipNeutral() {
			a.finishTurnBookkeeping()
			ink.Repaint()
			return true
		}
		return false
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
