package main

import (
	"image"
	"image/color"
	"math"

	ink "github.com/dennwc/inkview"

	"ataxx/game"
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
	boardMargin     = 16
	topMargin       = 40
)

// Layout maps between screen pixels and the 7x7 board.
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

// DrawBoard renders the grid, men, the current selection (if any) with its
// clone (near ring) and jump (far ring) destinations drawn distinctly, and a
// brief marker over men flipped by the last move.
func DrawBoard(l *Layout, a *app) {
	s := a.gs
	// Grid frame + lines.
	ink.DrawRect(l.BoardArea, ink.Black)
	ink.DrawRect(pad(l.BoardArea, 1), ink.Black)
	for i := 1; i < game.Size; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.BoardArea.Min.Y), image.Pt(px, l.BoardArea.Max.Y), ink.Black)
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, py), image.Pt(l.BoardArea.Max.X, py), ink.Black)
	}

	// Destination hints for a selected man: clone cells (near ring, distance
	// 1) as a small FILLED square, jump cells (far ring, distance 2) as a
	// distinct HOLLOW square — both are squares (never circles, so they're
	// never mistaken for a man) and filled-vs-hollow tells clone from jump.
	if a.hasSelection {
		for _, to := range s.Board.CloneDestinations(a.selected) {
			drawCloneHint(l.CellToScreen(to.X, to.Y))
		}
		for _, to := range s.Board.JumpDestinations(a.selected) {
			drawJumpHint(l.CellToScreen(to.X, to.Y))
		}
	}

	// Men. Black = filled disc; White = hollow (ring) disc.
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			switch s.Board.At(x, y) {
			case game.Black:
				drawMan(l.CellToScreen(x, y), true)
			case game.White:
				drawMan(l.CellToScreen(x, y), false)
			}
		}
	}

	// Selection highlight: a bold border around the selected man's square.
	if a.hasSelection {
		r := l.CellToScreen(a.selected.X, a.selected.Y)
		ink.DrawRect(pad(r, 2), ink.Black)
		ink.DrawRect(pad(r, 3), ink.Black)
	}

	// Briefly mark men flipped by the most recent move (a small corner flag,
	// so the disc underneath is still visible) — it vanishes on its own once
	// the next move replaces LastFlipped, so there's no separate timer.
	for _, p := range s.LastFlipped {
		drawFlipMark(l.CellToScreen(p.X, p.Y))
	}
}

