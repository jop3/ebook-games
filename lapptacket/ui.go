package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"lapptacket/game"
)

// --- Fonts (opened once in Init, never per-frame) ---------------------------

type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Small  *ink.Font
	Tiny   *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 32, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 34, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 36, true),
		Small:  ink.OpenFont(ink.DefaultFont, 26, true),
		Tiny:   ink.OpenFont(ink.DefaultFont, 22, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Small, f.Tiny} {
		if fn != nil {
			fn.Close()
		}
	}
}

// --- small generic helpers (mirrors this repo's other games) ---------------

func pad(r image.Rectangle, n int) image.Rectangle {
	return image.Rect(r.Min.X+n, r.Min.Y+n, r.Max.X-n, r.Max.Y-n)
}

func drawCenteredString(r image.Rectangle, s string, approxHeight int) {
	w := ink.StringWidth(s)
	x := r.Min.X + (r.Dx()-w)/2
	y := r.Min.Y + (r.Dy()-approxHeight)/2
	ink.DrawString(image.Pt(x, y), s)
}

func drawLeftString(r image.Rectangle, s string, approxHeight int) {
	x := r.Min.X + 16
	y := r.Min.Y + (r.Dy()-approxHeight)/2
	ink.DrawString(image.Pt(x, y), s)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// isqrt is an integer square root (floor), used by fillDisc.
func isqrt(n int) int {
	if n <= 0 {
		return 0
	}
	x := n
	y := (x + 1) / 2
	for y < x {
		x = y
		y = (x + n/x) / 2
	}
	return x
}

func fillDisc(r image.Rectangle, col color.Color) {
	cx := (r.Min.X + r.Max.X) / 2
	cy := (r.Min.Y + r.Max.Y) / 2
	rad := r.Dx() / 2
	if r.Dy()/2 < rad {
		rad = r.Dy() / 2
	}
	rr := rad * rad
	for dy := -rad; dy <= rad; dy++ {
		hw := isqrt(rr - dy*dy)
		y := cy + dy
		ink.DrawLine(image.Pt(cx-hw, y), image.Pt(cx+hw, y), col)
	}
}

// --- Layout ------------------------------------------------------------

const (
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	topMargin     = 36
	statusBarH    = 84
	timeTrackH    = 110
	patchStripH   = 170
	sectionGap    = 14
	buttonBarH    = 112
	opponentH     = 124
	boardSideMarg = 20
)

// Layout maps screen pixels to the game's various zones: the status text,
// the linear 0-53 time-track strip, the horizontal patch-queue strip, the
// active player's full-size 9x9 board, the opponent's compact summary, and
// the button bar.
type Layout struct {
	Screen       image.Rectangle
	StatusBar    image.Rectangle
	TimeTrack    image.Rectangle
	PatchStrip   image.Rectangle
	BoardArea    image.Rectangle
	GridOrigin   image.Point
	CellSize     int
	OpponentArea image.Rectangle
	ButtonBar    image.Rectangle

	PatchRects []image.Rectangle // tap targets for the visible queue tiles (first 3 buyable)
	ShowOppBtn image.Rectangle
}

func NewLayout(screen image.Point) Layout {
	W, H := screen.X, usableH
	l := Layout{Screen: image.Rect(0, 0, W, H)}

	// Bottom-anchored first (per the guide: build bottom-up to guarantee a
	// margin to the real ~1360 edge).
	l.ButtonBar = image.Rect(0, H-topMargin-buttonBarH, W, H-topMargin)
	l.OpponentArea = image.Rect(0, l.ButtonBar.Min.Y-sectionGap-opponentH, W, l.ButtonBar.Min.Y-sectionGap)

	y := topMargin
	l.StatusBar = image.Rect(0, y, W, y+statusBarH)
	y = l.StatusBar.Max.Y + sectionGap
	l.TimeTrack = image.Rect(0, y, W, y+timeTrackH)
	y = l.TimeTrack.Max.Y + sectionGap
	l.PatchStrip = image.Rect(0, y, W, y+patchStripH)
	y = l.PatchStrip.Max.Y + sectionGap

	avail := image.Rect(boardSideMarg, y, W-boardSideMarg, l.OpponentArea.Min.Y-sectionGap)
	side := avail.Dx()
	if avail.Dy() < side {
		side = avail.Dy()
	}
	cell := side / game.BoardSize
	if cell < 1 {
		cell = 1
	}
	l.CellSize = cell
	boardPx := cell * game.BoardSize
	l.GridOrigin = image.Pt(avail.Min.X+(avail.Dx()-boardPx)/2, avail.Min.Y+(avail.Dy()-boardPx)/2)
	l.BoardArea = image.Rect(l.GridOrigin.X, l.GridOrigin.Y, l.GridOrigin.X+boardPx, l.GridOrigin.Y+boardPx)

	return l
}

func (l *Layout) CellToScreen(x, y int) image.Rectangle {
	return image.Rect(
		l.GridOrigin.X+x*l.CellSize, l.GridOrigin.Y+y*l.CellSize,
		l.GridOrigin.X+(x+1)*l.CellSize, l.GridOrigin.Y+(y+1)*l.CellSize,
	)
}

func (l *Layout) ScreenToCell(p image.Point) (x, y int, ok bool) {
	if l.CellSize == 0 || !p.In(l.BoardArea) {
		return 0, 0, false
	}
	rel := p.Sub(l.GridOrigin)
	x, y = rel.X/l.CellSize, rel.Y/l.CellSize
	if x < 0 || x >= game.BoardSize || y < 0 || y >= game.BoardSize {
		return 0, 0, false
	}
	return x, y, true
}

// --- Buttons -------------------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

func DrawButtonBar(l *Layout, labels []string, f *Fonts) []Button {
	ink.FillArea(l.ButtonBar, ink.White)
	ink.DrawLine(image.Pt(l.ButtonBar.Min.X, l.ButtonBar.Min.Y), image.Pt(l.ButtonBar.Max.X, l.ButtonBar.Min.Y), ink.Black)
	f.Button.SetActive(ink.Black)
	n := len(labels)
	if n == 0 {
		return nil
	}
	gap, sideMargin := 16, 20
	usableW := l.ButtonBar.Dx() - 2*sideMargin
	bw := (usableW - gap*(n+1)) / n
	bh := l.ButtonBar.Dy() - 2*gap
	out := make([]Button, n)
	for i, label := range labels {
		x0 := l.ButtonBar.Min.X + sideMargin + gap + i*(bw+gap)
		y0 := l.ButtonBar.Min.Y + gap
		r := image.Rect(x0, y0, x0+bw, y0+bh)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawCenteredString(r, label, 32)
		out[i] = Button{Rect: r, Label: label}
	}
	return out
}

// --- Status bar --------------------------------------------------------

func DrawStatus(l *Layout, gs *game.GameState, f *Fonts, msg string) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	drawCenteredString(image.Rect(l.StatusBar.Min.X, l.StatusBar.Min.Y, l.StatusBar.Max.X, l.StatusBar.Min.Y+42), statusHeadline(gs), 30)
	f.Small.SetActive(ink.Black)
	drawCenteredString(image.Rect(l.StatusBar.Min.X, l.StatusBar.Min.Y+42, l.StatusBar.Max.X, l.StatusBar.Max.Y), msg, 26)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y), image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

