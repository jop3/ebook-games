// Command hertigen is Hertigen ("The Duke") for the PocketBook Verse Pro
// (PB634), built on the dennwc/inkview SDK.
//
// Two players face off on a 6x6 board. Every tile — including each side's
// Duke — is double-sided, with a printed move/jump/strike pattern on each
// face; acting flips the tile to its other face, so the same tile behaves
// differently next time it moves. Capture the enemy Duke to win. Recruit
// reserve troops onto empty squares next to your own Duke instead of moving.
// Play hot-seat against a friend or against a built-in alpha-beta AI.
//
// Pure game logic (board, tile patterns, actions, AI) lives in the
// hertigen/game package with no SDK dependency and is unit-tested; this file
// and ui.go handle rendering and input.
package main

import (
	"image"
	"os"
	"path/filepath"

	ink "github.com/dennwc/inkview"

	"hertigen/game"
)

type screen int

const (
	screenSplash screen = iota
	screenMenu
	screenGame
	screenRules
	screenLegend
)

// selectMode is the game screen's little tap-flow state machine: no
// selection, a board tile selected (showing its destinations/strikes), the
// Duke selected and a reserve troop type being chosen for recruiting, or a
// recruit type chosen and now waiting for a destination square tap.
type selectMode int

const (
	selNone selectMode = iota
	selTile
	selRecruitType
	selRecruitSquare
)

type app struct {
	fonts  *Fonts
	screen screen
	menu   *Menu

	gs      *game.GameState
	layout  Layout
	buttons []Button
	updates int
	aiPend  bool // an AI move is queued to run on the next Draw

	mode        selectMode
	selected    image.Point // valid whenever mode != selNone
	recruitType game.TileType
}

func main() {
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}
	if err := ink.Run(&app{}); err != nil {
		panic(err)
	}
}

// --- ink.App -----------------------------------------------------------------

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
		DrawSplash(screenSize, a.fonts, "Hertigen", drawSplashMotif)
		ink.FullUpdate()
	case screenMenu:
		a.menu.Draw(screenSize, a.fonts)
		ink.FullUpdate()
	case screenGame:
		a.drawGame(screenSize)
		// If it's the AI's turn, compute its move AFTER this frame is shown
		// so the player sees their own move land first, then trigger a
		// redraw.
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
		a.buttons = DrawRules(screenSize, a.fonts, "Hertigen", rulesParagraphs)
		ink.FullUpdate()
	case screenLegend:
		a.buttons = DrawLegend(screenSize, a.fonts)
		ink.FullUpdate()
	}
}

func (a *app) drawGame(screenSize image.Point) {
	a.layout = NewLayout(screenSize)
	ink.ClearScreen()
	DrawStatus(&a.layout, a.statusText(), a.fonts)
	DrawBoard(&a.layout, a)
	a.buttons = DrawButtonBar(&a.layout, a.currentButtonLabels(), a.fonts.Button)

	if a.gs.Phase == game.PhaseDone || a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(a.layout.Screen)
	}
}

// currentButtonLabels picks the button bar contents for the current
// selection state: the steady-state ["Ny","Meny"], "Rekrytera" prepended
// while the Duke is selected and recruiting is possible, the list of
// recruitable reserve type names (plus "Avbryt") while picking a type, or
// just "Avbryt"/"Meny" while picking a recruit destination square.
func (a *app) currentButtonLabels() []string {
	base := []string{"Ny", "Meny"}
	switch a.mode {
	case selTile:
		if a.canRecruitFromSelection() {
			return append([]string{"Rekrytera"}, base...)
		}
		return base
	case selRecruitType:
		var labels []string
		for _, t := range a.gs.Reserve[a.gs.Turn].Types() {
			labels = append(labels, t.Name())
		}
		return append(labels, "Avbryt")
	case selRecruitSquare:
		return []string{"Avbryt", "Meny"}
	default:
		return base
	}
}

// canRecruitFromSelection reports whether the current selection is the side
// to move's own Duke and at least one recruit action is available.
func (a *app) canRecruitFromSelection() bool {
	s := a.gs
	t := s.Board.At(a.selected.X, a.selected.Y)
	if t == nil || t.Type != game.Duke || t.Side != s.Turn {
		return false
	}
	return len(s.Board.RecruitActions(s.Turn, s.Reserve[s.Turn])) > 0
}

// nameToTroopType reverse-looks-up a troop type by its Swedish display name,
// as printed on the recruit-type buttons.
func nameToTroopType(name string) (game.TileType, bool) {
	for _, t := range game.TroopTypes {
		if t.Name() == name {
			return t, true
		}
	}
	return 0, false
}

