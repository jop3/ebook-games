package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"blackbox/game"
)

// Fonts held open for the app lifetime.
type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Marker *ink.Font // edge-point marker digits / H / R
}

// InitFonts opens the fonts used across screens.
func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 40, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 40, true),
		Marker: ink.OpenFont(ink.DefaultFontBold, 30, true),
	}
}

// Close releases the fonts.
func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Marker} {
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

// itoa is a tiny integer formatter to avoid pulling strconv across the UI.
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
	// statusBarTop leaves a small top margin so the centered status text
	// (approx. glyph height 40) clears the y>=40 safe-area minimum.
	statusBarTop    = 16
	statusBarHeight = 96
	buttonBarHeight = 140
	// edgeBand is the margin reserved outside the grid for the marker ring.
	edgeBand = 84
	boardPad = 12
)

// Layout maps between screen pixels and grid cells, and positions the ring of
// edge-point marker slots around the grid.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	ButtonBar image.Rectangle

	GridArea   image.Rectangle // pixel rect of the WxH cell grid
	GridOrigin image.Point
	CellSize   int
	Cols, Rows int

	edgeRects []image.Rectangle // one tappable rect per edge point index
}

// usableH is the device's real drawable height. ink.ScreenSize().Y reports
// 1448, but content below ~1360 wraps around to the top of the e-ink panel on
// actual hardware (ruler-tested on the PB634), so all vertical layout must be
// computed against this effective height instead.
const usableH = 1340

// NewLayout computes the layout for a grid on a given screen size.
func NewLayout(screen image.Point, g *game.Grid) Layout {
	H := usableH
	l := Layout{
		Screen: image.Rectangle{Max: image.Pt(screen.X, H)},
		Cols:   g.W,
		Rows:   g.H,
	}
	l.StatusBar = image.Rect(0, statusBarTop, screen.X, statusBarTop+statusBarHeight)
	l.ButtonBar = image.Rect(0, H-buttonBarHeight, screen.X, H)

	// Region available between the two bars, minus the edge band on all sides
	// for the marker ring.
	avail := image.Rect(edgeBand, l.StatusBar.Max.Y+edgeBand,
		screen.X-edgeBand, H-buttonBarHeight-edgeBand)
	avail = pad(avail, boardPad)

	cw := avail.Dx() / g.W
	ch := avail.Dy() / g.H
	cell := cw
	if ch < cell {
		cell = ch
	}
	if cell < 1 {
		cell = 1
	}
	l.CellSize = cell

	gridW := cell * g.W
	gridH := cell * g.H
	l.GridOrigin = image.Pt(
		avail.Min.X+(avail.Dx()-gridW)/2,
		avail.Min.Y+(avail.Dy()-gridH)/2,
	)
	l.GridArea = image.Rect(
		l.GridOrigin.X, l.GridOrigin.Y,
		l.GridOrigin.X+gridW, l.GridOrigin.Y+gridH,
	)

	l.computeEdgeRects(g)
	return l
}

// computeEdgeRects builds a tappable marker slot for each edge point, placed
// just outside the grid in the same clockwise order as game.Grid.EdgePoints.
func (l *Layout) computeEdgeRects(g *game.Grid) {
	eps := g.EdgePoints()
	l.edgeRects = make([]image.Rectangle, len(eps))
	c := l.CellSize
	band := edgeBand - boardPad
	for _, ep := range eps {
		ex, ey := ep.EntryCell()
		cellRect := l.CellToScreen(ex, ey)
		var r image.Rectangle
		switch ep.Side() {
		case game.SideTop:
			r = image.Rect(cellRect.Min.X, l.GridArea.Min.Y-band,
				cellRect.Min.X+c, l.GridArea.Min.Y)
		case game.SideBottom:
			r = image.Rect(cellRect.Min.X, l.GridArea.Max.Y,
				cellRect.Min.X+c, l.GridArea.Max.Y+band)
		case game.SideRight:
			r = image.Rect(l.GridArea.Max.X, cellRect.Min.Y,
				l.GridArea.Max.X+band, cellRect.Min.Y+c)
		case game.SideLeft:
			r = image.Rect(l.GridArea.Min.X-band, cellRect.Min.Y,
				l.GridArea.Min.X, cellRect.Min.Y+c)
		}
		l.edgeRects[ep.Index] = r
	}
}

