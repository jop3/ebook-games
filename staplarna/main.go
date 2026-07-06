// Command staplarna is Staplarna ("The Stacks", based on TZAAR, Kris Burm /
// the GIPF project) for the PocketBook Verse Pro (PB634), built on the
// dennwc/inkview SDK.
//
// Two players first alternate freely placing their own 30 pieces (6 Tzaar, 9
// Tzarra, 15 Tott) anywhere on an empty 61-cell hex board, then take turns
// moving one of their stacks in a straight line EXACTLY as many cells as the
// stack is tall. Landing on an enemy stack no taller than yours captures it
// whole — including any pieces buried underneath its top. Landing on your own
// stack merges the two into one taller stack. A side that loses every piece
// of any ONE type (not total piece count), or that has no legal move on its
// turn, loses the game. Play hot-seat against a friend or against a built-in
// alpha-beta AI.
//
// Pure game logic (hex geometry, setup placement, stack movement/capture, win
// detection, AI) lives in the staplarna/game package with no SDK dependency
// and is unit-tested; this file and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"staplarna/game"
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

	gs      *game.GameState
	layout  Layout
	buttons []Button
	updates int
	aiPend  bool // an AI action is queued to run on the next Draw

	setupType  game.PieceType // the type the human will place next during PhaseSetup
	setupChips []Chip         // tappable type-selector chips, set by the last DrawSetupSelector

	hasSelection bool
	selected     game.Point

	rulesBack image.Rectangle // back-button rect on the rules screen
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
		DrawSplash(screenSize, a.fonts, "Staplarna", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn (a setup placement or a play-phase move), act
		// AFTER this frame is shown so the player sees their own action land
		// first, then trigger a redraw.
		if a.aiPend {
			a.aiPend = false
			if a.gs.StepAI() {
				a.updates++
				a.resetSetupType()
				if a.gs.AITurn() {
					a.aiPend = true
				}
				ink.Repaint()
			}
		}
	case screenRules:
		a.rulesBack = DrawRules(screenSize, a.fonts, "Staplarna", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	DrawStatus(&a.layout, a, a.fonts)
	a.setupChips = DrawSetupSelector(&a.layout, a)
	DrawBoard(&a.layout, a)
	a.buttons = DrawButtonBar(&a.layout, []string{"Ny", "Meny"}, a.fonts)

	if a.gs.Phase == game.PhaseDone || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func sideName(s game.Side) string {
	if s == game.White {
		return "Vit"
	}
	return "Svart"
}

func (a *app) statusText() string {
	s := a.gs
	if s.Phase == game.PhaseDone {
		switch s.Winner() {
		case game.Black:
			return "Svart vann!"
		case game.White:
			return "Vit vann!"
		default:
			return "Spelet slut."
		}
	}
	turn := sideName(s.Turn)
	if s.AITurn() {
		turn += " (dator)"
	}
	if s.Phase == game.PhaseSetup {
		return turn + " placerar   ·   " + itoa(s.PlacedCount()) + "/" + itoa(2*game.TotalPerSide) + " utplacerade"
	}
	return turn + " drar"
}

// materialText renders a compact per-side, per-type remaining-on-board
// summary (Tz/Za/To abbreviations for Tzaar/Tzarra/Tott) — useful during play
// since losing the last piece of any ONE type ends the game immediately.
func (a *app) materialText() string {
	b := a.gs.Board
	side := func(s game.Side) string {
		return "Tz" + itoa(b.TypeCount(s, game.Tzaar)) +
			" Za" + itoa(b.TypeCount(s, game.Tzarra)) +
			" To" + itoa(b.TypeCount(s, game.Tott))
	}
	return "Svart " + side(game.Black) + "   ·   Vit " + side(game.White)
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
		a.startGame(choice.opponent, choice.aiDepth)
		return true
	}
	return false
}

func (a *app) startGame(opp game.Opponent, aiDepth int) {
	a.gs = game.NewGame(opp, aiDepth)
	a.screen = screenGame
	a.updates = 0
	a.hasSelection = false
	a.resetSetupType()
	a.aiPend = a.gs.AITurn()
	ink.Repaint()
}

// resetSetupType points the setup-phase type selector at the current turn's
// scarcest still-available type (AllTypes is scarcest-first) whenever the
// turn changes — a sensible default the player can override at any time by
// tapping a different chip before placing.
func (a *app) resetSetupType() {
	if a.gs == nil || a.gs.Phase != game.PhaseSetup {
		return
	}
	if avail := a.gs.AvailableTypes(a.gs.Turn); len(avail) > 0 {
		a.setupType = avail[0]
	}
}

func (a *app) tapGame(p image.Point) bool {
	for _, b := range a.buttons {
		if b.Hit(p) {
			return a.handleButton(b.Label)
		}
	}
	if a.gs.Phase == game.PhaseDone || a.gs.AITurn() {
		return false
	}
	switch a.gs.Phase {
	case game.PhaseSetup:
		return a.tapSetup(p)
	case game.PhasePlaying:
		return a.tapMove(p)
	}
	return false
}

func (a *app) tapSetup(p image.Point) bool {
	for _, c := range a.setupChips {
		if p.In(c.Rect) {
			a.setupType = c.Type
			ink.Repaint()
			return true
		}
	}
	pt, ok := a.layout.PointAt(p)
	if !ok {
		return false
	}
	if a.gs.PlacePiece(a.setupType, pt) {
		a.updates++
		a.resetSetupType()
		a.checkAIPend()
		ink.Repaint()
		return true
	}
	return false
}

func (a *app) tapMove(p image.Point) bool {
	pt, ok := a.layout.PointAt(p)
	if !ok {
		return false
	}
	if a.hasSelection {
		if pt == a.selected {
			a.hasSelection = false // tap the selected stack again to deselect
			ink.Repaint()
			return true
		}
		// Try the move FIRST: landing on one of the mover's own stacks is a
		// legal merge in TZAAR, not just a "reselect" gesture -- checking
		// "is this an own stack" before attempting Play would make merges
		// unreachable by tap entirely. Only fall back to "switch selection to
		// this other own piece" once Play confirms the destination is not
		// actually a legal move for the current selection.
		if a.gs.Play(a.selected, pt) {
			a.hasSelection = false
			a.updates++
			a.checkAIPend()
			ink.Repaint()
			return true
		}
		if st, occ := a.gs.Board.At(pt); occ && st.Owner == a.gs.Turn {
			a.selected = pt // not a legal destination -- switch selection instead
			ink.Repaint()
			return true
		}
		return false
	}
	if st, occ := a.gs.Board.At(pt); occ && st.Owner == a.gs.Turn {
		a.selected = pt
		a.hasSelection = true
		ink.Repaint()
		return true
	}
	return false
}

// checkAIPend queues an AI step if it is now the AI's turn (used after every
// human action that might hand control to the AI: a setup placement or a
// play-phase move).
func (a *app) checkAIPend() {
	if a.gs.AITurn() {
		a.aiPend = true
	}
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Ny":
		a.startGame(a.gs.Opponent, a.gs.AIDepth)
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
