// Command blackbox is the classic Black Box ray-tracing deduction game for the
// PocketBook Verse Pro (PB634), built on the dennwc/inkview SDK.
//
// The player fires rays into a hidden grid from perimeter edge points and reads
// the results (Hit "H", Reflection "R", or a numbered entry/exit pair for a
// detour), forms a hypothesis, then marks the suspected atom cells and submits.
// Score = rays fired + a configurable penalty per wrongly-guessed atom (lower
// is better).
//
// Pure game logic (grid, atoms, ray simulation, scoring) lives in the
// blackbox/game package with no SDK dependency, so it is fully unit-tested;
// this file and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"blackbox/game"
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
	updates int // partial-update counter for periodic FullUpdate (ghosting)

	rulesBack image.Rectangle
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

func (a *app) Draw() {
	screenSize := ink.ScreenSize()
	switch a.screen {
	case screenSplash:
		DrawSplash(screenSize, a.fonts, "Black Box", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Black Box", rulesParagraphs)
		ink.FullUpdate()
	}
}

// fullUpdateEvery forces a clean FullUpdate every N partial updates to clear
// the ghosting that repeated partial updates accumulate on e-ink.
const fullUpdateEvery = 6

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize, a.gs.Grid)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawGrid(&a.layout, a.gs, a.fonts)
	DrawEdgeMarkers(&a.layout, a.gs, a.fonts)
	a.buttons = DrawButtonBar(&a.layout, a.buttonLabels(), a.fonts)

	if a.gs.Phase == game.PhaseDone || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) buttonLabels() []string {
	switch a.gs.Phase {
	case game.PhaseProbing:
		return []string{"Gissa atomer", "Meny"}
	case game.PhaseGuessing:
		return []string{"Lämna in", "Meny"}
	default: // PhaseDone
		return []string{"Spela igen", "Meny"}
	}
}

func (a *app) statusText() string {
	s := a.gs
	switch s.Phase {
	case game.PhaseProbing:
		return "Strålar: " + itoa(s.RaysFired()) + "  —  tryck på en kantpunkt"
	case game.PhaseGuessing:
		return "Markera atomer: " + itoa(s.GuessCount()) + "/" + itoa(s.Cfg.Atoms)
	default:
		res := "Poäng " + itoa(s.Score) + " (" + itoa(s.RaysFired()) + " strålar"
		if s.WrongAtoms > 0 {
			res += " + " + itoa(s.WrongAtoms) + " fel"
		}
		res += ")"
		if s.Solved() {
			return "Löst! " + res
		}
		return res
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

// Pointer handles finger taps. On real PocketBook hardware touches are
// delivered as EVT_POINTER* events (to Pointer), NOT as EVT_TOUCH*, so this is
// the primary tap path; Touch below is a fallback. Both funnel into handleTap.
func (a *app) Pointer(e ink.PointerEvent) bool {
	if e.State != ink.PointerUp {
		return false
	}
	return a.handleTap(e.Point)
}

// Touch is a fallback tap path for platforms/emulators that deliver EVT_TOUCH*.
func (a *app) Touch(e ink.TouchEvent) bool {
	if e.State != ink.TouchUp {
		return false
	}
	return a.handleTap(e.Point)
}

// handleTap dispatches a tap at point p to the active screen.
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
	if i := a.menu.HandleTouch(p); i >= 0 {
		a.startGame(game.Presets[i])
		return true
	}
	return false
}

func (a *app) startGame(p game.Preset) {
	a.gs = game.NewGame(p)
	a.screen = screenGame
	a.updates = 0
	ink.Repaint()
}

func (a *app) tapGame(p image.Point) bool {
	// Button bar first.
	for _, b := range a.buttons {
		if b.Hit(p) {
			return a.handleButton(b.Label)
		}
	}

	switch a.gs.Phase {
	case game.PhaseProbing:
		if idx, ok := a.layout.EdgeAt(p); ok {
			if _, isNew := a.gs.FireAt(idx); isNew {
				a.updates++
				ink.Repaint()
			}
			return true
		}
	case game.PhaseGuessing:
		if x, y, ok := a.layout.ScreenToCell(p); ok {
			a.gs.ToggleGuess(x, y)
			a.updates++
			ink.Repaint()
			return true
		}
	}
	return false
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Gissa atomer":
		a.gs.EnterGuessing()
		a.updates = 0
		ink.Repaint()
		return true
	case "Lämna in":
		a.gs.Submit()
		a.updates = 0
		ink.Repaint()
		return true
	case "Spela igen":
		a.startGame(a.gs.Cfg)
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