func playerName(gs *game.GameState, side int) string {
	if side == 0 {
		return "Spelare 1"
	}
	if gs.Opponent == game.OpponentAI {
		return "Spelare 2 (dator)"
	}
	return "Spelare 2"
}

func statusHeadline(gs *game.GameState) string {
	if gs.Phase == game.PhaseDone {
		w := gs.Winner()
		if w == -1 {
			return "Oavgjort!  " + itoa(gs.FinalScore(0)) + " - " + itoa(gs.FinalScore(1))
		}
		return playerName(gs, w) + " vinner!  " + itoa(gs.FinalScore(0)) + " - " + itoa(gs.FinalScore(1))
	}
	active := gs.ActingPlayer()
	name := playerName(gs, active)
	if gs.Pending[active] > 0 {
		return name + " — placera bonuslapp"
	}
	return name + "s tur — Knappar " + itoa(gs.Buttons[active]) + " · Inkomst " + itoa(gs.Income[active])
}

// --- Time track (linear, 0..TrackEnd) ------------------------------------

func DrawTimeTrack(l *Layout, gs *game.GameState, f *Fonts) {
	area := l.TimeTrack
	ink.FillArea(area, ink.White)
	margin := 46 // wide enough that the "0"/"53" end labels never sit under a marker
	lineY := (area.Min.Y + area.Max.Y) / 2
	x0, x1 := area.Min.X+margin, area.Max.X-margin
	ink.DrawLine(image.Pt(x0, lineY), image.Pt(x1, lineY), ink.Black)

	posX := func(pos int) int {
		return x0 + (x1-x0)*pos/game.TrackEnd
	}

	// Income squares: small filled tick below the line.
	for _, p := range game.IncomeSquares {
		x := posX(p)
		ink.DrawLine(image.Pt(x, lineY+2), image.Pt(x, lineY+11), ink.DarkGray)
	}
	// Special free-patch squares: small square above the line, filled once claimed.
	for i, p := range game.SpecialPatchPositions {
		x := posX(p)
		r := image.Rect(x-5, lineY-13, x+5, lineY-3)
		ink.DrawRect(r, ink.Black)
		if gs.ClaimedSpecial[i] {
			ink.FillArea(pad(r, 1), ink.DarkGray)
		}
	}

	// End labels sit ON the line, in the side margins — clear of both the
	// income ticks/special squares (just above/below the line) and the
	// player markers (further out still), so nothing overlaps even when
	// both markers are at position 0.
	f.Tiny.SetActive(ink.Black)
	ink.DrawString(image.Pt(x0-14-ink.StringWidth("0"), lineY-11), "0")
	ink.DrawString(image.Pt(x1+14, lineY-11), "53")

	// Two player markers: player 0 = solid disc (above the line, clear of
	// the special-patch squares), player 1 = ring (below the line, clear
	// of the income ticks) — same visual convention as this repo's other
	// 2-player games. If tied, nudge them apart horizontally so both
	// remain visible.
	rad := 11
	x0m, x1m := posX(gs.Marker[0]), posX(gs.Marker[1])
	if abs(x0m-x1m) < rad {
		if x0m <= x1m {
			x0m -= rad
			x1m += rad
		} else {
			x0m += rad
			x1m -= rad
		}
	}
	c0 := image.Pt(x0m, lineY-13-rad-10)
	c1 := image.Pt(x1m, lineY+11+rad+10)
	fillDisc(image.Rect(c0.X-rad, c0.Y-rad, c0.X+rad, c0.Y+rad), ink.Black)
	fillDisc(image.Rect(c1.X-rad, c1.Y-rad, c1.X+rad, c1.Y+rad), ink.Black)
	fillDisc(pad(image.Rect(c1.X-rad, c1.Y-rad, c1.X+rad, c1.Y+rad), rad/3+2), ink.White)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// --- Patch queue strip (rendered as a straight horizontal strip, per the
// spec's explicit guidance — never as an actual circle) -------------------

const patchTilesShown = 5
const patchTilesBuyable = 3

// DrawPatchStrip renders up to patchTilesShown upcoming patches starting at
// the neutral token; the first 3 are buyable (bold border, tappable) and
// any further ones are a lighter preview only. Records l.PatchRects.
func DrawPatchStrip(l *Layout, gs *game.GameState, selOffset, orientIdx int, f *Fonts) {
	area := l.PatchStrip
	ink.FillArea(area, ink.White)
	ink.DrawLine(image.Pt(area.Min.X, area.Min.Y), image.Pt(area.Max.X, area.Min.Y), ink.Black)

	n := len(gs.Queue)
	shown := patchTilesShown
	if n < shown {
		shown = n
	}
	l.PatchRects = l.PatchRects[:0]
	if shown == 0 {
		f.Small.SetActive(ink.Black)
		drawCenteredString(area, "Inga fler lappar", 26)
		return
	}

	sideMargin, gap := 24, 10
	usableW := area.Dx() - 2*sideMargin
	tileW := (usableW - gap*(shown-1)) / shown
	tileH := area.Dy() - 20
	y0 := area.Min.Y + 10

	for i := 0; i < shown; i++ {
		patch := gs.Queue[(gs.TokenIdx+i)%n]
		x0 := area.Min.X + sideMargin + i*(tileW+gap)
		r := image.Rect(x0, y0, x0+tileW, y0+tileH)
		l.PatchRects = append(l.PatchRects, r)

		buyable := i < patchTilesBuyable
		var border color.Color = ink.Black
		if !buyable {
			border = ink.DarkGray
		}
		ink.DrawRect(r, border)
		if buyable {
			ink.DrawRect(pad(r, 1), border)
		}
		if buyable && selOffset == i {
			ink.DrawRect(pad(r, 3), ink.Black)
			ink.DrawRect(pad(r, 4), ink.Black)
		}

		iconBox := image.Rect(r.Min.X+6, r.Min.Y+6, r.Max.X-6, r.Min.Y+tileH*3/5)
		cells := patch.Cells
		if buyable && selOffset == i {
			orients := game.Orientations(patch.Cells)
			if orientIdx >= 0 && orientIdx < len(orients) {
				cells = orients[orientIdx]
			}
		}
		drawShapeIcon(iconBox, cells)

		f.Tiny.SetActive(ink.Black)
		stats := "K" + itoa(patch.Cost) + " T" + itoa(patch.TimeCost) + " +" + itoa(patch.Income)
		drawCenteredString(image.Rect(r.Min.X, r.Min.Y+tileH*3/5, r.Max.X, r.Max.Y), stats, 22)
	}
}

// drawShapeIcon draws a small line-art rendering of a patch shape (a list
// of unit-cell offsets) centered and scaled to fit inside box.
func drawShapeIcon(box image.Rectangle, cells []game.Offset) {
	maxX, maxY := game.BoundingBox(cells)
	cols, rows := maxX+1, maxY+1
	cell := box.Dx() / cols
	if h := box.Dy() / rows; h < cell {
		cell = h
	}
	if cell > 26 {
		cell = 26 // keep icons tidy even for a lone 1x1 patch
	}
	if cell < 4 {
		cell = 4
	}
	w, h := cell*cols, cell*rows
	ox := box.Min.X + (box.Dx()-w)/2
	oy := box.Min.Y + (box.Dy()-h)/2
	for _, c := range cells {
		r := image.Rect(ox+c[0]*cell, oy+c[1]*cell, ox+(c[0]+1)*cell, oy+(c[1]+1)*cell)
		ink.FillArea(pad(r, 1), ink.DarkGray)
		ink.DrawRect(r, ink.Black)
	}
}

// --- Active board (full-size) & opponent (compact) -----------------------

// DrawActiveBoard renders side's 9x9 quilt board full-size, filled cells
// shaded (bonus/free patches marked with a small dot), and — if legal is
// non-nil — a light highlight over every cell in legal (valid tap targets
// for the current selection or a pending free-patch placement).
func DrawActiveBoard(l *Layout, gs *game.GameState, side int, legal map[image.Point]bool) {
	b := &gs.Boards[side]
	ink.DrawRect(l.BoardArea, ink.Black)
	ink.DrawRect(pad(l.BoardArea, 1), ink.Black)
	for i := 1; i < game.BoardSize; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.BoardArea.Min.Y), image.Pt(px, l.BoardArea.Max.Y), ink.LightGray)
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, py), image.Pt(l.BoardArea.Max.X, py), ink.LightGray)
	}
	for y := 0; y < game.BoardSize; y++ {
		for x := 0; x < game.BoardSize; x++ {
			cellRect := l.CellToScreen(x, y)
			if b.Filled[y][x] {
				ink.FillArea(pad(cellRect, 2), ink.DarkGray)
				if b.Bonus[y][x] {
					cx, cy := (cellRect.Min.X+cellRect.Max.X)/2, (cellRect.Min.Y+cellRect.Max.Y)/2
					r := l.CellSize / 6
					if r < 3 {
						r = 3
					}
					fillDisc(image.Rect(cx-r, cy-r, cx+r, cy+r), ink.White)
				}
			} else if legal != nil && legal[image.Pt(x, y)] {
				cx, cy := (cellRect.Min.X+cellRect.Max.X)/2, (cellRect.Min.Y+cellRect.Max.Y)/2
				r := l.CellSize / 5
				ink.FillArea(image.Rect(cx-r, cy-r, cx+r, cy+r), ink.LightGray)
				ink.DrawRect(image.Rect(cx-r, cy-r, cx+r, cy+r), ink.DarkGray)
			}
		}
	}
}

