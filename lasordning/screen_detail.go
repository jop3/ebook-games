package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"lasordning/series"
)

// detailView shows one series as a single reading-order list that merges the
// books you own on the device with the full series list fetched online/cached.
// Owned books render normally; books you don't have render greyed with the
// position in brackets, e.g. [2] The Last Graduate — so at a glance you see
// what to read next and what to download.
type detailView struct {
	group  series.SeriesGroup
	full   series.FullSeries
	rows   []series.MergedRow
	status string // transient message

	top       int
	bookRects []image.Rectangle
	bookIdx   []int // index into rows for each tappable owned row
}

const detailRowH = 112

func (v *detailView) draw(screen image.Point, f *Fonts) []Button {
	ink.ClearScreen()

	// Title: series name + author + "next to download".
	f.Title.SetActive(ink.Black)
	ink.DrawString(image.Pt(24, topMargin+20), ellipsize2(f.Title, v.group.Name, screen.X-48))
	f.Small.SetActive(ink.DarkGray)
	sub := v.group.Author
	if next := series.NextToDownload(v.rows); next != "" {
		sub = "Ladda ner härnäst: " + next
	}
	ink.DrawString(image.Pt(24, topMargin+76), ellipsize2(f.Small, sub, screen.X-48))
	ink.DrawLine(image.Pt(0, titleBarH), image.Pt(screen.X, titleBarH), ink.Black)

	if v.status != "" {
		f.Body.SetActive(ink.Black)
		drawWrappedCentered(image.Rect(40, titleBarH+20, screen.X-40, screen.Y-buttonBarH), v.status, f)
		return drawButtonBar(screen, f,
			[]string{"Tillbaka", "Hamta hela serien"}, []string{"back", "online"})
	}

	areaTop := titleBarH + 10
	areaBottom := screen.Y - buttonBarH
	visible := (areaBottom - areaTop) / detailRowH
	if v.top < 0 {
		v.top = 0
	}
	if v.top > len(v.rows)-1 {
		v.top = maxInt(0, len(v.rows)-1)
	}

	v.bookRects = v.bookRects[:0]
	v.bookIdx = v.bookIdx[:0]
	y := areaTop
	end := minInt(len(v.rows), v.top+visible)
	for i := v.top; i < end; i++ {
		r := image.Rect(0, y, screen.X, y+detailRowH)
		v.drawRow(r, v.rows[i], f, screen.X)
		if v.rows[i].Owned {
			v.bookRects = append(v.bookRects, r)
			v.bookIdx = append(v.bookIdx, i)
		}
		y += detailRowH
	}

	// Legend when there are missing books.
	if hasMissing(v.rows) {
		f.Small.SetActive(ink.DarkGray)
		ink.DrawString(image.Pt(24, areaBottom-38), "[ ] = finns ej på enheten")
	}

	return drawButtonBar(screen, f,
		[]string{"Tillbaka", "▲", "▼", "Hamta serie"},
		[]string{"back", "up", "down", "online"})
}

func (v *detailView) drawRow(r image.Rectangle, row series.MergedRow, f *Fonts, screenW int) {
	badge := image.Rect(r.Min.X+20, r.Min.Y+16, r.Min.X+110, r.Min.Y+16+80)
	if row.Owned {
		// Solid black badge with the number.
		ink.FillArea(badge, ink.Black)
		f.Head.SetActive(ink.White)
		drawCentered(badge, itoa(row.Position), 40)
		f.Body.SetActive(ink.Black)
		ink.DrawString(image.Pt(badge.Max.X+24, r.Min.Y+18),
			ellipsize2(f.Body, row.Title, screenW-badge.Max.X-48))
		f.Small.SetActive(ink.DarkGray)
		yr := ""
		if row.Year > 0 {
			yr = itoa(row.Year)
		}
		ink.DrawString(image.Pt(badge.Max.X+24, r.Min.Y+66), "på enheten  "+yr)
	} else {
		// Greyed "[N]" badge for a book you don't own.
		ink.DrawRect(badge, ink.LightGray)
		f.Head.SetActive(ink.DarkGray)
		drawCentered(badge, "["+itoa(row.Position)+"]", 40)
		f.Body.SetActive(ink.DarkGray)
		ink.DrawString(image.Pt(badge.Max.X+24, r.Min.Y+18),
			ellipsize3(f.Body, row.Title, screenW-badge.Max.X-48))
		f.Small.SetActive(ink.LightGray)
		yr := "ladda ner"
		if row.Year > 0 {
			yr = "ladda ner  " + itoa(row.Year)
		}
		ink.DrawString(image.Pt(badge.Max.X+24, r.Min.Y+66), yr)
	}
	ink.DrawLine(image.Pt(r.Min.X, r.Max.Y), image.Pt(r.Max.X, r.Max.Y), ink.LightGray)
}

func hasMissing(rows []series.MergedRow) bool {
	for _, r := range rows {
		if !r.Owned {
			return true
		}
	}
	return false
}

func (v *detailView) scroll(dir int) { v.scrollRows(dir * 4) }

// scrollRows moves the viewport by n rows (n<0 = up), clamped.
func (v *detailView) scrollRows(n int) {
	v.top += n
	if v.top < 0 {
		v.top = 0
	}
	if v.top > len(v.rows)-1 {
		v.top = maxInt(0, len(v.rows)-1)
	}
}

// bookAt returns the group.Books index of a tapped owned row (for editing).
// It maps the merged row's BookID back to the group's slice index.
func (v *detailView) bookAt(p image.Point) (int, bool) {
	for k, r := range v.bookRects {
		if p.In(r) {
			id := v.rows[v.bookIdx[k]].BookID
			for gi, b := range v.group.Books {
				if b.ID == id {
					return gi, true
				}
			}
		}
	}
	return 0, false
}

// ellipsize2 sets black; ellipsize3 keeps the caller's (grey) colour.
func ellipsize2(fn *ink.Font, s string, maxW int) string {
	fn.SetActive(ink.Black)
	return ellipsize(s, maxW)
}
func ellipsize3(fn *ink.Font, s string, maxW int) string {
	fn.SetActive(ink.DarkGray)
	return ellipsize(s, maxW)
}