// CellToScreen returns the pixel rectangle of cell (x,y).
func (l *Layout) CellToScreen(x, y int) image.Rectangle {
	return image.Rect(
		l.GridOrigin.X+x*l.CellSize,
		l.GridOrigin.Y+y*l.CellSize,
		l.GridOrigin.X+(x+1)*l.CellSize,
		l.GridOrigin.Y+(y+1)*l.CellSize,
	)
}

// ScreenToCell resolves a pixel point to a grid cell.
func (l *Layout) ScreenToCell(p image.Point) (x, y int, ok bool) {
	if l.CellSize == 0 {
		return 0, 0, false
	}
	rel := p.Sub(l.GridOrigin)
	if rel.X < 0 || rel.Y < 0 {
		return 0, 0, false
	}
	x = rel.X / l.CellSize
	y = rel.Y / l.CellSize
	if x < 0 || x >= l.Cols || y < 0 || y >= l.Rows {
		return 0, 0, false
	}
	return x, y, true
}

// EdgeAt returns the edge point index whose marker slot contains p.
func (l *Layout) EdgeAt(p image.Point) (int, bool) {
	for i, r := range l.edgeRects {
		if p.In(r) {
			return i, true
		}
	}
	return 0, false
}

// --- Rendering -------------------------------------------------------------

// Button is a labelled tappable rectangle.
type Button struct {
	Rect  image.Rectangle
	Label string
}

// Hit reports whether p falls inside the button.
func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

// DrawStatus renders the top status bar.
func DrawStatus(l *Layout, text string, f *Fonts) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	drawCenteredString(l.StatusBar, text, 40)
	ink.DrawLine(
		image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y),
		ink.Black,
	)
}

// DrawGrid renders the grid lines, guessed atoms, and (in the done phase) the
// true atom layout with correct/missed/wrong annotations.
func DrawGrid(l *Layout, s *game.GameState, f *Fonts) {
	g := s.Grid
	ink.DrawRect(l.GridArea, ink.Black)
	for x := 1; x < g.W; x++ {
		px := l.GridOrigin.X + x*l.CellSize
		ink.DrawLine(image.Pt(px, l.GridArea.Min.Y), image.Pt(px, l.GridArea.Max.Y), ink.Black)
	}
	for y := 1; y < g.H; y++ {
		py := l.GridOrigin.Y + y*l.CellSize
		ink.DrawLine(image.Pt(l.GridArea.Min.X, py), image.Pt(l.GridArea.Max.X, py), ink.Black)
	}

	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			cell := l.CellToScreen(x, y)
			guessed := s.Guesses[game.Cell{X: x, Y: y}]
			actual := g.HasAtom(x, y)

			if s.Phase == game.PhaseDone {
				switch {
				case actual && guessed:
					drawAtomFilled(cell) // correctly found
				case actual && !guessed:
					drawAtomRing(cell) // missed (outline)
				case !actual && guessed:
					drawWrongMark(cell) // wrong guess (X)
				}
			} else if guessed {
				drawAtomFilled(cell)
			}
		}
	}
}

// drawAtomFilled draws a solid atom (filled square inset).
func drawAtomFilled(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/4)
	ink.FillArea(r, ink.Black)
}

// drawAtomRing draws a hollow atom (missed): a thick ring.
func drawAtomRing(cell image.Rectangle) {
	outer := pad(cell, cell.Dx()/4)
	ink.FillArea(outer, ink.Black)
	inner := pad(outer, outer.Dx()/3)
	ink.FillArea(inner, ink.White)
}

// drawWrongMark draws an X for an incorrectly-guessed cell.
func drawWrongMark(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/4)
	for o := 0; o <= 2; o++ {
		ink.DrawLine(image.Pt(r.Min.X+o, r.Min.Y), image.Pt(r.Max.X, r.Max.Y-o), ink.Black)
		ink.DrawLine(image.Pt(r.Min.X, r.Min.Y+o), image.Pt(r.Max.X-o, r.Max.Y), ink.Black)
		ink.DrawLine(image.Pt(r.Max.X-o, r.Min.Y), image.Pt(r.Min.X, r.Max.Y-o), ink.Black)
		ink.DrawLine(image.Pt(r.Max.X, r.Min.Y+o), image.Pt(r.Min.X+o, r.Max.Y), ink.Black)
	}
}

