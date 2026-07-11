package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"lasordning/series"
)

// listView is the overview screen: every series (grouped author -> series) with
// a book count, followed by a "Fristående" section for books with no series.
// It is a flat, scrollable list of rows; each series row is tappable.
type listView struct {
	top      int       // index of the first visible row
	rows     []listRow // last-rendered rows (for hit testing)
	rowRects []image.Rectangle
	status   string // transient status line (sync progress)
}

type listRowKind int

const (
	rowSeries listRowKind = iota
	rowSectionHeader
	rowStandalone
)

type listRow struct {
	kind      listRowKind
	seriesIdx int // into grouped.Series when kind==rowSeries
	text      string
	sub       string // secondary line (e.g. "3 böcker · titel")
}

const (
	// titleBarH is the height of the top title area (divider sits at its
	// bottom). It includes topMargin so the title text isn't at the screen edge.
	titleBarH = 110 + topMargin
	listRowH  = 108
)

// buildRows flattens the grouped data into display rows.
func (v *listView) buildRows(g series.Grouped) []listRow {
	var rows []listRow
	if len(g.Series) > 0 {
		rows = append(rows, listRow{kind: rowSectionHeader, text: "Serier"})
		for i, s := range g.Series {
			sub := itoa(s.Count()) + " böcker"
			if gaps := s.MissingBefore(); len(gaps) > 0 {
				sub += " · saknar #" + itoa(gaps[0])
			}
			sub += " · " + s.Source.String()
			rows = append(rows, listRow{
				kind:      rowSeries,
				seriesIdx: i,
				text:      s.Name,
				sub:       s.Author + " · " + sub,
			})
		}
	}
	if len(g.Standalone) > 0 {
		rows = append(rows, listRow{kind: rowSectionHeader,
			text: "Fristående (" + itoa(len(g.Standalone)) + ")"})
		for _, b := range g.Standalone {
			rows = append(rows, listRow{
				kind: rowStandalone,
				text: b.DisplayTitle(),
				sub:  b.Author,
			})
		}
	}
	if len(rows) == 0 {
		rows = append(rows, listRow{kind: rowSectionHeader, text: "Inga böcker hittades"})
	}
	return rows
}

func (v *listView) draw(screen image.Point, f *Fonts, g series.Grouped, loadErr error) []Button {
	ink.ClearScreen()

	// Title bar.
	f.Title.SetActive(ink.Black)
	ink.DrawString(image.Pt(24, topMargin+34), "Läsordning")
	ink.DrawLine(image.Pt(0, titleBarH), image.Pt(screen.X, titleBarH), ink.Black)

	if loadErr != nil {
		f.Body.SetActive(ink.Black)
		ink.DrawString(image.Pt(24, titleBarH+40), "Kunde inte läsa biblioteket:")
		f.Small.SetActive(ink.DarkGray)
		ink.DrawString(image.Pt(24, titleBarH+90), ellipsize(loadErr.Error(), screen.X-48))
		return drawButtonBar(screen, f, []string{"Uppdatera"}, []string{"reload"})
	}

	v.rows = v.buildRows(g)
	areaTop := titleBarH + 1
	areaBottom := screen.Y - buttonBarH
	visible := (areaBottom - areaTop) / listRowH

	if v.top > len(v.rows)-1 {
		v.top = maxInt(0, len(v.rows)-1)
	}
	if v.top < 0 {
		v.top = 0
	}

	v.rowRects = v.rowRects[:0]
	y := areaTop
	end := minInt(len(v.rows), v.top+visible)
	for i := v.top; i < end; i++ {
		r := image.Rect(0, y, screen.X, y+listRowH)
		v.drawRow(r, v.rows[i], f, screen.X)
		v.rowRects = append(v.rowRects, r)
		y += listRowH
	}
	// pad rowRects alignment: rowRects[k] corresponds to rows[v.top+k]

	// Scroll indicator / status.
	f.Small.SetActive(ink.DarkGray)
	pos := "rad " + itoa(v.top+1) + "–" + itoa(end) + " av " + itoa(len(v.rows))
	if v.status != "" {
		pos = v.status
	}
	ink.DrawString(image.Pt(24, areaBottom-40), pos)

	labels := []string{"▲ Upp", "▼ Ner", "Hämta alla"}
	ids := []string{"up", "down", "syncall"}
	return drawButtonBar(screen, f, labels, ids)
}

func (v *listView) drawRow(r image.Rectangle, row listRow, f *Fonts, screenW int) {
	switch row.kind {
	case rowSectionHeader:
		ink.FillArea(r, ink.LightGray)
		f.Head.SetActive(ink.Black)
		drawLeft(r, row.text, 40)
	case rowSeries:
		// Set the font BEFORE ellipsizing: ellipsize measures with the active
		// face, and the same face must then draw the string.
		f.Head.SetActive(ink.Black)
		title := ellipsize("> "+row.text, screenW-48)
		ink.DrawString(image.Pt(r.Min.X+24, r.Min.Y+14), title)
		f.Small.SetActive(ink.DarkGray)
		sub := ellipsize(row.sub, screenW-72)
		ink.DrawString(image.Pt(r.Min.X+48, r.Min.Y+62), sub)
		ink.DrawLine(image.Pt(r.Min.X, r.Max.Y), image.Pt(r.Max.X, r.Max.Y), ink.LightGray)
	case rowStandalone:
		f.Body.SetActive(ink.Black)
		title := ellipsize(row.text, screenW-48)
		ink.DrawString(image.Pt(r.Min.X+24, r.Min.Y+14), title)
		f.Small.SetActive(ink.DarkGray)
		sub := ellipsize(row.sub, screenW-48)
		ink.DrawString(image.Pt(r.Min.X+24, r.Min.Y+62), sub)
		ink.DrawLine(image.Pt(r.Min.X, r.Max.Y), image.Pt(r.Max.X, r.Max.Y), ink.LightGray)
	}
}

func (v *listView) scroll(dir int) { v.scrollRows(dir * 5) }

// scrollRows moves the viewport by n rows (n<0 = up), clamped.
func (v *listView) scrollRows(n int) {
	v.top += n
	if v.top < 0 {
		v.top = 0
	}
	if v.top > len(v.rows)-1 {
		v.top = maxInt(0, len(v.rows)-1)
	}
}

// seriesAt returns the grouped.Series index for a tapped series row.
func (v *listView) seriesAt(p image.Point) (int, bool) {
	for k, r := range v.rowRects {
		if p.In(r) {
			row := v.rows[v.top+k]
			if row.kind == rowSeries {
				return row.seriesIdx, true
			}
		}
	}
	return 0, false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
