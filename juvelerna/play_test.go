//go:build playtest

package main

// Headless PLAYTHROUGH tests for Juvelerna. They drive the real touch path
// — tapping bank gem piles to build a take-2/take-3 selection, tapping a
// tableau card/deck-back/own-reserved-card to select it for reserve/buy,
// then a contextual confirm button ("Ta"/"Reservera"/"Köp") — and check
// gameplay against the written rules (see rulesParagraphs in ui.go): the
// take-2-needs->=4-before gotcha, gold granted on reserve, buying with
// tokens+bonus discount+gold, the noble auto-claim vs. multi-qualify
// active-player choice, the token-cap forced discard, both game modes (all
// 3 AI difficulties), quitting, the rules screen, the opponent toggle, and
// a full hot-seat game driven entirely via simulated taps to a real winner.
// Runs under the pure-Go inkview emulator (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"juvelerna/game"
)

// --- helpers -----------------------------------------------------------

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

func startMode(t *testing.T, h *ink.Harness, a *app, mode game.Mode, aiLevel int) {
	t.Helper()
	for _, row := range a.menu.rows {
		if row.choice.mode == mode && (mode == game.ModeHotseat || row.choice.aiLevel == aiLevel) {
			h.TapRect(row.rect)
			if a.screen != screenGame || a.gs == nil || a.gs.Mode != mode {
				t.Fatalf("did not start mode %v (screen=%v)", mode, a.screen)
			}
			return
		}
	}
	t.Fatalf("no menu row for mode %v level %d; visible: %v", mode, aiLevel, texts(h))
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

func bankRectFor(a *app, c game.Color) (image.Rectangle, bool) {
	for _, hh := range a.layout.bankHits {
		if hh.color == c {
			return hh.rect, true
		}
	}
	return image.Rectangle{}, false
}

func tableauRectFor(a *app, tier, slot int) (image.Rectangle, bool) {
	for _, hh := range a.layout.tableauHits {
		if hh.tier == tier && hh.slot == slot {
			return hh.rect, true
		}
	}
	return image.Rectangle{}, false
}

func deckRectFor(a *app, tier int) (image.Rectangle, bool) {
	for _, hh := range a.layout.deckHits {
		if hh.tier == tier {
			return hh.rect, true
		}
	}
	return image.Rectangle{}, false
}

func reservedRectFor(a *app, idx int) (image.Rectangle, bool) {
	for _, hh := range a.layout.reservedHits {
		if hh.idx == idx {
			return hh.rect, true
		}
	}
	return image.Rectangle{}, false
}

func nobleRectFor(a *app, pendingIdx int) (image.Rectangle, bool) {
	for _, hh := range a.layout.nobleHits {
		if hh.idx == pendingIdx {
			return hh.rect, true
		}
	}
	return image.Rectangle{}, false
}

func discardColorRectFor(a *app, c game.Color) (image.Rectangle, bool) {
	for _, hh := range a.layout.discardHits {
		if !hh.isGold && hh.color == c {
			return hh.rect, true
		}
	}
	return image.Rectangle{}, false
}

func discardGoldRectFor(a *app) (image.Rectangle, bool) {
	for _, hh := range a.layout.discardHits {
		if hh.isGold {
			return hh.rect, true
		}
	}
	return image.Rectangle{}, false
}

// tapButton finds the on-screen button labeled s and taps it, returning
// whether the app actually handled (accepted) the tap — unlike
// h.TapText, which only reports whether the label was found on screen, not
// whether the underlying action succeeded.
func tapButton(t *testing.T, h *ink.Harness, label string) bool {
	t.Helper()
	sp, ok := h.FindText(label)
	if !ok {
		t.Fatalf("no on-screen button %q; visible=%v", label, texts(h))
	}
	return h.TapRect(sp.Rect)
}

// playActionViaTap drives a game.Action through the real 2-step UI flow:
// tap the source(s) (bank color(s), a tableau card, a deck-back, or a
// reserved card), then tap the matching confirm button.
func playActionViaTap(t *testing.T, h *ink.Harness, a *app, act game.Action) bool {
	t.Helper()
	switch act.Kind {
	case game.ActionTake3:
		for _, c := range act.Colors {
			r, ok := bankRectFor(a, c)
			if !ok {
				t.Fatalf("no bank rect for color %v", c)
			}
			if !h.TapRect(r) {
				t.Fatalf("bank tap rejected for color %v", c)
			}
		}
		return tapButton(t, h, "Ta")
	case game.ActionTake2:
		r, ok := bankRectFor(a, act.Colors[0])
		if !ok {
			t.Fatalf("no bank rect for color %v", act.Colors[0])
		}
		if !h.TapRect(r) {
			t.Fatalf("first bank tap rejected for color %v", act.Colors[0])
		}
		if !h.TapRect(r) {
			t.Fatalf("second bank tap rejected for color %v", act.Colors[0])
		}
		return tapButton(t, h, "Ta")
	case game.ActionReserveTableau:
		r, ok := tableauRectFor(a, act.Tier, act.Slot)
		if !ok {
			t.Fatalf("no tableau rect for tier=%d slot=%d", act.Tier, act.Slot)
		}
		h.TapRect(r)
		return tapButton(t, h, "Reservera")
	case game.ActionReserveBlind:
		r, ok := deckRectFor(a, act.Tier)
		if !ok {
			t.Fatalf("no deck rect for tier=%d", act.Tier)
		}
		h.TapRect(r)
		return tapButton(t, h, "Reservera")
	case game.ActionBuyTableau:
		r, ok := tableauRectFor(a, act.Tier, act.Slot)
		if !ok {
			t.Fatalf("no tableau rect for tier=%d slot=%d", act.Tier, act.Slot)
		}
		h.TapRect(r)
		return tapButton(t, h, "Köp")
	case game.ActionBuyReserved:
		r, ok := reservedRectFor(a, act.ReservedIdx)
		if !ok {
			t.Fatalf("no reserved rect for idx=%d", act.ReservedIdx)
		}
		h.TapRect(r)
		return tapButton(t, h, "Köp")
	}
	return false
}

// boardEqual compares two PlayerStates by value (they contain slices).
func boardEqual(a, b game.PlayerState) bool {
	if a.Tokens != b.Tokens || a.Gold != b.Gold || a.Bonuses != b.Bonuses || a.Prestige != b.Prestige {
		return false
	}
	return len(a.Cards) == len(b.Cards) && len(a.Reserved) == len(b.Reserved) && len(a.Nobles) == len(b.Nobles)
}

// --- RULE: take-3 of 3 distinct colors, via real taps -----------------------

func TestPlayJuvelernaTake3ViaTaps(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)
	h.Draw()

	act := game.Action{Kind: game.ActionTake3, Colors: [3]game.Color{game.ColorSolid, game.ColorRing, game.ColorCross}}
	if !playActionViaTap(t, h, a, act) {
		t.Fatal("taking 3 distinct colors via taps should be legal at game start")
	}
	p := a.gs.Players[0]
	if p.Tokens[game.ColorSolid] != 1 || p.Tokens[game.ColorRing] != 1 || p.Tokens[game.ColorCross] != 1 {
		t.Fatalf("player 0 should hold 1 of each taken color, got %+v", p.Tokens)
	}
	if a.gs.Turn != 1 {
		t.Fatalf("turn should have passed to player 1, got %d", a.gs.Turn)
	}
}

