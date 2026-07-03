package main

import (
	"fmt"
	"image"

	ink "github.com/dennwc/inkview"
)

// ---- helpers ----

// usableH is the effective drawable height. ink.ScreenSize().Y reports 1448,
// but the real device wraps content below ~1360 to the top of the screen, so
// all vertical layout must be computed against this smaller value instead.
const usableH = 1340

func centerText(f *ink.Font, y int, s string, w int) {
	f.SetActive(ink.Black)
	tw := ink.StringWidth(s)
	ink.DrawString(image.Pt((w-tw)/2, y), s)
}

// ---- menu ----

func (a *app) drawMenu() {
	sz := ink.ScreenSize()
	w, h := sz.X, usableH

	centerText(a.fonts.title, h/6, "Lights Out", w)
	centerText(a.fonts.medium, h/6+90, "Slack alla lampor", w)

	// three size buttons stacked in the middle
	labels := []struct {
		n     int
		label string
	}{
		{3, "3 x 3  (latt)"},
		{5, "5 x 5  (klassisk)"},
		{7, "7 x 7  (svar)"},
	}
	bw := w * 3 / 5
	bh := 130
	gap := 50
	total := len(labels)*bh + (len(labels)-1)*gap
	startY := h/2 - total/2
	x0 := (w - bw) / 2

	a.menuBtns = a.menuBtns[:0]
	for i, l := range labels {
		y := startY + i*(bh+gap)
		r := image.Rect(x0, y, x0+bw, y+bh)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(ink.Pad(r, 1), ink.Black)
		ink.DrawRect(ink.Pad(r, 2), ink.Black)
		a.fonts.big.SetActive(ink.Black)
		tw := ink.StringWidth(l.label)
		ink.DrawString(image.Pt(x0+(bw-tw)/2, y+bh/2-30), l.label)
		a.menuBtns = append(a.menuBtns, menuButton{rect: r, size: l.n})
	}

	// "Regler" button opens the full rules screen.
	rbW := bw
	rbH := 110
	rbY := startY + len(labels)*(bh+gap) + 20
	rb := image.Rect(x0, rbY, x0+rbW, rbY+rbH)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(ink.Pad(rb, 1), ink.Black)
	a.fonts.big.SetActive(ink.Black)
	rt := "Regler"
	rtw := ink.StringWidth(rt)
	ink.DrawString(image.Pt(x0+(rbW-rtw)/2, rbY+rbH/2-30), rt)
	a.menuRules = rb
}

// ---- play ----

