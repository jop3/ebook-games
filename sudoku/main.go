package main

import (
	"image"
	"math/rand"

	ink "github.com/dennwc/inkview"
	"sudoku/game"
)

func main() {
	ink.Run(newApp())
}

// screen is the top-level UI state.
type screen int

const (
	screenSplash screen = iota // shown on launch; tap -> menu
	screenMenu
	screenPlay
	screenRules // reached from a "Regler" button on the menu
)

// fonts holds every (typeface,size) opened ONCE in Init and reused.
type fonts struct {
	title  *ink.Font // big menu/title text
	cell   *ink.Font // digits in grid cells (bold)
	pencil *ink.Font // small pencil-mark notes
	button *ink.Font // button labels
	small  *ink.Font // status / hint text

	splash *ink.Font // big splash title
	rules  *ink.Font // rules body text
	motif  *ink.Font // digits inside the splash motif
}

type app struct {
	f      fonts
	rng    *rand.Rand
	screen screen

	diff   game.Difficulty
	puzzle game.Puzzle
	board  game.Board
	// notes[r][c] is a bitset of pencil marks (bit d set => note d present).
	notes [9][9]uint16

	selR, selC int  // selected cell, -1 if none
	noteMode   bool // pencil-mark input mode
	message    string
	showConf   bool // whether a "Klar?" check flagged conflicts / wrong cells

	rulesBack image.Rectangle // "Tillbaka" button rect on the rules screen
	menuRules image.Rectangle // "Regler" button rect on the menu
}

func newApp() *app {
	return &app{
		rng:    rand.New(rand.NewSource(rand.Int63())),
		screen: screenSplash,
		selR:   -1,
		selC:   -1,
		diff:   game.Medium,
	}
}

// --- ink.App interface ------------------------------------------------

func (a *app) Init() error {
	// Open every font ONCE (guide rule 2). Sizes are re-derived per draw
	// from ScreenSize scaling if needed, but fixed sizes are fine for a
	// single known device.
	a.f.title = ink.OpenFont(ink.DefaultFontBold, 64, true)
	a.f.cell = ink.OpenFont(ink.DefaultFontBold, 56, true)
	a.f.pencil = ink.OpenFont(ink.DefaultFont, 20, true)
	a.f.button = ink.OpenFont(ink.DefaultFontBold, 40, true)
	a.f.small = ink.OpenFont(ink.DefaultFont, 34, true)
	a.f.splash = ink.OpenFont(ink.DefaultFontBold, 76, true)
	a.f.rules = ink.OpenFont(ink.DefaultFont, 34, true)
	a.f.motif = ink.OpenFont(ink.DefaultFontBold, 60, true)
	ink.Repaint() // guide rule 4: avoid dead first tap
	return nil
}

func (a *app) Close() error {
	a.f.title.Close()
	a.f.cell.Close()
	a.f.pencil.Close()
	a.f.button.Close()
	a.f.small.Close()
	a.f.splash.Close()
	a.f.rules.Close()
	a.f.motif.Close()
	return nil
}

func (a *app) Orientation(o ink.Orientation) bool { return false }

func (a *app) Key(e ink.KeyEvent) bool {
	if e.State != ink.KeyStateUp {
		return false
	}
	if e.Key == ink.KeyBack {
		if a.screen == screenPlay || a.screen == screenRules {
			a.screen = screenMenu
			a.repaint()
			return true
		}
	}
	return false
}

// Pointer handles taps (guide rule 1: taps arrive here on PointerUp).
func (a *app) Pointer(e ink.PointerEvent) bool {
	if e.State != ink.PointerUp {
		return false
	}
	return a.handleTap(e.Point)
}

// Touch is the fallback path (rarely fires on this device).
func (a *app) Touch(e ink.TouchEvent) bool {
	if e.State != ink.TouchUp {
		return false
	}
	return a.handleTap(e.Point)
}

func (a *app) repaint() {
	ink.ClearScreen()
	ink.Repaint()
	ink.FullUpdate()
}

// --- game control -----------------------------------------------------

func (a *app) newGame(d game.Difficulty) {
	a.diff = d
	a.puzzle = game.Generate(d, a.rng)
	a.board = a.puzzle.Start
	a.notes = [9][9]uint16{}
	a.selR, a.selC = -1, -1
	a.noteMode = false
	a.message = ""
	a.showConf = false
	a.screen = screenPlay
}

// placeDigit fills or notes the selected cell with d (1..9). Given cells
// are immutable.
func (a *app) placeDigit(d int) {
	if a.selR < 0 || a.selC < 0 {
		return
	}
	if a.puzzle.Given[a.selR][a.selC] {
		return
	}
	a.showConf = false
	a.message = ""
	if a.noteMode {
		a.notes[a.selR][a.selC] ^= 1 << uint(d)
		return
	}
	if a.board[a.selR][a.selC] == d {
		a.board[a.selR][a.selC] = 0 // tap same digit clears
	} else {
		a.board[a.selR][a.selC] = d
		a.notes[a.selR][a.selC] = 0
	}
}

func (a *app) erase() {
	if a.selR < 0 || a.selC < 0 || a.puzzle.Given[a.selR][a.selC] {
		return
	}
	a.board[a.selR][a.selC] = 0
	a.notes[a.selR][a.selC] = 0
	a.showConf = false
	a.message = ""
}

// check evaluates the current board and sets the status message.
func (a *app) check() {
	a.showConf = true
	if !a.board.IsComplete() {
		if len(a.board.Conflicts()) > 0 {
			a.message = "Konflikter markerade"
		} else {
			a.message = "Inte klar än"
		}
		return
	}
	if a.board.IsSolved() {
		a.message = "Rätt! Klart!"
	} else {
		a.message = "Fel någonstans"
	}
}
