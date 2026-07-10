// Command lasordning ("Läsordning" = reading order) is a PocketBook Verse Pro
// app that reads the device's book library, works out which books belong to a
// series and in what order, and shows them so you always know where to start
// and how to continue.
//
// Series info is resolved through a chain: the library DB's own series columns,
// then a heuristic parse of the title, then online lookups (Wikidata, then the
// Wikipedia infobox). You can correct any book's number by hand, which is saved
// back to the device library.
//
// Pure logic (model, detection, grouping) lives in the ink-free ./series
// package so it is unit-tested without the SDK. This file plus ui.go and the
// screen files handle DB access, networking, rendering and input.
package main

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"time"

	ink "github.com/dennwc/inkview"

	"lasordning/series"
)

type screen int

const (
	screenList   screen = iota // series + standalone overview
	screenDetail               // one series, books in order
	screenEdit                 // set a book's number in its series
)

type app struct {
	fonts  *Fonts
	screen screen

	books   []series.Book
	grouped series.Grouped
	loadErr error

	list   *listView
	detail *detailView
	edit   *editView

	cache   *SeriesCache
	appDir  string
	buttons []Button
	updates int // partial-update counter for periodic FullUpdate

	downPt    image.Point // pointer-down position (for swipe detection)
	downValid bool
}

func main() {
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}
	if err := ink.Run(&app{}); err != nil {
		panic(err)
	}
}

// --- ink.App ----------------------------------------------------------------

func (a *app) Init() error {
	a.fonts = InitFonts()
	a.appDir = "."
	if exe, err := os.Executable(); err == nil {
		a.appDir = filepath.Dir(exe)
	}
	a.cache = LoadCache(a.appDir)
	a.reload()
	a.list = &listView{}
	a.screen = screenList
	ink.Repaint()
	return nil
}

func (a *app) Close() error {
	if a.fonts != nil {
		a.fonts.Close()
	}
	return nil
}

// reload reads the library and re-derives series info offline (DB metadata +
// title heuristics). Online enrichment happens on demand from the detail view.
func (a *app) reload() {
	books, err := LoadBooks(dbPath)
	a.loadErr = err
	// Fill in series info from the title where the DB had none.
	for i := range books {
		if !books[i].HasSeries() {
			if s := series.DetectFromTitle(books[i].DisplayTitle()); s.Name != "" {
				books[i].Series = s
			}
		}
	}
	a.books = books
	a.grouped = series.Group(books)
}

func (a *app) Draw() {
	screen := ink.ScreenSize()
	switch a.screen {
	case screenList:
		a.buttons = a.list.draw(screen, a.fonts, a.grouped, a.loadErr)
	case screenDetail:
		a.buttons = a.detail.draw(screen, a.fonts)
	case screenEdit:
		a.buttons = a.edit.draw(screen, a.fonts)
	}
	a.periodicUpdate()
}

// periodicUpdate does a clean FullUpdate every few repaints to clear e-ink
// ghosting, and a fast PartialUpdate otherwise.
func (a *app) periodicUpdate() {
	a.updates++
	if a.updates%4 == 0 {
		ink.FullUpdate()
	} else {
		ink.FullUpdate() // list/detail change wholesale; keep it simple and clean
	}
}

// --- input ------------------------------------------------------------------

// swipeThreshold is the minimum vertical finger travel (px) to treat a gesture
// as a scroll swipe rather than a tap. ~110px ≈ one list row on this screen.
const swipeThreshold = 110

func (a *app) Pointer(e ink.PointerEvent) bool {
	switch e.State {
	case ink.PointerDown:
		a.downPt = e.Point
		a.downValid = true
		return false
	case ink.PointerUp:
		if a.downValid {
			a.downValid = false
			dy := e.Point.Y - a.downPt.Y
			if dy >= swipeThreshold || dy <= -swipeThreshold {
				return a.handleSwipe(dy)
			}
		}
		return a.handleTap(e.Point)
	}
	return false
}

func (a *app) Touch(e ink.TouchEvent) bool {
	switch e.State {
	case ink.TouchDown:
		a.downPt = e.Point
		a.downValid = true
		return false
	case ink.TouchUp:
		if a.downValid {
			a.downValid = false
			dy := e.Point.Y - a.downPt.Y
			if dy >= swipeThreshold || dy <= -swipeThreshold {
				return a.handleSwipe(dy)
			}
		}
		return a.handleTap(e.Point)
	}
	return false
}

// handleSwipe scrolls the active list by roughly the number of rows the finger
// travelled. Swiping UP (finger moves up, dy<0) advances to later items;
// swiping DOWN goes back — the natural "drag the page" direction. rows is
// negated because dragging the page up (dy<0) should increase the top index.
func (a *app) handleSwipe(dy int) bool {
	const rowPx = 110
	rows := -dy / rowPx
	if rows == 0 {
		if dy < 0 {
			rows = 1
		} else {
			rows = -1
		}
	}
	switch a.screen {
	case screenList:
		a.list.scrollRows(rows)
	case screenDetail:
		a.detail.scrollRows(rows)
	default:
		return false
	}
	ink.Repaint()
	return true
}

