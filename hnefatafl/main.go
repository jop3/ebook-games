// Command hnefatafl is Hnefatafl, in its Brandub variant (7x7 board, 8
// attackers vs. 4 defenders + a king), for the PocketBook Verse Pro (PB634),
// built on the dennwc/inkview SDK.
//
// An asymmetric Viking siege game: the king and his defenders try to break
// out to any of the board's 4 corners, while a larger attacking force tries
// to surround and capture the king first. Movement is rook-like (any
// distance, orthogonal, no jumping); capture is custodial, exactly like
// hasami, except the king has his own separate surround rule and the empty
// throne counts as hostile terrain for BOTH sides when computing a capture.
// Play hot-seat against a friend, or against a built-in minimax AI that can
// take either side.
//
// Pure game logic (board, moves, captures, win conditions, AI) lives in the
// hnefatafl/game package with no SDK dependency and is unit-tested; this file
// and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"hnefatafl/game"
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
		DrawSplash(screenSize, a.fonts, "Hnefatafl", drawSplashMotif)
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
				if a.gs.AITurn() {
					a.aiPend = true
				}
				ink.Repaint()
			}
		}
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Hnefatafl", rulesParagraphs)
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

// sideName is the Swedish name for a side.
func sideName(s game.Side) string {
	if s == game.SideAttacker {
		return "Anfallarna"
	}
	return "Försvararna"
}

// reasonText describes, in Swedish, why the game just ended.
func reasonText(r game.Reason) string {
	switch r {
	case game.ReasonKingEscaped:
		return "kungen flydde till ett hörn"
	case game.ReasonKingCaptured:
		return "kungen tillfångatagen"
	case game.ReasonNoMoves:
		return "motståndaren saknar drag"
	default:
		return ""
	}
}

func (a *app) statusText() string {
	s := a.gs
	atk := s.Board.Count(game.Attacker)
	def := s.Board.DefenderSideCount()
	score := "Anfallare " + itoa(atk) + " – Försvarare " + itoa(def)
	if s.Phase == game.PhaseDone {
		winner, reason := s.Winner()
		return sideName(winner) + " vann!  (" + reasonText(reason) + ")   " + score
	}
	turn := sideName(s.Turn)
	if a.gs.AITurn() {
		turn += " (dator)"
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
	if a.menu.TapSideToggle(p) {
		ink.Repaint()
		return true
	}
	if choice, ok := a.menu.HandleTouch(p); ok {
		a.startGame(choice.opponent, a.menu.aiSide, choice.aiDepth)
		return true
	}
	return false
}

func (a *app) startGame(opp game.Opponent, aiSide game.Side, aiDepth int) {
	a.gs = game.NewGame(opp, aiSide, aiDepth)
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
	owned := func(x, y int) bool {
		c := a.gs.Board.At(x, y)
		return c != game.Empty && game.Owner(c) == a.gs.Turn
	}

	if a.hasSelection {
		if cell == a.selected {
			a.hasSelection = false // tap the selected piece again to deselect
			ink.Repaint()
			return true
		}
		if owned(x, y) {
			a.selected = cell // switch selection to another own piece
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

	if owned(x, y) {
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
		a.startGame(a.gs.Opponent, a.gs.AISide, a.gs.AIDepth)
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