// DrawEdgeMarkers renders the labels at each edge point: fired rays show their
// marker number (or H / R), unfired points show a small dot to hint that they
// are tappable.
func DrawEdgeMarkers(l *Layout, s *game.GameState, f *Fonts) {
	labels := map[int]string{}
	for _, fr := range s.Fired {
		switch fr.Result.Outcome {
		case game.OutcomeHit:
			labels[fr.Result.EntryIndex] = "H"
		case game.OutcomeReflection:
			labels[fr.Result.EntryIndex] = "R"
		case game.OutcomeDetour:
			m := itoa(fr.Marker)
			labels[fr.Result.EntryIndex] = m
			labels[fr.Result.ExitIndex] = m
		}
	}

	f.Marker.SetActive(ink.Black)
	for i, r := range l.edgeRects {
		if r.Empty() {
			continue
		}
		if lab, ok := labels[i]; ok {
			ink.DrawRect(r, ink.Black)
			drawCenteredString(r, lab, 30)
		} else {
			cx := (r.Min.X + r.Max.X) / 2
			cy := (r.Min.Y + r.Max.Y) / 2
			dot := image.Rect(cx-3, cy-3, cx+3, cy+3)
			ink.FillArea(dot, ink.LightGray)
		}
	}
}

// sideMargin is the horizontal safe-area inset: interactive elements (button
// borders, text) must stay within x ∈ [sideMargin, screenW-sideMargin].
const sideMargin = 24

// bottomMargin is the vertical safe-area inset from usableH: interactive
// elements must stay within y <= usableH-bottomMargin (i.e. y<=1316).
const bottomMargin = 24

