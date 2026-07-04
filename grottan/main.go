// Command grottan is a tap-driven port of Colossal Cave Adventure for the
// PocketBook Verse Pro (PB634), built on the dennwc/inkview SDK.
//
// The classic game uses a typed parser; typing on e-ink is miserable, so the
// parser is replaced with a tap verb + tap noun interface (the Scott-Adams
// two-word model, which Colossal Cave already fits). Tap an exit to move; arm a
// verb then tap a noun to act; discovered magic words appear under "Säg…".
//
// All game rules live in the ink-free grottan/story package (data generated from
// Open Adventure's adventure.yaml), so the engine unit-tests without a device.
// This file wires the SDK event loop; ui.go does the drawing.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"grottan/story"
)

type screen int

const (
	screenSplash screen = iota // shown on launch; tap → menu
	screenMenu
	screenGame
	screenRules
)

type app struct {
	fonts  *Fonts
	screen screen

	st  *story.State
	log []string // transcript, one logical line per entry ("" = blank separator)

	scroll    int  // index of first visible display line
	stickTail bool // keep the transcript pinned to the newest line
	armedVerb story.Verb
	verbArmed bool
	sayOpen   bool
	updates   int
	savePath  string
	hasSave   bool

	// hit regions captured each Draw.
	exitBtns   []button
	nounBtns   []button
	verbBtns   []button
	auxBtns    []button
	sayBtns    []button
	menuBtns   []button
	menuBtn    image.Rectangle
	scrollUp   image.Rectangle
	scrollDown image.Rectangle
	rulesBack  image.Rectangle

	// pointer-down tracking for swipe scrolling.
	downY     int
	downValid bool
}

// button is a labelled tappable rectangle carrying an integer payload (a Motion,
// an ObjID, or a Verb depending on the row).
type button struct {
	Rect    image.Rectangle
	Label   string
	Data    int
	Armed   bool
	Disable bool
}

func (b button) hit(p image.Point) bool { return !b.Disable && p.In(b.Rect) }

func main() {
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}
	if err := ink.Run(&app{}); err != nil {
		panic(err)
	}
}

// --- ink.App ----------------------------------------------------------------

func (a *app) Init() error {
	a.fonts = InitFonts()
	a.screen = screenSplash
	a.savePath = "grottan.sav"
	if s, err := story.LoadFile(a.savePath); err == nil {
		a.st = s
		a.hasSave = true
	}
	ink.Repaint()
	return nil
}

func (a *app) Close() error {
	if a.st != nil {
		_ = story.SaveFile(a.savePath, a.st) // autosave on exit (§5)
	}
	if a.fonts != nil {
		a.fonts.Close()
	}
	return nil
}

const fullUpdateEvery = 6

func (a *app) Draw() {
	sz := ink.ScreenSize()
	switch a.screen {
	case screenSplash:
		DrawSplash(sz, a.fonts, "Grottan", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.drawMenu(sz)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(sz)
	case screenRules:
		a.rulesBack = DrawRules(sz, a.fonts, "Grottan", rulesParagraphs)
		ink.FullUpdate()
	}
}

// --- input ------------------------------------------------------------------

// swipeThreshold is the vertical travel (px) above which a pointer gesture is a
// scroll rather than a tap (guide §5a).
const swipeThreshold = 110

func (a *app) Pointer(e ink.PointerEvent) bool {
	switch e.State {
	case ink.PointerDown:
		a.downY = e.Point.Y
		a.downValid = true
		return true
	case ink.PointerUp:
		return a.pointerUp(e.Point)
	}
	return false
}

func (a *app) Touch(e ink.TouchEvent) bool {
	switch e.State {
	case ink.TouchDown:
		a.downY = e.Point.Y
		a.downValid = true
		return true
	case ink.TouchUp:
		return a.pointerUp(e.Point)
	}
	return false
}

// pointerUp resolves a gesture: a long vertical drag over the transcript scrolls
// it; anything else is a tap dispatched to the active screen.
func (a *app) pointerUp(p image.Point) bool {
	if a.downValid && a.screen == screenGame {
		if dy := a.downY - p.Y; dy >= swipeThreshold || dy <= -swipeThreshold {
			a.downValid = false
			a.scrollBy(dy / lineH)
			return true
		}
	}
	a.downValid = false
	return a.handleTap(p)
}

func (a *app) Key(e ink.KeyEvent) bool {
	if e.State == ink.KeyStateUp && e.Key == ink.KeyBack {
		switch a.screen {
		case screenGame, screenRules:
			a.screen = screenMenu
			ink.Repaint()
			return true
		case screenMenu:
			a.screen = screenSplash
			ink.Repaint()
			return true
		}
	}
	return false
}

func (a *app) Orientation(o ink.Orientation) bool {
	ink.Repaint()
	return true
}

// --- tap dispatch -----------------------------------------------------------

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
	for _, b := range a.menuBtns {
		if b.hit(p) {
			switch menuAction(b.Data) {
			case menuContinue:
				if a.st == nil {
					a.st = story.New()
				}
				a.enterGame(false)
			case menuNew:
				a.st = story.New()
				a.enterGame(true)
			case menuRules:
				a.screen = screenRules
				ink.Repaint()
			}
			return true
		}
	}
	return false
}

