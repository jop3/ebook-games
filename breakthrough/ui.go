package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"breakthrough/game"
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

// --- Layout ------------------------------------------------------------

const (
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	statusBarHeight = 96
	buttonBarHeight = 140
	boardMargin     = 16
	topMargin       = 40
)

// Layout maps between screen pixels and the Cols x Rows board.
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
	cellW := avail.Dx() / game.Cols
	cellH := avail.Dy() / game.Rows
	cell := cellW
	if cellH < cell {
		cell = cellH
	}
	if cell < 1 {
		cell = 1
	}
	l.CellSize = cell
	boardW := cell * game.Cols
	boardH := cell * game.Rows
	l.GridOrigin = image.Pt(
		avail.Min.X+(avail.Dx()-boardW)/2,
		avail.Min.Y+(avail.Dy()-boardH)/2,
	)
	l.BoardArea = image.Rect(l.GridOrigin.X, l.GridOrigin.Y,
		l.GridOrigin.X+boardW, l.GridOrigin.Y+boardH)
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
	if x < 0 || x >= game.Cols || y < 0 || y >= game.Rows {
		return 0, 0, false
	}
	return x, y, true
}

// --- Rendering -----------------------------------------------------------

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

// DrawBoard renders the grid, pawns, the current selection (if any), and its
// legal destinations (plain moves vs. captures marked distinctly).
func DrawBoard(l *Layout, a *app) {
	s := a.gs
	// Grid frame + lines.
	ink.DrawRect(l.BoardArea, ink.Black)
	ink.DrawRect(pad(l.BoardArea, 1), ink.Black)
	for i := 1; i < game.Cols; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.BoardArea.Min.Y), image.Pt(px, l.BoardArea.Max.Y), ink.Black)
	}
	for i := 1; i < game.Rows; i++ {
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, py), image.Pt(l.BoardArea.Max.X, py), ink.Black)
	}

	// Legal destination hints for a selected pawn: a small square for a
	// quiet advance, a small ring for a capture (distinct pattern, since
	// e-ink has no color to tell them apart with).
	if a.hasSelection {
		for _, m := range s.Board.MovesFrom(a.selected, s.Turn) {
			cell := l.CellToScreen(m.To.X, m.To.Y)
			cx, cy := (cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2
			r := l.CellSize / 8
			if r < 3 {
				r = 3
			}
			if m.Capture {
				ink.DrawLine(image.Pt(cx-r, cy), image.Pt(cx+r, cy), ink.Black)
				ink.DrawLine(image.Pt(cx, cy-r), image.Pt(cx, cy+r), ink.Black)
				ink.DrawRect(image.Rect(cx-r, cy-r, cx+r, cy+r), ink.Black)
			} else {
				ink.FillArea(image.Rect(cx-r/2, cy-r/2, cx+r/2, cy+r/2), ink.LightGray)
			}
		}
	}

	// Pawns. Black = filled disc; White = hollow (ring) disc.
	for y := 0; y < game.Rows; y++ {
		for x := 0; x < game.Cols; x++ {
			switch s.Board.At(x, y) {
			case game.Black:
				drawMan(l.CellToScreen(x, y), true)
			case game.White:
				drawMan(l.CellToScreen(x, y), false)
			}
		}
	}

	// Selection highlight: a bold border around the selected pawn's square.
	if a.hasSelection {
		r := l.CellToScreen(a.selected.X, a.selected.Y)
		ink.DrawRect(pad(r, 2), ink.Black)
		ink.DrawRect(pad(r, 3), ink.Black)
	}

	// Briefly mark the destination of the most recent move, if it captured
	// a pawn (it's already gone; the marker vanishes on its own once the
	// next move replaces LastCaptured, so there is no separate timer).
	if s.LastCaptured {
		drawCaptureMark(l.CellToScreen(s.LastMove.To.X, s.LastMove.To.Y))
	}
}

// drawMan draws a pawn inside a cell. black=true renders a solid disc; false
// renders a hollow (white) disc with a black outline so both read on e-ink.
func drawMan(cell image.Rectangle, black bool) {
	r := pad(cell, cell.Dx()/8)
	fillDisc(r, ink.Black)
	if !black {
		inner := pad(r, r.Dx()/8+2)
		fillDisc(inner, ink.White)
	}
}

// drawCaptureMark overlays a diagonal cross on a cell to briefly flag it as
// just having captured a pawn there — e-ink has no real animation, so a
// single-frame distinct marker stands in for one.
func drawCaptureMark(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/4)
	ink.DrawLine(image.Pt(r.Min.X, r.Min.Y), image.Pt(r.Max.X, r.Max.Y), ink.DarkGray)
	ink.DrawLine(image.Pt(r.Max.X, r.Min.Y), image.Pt(r.Min.X, r.Max.Y), ink.DarkGray)
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
	opponent game.Opponent
	aiDepth  int
	label    string
}

type menuRow struct {
	rect   image.Rectangle
	choice menuChoice
}

// opponentChoices lists the opponent options offered on the start screen.
var opponentChoices = []menuChoice{
	{game.OpponentHotseat, 0, "2 spelare (hot-seat)"},
	{game.OpponentAI, game.DepthEasy, "Mot dator – Lätt"},
	{game.OpponentAI, game.DepthMedium, "Mot dator – Medel"},
	{game.OpponentAI, game.DepthHard, "Mot dator – Svår"},
}

