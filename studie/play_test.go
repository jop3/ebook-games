//go:build playtest

// Play/render tests for the mystery, run under the pure-Go inkview emulator via
// playtest/play.sh (gated by the build tag; never in the device build). The main
// test plays the WHOLE case start to finish through the real tap UI — gather
// every clue, make the four deductions (one unlocks the chemist), and win the
// accusation — asserting real state and rendering each screen.
package main

import (
	"os"
	"path/filepath"
	"testing"

	ink "github.com/dennwc/inkview"

	"studie/story"
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

func boot(t *testing.T) (*app, *ink.Harness) {
	t.Helper()
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	a.savePath = filepath.Join(t.TempDir(), "studie.sav")
	return a, h
}

func assertNoOverflow(t *testing.T, h *ink.Harness, where string) {
	t.Helper()
	for _, sp := range h.Texts() {
		if sp.Rect.Max.Y > usableH {
			t.Errorf("%s: text %q overflows bottom: y=%d > %d", where, sp.S, sp.Rect.Max.Y, usableH)
		}
	}
}

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

func hasBtn(btns []button, label string) bool {
	for _, b := range btns {
		if b.Label == label {
			return true
		}
	}
	return false
}

func labelsOf(btns []button) []string {
	out := make([]string, len(btns))
	for i, b := range btns {
		out[i] = b.Label
	}
	return out
}

// walk taps the exit with the given label and confirms the destination.
func walk(t *testing.T, a *app, h *ink.Harness, label string, dest story.LocID) {
	t.Helper()
	h.TapRect(findBtn(t, a.exitBtns, label).Rect)
	if a.st.Loc != dest {
		t.Fatalf("walk %q → loc %d, want %d", label, a.st.Loc, dest)
	}
}

// examine taps a noun (unarmed = examine) to record its clue.
func examine(t *testing.T, a *app, h *ink.Harness, noun string) {
	t.Helper()
	h.TapRect(findBtn(t, a.nounBtns, noun).Rect)
}

// combine opens is done on the notebook screen: tap the two clue rows for the
// given clue ids.
func combine(t *testing.T, a *app, h *ink.Harness, idA, idB string) {
	t.Helper()
	ia, ib := clueRow(t, a, idA), clueRow(t, a, idB)
	h.TapRect(a.clueBtns[ia].Rect)
	h.TapRect(a.clueBtns[ib].Rect)
}

func clueRow(t *testing.T, a *app, id string) int {
	t.Helper()
	for i, c := range a.st.Clues {
		if c.ID == id {
			return i
		}
	}
	t.Fatalf("clue %q not gathered; have %v", id, a.st.Clues)
	return -1
}

// TestPlayChrome renders the splash, menu, and rules.
func TestPlayChrome(t *testing.T) {
	a, h := boot(t)
	shot(t, h, "01_splash")

	h.TapXY(500, 900)
	a.hasSave = true
	h.Draw()
	shot(t, h, "02_menu")
	assertNoOverflow(t, h, "menu")

	h.TapRect(findBtn(t, a.menuBtns, "Regler").Rect)
	shot(t, h, "03_rules")
	assertNoOverflow(t, h, "rules")
	if _, ok := h.FindTextContains("Scarlet"); !ok {
		t.Error("rules missing the public-domain attribution note")
	}
}

// TestPlayFullCase solves the whole case through the UI and wins.
func TestPlayFullCase(t *testing.T) {
	a, h := boot(t)
	h.TapXY(500, 900)
	h.TapRect(findBtn(t, a.menuBtns, "Nytt fall").Rect)
	shot(t, h, "04_consulting_rooms")

	// Hub → street. The chemist must be HIDDEN until the poison is deduced.
	walk(t, a, h, "Gatan", story.LOC_STREET)
	if hasBtn(a.exitBtns, "Apoteket") {
		t.Fatal("the chemist should be gated until the poison is deduced")
	}
	shot(t, h, "05_street")

	// Hall: the burned letter + the landlady's account.
	walk(t, a, h, "In", story.LOC_HALL)
	examine(t, a, h, "Letter")
	examine(t, a, h, "Landlady")

	// Study: body, glass-ring, clock.
	walk(t, a, h, "Arbetsrummet", story.LOC_STUDY)
	examine(t, a, h, "Body")
	examine(t, a, h, "Ring")
	examine(t, a, h, "Clock")
	shot(t, h, "06_study")

	// Bedroom: the forced latch.
	walk(t, a, h, "Sovrummet", story.LOC_BEDROOM)
	examine(t, a, h, "Latch")

	// Yard: the boot-print.
	walk(t, a, h, "Arbetsrummet", story.LOC_STUDY)
	walk(t, a, h, "Hallen", story.LOC_HALL)
	walk(t, a, h, "Bakgården", story.LOC_YARD)
	examine(t, a, h, "Print")

	// Cabman: the timing.
	walk(t, a, h, "Hallen", story.LOC_HALL)
	walk(t, a, h, "Ut", story.LOC_STREET)
	walk(t, a, h, "Kusken", story.LOC_CABMAN)
	examine(t, a, h, "Cabman")
	walk(t, a, h, "Gatan", story.LOC_STREET)

	// Eight clues gathered; none from the chemist yet.
	for _, id := range []string{"letter", "visitor", "body", "ring", "clock", "latch", "boot", "cabtime"} {
		if !story.HasClue(a.st, id) {
			t.Fatalf("missing clue %q; have %v", id, a.st.Clues)
		}
	}

	// Notebook: deduce the poison, which unlocks the chemist.
	h.TapRect(a.bookBtn)
	shot(t, h, "07_notebook")
	assertNoOverflow(t, h, "notebook")
	combine(t, a, h, "body", "ring")
	if !a.st.Deductions["poison"] {
		t.Fatal("body+ring should deduce 'poison'")
	}
	h.TapRect(a.bookBack) // back to the street

	// The chemist is now reachable.
	if !hasBtn(a.exitBtns, "Apoteket") {
		t.Fatalf("the chemist should be unlocked after the poison deduction; exits=%v", labelsOf(a.exitBtns))
	}
	walk(t, a, h, "Apoteket", story.LOC_CHEMIST)
	examine(t, a, h, "Ledger")

	// Notebook: the remaining three deductions.
	h.TapRect(a.bookBtn)
	combine(t, a, h, "boot", "latch")
	combine(t, a, h, "clock", "cabtime")
	combine(t, a, h, "letter", "ledger")
	if n := story.DeductionCount(a.st); n != 4 {
		t.Fatalf("expected all 4 deductions, have %d: %v", n, a.st.Deductions)
	}
	shot(t, h, "08_notebook_full")
	assertNoOverflow(t, h, "notebook full")

	// Accuse. First a wrong motive is refused; then the correct charge wins.
	h.TapRect(a.accuseBtn)
	if a.screen != screenAccuse {
		t.Fatalf("Anklaga did not open the accusation (screen=%d)", a.screen)
	}
	h.TapRect(findBtn(t, a.accCulpritBtns, "crole").Rect)
	h.TapRect(findBtn(t, a.accMethodBtns, "poison").Rect)
	h.TapRect(findBtn(t, a.accMotiveBtns, "robbery").Rect)
	shot(t, h, "09_accuse")
	assertNoOverflow(t, h, "accuse")
	h.TapRect(a.accSubmit)
	if a.st.Won {
		t.Fatal("a wrong motive must not win")
	}
	if a.accResult == "" {
		t.Fatal("a refused accusation should explain itself")
	}

	// Correct the motive and charge again → win + resolution.
	h.TapRect(findBtn(t, a.accMotiveBtns, "debt").Rect)
	h.TapRect(a.accSubmit)
	if !a.st.Won || a.screen != screenResolution {
		t.Fatalf("the correct supported charge should win (won=%v screen=%d)", a.st.Won, a.screen)
	}
	shot(t, h, "10_resolution")
	assertNoOverflow(t, h, "resolution")
	if _, ok := h.FindTextContains("Case closed"); !ok {
		t.Error("resolution screen should show the closing text")
	}
}