// DrawButtonBar lays out buttons across the bottom bar and returns hit rects.
func DrawButtonBar(l *Layout, labels []string, f *Fonts) []Button {
	ink.FillArea(l.ButtonBar, ink.White)
	ink.DrawLine(
		image.Pt(l.ButtonBar.Min.X, l.ButtonBar.Min.Y),
		image.Pt(l.ButtonBar.Max.X, l.ButtonBar.Min.Y),
		ink.Black,
	)
	f.Button.SetActive(ink.Black)
	n := len(labels)
	if n == 0 {
		return nil
	}
	// Inset the bar horizontally so button borders never enter the side
	// margins (the raw ButtonBar spans the full screen width edge-to-edge).
	barX0 := l.ButtonBar.Min.X + sideMargin
	barX1 := l.ButtonBar.Max.X - sideMargin
	gap := 20
	totalGap := gap * (n + 1)
	bw := (barX1 - barX0 - totalGap) / n
	// Bottom-inset the button rect so it clears the real-hardware bottom
	// safe-area edge (y<=1316), not just the ButtonBar.Max.Y (=H).
	bottomInset := l.ButtonBar.Max.Y - (usableH - bottomMargin)
	if bottomInset < 0 {
		bottomInset = 0
	}
	bh := l.ButtonBar.Dy() - 2*gap - bottomInset

	buttons := make([]Button, n)
	for i, label := range labels {
		x0 := barX0 + gap + i*(bw+gap)
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

// Menu holds the start-screen state.
type Menu struct {
	presetRows []menuRow
	rulesBtn   image.Rectangle
}

// NewMenu returns a fresh menu.
func NewMenu() *Menu { return &Menu{} }

// RulesButton returns the tappable "Regler" rect (set during Draw).
func (m *Menu) RulesButton() image.Rectangle { return m.rulesBtn }

// Draw renders the difficulty menu and records hit regions.
func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()

	title := ink.OpenFont(ink.DefaultFontBold, 56, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Black Box")
	ink.DrawString(image.Pt((screen.X-tw)/2, 60), "Black Box")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 34, true)
	sub.SetActive(ink.Black)
	subT := "Välj svårighetsgrad"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 150), subT)
	sub.Close()

	f.Menu.SetActive(ink.Black)
	y := 260
	rowH := 120
	margin := 60
	rowW := screen.X - 2*margin

	m.presetRows = m.presetRows[:0]
	for _, p := range game.Presets {
		r := image.Rect(margin, y, margin+rowW, y+rowH-20)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawLeftString(r, p.Name, 40)
		m.presetRows = append(m.presetRows, menuRow{rect: r, label: p.Name})
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

// HandleTouch returns the index of a tapped preset, or -1.
func (m *Menu) HandleTouch(p image.Point) int {
	for i := range m.presetRows {
		if p.In(m.presetRows[i].rect) {
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
	"Mål: hitta de dolda atomerna inne i rutnätet — utan att se dem.",
	"Du skjuter strålar in från kantpunkterna runt rutnätet. Strålen studsar på osynliga regler beroende på var atomerna sitter, och du läser av vad som händer.",
	"H (träff): strålen gick rakt in i en atom och försvann.",
	"R (retur): strålen kom ut samma väg den gick in (t.ex. en atom rakt framför eller två atomer som speglar).",
	"En siffra: strålen tog en omväg och kom ut på ett annat kantställe. Samma siffra visas vid både ingången och utgången så du ser paret.",
	"Så böjer atomer strålen: en atom snett framför strålen böjer den 90° bort från atomen. En atom rakt framför stoppar den (H).",
	"När du tror dig veta var atomerna är: tryck \"Gissa atomer\", markera rutorna och tryck \"Lämna in\".",
	"Poäng = antal skjutna strålar + straff för varje felgissad atom. Lägre är bättre.",
}

// DrawRules renders the rules text with a back button and returns its rect.
func DrawRules(screen image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	ink.ClearScreen()

	tf := ink.OpenFont(ink.DefaultFontBold, 56, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, 50), title)
	tf.Close()

	body := ink.OpenFont(ink.DefaultFont, 32, true)
	body.SetActive(ink.Black)
	margin := 56
	maxW := screen.X - 2*margin
	y := 150
	lineH := 44
	paraGap := 20
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
	bottom := usableH - bottomMargin
	r := image.Rect((screen.X-bw)/2, bottom-bh, (screen.X+bw)/2, bottom)
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
func DrawSplash(screen image.Point, f *Fonts, title string, motif motifFunc) {
	ink.ClearScreen()

	tf := ink.OpenFont(ink.DefaultFontBold, 76, true)
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

// splashDisc fills an approximate circle (self-contained for the splash).
func splashDisc(cx, cy, rad int) {
	rr := rad * rad
	for dy := -rad; dy <= rad; dy++ {
		w := 0
		for w*w+dy*dy <= rr {
			w++
		}
		ink.DrawLine(image.Pt(cx-w, cy+dy), image.Pt(cx+w, cy+dy), ink.Black)
	}
}

// drawSplashMotif draws a 5x5 grid with one hidden atom and a ray that enters
// from the left edge and deflects around the atom — the essence of Black Box.
func drawSplashMotif(box image.Rectangle) {
	const n = 5
	cell := box.Dx() / n
	ox := box.Min.X + (box.Dx()-cell*n)/2
	oy := box.Min.Y + (box.Dy()-cell*n)/2
	grid := image.Rect(ox, oy, ox+cell*n, oy+cell*n)

	ink.DrawRect(grid, ink.Black)
	ink.DrawRect(pad(grid, 1), ink.Black)
	for i := 1; i < n; i++ {
		x := ox + i*cell
		ink.DrawLine(image.Pt(x, oy), image.Pt(x, oy+cell*n), ink.LightGray)
		y := oy + i*cell
		ink.DrawLine(image.Pt(ox, y), image.Pt(ox+cell*n, y), ink.LightGray)
	}

	// Atom at cell (2,1) (col,row).
	acx := ox + 2*cell + cell/2
	acy := oy + 1*cell + cell/2
	splashDisc(acx, acy, cell/3)

	// Ray: enters left at row 3, travels right, deflects up when it reaches the
	// column just below the atom, exits top. Two thick segments.
	entryY := oy + 3*cell + cell/2
	turnX := ox + 2*cell + cell/2
	for o := -2; o <= 2; o++ {
		ink.DrawLine(image.Pt(ox-cell/2, entryY+o), image.Pt(turnX, entryY+o), ink.Black)
		ink.DrawLine(image.Pt(turnX+o, entryY), image.Pt(turnX+o, oy+2*cell), ink.Black)
	}
}