func (a *app) drawPlay() {
	sz := ink.ScreenSize()
	w, h := sz.X, usableH
	n := a.size

	// Top status line: move counter or win message.
	topH := 150
	if a.won {
		a.fonts.big.SetActive(ink.Black)
		msg := fmt.Sprintf("Lost pa %d tryck!", a.board.Moves)
		tw := ink.StringWidth(msg)
		ink.DrawString(image.Pt((w-tw)/2, 60), msg)
	} else {
		a.fonts.big.SetActive(ink.Black)
		msg := fmt.Sprintf("Tryck: %d   Lampor: %d", a.board.Moves, a.board.Count())
		tw := ink.StringWidth(msg)
		ink.DrawString(image.Pt((w-tw)/2, 60), msg)
	}

	// Bottom button bar.
	barH := 150
	margin := 40
	barY := h - barH - margin
	bw := (w - 4*margin) / 3
	a.btnNew = image.Rect(margin, barY, margin+bw, barY+barH)
	a.btnHint = image.Rect(2*margin+bw, barY, 2*margin+2*bw, barY+barH)
	a.btnMenu = image.Rect(3*margin+2*bw, barY, 3*margin+3*bw, barY+barH)

	for _, b := range []struct {
		r     image.Rectangle
		label string
	}{
		{a.btnNew, "Ny"},
		{a.btnHint, "Losning"},
		{a.btnMenu, "Meny"},
	} {
		ink.DrawRect(b.r, ink.Black)
		ink.DrawRect(ink.Pad(b.r, 1), ink.Black)
		a.fonts.big.SetActive(ink.Black)
		tw := ink.StringWidth(b.label)
		ink.DrawString(image.Pt(b.r.Min.X+(b.r.Dx()-tw)/2, b.r.Min.Y+barH/2-30), b.label)
	}

	// Grid area: between status line and button bar.
	availTop := topH
	availBottom := barY - 30
	availH := availBottom - availTop
	availW := w - 2*margin
	cell := availW / n
	if availH/n < cell {
		cell = availH / n
	}
	gridW := cell * n
	gridX := (w - gridW) / 2
	gridY := availTop + (availH-gridW)/2

	a.gridX, a.gridY, a.cell = gridX, gridY, cell

	// Draw cells: filled = ON, empty = OFF.
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			x := gridX + c*cell
			y := gridY + r*cell
			rect := image.Rect(x, y, x+cell, y+cell)
			ink.DrawRect(rect, ink.Black)
			if a.board.Lit(r, c) {
				ink.FillArea(ink.Pad(rect, 6), ink.Black)
			}
			// solution hint: mark cells to press with a ring/dot
			if a.solved && a.solution != nil && a.solution[r][c] {
				if a.board.Lit(r, c) {
					// on a filled cell, punch a white-ish marker via border box
					inner := ink.Pad(rect, cell/2-14)
					ink.DrawRect(inner, ink.White)
					ink.FillArea(ink.Pad(inner, 2), ink.White)
				} else {
					inner := ink.Pad(rect, cell/2-14)
					ink.FillArea(inner, ink.Black)
				}
			}
		}
	}
}

// ---- tap dispatch ----

func (a *app) handleTap(p image.Point) bool {
	switch a.scr {
	case screenSplash:
		a.scr = screenMenu
		ink.Repaint()
		return true
	case screenRules:
		if p.In(a.rulesBack) {
			a.scr = screenMenu
			ink.Repaint()
			return true
		}
		return false
	case screenMenu:
		if p.In(a.menuRules) {
			a.scr = screenRules
			ink.Repaint()
			return true
		}
		for _, b := range a.menuBtns {
			if p.In(b.rect) {
				a.startGame(b.size)
				ink.Repaint()
				return true
			}
		}
		return false
	case screenPlay:
		if p.In(a.btnMenu) {
			a.scr = screenMenu
			ink.Repaint()
			return true
		}
		if p.In(a.btnNew) {
			a.startGame(a.size)
			ink.Repaint()
			return true
		}
		if p.In(a.btnHint) {
			a.toggleHint()
			ink.Repaint()
			return true
		}
		// grid tap?
		if a.won {
			return false
		}
		if a.cell > 0 {
			relX := p.X - a.gridX
			relY := p.Y - a.gridY
			if relX >= 0 && relY >= 0 {
				c := relX / a.cell
				r := relY / a.cell
				if r >= 0 && r < a.size && c >= 0 && c < a.size {
					a.board.Press(r, c)
					a.solved = false // invalidate stale hint
					a.solution = nil
					if a.board.Solved() {
						a.won = true
					}
					ink.Repaint()
					return true
				}
			}
		}
		return false
	}
	return false
}

// ---- splash ----

// drawSplash renders the start screen: big title, a small Lights Out grid motif
// with a few cells lit, and a "tap to start" hint.
func (a *app) drawSplash() {
	sz := ink.ScreenSize()
	w, h := sz.X, usableH

	centerText(a.fonts.title, h/6, "Lights Out", w)

	// Centered square motif box.
	side := w * 3 / 5
	box := image.Rect((w-side)/2, (h-side)/2, (w+side)/2, (h+side)/2)
	drawSplashMotif(box)

	a.fonts.medium.SetActive(ink.DarkGray)
	hint := "Tryck for att borja"
	hw := ink.StringWidth(hint)
	ink.DrawString(image.Pt((w-hw)/2, h*5/6), hint)
}