type Menu struct {
	rows     []menuRow
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{} }

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	H := usableH

	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Genombrott")
	ink.DrawString(image.Pt((screen.X-tw)/2, 56), "Genombrott")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 30, true)
	sub.SetActive(ink.Black)
	subT := "Välj motståndare"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 140), subT)
	sub.Close()

	margin := 60
	rowW := screen.X - 2*margin

	// Bottom-anchored "Regler" button, stacked up from H-margin.
	rbW := rowW / 2
	rbH := 100
	rb := image.Rect((screen.X-rbW)/2, H-margin-rbH, (screen.X+rbW)/2, H-margin)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	f.Menu.SetActive(ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	// Opponent rows fill the space between the subtitle and the Regler
	// button.
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
	for _, c := range opponentChoices {
		r := image.Rect(margin, y, margin+rowW, y+rowH-20)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawLeftString(r, c.label, 38)
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

// rulesParagraphs is the rules text for Genombrott (Breakthrough), one entry
// per paragraph.
var rulesParagraphs = []string{
	"Mål: få en egen bricka till motståndarens bortre kant, eller ta alla motståndarens brickor, eller låt motståndaren stå utan lagligt drag.",
	"Brädet är 8x6. Varje spelare fyller sina två närmsta rader med brickor. Svart står längst ner och drar mot rad 0 (överst); Vit står längst upp och drar mot understa raden. Svart börjar.",
	"En bricka flyttar exakt ett steg rakt framåt till en TOM ruta — det draget kan ALDRIG ta en fiendebricka.",
	"En bricka kan istället flytta exakt ett steg diagonalt framåt, men bara till en ruta med en FIENDEBRICKA, och det tar då alltid bort den — ett diagonalt drag till en tom ruta är aldrig tillåtet.",
	"Observera skillnaden mot schack: här är det tvärtom — rakt fram tar aldrig, diagonalt tar alltid. Inga dubbelsteg, ingen \"en passant\", ingen befordran.",
	"Segervillkor 1: en av dina brickor når motståndarens bortre kant (för Svart: rad 0; för Vit: understa raden).",
	"Segervillkor 2: motståndaren har inga brickor kvar.",
	"Segervillkor 3: motståndaren har inget lagligt drag när det är dennes tur.",
	"Tryck på en egen bricka för att välja den — lagliga drag markeras: en liten fylld ruta för ett vanligt steg, en liten ring för ett drag som tar en fiendebricka. Tryck sedan på en av markeringarna för att utföra draget. Tryck på samma bricka igen för att avmarkera.",
	"Breakthrough är ett modernt, allmänt känt abstrakt brädspel utan tärning eller dold information.",
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

	body := ink.OpenFont(ink.DefaultFont, 30, true)
	body.SetActive(ink.Black)
	bodyMargin := 50
	maxW := screen.X - 2*bodyMargin
	y := 150
	lineH := 40
	paraGap := 18
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
// opening screen.
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

// drawSplashMotif draws the game's core idea: a Black pawn stepping
// diagonally forward past a White pawn, capturing it — a small "×" marks
// the capture, and a straight arrow beside it shows the (non-capturing)
// straight advance for contrast.
func drawSplashMotif(box image.Rectangle) {
	cx := (box.Min.X + box.Max.X) / 2
	cy := (box.Min.Y + box.Max.Y) / 2
	r := box.Dx() / 10

	// The mover starts lower-left, steps diagonally up-right onto the
	// White pawn's square.
	from := image.Pt(cx-r*3, cy+r*2)
	to := image.Pt(cx-r, cy)
	whitePos := to

	fillDisc(image.Rect(from.X-r, from.Y-r, from.X+r, from.Y+r), ink.Black)
	// The White pawn being captured, drawn as a ring so it still reads
	// once the capture mark is drawn over it.
	fillDisc(image.Rect(whitePos.X-r, whitePos.Y-r, whitePos.X+r, whitePos.Y+r), ink.Black)
	fillDisc(image.Rect(whitePos.X-r+r/4+2, whitePos.Y-r+r/4+2, whitePos.X+r-r/4-2, whitePos.Y+r-r/4-2), ink.White)

	// Diagonal capture arrow from the mover to the White pawn's square.
	drawArrow(from, to)

	// Small capture "×" just past the captured pawn.
	markC := image.Pt(to.X+r*2, to.Y-r*2)
	mr := r * 2 / 3
	ink.DrawLine(image.Pt(markC.X-mr, markC.Y-mr), image.Pt(markC.X+mr, markC.Y+mr), ink.Black)
	ink.DrawLine(image.Pt(markC.X+mr, markC.Y-mr), image.Pt(markC.X-mr, markC.Y+mr), ink.Black)

	// A second Black pawn further right, with a plain straight arrow ahead
	// of it onto an empty square — contrasting the two move types.
	from2 := image.Pt(cx+r*3, cy+r*2)
	to2 := image.Pt(cx+r*3, cy)
	fillDisc(image.Rect(from2.X-r, from2.Y-r, from2.X+r, from2.Y+r), ink.Black)
	drawArrow(from2, to2)
}

// drawArrow draws a straight line from `from` to `to` with a small V-shaped
// arrowhead at `to`.
func drawArrow(from, to image.Point) {
	ink.DrawLine(from, to, ink.Black)
	dx, dy := to.X-from.X, to.Y-from.Y
	length := isqrt(dx*dx + dy*dy)
	if length == 0 {
		return
	}
	// Unit direction scaled to a small head size.
	head := length / 4
	if head < 8 {
		head = 8
	}
	ux, uy := dx*head/length, dy*head/length
	// Perpendicular offset for the two head tails.
	px, py := -uy/2, ux/2
	tailA := image.Pt(to.X-ux+px, to.Y-uy+py)
	tailB := image.Pt(to.X-ux-px, to.Y-uy-py)
	ink.DrawLine(to, tailA, ink.Black)
	ink.DrawLine(to, tailB, ink.Black)
}
