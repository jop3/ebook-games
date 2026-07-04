package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"goban/game"
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
		Button: ink.OpenFont(ink.DefaultFontBold, 36, true),
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

// ftoa1 formats v with at most one decimal digit (scores are always a whole
// number or a whole number plus .5, since komi moves in 0.5 steps).
func ftoa1(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	tenths := int(v*10 + 0.5)
	whole, frac := tenths/10, tenths%10
	s := itoa(whole)
	if frac != 0 {
		s += "." + itoa(frac)
	}
	if neg {
		s = "-" + s
	}
	return s
}

// --- Layout ------------------------------------------------------------------

const (
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	statusBarHeight = 96
	buttonBarHeight = 140
	boardMargin     = 24
	topMargin       = 40
)

// Layout maps between screen pixels and Go board intersections. Unlike the
// cell-filling boards in othello/hasami, Go stones sit ON grid intersections,
// so the board is laid out as (size-1) evenly spaced steps between the first
// and last line in each direction, not as size cells.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	ButtonBar image.Rectangle

	BoardArea  image.Rectangle
	GridOrigin image.Point
	Step       int
	Size       int
}

func NewLayout(screen image.Point, size int) Layout {
	H := usableH
	l := Layout{Screen: image.Rect(0, 0, screen.X, H), Size: size}
	l.StatusBar = image.Rect(0, topMargin, screen.X, topMargin+statusBarHeight)
	l.ButtonBar = image.Rect(0, H-topMargin-buttonBarHeight, screen.X, H-topMargin)

	avail := image.Rect(boardMargin, l.StatusBar.Max.Y+boardMargin,
		screen.X-boardMargin, l.ButtonBar.Min.Y-boardMargin)
	side := avail.Dx()
	if avail.Dy() < side {
		side = avail.Dy()
	}
	steps := size - 1
	if steps < 1 {
		steps = 1
	}
	// Stones are drawn ON the outermost grid lines, with radius step*9/20
	// (see drawStone), so the edge stones overhang the grid-line span itself.
	// Size the step so the grid span PLUS that overhang (steps+0.9 step-widths
	// total, computed as integer tenths to avoid float) fits inside avail —
	// otherwise the last row/column of stones gets clipped by the button bar's
	// background fill on small boards (9x9's large step) even though 19x19
	// (small step) happens to have enough slack to look fine.
	step := side * 10 / (steps*10 + 9)
	if step < 1 {
		step = 1
	}
	l.Step = step
	span := step * steps
	l.GridOrigin = image.Pt(
		avail.Min.X+(avail.Dx()-span)/2,
		avail.Min.Y+(avail.Dy()-span)/2,
	)
	l.BoardArea = image.Rect(l.GridOrigin.X, l.GridOrigin.Y,
		l.GridOrigin.X+span, l.GridOrigin.Y+span)
	return l
}

// PointToScreen returns the pixel center of board intersection p.
func (l *Layout) PointToScreen(p image.Point) image.Point {
	return image.Pt(l.GridOrigin.X+p.X*l.Step, l.GridOrigin.Y+p.Y*l.Step)
}

// PointRect returns a square tap-target rectangle centered on intersection p
// (used both for hit-testing and by tests driving taps through the harness).
func (l *Layout) PointRect(p image.Point) image.Rectangle {
	c := l.PointToScreen(p)
	r := l.Step / 2
	if r < 1 {
		r = 1
	}
	return image.Rect(c.X-r, c.Y-r, c.X+r, c.Y+r)
}

