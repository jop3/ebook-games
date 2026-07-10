package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"hex/game"
)

type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Edge   *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 36, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 40, true),
		Edge:   ink.OpenFont(ink.DefaultFontBold, 30, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Edge} {
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
	statusBarHeight = 96
	buttonBarHeight = 140
	boardMargin     = 40

	// usableH is the real drawable height on the PB634. ink.ScreenSize().Y
	// reports 1448, but content below ~1360 wraps to the top of the screen on
	// the actual device, so all vertical layout must target this instead.
	usableH = 1340
)

// Layout maps between screen pixels and hex cells. Cells are pointy-top hexagons
// in a rhombus: each row is shifted right by half a column step.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	ButtonBar image.Rectangle

	N      int
	Origin image.Point // center of cell (0,0)
	DX     int         // horizontal center spacing between columns
	DY     int         // vertical center spacing between rows
	Radius int         // hex "radius" for drawing
}

func NewLayout(screen image.Point, n int) Layout {
	H := usableH
	l := Layout{Screen: image.Rectangle{Max: image.Pt(screen.X, H)}, N: n}
	l.StatusBar = image.Rect(0, 0, screen.X, statusBarHeight)
	l.ButtonBar = image.Rect(0, H-buttonBarHeight, screen.X, H)

	avail := image.Rect(boardMargin, statusBarHeight+boardMargin,
		screen.X-boardMargin, H-buttonBarHeight-boardMargin)

	// Rhombus total extent in "column steps": width = (n + n/2) columns,
	// height = n rows. Pick DX so the widest row fits, DY so all rows fit.
	// Pointy-top hex: DY = DX * 0.75 (rows overlap by a quarter) approx; we use
	// DY = DX*7/8 for a comfortable pointy look with our neighbor topology.
	// Solve DX from width and height constraints.
	spanCols := float64(n) + float64(n-1)/2.0 // centers span, in DX units
	dxW := float64(avail.Dx()) / (spanCols + 1)
	// Pointy-top hexagons tessellate when the row pitch is 3/4 of the full
	// height. With DX = 2*radius the height is 2*radius = DX, so DY = 0.75*DX.
	dyRatio := 0.75
	dxH := float64(avail.Dy()) / (float64(n-1)*dyRatio + 1)
	dx := dxW
	if dxH < dx {
		dx = dxH
	}
	l.DX = int(dx)
	l.DY = int(dx * dyRatio)
	l.Radius = l.DX / 2

	// Total drawn extent, to center in avail.
	totalW := (l.N-1)*l.DX + (l.N-1)*l.DX/2 + l.DX
	totalH := (l.N-1)*l.DY + l.DX
	ox := avail.Min.X + (avail.Dx()-totalW)/2 + l.Radius
	oy := avail.Min.Y + (avail.Dy()-totalH)/2 + l.Radius
	l.Origin = image.Pt(ox, oy)
	return l
}

// Center returns the pixel center of cell (x,y).
func (l *Layout) Center(x, y int) image.Point {
	cx := l.Origin.X + x*l.DX + y*l.DX/2
	cy := l.Origin.Y + y*l.DY
	return image.Pt(cx, cy)
}

