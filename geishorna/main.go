// Command geishorna is Geishorna ("Hanamikoji"), a 2-player hidden-hand card
// duel for the PocketBook Verse Pro (PB634), built on the dennwc/inkview SDK.
//
// Baserat på Hanamikoji (Kota Nakayama / EmperorS4), reimplemented here with a
// neutral working title and an original geisha roster. 1 human plays against 1
// AI opponent — the round's hidden hands and face-down Secret card rule out a
// meaningful hot-seat mode. Each round both players spend four one-shot action
// markers (Secret, Trade-off, Gift, Competition) to steer item cards toward
// the seven geishas; whoever holds more items of a geisha's type wins her
// favor at round end. Favor markers persist across rounds; the match is won by
// the favor of 4+ geishas or 11+ charm points.
//
// Pure game logic (deck, round engine, scoring, favor, AI) lives in the
// geishorna/game package with no SDK dependency and is unit-tested; this file
// and ui.go handle rendering and input.
package main

import (
	"image"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	ink "github.com/dennwc/inkview"

	"geishorna/game"
)

type screen int

const (
	screenSplash screen = iota
	screenMenu
	screenGame
	screenRules
)

// uiMode is the current sub-state of the human's interaction on the game
// screen.
type uiMode int

const (
	modeAction   uiMode = iota // choosing which action marker to spend
	modeSelect                 // an action is chosen; selecting the cards for it
	modeCompPair               // four cards chosen for Competition; choosing the split
	modeChoose                 // responding to the AI's open Gift/Competition offer
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

	mode      uiMode
	selAction game.Action // the action chosen in modeSelect/modeCompPair
	sel       []int       // selected hand indices (in tap order)
	rejected  bool        // last input was illegal; show a hint

	updates int
	aiPend  bool
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
		DrawSplash(screenSize, a.fonts, "Geishorna", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// Run the AI only AFTER this frame is shown, so the player's own move
		// lands first — same deferred-AI pattern as expeditionen/sushi. The
		// loop drives the AI through any chain (e.g. resolving the human's
		// Gift, then taking its own turn) until it is the human's move again.
		if a.aiPend {
			a.aiPend = false
			for a.gs.AITurn() {
				a.gs.StepAI()
				a.updates++
			}
			a.afterState()
			ink.Repaint()
		}
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Geishorna", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	a.rects = DrawGameScreen(&a.layout, a)
	a.navButtons = DrawButtonBar(a.layout.NavBar, []string{"Ny", "Meny"}, a.fonts)

	if a.gs.Phase != game.PhaseAction || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
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
	a.updates = 0
	a.resetInput()
	a.afterState()
	ink.Repaint()
}

func (a *app) resetInput() {
	a.mode = modeAction
	a.sel = a.sel[:0]
	a.rejected = false
}

// afterState re-syncs the human's input mode to the current game state and
// queues the AI if it is the AI's move. Call after any state mutation.
func (a *app) afterState() {
	if a.gs.Phase != game.PhaseAction {
		return
	}
	if a.gs.AITurn() {
		a.aiPend = true
		return
	}
	// It is the human's responsibility now.
	a.sel = a.sel[:0]
	a.rejected = false
	if a.gs.Pending != nil {
		a.mode = modeChoose
	} else {
		a.mode = modeAction
	}
}

func (a *app) tapGame(p image.Point) bool {
	for _, b := range a.navButtons {
		if b.Hit(p) {
			return a.handleNavButton(b.Label)
		}
	}

	switch a.gs.Phase {
	case game.PhaseRoundEnd:
		if p.In(a.rects.continueBtn) {
			a.gs.Continue()
			a.updates = 0
			a.resetInput()
			a.afterState()
			ink.Repaint()
			return true
		}
		return false
	case game.PhaseMatchEnd:
		return false
	}

	if !a.gs.HumanTurn() {
		return false
	}

	switch a.mode {
	case modeAction:
		return a.tapActionButtons(p)
	case modeSelect:
		return a.tapSelect(p)
	case modeCompPair:
		return a.tapCompPair(p)
	case modeChoose:
		return a.tapChoose(p)
	}
	return false
}

// tapActionButtons handles picking which action marker to spend.
func (a *app) tapActionButtons(p image.Point) bool {
	for i, r := range a.rects.actionBtns {
		if !r.live {
			continue
		}
		if p.In(r.rect) {
			a.selAction = game.Action(i)
			a.mode = modeSelect
			a.sel = a.sel[:0]
			a.rejected = false
			ink.Repaint()
			return true
		}
	}
	return false
}

// tapSelect handles selecting cards for the chosen action, plus Confirm/Cancel.
func (a *app) tapSelect(p image.Point) bool {
	if p.In(a.rects.cancelBtn) {
		a.resetInput()
		ink.Repaint()
		return true
	}
	for i, r := range a.rects.handCards {
		if p.In(r) {
			a.toggleSel(i)
			a.rejected = false
			ink.Repaint()
			return true
		}
	}
	need := a.selAction.Cards()
	if a.selAction == game.Competition {
		// For Competition, selecting 4 cards moves on to the pairing step.
		if len(a.sel) == need && a.rects.confirmLive && p.In(a.rects.confirmBtn) {
			a.mode = modeCompPair
			ink.Repaint()
			return true
		}
		return false
	}
	if len(a.sel) == need && a.rects.confirmLive && p.In(a.rects.confirmBtn) {
		return a.commitSimpleAction()
	}
	return false
}

func (a *app) toggleSel(i int) {
	for k, v := range a.sel {
		if v == i {
			a.sel = append(a.sel[:k], a.sel[k+1:]...)
			return
		}
	}
	if len(a.sel) < a.selAction.Cards() {
		a.sel = append(a.sel, i)
	}
}

// commitSimpleAction performs Secret / Trade-off / Gift once the right number
// of cards is selected.
func (a *app) commitSimpleAction() bool {
	var err error
	switch a.selAction {
	case game.Secret:
		err = a.gs.DoSecret(game.HumanIdx, a.sel[0])
	case game.TradeOff:
		err = a.gs.DoTradeOff(game.HumanIdx, append([]int(nil), a.sel...))
	case game.Gift:
		err = a.gs.DoGift(game.HumanIdx, append([]int(nil), a.sel...))
	}
	if err != nil {
		a.rejected = true
		ink.Repaint()
		return true
	}
	a.updates++
	a.afterState()
	ink.Repaint()
	return true
}

// tapCompPair handles choosing one of the three ways to split the four
// selected Competition cards into two pairs. The four cards are already
// committed to Competition at this point, so the only choice is the split.
func (a *app) tapCompPair(p image.Point) bool {
	for i, r := range a.rects.pairBtns {
		if p.In(r) {
			sp := compSplits[i]
			groupA := []int{a.sel[sp[0][0]], a.sel[sp[0][1]]}
			groupB := []int{a.sel[sp[1][0]], a.sel[sp[1][1]]}
			if err := a.gs.DoCompetition(game.HumanIdx, groupA, groupB); err != nil {
				a.rejected = true
				a.mode = modeSelect
				ink.Repaint()
				return true
			}
			a.updates++
			a.afterState()
			ink.Repaint()
			return true
		}
	}
	return false
}

// tapChoose handles the human picking from the AI's open offer.
func (a *app) tapChoose(p image.Point) bool {
	pend := a.gs.Pending
	if pend == nil {
		return false
	}
	if pend.Action == game.Gift {
		for i, r := range a.rects.offerCards {
			if p.In(r) {
				if err := a.gs.ResolveGift(game.HumanIdx, i); err != nil {
					return false
				}
				a.updates++
				a.afterState()
				ink.Repaint()
				return true
			}
		}
	} else {
		for i, r := range a.rects.offerGroups {
			if p.In(r) {
				if err := a.gs.ResolveCompetition(game.HumanIdx, i); err != nil {
					return false
				}
				a.updates++
				a.afterState()
				ink.Repaint()
				return true
			}
		}
	}
	return false
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
