//go:build playtest

// Play/render tests for the mystery, run under the pure-Go inkview emulator via
// playtest/play.sh (gated by the build tag; never in the device build). They
// drive the real tap UI through the deduction loop — examine a clue, open the
// notebook, combine two clues into a conclusion — render every screen to PNG,
// and check nothing overflows the 1340px drawable height.
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

func labelsOf(btns []button) []string {
	out := make([]string, len(btns))
	for i, b := range btns {
		out[i] = b.Label
	}
	return out
}

// TestPlayChrome renders the splash, menu, and rules for the mystery.
func TestPlayChrome(t *testing.T) {
	a, h := boot(t)
	shot(t, h, "01_splash")

	h.TapXY(500, 900)
	a.hasSave = true
	h.Draw()
	shot(t, h, "02_menu")
	assertNoOverflow(t, h, "menu")

	h.TapRect(findBtn(t, a.menuBtns, "Regler").Rect)
	if a.screen != screenRules {
		t.Fatalf("Regler did not open rules (screen=%d)", a.screen)
	}
	shot(t, h, "03_rules")
	assertNoOverflow(t, h, "rules")
	if _, ok := h.FindTextContains("Scarlet"); !ok {
		t.Error("rules missing the public-domain attribution note")
	}
}

// TestPlayDeductionLoop is the heart of the mystery: walk to the scene, examine
// clues (which enter the notebook), then combine two into a deduction.
func TestPlayDeductionLoop(t *testing.T) {
	a, h := boot(t)
	h.TapXY(500, 900)
	h.TapRect(findBtn(t, a.menuBtns, "Nytt fall").Rect)
	if a.screen != screenGame {
		t.Fatalf("Nytt fall did not start the game (screen=%d)", a.screen)
	}
	shot(t, h, "04_consulting_rooms")
	assertNoOverflow(t, h, "hub")

	// Consulting-rooms → street → into the lodging house → study.
	h.TapRect(findBtn(t, a.exitBtns, "Gatan").Rect)
	if a.st.Loc != story.LOC_STREET {
		t.Fatalf("expected the street, at %d", a.st.Loc)
	}
	shot(t, h, "05_street")
	assertNoOverflow(t, h, "street")

	h.TapRect(findBtn(t, a.exitBtns, "In").Rect)           // hall
	h.TapRect(findBtn(t, a.exitBtns, "Arbetsrummet").Rect) // study
	if a.st.Loc != story.LOC_STUDY {
		t.Fatalf("expected the study, at %d", a.st.Loc)
	}
	shot(t, h, "06_study")
	assertNoOverflow(t, h, "study")

	// Examine the body and the scratched ring (unarmed noun tap = examine).
	h.TapRect(findBtn(t, a.nounBtns, "Body").Rect)
	h.TapRect(findBtn(t, a.nounBtns, "Ring").Rect)
	if !story.HasClue(a.st, "body") || !story.HasClue(a.st, "ring") {
		t.Fatalf("examining should record clues; have %v", a.st.Clues)
	}

	// Down to the yard for the boot-print.
	h.TapRect(findBtn(t, a.exitBtns, "Hallen").Rect) // back to hall
	h.TapRect(findBtn(t, a.exitBtns, "Bakgården").Rect)
	if a.st.Loc != story.LOC_YARD {
		t.Fatalf("expected the yard, at %d", a.st.Loc)
	}
	h.TapRect(findBtn(t, a.nounBtns, "Print").Rect)
	if !story.HasClue(a.st, "boot") {
		t.Fatal("examining the boot-print should record the 'boot' clue")
	}

	// Open the notebook: three clues gathered.
	h.TapRect(a.bookBtn)
	if a.screen != screenNotebook {
		t.Fatalf("Blocket did not open the notebook (screen=%d)", a.screen)
	}
	shot(t, h, "07_notebook")
	assertNoOverflow(t, h, "notebook")
	if len(a.clueBtns) != 3 {
		t.Fatalf("expected 3 clue rows, got %v", labelsOf(a.clueBtns))
	}

	// Combine boot + ring → the 'entry' deduction. Find their row indices.
	bootRow := clueRow(t, a, "boot")
	ringRow := clueRow(t, a, "ring")
	h.TapRect(a.clueBtns[bootRow].Rect) // select
	h.TapRect(a.clueBtns[ringRow].Rect) // combine
	if !a.st.Deductions["entry"] {
		t.Fatalf("boot+ring should have produced the 'entry' deduction; deductions=%v", a.st.Deductions)
	}
	shot(t, h, "08_deduction")
	assertNoOverflow(t, h, "notebook after deduction")
	if _, ok := h.FindTextContains("yard door"); !ok {
		t.Error("the deduction text should be shown on the notebook")
	}

	// A non-matching pair yields nothing.
	clockKnown := story.HasClue(a.st, "clock")
	if clockKnown {
		t.Fatal("clock clue should not be known yet in this walk")
	}
}

// clueRow returns the notebook row index whose clue has the given id.
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
