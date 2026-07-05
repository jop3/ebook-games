package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"munkar/game"
)

// Fonts held open for the app lifetime (opened once — never per draw).
type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Small  *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 36, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 38, true),
		Small:  ink.OpenFont(ink.DefaultFont, 28, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Small} {
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
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	statusBarHeight = 96
	buttonBarHeight = 140
	boardMargin     = 24
	topMargin       = 40
)

// Layout maps between screen pixels and the 6x6 board.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	ButtonBar image.Rectangle

	BoardArea  image.Rectangle
	GridOrigin image.Point
	CellSize   int
}

func NewLayout(screen image.Point) Layout {
	H := usableH
	l := Layout{Screen: image.Rect(0, 0, screen.X, H)}
	l.StatusBar = image.Rect(0, topMargin, screen.X, topMargin+statusBarHeight)
	l.ButtonBar = image.Rect(0, H-topMargin-buttonBarHeight, screen.X, H-topMargin)

	avail := image.Rect(boardMargin, l.StatusBar.Max.Y+boardMargin,
		screen.X-boardMargin, l.ButtonBar.Min.Y-boardMargin)
	side := avail.Dx()
	if avail.Dy() < side {
		side = avail.Dy()
	}
	cell := side / game.Size
	if cell < 1 {
		cell = 1
	}
	l.CellSize = cell
	boardPx := cell * game.Size
	l.GridOrigin = image.Pt(
		avail.Min.X+(avail.Dx()-boardPx)/2,
		avail.Min.Y+(avail.Dy()-boardPx)/2,
	)
	l.BoardArea = image.Rect(l.GridOrigin.X, l.GridOrigin.Y,
		l.GridOrigin.X+boardPx, l.GridOrigin.Y+boardPx)
	return l
}

func (l *Layout) CellToScreen(x, y int) image.Rectangle {
	return image.Rect(
		l.GridOrigin.X+x*l.CellSize,
		l.GridOrigin.Y+y*l.CellSize,
		l.GridOrigin.X+(x+1)*l.CellSize,
		l.GridOrigin.Y+(y+1)*l.CellSize,
	)
}

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
	if x < 0 || x >= game.Size || y < 0 || y >= game.Size {
		return 0, 0, false
	}
	return x, y, true
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
	drawCenteredString(l.StatusBar, text, 36)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// faintGray is a lighter tone than ink.LightGray, used for the per-cell line
// glyphs so they read as background decoration and never compete with the
// rings or the forced-line highlight drawn on top of them.
var faintGray = color.Gray{Y: 0xd6}

// DrawBoard renders the grid, each cell's faint line glyph, the forced-line
// highlight band (if any), the rings, and a brief flash over any rings
// flipped by the most recent placement.
func DrawBoard(l *Layout, s *game.GameState, f *Fonts) {
	// Grid frame + lines.
	ink.DrawRect(l.BoardArea, ink.Black)
	ink.DrawRect(pad(l.BoardArea, 1), ink.Black)
	for i := 1; i < game.Size; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.BoardArea.Min.Y), image.Pt(px, l.BoardArea.Max.Y), ink.Black)
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, py), image.Pt(l.BoardArea.Max.X, py), ink.Black)
	}

	// Faint line glyphs, one per cell, drawn before the highlight/rings so
	// they always sit underneath.
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			drawLineGlyph(l.CellToScreen(x, y), s.Board.Line[y][x])
		}
	}

	// Forced-line highlight: a thin band over every cell on the line the
	// side to move must play on next (if any constraint is active).
	if s.Phase == game.PhasePlaying {
		for _, p := range s.ForcedLine() {
			r := pad(l.CellToScreen(p.X, p.Y), l.CellSize/10)
			ink.DrawRect(r, ink.LightGray)
		}
	}

	// Rings. Black = filled disc; White = hollow (ring) disc — same
	// convention as othello/hasami's discs/men, reused via fillDisc.
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			switch s.Board.At(x, y) {
			case game.Black:
				drawRing(l.CellToScreen(x, y), true)
			case game.White:
				drawRing(l.CellToScreen(x, y), false)
			}
		}
	}

	// Briefly mark rings flipped by the most recent placement with a bold
	// double border — the same "single frame flash" idea as hasami's
	// capture marker, just drawn around a ring instead of an empty cell
	// (Munkar's flips recolor a ring rather than removing it).
	for _, p := range s.LastFlips {
		r := pad(l.CellToScreen(p.X, p.Y), l.CellSize/8)
		ink.DrawRect(r, ink.DarkGray)
		ink.DrawRect(pad(r, 1), ink.DarkGray)
	}
}