func divRound(a, b int) int {
	if b == 0 {
		return 0
	}
	if a >= 0 {
		return (a + b/2) / b
	}
	return -((-a + b/2) / b)
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ScreenToPoint maps a tap to the nearest board intersection, rejecting taps
// too far from any intersection (outside the board, or in the gutter between
// lines) so a miss doesn't get silently snapped to a distant point.
func (l *Layout) ScreenToPoint(p image.Point) (image.Point, bool) {
	if l.Step == 0 {
		return image.Point{}, false
	}
	relX := p.X - l.GridOrigin.X
	relY := p.Y - l.GridOrigin.Y
	x := divRound(relX, l.Step)
	y := divRound(relY, l.Step)
	if x < 0 || x >= l.Size || y < 0 || y >= l.Size {
		return image.Point{}, false
	}
	tol := l.Step * 3 / 5
	if absInt(relX-x*l.Step) > tol || absInt(relY-y*l.Step) > tol {
		return image.Point{}, false
	}
	return image.Pt(x, y), true
}

// --- Rendering ---------------------------------------------------------------

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

// DrawBoard renders the grid, hoshi points, stones, the last-move marker, a
// brief marker over just-captured points, and (during the mark-dead phase) a
// cross over every stone currently marked dead.
func DrawBoard(l *Layout, s *game.GameState, f *Fonts) {
	size := s.Board.Size()

	for i := 0; i < size; i++ {
		x := l.GridOrigin.X + i*l.Step
		ink.DrawLine(image.Pt(x, l.BoardArea.Min.Y), image.Pt(x, l.BoardArea.Max.Y), ink.Black)
		y := l.GridOrigin.Y + i*l.Step
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, y), image.Pt(l.BoardArea.Max.X, y), ink.Black)
	}

	hr := l.Step / 10
	if hr < 3 {
		hr = 3
	}
	for _, hp := range game.HoshiPoints(size) {
		if s.Board.At(hp) != game.Empty {
			continue
		}
		c := l.PointToScreen(hp)
		fillDisc(image.Rect(c.X-hr, c.Y-hr, c.X+hr, c.Y+hr), ink.Black)
	}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			p := image.Pt(x, y)
			switch s.Board.At(p) {
			case game.Black:
				drawStone(l, p, true)
			case game.White:
				drawStone(l, p, false)
			}
		}
	}

	// Last-move marker: a small contrasting square on top of the stone.
	if s.HasLastMove {
		c := l.PointToScreen(s.LastMove)
		mr := l.Step / 7
		if mr < 3 {
			mr = 3
		}
		mcol := ink.White
		if s.Board.At(s.LastMove) == game.White {
			mcol = ink.Black
		}
		ink.FillArea(image.Rect(c.X-mr, c.Y-mr, c.X+mr, c.Y+mr), mcol)
	}

	// A light ring over points captured by the move just applied, so a big
	// capture reads clearly even though the stones are simply now absent.
	for _, p := range s.LastCaptured {
		c := l.PointToScreen(p)
		rr := l.Step * 3 / 10
		ringDisc(image.Rect(c.X-rr, c.Y-rr, c.X+rr, c.Y+rr), l.Step/16+1, ink.DarkGray)
	}

	// Cross out every stone marked dead, during the marking phase itself and
	// on the final score screen afterward (the score already counts these as
	// the surrounder's territory, so the board should keep showing that).
	if s.Phase == game.PhaseMarking || s.Phase == game.PhaseDone {
		for p := range s.Dead {
			c := l.PointToScreen(p)
			rr := l.Step * 2 / 5
			col := ink.White
			if s.Board.At(p) == game.Black {
				col = ink.White
			} else {
				col = ink.Black
			}
			drawCross(c, rr, l.Step/14+1, col)
		}
	}
}

// drawStone draws a stone at intersection p: black=true renders a solid
// disc; false renders a hollow (white) disc with a black outline so both
// read clearly on e-ink.
func drawStone(l *Layout, p image.Point, black bool) {
	c := l.PointToScreen(p)
	r := l.Step * 9 / 20
	if r < 4 {
		r = 4
	}
	rect := image.Rect(c.X-r, c.Y-r, c.X+r, c.Y+r)
	fillDisc(rect, ink.Black)
	if !black {
		inner := pad(rect, r/6+2)
		fillDisc(inner, ink.White)
	}
}

