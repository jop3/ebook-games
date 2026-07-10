// Command juvelerna is Juvelerna ("The Gems"), a 2-player engine-building
// game for the PocketBook Verse Pro (PB634), built on the dennwc/inkview
// SDK. Based on Splendor (Marc André, Space Cowboys), reimplemented here
// with original card/noble values and a neutral working title.
//
// Players collect gem tokens and spend them (plus permanent per-card
// discounts) to buy development cards, racing to 15 prestige. Every game
// state (the token bank, both players' cards/tokens/reserves, the face-up
// tableau, the nobles) is fully visible — a perfect-information
// engine-builder — so hot-seat and vs-AI both play on one shared screen.
//
// Pure game logic (bank/tableau/nobles, the 4 turn actions, AI) lives in
// the juvelerna/game package with no SDK dependency and is unit-tested;
// this file and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"juvelerna/game"
)

type screen int

const (
	screenSplash screen = iota
	screenMenu
	screenGame
	screenRules
)

// selKind is the shape of the UI-only "in progress action" selection.
type selKind int

const (
	selNone       selKind = iota
	selBankColors         // building a take-2/take-3 token selection
	selCard               // a tableau card, or one of the active player's own
	// reserved cards, chosen for reserve/buy
	selDeck // a tier's face-down deck, chosen for a blind reserve
)

// selection is the UI-only "action in progress" state for the 2-tap flow:
// tap a source (bank colors / a card / a deck), then tap a confirming
// button (Ta / Reservera / Köp). Nothing is applied to game.GameState until
// that confirm succeeds. Mirrors mosaik's `selection` struct.
type selection struct {
	kind         selKind
	bankColors   []game.Color // up to 3 for selBankColors (a repeat = take-2)
	tier, slot   int          // selCard (tableau) or selDeck
	fromReserved bool         // selCard: true if it's the active player's own reserved card
	reservedIdx  int
}

// complete reports whether the current bank-color selection is a legal
// take-3 (3 distinct) or take-2 (2 identical) shape, ready to confirm.
func (s selection) complete() bool {
	if s.kind != selBankColors {
		return false
	}
	switch len(s.bankColors) {
	case 2:
		return s.bankColors[0] == s.bankColors[1]
	case 3:
		return true
	}
	return false
}

type app struct {
	fonts  *Fonts
	screen screen
	menu   *Menu

	gs      *game.GameState
	layout  Layout
	buttons []Button
	updates int
	aiPend  bool // an AI step is queued to run on the next Draw

	sel     selection
	showOpp bool // "Visa motståndare": show the OTHER player's board full-size, read-only
	msg     string

	rulesBack image.Rectangle
}

func main() {
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}
	if err := ink.Run(&app{}); err != nil {
		panic(err)
	}
}

// --- ink.App -----------------------------------------------------------

