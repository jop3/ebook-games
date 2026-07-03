// Command anagram is the PocketBook Verse Pro build of "Ordskrav", a Swedish
// anagram word game. It wires the ink (inkview) event loop to the pure logic in
// package game/ and the drawing code in ui.go.
package main

import (
	"math/rand"
	"time"

	"anagram/game"

	ink "github.com/dennwc/inkview"
)

// screen identifies which view is currently shown.
type screen int

const (
	screenSplash screen = iota // shown on launch; tap → menu
	screenMenu
	screenPlay
	screenGiveUp
	screenRules // reached from the "Regler" button on the menu
)

type fonts struct {
	title   *ink.Font // big title / letters
	big     *ink.Font // letters, input
	button  *ink.Font // button labels
	body    *ink.Font // found words list
	small   *ink.Font // hints / status
}

type app struct {
	dict  *game.Dictionary
	rng   *rand.Rand
	round *game.Round

	scr    screen
	fonts  fonts
	status string // last feedback message

	// tappable regions computed each Draw, used by handleTap.
	hits []hit
}

// hit is a tappable rectangle with an action.
type hit struct {
	r      rectangle
	action func()
}

type rectangle struct{ x0, y0, x1, y1 int }

func (r rectangle) contains(x, y int) bool {
	return x >= r.x0 && x < r.x1 && y >= r.y0 && y < r.y1
}

func newApp() *app {
	return &app{
		dict: game.NewDictionary(),
		rng:  rand.New(rand.NewSource(time.Now().UnixNano())),
		scr:  screenSplash,
	}
}

func (a *app) newRound() {
	a.round = game.NewRound(a.dict, a.rng)
	a.scr = screenPlay
	a.status = ""
}

// --- ink.App interface ---

func (a *app) Init() error {
	a.fonts.title = ink.OpenFont(ink.DefaultFontBold, 64, true)
	a.fonts.big = ink.OpenFont(ink.DefaultFontBold, 56, true)
	a.fonts.button = ink.OpenFont(ink.DefaultFontBold, 40, true)
	a.fonts.body = ink.OpenFont(ink.DefaultFont, 36, true)
	a.fonts.small = ink.OpenFont(ink.DefaultFont, 30, true)
	ink.Repaint()
	return nil
}

func (a *app) Close() error {
	a.fonts.title.Close()
	a.fonts.big.Close()
	a.fonts.button.Close()
	a.fonts.body.Close()
	a.fonts.small.Close()
	return nil
}

func (a *app) Draw() {
	ink.ClearScreen()
	a.hits = a.hits[:0]
	switch a.scr {
	case screenSplash:
		a.drawSplash()
	case screenMenu:
		a.drawMenu()
	case screenPlay:
		a.drawPlay()
	case screenGiveUp:
		a.drawGiveUp()
	case screenRules:
		a.drawRules()
	}
	ink.FullUpdate()
}

func (a *app) Key(e ink.KeyEvent) bool {
	if e.State != ink.KeyStateUp {
		return false
	}
	if e.Key == ink.KeyBack {
		switch a.scr {
		case screenPlay, screenGiveUp, screenRules:
			a.scr = screenMenu
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
	return a.handleTap(e.Point.X, e.Point.Y)
}

func (a *app) Touch(e ink.TouchEvent) bool {
	if e.State != ink.TouchUp {
		return false
	}
	return a.handleTap(e.Point.X, e.Point.Y)
}

func (a *app) Orientation(o ink.Orientation) bool { return false }

func (a *app) handleTap(x, y int) bool {
	// The splash screen advances to the menu on ANY tap.
	if a.scr == screenSplash {
		a.scr = screenMenu
		ink.Repaint()
		return true
	}
	for _, h := range a.hits {
		if h.r.contains(x, y) {
			h.action()
			ink.Repaint()
			return true
		}
	}
	return false
}

func main() {
	ink.Run(newApp())
}