func (a *app) enterGame(fresh bool) {
	a.verbArmed = false
	a.sayOpen = false
	a.scroll = 0
	a.stickTail = true
	if fresh {
		a.log = nil
		a.print(story.Describe(a.st))
		a.autosave()
	} else if len(a.log) == 0 {
		a.print(story.Describe(a.st))
	}
	a.screen = screenGame
	a.updates = 0
	ink.Repaint()
}

func (a *app) tapGame(p image.Point) bool {
	if p.In(a.menuBtn) {
		a.autosave()
		a.screen = screenMenu
		ink.Repaint()
		return true
	}
	if p.In(a.scrollUp) {
		a.scrollBy(-3)
		return true
	}
	if p.In(a.scrollDown) {
		a.scrollBy(3)
		return true
	}
	// Say… submenu takes priority while open.
	if a.sayOpen {
		for _, b := range a.sayBtns {
			if b.hit(p) {
				a.doMove(story.Motion(b.Data))
				a.sayOpen = false
				return true
			}
		}
	}
	for _, b := range a.exitBtns {
		if b.hit(p) {
			a.doMove(story.Motion(b.Data))
			return true
		}
	}
	for _, b := range a.verbBtns {
		if b.hit(p) {
			a.tapVerb(story.Verb(b.Data))
			return true
		}
	}
	for _, b := range a.auxBtns {
		if b.hit(p) {
			a.tapAux(b.Data)
			return true
		}
	}
	for _, b := range a.nounBtns {
		if b.hit(p) {
			a.tapNoun(story.ObjID(b.Data))
			return true
		}
	}
	return false
}

func (a *app) tapVerb(v story.Verb) {
	a.sayOpen = false
	if !story.NeedsNoun(v) { // LOOK / INVENTORY run immediately
		a.print(story.Act(a.st, v, story.OBJ_NONE))
		a.verbArmed = false
		a.autosave()
		ink.Repaint()
		return
	}
	if a.verbArmed && a.armedVerb == v {
		a.verbArmed = false // tapping the armed verb again cancels
	} else {
		a.armedVerb = v
		a.verbArmed = true
	}
	ink.Repaint()
}

func (a *app) tapNoun(n story.ObjID) {
	v := story.VerbExamine // an unarmed noun tap examines it
	if a.verbArmed {
		v = a.armedVerb
	}
	a.print(story.Act(a.st, v, n))
	a.verbArmed = false
	a.autosave()
	ink.Repaint()
}

// aux button payloads.
const (
	auxInventory = iota
	auxSay
)

func (a *app) tapAux(which int) {
	switch which {
	case auxInventory:
		a.sayOpen = false
		a.print(story.Act(a.st, story.VerbInventory, story.OBJ_NONE))
		a.verbArmed = false
		a.autosave()
	case auxSay:
		a.sayOpen = !a.sayOpen
	}
	ink.Repaint()
}

func (a *app) doMove(m story.Motion) {
	a.sayOpen = false
	a.verbArmed = false
	a.print(story.Move(a.st, m))
	a.updates = 0 // room changed: force a clean FullUpdate to clear ghosting
	a.autosave()
	ink.Repaint()
}

// --- transcript -------------------------------------------------------------

// print appends narration to the transcript (preceded by a blank separator) and
// pins the view to the newest line.
func (a *app) print(lines []string) {
	if len(a.log) > 0 {
		a.log = append(a.log, "")
	}
	a.log = append(a.log, lines...)
	a.stickTail = true
}

func (a *app) scrollBy(delta int) {
	a.scroll += delta
	a.stickTail = false
	if a.scroll < 0 {
		a.scroll = 0
	}
	ink.Repaint()
}

func (a *app) autosave() {
	if a.st != nil {
		_ = story.SaveFile(a.savePath, a.st)
		a.hasSave = true
	}
}
