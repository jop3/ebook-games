//go:build playtest

package main

// Headless PLAYTHROUGH tests for Expeditionen. They drive the real touch
// path (screen state machine, hand-card selection, Spela/Kasta, the 6 draw
// sources, the human/AI turn handoff) exactly as a finger would on the
// device, under the pure-Go inkview emulator (playtest/play.sh). Modeled on
// sushi/play_test.go's shape (another hidden-info card game), adapted for
// Expeditionen's strict-alternating (not simultaneous) turn structure.

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"expeditionen/game"
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
		t.Fatal("tapping Starta spel should be accepted")
	}
	if a.screen != screenGame || a.gs == nil {
		t.Fatalf("did not start a game (screen=%v)", a.screen)
	}
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

func suitIndex(s game.Suit) int {
	for i, x := range game.AllSuits {
		if x == s {
			return i
		}
	}
	return -1
}

func tapHandCard(h *ink.Harness, a *app, i int) bool {
	if i < 0 || i >= len(a.rects.handCards) {
		return false
	}
	return h.TapRect(a.rects.handCards[i])
}

// playOneHumanTurn selects the human's hand index 0, plays it if legal
// (otherwise discards it), then draws from the deck — the simplest possible
// deterministic human policy, sufficient to drive a full round end-to-end.
func playOneHumanTurn(t *testing.T, h *ink.Harness, a *app) {
	t.Helper()
	if len(a.rects.handCards) == 0 {
		t.Fatal("no tappable hand cards on the human's turn")
	}
	card := a.gs.Players[game.HumanIdx].Hand[0]
	if !tapHandCard(h, a, 0) {
		t.Fatal("selecting hand card 0 should be accepted")
	}
	if !a.rects.haveAction {
		t.Fatal("Spela/Kasta should be offered once a hand card is selected")
	}
	row := a.gs.Players[game.HumanIdx].Rows[card.Suit]
	if game.LegalPlay(card, row) {
		if !h.TapRect(a.rects.playBtn) {
			t.Fatal("tapping Spela should be accepted for a legal play")
		}
	} else {
		if !h.TapRect(a.rects.discardBtn) {
			t.Fatal("tapping Kasta should always be accepted")
		}
	}
	if !a.gs.AwaitingDraw() {
		t.Fatal("after playing/discarding, the human should now owe a draw")
	}
	if !h.TapRect(a.rects.drawSlots[game.NumSuits]) {
		t.Fatal("tapping the deck to draw should be accepted")
	}
}

// playFullRound drives an entire round to game.PhaseDone using
// playOneHumanTurn every human turn (the AI's reply is deferred-computed
// automatically by the harness re-running Draw() after each Repaint).
func playFullRound(t *testing.T, h *ink.Harness, a *app) {
	t.Helper()
	for turns := 0; a.gs.Phase != game.PhaseDone; turns++ {
		if turns > 600 {
			t.Fatalf("round did not reach PhaseDone (stuck after %d turns, phase=%v turn=%d)", turns, a.gs.Phase, a.gs.Turn)
		}
		if !a.gs.HumanTurn() {
			t.Fatalf("turn %d: expected the human's turn but Turn=%d (AI's deferred move should already have resolved)", turns, a.gs.Turn)
		}
		playOneHumanTurn(t, h, a)
	}
}

// --- Splash -> menu -> start ------------------------------------------------

func TestPlayExpeditionenSplashMenuStart(t *testing.T) {
	h, a := bootToMenu(t)
	if _, ok := h.FindTextContains("Starta spel"); !ok {
		t.Fatalf("menu should show a Starta spel button; visible: %v", texts(h))
	}
	startGame(t, h, a)
	if len(a.gs.Players[game.HumanIdx].Hand) != game.HandSize {
		t.Fatalf("human hand = %d, want %d", len(a.gs.Players[game.HumanIdx].Hand), game.HandSize)
	}
	if len(a.gs.Players[game.AIIdx].Hand) != game.HandSize {
		t.Fatalf("AI hand = %d, want %d", len(a.gs.Players[game.AIIdx].Hand), game.HandSize)
	}
	if len(a.rects.handCards) != game.HandSize {
		t.Fatalf("rendered %d tappable hand cards, want %d", len(a.rects.handCards), game.HandSize)
	}
	if !a.gs.HumanTurn() {
		t.Fatal("the human should move first")
	}
}

// --- Selecting and playing a legal card via real taps -----------------------

