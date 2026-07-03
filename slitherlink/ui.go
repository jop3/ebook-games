package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"slitherlink/game"
)

type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Clue   *ink.Font // clue numbers; opened per size in Init via SetActive reuse
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 32, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 40, true),
		Clue:   ink.OpenFont(ink.DefaultFontBold, 30, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Clue} {
		if fn != nil {
			fn.Close()
		}
	}
}

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
	x := r.Min.X + 24
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

// --- Layout ----------------------------------------------------------------

const (
	usableH         = 1340 // real drawable height; ScreenSize().Y (1448) lies
	statusBarHeight = 84
	buttonBarHeight = 140
	boardMargin     = 16
)

// Layout positions the dot grid (W+1 x H+1 dots forming W x H cells). All
// positions derive from CellSize/GridOrigin so nothing overflows the usable
// 1072x1340 area.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	ButtonBar image.Rectangle

	GridArea   image.Rectangle // dot-to-dot bounding box
	GridOrigin image.Point     // screen position of dot (0,0)
	CellSize   int
	W, H       int
}

func NewLayout(screenPt image.Point, puz *game.Puzzle) Layout {
	screen := image.Pt(screenPt.X, usableH)
	l := Layout{Screen: image.Rectangle{Max: screen}, W: puz.W, H: puz.H}
	l.StatusBar = image.Rect(0, 40, screen.X, 40+statusBarHeight)
	l.ButtonBar = image.Rect(0, screen.Y-buttonBarHeight-40, screen.X, screen.Y-40)

	avail := image.Rect(boardMargin, l.StatusBar.Max.Y+boardMargin,
		screen.X-boardMargin, l.ButtonBar.Min.Y-boardMargin)

	// Largest cell size so the W x H grid of cells fits both dimensions.
	cell := 0
	for c := 140; c >= 12; c-- {
		gw := c * puz.W
		gh := c * puz.H
		if gw <= avail.Dx() && gh <= avail.Dy() {
			cell = c
			break
		}
	}
	if cell == 0 {
		cell = 12
	}
	l.CellSize = cell

	gridW := cell * puz.W
	gridH := cell * puz.H
	ox := avail.Min.X + (avail.Dx()-gridW)/2
	oy := avail.Min.Y + (avail.Dy()-gridH)/2
	l.GridOrigin = image.Pt(ox, oy)
	l.GridArea = image.Rect(ox, oy, ox+gridW, oy+gridH)
	return l
}

// DotScreen returns the screen point of dot (x,y).
func (l *Layout) DotScreen(x, y int) image.Point {
	return image.Pt(l.GridOrigin.X+x*l.CellSize, l.GridOrigin.Y+y*l.CellSize)
}

// CellToScreen returns the screen rect for cell (x,y), used to center its
// clue number.
func (l *Layout) CellToScreen(x, y int) image.Rectangle {
	return image.Rect(
		l.GridOrigin.X+x*l.CellSize,
		l.GridOrigin.Y+y*l.CellSize,
		l.GridOrigin.X+(x+1)*l.CellSize,
		l.GridOrigin.Y+(y+1)*l.CellSize,
	)
}

