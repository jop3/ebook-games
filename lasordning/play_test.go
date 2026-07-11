//go:build playtest

package main

// Headless play tests for Läsordning under the pure-Go inkview emulator
// (playtest/play.sh lasordning). The app is a library utility, not a game, so
// the suite covers what CAN run off-device: booting without the device DB
// (the error screen), list/detail/edit navigation through real taps on a
// constructed in-memory library, hardware-Back navigation, and swipe
// scrolling. The device-DB and online paths are exercised on hardware only.

import (
	"image"
	"testing"

	ink "github.com/dennwc/inkview"

	"lasordning/series"
)

func bootApp(t *testing.T) (*ink.Harness, *app) {
	t.Helper()
	a := &app{}
	h, err := ink.Boot(a)
	if err != nil {
		t.Fatal(err)
	}
	return h, a
}

// loadFake replaces the (unreadable off-device) library with a small
// in-memory one: one two-book series and one standalone book.
func loadFake(a *app) {
	books := []series.Book{
		{ID: 1, Title: "Vintern", Author: "Anna Berg", FirstAuthor: "Berg, Anna",
			Series: series.Series{Name: "Årstiderna", Number: 1, Source: series.SourceMetadata}},
		{ID: 2, Title: "Sommaren", Author: "Anna Berg", FirstAuthor: "Berg, Anna",
			Series: series.Series{Name: "Årstiderna", Number: 2, Source: series.SourceMetadata}},
		{ID: 3, Title: "Ensam bok", Author: "Carl Dahl", FirstAuthor: "Dahl, Carl"},
	}
	a.books = books
	a.loadErr = nil
	a.grouped = series.Group(books)
}

// --- boot without the device library -----------------------------------------

func TestPlayLasordningBootsWithoutLibrary(t *testing.T) {
	h, a := bootApp(t)
	if a.loadErr == nil {
		t.Skip("device library unexpectedly present; error path not reachable")
	}
	if _, ok := h.FindText("Läsordning"); !ok {
		t.Fatal("title should be visible on the list screen")
	}
	if _, ok := h.FindTextContains("Kunde inte läsa biblioteket"); !ok {
		t.Fatal("the load error should be reported to the user")
	}
	// Uppdatera retries the load; still failing, but must not crash and must
	// keep the error screen up.
	if err := h.TapText("Uppdatera"); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.FindTextContains("Kunde inte läsa biblioteket"); !ok {
		t.Fatal("after a failed reload the error should still be shown")
	}
}

// --- list -> detail -> edit navigation via real taps --------------------------

func TestPlayLasordningNavigation(t *testing.T) {
	h, a := bootApp(t)
	loadFake(a)
	h.Draw()

	if _, ok := h.FindText("Serier"); !ok {
		t.Fatal("the Serier section header should be visible")
	}
	if _, ok := h.FindTextContains("Fristående"); !ok {
		t.Fatal("the standalone section header should be visible")
	}

	// Tap the series row -> detail view.
	if err := h.TapTextContains("Årstiderna"); err != nil {
		t.Fatal(err)
	}
	if a.screen != screenDetail {
		t.Fatalf("tapping a series row should open the detail view, screen=%v", a.screen)
	}
	if _, ok := h.FindText("Vintern"); !ok {
		t.Fatal("the detail view should list the owned books")
	}
	if len(a.detail.bookRects) != 2 {
		t.Fatalf("both owned books should be tappable, got %d rects", len(a.detail.bookRects))
	}

	// Tap the first book row -> edit view, seeded with its current number.
	if !h.TapRect(a.detail.bookRects[0]) {
		t.Fatal("tapping an owned book row should be handled")
	}
	if a.screen != screenEdit {
		t.Fatalf("tapping a book should open the edit view, screen=%v", a.screen)
	}
	if a.edit.number != 1 {
		t.Fatalf("edit view should start at the book's series number 1, got %d", a.edit.number)
	}

	// + and − adjust the number; − clamps at 0.
	if err := h.TapText("+"); err != nil {
		t.Fatal(err)
	}
	if a.edit.number != 2 {
		t.Fatalf("+ should increment to 2, got %d", a.edit.number)
	}
	for i := 0; i < 4; i++ {
		if err := h.TapText("−"); err != nil {
			t.Fatal(err)
		}
	}
	if a.edit.number != 0 {
		t.Fatalf("− should clamp at 0, got %d", a.edit.number)
	}

	// Avbryt returns to the detail view without saving.
	if err := h.TapText("Avbryt"); err != nil {
		t.Fatal(err)
	}
	if a.screen != screenDetail {
		t.Fatalf("Avbryt should return to the detail view, screen=%v", a.screen)
	}

	// Tillbaka returns to the list.
	if err := h.TapText("Tillbaka"); err != nil {
		t.Fatal(err)
	}
	if a.screen != screenList {
		t.Fatalf("Tillbaka should return to the list, screen=%v", a.screen)
	}
}

// --- hardware Back walks up one screen at a time ------------------------------

func TestPlayLasordningBackKey(t *testing.T) {
	h, a := bootApp(t)
	loadFake(a)
	h.Draw()

	if err := h.TapTextContains("Årstiderna"); err != nil {
		t.Fatal(err)
	}
	if !h.TapRect(a.detail.bookRects[0]) {
		t.Fatal("could not open the edit view")
	}
	if a.screen != screenEdit {
		t.Fatalf("expected edit view, screen=%v", a.screen)
	}

	if !h.Back() {
		t.Fatal("Back on the edit view should be handled")
	}
	if a.screen != screenDetail {
		t.Fatalf("Back should go edit -> detail, screen=%v", a.screen)
	}
	if !h.Back() {
		t.Fatal("Back on the detail view should be handled")
	}
	if a.screen != screenList {
		t.Fatalf("Back should go detail -> list, screen=%v", a.screen)
	}
	if h.Back() {
		t.Fatal("Back on the list must stay unhandled so the firmware exits the app")
	}
}

// --- swipe scrolling ----------------------------------------------------------

func TestPlayLasordningSwipeScrolls(t *testing.T) {
	h, a := bootApp(t)
	loadFake(a)
	h.Draw()

	// A short tap-like release (below the swipe threshold) must NOT scroll.
	a.Pointer(ink.PointerEvent{Point: image.Pt(500, 800), State: ink.PointerDown})
	a.Pointer(ink.PointerEvent{Point: image.Pt(500, 800-swipeThreshold+1), State: ink.PointerUp})
	h.Draw()
	if a.list.top != 0 {
		t.Fatalf("sub-threshold drag should not scroll, top=%d", a.list.top)
	}

	// Dragging the page up (finger travels up) advances the list.
	a.Pointer(ink.PointerEvent{Point: image.Pt(500, 900), State: ink.PointerDown})
	a.Pointer(ink.PointerEvent{Point: image.Pt(500, 900-swipeThreshold), State: ink.PointerUp})
	h.Draw()
	if a.list.top != 1 {
		t.Fatalf("an upward swipe should scroll down one row, top=%d", a.list.top)
	}

	// Dragging the page down goes back, clamped at 0.
	for i := 0; i < 3; i++ {
		a.Pointer(ink.PointerEvent{Point: image.Pt(500, 600), State: ink.PointerDown})
		a.Pointer(ink.PointerEvent{Point: image.Pt(500, 600+2*swipeThreshold), State: ink.PointerUp})
	}
	h.Draw()
	if a.list.top != 0 {
		t.Fatalf("downward swipes should scroll back and clamp at 0, top=%d", a.list.top)
	}
}
