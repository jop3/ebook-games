// Command nim is the Nim (matchsticks) game for the PocketBook Verse Pro
// (PB634), built on the dennwc/inkview SDK. Two-player hot-seat or solo against
// a perfect Sprague-Grundy (nim-sum) AI, in Normal and Misère variants.
package main

import (
	"image"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	ink "github.com/dennwc/inkview"

	"nim/game"
)

// screen is the top-level mode of the app.
type screen int

const (
	screenSplash screen = iota // shown on launch; any tap → menu
	screenMenu
	screenGame
	screenRules // reached from the menu "Regler" button
)

type app struct {
	fonts  *Fonts
	screen screen
	menu   *Menu

	gs     *game.GameState
	layout Layout
	rng    *rand.Rand

	selPile  int // currently selected pile, -1 = none
	selCount int // sticks marked for removal

	status    string // transient status line under the header
	aiPending bool   // AI must move after this Draw (guide §6)
	moveCount int    // moves since last FullUpdate (e-ink ghosting control)

	rulesBack image.Rectangle // "Tillbaka" button rect on the rules screen
}

func main() {
	// The device launches apps with cwd = FS root; relocate to the binary dir
	// (matches the other working games).
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}
	if err := ink.Run(&app{}); err != nil {
		panic(err)
	}
}

// --- ink.App ---

func (a *app) Init() error {
	a.fonts = InitFonts()
	a.menu = NewMenu()
	a.screen = screenSplash
	a.selPile = -1
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

func (a *app) Draw() {
	size := ink.ScreenSize()
	size.Y = usableH // real drawable height is ~1340, not the reported 1448 (guide §5)
	switch a.screen {
	case screenSplash:
		DrawSplash(size, a.fonts, "NIM", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(size, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.layout = DrawGame(size, a.fonts, a.gs, a.selPile, a.selCount, a.status, false)
		// Full refresh each frame to keep the e-ink board clean (few frames/move).
		ink.FullUpdate()
	case screenRules:
		a.rulesBack = DrawRules(size, a.fonts, "NIM — Regler", rulesParagraphs)
		ink.FullUpdate()
	}

	// After the player's move is on screen, let the AI think (guide §6).
	if a.aiPending {
		a.aiPending = false
		a.runAI()
	}
}

// --- input ---

func (a *app) Pointer(e ink.PointerEvent) bool {
	if e.State != ink.PointerUp {
		return false
	}
	return a.handleTap(e.Point)
}

func (a *app) Touch(e ink.TouchEvent) bool { // fallback
	if e.State != ink.TouchUp {
		return false
	}
	return a.handleTap(e.Point)
}

func (a *app) Key(e ink.KeyEvent) bool {
	if e.State != ink.KeyStateUp {
		return false
	}
	if e.Key == ink.KeyBack {
		if a.screen == screenGame || a.screen == screenRules {
			a.screen = screenMenu
			ink.Repaint()
			return true
		}
	}
	return false
}

func (a *app) Orientation(o ink.Orientation) bool { return false }

// handleTap dispatches a tap by current screen.
func (a *app) handleTap(p image.Point) bool {
	switch a.screen {
	case screenSplash:
		// Any tap advances to the menu.
		a.screen = screenMenu
		ink.Repaint()
		return true
	case screenMenu:
		return a.handleMenuTap(p)
	case screenGame:
		return a.handleGameTap(p)
	case screenRules:
		if p.In(a.rulesBack) {
			a.screen = screenMenu
			ink.Repaint()
			return true
		}
	}
	return false
}

func (a *app) handleMenuTap(p image.Point) bool {
	for _, b := range a.menu.Buttons() {
		if !b.Hit(p) {
			continue
		}
		switch b.ID {
		case "variant":
			if a.menu.Variant == game.Normal {
				a.menu.Variant = game.Misere
			} else {
				a.menu.Variant = game.Normal
			}
		case "mode":
			if a.menu.Mode == game.SoloAI {
				a.menu.Mode = game.TwoPlayer
			} else {
				a.menu.Mode = game.SoloAI
			}
		case "preset":
			a.menu.PresetIdx = (a.menu.PresetIdx + 1) % len(game.Presets)
		case "start":
			a.startGame()
		case "rules":
			a.screen = screenRules
		}
		ink.Repaint()
		return true
	}
	return false
}

// startGame builds a fresh GameState from the menu selections.
func (a *app) startGame() {
	preset := game.Presets[a.menu.PresetIdx]
	piles := preset.Piles
	if piles == nil { // "Slumpad"
		piles = game.RandomPiles(a.rng)
	}
	a.gs = game.NewGame(piles, a.menu.Variant, a.menu.Mode)
	a.selPile = -1
	a.selCount = 0
	a.status = ""
	a.moveCount = 0
	a.screen = screenGame
}

func (a *app) handleGameTap(p image.Point) bool {
	if a.gs == nil {
		return false
	}
	// Menu button always available.
	if a.layout.MenuBtn.Hit(p) {
		a.screen = screenMenu
		ink.Repaint()
		return true
	}
	if a.gs.Over {
		// Any tap (besides menu) restarts with same settings.
		a.startGame()
		ink.Repaint()
		return true
	}
	// Block input while it's the AI's turn.
	if a.gs.Mode == game.SoloAI && a.gs.Turn == 1 {
		return false
	}

	// Pile selection.
	for i, r := range a.layout.PileRects {
		if p.In(r) && a.gs.Piles[i] > 0 {
			a.selPile = i
			a.selCount = 1 // default take one
			a.status = ""
			ink.Repaint()
			return true
		}
	}

	// Minus / Plus adjust the count for the selected pile.
	if a.layout.Minus.Hit(p) && a.selPile >= 0 {
		if a.selCount > 1 {
			a.selCount--
			ink.Repaint()
		}
		return true
	}
	if a.layout.Plus.Hit(p) && a.selPile >= 0 {
		if a.selCount < a.gs.Piles[a.selPile] {
			a.selCount++
			ink.Repaint()
		}
		return true
	}

	// Confirm.
	if a.layout.Confirm.Hit(p) && a.selPile >= 0 {
		a.commitMove(game.Move{Pile: a.selPile, Count: a.selCount})
		return true
	}
	return false
}

// commitMove applies the human move, then (solo) schedules the AI to move after
// the player's move is drawn (guide §6: set aiPending, repaint, compute later).
func (a *app) commitMove(m game.Move) {
	if err := a.gs.Validate(m); err != nil {
		return
	}
	a.gs.ApplyMove(m)
	a.selPile = -1
	a.selCount = 0
	a.moveCount++
	if !a.gs.Over && a.gs.Mode == game.SoloAI && a.gs.Turn == 1 {
		a.aiPending = true
		a.status = "AI tänker..."
	}
	ink.Repaint()
}

// runAI computes and applies the AI's move (called from Draw after the player's
// move has been rendered).
func (a *app) runAI() {
	if a.gs == nil || a.gs.Over || a.gs.Turn != 1 {
		return
	}
	m, ok := a.gs.BestMove()
	if !ok {
		return
	}
	a.gs.ApplyMove(m)
	a.status = ""
	a.selPile = -1
	a.selCount = 0
	a.moveCount++
	ink.Repaint()
}