// drawCross draws an X centered at c, spanning +-r, in color col.
func drawCross(c image.Point, r, th int, col color.Color) {
	ink.DrawLine(image.Pt(c.X-r, c.Y-r), image.Pt(c.X+r, c.Y+r), col)
	ink.DrawLine(image.Pt(c.X-r, c.Y+r), image.Pt(c.X+r, c.Y-r), col)
	// Thicken by drawing a couple of near-parallel lines (no line-width API).
	for i := 1; i <= th; i++ {
		ink.DrawLine(image.Pt(c.X-r+i, c.Y-r), image.Pt(c.X+r, c.Y+r-i), col)
		ink.DrawLine(image.Pt(c.X-r, c.Y-r+i), image.Pt(c.X+r-i, c.Y+r), col)
		ink.DrawLine(image.Pt(c.X-r+i, c.Y+r), image.Pt(c.X+r, c.Y-r+i), col)
		ink.DrawLine(image.Pt(c.X-r, c.Y+r-i), image.Pt(c.X+r-i, c.Y-r), col)
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

// ringDisc draws a circular ring (outline band of thickness th) inside rect r.
func ringDisc(r image.Rectangle, th int, col color.Color) {
	cx := (r.Min.X + r.Max.X) / 2
	cy := (r.Min.Y + r.Max.Y) / 2
	rad := r.Dx() / 2
	if r.Dy()/2 < rad {
		rad = r.Dy() / 2
	}
	if th < 1 {
		th = 1
	}
	inner := rad - th
	if inner < 0 {
		inner = 0
	}
	rr, ir := rad*rad, inner*inner
	for dy := -rad; dy <= rad; dy++ {
		for dx := -rad; dx <= rad; dx++ {
			d2 := dx*dx + dy*dy
			if d2 <= rr && d2 >= ir {
				p := image.Pt(cx+dx, cy+dy)
				ink.DrawLine(p, p, col)
			}
		}
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
		drawCenteredString(r, label, 36)
		buttons[i] = Button{Rect: r, Label: label}
	}
	return buttons
}

// --- Menu ----------------------------------------------------------------

type menuChoice struct {
	opponent game.Opponent
	label    string
}

var opponentChoices = []menuChoice{
	{game.OpponentHotseat, "2 spelare"},
	{game.OpponentAI, "Mot dator (svag)"},
}

type menuRow struct {
	rect   image.Rectangle
	choice menuChoice
}

// Menu holds the persistent size/komi settings plus the tappable regions
// computed by the most recent Draw.
type Menu struct {
	size int
	komi float64

	sizeBtns  [3]image.Rectangle
	sizeVals  [3]int
	komiMinus image.Rectangle
	komiPlus  image.Rectangle
	rows      []menuRow
	rulesBtn  image.Rectangle
}

func NewMenu() *Menu {
	return &Menu{size: 9, komi: game.DefaultKomi}
}

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	H := usableH

	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Go")
	ink.DrawString(image.Pt((screen.X-tw)/2, 56), "Go")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 30, true)
	sub.SetActive(ink.Black)
	subT := "Välj storlek och motståndare"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 140), subT)
	sub.Close()

	margin := 60
	rowW := screen.X - 2*margin

	// Size toggle: three buttons, the selected one double-bordered.
	sizeY := 190
	sizeH := 84
	gap := 20
	sizeBtnW := (rowW - 2*gap) / 3
	sizes := [3]int{9, 13, 19}
	labels := [3]string{"9x9", "13x13", "19x19"}
	f.Menu.SetActive(ink.Black)
	for i := 0; i < 3; i++ {
		x0 := margin + i*(sizeBtnW+gap)
		r := image.Rect(x0, sizeY, x0+sizeBtnW, sizeY+sizeH)
		ink.DrawRect(r, ink.Black)
		if m.size == sizes[i] {
			ink.DrawRect(pad(r, 3), ink.Black)
			ink.DrawRect(pad(r, 4), ink.Black)
		}
		drawCenteredString(r, labels[i], 34)
		m.sizeBtns[i] = r
		m.sizeVals[i] = sizes[i]
	}

	// Komi stepper.
	komiY := sizeY + sizeH + 26
	komiH := 84
	stepBtnW := 110
	komiRect := image.Rect(margin, komiY, margin+rowW, komiY+komiH)
	ink.DrawRect(komiRect, ink.Black)
	minus := image.Rect(komiRect.Min.X, komiY, komiRect.Min.X+stepBtnW, komiY+komiH)
	plus := image.Rect(komiRect.Max.X-stepBtnW, komiY, komiRect.Max.X, komiY+komiH)
	ink.DrawRect(minus, ink.Black)
	ink.DrawRect(plus, ink.Black)
	drawCenteredString(minus, "-0.5", 34)
	drawCenteredString(plus, "+0.5", 34)
	drawCenteredString(komiRect, "Komi: "+ftoa1(m.komi), 34)
	m.komiMinus = minus
	m.komiPlus = plus

	// Bottom-anchored "Regler" button, stacked up from H-margin.
	rbW := rowW / 2
	rbH := 100
	rb := image.Rect((screen.X-rbW)/2, H-margin-rbH, (screen.X+rbW)/2, H-margin)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	// Opponent rows fill the space between the komi stepper and Regler.
	// "Mot dator" is offered only for 9x9 — the AI isn't shippable at 13/19.
	choices := opponentChoices
	if m.size != 9 {
		choices = opponentChoices[:1]
	}
	rowH := 116
	top := komiY + komiH + 30
	bottom := rb.Min.Y - 30
	n := len(choices)
	avail := bottom - top
	if avail < rowH*n {
		rowH = avail / n
	}
	y := top
	m.rows = m.rows[:0]
	for _, c := range choices {
		r := image.Rect(margin, y, margin+rowW, y+rowH-18)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawLeftString(r, c.label, 38)
		m.rows = append(m.rows, menuRow{rect: r, choice: c})
		y += rowH
	}
}