// DrawOpponentStrip renders the OTHER board as a compact mini-grid plus a
// one-line summary, with the "Visa motståndare"/"Mitt bräde" toggle button
// — the same 2-player density mitigation this repo's mosaik (Azul) port
// uses. Records the toggle rect into l.ShowOppBtn.
func DrawOpponentStrip(l *Layout, gs *game.GameState, side int, showOpp bool, f *Fonts) {
	area := l.OpponentArea
	ink.FillArea(area, ink.White)
	ink.DrawLine(image.Pt(area.Min.X, area.Min.Y), image.Pt(area.Max.X, area.Min.Y), ink.Black)
	b := &gs.Boards[side]

	miniCell := 11
	miniSize := miniCell * game.BoardSize
	miniOrigin := image.Pt(area.Min.X+24, area.Min.Y+(area.Dy()-miniSize)/2)
	for y := 0; y < game.BoardSize; y++ {
		for x := 0; x < game.BoardSize; x++ {
			r := image.Rect(miniOrigin.X+x*miniCell, miniOrigin.Y+y*miniCell,
				miniOrigin.X+(x+1)*miniCell, miniOrigin.Y+(y+1)*miniCell)
			if b.Filled[y][x] {
				ink.FillArea(pad(r, 1), ink.Black)
			} else {
				ink.DrawRect(r, ink.LightGray)
			}
		}
	}

	btnW, btnH := 260, 64
	btn := image.Rect(area.Max.X-btnW-24, area.Min.Y+(area.Dy()-btnH)/2, area.Max.X-24, area.Min.Y+(area.Dy()-btnH)/2+btnH)
	ink.DrawRect(btn, ink.Black)
	ink.DrawRect(pad(btn, 1), ink.Black)
	label := "Visa motståndare"
	if showOpp {
		label = "Mitt bräde"
	}
	toggleFont := ink.OpenFont(ink.DefaultFontBold, 24, true)
	toggleFont.SetActive(ink.Black)
	drawCenteredString(btn, label, 24)
	toggleFont.Close()
	l.ShowOppBtn = btn

	textArea := image.Rect(miniOrigin.X+miniSize+20, area.Min.Y, btn.Min.X-16, area.Max.Y)
	f.Small.SetActive(ink.Black)
	line1 := playerName(gs, side) + "   Knappar " + itoa(gs.Buttons[side])
	drawLeftString(image.Rect(textArea.Min.X, area.Min.Y+18, textArea.Max.X, area.Min.Y+48), line1, 26)
	f.Tiny.SetActive(ink.Black)
	line2 := "Inkomst " + itoa(gs.Income[side]) + "   Position " + itoa(gs.Marker[side]) + "/" + itoa(game.TrackEnd) +
		"   Tomma " + itoa(b.EmptyCount())
	drawLeftString(image.Rect(textArea.Min.X, area.Min.Y+52, textArea.Max.X, area.Min.Y+80), line2, 22)
}