func (a *app) handleTap(p image.Point) bool {
	switch a.screen {
	case screenList:
		return a.tapList(p)
	case screenDetail:
		return a.tapDetail(p)
	case screenEdit:
		return a.tapEdit(p)
	}
	return false
}

func (a *app) tapList(p image.Point) bool {
	// Buttons first.
	for _, b := range a.buttons {
		if b.Hit(p) {
			switch b.ID {
			case "up":
				a.list.scroll(-1)
			case "down":
				a.list.scroll(1)
			case "reload":
				a.reload()
			case "syncall":
				a.list.status = "Hämtar alla serier…"
				ink.Repaint()
				a.enrichAll()
				a.list.status = "Sparat " + itoa(a.cache.Count()) + " serier offline"
			}
			ink.Repaint()
			return true
		}
	}
	// A series row?
	if gi, ok := a.list.seriesAt(p); ok {
		a.openDetail(a.grouped.Series[gi])
		ink.Repaint()
		return true
	}
	return false
}

// openDetail builds the detail view for a series, merging in any cached full
// series list so missing books show up (offline-friendly).
func (a *app) openDetail(g series.SeriesGroup) {
	full, _ := a.cache.Get(g.Name)
	a.detail = &detailView{
		group: g,
		full:  full,
		rows:  series.Merge(g.Books, full),
	}
	a.screen = screenDetail
}

func (a *app) tapDetail(p image.Point) bool {
	for _, b := range a.buttons {
		if b.Hit(p) {
			switch b.ID {
			case "back":
				a.screen = screenList
			case "up":
				a.detail.scroll(-1)
			case "down":
				a.detail.scroll(1)
			case "online":
				a.enrichDetail()
			}
			ink.Repaint()
			return true
		}
	}
	// A book row -> edit its number.
	if bi, ok := a.detail.bookAt(p); ok {
		a.edit = &editView{
			book:   a.detail.group.Books[bi],
			series: a.detail.group.Name,
		}
		a.edit.number = int(a.detail.group.Books[bi].Series.Number)
		a.screen = screenEdit
		ink.Repaint()
		return true
	}
	return false
}

// enrichDetail fetches the complete book list for the current series online,
// caches it to disk (so it is available offline later), and rebuilds the merged
// owned+missing rows.
func (a *app) enrichDetail() {
	a.detail.status = "Hämtar hela serien…"
	ink.Repaint()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var report LookupReport
	full := SeriesBooks(ctx, a.detail.group.Name, &report, seedTitles(a.detail.group.Books)...)
	if len(full.Books) == 0 {
		msg := "Hittade inte hela serien online."
		for _, ln := range report.Lines {
			msg += "  •  " + ln
		}
		msg += "   Du kan sätta ordningen själv genom att trycka på en bok."
		a.detail.status = msg
		return
	}
	if err := a.cache.Put(full); err != nil {
		a.detail.status = "Kunde inte spara cache: " + err.Error()
		// still show what we fetched this session
	} else {
		a.detail.status = ""
	}
	a.detail.full = full
	a.detail.rows = series.Merge(a.detail.group.Books, full)
}

// enrichAll fetches full lists for EVERY series at once (the "load everything
// while online, then browse offline" workflow) and caches them.
func (a *app) enrichAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	for _, g := range a.grouped.Series {
		if _, ok := a.cache.Get(g.Name); ok {
			continue // already cached
		}
		var rep LookupReport
		full := SeriesBooks(ctx, g.Name, &rep, seedTitles(g.Books)...)
		if len(full.Books) > 0 {
			_ = a.cache.Put(full)
		}
	}
}

func (a *app) tapEdit(p image.Point) bool {
	for _, b := range a.buttons {
		if b.Hit(p) {
			switch b.ID {
			case "minus":
				if a.edit.number > 0 {
					a.edit.number--
				}
			case "plus":
				a.edit.number++
			case "save":
				a.saveEdit()
				a.screen = screenDetail
			case "cancel":
				a.screen = screenDetail
			}
			ink.Repaint()
			return true
		}
	}
	return false
}

// saveEdit writes the chosen number (and the series name) back to the DB and
// updates the in-memory model.
func (a *app) saveEdit() {
	name := a.edit.series
	num := float64(a.edit.number)
	if err := SaveSeries(dbPath, a.edit.book.ID, name, num); err != nil {
		a.edit.err = err.Error()
		return
	}
	for i := range a.books {
		if a.books[i].ID == a.edit.book.ID {
			a.books[i].Series = series.Series{Name: name, Number: num, Source: series.SourceManual}
		}
	}
	a.grouped = series.Group(a.books)
	for _, g := range a.grouped.Series {
		if g.Name == name {
			a.detail.group = g
			a.detail.rows = series.Merge(g.Books, a.detail.full)
			break
		}
	}
}

// seedTitles returns owned book titles to use as chain-walk seeds for the
// Wikipedia series-reconstruction fallback.
func seedTitles(books []series.Book) []string {
	out := make([]string, 0, len(books))
	for _, b := range books {
		out = append(out, b.DisplayTitle())
	}
	return out
}

// unused ink.App methods.
func (a *app) Key(e ink.KeyEvent) bool            { return false }
func (a *app) Orientation(o ink.Orientation) bool { return false }
