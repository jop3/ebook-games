// Command irad is the "I rad" line-up game engine for the PocketBook
// Verse Pro (PB634), built on the dennwc/inkview SDK.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"irad/game"
	"irad/ui"
)

// screen is the top-level mode of the application.
type screen int

const (
	screenSplash screen = iota
	screenMenu
	screenGame
	screenRules
)

type app struct {
	fonts  *ui.Fonts
	screen screen

	menu *ui.Menu

	gs          *game.GameState
	layout      ui.Layout
	gameButtons []ui.Button // current in-game button hit regions
	moveCount   int         // moves since last FullUpdate
	aiPending   bool        // AI is thinking; status shows "AI tänker..."

	rulesBack image.Rectangle
}

func main() {
	// Match the working Black Box app exactly: relocate to the binary's dir
	// (the device starts apps with cwd = FS root).
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}
	if err := ink.Run(&app{}); err != nil {
		panic(err)
	}
}

// --- ink.App interface ---

func (a *app) Init() error {
	a.fonts = ui.InitFonts()
	a.menu = ui.NewMenu()
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

func (a *app) Draw() {
	screenSize := ink.ScreenSize()
	switch a.screen {
	case screenSplash:
		ui.DrawSplash(screenSize, a.fonts, "I rad", ui.SplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
	case screenRules:
		a.rulesBack = ui.DrawRules(screenSize, a.fonts, "I rad", ui.RulesParagraphs)
		ink.FullUpdate()
	}
}

// fullUpdateEvery forces a clean FullUpdate every N moves to clear the
// ghosting that repeated partial updates accumulate on e-ink (§10).
const fullUpdateEvery = 6

func (a *app) drawGame(screenSize image.Point) {
	a.layout = ui.NewLayout(screenSize, &a.gs.Board)
	ink.ClearScreen()
	ui.DrawStatus(&a.layout, a.statusText(), a.fonts)
	ui.DrawBoard(&a.layout, a.gs, a.fonts)
	a.drawButtons()

	// Quality/speed tradeoff: a full-screen partial update is fast and good
	// enough for normal moves; a periodic and end-of-game FullUpdate clears
	// accumulated ghosting.
	if a.gs.Phase == game.PhaseGameOver || a.moveCount == 0 || a.moveCount%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

// drawButtons renders the in-game button bar appropriate to the phase.
func (a *app) drawButtons() {
	labels := []string{"Spela igen", "Byt variant"}
	a.gameButtons = ui.DrawButtonBar(&a.layout, labels, a.fonts)
}

// playerName returns the display name of a seat, including its mark glyph so
// players can tell which symbol is theirs (X O △ □).
func playerName(p game.Player) string {
	switch p {
	case game.Player1:
		return "Spelare 1 (X)"
	case game.Player2:
		return "Spelare 2 (O)"
	case game.Player3:
		return "Spelare 3 (△)"
	case game.Player4:
		return "Spelare 4 (□)"
	default:
		return "Spelare"
	}
}

func (a *app) statusText() string {
	switch a.gs.Phase {
	case game.PhaseGameOver:
		if a.gs.Winner == game.PlayerNone {
			return "Oavgjort!"
		}
		if a.gs.VsAI {
			if a.gs.Winner == a.gs.AIPlayer {
				return "AI vann!"
			}
			return "Du vann!"
		}
		return playerName(a.gs.Winner) + " vann!"
	default:
		if a.aiPending {
			return "AI tänker..."
		}
		if a.gs.VsAI {
			if a.gs.Turn == a.gs.AIPlayer {
				return "AI tänker..."
			}
			return "Din tur"
		}
		verb := " placerar"
		if a.gs.Phase == game.PhaseMoving {
			verb = " flyttar"
		}
		return playerName(a.gs.Turn) + verb
	}
}

func (a *app) Key(e ink.KeyEvent) bool {
	// Side keys reserved for menu navigation only; ignored during play (§8).
	// Hardware Back returns to the menu from a game.
	if e.State == ink.KeyStateUp && e.Key == ink.KeyBack {
		if a.screen == screenGame || a.screen == screenRules {
			a.screen = screenMenu
			ink.Repaint()
			return true
		}
	}
	return false
}

// Pointer receives finger taps on PocketBook hardware. On these devices a
// normal single tap is delivered as EVT_POINTER* (not EVT_TOUCH*), so this is
// where interaction actually arrives. We act on PointerUp (finger lifted).
func (a *app) Pointer(e ink.PointerEvent) bool {
	if e.State != ink.PointerUp {
		return false
	}
	return a.handleTap(e.Point)
}

// Touch is kept as a fallback for any firmware that emits multitouch
// EVT_TOUCH* events instead of pointer events. It routes to the same logic.
func (a *app) Touch(e ink.TouchEvent) bool {
	if e.State != ink.TouchUp {
		return false
	}
	return a.handleTap(e.Point)
}

// handleTap dispatches a tap at point p to the active screen.
func (a *app) handleTap(p image.Point) bool {
	switch a.screen {
	case screenSplash:
		a.screen = screenMenu
		ink.Repaint()
		return true
	case screenMenu:
		return a.touchMenu(p)
	case screenGame:
		return a.touchGame(p)
	case screenRules:
		if p.In(a.rulesBack) {
			a.screen = screenMenu
			ink.Repaint()
			return true
		}
	}
	return false
}

func (a *app) touchMenu(p image.Point) bool {
	action := a.menu.HandleTouch(p)
	switch {
	case action.StartPreset != nil:
		a.startGame(*action.StartPreset)
		return true
	case action.OpenRules:
		a.screen = screenRules
		ink.Repaint()
		return true
	case action.Redraw:
		ink.Repaint()
		return true
	}
	return false
}

func (a *app) startGame(p game.Preset) {
	if a.menu.VsAI() {
		a.gs = game.NewGame(p, true) // 1 human + AI (two-player game)
	} else {
		a.gs = game.NewGameN(p, a.menu.NumPlayers())
	}
	a.screen = screenGame
	a.moveCount = 0
	a.aiPending = false
	ink.Repaint()
}

func (a *app) touchGame(p image.Point) bool {
	// Button bar first.
	for _, b := range a.gameButtons {
		if b.Hit(p) {
			switch b.Label {
			case "Spela igen":
				a.gs.Restart()
				a.moveCount = 0
				a.aiPending = false
				ink.Repaint()
				return true
			case "Byt variant":
				a.screen = screenMenu
				ink.Repaint()
				return true
			}
		}
	}

	if a.gs.Phase == game.PhaseGameOver {
		return false
	}
	// Block human input while it's the AI's turn.
	if a.gs.AITurn() {
		return false
	}

	res := ui.ResolveTouch(&a.layout, a.gs, p)
	switch {
	case res.HasMove:
		a.commitMove(res.Move)
		return true
	case res.Changed:
		a.gs.Selected = res.NewlySelected
		// Cheap local repaint of the board is fine; full redraw keeps code simple.
		ink.Repaint()
		return true
	}
	return false
}

// commitMove applies a human or post-AI move and advances the game, handling
// the periodic FullUpdate and scheduling the AI's reply if needed.
func (a *app) commitMove(m game.Move) {
	a.gs.ApplyMove(m)
	a.afterMove()
}

// afterMove handles redraw cadence and AI scheduling after any move.
func (a *app) afterMove() {
	a.moveCount++

	if a.gs.AITurn() {
		a.aiPending = true
	}
	ink.Repaint()

	// Compute the AI move synchronously, right here in the handler. The
	// heuristic is sub-second, so single-threaded and predictable beats a
	// deferred compute; by the time the queued repaint renders, the AI's
	// reply is already on the board.
	if a.aiPending {
		a.runAI()
	}
}

// runAI plays the AI's move (possibly several? no — one per turn) and clears
// the pending flag.
func (a *app) runAI() {
	if !a.gs.AITurn() {
		a.aiPending = false
		return
	}
	m, ok := game.BestMove(a.gs.Board, a.gs.Turn, a.gs.Phase)
	a.aiPending = false
	if !ok {
		// No legal move (e.g. stalemate in moving phase): treat as game over.
		a.gs.Phase = game.PhaseGameOver
		ink.Repaint()
		return
	}
	a.gs.ApplyMove(m)
	a.moveCount++
	ink.Repaint()
}

func (a *app) Orientation(o ink.Orientation) bool {
	// Recompute layout on next Draw; force a clean repaint.
	ink.Repaint()
	return true
}