// CellAt returns the cell whose center is nearest p, if within radius.
func (l *Layout) CellAt(p image.Point) (x, y int, ok bool) {
	bestD := l.Radius*l.Radius + l.Radius*l.Radius
	for yy := 0; yy < l.N; yy++ {
		for xx := 0; xx < l.N; xx++ {
			c := l.Center(xx, yy)
			dx := p.X - c.X
			dy := p.Y - c.Y
			d := dx*dx + dy*dy
			if d < bestD {
				bestD = d
				x, y, ok = xx, yy, true
			}
		}
	}
	return
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
	// Nudge the text down so it clears the top safe-area margin (y >= 40);
	// centering in the full StatusBar rect alone can land a couple px short.
	safe := image.Rect(l.StatusBar.Min.X, 40, l.StatusBar.Max.X, l.StatusBar.Max.Y)
	drawCenteredString(safe, text, 36)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawBoard renders the rhombus of hexagons, edge ownership bars, and stones.
func DrawBoard(l *Layout, s *game.GameState, f *Fonts) {
	// Edge indicator bars: top/bottom are Black's, left/right are White's.
	drawEdgeBars(l)

	for y := 0; y < l.N; y++ {
		for x := 0; x < l.N; x++ {
			c := l.Center(x, y)
			drawHexOutline(c, l.Radius)
			switch s.Board.At(x, y) {
			case game.Black:
				drawStone(c, l.Radius, true)
			case game.White:
				drawStone(c, l.Radius, false)
			}
		}
	}
}

// drawEdgeBars draws thick guides showing which edges each color must connect.
func drawEdgeBars(l *Layout) {
	// Black edges: along the top row centers and bottom row centers.
	top0 := l.Center(0, 0)
	topN := l.Center(l.N-1, 0)
	bot0 := l.Center(0, l.N-1)
	botN := l.Center(l.N-1, l.N-1)
	off := l.Radius + 8
	// top black bar
	thickLine(image.Pt(top0.X-off, top0.Y-off), image.Pt(topN.X+off, topN.Y-off), ink.Black, 4)
	thickLine(image.Pt(bot0.X-off, bot0.Y+off), image.Pt(botN.X+off, botN.Y+off), ink.Black, 4)
	// White edges: left and right sides (dashed-ish via gray + thinner double).
	left0 := l.Center(0, 0)
	leftN := l.Center(0, l.N-1)
	right0 := l.Center(l.N-1, 0)
	rightN := l.Center(l.N-1, l.N-1)
	thickLine(image.Pt(left0.X-off, left0.Y-off), image.Pt(leftN.X-off, leftN.Y+off), ink.DarkGray, 4)
	thickLine(image.Pt(right0.X+off, right0.Y-off), image.Pt(rightN.X+off, rightN.Y+off), ink.DarkGray, 4)
}

func thickLine(a, b image.Point, col color.Color, w int) {
	for o := 0; o < w; o++ {
		ink.DrawLine(image.Pt(a.X, a.Y+o), image.Pt(b.X, b.Y+o), col)
		ink.DrawLine(image.Pt(a.X+o, a.Y), image.Pt(b.X+o, b.Y), col)
	}
}

// drawHexOutline draws a pointy-top hexagon outline centered at c.
func drawHexOutline(c image.Point, r int) {
	pts := hexCorners(c, r)
	for i := 0; i < 6; i++ {
		ink.DrawLine(pts[i], pts[(i+1)%6], ink.Black)
	}
}

// hexCorners returns the 6 corners of a pointy-top hexagon.
func hexCorners(c image.Point, r int) [6]image.Point {
	// pointy-top: corners at angles 90,150,210,270,330,30 degrees.
	// Precomputed offsets (approx) scaled by r. Using integer approximations of
	// cos/sin at those angles.
	// angle 90:  (0, -r)
	// angle 30:  (0.866r, -0.5r)
	// angle 330: (0.866r, 0.5r)
	// angle 270: (0, r)
	// angle 210: (-0.866r, 0.5r)
	// angle 150: (-0.866r, -0.5r)
	hx := r * 866 / 1000
	hy := r / 2
	return [6]image.Point{
		{c.X, c.Y - r},
		{c.X + hx, c.Y - hy},
		{c.X + hx, c.Y + hy},
		{c.X, c.Y + r},
		{c.X - hx, c.Y + hy},
		{c.X - hx, c.Y - hy},
	}
}

// drawStone draws a stone inside a hex. black=true is a solid disc; false is a
// hollow disc with a black ring so both read on e-ink.
func drawStone(c image.Point, r int, black bool) {
	rr := r * 3 / 4
	fillCircle(c, rr, ink.Black)
	if !black {
		fillCircle(c, rr*2/3, ink.White)
	}
}

func fillCircle(c image.Point, rad int, col color.Color) {
	rr := rad * rad
	for dy := -rad; dy <= rad; dy++ {
		hw := isqrt(rr - dy*dy)
		ink.DrawLine(image.Pt(c.X-hw, c.Y+dy), image.Pt(c.X+hw, c.Y+dy), col)
	}
}

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
	topGap := gap
	bottomGap := 24  // matches ink.SafeMarginBottom so buttons never enter the margin
	sideMargin := 24 // keep buttons clear of the left/right safe-area margin
	totalGap := gap*(n-1) + 2*sideMargin
	bw := (l.ButtonBar.Dx() - totalGap) / n
	bh := l.ButtonBar.Dy() - topGap - bottomGap
	buttons := make([]Button, n)
	for i, label := range labels {
		x0 := l.ButtonBar.Min.X + sideMargin + i*(bw+gap)
		y0 := l.ButtonBar.Min.Y + topGap
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
	n     int
	mode  game.Mode
	label string
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

// RulesButton returns the tappable "Regler" rect (set during Draw).
func (m *Menu) RulesButton() image.Rectangle { return m.rulesBtn }

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()

	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Hex")
	ink.DrawString(image.Pt((screen.X-tw)/2, 60), "Hex")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 32, true)
	sub.SetActive(ink.Black)
	subT := "Välj spelläge"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 150), subT)
	sub.Close()

	f.Menu.SetActive(ink.Black)
	y := 240
	rowH := 110
	margin := 60
	rowW := screen.X - 2*margin

	m.rows = m.rows[:0]
	for _, c := range menuChoices {
		r := image.Rect(margin, y, margin+rowW, y+rowH-16)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawLeftString(r, c.label, 40)
		m.rows = append(m.rows, menuRow{rect: r, choice: c})
		y += rowH
	}

	// "Regler" button opens the full rules screen.
	rbW := rowW / 2
	rb := image.Rect((screen.X-rbW)/2, y+20, (screen.X+rbW)/2, y+20+96)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb
}