// TapSize handles a tap on the 9x9/13x13/19x19 toggle.
func (m *Menu) TapSize(p image.Point) bool {
	for i, r := range m.sizeBtns {
		if p.In(r) {
			m.size = m.sizeVals[i]
			if m.size != 9 {
				// Mot dator is 9x9-only; falling back to hotseat here would
				// silently change the user's choice, so instead we simply
				// stop offering that row on the next Draw (see Draw above).
				_ = m.size
			}
			return true
		}
	}
	return false
}

// TapKomi handles a tap on the -0.5/+0.5 komi stepper, clamped to [0, 9.5].
func (m *Menu) TapKomi(p image.Point) bool {
	if p.In(m.komiMinus) {
		m.komi -= 0.5
		if m.komi < 0 {
			m.komi = 0
		}
		return true
	}
	if p.In(m.komiPlus) {
		m.komi += 0.5
		if m.komi > 9.5 {
			m.komi = 9.5
		}
		return true
	}
	return false
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

// --- Rules screen --------------------------------------------------------

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

// rulesParagraphs is the rules text for Go, one entry per paragraph.
var rulesParagraphs = []string{
	"Mål: omringa störst område med dina stenar. Svart och Vit turas om att lägga en sten på en ledig skärningspunkt; Svart börjar.",
	"Fångst: en grupp stenar utan lediga friheter (tomma angränsande punkter) tas bort från brädet. Du får inte göra ett drag som fångar dina egna stenar utan att samtidigt fånga minst en motståndarsten (självmord är förbjudet).",
	"Ko: du får inte återskapa exakt samma brädposition som fanns innan motståndarens senaste drag. Regeln lyfts så fort du spelar ett drag någon annanstans.",
	"Passa: två pass i rad avslutar partiet. Då går ni igenom brädet och trycker på döda grupper för att markera dem som döda innan slutresultatet räknas.",
	"Poäng räknas area-vis (kinesisk räkning): din poäng = dina stenar på brädet + tomma punkter som enbart dina stenar omringar. En punkt som gränsar till båda färgerna räknas inte till någon. Vit får dessutom komi som kompensation för att Svart drar först.",
	"Storlek: 9x9, 13x13 eller 19x19. Mot dator (en svag inbyggd motståndare) finns bara på 9x9 — 13x13 och 19x19 spelas 2 spelare.",
}

func DrawRules(screen image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	ink.ClearScreen()
	H := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 56, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, 60), title)
	tf.Close()

	margin := 40
	bh := 110
	bw := screen.X / 2
	r := image.Rect((screen.X-bw)/2, H-margin-bh, (screen.X+bw)/2, H-margin)
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 1), ink.Black)
	f.Button.SetActive(ink.Black)
	drawCenteredString(r, "Tillbaka", 36)

	body := ink.OpenFont(ink.DefaultFont, 32, true)
	body.SetActive(ink.Black)
	bodyMargin := 60
	maxW := screen.X - 2*bodyMargin
	y := 180
	lineH := 44
	paraGap := 22
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

