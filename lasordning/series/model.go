// Package series holds the pure, SDK-free logic for the Läsordning app:
// the book data model, series/number detection from titles, and the grouping
// that turns a flat book list into an ordered "reading order" view.
//
// Nothing here imports the inkview SDK, so it is fully unit-testable with the
// portable Go toolchain (no cgo, no device).
package series

import "sort"

// Source records where a book's series information came from, so the UI can
// show how confident we are and the user knows what to trust / fix.
type Source int

const (
	SourceNone     Source = iota // no series known
	SourceMetadata               // books_impl.series / numinseries (device DB)
	SourceTitle                  // parsed out of the title heuristically
	SourceWikidata               // Wikidata P179 + P1545 ordinal
	SourceWikipedia              // Wikipedia infobox series / #N
	SourceManual                 // user set it by hand in the app
)

func (s Source) String() string {
	switch s {
	case SourceMetadata:
		return "metadata"
	case SourceTitle:
		return "titel"
	case SourceWikidata:
		return "Wikidata"
	case SourceWikipedia:
		return "Wikipedia"
	case SourceManual:
		return "manuellt"
	default:
		return "okänd"
	}
}

// Book is one row from the device library, enriched with series info.
type Book struct {
	ID          int64  // books_impl.id
	Title       string // books_impl.title (falls back to filename if empty)
	Author      string // books_impl.author (display)
	FirstAuthor string // books_impl.firstauthor ("Surname, First" — used for sort/group)
	Ext         string // file extension (epub/fb2/…)

	Series Series // resolved series info (may be empty)
}

// Series is the resolved series membership for a book.
type Series struct {
	Name   string  // "" means standalone / unknown
	Number float64 // position in series; 0 means "unnumbered but in series"
	Source Source
}

// HasSeries reports whether the book is known to belong to a series.
func (b Book) HasSeries() bool { return b.Series.Name != "" }

// DisplayTitle returns the title, or the ext-stripped filename fallback.
func (b Book) DisplayTitle() string {
	if b.Title != "" {
		return b.Title
	}
	return "(namnlös)"
}

// --- Grouping ---------------------------------------------------------------

// SeriesGroup is one series with its books in reading order.
type SeriesGroup struct {
	Author string // display author (the group's dominant author)
	Name   string // series name
	Books  []Book // sorted by Number (unnumbered last, then title)
	Source Source // strongest source among the books
}

// Count returns how many books we have on the device for this series.
func (g SeriesGroup) Count() int { return len(g.Books) }

// MaxNumber returns the highest known series number (0 if none numbered).
func (g SeriesGroup) MaxNumber() float64 {
	var m float64
	for _, b := range g.Books {
		if b.Series.Number > m {
			m = b.Series.Number
		}
	}
	return m
}

// MissingBefore returns the list of integer positions between 1 and the max
// known number that we do NOT have a book for — i.e. gaps in the series such
// as "you have #1 and #3 but are missing #2". Only meaningful when numbers are
// present. Returns nil if the series has no numbering.
func (g SeriesGroup) MissingBefore() []int {
	max := int(g.MaxNumber())
	if max <= 1 {
		return nil
	}
	have := make(map[int]bool)
	for _, b := range g.Books {
		if b.Series.Number > 0 {
			have[int(b.Series.Number)] = true
		}
	}
	var gaps []int
	for i := 1; i <= max; i++ {
		if !have[i] {
			gaps = append(gaps, i)
		}
	}
	return gaps
}

// Grouped is the full organised view the UI renders.
type Grouped struct {
	Series     []SeriesGroup // series, sorted by author then series name
	Standalone []Book        // books with no series, sorted by author then title
}

// Group turns a flat list of books into the reading-order view.
//
// Books with a series are bucketed by series name, ordered within the bucket by
// Number (unnumbered books sink to the bottom, ordered by title). Series groups
// are ordered by their dominant author's firstauthor, then by series name.
// Books with no series go into Standalone, ordered by author then title.
func Group(books []Book) Grouped {
	buckets := map[string]*SeriesGroup{}
	var standalone []Book

	for _, b := range books {
		if !b.HasSeries() {
			standalone = append(standalone, b)
			continue
		}
		key := normSeriesKey(b.Series.Name)
		g := buckets[key]
		if g == nil {
			g = &SeriesGroup{Name: b.Series.Name, Author: b.Author}
			buckets[key] = g
		}
		g.Books = append(g.Books, b)
		if b.Series.Source > g.Source {
			g.Source = b.Series.Source
		}
	}

	out := Grouped{}
	for _, g := range buckets {
		sortBooksInSeries(g.Books)
		g.Author = dominantAuthor(g.Books)
		out.Series = append(out.Series, *g)
	}

	sort.Slice(out.Series, func(i, j int) bool {
		ai, aj := sortAuthor(out.Series[i]), sortAuthor(out.Series[j])
		if ai != aj {
			return ai < aj
		}
		return out.Series[i].Name < out.Series[j].Name
	})

	sort.Slice(standalone, func(i, j int) bool {
		if standalone[i].FirstAuthor != standalone[j].FirstAuthor {
			return standalone[i].FirstAuthor < standalone[j].FirstAuthor
		}
		return standalone[i].DisplayTitle() < standalone[j].DisplayTitle()
	})
	out.Standalone = standalone
	return out
}

func sortAuthor(g SeriesGroup) string {
	if len(g.Books) > 0 && g.Books[0].FirstAuthor != "" {
		return g.Books[0].FirstAuthor
	}
	return g.Author
}

// sortBooksInSeries orders books by their series number; numbered books first
// (ascending), unnumbered books after, ordered by title.
func sortBooksInSeries(bs []Book) {
	sort.Slice(bs, func(i, j int) bool {
		ni, nj := bs[i].Series.Number, bs[j].Series.Number
		switch {
		case ni > 0 && nj > 0:
			if ni != nj {
				return ni < nj
			}
			return bs[i].DisplayTitle() < bs[j].DisplayTitle()
		case ni > 0:
			return true // numbered before unnumbered
		case nj > 0:
			return false
		default:
			return bs[i].DisplayTitle() < bs[j].DisplayTitle()
		}
	})
}

// dominantAuthor returns the most common author among a series' books.
func dominantAuthor(bs []Book) string {
	counts := map[string]int{}
	best, bestN := "", 0
	for _, b := range bs {
		counts[b.Author]++
		if counts[b.Author] > bestN {
			best, bestN = b.Author, counts[b.Author]
		}
	}
	return best
}
