// Command stadskarnan ("Stadskärnan" / The Town Core) is a territory-
// enclosure building game for the PocketBook Verse Pro (PB634), built on the
// dennwc/inkview SDK.
//
// Two players face off on a 10x10 board. A single neutral cross-shaped
// Cathedral piece goes down first (placed by Black); players then alternate
// placing their own secular "building" pieces (any rotation/reflection) on
// empty cells. Fully wall off a region with your own color (and/or the
// Cathedral) without touching the board edge and any of the opponent's
// pieces caught inside are captured, returned to that opponent's hand to
// place again later; an empty pocket sealed the same way is simply blocked
// off forever. The game ends once neither side can place any remaining
// piece; whoever has fewest total unplaced squares left in hand wins. Play
// hot-seat against a friend, or against a heuristic practice-strength AI.
//
// Pure game logic (board, pieces, placement legality, enclosure/capture, win
// condition, AI) lives in the stadskarnan/game package with no SDK
// dependency and is unit-tested; this file and ui.go handle rendering and
// input.
//
// This is our own original port of the enclosure/capture MECHANIC found in
// Cathedral (Gamewright/Playroom Entertainment) — the piece geometry, names
// and art are original to this implementation (see game/pieces.go).
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"stadskarnan/game"
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

	selectedPiece int // -1 = none selected
	orientIdx     int

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
	a.selectedPiece = -1
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
		// The launcher/view.json app title (outside this repo) should stay
		// ASCII-safe per the gamedev guide; the in-app splash/menu/rules text
		// is free to use å/ä/ö, so we use the full Swedish name here.
		DrawSplash(screenSize, a.fonts, "Stadskärnan", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn, compute its move AFTER this frame is shown
		// so the player sees their own move (and any capture) land first,
		// then trigger a redraw.
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
		a.rulesBack = DrawRules(screenSize, a.fonts, "Stadskärnan", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawBoard(&a.layout, a)
	if a.gs.Phase == game.PhasePlaying {
		DrawTray(&a.layout, a)
	}
	a.buttons = DrawButtonBar(&a.layout, a.buttonLabels(), a.fonts)

	if a.gs.Phase == game.PhaseDone || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) buttonLabels() []string {
	if a.gs.Phase == game.PhasePlaying && a.selectedPiece >= 0 {
		return []string{"Rotera", "Ny", "Meny"}
	}
	return []string{"Ny", "Meny"}
}

func (a *app) statusText() string {
	s := a.gs
	rb, rw := s.Hand(game.Black).RemainingSquares(), s.Hand(game.White).RemainingSquares()
	score := "Svart " + itoa(rb) + " – Vit " + itoa(rw) + " kvar"
	switch s.Phase {
	case game.PhaseCathedral:
		return "Svart placerar Katedralen"
	case game.PhaseDone:
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
	return turn + " bygger   ·   " + score
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
	if opp, ok := a.menu.HandleTouch(p); ok {
		a.startGame(opp)
		return true
	}
	return false
}

func (a *app) startGame(opp game.Opponent) {
	a.gs = game.NewGame(opp)
	a.screen = screenGame
	a.updates = 0
	a.aiPend = false
	a.selectedPiece = -1
	a.orientIdx = 0
	ink.Repaint()
}

func (a *app) tapGame(p image.Point) bool {
	for _, b := range a.buttons {
		if b.Hit(p) {
			return a.handleButton(b.Label)
		}
	}

	if a.gs.Phase == game.PhaseCathedral {
		x, y, ok := a.layout.ScreenToCell(p)
		if !ok {
			return false
		}
		if a.gs.PlaceCathedral(image.Pt(x, y)) {
			a.updates++
			if a.gs.AITurn() {
				a.aiPend = true
			}
			ink.Repaint()
			return true
		}
		return false
	}

	if a.gs.Phase != game.PhasePlaying || a.gs.AITurn() {
		return false
	}

	hand := a.gs.Hand(a.gs.Turn)
	for id, r := range a.layout.TrayRects {
		if !hand[id] {
			continue
		}
		if p.In(r) {
			if a.selectedPiece == id {
				a.selectedPiece = -1 // tap the selected piece again to deselect
			} else {
				a.selectedPiece = id
				a.orientIdx = 0
			}
			ink.Repaint()
			return true
		}
	}

	if a.selectedPiece < 0 {
		return false
	}
	x, y, ok := a.layout.ScreenToCell(p)
	if !ok {
		return false
	}
	if a.gs.Place(a.gs.Turn, a.selectedPiece, a.orientIdx, image.Pt(x, y)) {
		a.selectedPiece = -1
		a.orientIdx = 0
		a.updates++
		if a.gs.AITurn() {
			a.aiPend = true
		}
		ink.Repaint()
		return true
	}
	return false
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Rotera":
		if a.selectedPiece >= 0 {
			n := len(game.Orientations(game.Pieces[a.selectedPiece].Cells))
			a.orientIdx = (a.orientIdx + 1) % n
			ink.Repaint()
		}
		return true
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