// --- Menu ----------------------------------------------------------------

type menuChoice struct {
	opponent game.Opponent
	label    string
}

type menuRow struct {
	rect   image.Rectangle
	choice menuChoice
}

var opponentChoices = []menuChoice{
	{game.OpponentHotseat, "2 spelare (hot-seat)"},
	{game.OpponentAI, "Mot dator"},
}

type Menu struct {
	rows     []menuRow
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{} }

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	H := usableH
	W := screen.X

	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Lapptacket")
	ink.DrawString(image.Pt((W-tw)/2, 56), "Lapptacket")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 30, true)
	sub.SetActive(ink.Black)
	subT := "Välj läge"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((W-sw)/2, 140), subT)
	sub.Close()

	margin := 60
	rowW := W - 2*margin

	rbW := rowW / 2
	rbH := 100
	rb := image.Rect((W-rbW)/2, H-margin-rbH, (W+rbW)/2, H-margin)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	f.Button.SetActive(ink.Black)
	drawCenteredString(rb, "Regler", 34)
	m.rulesBtn = rb

	rowH := 130
	top := 210
	bottom := rb.Min.Y - 30
	n := len(opponentChoices)
	avail := bottom - top
	if avail < rowH*n {
		rowH = avail / n
	}
	y := top
	m.rows = m.rows[:0]
	f.Menu.SetActive(ink.Black)
	for _, c := range opponentChoices {
		r := image.Rect(margin, y, margin+rowW, y+rowH-20)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawLeftString(r, c.label, 34)
		m.rows = append(m.rows, menuRow{rect: r, choice: c})
		y += rowH
	}
}

