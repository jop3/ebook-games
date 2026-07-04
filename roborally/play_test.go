//go:build playtest

package main

import (
	"testing"

	ink "github.com/dennwc/inkview"

	"roborally/game"
)

// boot brings the app up to the menu (past the splash).
func boot(t *testing.T) (*app, *ink.Harness) {
	t.Helper()
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700) // dismiss splash
	if a.screen != screenMenu {
		t.Fatalf("expected menu after splash, got %d", a.screen)
	}
	return a, h
}

// startGameVia taps the menu "Starta" row.
func startGameVia(t *testing.T, a *app, h *ink.Harness) {
	t.Helper()
	for _, row := range a.menuRows {
		if row.id == "start" {
			h.TapRect(row.rect)
			return
		}
	}
	t.Fatal("no start row on menu")
}

// programRound fills the human's registers by tapping hand cards, then Kör.
func programRound(t *testing.T, a *app, h *ink.Harness) {
	t.Helper()
	for i := range a.handRects {
		if a.gs.ProgramComplete() {
			break
		}
		h.TapRect(a.handRects[i])
	}
	if !a.gs.ProgramComplete() {
		t.Fatal("could not complete a program from the hand")
	}
	h.TapRect(a.korBtn)
	if a.screen != screenResolve {
		t.Fatalf("Kör should enter resolution, screen=%d", a.screen)
	}
}

// TestPlayRoboRallyFullGame plays a whole game through the UI until someone wins,
// then verifies the winner really reached every checkpoint.
func TestPlayRoboRallyFullGame(t *testing.T) {
	a, h := boot(t)
	startGameVia(t, a, h)
	if a.gs == nil || a.gs.Board.NCheck == 0 {
		t.Fatal("game did not start")
	}
	for round := 0; round < 120 && a.screen != screenDone; round++ {
		switch a.screen {
		case screenProgram:
			programRound(t, a, h)
		case screenResolve:
			h.TapRect(a.nastaBtn)
		default:
			t.Fatalf("unexpected screen %d mid-game", a.screen)
		}
	}
	if a.screen != screenDone {
		t.Fatalf("game did not finish; final screen=%d winner=%d", a.screen, a.gs.Winner)
	}
	w := a.gs.Winner
	if w < 0 {
		t.Fatal("done screen but no winner set")
	}
	// Independent check: the winner touched every checkpoint in order.
	if int(a.gs.Robots[w].NextCheck) <= int(a.gs.Board.NCheck) {
		t.Fatalf("winner %d has not finished: NextCheck=%d NCheck=%d",
			w, a.gs.Robots[w].NextCheck, a.gs.Board.NCheck)
	}
	if _, ok := h.FindTextContains("vann"); !ok {
		t.Fatal("win banner not rendered")
	}
	// "Spela igen" restarts.
	h.TapRect(a.againBtn)
	if a.screen != screenProgram || a.gs.Round != 1 {
		t.Fatalf("replay should start a fresh game, screen=%d round=%d", a.screen, a.gs.Round)
	}
}

// TestPlayProgramGuards checks Kör is inert until five cards are placed and that
// tapping a filled register returns its card to the hand.
func TestPlayProgramGuards(t *testing.T) {
	a, h := boot(t)
	startGameVia(t, a, h)

	// Kör before any placement must not start resolution.
	h.TapRect(a.korBtn)
	if a.screen != screenProgram {
		t.Fatal("Kör should be inert with an empty program")
	}
	// Place one card, then clear it via the register.
	h.TapRect(a.handRects[0])
	placed := 0
	for r := 0; r < 5; r++ {
		if a.gs.Registers[0][r] != game.CardNone {
			placed++
		}
	}
	if placed != 1 {
		t.Fatalf("expected 1 placed card, got %d", placed)
	}
	h.TapRect(a.regRects[0])
	if a.gs.Registers[0][0] != game.CardNone {
		t.Fatal("tapping a filled register should clear it")
	}
	// Now complete and run one register.
	programRound(t, a, h)
	before := a.gs.CurReg
	h.TapRect(a.nastaBtn)
	if a.gs.CurReg == before && a.screen == screenResolve {
		t.Fatal("Nästa should advance the register")
	}
}