func (a *app) Init() error {
	a.fonts = InitFonts()
	a.menu = NewMenu()
	a.screen = screenSplash
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
		DrawSplash(screenSize, a.fonts, "Juvelerna", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn (main action OR a pending noble
		// choice/discard sub-phase), compute its step AFTER this frame is
		// shown so the player sees their own move land first, then trigger
		// a redraw — the same aiPend-after-paint pattern as hasami/mosaik.
		if a.aiPend {
			a.aiPend = false
			if a.gs.StepAI() {
				a.updates++
				if a.gs.AITurn() {
					a.aiPend = true
				}
				ink.Repaint()
			}
		} else if !a.gs.AITurn() && a.gs.CanPass() {
			// Official-rules escape hatch: a player with NO legal action
			// passes. The AI gets this via LegalActions; for the human we
			// auto-pass with a message so a turn can never deadlock.
			if a.gs.Pass() {
				a.msg = "Inga drag möjliga — du passar"
				a.finishStep()
			}
		}
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Juvelerna", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.gs, a.fonts, a.msg)
	DrawNobleRow(&a.layout, a.gs, a.fonts)
	DrawBankRow(&a.layout, a.gs, a.sel)
	DrawTableau(&a.layout, a.gs, a.sel)

	fullSide := a.fullBoardSide()
	compactSide := 1 - fullSide
	interactive := !a.showOpp && a.gs.Phase != game.PhaseDone
	DrawActivePanel(&a.layout, a.gs, fullSide, a.fonts, a.sel,
		interactive && a.gs.Phase == game.PhaseDiscard,
		interactive && a.gs.Phase == game.PhasePlaying)
	DrawOpponentStrip(&a.layout, a.gs, compactSide, a.showOpp, a.fonts)

	if a.gs.Phase == game.PhaseDone {
		DrawWinner(&a.layout, a.gs, a.fonts)
		a.buttons = DrawButtonBar(&a.layout, []string{"Ny", "Meny"}, a.fonts)
	} else {
		a.buttons = DrawButtonBar(&a.layout, a.contextButtons(), a.fonts)
	}

	if a.gs.Phase == game.PhaseDone || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

// contextButtons returns the button-bar labels for the current selection
// (kept to 2-3 contextual actions plus the always-present "Meny", per the
// gamedev guide's "3-4 buttons max" rule).
func (a *app) contextButtons() []string {
	if a.showOpp || a.gs.Phase != game.PhasePlaying {
		return []string{"Meny"}
	}
	switch a.sel.kind {
	case selBankColors:
		return []string{"Ta", "Rensa", "Meny"}
	case selCard:
		if a.sel.fromReserved {
			return []string{"Köp", "Rensa", "Meny"}
		}
		return []string{"Reservera", "Köp", "Rensa", "Meny"}
	case selDeck:
		return []string{"Reservera", "Rensa", "Meny"}
	}
	return []string{"Ny", "Meny"}
}

// fullBoardSide is whichever player's board is shown full-size: normally
// the side to move, or — while "Visa motståndare" is toggled on — the
// other side (shown read-only).
func (a *app) fullBoardSide() int {
	if a.showOpp {
		return 1 - a.gs.Turn
	}
	return a.gs.Turn
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
	if choice, ok := a.menu.HandleTouch(p); ok {
		a.startGame(choice.mode, choice.aiLevel)
		return true
	}
	return false
}

func (a *app) startGame(mode game.Mode, aiLevel int) {
	a.gs = game.NewGame(mode, aiLevel)
	a.screen = screenGame
	a.updates = 0
	a.aiPend = false
	a.sel = selection{}
	a.showOpp = false
	a.msg = ""
	ink.Repaint()
}

func (a *app) tapGame(p image.Point) bool {
	for _, b := range a.buttons {
		if b.Hit(p) {
			return a.handleButton(b.Label)
		}
	}
	// The opponent-toggle button is always live (even the "Ny"/"Meny"-only
	// button bar on the winner screen still shows the compact strip).
	if p.In(a.layout.ShowOppBtn) {
		a.showOpp = !a.showOpp
		a.sel = selection{}
		ink.Repaint()
		return true
	}
	if a.showOpp || a.gs.AITurn() {
		return false // read-only view, or not a human's turn to act
	}
	switch a.gs.Phase {
	case game.PhasePlaying:
		return a.tapPlaying(p)
	case game.PhaseNobleChoice:
		return a.tapNobleChoice(p)
	case game.PhaseDiscard:
		return a.tapDiscard(p)
	}
	return false
}

func (a *app) tapNobleChoice(p image.Point) bool {
	if idx, ok := a.layout.HitNoble(p); ok {
		if a.gs.ChooseNoble(idx) {
			a.msg = ""
			a.finishStep()
			return true
		}
	}
	return false
}

func (a *app) tapDiscard(p image.Point) bool {
	if c, isGold, ok := a.layout.HitDiscard(p); ok {
		var applied bool
		if isGold {
			applied = a.gs.DiscardGold()
		} else {
			applied = a.gs.DiscardColor(c)
		}
		if applied {
			a.msg = ""
			a.finishStep()
			return true
		}
	}
	return false
}

func (a *app) tapPlaying(p image.Point) bool {
	// 1) Tap a bank color pile: build/extend the take-2/take-3 selection.
	if c, ok := a.layout.HitBank(p); ok {
		a.tapBankColor(c)
		ink.Repaint()
		return true
	}
	// 2) Tap a tableau card: select it (clears any other selection).
	if tier, slot, ok := a.layout.HitTableau(p); ok {
		if a.sel.kind == selCard && !a.sel.fromReserved && a.sel.tier == tier && a.sel.slot == slot {
			a.sel = selection{} // tap again to deselect
		} else {
			a.sel = selection{kind: selCard, tier: tier, slot: slot}
		}
		a.msg = ""
		ink.Repaint()
		return true
	}
	// 3) Tap a tier's face-down deck: select it for a blind reserve.
	if tier, ok := a.layout.HitDeck(p); ok {
		if a.sel.kind == selDeck && a.sel.tier == tier {
			a.sel = selection{}
		} else {
			a.sel = selection{kind: selDeck, tier: tier}
		}
		a.msg = ""
		ink.Repaint()
		return true
	}
	// 4) Tap one of the active player's own reserved cards.
	if idx, ok := a.layout.HitReserved(p); ok {
		if a.sel.kind == selCard && a.sel.fromReserved && a.sel.reservedIdx == idx {
			a.sel = selection{}
		} else {
			a.sel = selection{kind: selCard, fromReserved: true, reservedIdx: idx}
		}
		a.msg = ""
		ink.Repaint()
		return true
	}
	return false
}

// tapBankColor extends the in-progress take-2/take-3 selection with color c,
// per the rule: tapping the SAME color a 2nd time proposes a take-2;
// tapping up to 2 more DISTINCT colors builds a take-3. A completed
// selection (2 same or 3 distinct) ignores further bank taps until
// confirmed or cleared.
func (a *app) tapBankColor(c game.Color) {
	if a.sel.kind != selBankColors {
		a.sel = selection{kind: selBankColors, bankColors: []game.Color{c}}
		return
	}
	if a.sel.complete() {
		return // already a full take-2/take-3 shape; must confirm or clear
	}
	switch len(a.sel.bankColors) {
	case 1:
		a.sel.bankColors = append(a.sel.bankColors, c) // same color -> take-2 candidate; different -> take-3 building
	case 2:
		if a.sel.bankColors[0] == c || a.sel.bankColors[1] == c {
			return // can't mix a take-2 pair with a 3rd different color
		}
		a.sel.bankColors = append(a.sel.bankColors, c)
	}
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Ta":
		return a.confirmTake()
	case "Reservera":
		return a.confirmReserve()
	case "Köp":
		return a.confirmBuy()
	case "Rensa":
		a.sel = selection{}
		a.msg = ""
		ink.Repaint()
		return true
	case "Ny":
		a.startGame(a.gs.Mode, a.gs.AILevel)
		return true
	case "Meny":
		a.screen = screenMenu
		ink.Repaint()
		return true
	}
	return false
}

func (a *app) confirmTake() bool {
	if a.sel.kind != selBankColors || len(a.sel.bankColors) == 0 {
		return false
	}
	var ok bool
	if len(a.sel.bankColors) == 2 && a.sel.bankColors[0] == a.sel.bankColors[1] {
		ok = a.gs.Take2(a.sel.bankColors[0])
	} else {
		// 3 distinct colors, or fewer when the bank has run dry (the
		// official take-fewer fallback) — TakeColors enforces that taking
		// fewer than the bank can supply stays illegal.
		colors := make([]game.Color, len(a.sel.bankColors))
		copy(colors, a.sel.bankColors)
		ok = a.gs.TakeColors(colors)
	}
	return a.finishAction(ok, "Ogiltigt drag")
}

func (a *app) confirmReserve() bool {
	var ok bool
	switch a.sel.kind {
	case selCard:
		if a.sel.fromReserved {
			return false
		}
		ok = a.gs.ReserveTableau(a.sel.tier, a.sel.slot)
	case selDeck:
		ok = a.gs.ReserveBlind(a.sel.tier)
	default:
		return false
	}
	return a.finishAction(ok, "Kan inte reservera (max 3, eller inget kort kvar)")
}

func (a *app) confirmBuy() bool {
	if a.sel.kind != selCard {
		return false
	}
	var ok bool
	if a.sel.fromReserved {
		ok = a.gs.BuyReserved(a.sel.reservedIdx)
	} else {
		ok = a.gs.BuyTableau(a.sel.tier, a.sel.slot)
	}
	return a.finishAction(ok, "Har inte råd med det kortet")
}

// finishAction applies the outcome of a confirmed action: on success it
// clears the selection and advances (queuing the AI if needed); on failure
// it surfaces a Swedish hint message and leaves the selection as-is so the
// player can try a different confirm.
func (a *app) finishAction(ok bool, failMsg string) bool {
	if !ok {
		a.msg = failMsg
		ink.Repaint()
		return false
	}
	a.sel = selection{}
	a.msg = ""
	a.finishStep()
	return true
}

// finishStep is called after ANY successful game-state mutation (a main
// action, a noble choice, or a discard) to bump the redraw counter and, if
// the turn has now landed on the AI, queue its next step.
func (a *app) finishStep() {
	a.updates++
	if a.gs.AITurn() {
		a.aiPend = true
	}
	ink.Repaint()
}

func (a *app) Orientation(o ink.Orientation) bool {
	ink.Repaint()
	return true
}
