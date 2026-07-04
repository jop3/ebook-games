// Command mosaik is Mosaik, a simplified 2-player tile-drafting game for the
// PocketBook Verse Pro (PB634), built on the dennwc/inkview SDK. Based on
// Azul (Michael Kiesling, Next Move/Plan B Games), reimplemented here with
// original greyscale-pattern art and a neutral name.
//
// Players draft coloured tiles from 5 shared factory displays (or the
// central pool) onto their own board's pattern lines, then — once every
// factory and the pool are empty — tile completed lines onto a fixed 5x5
// wall for points. Overflowing tiles land on the floor line and cost
// points. The game ends at the end of the round in which either player
// completes a full wall row; end-game bonuses are added once, and the
// highest total wins.
//
// Pure game logic (drafting, wall-tiling, scoring, AI) lives in the
// mosaik/game package with no SDK dependency and is unit-tested; this file
// and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"mosaik/game"
)

type screen int

const (
	screenSplash screen = iota
	screenMenu
	screenGame
	screenRules
)

// selection is the UI-only "tiles in hand" state for the 2-tap drafting
// flow: tapping any tile swatch inside a factory (or any color chip in the
// center) simultaneously picks the source AND the color (each visible
// swatch already IS a specific color from a specific source, so this
// collapses the spec's "tap source -> tap color" into one deliberate tap);
// a second tap on a legal pattern line (or the floor) then commits the
// move. Tapping a different swatch before committing simply re-selects;
// nothing is applied to game.GameState until the placing tap lands on a
// legal target, which is the "confirm step" that avoids mis-taps.
type selection struct {
	active bool
	source int
	color  game.Color
}

type app struct {
	fonts  *Fonts
	screen screen
	menu   *Menu

	gs      *game.GameState
	layout  Layout
	buttons []Button
	updates int
	aiPend  bool // an AI move is queued to run on the next Draw

	sel     selection
	showOpp bool // "Visa motståndare": show the OTHER board full-size, read-only
	msg     string

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
		DrawSplash(screenSize, a.fonts, "Mosaik", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn, compute its move AFTER this frame is shown
		// so the player sees their own move land first, then trigger a
		// redraw. Only relevant while actually drafting (PhasePlaying); the
		// transitional screens (tiling/bonus/winner) always wait for a
		// human tap on "Fortsätt", in every mode.
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
		a.rulesBack = DrawRules(screenSize, a.fonts, "Mosaik", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	switch a.gs.Phase {
	case game.PhasePlaying:
		a.buttons = a.drawDrafting()
	case game.PhaseTiling:
		a.buttons = DrawTiling(&a.layout, a.gs, a.fonts)
	case game.PhaseBonus:
		a.buttons = DrawBonus(&a.layout, a.gs, a.fonts)
	case game.PhaseDone:
		a.buttons = DrawWinner(&a.layout, a.gs, a.fonts)
	}
	if a.gs.Phase != game.PhasePlaying || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) drawDrafting() []Button {
	DrawStatus(&a.layout, a.gs, a.fonts, a.msg)
	DrawFactories(&a.layout, a.gs, a.sel)
	fullSide := a.fullBoardSide()
	DrawActiveBoard(&a.layout, a.gs, fullSide, a.fonts, a.legalLineTargets(), a.sel)
	compactSide := 1 - fullSide
	DrawOpponentStrip(&a.layout, a.gs, compactSide, a.showOpp, a.fonts)
	return DrawButtonBar(&a.layout, []string{"Ny", "Meny"}, a.fonts)
}

// fullBoardSide is whichever board is currently shown full-size in the
// middle band: normally the side to move, or — while "Visa motståndare" is
// toggled on — the other board (shown read-only).
func (a *app) fullBoardSide() int {
	if a.showOpp {
		return 1 - a.gs.Turn
	}
	return a.gs.Turn
}

// legalLineTargets returns, for the currently selected (source, color), which
// pattern-line indices (0..4, plus -1 for the floor, always legal) accept it
// — used to grey out illegal targets.
func (a *app) legalLineTargets() map[int]bool {
	out := map[int]bool{}
	if !a.sel.active {
		return out
	}
	for _, m := range a.gs.LegalMoves() {
		if m.Source == a.sel.source && m.Color == a.sel.color {
			out[m.TargetLine] = true
		}
	}
	return out
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
	a.sel = selection{}
	a.showOpp = false
	a.msg = ""
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
		return a.tapDrafting(p)
	default:
		return false // only the button bar (Fortsätt/Ny/Meny) is live here
	}
}

func (a *app) tapDrafting(p image.Point) bool {
	// The opponent-summary strip's toggle button is always live, even
	// mid-selection.
	if p.In(a.layout.ShowOppBtn) {
		a.showOpp = !a.showOpp
		a.sel = selection{}
		ink.Repaint()
		return true
	}
	if a.showOpp || a.gs.AITurn() {
		return false // read-only view, or not the human's turn
	}

	// 1) Tap a tile swatch in a factory, or a color chip in the center:
	// selects (source, color) — "tiles in hand".
	if src, col, ok := a.layout.HitSource(p); ok {
		if a.sel.active && a.sel.source == src && a.sel.color == col {
			a.sel = selection{} // tapping the same selection again cancels it
		} else {
			a.sel = selection{active: true, source: src, color: col}
		}
		a.msg = ""
		ink.Repaint()
		return true
	}

	// 2) Tap a pattern line (or the floor) on the ACTIVE board: commits.
	if a.sel.active {
		if target, ok := a.layout.HitTarget(p); ok {
			mv := game.Move{Source: a.sel.source, Color: a.sel.color, TargetLine: target}
			if a.gs.Play(mv) {
				a.sel = selection{}
				a.msg = ""
				a.updates++
				if a.gs.AITurn() {
					a.aiPend = true
				}
				ink.Repaint()
				return true
			}
			a.msg = "Ogiltigt drag — den raden passar inte"
			ink.Repaint()
			return false
		}
	}
	return false
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Fortsätt":
		if a.gs.Continue() {
			a.sel = selection{}
			a.showOpp = false
			if a.gs.AITurn() {
				a.aiPend = true
			}
			ink.Repaint()
			return true
		}
		return false
	case "Ny", "Spela igen":
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