func (m *Menu) HandleTouch(p image.Point) (menuChoice, bool) {
	for _, r := range m.rows {
		if p.In(r.rect) {
			return r.choice, true
		}
	}
	return menuChoice{}, false
}

func (m *Menu) RulesButton() image.Rectangle { return m.rulesBtn }

// --- Rules + splash screens (standard shape across this repo's games) ------

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

// rulesParagraphs is the full Swedish rules text for Lapptäcket.
var rulesParagraphs = []string{
	"Mål: fyll ditt 9x9-täcke så effektivt som möjligt med lappar i olika former, köpta för knappar (pengar) och tid.",
	"Turordning — VIKTIGAST AV ALLT: det är INTE turas om i vanlig mening. Den spelare vars markör på tidslinjen ligger LÄNGST BAK får alltid göra nästa drag. Står markörerna lika går spelare 1 först. Det betyder att samma spelare ibland får dra flera gånger i rad.",
	"En tur: välj en av de tre närmaste lapparna i kön (längst till vänster i remsan), betala dess knapp-kostnad, lägg den var som helst på ditt eget täcke (valfri rotation/spegling) på tomma rutor — eller välj att avancera: flytta din egen markör till strax förbi motståndarens markör på tidslinjen och få 1 knapp per ruta du flyttade.",
	"När du köper en lapp flyttas din markör också framåt på tidslinjen, lika många steg som lappens tidskostnad (T).",
	"Inkomst: när din markör PASSERAR ELLER LANDAR PÅ en inkomstruta (markerad med en liten tagg under linjen) får du knappar motsvarande summan av inkomstvärdet (+N) på alla lappar du har lagt — även om du hoppar över flera inkomstrutor i samma drag räknas de alla.",
	"Bonuslappar: några fasta platser på tidslinjen (markerade med små rutor ovanför linjen) ger en gratis 1x1-lapp värd +1 i permanent inkomst till den första spelare vars markör når eller passerar platsen. Den lappen måste placeras direkt någonstans på ditt täcke innan turordningen går vidare.",
	"7x7-bonus: den första spelaren som fyller en hel 7x7-yta någonstans på sitt täcke får omedelbart +7 poäng. Detta kan bara hända EN gång totalt i hela spelet — inte en gång per spelare.",
	"Spelet slutar när BÅDA spelarnas markörer har nått slutet av tidslinjen (ruta 53).",
	"Slutpoäng = knappar i handen minus 2 poäng per tom ruta som återstår på ditt täcke, plus 7 om du tog 7x7-bonusen. Flest poäng vinner.",
	"Tryck på en av de tre köpbara lapparna för att välja den, tryck Rotera för att vända/spegla den, tryck sedan på en tom ruta på ditt eget täcke för att lägga den där. Ljusa markeringar visar var den får plats.",
	"Baserad på Patchwork av Uwe Rosenberg. Denna version använder en egen, påhittad uppsättning av 33 lappar (former, kostnader, tider och inkomster) — inte det riktiga spelets exakta bricklista.",
}

