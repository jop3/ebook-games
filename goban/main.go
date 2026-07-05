// Command goban is Go (baduk/weiqi) for the PocketBook Verse Pro (PB634),
// built on the dennwc/inkview SDK. Named "goban" (the term for a Go board)
// to avoid clashing with the Go toolchain's own name.
//
// Two players place Black and White stones on a 9x9, 13x13, or 19x19 board
// to surround territory and capture enemy groups. Capture removes any group
// left without a liberty (an empty adjacent point); a simple positional ko
// rule forbids immediately recreating the board position from before the
// opponent's last move. Two consecutive passes end normal play and enter a
// mark-dead phase where players tap groups to mark them dead before the
// final area (Chinese) score is computed, with komi added to White. Play
// hot-seat against a friend, or (9x9 only) against a weak built-in AI.
//
// Pure game logic (board, groups, captures, ko, scoring, AI) lives in the
// goban/game package with no SDK dependency and is unit-tested; this file
// and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"goban/game"
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
		DrawSplash(screenSize, a.fonts, "Go", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn, compute its move AFTER this frame is shown
		// so the player sees their own move (or pass) land first, then
		// trigger a redraw.
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
		a.rulesBack = DrawRules(screenSize, a.fonts, "Go", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize, a.gs.Board.Size())
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawBoard(&a.layout, a.gs, a.fonts)
	a.buttons = DrawButtonBar(&a.layout, a.buttonLabels(), a.fonts)

	// Big captures and phase transitions change many cells at once — force a
	// FullUpdate to clear e-ink ghosting, not just the periodic refresh.
	bigChange := len(a.gs.LastCaptured) > 0 || a.gs.Phase != game.PhasePlaying
	if bigChange || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) buttonLabels() []string {
	switch a.gs.Phase {
	case game.PhaseMarking:
		return []string{"Klar", "Meny"}
	case game.PhaseDone:
		return []string{"Ny", "Meny"}
	default:
		return []string{"Passa", "Ny", "Meny"}
	}
}

func (a *app) statusText() string {
	s := a.gs
	switch s.Phase {
	case game.PhaseMarking:
		return "Markera döda grupper — tryck Klar när alla är markerade"
	case game.PhaseDone:
		score := "Svart " + ftoa1(s.BlackScore) + " – Vit " + ftoa1(s.WhiteScore)
		switch s.Winner() {
		case game.Black:
			return "Svart vinner!  " + score
		case game.White:
			return "Vit vinner!  " + score
		default:
			return "Oavgjort!  " + score
		}
	default:
		score := "Svart " + itoa(s.Board.Count(game.Black)) + " – Vit " + itoa(s.Board.Count(game.White))
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
		return pre + turn + " lägger sten   ·   " + score
	}
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
	if a.menu.TapSize(p) {
		ink.Repaint()
		return true
	}
	if a.menu.TapKomi(p) {
		ink.Repaint()
		return true
	}
	if choice, ok := a.menu.HandleTouch(p); ok {
		a.startGame(a.menu.size, choice.opponent, a.menu.komi)
		return true
	}
	return false
}

func (a *app) startGame(size int, opp game.Opponent, komi float64) {
	a.gs = game.NewGame(size, opp, komi)
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
	switch a.gs.Phase {
	case game.PhasePlaying:
		if a.gs.AITurn() {
			return false
		}
		pt, ok := a.layout.ScreenToPoint(p)
		if !ok {
			return false
		}
		if a.gs.Play(pt.X, pt.Y) {
			a.updates++
			if a.gs.AITurn() {
				a.aiPend = true
			}
			ink.Repaint()
			return true
		}
		return false
	case game.PhaseMarking:
		pt, ok := a.layout.ScreenToPoint(p)
		if !ok {
			return false
		}
		if a.gs.ToggleDead(pt.X, pt.Y) {
			ink.Repaint()
			return true
		}
	}
	return false
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Passa":
		if a.gs.Phase != game.PhasePlaying || a.gs.AITurn() {
			return false
		}
		if a.gs.Pass() {
			a.updates++
			if a.gs.AITurn() {
				a.aiPend = true
			}
			ink.Repaint()
			return true
		}
		return false
	case "Klar":
		a.gs.FinishMarking()
		a.updates++
		ink.Repaint()
		return true
	case "Ny":
		a.startGame(a.gs.Board.Size(), a.gs.Opponent, a.gs.Komi)
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
