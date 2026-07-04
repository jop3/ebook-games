//go:build playtest

package main

// Headless PLAYTHROUGH tests for Ordskrav (the Swedish anagram game). They drive
// the real touch path and check the gameplay against the rules as written (see
// rulesParagraphs in ui.go): build valid Swedish words (>= 3 letters) from the
// shuffled tiles; each accepted word scores its length; duplicates, too-short
// and non-words are rejected. Runs under the pure-Go inkview emulator
// (playtest/play.sh).

import (
	"image"
	"os"
	"testing"

	ink "github.com/dennwc/inkview"

	"anagram/game"
)

// --- helpers ----------------------------------------------------------------

func bootToMenu(t *testing.T) (*ink.Harness, *app) {
	t.Helper()
	a := newApp()
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	h.TapXY(500, 700)
	if a.scr != screenMenu {
		t.Fatalf("splash tap did not open menu, scr=%v", a.scr)
	}
	return h, a
}

func startRound(t *testing.T, h *ink.Harness, a *app) {
	t.Helper()
	if err := h.TapText("SPELA"); err != nil {
		t.Fatalf("no SPELA button: %v", err)
	}
	if a.scr != screenPlay || a.round == nil {
		t.Fatalf("did not enter play, scr=%v", a.scr)
	}
}

// tileCenters returns the screen centre of each tile. Valid on a fresh round
// (all tiles unused), where hits[0..n-1] are the tiles in Letters() order.
func tileCenters(a *app) []image.Point {
	ls := a.round.Letters()
	pts := make([]image.Point, len(ls))
	for i := 0; i < len(ls) && i < len(a.hits); i++ {
		r := a.hits[i].r
		pts[i] = image.Pt((r.x0+r.x1)/2, (r.y0+r.y1)/2)
	}
	return pts
}

// formWord taps the tiles that spell w (using each tile at most once), matching
// letters against the currently-unused tiles.
func formWord(t *testing.T, h *ink.Harness, a *app, centers []image.Point, w string) {
	t.Helper()
	letters := a.round.Letters()
	for _, ch := range w {
		idx := -1
		for i := range letters {
			if letters[i] == ch && !a.round.LetterUsed(i) {
				idx = i
				break
			}
		}
		if idx < 0 {
			t.Fatalf("cannot form %q: no free tile for %q", w, string(ch))
		}
		h.Tap(centers[idx])
	}
}

func submit(t *testing.T, h *ink.Harness, a *app) {
	t.Helper()
	if err := h.TapText("LÄMNA IN"); err != nil {
		t.Fatalf("no LÄMNA IN button: %v", err)
	}
}

func texts(h *ink.Harness) []string {
	var out []string
	for _, s := range h.Texts() {
		out = append(out, s.S)
	}
	return out
}

// sumLen returns the total length of a list of words (the expected score).
func sumLen(ws []string) int {
	n := 0
	for _, w := range ws {
		n += len([]rune(w))
	}
	return n
}

// --- Form valid words; score = sum of their lengths -------------------------

func TestPlayAnagramFormWordsAndScore(t *testing.T) {
	h, a := bootToMenu(t)
	startRound(t, h, a)
	centers := tileCenters(a)

	sols := a.dict.AllSolutions(a.round.BaseWord(), game.MinWordLen)
	if len(sols) == 0 {
		t.Skip("round has no solutions")
	}
	// RULE consistency: TotalSolutions matches the dictionary enumeration.
	if a.round.TotalSolutions() != len(sols) {
		t.Fatalf("TotalSolutions()=%d, dictionary says %d", a.round.TotalSolutions(), len(sols))
	}

	// Submit up to three distinct solutions.
	want := sols
	if len(want) > 3 {
		want = want[:3]
	}
	for _, w := range want {
		formWord(t, h, a, centers, w)
		before := a.round.Score()
		submit(t, h, a)
		if a.round.Score() != before+len([]rune(w)) {
			t.Fatalf("word %q scored %d, expected +%d", w, a.round.Score()-before, len([]rune(w)))
		}
	}
	if a.round.Score() != sumLen(want) {
		t.Fatalf("total score %d != sum of word lengths %d", a.round.Score(), sumLen(want))
	}
	if len(a.round.Found()) != len(want) {
		t.Fatalf("found %d words, submitted %d", len(a.round.Found()), len(want))
	}
}

// --- RULE: a word already found is rejected, score unchanged ----------------