func DrawRules(screen image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	ink.ClearScreen()
	H := usableH
	W := screen.X

	tf := ink.OpenFont(ink.DefaultFontBold, 52, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((W-tw)/2, 44), title)
	tf.Close()

	margin := 40
	bh := 104
	bw := W / 2
	r := image.Rect((W-bw)/2, H-margin-bh, (W+bw)/2, H-margin)
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 1), ink.Black)
	f.Button.SetActive(ink.Black)
	drawCenteredString(r, "Tillbaka", 34)

	body := ink.OpenFont(ink.DefaultFont, 27, true)
	body.SetActive(ink.Black)
	bodyMargin := 44
	maxW := W - 2*bodyMargin
	y := 130
	lineH := 35
	paraGap := 14
	limit := r.Min.Y - 16
	for _, p := range paragraphs {
		for _, ln := range wrapText(p, maxW) {
			if y+lineH > limit {
				break
			}
			ink.DrawString(image.Pt(bodyMargin, y), ln)
			y += lineH
		}
		y += paraGap
	}
	body.Close()

	return r
}

type motifFunc func(box image.Rectangle)

func DrawSplash(screen image.Point, f *Fonts, title string, motif motifFunc) {
	ink.ClearScreen()
	H := usableH
	W := screen.X

	tf := ink.OpenFont(ink.DefaultFontBold, 76, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((W-tw)/2, H/6), title)
	tf.Close()

	w := W * 3 / 5
	h := w
	box := image.Rect((W-w)/2, (H-h)/2, (W+w)/2, (H+h)/2)
	motif(box)

	hint := ink.OpenFont(ink.DefaultFont, 32, true)
	hint.SetActive(ink.DarkGray)
	ht := "Tryck för att börja"
	hw := ink.StringWidth(ht)
	ink.DrawString(image.Pt((W-hw)/2, H*5/6), ht)
	hint.Close()
}

