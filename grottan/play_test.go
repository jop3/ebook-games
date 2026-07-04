//go:build playtest

// Play/render tests for Grottan, run under the pure-Go inkview emulator via
// playtest/play.sh (never compiled into the device build — the build tag gates
// them). They drive the real app through Init→Draw→Pointer, assert the scripted
// walk works end to end through the tap UI, render every screen to a PNG, and
// check that nothing overflows the real 1340px drawable height at the worst case.
//
// This is the current harness convention (guide §6b): the `//go:build playtest`
// tag makes the file invisible to `go build`/`go vet` and the device Docker
// build, so — like the other 20 games' play suites — it is committed and kept as
// a regression guard, NOT deleted before shipping.
package main

import (
	"os"
	"path/filepath"
	"testing"

	ink "github.com/dennwc/inkview"

	"grottan/story"
)

func shot(t *testing.T, h *ink.Harness, name string) {
	t.Helper()
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := h.Screenshot(filepath.Join(dir, name+".png")); err != nil {
		t.Fatalf("screenshot %s: %v", name, err)
	}
}

// boot returns a freshly booted app wired to a temp save slot.
func boot(t *testing.T) (*app, *ink.Harness) {
	t.Helper()
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	a.savePath = filepath.Join(t.TempDir(), "grottan.sav")
	return a, h
}

// assertNoOverflow fails if any drawn text extends past the real drawable height
// (guide §5: content below ~1360 wraps to the top on hardware).
func assertNoOverflow(t *testing.T, h *ink.Harness, where string) {
	t.Helper()
	for _, sp := range h.Texts() {
		if sp.Rect.Max.Y > usableH {
			t.Errorf("%s: text %q overflows bottom: y=%d > %d", where, sp.S, sp.Rect.Max.Y, usableH)
		}
		if sp.Rect.Min.Y < 0 {
			t.Errorf("%s: text %q above top edge: y=%d", where, sp.S, sp.Rect.Min.Y)
		}
	}
}

// findBtn returns the button with the given label from a slice, or fails.
func findBtn(t *testing.T, btns []button, label string) button {
	t.Helper()
	for _, b := range btns {
		if b.Label == label {
			return b
		}
	}
	t.Fatalf("no button labelled %q among %v", label, labelsOf(btns))
	return button{}
}

func labelsOf(btns []button) []string {
	out := make([]string, len(btns))
	for i, b := range btns {
		out[i] = b.Label
	}
	return out
}

// TestPlaySplashMenuRules renders the three chrome screens and checks overflow.
func TestPlaySplashMenuRules(t *testing.T) {
	a, h := boot(t)
	if a.screen != screenSplash {
		t.Fatalf("initial screen = %d, want splash", a.screen)
	}
	shot(t, h, "01_splash")

	// Any tap advances to the menu.
	h.TapXY(500, 900)
	if a.screen != screenMenu {
		t.Fatalf("after splash tap screen = %d, want menu", a.screen)
	}
	// Force the "Fortsätt" row to render (worst case: three menu buttons).
	a.hasSave = true
	h.Draw()
	shot(t, h, "02_menu")
	assertNoOverflow(t, h, "menu")
	if _, ok := h.FindText("Nytt spel"); !ok {
		t.Fatal("menu missing 'Nytt spel'")
	}

	// Open the rules screen.
	rules := findBtn(t, a.menuBtns, "Regler")
	h.TapRect(rules.Rect)
	if a.screen != screenRules {
		t.Fatalf("after Regler screen = %d, want rules", a.screen)
	}
	shot(t, h, "03_rules")
	assertNoOverflow(t, h, "rules")
	if _, ok := h.FindTextContains("Crowther"); !ok {
		t.Fatal("rules screen missing the BSD attribution credit line")
	}
}