func TestPlayExpeditionenLegalPlayAdvancesState(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)

	// Force a known hand so the first card is guaranteed legal to play.
	a.gs.Players[game.HumanIdx].Hand[0] = game.Card{Suit: game.SuitOken, Rank: 5}
	h.Draw()

	if !tapHandCard(h, a, 0) {
		t.Fatal("selecting hand card 0 should be accepted")
	}
	if !h.TapRect(a.rects.playBtn) {
		t.Fatal("tapping Spela should be accepted")
	}
	row := a.gs.Players[game.HumanIdx].Rows[game.SuitOken]
	if len(row) != 1 || row[0].Rank != 5 {
		t.Fatalf("Öknen row = %v, want [{Suit:Öknen Rank:5}]", row)
	}
	if !a.gs.AwaitingDraw() {
		t.Fatal("should now be awaiting a draw")
	}
	if a.selected != -1 {
		t.Fatal("selection should clear after a successful play")
	}
}

// --- Illegal plays are rejected, not silently accepted ----------------------

func TestPlayExpeditionenIllegalPlayIsRejected(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)

	// Row already has a 6; hand[0] is a lower-ranked 3 in the same suit.
	a.gs.Players[game.HumanIdx].Rows[game.SuitHavet] = []game.Card{{Suit: game.SuitHavet, Rank: 6}}
	a.gs.Players[game.HumanIdx].Hand[0] = game.Card{Suit: game.SuitHavet, Rank: 3}
	h.Draw()

	if !tapHandCard(h, a, 0) {
		t.Fatal("selecting the card should still be accepted (selection, not the play itself)")
	}
	if !h.TapRect(a.rects.playBtn) {
		t.Fatal("tapping Spela itself should register as a tap even though the play is illegal")
	}
	if a.gs.AwaitingDraw() {
		t.Fatal("an illegal play must not have consumed the turn")
	}
	if len(a.gs.Players[game.HumanIdx].Rows[game.SuitHavet]) != 1 {
		t.Fatal("the illegal card must not have been added to the row")
	}
	if !a.playRejected {
		t.Fatal("playRejected should be flagged so the UI can show a rejection hint")
	}
	if _, ok := h.FindTextContains("Ogiltigt"); !ok {
		t.Fatalf("rejection hint text not shown; visible: %v", texts(h))
	}
}

// --- Discarding is always legal, even for a card that couldn't be played ----

func TestPlayExpeditionenDiscardAlwaysWorks(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)

	a.gs.Players[game.HumanIdx].Rows[game.SuitVulkanen] = []game.Card{{Suit: game.SuitVulkanen, Rank: 8}}
	a.gs.Players[game.HumanIdx].Hand[0] = game.Card{Suit: game.SuitVulkanen, Rank: 2}
	h.Draw()

	tapHandCard(h, a, 0)
	if !h.TapRect(a.rects.discardBtn) {
		t.Fatal("tapping Kasta should be accepted")
	}
	pile := a.gs.Discards[game.SuitVulkanen]
	if len(pile) != 1 || pile[0].Rank != 2 {
		t.Fatalf("Vulkanen discard pile = %v, want the discarded {Rank:2}", pile)
	}
	if !a.gs.AwaitingDraw() {
		t.Fatal("should now be awaiting a draw")
	}
}

// --- Drawing from a visible discard-pile top --------------------------------

func TestPlayExpeditionenDrawFromDiscardPile(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)

	a.gs.Discards[game.SuitDjungeln] = []game.Card{{Suit: game.SuitDjungeln, Rank: 4}, {Suit: game.SuitDjungeln, Rank: 7}}
	a.gs.Players[game.HumanIdx].Hand[0] = game.Card{Suit: game.SuitOken, Rank: 2}
	h.Draw()

	tapHandCard(h, a, 0)
	h.TapRect(a.rects.discardBtn)

	idx := suitIndex(game.SuitDjungeln)
	handBefore := len(a.gs.Players[game.HumanIdx].Hand)
	if !h.TapRect(a.rects.drawSlots[idx]) {
		t.Fatal("tapping the Djungeln discard pile to draw should be accepted")
	}
	if len(a.gs.Players[game.HumanIdx].Hand) != handBefore+1 {
		t.Fatalf("hand length = %d, want %d", len(a.gs.Players[game.HumanIdx].Hand), handBefore+1)
	}
	got := a.gs.Players[game.HumanIdx].Hand[len(a.gs.Players[game.HumanIdx].Hand)-1]
	if got != (game.Card{Suit: game.SuitDjungeln, Rank: 7}) {
		t.Fatalf("drew %v, want the pile's TOP card {Djungeln,7}", got)
	}
	// NOTE: can't assert the pile is down to exactly 1 card here — the
	// human's draw ends their turn, and the harness synchronously resolves
	// the AI's whole deferred reply (including its OWN draw choice) before
	// TapRect returns; the AI may itself legally draw the remaining
	// {Djungeln,4} off this same pile, which would make it flaky to assert
	// on. What must always hold, regardless of what the AI does next, is
	// that the {7} is gone from the pile for good.
	for _, c := range a.gs.Discards[game.SuitDjungeln] {
		if c == (game.Card{Suit: game.SuitDjungeln, Rank: 7}) {
			t.Fatal("the drawn {Djungeln,7} must have left the discard pile")
		}
	}
}