func TestPlayAnagramAlreadyFound(t *testing.T) {
	h, a := bootToMenu(t)
	startRound(t, h, a)
	centers := tileCenters(a)

	sols := a.dict.AllSolutions(a.round.BaseWord(), game.MinWordLen)
	if len(sols) == 0 {
		t.Skip("no solutions")
	}
	w := sols[0]

	formWord(t, h, a, centers, w)
	submit(t, h, a)
	scoreAfter := a.round.Score()
	foundAfter := len(a.round.Found())

	// Submit it again -> rejected.
	formWord(t, h, a, centers, w)
	submit(t, h, a)
	if a.round.Score() != scoreAfter || len(a.round.Found()) != foundAfter {
		t.Fatalf("duplicate word changed score/found (%d/%d -> %d/%d)",
			scoreAfter, foundAfter, a.round.Score(), len(a.round.Found()))
	}
	if _, ok := h.FindTextContains("redan hittat"); !ok {
		t.Fatalf("no 'already found' message; visible: %v", texts(h))
	}
}

// --- RULE: a too-short entry is rejected ------------------------------------

func TestPlayAnagramTooShort(t *testing.T) {
	h, a := bootToMenu(t)
	startRound(t, h, a)
	centers := tileCenters(a)

	// Tap two tiles (below MinWordLen) and submit.
	h.Tap(centers[0])
	h.Tap(centers[1])
	submit(t, h, a)
	if len(a.round.Found()) != 0 || a.round.Score() != 0 {
		t.Fatal("a two-letter entry scored")
	}
	if _, ok := h.FindTextContains("För kort"); !ok {
		t.Fatalf("no 'too short' message; visible: %v", texts(h))
	}
}

// --- BLANDA (shuffle) keeps the letters; RENSA clears the input -------------

func TestPlayAnagramShuffleAndClear(t *testing.T) {
	h, a := bootToMenu(t)
	startRound(t, h, a)
	centers := tileCenters(a)

	// Build a partial input, then RENSA clears it.
	h.Tap(centers[0])
	h.Tap(centers[1])
	if a.round.Input() == "" {
		t.Fatal("tiles did not enter input")
	}
	if err := h.TapText("RENSA"); err != nil {
		t.Fatalf("no RENSA: %v", err)
	}
	if a.round.Input() != "" {
		t.Fatalf("RENSA did not clear the input (%q)", a.round.Input())
	}

	// BLANDA preserves the multiset of letters.
	before := sortedRunes(a.round.Letters())
	if err := h.TapText("BLANDA"); err != nil {
		t.Fatalf("no BLANDA: %v", err)
	}
	after := sortedRunes(a.round.Letters())
	if string(before) != string(after) {
		t.Fatalf("BLANDA changed the letter multiset: %q -> %q", string(before), string(after))
	}
}

func sortedRunes(rs []rune) []rune {
	out := append([]rune(nil), rs...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// --- Give up, new round, quit, rules ----------------------------------------

func TestPlayAnagramGiveUpQuitRules(t *testing.T) {
	h, a := bootToMenu(t)
	startRound(t, h, a)

	if err := h.TapText("GE UPP"); err != nil {
		t.Fatalf("no GE UPP: %v", err)
	}
	if a.scr != screenGiveUp {
		t.Fatalf("GE UPP did not open the give-up screen, scr=%v", a.scr)
	}
	if err := h.TapText("NYTT SPEL"); err != nil {
		t.Fatalf("no NYTT SPEL: %v", err)
	}
	if a.scr != screenPlay {
		t.Fatalf("NYTT SPEL did not start a new round, scr=%v", a.scr)
	}

	h.Back()
	if a.scr != screenMenu {
		t.Fatalf("Back did not return to menu, scr=%v", a.scr)
	}

	if err := h.TapText("REGLER"); err != nil {
		t.Fatalf("no REGLER: %v", err)
	}
	if a.scr != screenRules {
		t.Fatalf("REGLER did not open rules, scr=%v", a.scr)
	}
	if _, ok := h.FindTextContains("giltiga svenska ord"); !ok {
		t.Fatalf("rules text missing; visible: %v", texts(h))
	}
	h.Back()
	if a.scr != screenMenu {
		t.Fatalf("Back did not leave rules, scr=%v", a.scr)
	}
}

// --- Screenshot -------------------------------------------------------------

func TestPlayAnagramScreenshot(t *testing.T) {
	dir := os.Getenv("PLAYTEST_SHOTS")
	if dir == "" {
		t.Skip("set PLAYTEST_SHOTS to capture a screenshot")
	}
	h, a := bootToMenu(t)
	startRound(t, h, a)
	centers := tileCenters(a)
	sols := a.dict.AllSolutions(a.round.BaseWord(), game.MinWordLen)
	n := len(sols)
	if n > 4 {
		n = 4
	}
	for _, w := range sols[:n] {
		formWord(t, h, a, centers, w)
		submit(t, h, a)
	}
	if err := h.Screenshot(dir + "/anagram.png"); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}
