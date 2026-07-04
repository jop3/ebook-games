// Command studie is "En Studie i Grått" — an original tap-driven mystery for the
// PocketBook Verse Pro, and the second story on the Grottan adventure engine.
//
// It reuses grottan's engine package BYTE-FOR-BYTE (story/model.go, engine.go,
// save.go are identical files); this game is "shared engine + one extension
// (the Notebook, story/notebook.go) + authored data (story/storydata.go) +
// themed chrome". Examine things to gather clues into the notebook; tap two
// clues there to draw a deduction.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"studie/story"
)

type screen int

const (
	screenSplash screen = iota // shown on launch; tap → menu
	screenMenu
	screenGame
	screenRules
	screenNotebook   // the detective's pad — clues + deductions (spec §10b)
	screenAccuse     // the accusation: culprit / method / motive (spec §10d)
	screenResolution // the closing scene, on a solved case
)

type app struct {
	fonts  *Fonts
	screen screen

	st  *story.State
	log []string

	scroll    int
	stickTail bool
	armedVerb story.Verb
	verbArmed bool
	updates   int
	savePath  string
	hasSave   bool

	// notebook selection + last combine result.
	selClue int
	nbMsg   string

	// accusation state.
	accCulprit string
	accMethod  string
	accMotive  string
	accResult  string
	resolution string

	exitBtns   []button
	nounBtns   []button
	verbBtns   []button
	auxBtns    []button
	clueBtns   []button
	menuBtns   []button
	menuBtn    image.Rectangle
	bookBtn    image.Rectangle
	scrollUp   image.Rectangle
	scrollDown image.Rectangle
	rulesBack  image.Rectangle
	bookBack   image.Rectangle

	accuseBtn      image.Rectangle // "Anklaga…" on the notebook
	accSubmit      image.Rectangle
	accBack        image.Rectangle
	accCulpritBtns []button
	accMethodBtns  []button
	accMotiveBtns  []button
	resolutionBack image.Rectangle

	downY     int
	downValid bool
}

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

func (a *app) Init() error {
	a.fonts = InitFonts()
	a.screen = screenSplash
	a.savePath = "studie.sav"
	a.selClue = -1
	if s, err := story.LoadFile(a.savePath); err == nil {
		a.st = s
		a.hasSave = true
	}
	ink.Repaint()
	return nil
}

func (a *app) Close() error {
	if a.st != nil {
		_ = story.SaveFile(a.savePath, a.st)
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
		DrawSplash(sz, a.fonts, "En Studie i Grått", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.drawMenu(sz)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(sz)
	case screenRules:
		a.rulesBack = DrawRules(sz, a.fonts, "En Studie i Grått", rulesParagraphs)
		ink.FullUpdate()
	case screenNotebook:
		a.bookBack = a.drawNotebook(sz)
		ink.FullUpdate()
	case screenAccuse:
		a.accBack = a.drawAccuse(sz)
		ink.FullUpdate()
	case screenResolution:
		a.resolutionBack = a.drawResolution(sz)
		ink.FullUpdate()
	}
}

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
		case screenAccuse:
			a.screen = screenNotebook
			ink.Repaint()
			return true
		case screenNotebook, screenResolution:
			a.screen = screenGame
			ink.Repaint()
			return true
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
	case screenNotebook:
		return a.tapNotebook(p)
	case screenAccuse:
		return a.tapAccuse(p)
	case screenResolution:
		if p.In(a.resolutionBack) {
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
	a.selClue = -1
	a.scroll = 0
	a.stickTail = true
	if fresh {
		a.accCulprit, a.accMethod, a.accMotive = "", "", ""
		a.accResult, a.resolution, a.nbMsg = "", "", ""
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
	if p.In(a.bookBtn) {
		a.selClue = -1
		a.screen = screenNotebook
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
			a.print(story.Act(a.st, story.VerbInventory, story.OBJ_NONE))
			a.verbArmed = false
			a.autosave()
			ink.Repaint()
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
	if !story.NeedsNoun(v) {
		a.print(story.Act(a.st, v, story.OBJ_NONE))
		a.verbArmed = false
		a.autosave()
		ink.Repaint()
		return
	}
	if a.verbArmed && a.armedVerb == v {
		a.verbArmed = false
	} else {
		a.armedVerb = v
		a.verbArmed = true
	}
	ink.Repaint()
}

// tapNoun runs the armed verb on the object; an unarmed tap examines it. For an
// examinable clue object, examining prints the full observation and records it.
func (a *app) tapNoun(n story.ObjID) {
	v := story.VerbExamine
	if a.verbArmed {
		v = a.armedVerb
	}
	if v == story.VerbExamine {
		if clue, ok := story.ClueFor[n]; ok {
			_, added := story.AddClueFor(a.st, n)
			a.print([]string{clue.Text})
			if added {
				a.print([]string{"» Nedtecknat i anteckningsboken."})
			}
		} else {
			a.print(story.Act(a.st, v, n))
		}
	} else {
		a.print(story.Act(a.st, v, n))
	}
	a.verbArmed = false
	a.autosave()
	ink.Repaint()
}

func (a *app) doMove(m story.Motion) {
	a.verbArmed = false
	a.print(story.Move(a.st, m))
	a.updates = 0
	a.autosave()
	ink.Repaint()
}

// tapNotebook handles clue selection/combining and opening the accusation.
func (a *app) tapNotebook(p image.Point) bool {
	if p.In(a.bookBack) {
		a.screen = screenGame
		ink.Repaint()
		return true
	}
	if p.In(a.accuseBtn) {
		a.accResult = ""
		a.screen = screenAccuse
		ink.Repaint()
		return true
	}
	for _, b := range a.clueBtns {
		if b.hit(p) {
			idx := b.Data
			switch {
			case a.selClue == idx:
				a.selClue = -1 // deselect
			case a.selClue < 0:
				a.selClue = idx
				a.nbMsg = ""
			default:
				if d, ok := story.Combine(a.st, a.selClue, idx); ok {
					a.nbMsg = d.Text
					a.autosave()
				} else {
					a.nbMsg = "Inget uppenbart samband — ännu."
				}
				a.selClue = -1
			}
			ink.Repaint()
			return true
		}
	}
	return false
}

// tapAccuse handles the culprit/method/motive pickers and the charge.
func (a *app) tapAccuse(p image.Point) bool {
	if p.In(a.accBack) {
		a.screen = screenNotebook
		ink.Repaint()
		return true
	}
	for _, b := range a.accCulpritBtns {
		if b.hit(p) {
			a.accCulprit = b.Label
			a.accResult = ""
			ink.Repaint()
			return true
		}
	}
	for _, b := range a.accMethodBtns {
		if b.hit(p) {
			a.accMethod = b.Label
			a.accResult = ""
			ink.Repaint()
			return true
		}
	}
	for _, b := range a.accMotiveBtns {
		if b.hit(p) {
			a.accMotive = b.Label
			a.accResult = ""
			ink.Repaint()
			return true
		}
	}
	if p.In(a.accSubmit) {
		if a.accCulprit == "" || a.accMethod == "" || a.accMotive == "" {
			a.accResult = "Välj en skyldig, en metod och ett motiv innan du anklagar."
			ink.Repaint()
			return true
		}
		v := story.Accuse(a.st, a.accCulprit, a.accMethod, a.accMotive)
		a.autosave()
		if v.Win {
			a.resolution = v.Text
			a.screen = screenResolution
		} else {
			a.accResult = v.Text
		}
		ink.Repaint()
		return true
	}
	return false
}

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
