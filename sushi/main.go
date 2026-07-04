// Command sushi is Sushi, a fast card-drafting game for the PocketBook Verse
// Pro (PB634), built on the dennwc/inkview SDK — the library's first
// card-drafting game rather than a board game.
//
// Baserat på Sushi Go! (Gamewright), reimplemented here with original icons
// and a neutral name. 1 human plays against 1-4 AI opponents: everyone is
// dealt a hand, and each turn every player simultaneously keeps one card
// (or two, via Chopsticks) and passes the rest along. Repeat until hands run
// out, score the round, then play 3 rounds total; Pudding is tallied once at
// the very end.
//
// Pure game logic (deck, drafting/passing engine, scoring, AI) lives in the
// sushi/game package with no SDK dependency and is unit-tested; this file
// and ui.go handle rendering and input.
package main

import (
	"image"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	ink "github.com/dennwc/inkview"

	"sushi/game"
)

type screen int

const (
	screenSplash screen = iota
	screenMenu
	screenGame
	screenRoundEnd
	screenGameEnd
	screenRules
)

type app struct {
	fonts  *Fonts
	screen screen
	menu   *Menu

	gs         *game.State
	numPlayers int

	layout    Layout
	handRects []image.Rectangle // tappable hand-card rects, index-aligned with gs.Players[0].Hand
	chopMode  bool              // "Använd ätpinnar" toggled on: next 2 taps form one pick
	selected  []int             // hand indices chosen so far this turn (chopMode only)
	chopBtn   image.Rectangle   // valid only when the chopsticks button is shown
	buttons   []Button          // bottom bar: Meny / Regler

	rulesBack   image.Rectangle
	roundEndBtn image.Rectangle
	gameEndBtns []Button

	updates int
	rng     *rand.Rand
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
		DrawSplash(screenSize, a.fonts, "Sushi", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
	case screenRoundEnd:
		a.drawRoundEnd(screenSize)
		ink.FullUpdate()
	case screenGameEnd:
		a.drawGameEnd(screenSize)
		ink.FullUpdate()
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Sushi", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	a.handRects = DrawGameScreen(&a.layout, a)
	a.buttons = DrawButtonBar(&a.layout.ButtonBar, []string{"Regler", "Meny"}, a.fonts)

	if a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) Key(e ink.KeyEvent) bool {
	if e.State == ink.KeyStateUp && e.Key == ink.KeyBack {
		switch a.screen {
		case screenGame, screenRoundEnd, screenGameEnd, screenRules:
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
	case screenRoundEnd:
		return a.tapRoundEnd(p)
	case screenGameEnd:
		return a.tapGameEnd(p)
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
	if n, ok := a.menu.HandleTouch(p); ok {
		a.startGame(n)
		return true
	}
	return false
}

func (a *app) startGame(numPlayers int) {
	a.numPlayers = numPlayers
	a.gs = game.NewGame(numPlayers, a.rng.Shuffle)
	a.screen = screenGame
	a.chopMode = false
	a.selected = nil
	a.updates = 0
	ink.Repaint()
}

// tapGame handles a tap on the drafting screen: the bottom bar (Regler/Meny),
// the "Använd ätpinnar" toggle, or a hand card.
func (a *app) tapGame(p image.Point) bool {
	for _, b := range a.buttons {
		if b.Hit(p) {
			return a.handleButton(b.Label)
		}
	}
	if a.chopAvailable() && p.In(a.chopBtn) {
		a.chopMode = !a.chopMode
		a.selected = nil
		ink.Repaint()
		return true
	}
	for i, r := range a.handRects {
		if p.In(r) {
			return a.tapHandCard(i)
		}
	}
	return false
}

// chopAvailable reports whether the human currently has an unplayed
// Chopsticks card sitting in their tableau (the only time a 2-card pick is
// legal).
func (a *app) chopAvailable() bool {
	if a.gs == nil || len(a.gs.Players) == 0 {
		return false
	}
	for _, c := range a.gs.Players[0].Tableau {
		if c.Kind == game.KindChopsticks {
			return true
		}
	}
	return false
}

func (a *app) tapHandCard(i int) bool {
	if !a.chopMode {
		a.playHuman(game.Pick{Idx: []int{i}})
		return true
	}
	// Chopsticks mode: build a 2-card pick across two taps; tapping an
	// already-selected card again deselects it instead.
	for k, s := range a.selected {
		if s == i {
			a.selected = append(a.selected[:k], a.selected[k+1:]...)
			ink.Repaint()
			return true
		}
	}
	a.selected = append(a.selected, i)
	if len(a.selected) == 2 {
		idx := append([]int(nil), a.selected...)
		sortInts2(idx)
		a.playHuman(game.Pick{Idx: idx})
		return true
	}
	ink.Repaint()
	return true
}

func sortInts2(idx []int) {
	if len(idx) == 2 && idx[0] > idx[1] {
		idx[0], idx[1] = idx[1], idx[0]
	}
}

// playHuman resolves the current turn (human pick + every AI seat's pick,
// all against the same pre-turn snapshot) and advances the screen if the
// round or game just ended.
func (a *app) playHuman(pick game.Pick) {
	if err := a.gs.PlayTurn(pick); err != nil {
		return // stale/invalid tap (e.g. double-fired event); ignore
	}
	a.chopMode = false
	a.selected = nil
	a.updates++
	switch a.gs.Phase {
	case game.PhaseRoundEnd:
		a.screen = screenRoundEnd
	case game.PhaseGameEnd:
		a.screen = screenGameEnd
	}
	ink.Repaint()
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Regler":
		a.screen = screenRules
		ink.Repaint()
		return true
	case "Meny":
		a.screen = screenMenu
		ink.Repaint()
		return true
	}
	return false
}

func (a *app) drawRoundEnd(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	a.roundEndBtn, a.gameEndBtns = DrawRoundEnd(&a.layout, a)
}

func (a *app) tapRoundEnd(p image.Point) bool {
	if p.In(a.roundEndBtn) {
		a.gs.AdvanceRound()
		a.screen = screenGame
		a.updates = 0
		ink.Repaint()
		return true
	}
	for _, b := range a.gameEndBtns {
		if b.Hit(p) && b.Label == "Meny" {
			a.screen = screenMenu
			ink.Repaint()
			return true
		}
	}
	return false
}

func (a *app) drawGameEnd(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	a.gameEndBtns = DrawGameEnd(&a.layout, a)
}

func (a *app) tapGameEnd(p image.Point) bool {
	for _, b := range a.gameEndBtns {
		if b.Hit(p) {
			switch b.Label {
			case "Ny omgång":
				a.startGame(a.numPlayers)
				return true
			case "Meny":
				a.screen = screenMenu
				ink.Repaint()
				return true
			}
		}
	}
	return false
}

func (a *app) Orientation(o ink.Orientation) bool {
	ink.Repaint()
	return true
}
