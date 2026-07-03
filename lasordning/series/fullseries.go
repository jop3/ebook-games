package series

import "sort"

// SeriesEntry is one book in a series' *complete* list (from online lookup),
// independent of whether it is on the device.
type SeriesEntry struct {
	Title  string  // canonical title from the source
	Number float64 // position in series (0 if unknown; then Year orders it)
	Year   int     // first publication year (fallback ordering / display)
}

// FullSeries is the complete, ordered book list for a series, as fetched online
// and cached. It is keyed by series name in the cache.
type FullSeries struct {
	Name      string        `json:"name"`
	Books     []SeriesEntry `json:"books"`
	FetchedTS int64         `json:"fetched_ts"` // unix seconds; 0 = unknown
}

// Ordered returns the entries sorted into reading order: by Number when known,
// otherwise by Year, otherwise by Title.
func (fs FullSeries) Ordered() []SeriesEntry {
	out := make([]SeriesEntry, len(fs.Books))
	copy(out, fs.Books)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch {
		case a.Number > 0 && b.Number > 0 && a.Number != b.Number:
			return a.Number < b.Number
		case a.Number > 0 && b.Number == 0:
			return true
		case a.Number == 0 && b.Number > 0:
			return false
		case a.Year != 0 && b.Year != 0 && a.Year != b.Year:
			return a.Year < b.Year
		default:
			return a.Title < b.Title
		}
	})
	// If nothing was numbered, assign display positions by the sorted order.
	return out
}

// MergedRow is a row in the detail view: a slot in the full series, marked with
// whether the user owns it on the device.
type MergedRow struct {
	Position int    // 1-based reading position
	Title    string // display title (owned book's title if owned, else canonical)
	Owned    bool
	BookID   int64 // device book id when Owned
	Year     int
}

// Merge combines the device's books for a series with the full online list.
// Owned books are matched to full-list slots by title similarity / number;
// any device books not in the full list are appended (owned, at the end);
// full-list books the user lacks appear as unowned rows ("[N] Title").
//
// If full is empty (never fetched / offline first run), Merge falls back to just
// the owned books in their existing order.
func Merge(owned []Book, full FullSeries) []MergedRow {
	ordered := full.Ordered()
	if len(ordered) == 0 {
		rows := make([]MergedRow, 0, len(owned))
		pos := 1
		for _, b := range owned {
			p := pos
			if b.Series.Number > 0 {
				p = int(b.Series.Number)
			}
			rows = append(rows, MergedRow{Position: p, Title: b.DisplayTitle(), Owned: true, BookID: b.ID, Year: 0})
			pos++
		}
		return rows
	}

	// Index owned books by normalized title for matching.
	ownedByTitle := map[string]Book{}
	used := map[int64]bool{}
	for _, b := range owned {
		ownedByTitle[normTitle(b.DisplayTitle())] = b
	}

	rows := make([]MergedRow, 0, len(ordered)+len(owned))
	for i, e := range ordered {
		row := MergedRow{Position: i + 1, Title: e.Title, Year: e.Year}
		if int(e.Number) > 0 {
			row.Position = int(e.Number)
		}
		if b, ok := ownedByTitle[normTitle(e.Title)]; ok {
			row.Owned = true
			row.BookID = b.ID
			row.Title = b.DisplayTitle() // prefer the device's own title text
			used[b.ID] = true
		}
		rows = append(rows, row)
	}
	// Owned books that didn't match any full-list entry: append as owned extras.
	next := len(ordered) + 1
	for _, b := range owned {
		if !used[b.ID] {
			rows = append(rows, MergedRow{Position: next, Title: b.DisplayTitle(), Owned: true, BookID: b.ID})
			next++
		}
	}
	return rows
}

// NextToDownload returns the first unowned title that comes after the user's
// highest owned position — i.e. "what to grab next after finishing what you
// have". Returns "" if the user owns everything known.
func NextToDownload(rows []MergedRow) string {
	highestOwned := 0
	for _, r := range rows {
		if r.Owned && r.Position > highestOwned {
			highestOwned = r.Position
		}
	}
	for _, r := range rows {
		if !r.Owned && r.Position > highestOwned {
			return r.Title
		}
	}
	// else: any unowned at all (e.g. missing earlier books)?
	for _, r := range rows {
		if !r.Owned {
			return r.Title
		}
	}
	return ""
}