// --- GOTCHA: take-2 requires the bank pile to have had >=4 BEFORE taking,
// checked through the real UI, not just the game package. -------------------

func TestPlayJuvelernaTake2RequiresBankAtLeast4ViaTaps(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	a.gs.Bank[game.ColorDot] = 3 // one short of the required 4
	h.Draw()

	r, ok := bankRectFor(a, game.ColorDot)
	if !ok {
		t.Fatal("no bank rect for Onyx (Dot)")
	}
	h.TapRect(r)
	h.TapRect(r)
	if tapButton(t, h, "Ta") {
		t.Fatal("take-2 with only 3 in the bank beforehand must be rejected")
	}
	if a.gs.Turn != 0 {
		t.Fatal("a rejected take-2 must not pass the turn")
	}
	if _, ok := h.FindTextContains("Ogiltigt drag"); !ok {
		t.Fatalf("an illegal take-2 should surface a hint; visible=%v", texts(h))
	}

	// Now with exactly 4 in the bank, the identical tap sequence must
	// succeed.
	a.gs.Bank[game.ColorDot] = 4
	h.Draw()
	if !h.TapRect(r) {
		t.Fatal("bank tap should be accepted")
	}
	if !h.TapRect(r) {
		t.Fatal("second bank tap (same color) should be accepted")
	}
	if !tapButton(t, h, "Ta") {
		t.Fatal("take-2 with exactly 4 in the bank beforehand should succeed")
	}
	if a.gs.Players[0].Tokens[game.ColorDot] != 2 {
		t.Fatalf("player should now hold 2 Onyx tokens, got %d", a.gs.Players[0].Tokens[game.ColorDot])
	}
	if a.gs.Bank[game.ColorDot] != 2 {
		t.Fatalf("bank should now hold 2, got %d", a.gs.Bank[game.ColorDot])
	}
}