// TestPlayMenuCyclesAndQuit exercises the menu selectors and both quit paths.
func TestPlayMenuCyclesAndQuit(t *testing.T) {
	a, h := boot(t)

	// Cycle course difficulty through all three and back.
	first := a.cfg.diff
	for _, row := range a.menuRows {
		if row.id == "diff" {
			h.TapRect(row.rect)
		}
	}
	if a.cfg.diff == first {
		t.Fatal("tapping Bana should change difficulty")
	}
	// Cycle opponent count.
	nBefore := a.cfg.nAI
	for _, row := range a.menuRows {
		if row.id == "nai" {
			h.TapRect(row.rect)
		}
	}
	if a.cfg.nAI == nBefore {
		t.Fatal("tapping Motståndare should change AI count")
	}

	// Start, then quit with Back to the menu.
	startGameVia(t, a, h)
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back should return to menu, got %d", a.screen)
	}
	// Start again, run into resolution, quit with the Meny button.
	startGameVia(t, a, h)
	programRound(t, a, h)
	h.TapRect(a.menyBtn)
	if a.screen != screenMenu {
		t.Fatalf("Meny should return to menu, got %d", a.screen)
	}
}

// TestPlayAllDifficulties boots each course difficulty and plays a few rounds to
// make sure every board renders and resolves without stalling.
func TestPlayAllDifficulties(t *testing.T) {
	for diff := 0; diff < 3; diff++ {
		a, h := boot(t)
		for _, row := range a.menuRows {
			if row.id == "diff" {
				for int(a.cfg.diff) != diff {
					h.TapRect(row.rect)
				}
			}
		}
		startGameVia(t, a, h)
		for step := 0; step < 20 && a.screen != screenDone; step++ {
			switch a.screen {
			case screenProgram:
				programRound(t, a, h)
			case screenResolve:
				h.TapRect(a.nastaBtn)
			}
		}
		if a.gs.Round < 1 {
			t.Fatalf("diff %d: game did not progress", diff)
		}
	}
}

// TestPlayScreens writes screenshots of every screen for visual review.
func TestPlayScreens(t *testing.T) {
	dir := shotDir()
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture")
	}
	// Splash (before dismissing it).
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	h.Screenshot(dir + "/01_splash.png")
	h.TapXY(500, 700) // -> menu
	h.Screenshot(dir + "/02_menu.png")

	// Rules screen.
	h.TapRect(a.rulesBtn)
	h.Screenshot(dir + "/03_rules.png")
	h.Back() // -> menu

	// Use 3 AI opponents so all four robot emblems appear on the board.
	for _, row := range a.menuRows {
		if row.id == "nai" {
			for a.cfg.nAI != 3 {
				h.TapRect(row.rect)
			}
		}
	}
	// Program screen (empty then full) on the default Mellan course.
	startGameVia(t, a, h)
	h.Screenshot(dir + "/04_program_empty.png")
	for i := range a.handRects {
		if a.gs.ProgramComplete() {
			break
		}
		h.TapRect(a.handRects[i])
	}
	h.Screenshot(dir + "/05_program_full.png")

	// First resolution frame.
	h.TapRect(a.korBtn)
	h.Screenshot(dir + "/06_resolve_r1.png")

	// Play several rounds so robots spread out, then grab a mid-game board.
	for round := 0; round < 8 && a.screen != screenDone; round++ {
		switch a.screen {
		case screenProgram:
			for i := range a.handRects {
				if a.gs.ProgramComplete() {
					break
				}
				h.TapRect(a.handRects[i])
			}
			h.TapRect(a.korBtn)
		case screenResolve:
			h.TapRect(a.nastaBtn)
		}
	}
	h.Screenshot(dir + "/07_midgame.png")

	// Drive to a finished game and capture the result screen.
	for round := 0; round < 200 && a.screen != screenDone; round++ {
		switch a.screen {
		case screenProgram:
			for i := range a.handRects {
				if a.gs.ProgramComplete() {
					break
				}
				h.TapRect(a.handRects[i])
			}
			h.TapRect(a.korBtn)
		case screenResolve:
			h.TapRect(a.nastaBtn)
		}
	}
	h.Screenshot(dir + "/08_result.png")
}

func shotDir() string { return shotDirEnv }