// drawCloneHint marks a clone (distance-1) destination: a small filled
// dark-gray square, centered in the cell.
func drawCloneHint(cell image.Rectangle) {
	r := cell.Dx() / 6
	if r < 3 {
		r = 3
	}
	cx, cy := (cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2
	ink.FillArea(image.Rect(cx-r, cy-r, cx+r, cy+r), ink.DarkGray)
}

// drawJumpHint marks a jump (distance-2) destination: a distinct HOLLOW
// square outline, larger than the clone hint, so the two destination kinds
// never look alike.
func drawJumpHint(cell image.Rectangle) {
	r := cell.Dx() / 4
	if r < 5 {
		r = 5
	}
	cx, cy := (cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2
	rect := image.Rect(cx-r, cy-r, cx+r, cy+r)
	ink.DrawRect(rect, ink.Black)
	ink.DrawRect(pad(rect, 1), ink.Black)
}

// drawMan draws a man inside a cell. black=true renders a solid disc; false
// renders a hollow (white) disc with a black outline so both read on e-ink.
func drawMan(cell image.Rectangle, black bool) {
	r := pad(cell, cell.Dx()/8)
	fillDisc(r, ink.Black)
	if !black {
		inner := pad(r, r.Dx()/8+2)
		fillDisc(inner, ink.White)
	}
}

// drawFlipMark overlays a small filled triangle in the top-left corner of a
// cell to flag that the man there just flipped color — the disc underneath
// is still fully visible everywhere else in the cell.
func drawFlipMark(cell image.Rectangle) {
	s := cell.Dx() / 4
	if s < 6 {
		s = 6
	}
	x0, y0 := cell.Min.X+2, cell.Min.Y+2
	for i := 0; i < s; i++ {
		ink.DrawLine(image.Pt(x0, y0+i), image.Pt(x0+s-i, y0+i), ink.DarkGray)
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
	tw := ink.StringWidth("Ataxx")
	ink.DrawString(image.Pt((screen.X-tw)/2, 56), "Ataxx")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 30, true)
	sub.SetActive(ink.Black)
	subT := "Välj läge"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 150), subT)
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
	top := 230
	bottom := rb.Min.Y - 30
	n := len(opponentChoices)
	avail := bottom - top
	if avail < rowH*n {
		rowH = avail / n
	}
	y := top

	m.rows = m.rows[:0]
	for _, c := range opponentChoices {
		r := image.Rect(margin, y, margin+rowW, y+rowH-18)
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

// rulesParagraphs is the rules text for Ataxx, one entry per paragraph.
var rulesParagraphs = []string{
	"Mål: ha flest brickor på brädet när spelet tar slut.",
	"Brädet är 7x7. Svart börjar med en bricka i övre vänstra och nedre högra hörnet; Vit har en bricka i de två andra hörnen.",
	"En brickas drag är antingen en KLONING eller ett HOPP. Kloning: ett steg i valfri av de 8 riktningarna (vågrätt, lodrätt eller diagonalt) — din ursprungsbricka blir kvar och en ny bricka av din färg dyker upp på den tomma målrutan.",
	"Hopp: två steg bort i valfri riktning (även \"sneda\" hopp som två steg åt sidan och ett steg upp räknas) — ursprungsrutan töms och bricka landar på målrutan.",
	"Oavsett kloning eller hopp: varje fiendebricka som ligger direkt intill målrutan, i alla 8 riktningar, byter färg till din — inte bara de fyra rakt intill.",
	"Spelet tar slut när brädet är fullt, eller när spelaren i tur inte har något lagligt drag kvar (eller inga brickor kvar). Den som då har flest brickor vinner — oavgjort är möjligt om ingen har flest. Att inte kunna dra förlorar INTE automatiskt; det är antalet brickor som avgör.",
	"Tryck på en egen bricka för att välja den. Kloningsrutor (ett steg bort) markeras med en fylld liten fyrkant; hoppsrutor (två steg bort) markeras med en ihålig fyrkant. Tryck på en av de markerade rutorna för att flytta dit. Tryck på samma bricka igen för att avmarkera.",
	"Ataxx (även känt som \"Smitta\") är ett klassiskt strategispel för två spelare, ursprungligen från arkadhallarnas 1990-tal.",
}

// DrawRules renders the scrolling rules text with a back button and returns the
// back button rect.
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

// --- Splash screen ---------------------------------------------------------

// motifFunc draws the game's line-art centered in the given box.
type motifFunc func(box image.Rectangle)

// DrawSplash renders the start screen: the game title, a large line-art motif,
// and a "tap to start" hint — echoing the built-in chess app's opening screen.
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

// discRect returns the bounding box of a disc of radius r centered at c.
func discRect(c image.Point, r int) image.Rectangle {
	return image.Rect(c.X-r, c.Y-r, c.X+r, c.Y+r)
}

// drawArrow draws a straight line from "from" to "to" with a small V
// arrowhead at "to", pointing in the direction of travel.
func drawArrow(from, to image.Point, headSize int) {
	ink.DrawLine(from, to, ink.Black)
	dx := float64(to.X - from.X)
	dy := float64(to.Y - from.Y)
	length := math.Hypot(dx, dy)
	if length == 0 {
		return
	}
	ux, uy := dx/length, dy/length
	px, py := -uy, ux // perpendicular unit vector
	hs := float64(headSize)
	leftX := float64(to.X) - ux*hs + px*hs*0.6
	leftY := float64(to.Y) - uy*hs + py*hs*0.6
	rightX := float64(to.X) - ux*hs - px*hs*0.6
	rightY := float64(to.Y) - uy*hs - py*hs*0.6
	ink.DrawLine(to, image.Pt(int(leftX), int(leftY)), ink.Black)
	ink.DrawLine(to, image.Pt(int(rightX), int(rightY)), ink.Black)
}

// drawSplashMotif draws Ataxx's core idea: a disc cloning into an adjacent
// cell (solid arrow from the source disc to the new clone disc), with a
// small flip arrow from the clone onto a neighboring enemy disc (drawn as a
// ring, about to flip to solid).
func drawSplashMotif(box image.Rectangle) {
	cx := (box.Min.X + box.Max.X) / 2
	cy := (box.Min.Y + box.Max.Y) / 2
	r := box.Dx() / 10
	spacing := r * 4

	source := image.Pt(cx-spacing, cy)
	clone := image.Pt(cx+spacing/4, cy)
	enemy := image.Pt(cx+spacing, cy+spacing*3/4)

	// Source and clone: both solid Black discs (the mover's color).
	fillDisc(discRect(source, r), ink.Black)
	fillDisc(discRect(clone, r), ink.Black)
	// Enemy: a ring (White) disc, about to flip.
	fillDisc(discRect(enemy, r), ink.Black)
	fillDisc(pad(discRect(enemy, r), r/4+3), ink.White)

	// Arrow 1: source -> clone (the cloning move).
	drawArrow(image.Pt(source.X+r+6, source.Y), image.Pt(clone.X-r-6, clone.Y), r/2)
	// Arrow 2: small flip arrow from the clone toward the enemy disc.
	fromX := clone.X + int(float64(r)*0.7)
	fromY := clone.Y + int(float64(r)*0.7)
	toX := enemy.X - int(float64(r)*0.7)
	toY := enemy.Y - int(float64(r)*0.7)
	drawArrow(image.Pt(fromX, fromY), image.Pt(toX, toY), r/3)
}