// --- RULE: reserving grants a gold token, via real taps ---------------------

func TestPlayJuvelernaReserveGrantsGoldViaTaps(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)
	h.Draw()

	card := a.gs.Tableau[0][0]
	if !playActionViaTap(t, h, a, game.Action{Kind: game.ActionReserveTableau, Tier: 0, Slot: 0}) {
		t.Fatal("reserving a face-up tier-1 card should be legal")
	}
	p := a.gs.Players[0]
	if len(p.Reserved) != 1 || p.Reserved[0] != card {
		t.Fatalf("reserved list should hold the exact card taken, got %+v", p.Reserved)
	}
	if p.Gold != 1 {
		t.Fatalf("reserving should grant 1 gold, got %d", p.Gold)
	}
	if a.gs.Turn != 1 {
		t.Fatal("turn should have passed")
	}
}

// --- RULE: buying pays with tokens + bonus discount + gold, via real taps --

func TestPlayJuvelernaBuyViaTaps(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	a.gs.Tableau[0][0] = game.Card{Tier: 1, Color: game.ColorDot, Points: 1, Cost: [game.NumColors]int{3, 0, 0, 0, 0}}
	p := &a.gs.Players[0]
	p.Tokens[game.ColorSolid] = 1
	p.Gold = 2
	h.Draw()

	if !playActionViaTap(t, h, a, game.Action{Kind: game.ActionBuyTableau, Tier: 0, Slot: 0}) {
		t.Fatal("1 token + 2 gold against a cost of 3 should be affordable via taps")
	}
	if p.Gold != 0 || p.Tokens[game.ColorSolid] != 0 {
		t.Fatalf("tokens and gold should have been spent, got tokens=%+v gold=%d", p.Tokens, p.Gold)
	}
	if p.Bonuses[game.ColorDot] != 1 {
		t.Fatal("the bought card's color bonus should now be owned")
	}
	if p.Prestige != 1 {
		t.Fatalf("prestige should include the card's point, got %d", p.Prestige)
	}
}

// --- GOTCHA: 2+ nobles qualifying at once requires the ACTIVE player's
// choice — via a real tap on the SECOND (non-first) pending noble tile. -----

