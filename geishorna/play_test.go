//go:build playtest

package main

// Headless PLAYTHROUGH tests for Geishorna. They drive the real touch path
// (screen state machine, action buttons, card selection, the Competition
// pairing step, responding to the AI's Gift/Competition offers, and the
// round-end/match-end flow) exactly as a finger would, under the pure-Go
// inkview emulator (playtest/play.sh). The AI runs itself via the deferred-AI
// drain the harness performs after every tap.

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"geishorna/game"
)

// --- helpers ----------------------------------------------------------------

func bootToMenu(t *testing.T) (*ink.Harness, *app) {
	t.Helper()
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700) // dismiss splash
	if a.screen != screenMenu {
		t.Fatalf("splash tap did not open menu, screen=%v", a.screen)
	}
	return h, a
}

func startGame(t *testing.T, h *ink.Harness, a *app) {
	t.Helper()
	if !h.TapRect(a.menu.StartButton()) {
		t.Fatal("tapping Starta match should be accepted")
	}
	if a.screen != screenGame || a.gs == nil {
		t.Fatalf("did not start a game (screen=%v)", a.screen)
	}
}

// firstAffordableAction returns the lowest-ordered action the human can spend
// this turn, or -1 if none.
func firstAffordableAction(a *app) game.Action {
	for _, act := range game.AllActions {
		if a.gs.Available(game.HumanIdx, act) {
			return act
		}
	}
	return -1
}

// driveHumanDecision performs exactly one human decision through the UI (a full
// action, an offer-response, or a round-end continue). It fails the test on any
// malformed UI state. Returns false only at PhaseMatchEnd (nothing to do).
func driveHumanDecision(t *testing.T, h *ink.Harness, a *app) bool {
	t.Helper()
	switch a.gs.Phase {
	case game.PhaseMatchEnd:
		return false
	case game.PhaseRoundEnd:
		if a.rects.continueBtn.Empty() {
			t.Fatal("round-end screen has no continue button")
		}
		h.TapRect(a.rects.continueBtn)
		return true
	}
	if !a.gs.HumanTurn() {
		t.Fatalf("not the human's turn and not a terminal phase (toAct=%d, pending=%v)", a.gs.ToAct(), a.gs.Pending != nil)
	}

	if a.mode == modeChoose {
		driveOfferResponse(t, h, a)
		return true
	}

	// modeAction: pick an action, select its cards, confirm.
	act := firstAffordableAction(a)
	if act < 0 {
		t.Fatal("human has no affordable action on their turn")
	}
	if !h.TapRect(a.rects.actionBtns[act].rect) {
		t.Fatalf("tapping action %s was not accepted", act.Name())
	}
	if a.mode != modeSelect || a.selAction != act {
		t.Fatalf("tapping action did not enter select mode (mode=%d)", a.mode)
	}
	need := act.Cards()
	for i := 0; i < need; i++ {
		if i >= len(a.rects.handCards) {
			t.Fatalf("not enough hand cards (%d) to select %d for %s", len(a.rects.handCards), need, act.Name())
		}
		h.TapRect(a.rects.handCards[i])
	}
	if len(a.sel) != need {
		t.Fatalf("selected %d cards, want %d for %s", len(a.sel), need, act.Name())
	}
	if !a.rects.confirmLive {
		t.Fatal("confirm button should be live once the right number of cards is selected")
	}
	h.TapRect(a.rects.confirmBtn)

	if act == game.Competition {
		if a.mode != modeCompPair {
			t.Fatalf("competition confirm did not enter the pairing step (mode=%d)", a.mode)
		}
		h.TapRect(a.rects.pairBtns[0])
	}
	return true
}

func driveOfferResponse(t *testing.T, h *ink.Harness, a *app) {
	t.Helper()
	pend := a.gs.Pending
	if pend == nil {
		t.Fatal("modeChoose with no pending offer")
	}
	if pend.Action == game.Gift {
		if len(a.rects.offerCards) != 3 {
			t.Fatalf("gift offer shows %d cards, want 3", len(a.rects.offerCards))
		}
		h.TapRect(a.rects.offerCards[0])
	} else {
		if len(a.rects.offerGroups) != 2 {
			t.Fatalf("competition offer shows %d groups, want 2", len(a.rects.offerGroups))
		}
		h.TapRect(a.rects.offerGroups[0])
	}
}

// --- tests ------------------------------------------------------------------

func TestPlaySplashMenuStart(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)
	if a.gs.Phase != game.PhaseAction {
		t.Fatalf("fresh game phase = %v, want action", a.gs.Phase)
	}
	if a.gs.Round != 1 {
		t.Fatalf("fresh game round = %d, want 1", a.gs.Round)
	}
	if !a.gs.HumanTurn() {
		t.Fatal("human should lead round 1")
	}
}

func TestPlayRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open the rules screen, screen=%v", a.screen)
	}
	if err := h.TapText("Tillbaka"); err != nil {
		t.Fatalf("no Tillbaka button: %v", err)
	}
	if a.screen != screenMenu {
		t.Fatalf("Tillbaka did not return to menu, screen=%v", a.screen)
	}
}

