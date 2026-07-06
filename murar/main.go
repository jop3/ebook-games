// Command murar is "Murar" (Swedish for "Walls" — based on Quoridor) for the
// PocketBook Verse Pro (PB634), built on the dennwc/inkview SDK.
//
// Two players race pawns across a 9x9 board: each turn, step your pawn one
// cell closer to the opposite edge (with a jump/diagonal exception when the
// opponent's pawn is in the way), or spend one of your 10 walls to slow the
// opponent down — as long as neither player is ever left with zero path to
// their own goal edge. First pawn to reach any cell of the far edge wins.
//
// Pure game logic (board, moves, walls, win conditions, AI) lives in the
// murar/game package with no SDK dependency and is unit-tested; this file
// and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"murar/game"
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

	buildMode    bool       // false = move pawn, true = build wall
	pendingWall  *game.Wall // previewed-but-not-yet-confirmed wall in build mode
	wallRejected bool       // last confirm attempt at pendingWall's spot was illegal

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
		DrawSplash(screenSize, a.fonts, "Murar", drawSplashMotif)
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
		a.rulesBack = DrawRules(screenSize, a.fonts, "Murar", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawBoard(&a.layout, a)

	var labels []string
	if a.buildMode {
		labels = []string{"Flytta pjäs", "Rotera", "Ny", "Meny"}
	} else {
		labels = []string{"Bygg mur", "Ny", "Meny"}
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
		if w, ok := s.Winner(); ok {
			if w == game.P1 {
				return "Svart vann!"
			}
			return "Vit vann!"
		}
		return "Spelet är slut"
	}
	turn := "Svart"
	if s.Turn == game.P2 {
		turn = "Vit"
	}
	if s.AITurn() {
		turn += " (dator)"
	}
	walls := "Väggar " + itoa(s.Board.WallsLeft[game.P1]) + "–" + itoa(s.Board.WallsLeft[game.P2])
	text := turn + " drar   ·   " + walls
	if a.buildMode && a.wallRejected {
		text += "   ·   Ogiltig mur!"
	}
	return text
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
	a.buildMode = false
	a.pendingWall = nil
	a.wallRejected = false
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

	if a.buildMode {
		return a.tapBuild(p)
	}
	return a.tapMove(p)
}

func (a *app) tapMove(p image.Point) bool {
	x, y, ok := a.layout.ScreenToCell(p)
	if !ok {
		return false
	}
	if a.gs.PlayMove(image.Pt(x, y)) {
		a.afterOwnMove()
		return true
	}
	return false
}

func (a *app) tapBuild(p image.Point) bool {
	gx, gy, ok := a.layout.ScreenToIntersection(p)
	if !ok {
		return false
	}

	if a.pendingWall != nil && a.pendingWall.X == gx && a.pendingWall.Y == gy {
		// Tapping the previewed spot again attempts to confirm it.
		w := *a.pendingWall
		if a.gs.PlaceWall(w) {
			a.pendingWall = nil
			a.wallRejected = false
			a.afterOwnMove()
			return true
		}
		a.wallRejected = true
		ink.Repaint()
		return true
	}

	// New (or moved) preview; keep the last-chosen orientation for
	// convenience, defaulting to horizontal the first time.
	orient := game.Horizontal
	if a.pendingWall != nil {
		orient = a.pendingWall.Orient
	}
	w := game.Wall{X: gx, Y: gy, Orient: orient}
	a.pendingWall = &w
	a.wallRejected = false
	ink.Repaint()
	return true
}

// afterOwnMove is called once a human's pawn move or wall placement has been
// applied: it queues the AI's reply (if any) to run after this frame paints.
func (a *app) afterOwnMove() {
	a.updates++
	if a.gs.AITurn() {
		a.aiPend = true
	}
	ink.Repaint()
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Bygg mur":
		a.buildMode = true
		a.pendingWall = nil
		a.wallRejected = false
		ink.Repaint()
		return true
	case "Flytta pjäs":
		a.buildMode = false
		a.pendingWall = nil
		a.wallRejected = false
		ink.Repaint()
		return true
	case "Rotera":
		if a.pendingWall != nil {
			if a.pendingWall.Orient == game.Horizontal {
				a.pendingWall.Orient = game.Vertical
			} else {
				a.pendingWall.Orient = game.Horizontal
			}
			a.wallRejected = false
			ink.Repaint()
		}
		return true
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