func TestPlayJuvelernaNobleMultipleQualifyChoiceViaTap(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	a.gs.Nobles = []game.Noble{
		{ID: 0, Requirement: [game.NumColors]int{3, 3, 0, 0, 0}},
		{ID: 1, Requirement: [game.NumColors]int{3, 0, 3, 0, 0}},
	}
	a.gs.Tableau[0][0] = game.Card{Tier: 1, Color: game.ColorSolid, Cost: [game.NumColors]int{0, 0, 0, 0, 0}}
	p := &a.gs.Players[0]
	p.Bonuses[game.ColorRing] = 3
	p.Bonuses[game.ColorCross] = 3
	p.Bonuses[game.ColorSolid] = 2 // the free buy below brings this to 3, qualifying BOTH nobles
	h.Draw()

	if !playActionViaTap(t, h, a, game.Action{Kind: game.ActionBuyTableau, Tier: 0, Slot: 0}) {
		t.Fatal("free card buy should succeed via taps")
	}
	if a.gs.Phase != game.PhaseNobleChoice {
		t.Fatalf("2 qualifying nobles must require an active-player choice, got phase %v", a.gs.Phase)
	}
	if _, ok := h.FindTextContains("väljer en adelsperson"); !ok {
		t.Fatalf("status bar should prompt the noble choice; visible=%v", texts(h))
	}

	// Tap the SECOND pending noble tile (index 1), not the first — proving
	// this is a real choice, not an automatic first-match.
	wantNobleID := a.gs.Nobles[a.gs.PendingNobles[1]].ID
	r, ok := nobleRectFor(a, 1)
	if !ok {
		t.Fatal("no tappable rect for the 2nd pending noble")
	}
	if !h.TapRect(r) {
		t.Fatal("tapping the 2nd pending noble should be accepted")
	}
	if a.gs.Phase == game.PhaseNobleChoice {
		t.Fatal("phase should have advanced past the noble choice")
	}
	if len(a.gs.Players[0].Nobles) != 1 || a.gs.Players[0].Nobles[0].ID != wantNobleID {
		t.Fatalf("player should own exactly the CHOSEN noble (ID %d), got %+v", wantNobleID, a.gs.Players[0].Nobles)
	}
}

// --- RULE: token cap forces a discard sub-phase, resolved via a real tap ---

func TestPlayJuvelernaDiscardViaTap(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	a.gs.Players[0].Tokens[game.ColorSolid] = 9
	a.gs.Bank[game.ColorRing] = 4
	h.Draw()

	if !playActionViaTap(t, h, a, game.Action{Kind: game.ActionTake2, Colors: [3]game.Color{game.ColorRing}}) {
		t.Fatal("take-2 should be legal")
	}
	if a.gs.Phase != game.PhaseDiscard {
		t.Fatalf("holding 11 tokens should force PhaseDiscard, got %v", a.gs.Phase)
	}
	if _, ok := h.FindTextContains("måste kasta"); !ok {
		t.Fatalf("status bar should prompt the discard; visible=%v", texts(h))
	}

	r, ok := discardColorRectFor(a, game.ColorSolid)
	if !ok {
		t.Fatal("no tappable discard rect for the held Diamant tokens")
	}
	if !h.TapRect(r) {
		t.Fatal("tapping a held token to discard it should be accepted")
	}
	if a.gs.Phase != game.PhasePlaying {
		t.Fatal("phase should return to playing once the discard need reaches 0")
	}
	if a.gs.Turn != 1 {
		t.Fatal("turn should now have passed")
	}
}

// --- "Visa motståndare" toggle: read-only, blocks action taps --------------

func TestPlayJuvelernaShowOpponentToggle(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	if a.showOpp {
		t.Fatal("should start with the toggle off")
	}
	if a.fullBoardSide() != a.gs.Turn {
		t.Fatal("full board should default to the side to move")
	}
	if !h.TapRect(a.layout.ShowOppBtn) {
		t.Fatal("the toggle button should be tappable")
	}
	if !a.showOpp {
		t.Fatal("toggle should now be on")
	}
	if a.fullBoardSide() != 1-a.gs.Turn {
		t.Fatal("full board should now show the OTHER side while toggled on")
	}
	// While toggled on, bank taps must be inert (read-only view).
	h.Draw()
	if r, ok := bankRectFor(a, game.ColorSolid); ok && h.TapRect(r) {
		if a.sel.kind != selNone {
			t.Fatal("tapping the bank while showing the opponent must be a no-op")
		}
	}
	if !h.TapRect(a.layout.ShowOppBtn) {
		t.Fatal("toggling back off should be tappable")
	}
	if a.showOpp {
		t.Fatal("toggle should now be off again")
	}
}

// --- Both game modes, all 3 AI difficulties ---------------------------------

func TestPlayJuvelernaHotseatMode(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)
	act, ok := game.BestAction(a.gs, game.DepthEasy)
	if !ok {
		t.Fatal("no legal opening action")
	}
	if !playActionViaTap(t, h, a, act) {
		t.Fatal("a legal opening action should be accepted")
	}
	if a.gs.AITurn() {
		t.Fatal("hot-seat mode should never report an AI turn")
	}
}