// ScreenToEdge hit-tests a tap against the midpoints of all horizontal and
// vertical edges, returning the nearest one if it falls within a generous
// tappable band (edges are thin, so this is the trickiest input in the
// game). isH=true means an HEdge(x,y); isH=false means a VEdge(x,y).
//
// For each candidate edge we compute two distances: "along" (distance from
// the tap to the segment's own axis, projected onto its length — must be
// within half the segment's length so the whole segment, not just its exact
// center, is tappable) and "perp" (perpendicular distance from the tap to the
// segment's line — must be within the tappable band). Among all candidates
// that pass both checks, we keep the one with the smallest perpendicular
// distance.
func (l *Layout) ScreenToEdge(p image.Point) (isH bool, x, y int, ok bool) {
	if l.CellSize == 0 {
		return false, 0, 0, false
	}
	half := l.CellSize / 2
	band := half
	if band < 24 {
		band = 24
	}

	bestPerp := band + 1
	found := false

	// Horizontal edges: run along X, midpoint at (x+0.5, y) in dot units.
	for ey := 0; ey <= l.H; ey++ {
		for ex := 0; ex < l.W; ex++ {
			midX := l.GridOrigin.X + ex*l.CellSize + half
			lineY := l.GridOrigin.Y + ey*l.CellSize
			along := p.X - midX
			perp := abs(p.Y - lineY)
			if abs(along) > half || perp > band {
				continue
			}
			if perp < bestPerp {
				bestPerp, isH, x, y, found = perp, true, ex, ey, true
			}
		}
	}
	// Vertical edges: run along Y, midpoint at (x, y+0.5) in dot units.
	for ey := 0; ey < l.H; ey++ {
		for ex := 0; ex <= l.W; ex++ {
			midY := l.GridOrigin.Y + ey*l.CellSize + half
			lineX := l.GridOrigin.X + ex*l.CellSize
			along := p.Y - midY
			perp := abs(p.X - lineX)
			if abs(along) > half || perp > band {
				continue
			}
			if perp < bestPerp {
				bestPerp, isH, x, y, found = perp, false, ex, ey, true
			}
		}
	}
	return isH, x, y, found
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// --- Rendering -------------------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

func DrawStatus(l *Layout, text string, f *Fonts) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	drawCenteredString(l.StatusBar, text, 32)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawGrid renders the dot grid, clue numbers, ON edges (thick black
// segments), and OFF marks (small X at the segment midpoint).
func DrawGrid(l *Layout, s *game.GameState, f *Fonts) {
	b := s.Bd

	// Faint dot-to-dot grid lines first, as a guide.
	for y := 0; y <= l.H; y++ {
		p0 := l.DotScreen(0, y)
		p1 := l.DotScreen(l.W, y)
		ink.DrawLine(p0, p1, ink.LightGray)
	}
	for x := 0; x <= l.W; x++ {
		p0 := l.DotScreen(x, 0)
		p1 := l.DotScreen(x, l.H)
		ink.DrawLine(p0, p1, ink.LightGray)
	}

	// Clue numbers, centered in their cell.
	f.Clue.SetActive(ink.Black)
	for y := 0; y < l.H; y++ {
		for x := 0; x < l.W; x++ {
			c := b.Clue[y][x]
			if c < 0 {
				continue
			}
			drawCenteredString(l.CellToScreen(x, y), itoa(c), 30)
		}
	}

	// ON edges: thick black segments.
	thick := l.CellSize / 8
	if thick < 3 {
		thick = 3
	}
	for y := 0; y <= l.H; y++ {
		for x := 0; x < l.W; x++ {
			switch b.HEdge[y][x] {
			case game.EdgeOn:
				drawThickLine(l.DotScreen(x, y), l.DotScreen(x+1, y), thick)
			case game.EdgeOff:
				drawEdgeX(midPoint(l.DotScreen(x, y), l.DotScreen(x+1, y)), l.CellSize)
			}
		}
	}
	for y := 0; y < l.H; y++ {
		for x := 0; x <= l.W; x++ {
			switch b.VEdge[y][x] {
			case game.EdgeOn:
				drawThickLine(l.DotScreen(x, y), l.DotScreen(x, y+1), thick)
			case game.EdgeOff:
				drawEdgeX(midPoint(l.DotScreen(x, y), l.DotScreen(x, y+1)), l.CellSize)
			}
		}
	}

	// Dots on top.
	r := l.CellSize / 14
	if r < 2 {
		r = 2
	}
	for y := 0; y <= l.H; y++ {
		for x := 0; x <= l.W; x++ {
			p := l.DotScreen(x, y)
			ink.FillArea(image.Rect(p.X-r, p.Y-r, p.X+r, p.Y+r), ink.Black)
		}
	}
}

func midPoint(a, b image.Point) image.Point {
	return image.Pt((a.X+b.X)/2, (a.Y+b.Y)/2)
}

// drawThickLine draws a line with the given thickness by stacking parallel
// 1px lines (inkview has no line-width parameter).
func drawThickLine(a, b image.Point, thick int) {
	if a.Y == b.Y { // horizontal
		for i := -thick / 2; i <= thick/2; i++ {
			ink.DrawLine(image.Pt(a.X, a.Y+i), image.Pt(b.X, b.Y+i), ink.Black)
		}
	} else { // vertical
		for i := -thick / 2; i <= thick/2; i++ {
			ink.DrawLine(image.Pt(a.X+i, a.Y), image.Pt(b.X+i, b.Y), ink.Black)
		}
	}
}

// drawEdgeX draws a small X centered at an edge's midpoint, marking it as
// definitely not part of the loop.
func drawEdgeX(c image.Point, cellSize int) {
	r := cellSize / 8
	if r < 6 {
		r = 6
	}
	ink.DrawLine(image.Pt(c.X-r, c.Y-r), image.Pt(c.X+r, c.Y+r), ink.DarkGray)
	ink.DrawLine(image.Pt(c.X+r, c.Y-r), image.Pt(c.X-r, c.Y+r), ink.DarkGray)
}

func DrawButtonBar(l *Layout, labels []string, f *Fonts) []Button {
	ink.FillArea(l.ButtonBar, ink.White)
	ink.DrawLine(image.Pt(l.ButtonBar.Min.X, l.ButtonBar.Min.Y),
		image.Pt(l.ButtonBar.Max.X, l.ButtonBar.Min.Y), ink.Black)
	f.Button.SetActive(ink.Black)
	n := len(labels)
	if n == 0 {
		return nil
	}
	gap := 20
	sideMargin := 28
	totalGap := gap*(n-1) + 2*sideMargin
	bw := (l.ButtonBar.Dx() - totalGap) / n
	bh := l.ButtonBar.Dy() - 2*gap
	buttons := make([]Button, n)
	for i, label := range labels {
		x0 := l.ButtonBar.Min.X + sideMargin + i*(bw+gap)
		y0 := l.ButtonBar.Min.Y + gap
		r := image.Rect(x0, y0, x0+bw, y0+bh)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawCenteredString(r, label, 38)
		buttons[i] = Button{Rect: r, Label: label}
	}
	return buttons
}

// --- Menu ------------------------------------------------------------------

type menuRow struct {
	rect  image.Rectangle
	label string
}

type Menu struct {
	rows     []menuRow
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{} }

// RulesButton returns the tappable "Regler" rect (set during Draw).
func (m *Menu) RulesButton() image.Rectangle { return m.rulesBtn }

func (m *Menu) Draw(screenPt image.Point, f *Fonts) {
	screen := image.Pt(screenPt.X, usableH)
	ink.ClearScreen()

	title := ink.OpenFont(ink.DefaultFontBold, 60, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Slitherlink")
	ink.DrawString(image.Pt((screen.X-tw)/2, 70), "Slitherlink")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 34, true)
	sub.SetActive(ink.Black)
	subT := "Välj storlek"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 170), subT)
	sub.Close()

	f.Menu.SetActive(ink.Black)
	y := 300
	rowH := 130
	margin := 60
	rowW := screen.X - 2*margin

	m.rows = m.rows[:0]
	for _, p := range game.Presets {
		r := image.Rect(margin, y, margin+rowW, y+rowH-20)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawLeftString(r, p.Name, 40)
		m.rows = append(m.rows, menuRow{rect: r, label: p.Name})
		y += rowH
	}

	// "Regler" button opens the full rules screen.
	rbW := rowW / 2
	rb := image.Rect((screen.X-rbW)/2, y+30, (screen.X+rbW)/2, y+30+100)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb
}

