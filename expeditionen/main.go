// Command expeditionen is Expeditionen ("Lost Cities"), a 2-player
// hand-management card game for the PocketBook Verse Pro (PB634), built on
// the dennwc/inkview SDK.
//
// Baserat på Lost Cities (Kosmos / Reiner Knizia), reimplemented here with a
// neutral working title and original card rendering. 1 human plays against 1
// AI opponent — the game's hidden hands rule out a meaningful hot-seat mode.
// Each turn, play a card to your own expedition row (in non-decreasing
// order, investment cards only before any number card) or discard it face
// up, then draw one card from the deck or the top of any discard pile.
// Score is settled per expedition once the deck runs out.
//
// Pure game logic (deck, rows, legality, scoring, AI) lives in the
// expeditionen/game package with no SDK dependency and is unit-tested; this
// file and ui.go handle rendering and input.
package main

import (
	"image"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	ink "github.com/dennwc/inkview"

	"expeditionen/game"
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

	gs     *game.State
	layout Layout
	rects  gameRects

	navButtons []Button
	rulesBack  image.Rectangle

	selected     int // index into the human's hand, or -1 for none
	playRejected bool

	updates int
	aiPend  bool // an AI move is queued to run on the next Draw
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
	a.selected = -1
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
		DrawSplash(screenSize, a.fonts, "Expeditionen", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// Compute the AI's move only AFTER this frame is shown, so the
		// player sees their own move land first — same deferred-AI pattern
		// as hasami/sushi.
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
		a.rulesBack = DrawRules(screenSize, a.fonts, "Expeditionen", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	a.rects = DrawGameScreen(&a.layout, a)
	a.navButtons = DrawButtonBar(a.layout.NavBar, []string{"Ny", "Meny"}, a.fonts)

	if a.gs.Phase == game.PhaseDone || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

// canDraw reports whether the human is currently allowed to tap a draw
// source (deck or a discard-pile top).
func (a *app) canDraw() bool {
	return a.gs != nil && a.gs.Phase == game.PhasePlaying && a.gs.HumanTurn() && a.gs.AwaitingDraw()
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
	if p.In(a.menu.StartButton()) {
		a.startGame()
		return true
	}
	return false
}

func (a *app) startGame() {
	a.gs = game.NewGame(a.rng.Shuffle)
	a.screen = screenGame
	a.selected = -1
	a.playRejected = false
	a.updates = 0
	a.aiPend = false
	ink.Repaint()
}

func (a *app) tapGame(p image.Point) bool {
	for _, b := range a.navButtons {
		if b.Hit(p) {
			return a.handleNavButton(b.Label)
		}
	}
	if a.gs.Phase != game.PhasePlaying || !a.gs.HumanTurn() {
		return false
	}

	if a.canDraw() {
		return a.tapDrawSlot(p)
	}

	// Choose-action sub-phase: tap a hand card to select/deselect it, or
	// tap Spela/Kasta once one is selected.
	for i, r := range a.rects.handCards {
		if p.In(r) {
			if a.selected == i {
				a.selected = -1
			} else {
				a.selected = i
			}
			a.playRejected = false
			ink.Repaint()
			return true
		}
	}
	if a.rects.haveAction {
		if p.In(a.rects.playBtn) {
			return a.attemptPlay()
		}
		if p.In(a.rects.discardBtn) {
			return a.attemptDiscard()
		}
	}
	return false
}

func (a *app) attemptPlay() bool {
	if a.selected < 0 {
		return false
	}
	if err := a.gs.PlayCard(game.HumanIdx, a.selected); err != nil {
		a.playRejected = true
		ink.Repaint()
		return true
	}
	a.afterHumanAction()
	return true
}

func (a *app) attemptDiscard() bool {
	if a.selected < 0 {
		return false
	}
	if err := a.gs.DiscardCard(game.HumanIdx, a.selected); err != nil {
		a.playRejected = true
		ink.Repaint()
		return true
	}
	a.afterHumanAction()
	return true
}

// afterHumanAction clears the selection/rejection state after a successful
// play or discard (the human still owes a draw before the AI's turn).
func (a *app) afterHumanAction() {
	a.selected = -1
	a.playRejected = false
	a.updates++
	ink.Repaint()
}

// tapDrawSlot handles a tap during the draw sub-phase: the 5 discard-pile
// tiles, then the deck tile (index game.NumSuits), per a.rects.drawSlots.
func (a *app) tapDrawSlot(p image.Point) bool {
	for i, s := range game.AllSuits {
		if p.In(a.rects.drawSlots[i]) {
			if err := a.gs.DrawFromDiscard(game.HumanIdx, s); err != nil {
				return false // empty pile; tap ignored
			}
			a.afterDraw()
			return true
		}
	}
	if p.In(a.rects.drawSlots[game.NumSuits]) {
		if err := a.gs.DrawFromDeck(game.HumanIdx); err != nil {
			return false
		}
		a.afterDraw()
		return true
	}
	return false
}

// afterDraw runs once the human's draw completes their turn: queue the AI's
// reply (if the round isn't over) and request a redraw.
func (a *app) afterDraw() {
	a.updates++
	if a.gs.AITurn() {
		a.aiPend = true
	}
	ink.Repaint()
}

func (a *app) handleNavButton(label string) bool {
	switch label {
	case "Ny":
		a.startGame()
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
