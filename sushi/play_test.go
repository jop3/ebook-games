//go:build playtest

package main

// Headless PLAYTHROUGH tests for Sushi. They drive the real touch path
// (screen state machine, hand-card taps, the Chopsticks toggle, round/game
// transitions) exactly as a finger would on the device, under the pure-Go
// inkview emulator (playtest/play.sh). Modeled on hasami/play_test.go's
// structure and depth, adapted for a drafting game: there is no board to
// tap cells on, so "moves" are picks against a.handRects.

import (
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"sushi/game"
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

// startPlayerCount taps the menu row for an n-player game and enters it.
func startPlayerCount(t *testing.T, h *ink.Harness, a *app, n int) {
	t.Helper()
	for _, row := range a.menu.rows {
		if row.n == n {
			h.TapRect(row.rect)
			if a.screen != screenGame || a.gs == nil || a.gs.NumPlayers != n {
				t.Fatalf("did not start a %d-player game (screen=%v)", n, a.screen)
			}
			return
		}
	}
	t.Fatalf("no menu row for %d players; visible: %v", n, texts(h))
}

func tapHandCard(h *ink.Harness, a *app, i int) bool {
	if i < 0 || i >= len(a.handRects) {
		return false
	}
	return h.TapRect(a.handRects[i])
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// playFullGame drives an entire game to PhaseGameEnd using the simplest
// possible deterministic human policy: always take hand index 0 (never use
// Chopsticks), and always tap through round-end screens. This is enough to
// reach a real final ranking end-to-end without needing an EV policy.
func playFullGame(t *testing.T, h *ink.Harness, a *app) {
	t.Helper()
	for turns := 0; a.screen != screenGameEnd; turns++ {
		if turns > 3000 {
			t.Fatalf("game did not reach screenGameEnd (stuck on screen=%v)", a.screen)
		}
		switch a.screen {
		case screenGame:
			if len(a.handRects) == 0 {
				t.Fatalf("no tappable hand cards while in screenGame (round=%d)", a.gs.Round)
			}
			if !tapHandCard(h, a, 0) {
				t.Fatal("tapping hand index 0 was rejected")
			}
		case screenRoundEnd:
			if !h.TapRect(a.roundEndBtn) {
				t.Fatal("tapping the round-end continue button was rejected")
			}
		default:
			t.Fatalf("unexpected screen mid-game: %v", a.screen)
		}
	}
}

// --- Player-count selection (2p, 4p, and one other) -------------------------

func TestPlaySushiPlayerCountSelection(t *testing.T) {
	for _, n := range []int{2, 4, 5} {
		n := n
		t.Run(itoa(n), func(t *testing.T) {
			h, a := bootToMenu(t)
			startPlayerCount(t, h, a, n)
			if a.gs.NumPlayers != n {
				t.Fatalf("NumPlayers = %d, want %d", a.gs.NumPlayers, n)
			}
			want := game.HandSize(n)
			for i, p := range a.gs.Players {
				if len(p.Hand) != want {
					t.Fatalf("player %d hand = %d cards, want %d", i, len(p.Hand), want)
				}
			}
			if len(a.handRects) != want {
				t.Fatalf("rendered %d tappable hand cards, want %d", len(a.handRects), want)
			}
		})
	}
}

// --- Drafting a card via a real tap -----------------------------------------

func TestPlaySushiDraftAdvancesState(t *testing.T) {
	h, a := bootToMenu(t)
	startPlayerCount(t, h, a, 4)

	before := a.gs.HandLen()
	beforeTableau := len(a.gs.Players[0].Tableau)
	pickedCard := a.gs.Players[0].Hand[0]

	if !tapHandCard(h, a, 0) {
		t.Fatal("tapping the first hand card should be accepted")
	}
	if a.gs.HandLen() != before-1 {
		t.Fatalf("hand length = %d, want %d", a.gs.HandLen(), before-1)
	}
	if len(a.gs.Players[0].Tableau) != beforeTableau+1 {
		t.Fatalf("tableau length = %d, want %d", len(a.gs.Players[0].Tableau), beforeTableau+1)
	}
	if a.gs.Players[0].Tableau[beforeTableau] != pickedCard {
		t.Fatalf("tableau should hold the exact card that was tapped, got %v want %v",
			a.gs.Players[0].Tableau[beforeTableau], pickedCard)
	}
	// Every AI seat should also have drafted something this same turn.
	for i := 1; i < a.gs.NumPlayers; i++ {
		if len(a.gs.Players[i].Tableau) != 1 {
			t.Fatalf("AI seat %d should have drafted exactly 1 card, has tableau %v", i, a.gs.Players[i].Tableau)
		}
	}
}

// --- Chopsticks 2-pick via real taps ----------------------------------------

func TestPlaySushiChopsticksTwoPickViaTaps(t *testing.T) {
	h, a := bootToMenu(t)
	startPlayerCount(t, h, a, 2)

	// Force an unplayed Chopsticks into the human's tableau and a known hand.
	a.gs.Players[0].Tableau = []game.Card{game.Chopsticks()}
	a.gs.Players[0].Hand = []game.Card{game.Tempura(), game.Sashimi(), game.Dumpling()}
	a.gs.Players[1].Hand = []game.Card{game.Tempura(), game.Sashimi(), game.Dumpling()}
	h.Draw()

	if !a.chopAvailable() {
		t.Fatal("chopAvailable() should be true with an unplayed Chopsticks in the tableau")
	}
	if !h.TapRect(a.chopBtn) {
		t.Fatal("tapping 'Använd ätpinnar' should be accepted")
	}
	if !a.chopMode {
		t.Fatal("chopMode should now be on")
	}
	if !tapHandCard(h, a, 0) {
		t.Fatal("first card tap (selecting, not yet committing) should be accepted")
	}
	if a.gs.HandLen() != 3 {
		t.Fatal("selecting the first of 2 chopsticks cards must not resolve the turn yet")
	}
	if !tapHandCard(h, a, 1) {
		t.Fatal("second card tap should commit the 2-pick")
	}
	// Turn resolved: both drafted cards in tableau, chopsticks left the
	// tableau (now sitting back in the passed-on hand), hand length -1 net.
	if a.gs.HandLen() != 2 {
		t.Fatalf("hand length after a chopsticks turn = %d, want 2 (net -1)", a.gs.HandLen())
	}
	tab := a.gs.Players[0].Tableau
	if len(tab) != 2 {
		t.Fatalf("tableau should now hold exactly the 2 drafted cards, got %v", tab)
	}
	for _, c := range tab {
		if c.Kind == game.KindChopsticks {
			t.Fatal("the used chopsticks card must have left the tableau")
		}
	}
}

// --- Round-end score breakdown ----------------------------------------------

func TestPlaySushiRoundEndBreakdown(t *testing.T) {
	h, a := bootToMenu(t)
	startPlayerCount(t, h, a, 3)

	// Force both hands down to their last card so one more tap ends round 1.
	a.gs.Players[0].Hand = []game.Card{game.NigiriSquid()}
	a.gs.Players[1].Hand = []game.Card{game.Tempura()}
	a.gs.Players[2].Hand = []game.Card{game.Maki2()}
	h.Draw()

	if !tapHandCard(h, a, 0) {
		t.Fatal("the round-ending tap should be accepted")
	}
	if a.screen != screenRoundEnd {
		t.Fatalf("screen = %v, want screenRoundEnd", a.screen)
	}
	if _, ok := h.FindTextContains("avslutad"); !ok {
		t.Fatalf("round-end title not shown; visible: %v", texts(h))
	}
	if _, ok := h.FindTextContains("Du"); !ok {
		t.Fatalf("breakdown table should list the human player 'Du'; visible: %v", texts(h))
	}
	if a.gs.LastRoundScores[0].Nigiri != 3 {
		t.Fatalf("player 0 should have scored 3 for a lone squid nigiri, got %+v", a.gs.LastRoundScores[0])
	}
}

// --- Pudding carries across rounds; everything else resets ------------------

func TestPlaySushiPuddingCarriesAcrossRounds(t *testing.T) {
	h, a := bootToMenu(t)
	startPlayerCount(t, h, a, 2)

	a.gs.Players[0].Hand = []game.Card{game.Pudding()}
	a.gs.Players[1].Hand = []game.Card{game.Tempura()}
	h.Draw()
	tapHandCard(h, a, 0)
	if a.screen != screenRoundEnd {
		t.Fatalf("screen = %v, want screenRoundEnd", a.screen)
	}
	if a.gs.Players[0].Pudding != 1 {
		t.Fatalf("player 0 pudding = %d, want 1", a.gs.Players[0].Pudding)
	}

	if !h.TapRect(a.roundEndBtn) {
		t.Fatal("advancing to round 2 should be accepted")
	}
	if a.gs.Round != 2 {
		t.Fatalf("Round = %d, want 2", a.gs.Round)
	}
	if len(a.gs.Players[0].Tableau) != 0 {
		t.Fatal("tableau must reset for the new round")
	}
	if a.gs.Players[0].Pudding != 1 {
		t.Fatal("pudding must persist into round 2, not reset")
	}
}

// --- Full 3-round game played to a real final ranking -----------------------

func TestPlaySushiFullGameToFinalRanking(t *testing.T) {
	h, a := bootToMenu(t)
	startPlayerCount(t, h, a, 4)
	playFullGame(t, h, a)

	if a.gs.Phase != game.PhaseGameEnd {
		t.Fatalf("Phase = %v, want PhaseGameEnd", a.gs.Phase)
	}
	if a.gs.Round != game.NumRounds {
		t.Fatalf("Round = %d, want %d", a.gs.Round, game.NumRounds)
	}
	winners := a.gs.Winner()
	if len(winners) == 0 {
		t.Fatal("Winner() returned nobody")
	}
	wantBanner := "vinner"
	if len(winners) > 1 {
		wantBanner = "Oavgjort"
	}
	if _, ok := h.FindTextContains(wantBanner); !ok {
		t.Fatalf("final banner %q not shown; visible: %v", wantBanner, texts(h))
	}
}

// --- Quit mid-game: Back key AND the Meny button ----------------------------

func TestPlaySushiQuitBackKey(t *testing.T) {
	h, a := bootToMenu(t)
	startPlayerCount(t, h, a, 3)
	tapHandCard(h, a, 0)

	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back mid-game did not return to menu, screen=%v", a.screen)
	}
	// Menu still usable afterwards.
	startPlayerCount(t, h, a, 2)
}

func TestPlaySushiQuitMenyButton(t *testing.T) {
	h, a := bootToMenu(t)
	startPlayerCount(t, h, a, 3)
	tapHandCard(h, a, 0)

	tappedMeny := false
	for _, b := range a.buttons {
		if b.Label == "Meny" {
			h.TapRect(b.Rect)
			tappedMeny = true
		}
	}
	if !tappedMeny {
		t.Fatalf("no Meny button in the drafting screen bar; visible: %v", texts(h))
	}
	if a.screen != screenMenu {
		t.Fatalf("Meny button did not return to menu, screen=%v", a.screen)
	}
}

// --- Rules screen ------------------------------------------------------------

func TestPlaySushiRulesScreen(t *testing.T) {
	h, a := bootToMenu(t)
	if err := h.TapText("Regler"); err != nil {
		t.Fatalf("no Regler button: %v", err)
	}
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules, screen=%v", a.screen)
	}
	if _, ok := h.FindTextContains("Sushi Go"); !ok {
		t.Fatalf("rules text should credit the original game; visible: %v", texts(h))
	}
	h.Back()
	if a.screen != screenMenu {
		t.Fatalf("Back did not leave the rules screen, screen=%v", a.screen)
	}
	// Also exercise the Tillbaka button, not just the hardware key.
	h.TapText("Regler")
	if err := h.TapText("Tillbaka"); err != nil {
		t.Fatalf("no Tillbaka button: %v", err)
	}
	if a.screen != screenMenu {
		t.Fatalf("Tillbaka did not leave the rules screen, screen=%v", a.screen)
	}
}