// drawSplashMotif draws a small Lights Out grid: some cells lit (filled), some
// off (empty outline) — the game's own icon-like line art.
func drawSplashMotif(box image.Rectangle) {
	n := 4
	// A fixed pattern of lit cells for a pleasing "some on, some off" look.
	lit := [4][4]bool{
		{true, false, true, false},
		{false, true, false, true},
		{true, false, false, true},
		{false, true, true, false},
	}
	cell := box.Dx() / n
	gridW := cell * n
	x0 := box.Min.X + (box.Dx()-gridW)/2
	y0 := box.Min.Y + (box.Dy()-gridW)/2
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			x := x0 + c*cell
			y := y0 + r*cell
			rect := image.Rect(x, y, x+cell, y+cell)
			ink.DrawRect(rect, ink.Black)
			ink.DrawRect(ink.Pad(rect, 1), ink.Black)
			if lit[r][c] {
				ink.FillArea(ink.Pad(rect, 8), ink.Black)
			}
		}
	}
}

// ---- rules ----

var rulesParagraphs = []string{
	"Mal: slack ALLA lampor pa spelplanen.",
	"Nar du trycker pa en ruta vaxlar den mellan tand och slackt - och samma sak hander med dess fyra grannar: rutorna ovanfor, nedanfor, till vanster och till hoger.",
	"En tand ruta ar fylld (svart), en slackt ruta ar tom. Trycken vid kanterna paverkar farre grannar.",
	"Uppe visas drag-raknaren (antal tryck) och hur manga lampor som fortfarande lyser.",
	"\"Losning\" markerar vilka rutor du ska trycka pa for att slacka allt; tryck igen for att dolja.",
	"\"Ny\" blandar en ny bana i samma storlek. Alla banor gar alltid att losa.",
	"Storlekar i menyn: 3x3, 5x5 och 7x7.",
}

// wrapText greedily word-wraps s to fit within maxW pixels, measured with the
// currently active font via ink.StringWidth.
func wrapText(s string, maxW int) []string {
	var lines []string
	var cur string
	for _, word := range splitWords(s) {
		try := word
		if cur != "" {
			try = cur + " " + word
		}
		if ink.StringWidth(try) > maxW && cur != "" {
			lines = append(lines, cur)
			cur = word
		} else {
			cur = try
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func splitWords(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ' ' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// drawRules renders the full Swedish rules with a "Tillbaka" button and stores
// the button rect for handleTap.
func (a *app) drawRules() {
	sz := ink.ScreenSize()
	w, h := sz.X, usableH

	centerText(a.fonts.title, 90, "Regler", w)

	// "Tillbaka" button centered above the bottom margin (reserve its space first).
	bh := 110
	bw := w / 2
	r := image.Rect((w-bw)/2, h-bh-40, (w+bw)/2, h-40)

	a.fonts.medium.SetActive(ink.Black)
	margin := 60
	maxW := w - 2*margin
	y := 210
	lineH := 48
	paraGap := 22
	limit := r.Min.Y - 20 // never draw text into the button
	for _, p := range rulesParagraphs {
		for _, ln := range wrapText(p, maxW) {
			if y > limit {
				break
			}
			ink.DrawString(image.Pt(margin, y), ln)
			y += lineH
		}
		if y > limit {
			break
		}
		y += paraGap
	}

	ink.DrawRect(r, ink.Black)
	ink.DrawRect(ink.Pad(r, 1), ink.Black)
	a.fonts.big.SetActive(ink.Black)
	bt := "Tillbaka"
	btw := ink.StringWidth(bt)
	ink.DrawString(image.Pt(r.Min.X+(r.Dx()-btw)/2, r.Min.Y+bh/2-30), bt)
	a.rulesBack = r
}

// toggleHint computes (or hides) the solution overlay.
func (a *app) toggleHint() {
	if a.solved {
		a.solved = false
		a.solution = nil
		return
	}
	sol, ok := a.board.Solve()
	if ok {
		a.solution = sol
		a.solved = true
	}
}