// drawLineGlyph draws the faint line-art for a single cell's orientation.
func drawLineGlyph(cell image.Rectangle, o game.Orient) {
	r := pad(cell, cell.Dx()/5)
	cx, cy := (r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2
	switch o {
	case game.OrientH:
		ink.DrawLine(image.Pt(r.Min.X, cy), image.Pt(r.Max.X, cy), faintGray)
	case game.OrientV:
		ink.DrawLine(image.Pt(cx, r.Min.Y), image.Pt(cx, r.Max.Y), faintGray)
	case game.OrientD1: // "╱": bottom-left to top-right
		ink.DrawLine(image.Pt(r.Min.X, r.Max.Y), image.Pt(r.Max.X, r.Min.Y), faintGray)
	case game.OrientD2: // "╲": top-left to bottom-right
		ink.DrawLine(image.Pt(r.Min.X, r.Min.Y), image.Pt(r.Max.X, r.Max.Y), faintGray)
	}
}

// drawRing draws a ring inside a cell. black=true renders a solid disc
// (Black); false renders a hollow (white) disc with a black outline so both
// read on e-ink.
func drawRing(cell image.Rectangle, black bool) {
	r := pad(cell, cell.Dx()/8)
	fillDisc(r, ink.Black)
	if !black {
		inner := pad(r, r.Dx()/8+2)
		fillDisc(inner, ink.White)
	}
}

// fillDisc approximates a filled circle inside rect r using horizontal spans.
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

// isqrt is an integer square root (floor).
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
	sideMargin := 24
	usableW := l.ButtonBar.Dx() - 2*sideMargin
	totalGap := gap * (n + 1)
	bw := (usableW - totalGap) / n
	bh := l.ButtonBar.Dy() - 2*gap
	buttons := make([]Button, n)
	for i, label := range labels {
		x0 := l.ButtonBar.Min.X + sideMargin + gap + i*(bw+gap)
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

type menuChoice struct {
	mode    game.Mode
	aiLevel int
	label   string
}

type menuRow struct {
	rect   image.Rectangle
	choice menuChoice
}

type Menu struct {
	rows     []menuRow
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{} }

// choices lists the game modes offered on the start screen.
var choices = []menuChoice{
	{game.ModeHotseat, 0, "2 spelare"},
	{game.ModeAI, game.DepthEasy, "Mot dator – Lätt"},
	{game.ModeAI, game.DepthMedium, "Mot dator – Medel"},
	{game.ModeAI, game.DepthHard, "Mot dator – Svår"},
}

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	H := usableH

	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Munkar")
	ink.DrawString(image.Pt((screen.X-tw)/2, 70), "Munkar")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 34, true)
	sub.SetActive(ink.Black)
	subT := "Välj spelläge"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 170), subT)
	sub.Close()

	// Bottom-anchored "Regler" button, stacked up from H-margin.
	margin := 60
	rowW := screen.X - 2*margin
	rbW := rowW / 2
	rbH := 100
	rb := image.Rect((screen.X-rbW)/2, H-margin-rbH, (screen.X+rbW)/2, H-margin)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	// Mode rows fill the space between the subtitle and the Regler button.
	f.Menu.SetActive(ink.Black)
	rowH := 130
	top := 300
	bottom := rb.Min.Y - 30
	n := len(choices)
	avail := bottom - top
	if avail < rowH*n {
		rowH = avail / n
	}
	y := top

	m.rows = m.rows[:0]
	for _, c := range choices {
		r := image.Rect(margin, y, margin+rowW, y+rowH-20)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawLeftString(r, c.label, 40)
		m.rows = append(m.rows, menuRow{rect: r, choice: c})
		y += rowH
	}
}

func (m *Menu) HandleTouch(p image.Point) (menuChoice, bool) {
	for i := range m.rows {
		if p.In(m.rows[i].rect) {
			return m.rows[i].choice, true
		}
	}
	return menuChoice{}, false
}

// RulesButton is the tappable "Regler" chip on the menu; set during Draw.
func (m *Menu) RulesButton() image.Rectangle { return m.rulesBtn }

// --- Rules screen ------------------------------------------------------------

// wrapText breaks s into lines no wider than maxW pixels for the active font.
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

// splitWords splits on spaces (small helper to avoid importing strings).
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

// rulesParagraphs is the rules text for Munkar, one entry per paragraph. Kept
// deliberately tight (a fixed screen height with no scrolling, like the
// other games in this repo) so the credit line at the end always fits.
var rulesParagraphs = []string{
	"Mål: bilda en rad av 5 av dina egna ringar – vågrätt, lodrätt eller diagonalt.",
	"Brädet är 6x6, byggt av fyra 3x3-plattor. Varje ruta har en linje: vågrät (—), lodrät (│) eller diagonal (stigande eller fallande).",
	"Spelarna turas om att placera en ring på en tom ruta. Den som börjar (Svart) får placera var som helst.",
	"Tvingad riktning: linjen i rutan du just fyllde pekar ut raden, kolumnen eller diagonalen din motståndare måste placera på härnäst. Är den linjen full får motståndaren placera var som helst.",
	"Erövring: efter din placering, se längs alla fyra riktningar genom rutan. Bildar dina ringar (inklusive den nya) en rad omsluten av en motståndarring i BÅDA ändar, vänds FIENDERINGARNA i ändarna till din färg – aldrig tvärtom. Exempel: fiende–tomt–fiende blir fiende–du–fiende när du fyller luckan.",
	"Fem i rad vinner omedelbart. Blir brädet fullt utan det, avgörs partiet av vem som har den största sammanhängande gruppen (vågräta/lodräta grannar, inte diagonala). Lika stora grupper ger oavgjort.",
	"Tryck på en tom (och tillåten) ruta för att placera. Den markerade linjen visar var du får spela.",
	"Baserat på Donuts (Funforge).",
}