func (a *app) statusText() string {
	s := a.gs
	bl, wh := s.Board.Count(game.Black), s.Board.Count(game.White)
	rl, rw := len(s.Reserve[game.Black].Types()), len(s.Reserve[game.White].Types())
	score := "Svart " + itoa(bl) + "+" + itoa(rl) + " – Vit " + itoa(wh) + "+" + itoa(rw)
	if s.Phase == game.PhaseDone {
		winner, _ := s.Winner()
		switch winner {
		case game.Black:
			return "Svart vann!  " + score
		default:
			return "Vit vann!  " + score
		}
	}
	turn := "Svart"
	if s.Turn == game.White {
		turn = "Vit"
	}
	if s.AITurn() {
		turn = "Vit (dator)"
	}
	label := turn + " drar"
	switch a.mode {
	case selRecruitType:
		label = turn + ": välj trupptyp"
	case selRecruitSquare:
		label = turn + ": välj ruta intill Hertigen"
	}
	return label + "   ·   " + score
}

func (a *app) Key(e ink.KeyEvent) bool {
	if e.State == ink.KeyStateUp && e.Key == ink.KeyBack {
		switch a.screen {
		case screenGame, screenRules:
			a.screen = screenMenu
			ink.Repaint()
			return true
		case screenLegend:
			a.screen = screenRules
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
		for _, b := range a.buttons {
			if b.Hit(p) {
				switch b.Label {
				case "Pjäslegend":
					a.screen = screenLegend
					ink.Repaint()
					return true
				case "Tillbaka":
					a.screen = screenMenu
					ink.Repaint()
					return true
				}
			}
		}
	case screenLegend:
		for _, b := range a.buttons {
			if b.Hit(p) && b.Label == "Tillbaka" {
				a.screen = screenRules
				ink.Repaint()
				return true
			}
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
	a.mode = selNone
	ink.Repaint()
}

func (a *app) tapGame(p image.Point) bool {
	for _, b := range a.buttons {
		if b.Hit(p) {
			return a.handleButton(b.Label)
		}
	}
	if a.gs.Phase != game.PhasePlaying || a.gs.AITurn() {
		return false
	}
	x, y, ok := a.layout.ScreenToCell(p)
	if !ok {
		return false
	}
	cell := image.Pt(x, y)

	switch a.mode {
	case selRecruitSquare:
		return a.tapRecruitSquare(cell)
	case selRecruitType:
		return false // must use the type buttons, not the board, in this mode
	case selTile:
		return a.tapWithSelection(cell)
	default:
		return a.tapNoSelection(cell)
	}
}

func (a *app) tapNoSelection(cell image.Point) bool {
	if t := a.gs.Board.At(cell.X, cell.Y); t != nil && t.Side == a.gs.Turn {
		a.selected = cell
		a.mode = selTile
		ink.Repaint()
		return true
	}
	return false
}

func (a *app) tapWithSelection(cell image.Point) bool {
	s := a.gs
	if cell == a.selected {
		a.mode = selNone
		ink.Repaint()
		return true
	}
	if t := s.Board.At(cell.X, cell.Y); t != nil && t.Side == s.Turn {
		a.selected = cell
		ink.Repaint()
		return true
	}
	for _, act := range s.Board.ActionsFrom(a.selected) {
		if act.To == cell {
			if s.Play(act) {
				a.finishAction()
				return true
			}
		}
	}
	return false
}

func (a *app) tapRecruitSquare(cell image.Point) bool {
	s := a.gs
	for _, act := range s.Board.RecruitActions(s.Turn, s.Reserve[s.Turn]) {
		if act.Recruit == a.recruitType && act.To == cell {
			if s.Play(act) {
				a.finishAction()
				return true
			}
		}
	}
	return false
}

// finishAction resets the selection state after a successfully applied
// action and, if it just became the AI's turn, queues its reply.
func (a *app) finishAction() {
	a.mode = selNone
	a.updates++
	if a.gs.AITurn() {
		a.aiPend = true
	}
	ink.Repaint()
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
	case "Rekrytera":
		if a.mode == selTile && a.canRecruitFromSelection() {
			a.mode = selRecruitType
			ink.Repaint()
			return true
		}
		return false
	case "Avbryt":
		a.mode = selNone
		ink.Repaint()
		return true
	}
	if a.mode == selRecruitType {
		if t, ok := nameToTroopType(label); ok {
			a.recruitType = t
			a.mode = selRecruitSquare
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