// drawSplashMotif renders a small 3x3 corner of a quilt board with 2-3
// differently-shaped patches already placed, per the spec's splash motif.
func drawSplashMotif(box image.Rectangle) {
	n := 3
	cell := box.Dx() / n
	if h := box.Dy() / n; h < cell {
		cell = h
	}
	w := cell * n
	ox := box.Min.X + (box.Dx()-w)/2
	oy := box.Min.Y + (box.Dy()-w)/2

	// Grid lines for the 3x3 corner.
	for i := 0; i <= n; i++ {
		ink.DrawLine(image.Pt(ox+i*cell, oy), image.Pt(ox+i*cell, oy+w), ink.Black)
		ink.DrawLine(image.Pt(ox, oy+i*cell), image.Pt(ox+w, oy+i*cell), ink.Black)
	}
	// An L-tromino patch: (0,0),(1,0),(0,1).
	for _, c := range [][2]int{{0, 0}, {1, 0}, {0, 1}} {
		r := image.Rect(ox+c[0]*cell, oy+c[1]*cell, ox+(c[0]+1)*cell, oy+(c[1]+1)*cell)
		ink.FillArea(pad(r, 2), ink.DarkGray)
	}
	// A domino patch: (2,0),(2,1) — drawn with a bonus dot to distinguish
	// it visually from the plain tromino above.
	for _, c := range [][2]int{{2, 0}, {2, 1}} {
		r := image.Rect(ox+c[0]*cell, oy+c[1]*cell, ox+(c[0]+1)*cell, oy+(c[1]+1)*cell)
		ink.FillArea(pad(r, 2), ink.DarkGray)
	}
	// A lone 1x1 bonus patch at (1,2), marked with a small white dot like
	// the real board rendering marks free special patches.
	bonusRect := image.Rect(ox+1*cell, oy+2*cell, ox+2*cell, oy+3*cell)
	ink.FillArea(pad(bonusRect, 2), ink.DarkGray)
	cx, cy := (bonusRect.Min.X+bonusRect.Max.X)/2, (bonusRect.Min.Y+bonusRect.Max.Y)/2
	fillDisc(image.Rect(cx-cell/6, cy-cell/6, cx+cell/6, cy+cell/6), ink.White)
}