// Tapping an EMPTY discard pile must be a no-op (it must not consume the
// human's draw / advance the turn).
func TestPlayExpeditionenTappingEmptyDiscardPileIsIgnored(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)

	a.gs.Discards[game.SuitPolaren] = nil // guaranteed empty
	a.gs.Players[game.HumanIdx].Hand[0] = game.Card{Suit: game.SuitOken, Rank: 2}
	h.Draw()
	tapHandCard(h, a, 0)
	h.TapRect(a.rects.discardBtn)

	idx := suitIndex(game.SuitPolaren)
	h.TapRect(a.rects.drawSlots[idx])
	if !a.gs.AwaitingDraw() {
		t.Fatal("tapping an empty discard pile must not have consumed the draw")
	}
	if a.gs.Turn != game.HumanIdx {
		t.Fatal("turn must not have passed after a no-op tap")
	}
}

// --- AI reply lands automatically after the human's draw --------------------

func TestPlayExpeditionenAIRepliesAfterHumanTurn(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)

	aiHandBefore := len(a.gs.Players[game.AIIdx].Hand)
	playOneHumanTurn(t, h, a)

	if a.gs.Turn != game.HumanIdx {
		t.Fatalf("Turn = %d, want HumanIdx once the AI's deferred reply has resolved", a.gs.Turn)
	}
	if a.gs.AwaitingDraw() {
		t.Fatal("should not be mid-turn after the AI's full reply")
	}
	if len(a.gs.Players[game.AIIdx].Hand) != aiHandBefore {
		t.Fatalf("AI hand size = %d, want unchanged at %d (one out, one drawn)", len(a.gs.Players[game.AIIdx].Hand), aiHandBefore)
	}
	// The AI must have taken exactly one action this turn (played into its
	// rows XOR discarded) — since the round just started, its rows can hold
	// at most 1 card total.
	totalAIRowCards := 0
	for _, r := range a.gs.Players[game.AIIdx].Rows {
		totalAIRowCards += len(r)
	}
	if totalAIRowCards > 1 {
		t.Fatalf("AI should have played at most 1 card this turn, rows have %d total", totalAIRowCards)
	}
}

// --- A full round reaches PhaseDone with a real, independently-checked -----
// --- final score -------------------------------------------------------

func TestPlayExpeditionenFullRoundToFinalScore(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)
	playFullRound(t, h, a)

	if a.gs.Phase != game.PhaseDone {
		t.Fatalf("Phase = %v, want PhaseDone", a.gs.Phase)
	}
	if len(a.gs.Deck) != 0 {
		t.Fatalf("deck = %d at round end, want 0", len(a.gs.Deck))
	}

	// Independently recompute both totals straight from the rows (not by
	// trusting Player.Score(), even though it's the same formula — this
	// cross-checks TotalScore/Score end to end through the real UI state).
	wantHuman := 0
	for _, row := range a.gs.Players[game.HumanIdx].Rows {
		wantHuman += game.Score(row)
	}
	wantAI := 0
	for _, row := range a.gs.Players[game.AIIdx].Rows {
		wantAI += game.Score(row)
	}
	if got := a.gs.Players[game.HumanIdx].Score(); got != wantHuman {
		t.Fatalf("human Score() = %d, want %d", got, wantHuman)
	}
	if got := a.gs.Players[game.AIIdx].Score(); got != wantAI {
		t.Fatalf("AI Score() = %d, want %d", got, wantAI)
	}

	winners := a.gs.Winner()
	if len(winners) == 0 {
		t.Fatal("Winner() returned nobody")
	}
	wantBanner := "vann"
	if len(winners) > 1 {
		wantBanner = "Oavgjort"
	}
	if _, ok := h.FindTextContains(wantBanner); !ok {
		t.Fatalf("final banner %q not shown; visible: %v", wantBanner, texts(h))
	}
	// The final breakdown table should be visible with both players' totals.
	if _, ok := h.FindTextContains("Totalt"); !ok {
		t.Fatalf("final breakdown table not shown; visible: %v", texts(h))
	}
}

