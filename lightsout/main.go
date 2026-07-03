package main

import (
	"image"
	"math/rand"
	"time"

	ink "github.com/dennwc/inkview"
	"lightsout/game"
)

type screen int

const (
	screenSplash screen = iota // shown on launch; tap → menu
	screenMenu
	screenPlay
	screenRules // reached from a "Regler" button on the menu
)

// fonts opened once in Init, reused every Draw.
type fonts struct {
	title  *ink.Font // big menu title
	big    *ink.Font // buttons / status
	medium *ink.Font // labels
	huge   *ink.Font // win banner
}

type app struct {
	scr    screen
	board  *game.Board
	size   int // current board dimension (3/5/7)
	rng    *rand.Rand
	fonts  fonts
	won    bool
	solved bool // showing solution hint overlay

	// solution overlay: cells to press (from game.Solve)
	solution [][]bool

	// cached geometry for the current Draw (set in drawPlay, read by handleTap)
	gridX, gridY, cell int

	// button rectangles (screen coords), recomputed each Draw
	btnNew    image.Rectangle
	btnMenu   image.Rectangle
	btnHint   image.Rectangle
	menuBtns  []menuButton

	menuRules image.Rectangle // "Regler" button on the menu
	rulesBack image.Rectangle // "Tillbaka" button on the rules screen
}

type menuButton struct {
	rect image.Rectangle
	size int // board size this button starts
}

func newApp() *app {
	return &app{
		scr:  screenSplash,
		size: 5,
		rng:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (a *app) Init() error {
	a.fonts.title = ink.OpenFont(ink.DefaultFontBold, 84, true)
	a.fonts.big = ink.OpenFont(ink.DefaultFontBold, 52, true)
	a.fonts.medium = ink.OpenFont(ink.DefaultFont, 40, true)
	a.fonts.huge = ink.OpenFont(ink.DefaultFontBold, 96, true)
	ink.Repaint() // avoid dead first-tap
	return nil
}

func (a *app) Close() error {
	a.fonts.title.Close()
	a.fonts.big.Close()
	a.fonts.medium.Close()
	a.fonts.huge.Close()
	return nil
}

func (a *app) Draw() {
	ink.ClearScreen()
	switch a.scr {
	case screenSplash:
		a.drawSplash()
	case screenMenu:
		a.drawMenu()
	case screenPlay:
		a.drawPlay()
	case screenRules:
		a.drawRules()
	}
	ink.FullUpdate()
}

// startGame builds a fresh solvable puzzle of the given size.
func (a *app) startGame(n int) {
	a.size = n
	a.board = game.New(n)
	// Scramble strength scales with the board so puzzles feel non-trivial.
	a.board.Generate(n*n, a.rng)
	a.won = false
	a.solved = false
	a.solution = nil
	a.scr = screenPlay
}

func (a *app) Pointer(e ink.PointerEvent) bool {
	if e.State != ink.PointerUp {
		return false
	}
	return a.handleTap(e.Point)
}

func (a *app) Touch(e ink.TouchEvent) bool { // fallback path
	if e.State != ink.TouchUp {
		return false
	}
	return a.handleTap(e.Point)
}

func (a *app) Key(e ink.KeyEvent) bool {
	if e.State != ink.KeyStateUp {
		return false
	}
	if e.Key == ink.KeyBack {
		if a.scr == screenPlay || a.scr == screenRules {
			a.scr = screenMenu
			ink.Repaint()
			return true
		}
	}
	return false
}

func (a *app) Orientation(o ink.Orientation) bool { return false }

func main() {
	ink.Run(newApp())
}