// DrawRules renders the scrolling rules text with a back button and returns
// the back button rect.
func DrawRules(screen image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	ink.ClearScreen()
	H := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 56, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, 50), title)
	tf.Close()

	// Back button, bottom-anchored.
	margin := 40
	bh := 110
	bw := screen.X / 2
	r := image.Rect((screen.X-bw)/2, H-margin-bh, (screen.X+bw)/2, H-margin)
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 1), ink.Black)
	f.Button.SetActive(ink.Black)
	drawCenteredString(r, "Tillbaka", 38)

	body := ink.OpenFont(ink.DefaultFont, 28, true)
	body.SetActive(ink.Black)
	bodyMargin := 50
	maxW := screen.X - 2*bodyMargin
	y := 150
	lineH := 38
	paraGap := 14
	limit := r.Min.Y - 20
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

// --- Splash screen -----------------------------------------------------------

// motifFunc draws the game's line-art centered in the given box.
type motifFunc func(box image.Rectangle)

// DrawSplash renders the start screen: the game title, a large line-art
// motif, and a "tap to start" hint — echoing the built-in chess app's
// opening screen (and this repo's other games).
func DrawSplash(screen image.Point, f *Fonts, title string, motif motifFunc) {
	ink.ClearScreen()
	H := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 80, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, H/6), title)
	tf.Close()

	// Motif occupies a centered box in the middle of the screen.
	w := screen.X * 4 / 5
	h := screen.X * 2 / 5
	box := image.Rect((screen.X-w)/2, (H-h)/2, (screen.X+w)/2, (H+h)/2)
	motif(box)

	hint := ink.OpenFont(ink.DefaultFont, 34, true)
	hint.SetActive(ink.DarkGray)
	ht := "Tryck för att börja"
	hw := ink.StringWidth(ht)
	ink.DrawString(image.Pt((screen.X-hw)/2, H*5/6), ht)
	hint.Close()
}

// drawSplashMotif echoes the game's two core ideas: a couple of small board
// cells showing their faint line glyphs, and a short row of three rings —
// hollow, filled, hollow (○ ● ○) — with small OUTWARD-pointing arrows from
// the center ring toward each outer one, showing that placing the middle
// (filled) ring is what flips the two outer (enemy) rings toward its color.
// This is the mirror image of hasami's inward-arrow motif, since Munkar's
// capture direction is inverted: the mover's own ring in the middle causes
// the flip to radiate OUT to the enemy bookends, not in from them.
func drawSplashMotif(box image.Rectangle) {
	cx := (box.Min.X + box.Max.X) / 2
	cy := (box.Min.Y+box.Max.Y)/2 + box.Dy()/6

	// A couple of small decorative cells with faint line glyphs, above the
	// ring flank.
	cellSize := box.Dx() / 6
	glyphY := box.Min.Y + box.Dy()/8
	glyphs := [3]game.Orient{game.OrientD2, game.OrientH, game.OrientD1}
	gap := cellSize + cellSize/2
	for i, o := range glyphs {
		x0 := cx - gap + i*gap
		cell := image.Rect(x0-cellSize/2, glyphY-cellSize/2, x0+cellSize/2, glyphY+cellSize/2)
		ink.DrawRect(cell, ink.LightGray)
		drawLineGlyph(cell, o)
	}

	r := box.Dx() / 8
	gapR := r * 3
	centers := [3]image.Point{{X: cx - gapR, Y: cy}, {X: cx, Y: cy}, {X: cx + gapR, Y: cy}}
	filled := [3]bool{false, true, false}
	for i, c := range centers {
		cell := image.Rect(c.X-r, c.Y-r, c.X+r, c.Y+r)
		if filled[i] {
			fillDisc(cell, ink.Black)
		} else {
			fillDisc(cell, ink.Black)
			fillDisc(pad(cell, r/4+3), ink.White)
		}
	}
	arrowY := cy - r*2
	drawOutwardArrow(image.Pt(cx-r*2, arrowY), false, r)
	drawOutwardArrow(image.Pt(cx+r*2, arrowY), true, r)
}

// drawOutwardArrow draws a small chevron pointing away from the motif's
// center ring, toward one of the two outer rings it flips.
func drawOutwardArrow(c image.Point, pointRight bool, size int) {
	dx := size
	if !pointRight {
		dx = -dx
	}
	tip := image.Pt(c.X+dx/2, c.Y)
	tailA := image.Pt(c.X-dx/2, c.Y-size/2)
	tailB := image.Pt(c.X-dx/2, c.Y+size/2)
	ink.DrawLine(tip, tailA, ink.Black)
	ink.DrawLine(tip, tailB, ink.Black)
}