// menuChoices lists board sizes and modes.
var menuChoices = []menuChoice{
	{7, game.ModeHotseat, "7×7 – 2 spelare"},
	{7, game.ModeAI, "7×7 – mot dator"},
	{9, game.ModeHotseat, "9×9 – 2 spelare"},
	{9, game.ModeAI, "9×9 – mot dator"},
	{11, game.ModeHotseat, "11×11 – 2 spelare"},
}

func (m *Menu) HandleTouch(p image.Point) (menuChoice, bool) {
	for i := range m.rows {
		if p.In(m.rows[i].rect) {
			return m.rows[i].choice, true
		}
	}
	return menuChoice{}, false
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
	"Mål: bygg en obruten kedja av dina stenar mellan dina två sidor av brädet.",
	"Svart ska koppla ihop övre och nedre kanten (de heldragna kanterna). Vit ska koppla ihop vänster och höger kant (de grå kanterna).",
	"Spelarna turas om att lägga en sten på valfri tom ruta. Stenar flyttas aldrig och tas aldrig bort.",
	"Svart börjar. Två celler hänger ihop om de gränsar till varandra i sexhörnsmönstret.",
	"I Hex kan partiet aldrig sluta oavgjort — exakt en spelare lyckas alltid koppla sina sidor.",
	"Tips: en sten spärrar både för motståndaren och bygger din egen väg. Tänk på båda samtidigt.",
}

// DrawRules renders the rules text with a back button and returns its rect.
func DrawRules(screen image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
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
	r := image.Rect((screen.X-bw)/2, usableH-bh-24, (screen.X+bw)/2, usableH-24)
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

// drawSplashMotif draws a seven-hex honeycomb flower with a couple of stones —
// the game's board cells, in simple line graphics.
func drawSplashMotif(box image.Rectangle) {
	cx := (box.Min.X + box.Max.X) / 2
	cy := (box.Min.Y + box.Max.Y) / 2
	r := box.Dx() / 6
	dx := r * 866 / 1000 * 2
	dy := r * 3 / 2

	// A center hex plus the ring of six neighbors around it (a honeycomb flower).
	centers := []image.Point{{cx, cy}}
	ring := [6][2]int{{0, -1}, {1, -1}, {1, 0}, {0, 1}, {-1, 1}, {-1, 0}}
	for _, o := range ring {
		hx := cx + o[0]*dx + o[1]*(dx/2)
		hy := cy + o[1]*dy
		centers = append(centers, image.Pt(hx, hy))
	}

	for _, c := range centers {
		drawHexOutline(c, r)
	}
	// A black stone in the center and one white on a corner cell.
	drawStone(centers[0], r, true)
	drawStone(centers[2], r, false)
}
