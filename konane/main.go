// Command konane is Konane ("stone jumping"), the traditional Hawaiian
// jump-capture game, for the PocketBook Verse Pro (PB634), built on the
// dennwc/inkview SDK.
//
// The board is 8x8, completely filled with alternating black/white stones —
// a full checkerboard, with no empty squares to start. A one-time opening
// removes exactly two stones (Black removes one of the two center stones,
// then White removes one of its own stones adjacent to the resulting gap).
// From then on the ONLY move in the game is a jump: move a stone orthogonally
// over an adjacent enemy stone into the empty cell beyond, removing it. A
// single turn may chain several jumps with the same stone, stopping whenever
// the player chooses. A side with zero legal jumps on its turn loses at
// once — there is no other way to end the game, and no pass.
//
// Pure game logic (board, opening, jumps/chains, win condition, AI) lives in
// the konane/game package with no SDK dependency and is unit-tested; this
// file and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"konane/game"
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

	// hasSelection/selected track the UI's pending "which stone did the
	// player tap to start a jump with" state, before the first jump of a
	// turn. Once a jump is made, the game package's own ChainActive/ChainFrom
	// take over (a chain can only ever continue with the same stone, which
	// is a rule the game package enforces, not just a UI convenience).
	hasSelection bool
	selected     image.Point

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

// --- ink.App ---------------------------------------------------------------

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
		DrawSplash(screenSize, a.fonts, "Konane", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn (an opening removal or a full play-phase
		// move), act AFTER this frame is shown so the player sees their own
		// move land first, then trigger a redraw.
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
		a.rulesBack = DrawRules(screenSize, a.fonts, "Konane", rulesParagraphs)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawBoard(&a.layout, a)
	labels := []string{"Ny", "Meny"}
	if a.gs.Phase == game.PhasePlaying && a.gs.ChainActive {
		labels = []string{"Klart", "Meny"}
	}
	a.buttons = DrawButtonBar(&a.layout, labels, a.fonts)

	if a.gs.Phase == game.PhaseDone || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

func (a *app) statusText() string {
	s := a.gs
	bl, wh := s.Board.Count(game.Black), s.Board.Count(game.White)
	score := "Svart " + itoa(bl) + " – Vit " + itoa(wh)
	switch s.Phase {
	case game.PhaseOpeningBlackRemove:
		return "Öppning: Svart tar bort en mittsten"
	case game.PhaseOpeningWhiteRemove:
		who := "Vit"
		if s.AITurn() {
			who = "Vit (dator)"
		}
		return "Öppning: " + who + " tar bort en sten intill luckan"
	case game.PhaseDone:
		switch s.Winner() {
		case game.Black:
			return "Svart vann!  " + score
		case game.White:
			return "Vit vann!  " + score
		default:
			return "Spelet slut.  " + score
		}
	default: // PhasePlaying
		turn := "Svart"
		if s.Turn == game.White {
			turn = "Vit"
		}
		if s.AITurn() {
			turn = "Vit (dator)"
		}
		suffix := ""
		if s.ChainActive {
			suffix = "   ·   Fortsätt hoppa eller tryck Klart"
		}
		return turn + " drar   ·   " + score + suffix
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
	a.aiPend = false
	a.hasSelection = false
	if a.gs.AITurn() {
		a.aiPend = true
	}
	ink.Repaint()
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
	case game.PhaseOpeningBlackRemove:
		x, y, ok := a.layout.ScreenToCell(p)
		if !ok {
			return false
		}
		if a.gs.RemoveOpeningBlack(image.Pt(x, y)) {
			a.updates++
			if a.gs.AITurn() {
				a.aiPend = true
			}
			ink.Repaint()
			return true
		}
		return false
	case game.PhaseOpeningWhiteRemove:
		x, y, ok := a.layout.ScreenToCell(p)
		if !ok {
			return false
		}
		if a.gs.RemoveOpeningWhite(image.Pt(x, y)) {
			a.updates++
			ink.Repaint()
			return true
		}
		return false
	case game.PhasePlaying:
		return a.tapPlaying(p)
	}
	return false
}

// tapPlaying drives the jump/chain flow: tap own stone with a legal jump ->
// tap a highlighted destination to jump -> keep tapping highlighted
// destinations to continue the chain, or hit "Klart" to stop early.
func (a *app) tapPlaying(p image.Point) bool {
	x, y, ok := a.layout.ScreenToCell(p)
	if !ok {
		return false
	}
	cell := image.Pt(x, y)

	if a.gs.ChainActive {
		if a.gs.ContinueJump(cell) {
			a.afterMove()
			return true
		}
		return false
	}

	if a.hasSelection {
		if cell == a.selected {
			a.hasSelection = false // tap the selected stone again to deselect
			ink.Repaint()
			return true
		}
		if a.gs.Board.At(x, y) == a.gs.Turn && len(a.gs.Board.LegalJumpsFrom(cell, a.gs.Turn)) > 0 {
			a.selected = cell // switch selection to another jumpable stone
			ink.Repaint()
			return true
		}
		if a.gs.StartJump(a.selected, cell) {
			a.hasSelection = false
			a.afterMove()
			return true
		}
		return false
	}

	if a.gs.Board.At(x, y) == a.gs.Turn && len(a.gs.Board.LegalJumpsFrom(cell, a.gs.Turn)) > 0 {
		a.selected = cell
		a.hasSelection = true
		ink.Repaint()
		return true
	}
	return false
}

// afterMove is called after any successful jump (start or continue): queue
// the AI if it's now their turn, bump the redraw counter, repaint.
func (a *app) afterMove() {
	a.updates++
	if a.gs.AITurn() {
		a.aiPend = true
	}
	ink.Repaint()
}

func (a *app) handleButton(label string) bool {
	switch label {
	case "Klart":
		if a.gs.EndChain() {
			a.afterMove()
			return true
		}
		return false
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