func (m *Menu) HandleTouch(p image.Point) int {
	for i := range m.rows {
		if p.In(m.rows[i].rect) {
			return i
		}
	}
	return -1
}

// --- Rules screen ----------------------------------------------------------

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

var rulesParagraphs = []string{
	"Mål: rita en enda sluten slinga längs rutnätets linjer, så att varje siffra i rutnätet stämmer.",
	"Rutnätet består av prickar. Mellan varannan prick finns en kant (ett streck) som du kan slå på eller av. Siffran i en ruta (0–3) anger exakt hur många av dess fyra kanter som ska vara på i den färdiga slingan. Tomma rutor har ingen begränsning.",
	"Tryck nära mitten av en kant för att växla: streck (på) → X (säkert av) → tom. X är bara din egen minnesanteckning.",
	"Slingan måste vara en enda sammanhängande, sluten bana utan grenar eller korsningar — varje prick som ingår i slingan har exakt två kanter på.",
	"Alla pussel är gjorda så att de går att lösa med ren logik — du behöver aldrig gissa.",
	"Knappen Rensa nollställer hela rutnätet om du vill börja om.",
}

// DrawRules renders the rules text with a back button and returns its rect.
func DrawRules(screenPt image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	screen := image.Pt(screenPt.X, usableH)
	ink.ClearScreen()

	tf := ink.OpenFont(ink.DefaultFontBold, 56, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, 60), title)
	tf.Close()

	body := ink.OpenFont(ink.DefaultFont, 34, true)
	body.SetActive(ink.Black)
	margin := 60
	maxW := screen.X - 2*margin
	y := 180
	lineH := 46
	paraGap := 24
	for _, p := range paragraphs {
		for _, ln := range wrapText(p, maxW) {
			ink.DrawString(image.Pt(margin, y), ln)
			y += lineH
		}
		y += paraGap
	}
	body.Close()

	bh := 110
	bw := screen.X / 2
	r := image.Rect((screen.X-bw)/2, screen.Y-bh-40, (screen.X+bw)/2, screen.Y-40)
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 1), ink.Black)
	f.Button.SetActive(ink.Black)
	drawCenteredString(r, "Tillbaka", 38)
	return r
}