// GOTCHA: the AI's reply is computed AFTER the human's move is drawn but
// still within the SAME tap call — Draw()'s aiPend mechanism is drained
// synchronously by the emulator's drainRepeat before h.TapRect returns (same
// pattern as othello/hasami/mosaik's own AllDifficultiesReply tests).
func TestPlayJuvelernaAllDifficultiesReply(t *testing.T) {
	for _, diff := range []int{game.DepthEasy, game.DepthMedium, game.DepthHard} {
		diff := diff
		t.Run(itoa(diff), func(t *testing.T) {
			h, a := bootToMenu(t)
			startMode(t, h, a, game.ModeAI, diff)
			if a.gs.AILevel != diff {
				t.Fatalf("AILevel = %d, want %d", a.gs.AILevel, diff)
			}
			before := a.gs.Players[1]
			act, ok := game.BestAction(a.gs, game.DepthEasy)
			if !ok {
				t.Fatal("no legal opening action for the human")
			}
			if !playActionViaTap(t, h, a, act) {
				t.Fatal("the human's opening action should be legal")
			}
			if a.gs.AITurn() {
				t.Fatal("control returned on the AI's turn (deferred reply not drained)")
			}
			if a.gs.Phase == game.PhasePlaying && boardEqual(a.gs.Players[1], before) {
				t.Fatal("the AI (player 1) never replied")
			}
		})
	}
}

// --- Quit mid-game (Back key AND the Meny button) ---------------------------

func TestPlayJuvelernaQuit(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)
	act, _ := game.BestAction(a.gs, game.DepthEasy)
	playActionViaTap(t, h, a, act)

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}

	startMode(t, h, a, game.ModeAI, game.DepthEasy)
	tappedMeny := false
	for _, b := range a.buttons {
		if b.Label == "Meny" {
			h.TapRect(b.Rect)
			tappedMeny = true
		}
	}
	if !tappedMeny {
		t.Fatalf("no Meny button in game; visible: %v", texts(h))
	}
	if a.screen != screenMenu {
		t.Fatalf("Meny button did not return to menu, screen=%v", a.screen)
	}
	startMode(t, h, a, game.ModeHotseat, 0) // menu still usable afterwards
}

// --- Rules screen ------------------------------------------------------------

func TestPlayJuvelernaRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("Baserat på Splendor"); !ok {
		t.Fatalf("rules text missing the original-game credit; visible: %v", texts(h))
	}
	if _, ok := h.FindTextContains("MINST 4"); !ok {
		t.Fatalf("rules text missing the explicit take-2 >=4-before rule; visible: %v", texts(h))
	}
	if _, ok := h.FindTextContains("FLERA"); !ok {
		t.Fatalf("rules text missing the multi-noble active-player-choice rule; visible: %v", texts(h))
	}
	if _, ok := h.FindTextContains("rundan spelas färdigt"); !ok {
		t.Fatalf("rules text missing the finish-the-round end-game timing; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- A full hot-seat game, played entirely via simulated taps to a real
// winner (someone reaches 15+ prestige, the round completes, and the
// winner is determined). ----------------------------------------------------

func TestPlayJuvelernaFullGameToWinner(t *testing.T) {
	h, a := bootToMenu(t)
	startMode(t, h, a, game.ModeHotseat, 0)

	finished := false
	for iter := 0; iter < 4000 && !finished; iter++ {
		switch a.gs.Phase {
		case game.PhasePlaying:
			act, ok := game.BestAction(a.gs, game.DepthEasy)
			if !ok {
				t.Fatalf("no legal action at iter %d (turn=%d)", iter, a.gs.Turn)
			}
			if !playActionViaTap(t, h, a, act) {
				t.Fatalf("action %+v rejected at iter %d", act, iter)
			}
		case game.PhaseNobleChoice:
			r, ok := nobleRectFor(a, 0)
			if !ok {
				t.Fatalf("no pending noble rect at iter %d", iter)
			}
			if !h.TapRect(r) {
				t.Fatalf("noble choice tap failed at iter %d", iter)
			}
		case game.PhaseDiscard:
			p := a.gs.Players[a.gs.Turn]
			discarded := false
			for c := game.Color(0); c < game.NumColors && !discarded; c++ {
				if p.Tokens[c] > 0 {
					if r, ok := discardColorRectFor(a, c); ok {
						discarded = h.TapRect(r)
					}
				}
			}
			if !discarded {
				if r, ok := discardGoldRectFor(a); ok {
					discarded = h.TapRect(r)
				}
			}
			if !discarded {
				t.Fatalf("could not find a discard target at iter %d", iter)
			}
		case game.PhaseDone:
			finished = true
		}
	}
	if !finished {
		t.Fatal("game did not reach PhaseDone within the iteration budget")
	}
	if a.gs.Players[0].Prestige < game.PrestigeToWin && a.gs.Players[1].Prestige < game.PrestigeToWin {
		t.Fatalf("neither player reached %d prestige (%d vs %d)",
			game.PrestigeToWin, a.gs.Players[0].Prestige, a.gs.Players[1].Prestige)
	}
	want := winnerText(a.gs)
	if _, ok := h.FindTextContains(want); !ok {
		t.Fatalf("winner banner %q not shown; visible=%v", want, texts(h))
	}
	t.Logf("full game finished: %s (final %d–%d, %d–%d cards)",
		want, a.gs.Players[0].Prestige, a.gs.Players[1].Prestige, len(a.gs.Players[0].Cards), len(a.gs.Players[1].Cards))
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayJuvelernaScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	shot := func(name string) {
		t.Helper()
		if err := h.Screenshot(dir + "/" + name); err != nil {
			t.Fatalf("screenshot %s: %v", name, err)
		}
	}

	shot("juvelerna_splash.png")
	h.TapXY(500, 700)
	shot("juvelerna_menu.png")
	h.TapText("Regler")
	shot("juvelerna_rules.png")
	h.Back()

	startMode(t, h, a, game.ModeAI, game.DepthMedium)
	// Engineer a busy, representative mid-game position: partial tokens,
	// bonuses, a reserved card, and a noble close to being claimed.
	p0 := &a.gs.Players[0]
	p0.Tokens = [game.NumColors]int{2, 1, 0, 2, 1}
	p0.Gold = 1
	p0.Bonuses = [game.NumColors]int{1, 2, 0, 0, 1}
	p0.Prestige = 6
	p0.Reserved = []game.Card{
		{Tier: 2, Color: game.ColorCross, Points: 2, Cost: [game.NumColors]int{2, 0, 0, 3, 0}},
	}
	p1 := &a.gs.Players[1]
	p1.Tokens = [game.NumColors]int{1, 1, 1, 0, 0}
	p1.Bonuses = [game.NumColors]int{0, 1, 1, 0, 0}
	p1.Prestige = 3
	h.Draw()
	shot("juvelerna_board_midgame.png")

	if !h.TapRect(a.layout.ShowOppBtn) {
		t.Fatal("toggle button should be tappable")
	}
	shot("juvelerna_board_show_opponent.png")
	h.TapRect(a.layout.ShowOppBtn)

	// Noble-choice screen.
	a.gs.Nobles = []game.Noble{
		{ID: 0, Requirement: [game.NumColors]int{3, 3, 0, 0, 0}},
		{ID: 1, Requirement: [game.NumColors]int{0, 3, 3, 0, 0}},
		{ID: 2, Requirement: [game.NumColors]int{0, 0, 0, 3, 3}},
	}
	a.gs.Phase = game.PhaseNobleChoice
	a.gs.PendingNobles = []int{0, 1}
	h.Draw()
	shot("juvelerna_noble_choice.png")
	a.gs.Phase = game.PhasePlaying
	a.gs.PendingNobles = nil

	// Discard screen.
	p0.Tokens[game.ColorSolid] = 8
	a.gs.Phase = game.PhaseDiscard
	a.gs.DiscardNeeded = 2
	h.Draw()
	shot("juvelerna_discard.png")

	// Winner screen.
	a.gs.Phase = game.PhaseDone
	a.gs.WinnerIdx = game.FinalWinner(
		game.PlayerSummary{Prestige: p0.Prestige, CardCount: 5},
		game.PlayerSummary{Prestige: p1.Prestige, CardCount: 3},
	)
	h.Draw()
	shot("juvelerna_winner.png")
}