// TestPlayScriptedWalk drives the canonical opening entirely through taps:
// well house → lamp/keys → grate → unlock → descend, asserting real state.
func TestPlayScriptedWalk(t *testing.T) {
	a, h := boot(t)
	h.TapXY(500, 900) // dismiss splash
	h.TapRect(findBtn(t, a.menuBtns, "Nytt spel").Rect)
	if a.screen != screenGame {
		t.Fatalf("after Nytt spel screen = %d, want game", a.screen)
	}
	shot(t, h, "04_room_start")
	assertNoOverflow(t, h, "start room")

	// Into the well house (east).
	h.TapRect(findBtn(t, a.exitBtns, "Öster").Rect)
	if a.st.Loc != story.LOC_BUILDING {
		t.Fatalf("after Öster loc = %d, want building", a.st.Loc)
	}
	shot(t, h, "05_wellhouse")
	assertNoOverflow(t, h, "well house")

	// Arm "Ta" and take the lamp, then the keys.
	h.TapRect(findBtn(t, a.verbBtns, "Ta").Rect)
	if !a.verbArmed || a.armedVerb != story.VerbTake {
		t.Fatal("Ta did not arm")
	}
	h.TapRect(findBtn(t, a.nounBtns, "Lamp").Rect)
	h.TapRect(findBtn(t, a.verbBtns, "Ta").Rect)
	h.TapRect(findBtn(t, a.nounBtns, "Keys").Rect)
	if !a.st.Carried[story.OBJ_LAMP] || !a.st.Carried[story.OBJ_KEYS] {
		t.Fatalf("failed to take lamp/keys via UI; carried=%v", story.CarriedObjects(a.st))
	}

	// Back to the road, then to the grate depression.
	h.TapRect(findBtn(t, a.exitBtns, "Väster").Rect)
	if a.st.Loc != story.LOC_START {
		t.Fatalf("west from building loc = %d, want start", a.st.Loc)
	}
	h.TapRect(findBtn(t, a.exitBtns, "Fördjupningen").Rect)
	if a.st.Loc != story.LOC_GRATE {
		t.Fatalf("depression loc = %d, want grate", a.st.Loc)
	}

	// The grate is locked: no descent exit yet.
	for _, b := range a.exitBtns {
		if b.Label == "Ner" {
			t.Fatal("descent exit shown while grate is still locked")
		}
	}
	// Unlock the grate with the keys.
	h.TapRect(findBtn(t, a.verbBtns, "Lås upp").Rect)
	h.TapRect(findBtn(t, a.nounBtns, "Grate").Rect)
	if a.st.ObjState[story.OBJ_GRATE] != story.GRATE_OPEN {
		t.Fatal("grate did not unlock via UI")
	}
	shot(t, h, "06_grate_unlocked")

	// Now descend.
	h.TapRect(findBtn(t, a.exitBtns, "Ner").Rect)
	if a.st.Loc != story.LOC_BELOWGRATE {
		t.Fatalf("descend loc = %d, want below grate", a.st.Loc)
	}

	// Autosave produced a slot that reloads to the same place.
	if reloaded, err := story.LoadFile(a.savePath); err != nil {
		t.Fatalf("autosave not readable: %v", err)
	} else if reloaded.Loc != story.LOC_BELOWGRATE {
		t.Fatalf("autosave loc = %d, want below grate", reloaded.Loc)
	}
}

// TestPlayDarkRoom renders the pitch-dark message and confirms the room reads as
// dark until the lamp is lit.
func TestPlayDarkRoom(t *testing.T) {
	a, h := boot(t)
	h.TapXY(500, 900)
	h.TapRect(findBtn(t, a.menuBtns, "Nytt spel").Rect)

	// Jump straight into the dark debris room carrying an unlit lamp.
	a.st = story.New()
	a.st.Loc = story.LOC_DEBRIS
	a.st.Carried[story.OBJ_LAMP] = true
	a.log = nil
	a.print(story.Describe(a.st))
	h.Draw()
	if _, ok := h.FindTextContains("pitch dark"); !ok {
		t.Fatal("dark room did not render the pitch-dark warning")
	}
	shot(t, h, "07_dark_room")
	assertNoOverflow(t, h, "dark room")

	// Light the lamp; the room description appears.
	h.TapRect(findBtn(t, a.verbBtns, "Tänd").Rect)
	h.TapRect(findBtn(t, a.nounBtns, "•Lamp").Rect)
	if story.IsDark(a.st) {
		t.Fatal("room still dark after lighting the lamp")
	}
	shot(t, h, "08_lit_room")
	assertNoOverflow(t, h, "lit debris room")
}