// --- Splash screen ---------------------------------------------------------

type motifFunc func(box image.Rectangle)

// DrawSplash renders the start screen: title, a large line-art motif, and a
// "tap to start" hint — echoing the built-in chess app's opening screen.
func DrawSplash(screenPt image.Point, f *Fonts, title string, motif motifFunc) {
	screen := image.Pt(screenPt.X, usableH)
	ink.ClearScreen()

	tf := ink.OpenFont(ink.DefaultFontBold, 80, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, screen.Y/6), title)
	tf.Close()

	side := screen.X * 3 / 5
	box := image.Rect((screen.X-side)/2, (screen.Y-side)/2,
		(screen.X+side)/2, (screen.Y+side)/2)
	motif(box)

	hint := ink.OpenFont(ink.DefaultFont, 34, true)
	hint.SetActive(ink.DarkGray)
	ht := "Tryck för att börja"
	hw := ink.StringWidth(ht)
	ink.DrawString(image.Pt((screen.X-hw)/2, screen.Y*5/6), ht)
	hint.Close()
}

// drawSplashMotif draws a small 3x3 dot grid with a short closed loop
// (a 2x1 rectangle) traced in thick lines, plus a couple of clue numbers —
// the essence of the game in miniature.
func drawSplashMotif(box image.Rectangle) {
	const n = 3 // 3x3 dots -> 2x2 cells
	cell := box.Dx() / n
	ox := box.Min.X + (box.Dx()-cell*(n-1))/2
	oy := box.Min.Y + (box.Dy()-cell*(n-1))/2

	dot := func(x, y int) image.Point { return image.Pt(ox+x*cell, oy+y*cell) }

	// Faint full dot grid.
	for y := 0; y < n; y++ {
		ink.DrawLine(dot(0, y), dot(n-1, y), ink.LightGray)
	}
	for x := 0; x < n; x++ {
		ink.DrawLine(dot(x, 0), dot(x, n-1), ink.LightGray)
	}

	// A loop tracing the left 1x2 rectangle: (0,0)-(1,0)-(1,2)-(0,2)-(0,0).
	thick := cell / 8
	if thick < 3 {
		thick = 3
	}
	loop := []image.Point{dot(0, 0), dot(1, 0), dot(1, 2), dot(0, 2), dot(0, 0)}
	for i := 0; i < len(loop)-1; i++ {
		a, b := loop[i], loop[i+1]
		if a.Y == b.Y {
			for t := -thick / 2; t <= thick/2; t++ {
				ink.DrawLine(image.Pt(a.X, a.Y+t), image.Pt(b.X, b.Y+t), ink.Black)
			}
		} else {
			for t := -thick / 2; t <= thick/2; t++ {
				ink.DrawLine(image.Pt(a.X+t, a.Y), image.Pt(b.X+t, b.Y), ink.Black)
			}
		}
	}

	// Dots.
	r := cell / 14
	if r < 3 {
		r = 3
	}
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			p := dot(x, y)
			ink.FillArea(image.Rect(p.X-r, p.Y-r, p.X+r, p.Y+r), ink.Black)
		}
	}

	// Clue numbers: a 3 inside the loop cell, a 1 in the empty right cell.
	clueFont := ink.OpenFont(ink.DefaultFontBold, cell/3, true)
	clueFont.SetActive(ink.Black)
	put := func(cellX, cellY int, s string) {
		w := ink.StringWidth(s)
		topLeft := dot(cellX, cellY)
		botRight := dot(cellX+1, cellY+1)
		cxp := (topLeft.X + botRight.X) / 2
		cyp := (topLeft.Y + botRight.Y) / 2
		ink.DrawString(image.Pt(cxp-w/2, cyp-cell/6), s)
	}
	put(0, 0, "3")
	put(1, 0, "1")
	clueFont.Close()
}