func TestPlaySingleActionAdvancesToAI(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)
	// Spend the human's Secret on hand card 0.
	if !h.TapRect(a.rects.actionBtns[game.Secret].rect) {
		t.Fatal("Secret button not accepted")
	}
	h.TapRect(a.rects.handCards[0])
	if !a.rects.confirmLive {
		t.Fatal("confirm not live after selecting 1 card for Secret")
	}
	h.TapRect(a.rects.confirmBtn)
	// After the human's action the AI takes its own turn (drained), so control
	// should be back with the human (or a later phase), never stuck on the AI.
	if a.gs.Phase == game.PhaseAction && a.gs.AITurn() {
		t.Fatal("stuck on the AI's turn after the human acted")
	}
	if !a.gs.Players[game.HumanIdx].Used[game.Secret] {
		t.Fatal("human's Secret marker was not spent")
	}
}

func TestPlayFullMatchReachesAWinner(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)
	guard := 0
	for a.gs.Phase != game.PhaseMatchEnd {
		guard++
		if guard > 4000 {
			t.Fatalf("match did not finish; phase=%v round=%d", a.gs.Phase, a.gs.Round)
		}
		if !driveHumanDecision(t, h, a) {
			break
		}
		// Invariant: whenever it is the human's action turn, hand size matches
		// the cards still owed (never negative, never stranded).
		if a.gs.Phase == game.PhaseAction && a.gs.HumanTurn() && a.gs.Pending == nil {
			if firstAffordableAction(a) < 0 {
				t.Fatalf("human turn with no affordable action; hand=%d", len(a.gs.Players[game.HumanIdx].Hand))
			}
		}
	}
	if a.gs.MatchWinner != game.HumanIdx && a.gs.MatchWinner != game.AIIdx {
		t.Fatalf("match ended without a valid winner (%d)", a.gs.MatchWinner)
	}
	g, c := a.gs.Standing(a.gs.MatchWinner)
	if g < game.GeishasToWin && c < game.CharmToWin {
		t.Fatalf("winner below both thresholds: %d geishas / %d charm", g, c)
	}
	// The match-end banner should be showing.
	if _, ok := h.FindTextContains("vann matchen"); !ok {
		t.Fatalf("no match-end banner; visible=%v", texts(h))
	}
}

func TestPlayExercisesEveryActionAndOffer(t *testing.T) {
	// Play several full matches so the driver is very likely to have exercised
	// each action type, the pairing step, and both kinds of AI offer without
	// crashing or wedging.
	for m := 0; m < 6; m++ {
		h, a := bootToMenu(t)
		startGame(t, h, a)
		guard := 0
		for a.gs.Phase != game.PhaseMatchEnd {
			guard++
			if guard > 4000 {
				t.Fatalf("match %d did not finish", m)
			}
			driveHumanDecision(t, h, a)
		}
	}
}

func TestPlayNyStartsFreshMatch(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)
	// Advance a little.
	driveHumanDecision(t, h, a)
	// "Ny" should reset to a fresh round 1 with neutral favor.
	for _, b := range a.navButtons {
		if b.Label == "Ny" {
			h.TapRect(b.Rect)
		}
	}
	if a.gs.Round != 1 {
		t.Fatalf("Ny did not reset to round 1 (round=%d)", a.gs.Round)
	}
	for i := 0; i < game.NumGeishas; i++ {
		if a.gs.Favor[i] != -1 {
			t.Fatalf("Ny did not reset favor for geisha %d", i)
		}
	}
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// --- Screenshots ------------------------------------------------------------

func TestPlayScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture screenshots")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	_ = h.Screenshot(dir + "/geishorna_splash.png")
	h.TapXY(500, 700)
	_ = h.Screenshot(dir + "/geishorna_menu.png")
	h.TapText("Regler")
	_ = h.Screenshot(dir + "/geishorna_rules.png")
	h.Back()

	startGame(t, h, a)
	// Seed a mid-round-ish board for a representative game screenshot.
	a.gs.Players[game.HumanIdx].Field = [game.NumGeishas]int{2, 1, 0, 1, 0, 0, 0}
	a.gs.Players[game.AIIdx].Field = [game.NumGeishas]int{1, 0, 2, 0, 1, 0, 0}
	a.gs.Favor = [game.NumGeishas]int{game.HumanIdx, -1, game.AIIdx, -1, -1, -1, -1}
	h.Draw()
	_ = h.Screenshot(dir + "/geishorna_game.png")

	// Action -> select mode screenshot.
	h.TapRect(a.rects.actionBtns[game.Gift].rect)
	h.TapRect(a.rects.handCards[0])
	_ = h.Screenshot(dir + "/geishorna_select.png")

	// Responding to an AI Competition offer.
	a.resetInput()
	a.gs.Pending = &game.Pending{
		Action:  game.Competition,
		Actor:   game.AIIdx,
		Chooser: game.HumanIdx,
		Groups:  [2][]game.Card{{{Geisha: 0}, {Geisha: 2}}, {{Geisha: 4}, {Geisha: 5}}},
	}
	a.gs.Turn = game.AIIdx
	a.mode = modeChoose
	h.Draw()
	_ = h.Screenshot(dir + "/geishorna_offer.png")

	// Round-end summary.
	a.gs.Pending = nil
	a.gs.Phase = game.PhaseRoundEnd
	a.gs.Result = &game.RoundResult{
		HasHumanSecret: true, HumanSecret: game.Card{Geisha: 0},
		HasAISecret: true, AISecret: game.Card{Geisha: 3},
	}
	h.Draw()
	_ = h.Screenshot(dir + "/geishorna_roundend.png")
}