// --- Screenshots of every screen for visual review --------------------------

func TestPlaySushiScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Screenshot(dir + "/sushi_splash.png"); err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if err := h.Screenshot(dir + "/sushi_menu.png"); err != nil {
		t.Fatal(err)
	}
	h.TapText("Regler")
	if err := h.Screenshot(dir + "/sushi_rules.png"); err != nil {
		t.Fatal(err)
	}
	h.Back()

	startPlayerCount(t, h, a, 4)

	// Mid-round hand/tableau view with several kinds visible, including a
	// wasabi+nigiri pairing and some maki.
	a.gs.Players[0].Tableau = []game.Card{
		game.Wasabi(), game.NigiriSquid(), game.Tempura(), game.Tempura(),
		game.Sashimi(), game.Sashimi(), game.Maki3(), game.Dumpling(),
	}
	a.gs.Players[0].Hand = []game.Card{
		game.NigiriEgg(), game.Wasabi(), game.Maki2(), game.Sashimi(),
		game.Dumpling(), game.Chopsticks(), game.Pudding(),
	}
	a.gs.Players[1].Tableau = []game.Card{game.Maki2(), game.Tempura()}
	a.gs.Players[2].Tableau = []game.Card{game.Sashimi(), game.Sashimi()}
	a.gs.Players[3].Tableau = []game.Card{game.Pudding()}
	h.Draw()
	if err := h.Screenshot(dir + "/sushi_hand_tableau.png"); err != nil {
		t.Fatal(err)
	}

	// The "use chopsticks" button state: force one into the tableau too, and
	// select one of its two cards to show the mid-selection state.
	a.gs.Players[0].Tableau = append(a.gs.Players[0].Tableau, game.Chopsticks())
	h.Draw()
	if !h.TapRect(a.chopBtn) {
		t.Fatal("tapping the chopsticks button should be accepted")
	}
	tapHandCard(h, a, 0)
	if err := h.Screenshot(dir + "/sushi_chopsticks.png"); err != nil {
		t.Fatal(err)
	}
	// Reset selection state so it doesn't leak into the next forced state.
	a.chopMode = false
	a.selected = nil

	// Round-end breakdown.
	a.gs.Players[0].Hand = []game.Card{game.NigiriSquid()}
	a.gs.Players[1].Hand = []game.Card{game.Tempura()}
	a.gs.Players[2].Hand = []game.Card{game.Maki1()}
	a.gs.Players[3].Hand = []game.Card{game.Dumpling()}
	h.Draw()
	tapHandCard(h, a, 0)
	if a.screen != screenRoundEnd {
		t.Fatalf("expected screenRoundEnd, got %v", a.screen)
	}
	if err := h.Screenshot(dir + "/sushi_round_end.png"); err != nil {
		t.Fatal(err)
	}

	// Final game-end banner: force straight to PhaseGameEnd with a clear
	// winner and a believable pudding split.
	a.screen = screenGame
	a.gs.Phase = game.PhasePlaying
	a.gs.Round = game.NumRounds
	a.gs.Players[0].Tableau = nil
	a.gs.Players[1].Tableau = nil
	a.gs.Players[2].Tableau = nil
	a.gs.Players[3].Tableau = nil
	a.gs.Players[0].Hand = []game.Card{game.Pudding()}
	a.gs.Players[1].Hand = []game.Card{game.Tempura()}
	a.gs.Players[2].Hand = []game.Card{game.Tempura()}
	a.gs.Players[3].Hand = []game.Card{game.Tempura()}
	a.gs.Players[0].Score = 40
	a.gs.Players[1].Score = 25
	a.gs.Players[2].Score = 20
	a.gs.Players[3].Score = 15
	h.Draw()
	if !tapHandCard(h, a, 0) {
		t.Fatal("the final tap of the game should be accepted")
	}
	if a.screen != screenGameEnd {
		t.Fatalf("expected screenGameEnd, got %v", a.screen)
	}
	if err := h.Screenshot(dir + "/sushi_game_end.png"); err != nil {
		t.Fatal(err)
	}
}