type motifFunc func(box image.Rectangle)

func DrawSplash(screen image.Point, f *Fonts, title string, motif motifFunc) {
	ink.ClearScreen()
	H := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 80, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, H/6), title)
	tf.Close()

	side := screen.X * 3 / 5
	box := image.Rect((screen.X-side)/2, (H-side)/2,
		(screen.X+side)/2, (H+side)/2)
	motif(box)

	hint := ink.OpenFont(ink.DefaultFont, 34, true)
	hint.SetActive(ink.DarkGray)
	ht := "Tryck för att börja"
	hw := ink.StringWidth(ht)
	ink.DrawString(image.Pt((screen.X-hw)/2, H*5/6), ht)
	hint.Close()
}

// drawSplashMotif draws a small grid corner with a black group in atari (one
// liberty) about to be captured by White — teaches the core idea (liberties,
// capture) at a glance, per the game's own rules.
func drawSplashMotif(box image.Rectangle) {
	n := 3 // a 3x3 intersection corner (2x2 cells)
	margin := box.Dx() / 6
	span := box.Dx() - 2*margin
	step := span / (n - 1)
	origin := image.Pt(box.Min.X+margin, box.Min.Y+margin)

	for i := 0; i < n; i++ {
		x := origin.X + i*step
		ink.DrawLine(image.Pt(x, origin.Y), image.Pt(x, origin.Y+step*(n-1)), ink.Black)
		y := origin.Y + i*step
		ink.DrawLine(image.Pt(origin.X, y), image.Pt(origin.X+step*(n-1), y), ink.Black)
	}

	pt := func(x, y int) image.Point { return image.Pt(origin.X+x*step, origin.Y+y*step) }
	r := step * 2 / 5

	// Black group in atari at the center intersection, White on 3 sides.
	for _, xy := range [][2]int{{0, 1}, {2, 1}, {1, 0}} {
		c := pt(xy[0], xy[1])
		rect := image.Rect(c.X-r, c.Y-r, c.X+r, c.Y+r)
		fillDisc(rect, ink.Black)
		fillDisc(pad(rect, r/4+2), ink.White)
	}
	c := pt(1, 1)
	fillDisc(image.Rect(c.X-r, c.Y-r, c.X+r, c.Y+r), ink.Black)

	// Highlight the one remaining liberty White is about to fill.
	lib := pt(1, 2)
	lr := step / 5
	ringDisc(image.Rect(lib.X-lr, lib.Y-lr, lib.X+lr, lib.Y+lr), 2, ink.Black)
}
