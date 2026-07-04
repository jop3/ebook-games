// Command twenty48 is the 2048 sliding-tile puzzle for the PocketBook Verse
// Pro (PB634), built on the dennwc/inkview SDK.
//
// Swipe (or tap an arrow) to slide every tile on a 4x4 grid; equal tiles that
// collide merge into their sum. Reach 2048 to win, keep playing for a higher
// score, or run out of moves and start over. Single player, untimed.
//
// Pure game logic (board, slide/merge, spawn, win/game-over) lives in the
// twenty48/game package with no SDK dependency and is unit-tested; this file
// and ui.go handle rendering and input.
package main

import (
	"image"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	ink "github.com/dennwc/inkview"

	"twenty48/game"
)

type screen int

const (
	screenSplash screen = iota
	screenMenu
	screenGame
	screenRules
)

const bestScoreFile = "2048_best.txt"

type app struct {
	fonts  *Fonts
	screen screen
	menu   *Menu
	rng    *rand.Rand

	gs      *game.GameState
	layout  Layout
	buttons []Button
	updates int

	rulesBack image.Rectangle

	// Swipe tracking: the pointer/touch-down point, to compare against the
	// up point and classify a swipe vs. a tap.
	downPt image.Point
	downOK bool
}

func main() {
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}
	if err := ink.Run(&app{}); err != nil {
		panic(err)
	}
}

// --- ink.App -----------------------------------------------------------------

func (a *app) Init() error {
	a.fonts = InitFonts()
	a.menu = NewMenu()
	a.screen = screenSplash
	a.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
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
		DrawSplash(screenSize, a.fonts, "2048", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts, a.loadBest())
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "2048", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.gs, a.fonts)
	DrawBoard(&a.layout, a.gs.Board, a.fonts)
	a.buttons = DrawButtonBar(&a.layout, a.buttonLabels(), a.fonts)
	if a.gs.Status != game.StatusPlaying {
		DrawBanner(&a.layout, a.fonts, a.bannerText())
	}

	if a.updates == 0 || a.updates%fullUpdateEvery == 0 || a.gs.Status != game.StatusPlaying {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) buttonLabels() []string {
	switch a.gs.Status {
	case game.StatusWon:
		return []string{"Fortsätt", "Ny", "Meny"}
	case game.StatusOver:
		return []string{"Ny", "Meny"}
	default:
		return []string{"Ny", "Meny"}
	}
}

func (a *app) bannerText() string {
	if a.gs.Status == game.StatusWon {
		return "Du klarade 2048!"
	}
	return "Spelet slut"
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

// swipeThreshold is the minimum pixel delta (on the larger axis) that counts
// as a swipe rather than a tap (guide §5a's swipe recipe).
const swipeThreshold = 110

func (a *app) Pointer(e ink.PointerEvent) bool {
	switch e.State {
	case ink.PointerDown:
		a.downPt = e.Point
		a.downOK = true
		return false
	case ink.PointerUp:
		return a.handleUp(e.Point)
	}
	return false
}

func (a *app) Touch(e ink.TouchEvent) bool {
	switch e.State {
	case ink.TouchDown:
		a.downPt = e.Point
		a.downOK = true
		return false
	case ink.TouchUp:
		return a.handleUp(e.Point)
	}
	return false
}

// handleUp classifies the gesture that just ended at p: a swipe (if far
// enough from the down point, only meaningful during play) or a tap.
func (a *app) handleUp(p image.Point) bool {
	if a.downOK && a.screen == screenGame && a.gs.Status == game.StatusPlaying {
		dx := p.X - a.downPt.X
		dy := p.Y - a.downPt.Y
		adx, ady := abs(dx), abs(dy)
		if adx >= swipeThreshold || ady >= swipeThreshold {
			a.downOK = false
			var dir game.Dir
			if adx >= ady {
				if dx > 0 {
					dir = game.Right
				} else {
					dir = game.Left
				}
			} else {
				if dy > 0 {
					dir = game.Down
				} else {
					dir = game.Up
				}
			}
			return a.applyMove(dir)
		}
	}
	a.downOK = false
	return a.handleTap(p)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
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
	if target, ok := a.menu.HandleTouch(p); ok {
		a.startGame(target)
		return true
	}
	return false
}

func (a *app) startGame(target int) {
	a.gs = game.NewGame(target, a.loadBest(), a.rng)
	a.screen = screenGame
	a.updates = 0
	ink.Repaint()
}

func (a *app) tapGame(p image.Point) bool {
	for _, b := range a.buttons {
		if b.Hit(p) {
			return a.handleButton(b.Label)
		}
	}
	if arrow, ok := a.layout.ArrowHit(p); ok {
		return a.applyMove(arrow)
	}
	return false
}

func (a *app) applyMove(dir game.Dir) bool {
	if a.gs.Status != game.StatusPlaying {
		return false
	}
	if a.gs.Move(dir) {
		a.updates++
		a.saveBest()
		ink.Repaint()
		return true
	}
	return false
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Fortsätt":
		a.gs.Continue()
		ink.Repaint()
		return true
	case "Ny":
		a.startGame(a.gs.Target)
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

// --- Best-score persistence --------------------------------------------------
//
// Stored next to the executable (cwd is chdir'd there in main, matching the
// rest of the library's save-file convention, e.g. studie's "studie.sav").
// A missing or corrupt file is treated as best=0 — never fatal.

func (a *app) loadBest() int {
	data, err := os.ReadFile(bestScoreFile)
	if err != nil {
		return 0
	}
	return game.ParseBest(data)
}

func (a *app) saveBest() {
	if a.gs == nil {
		return
	}
	_ = os.WriteFile(bestScoreFile, game.FormatBest(a.gs.Best), 0o644)
}
