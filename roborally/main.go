// Command roborally is a simplified, solo-vs-AI port of Robo Rally for the
// PocketBook Verse Pro (PB634), built on the dennwc/inkview SDK.
//
// One human races 1–3 AI robots across a hazard-filled factory floor: each round
// everyone secretly programs five movement cards into registers, then the
// registers resolve one at a time in priority order while conveyors, gears and
// lasers shove and singe the robots. First to touch every checkpoint in order
// wins. All rules, physics, the level generator and the (blind) AI live in the
// roborally/game package with no SDK dependency and are unit-tested; this file
// and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"roborally/game"
)

type screen int

const (
	screenSplash screen = iota
	screenMenu
	screenProgram
	screenResolve
	screenDone
	screenRules
)

// config holds the menu selections for the next game. courseSel indexes the
// combined course list: 0..NumCurated-1 are the hand-authored "Bana N", and the
// three after that are "Slump" at each difficulty tier.
type config struct {
	courseSel int
	nAI       int
	ai        game.AILevel
}

// courseCount is the total number of selectable courses (curated + 3 random).
func courseCount() int { return game.NumCurated() + 3 }

type app struct {
	fonts  *Fonts
	screen screen
	cfg    config
	seed   int64

	gs     *game.GameState
	layout BoardLayout

	// hit rects, refreshed each Draw of the relevant screen
	menuRows  []menuRow
	regRects  [5]image.Rectangle
	handRects []image.Rectangle
	korBtn    image.Rectangle
	menyBtn   image.Rectangle
	nastaBtn  image.Rectangle
	againBtn  image.Rectangle
	rulesBack image.Rectangle
	rulesBtn  image.Rectangle

	updates int
}

func main() {
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}
	if err := ink.Run(&app{}); err != nil {
		panic(err)
	}
}

func (a *app) Init() error {
	a.fonts = InitFonts()
	a.screen = screenSplash
	a.cfg = config{courseSel: 0, nAI: 2, ai: game.AIMedium}
	a.seed = 1
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
	sz := ink.ScreenSize()
	switch a.screen {
	case screenSplash:
		DrawSplash(sz, a.fonts, "Robo Rally", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menuRows, a.rulesBtn = DrawMenu(sz, a.fonts, a.cfg)
		ink.FullUpdate()
	case screenProgram:
		a.drawProgram(sz)
		ink.FullUpdate()
	case screenResolve:
		a.drawResolve(sz)
		ink.FullUpdate()
	case screenDone:
		a.againBtn, a.menyBtn = a.drawDone(sz)
		ink.FullUpdate()
	case screenRules:
		a.rulesBack = DrawRules(sz, a.fonts, "Robo Rally", rulesParagraphs)
		ink.FullUpdate()
	}
}

// --- input -----------------------------------------------------------------

func (a *app) Key(e ink.KeyEvent) bool {
	if e.State == ink.KeyStateUp && e.Key == ink.KeyBack {
		switch a.screen {
		case screenProgram, screenResolve, screenRules, screenDone:
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
	case screenProgram:
		return a.tapProgram(p)
	case screenResolve:
		return a.tapResolve(p)
	case screenDone:
		if p.In(a.againBtn) {
			a.startGame()
			return true
		}
		if p.In(a.menyBtn) {
			a.screen = screenMenu
			ink.Repaint()
			return true
		}
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
	if p.In(a.rulesBtn) {
		a.screen = screenRules
		ink.Repaint()
		return true
	}
	for _, row := range a.menuRows {
		if p.In(row.rect) {
			a.applyMenu(row.id)
			ink.Repaint()
			return true
		}
	}
	return false
}

// applyMenu cycles a setting or starts the game.
func (a *app) applyMenu(id string) {
	switch id {
	case "bana":
		a.cfg.courseSel = (a.cfg.courseSel + 1) % courseCount()
	case "nai":
		a.cfg.nAI++
		if a.cfg.nAI > 3 {
			a.cfg.nAI = 1
		}
	case "ai":
		a.cfg.ai = game.AILevel((int(a.cfg.ai) + 1) % 3)
	case "start":
		a.startGame()
	}
}

func (a *app) startGame() {
	var board *game.Board
	if a.cfg.courseSel < game.NumCurated() {
		board = game.CuratedCourse(a.cfg.courseSel)
	} else {
		diff := game.CourseDiff(a.cfg.courseSel - game.NumCurated())
		a.seed++
		board = game.GenerateCourse(diff, a.seed*2654435761)
	}
	a.gs = game.NewGame(board, a.cfg.nAI, a.cfg.ai, a.seed*97+13)
	a.screen = screenProgram
	a.updates = 0
	ink.Repaint()
}

func (a *app) tapProgram(p image.Point) bool {
	if p.In(a.menyBtn) {
		a.screen = screenMenu
		ink.Repaint()
		return true
	}
	if p.In(a.korBtn) && a.gs.ProgramComplete() {
		a.gs.StartResolve()
		a.screen = screenResolve
		ink.Repaint()
		return true
	}
	// Tap a filled register to clear it.
	for r := 0; r < 5; r++ {
		if p.In(a.regRects[r]) {
			if a.gs.ClearRegister(r) {
				ink.Repaint()
				return true
			}
		}
	}
	// Tap a hand card to place it.
	for i, rect := range a.handRects {
		if p.In(rect) {
			if a.gs.PlaceFromHand(i) {
				ink.Repaint()
				return true
			}
		}
	}
	return false
}

func (a *app) tapResolve(p image.Point) bool {
	if p.In(a.menyBtn) {
		a.screen = screenMenu
		ink.Repaint()
		return true
	}
	if p.In(a.nastaBtn) {
		a.gs.StepRegister()
		a.updates++
		switch a.gs.Phase {
		case game.PhaseProgram:
			a.screen = screenProgram
		case game.PhaseDone:
			a.screen = screenDone
		}
		ink.Repaint()
		return true
	}
	return false
}

func (a *app) Orientation(o ink.Orientation) bool {
	ink.Repaint()
	return true
}