// TestPlayMap explores a few rooms, opens the Karta screen, and checks the map
// renders the visited nodes, the current-room highlight, and unexplored stubs.
func TestPlayMap(t *testing.T) {
	a, h := boot(t)
	h.TapXY(500, 900)
	h.TapRect(findBtn(t, a.menuBtns, "Nytt spel").Rect)

	// Walk a short loop so several rooms are visited.
	h.TapRect(findBtn(t, a.exitBtns, "Öster").Rect)  // -> well house
	h.TapRect(findBtn(t, a.exitBtns, "Väster").Rect) // -> road
	h.TapRect(findBtn(t, a.exitBtns, "Söder").Rect)  // -> valley
	if a.st.Loc != story.LOC_VALLEY {
		t.Fatalf("expected to be in the valley, at %d", a.st.Loc)
	}

	// Open the map.
	h.TapRect(a.mapBtn)
	if a.screen != screenMap {
		t.Fatalf("Karta did not open the map (screen=%d)", a.screen)
	}
	shot(t, h, "12_map")
	assertNoOverflow(t, h, "map")

	// Visited room labels must be on screen; a far unvisited room must not.
	if _, ok := h.FindText(story.MapLabels[story.LOC_BUILDING]); !ok {
		t.Error("map missing the visited well-house node")
	}
	if _, ok := h.FindText(story.MapLabels[story.LOC_NUGGET]); ok {
		t.Error("map is leaking an unvisited far cave room")
	}
	if _, ok := h.FindTextContains("du är här"); !ok {
		t.Error("map missing the 'you are here' marker")
	}

	// Tillbaka returns to play.
	h.TapRect(a.mapBack)
	if a.screen != screenGame {
		t.Fatalf("Tillbaka did not return to game (screen=%d)", a.screen)
	}

	// Render a fully-explored map so the whole cave graph is visible.
	for loc := range story.MapPositions {
		a.st.Visited[loc] = true
	}
	a.st.Loc = story.LOC_MISTHALL
	a.screen = screenMap
	h.Draw()
	shot(t, h, "13_map_full")
	assertNoOverflow(t, h, "full map")
}

// TestPlayWorstCaseAndScroll builds a crowded room (many exits + many objects)
// and a long transcript, then scrolls — the hardest layout to keep on-screen.
func TestPlayWorstCaseAndScroll(t *testing.T) {
	a, h := boot(t)
	h.TapXY(500, 900)
	h.TapRect(findBtn(t, a.menuBtns, "Nytt spel").Rect)

	// Below-the-grate has four exits; load it up with a full pack of objects so
	// the noun row wraps and the exit row is busy — the worst case for fit.
	a.st = story.New()
	a.st.Loc = story.LOC_DEBRIS
	a.st.ObjState[story.OBJ_LAMP] = story.LAMP_BRIGHT
	for _, id := range []story.ObjID{story.OBJ_LAMP, story.OBJ_KEYS, story.OBJ_CAGE,
		story.OBJ_FOOD, story.OBJ_BOTTLE, story.OBJ_NUGGET} {
		a.st.Carried[id] = true
		a.st.ObjAt[id] = story.LOC_NOWHERE
	}
	a.st.Known[story.MagicWordText[story.MotXYZZY]] = true

	// A long transcript to force scrolling.
	a.log = nil
	for i := 0; i < 40; i++ {
		a.print(story.Describe(a.st))
	}
	h.Draw()
	shot(t, h, "09_worstcase")
	assertNoOverflow(t, h, "worst-case room")

	// The exit row should carry the busy set of directions.
	if len(a.exitBtns) < 4 {
		t.Fatalf("expected a crowded exit row, got %v", labelsOf(a.exitBtns))
	}
	// The noun row must include carried items (wrapped across rows).
	if len(a.nounBtns) < 5 {
		t.Fatalf("expected many noun buttons, got %v", labelsOf(a.nounBtns))
	}

	// Scroll up to reveal older lines, then render again.
	top0 := a.scroll
	h.TapRect(a.scrollUp)
	h.TapRect(a.scrollUp)
	if a.scroll >= top0 {
		t.Fatalf("scroll up did not move the view (%d -> %d)", top0, a.scroll)
	}
	shot(t, h, "10_scrolled")
	assertNoOverflow(t, h, "scrolled transcript")

	// Open the Säg… magic-word popup and render it.
	h.TapRect(findBtn(t, a.auxBtns, "Säg…").Rect)
	if !a.sayOpen {
		t.Fatal("Säg… did not open")
	}
	h.Draw()
	shot(t, h, "11_say_popup")
	assertNoOverflow(t, h, "say popup")
	if len(a.sayBtns) == 0 {
		t.Fatal("say popup has no word buttons")
	}
}