// --- No input is accepted once the round is over, except Ny/Meny -----------

func TestPlayExpeditionenNoInputAfterRoundEnds(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)
	playFullRound(t, h, a)

	handBefore := len(a.gs.Players[game.HumanIdx].Hand)
	// The hand-card area is gone from the breakdown screen, but even a raw
	// tap in that region must not mutate game state.
	h.TapXY(200, 900)
	if len(a.gs.Players[game.HumanIdx].Hand) != handBefore {
		t.Fatal("tapping after PhaseDone must not change the hand")
	}
	if a.gs.Phase != game.PhaseDone {
		t.Fatal("Phase must remain Done")
	}
}

// --- Replaying: "Ny" starts a fresh round -----------------------------------

func TestPlayExpeditionenNyStartsAFreshRound(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)
	playFullRound(t, h, a)

	found := false
	for _, b := range a.navButtons {
		if b.Label == "Ny" {
			h.TapRect(b.Rect)
			found = true
		}
	}
	if !found {
		t.Fatalf("no Ny button on the finished round screen; visible: %v", texts(h))
	}
	if a.gs.Phase != game.PhasePlaying {
		t.Fatalf("Phase = %v, want PhasePlaying after Ny", a.gs.Phase)
	}
	if len(a.gs.Deck) != 60-2*game.HandSize {
		t.Fatalf("deck = %d after Ny, want a fresh %d-card draw pile", len(a.gs.Deck), 60-2*game.HandSize)
	}
}

// --- Quit mid-game: Back key AND the Meny button ----------------------------

func TestPlayExpeditionenQuitBackKey(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)
	playOneHumanTurn(t, h, a)

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}
	startGame(t, h, a) // menu still usable afterwards
}

func TestPlayExpeditionenQuitMenyButton(t *testing.T) {
	h, a := bootToMenu(t)
	startGame(t, h, a)
	playOneHumanTurn(t, h, a)

	tappedMeny := false
	for _, b := range a.navButtons {
		if b.Label == "Meny" {
			h.TapRect(b.Rect)
			tappedMeny = true
		}
	}
	if !tappedMeny {
		t.Fatalf("no Meny button in the game screen's nav bar; visible: %v", texts(h))
	}
	if a.screen != screenMenu {
		t.Fatalf("Meny button did not return to menu, screen=%v", a.screen)
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlayExpeditionenRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("Lost Cities"); !ok {
		t.Fatalf("rules text should credit the original game; visible: %v", texts(h))
	}
	if _, ok := h.FindTextContains("investeringskort"); !ok {
		t.Fatalf("rules text should explain the investment-card ordering rule; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
	h.TapText("Regler")
	if err := h.TapText("Tillbaka"); err != nil {
		t.Fatalf("no Tillbaka button: %v", err)
	}
	if a.screen != screenMenu {
		t.Fatalf("Tillbaka did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlayExpeditionenScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/expeditionen_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/expeditionen_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/expeditionen_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startGame(t, h, a)
	a.gs.Players[game.HumanIdx].Rows[game.SuitOken] = []game.Card{
		{Suit: game.SuitOken, Rank: game.RankInvestment}, {Suit: game.SuitOken, Rank: 2},
		{Suit: game.SuitOken, Rank: 4}, {Suit: game.SuitOken, Rank: 7},
	}
	a.gs.Players[game.HumanIdx].Rows[game.SuitHavet] = []game.Card{{Suit: game.SuitHavet, Rank: 9}}
	a.gs.Discards[game.SuitDjungeln] = []game.Card{{Suit: game.SuitDjungeln, Rank: 5}}
	a.gs.Players[game.AIIdx].Rows[game.SuitVulkanen] = []game.Card{{Suit: game.SuitVulkanen, Rank: 8}, {Suit: game.SuitVulkanen, Rank: 8}}
	h.Draw()
	if err := h.Screenshot(dir + "/expeditionen_game.png"); err != nil {
		t.Fatal(err)
	}

	tapHandCard(h, a, 0)
	if err := h.Screenshot(dir + "/expeditionen_selected.png"); err != nil {
		t.Fatal(err)
	}

	// Force PhaseDone for the breakdown screenshot.
	a.gs.Deck = nil
	a.gs.Phase = game.PhaseDone
	h.Draw()
	if err := h.Screenshot(dir + "/expeditionen_final.png"); err != nil {
		t.Fatal(err)
	}
}
