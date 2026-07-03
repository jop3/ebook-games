package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"hashiwokakero/game"
)

type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Island *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 32, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 40, true),
		Island: ink.OpenFont(ink.DefaultFontBold, 34, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Island} {
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
	usableH         = 1340 // real drawable height; ScreenSize().Y (1448) lies
	statusBarHeight = 84
	buttonBarHeight = 140
	boardMargin     = 24
)

// Layout maps island grid coordinates to screen pixels.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	ButtonBar image.Rectangle

	GridArea image.Rectangle
	Origin   image.Point
	Cell     int
	W, H     int
}

func NewLayout(screenPt image.Point, puz *game.Puzzle) Layout {
	screen := image.Pt(screenPt.X, usableH)
	l := Layout{Screen: image.Rectangle{Max: screen}, W: puz.W, H: puz.H}
	l.StatusBar = image.Rect(0, 40, screen.X, 40+statusBarHeight)
	l.ButtonBar = image.Rect(0, screen.Y-buttonBarHeight-40, screen.X, screen.Y-40)

	avail := image.Rect(boardMargin, l.StatusBar.Max.Y+boardMargin,
		screen.X-boardMargin, l.ButtonBar.Min.Y-boardMargin)

	cell := avail.Dx() / (puz.W)
	if h := avail.Dy() / (puz.H); h < cell {
		cell = h
	}
	if cell < 24 {
		cell = 24
	}
	if cell > 130 {
		cell = 130
	}
	l.Cell = cell

	gridW := cell * (puz.W - 1)
	gridH := cell * (puz.H - 1)
	if puz.W == 1 {
		gridW = 0
	}
	if puz.H == 1 {
		gridH = 0
	}
	ox := avail.Min.X + (avail.Dx()-gridW)/2
	oy := avail.Min.Y + (avail.Dy()-gridH)/2
	l.Origin = image.Pt(ox, oy)
	l.GridArea = image.Rect(ox, oy, ox+gridW, oy+gridH)
	return l
}

// IslandCenter returns the screen point for an island grid coordinate.
func (l *Layout) IslandCenter(x, y int) image.Point {
	return image.Pt(l.Origin.X+x*l.Cell, l.Origin.Y+y*l.Cell)
}

// IslandRadius is the drawn circle radius for an island.
func (l *Layout) IslandRadius() int {
	r := l.Cell * 2 / 5
	if r > 46 {
		r = 46
	}
	if r < 14 {
		r = 14
	}
	return r
}

// HitIsland returns the island index at point p, if within tap range.
func HitIsland(l *Layout, puz *game.Puzzle, p image.Point) (int, bool) {
	r := l.IslandRadius() + 14 // generous tap band
	for i, isl := range puz.Islands {
		c := l.IslandCenter(isl.X, isl.Y)
		dx, dy := p.X-c.X, p.Y-c.Y
		if dx*dx+dy*dy <= r*r {
			return i, true
		}
	}
	return 0, false
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
	drawCenteredString(l.StatusBar, text, 32)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawBoard renders bridges (as one or two parallel lines), then islands (as
// circles with the required-bridge number, filled solid once satisfied), plus
// a highlight ring on the currently-selected island.
func DrawBoard(l *Layout, gs *game.GameState, f *Fonts, selected int, hasSelection bool) {
	// Bridges first, so islands draw on top.
	for k, cnt := range gs.Bridges {
		if cnt == 0 {
			continue
		}
		a, b := gs.Puz.Islands[k[0]], gs.Puz.Islands[k[1]]
		ca, cb := l.IslandCenter(a.X, a.Y), l.IslandCenter(b.X, b.Y)
		drawBridge(ca, cb, cnt)
	}

	r := l.IslandRadius()
	for i, isl := range gs.Puz.Islands {
		c := l.IslandCenter(isl.X, isl.Y)
		satisfied := gs.Degree(i) == isl.Need
		if satisfied {
			fillDisc(c, r)
			f.Island.SetActive(ink.White)
		} else {
			ink.FillArea(image.Rect(c.X-r, c.Y-r, c.X+r, c.Y+r), ink.White)
			ringDisc(c, r)
			f.Island.SetActive(ink.Black)
		}
		numRect := image.Rect(c.X-r, c.Y-r, c.X+r, c.Y+r)
		drawCenteredString(numRect, itoa(isl.Need), 30)
		if hasSelection && i == selected {
			ringDisc(c, r+8)
			ringDisc(c, r+11)
		}
	}
}

func drawBridge(a, b image.Point, count int) {
	if a.Y == b.Y {
		y := a.Y
		if count == 1 {
			ink.DrawLine(a, b, ink.Black)
		} else {
			ink.DrawLine(image.Pt(a.X, y-4), image.Pt(b.X, y-4), ink.Black)
			ink.DrawLine(image.Pt(a.X, y+4), image.Pt(b.X, y+4), ink.Black)
		}
		return
	}
	x := a.X
	if count == 1 {
		ink.DrawLine(a, b, ink.Black)
	} else {
		ink.DrawLine(image.Pt(x-4, a.Y), image.Pt(x-4, b.Y), ink.Black)
		ink.DrawLine(image.Pt(x+4, a.Y), image.Pt(x+4, b.Y), ink.Black)
	}
}

func fillDisc(c image.Point, r int) {
	for dy := -r; dy <= r; dy++ {
		half := isqrt(r*r - dy*dy)
		ink.DrawLine(image.Pt(c.X-half, c.Y+dy), image.Pt(c.X+half, c.Y+dy), ink.Black)
	}
}

func ringDisc(c image.Point, r int) {
	const steps = 72
	prev := image.Pt(c.X+r, c.Y)
	for i := 1; i <= steps; i++ {
		ang := 2 * 3.14159265 * float64(i) / steps
		p := image.Pt(c.X+int(float64(r)*cosf(ang)), c.Y+int(float64(r)*sinf(ang)))
		ink.DrawLine(prev, p, ink.Black)
		prev = p
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

func sinf(x float64) float64 {
	for x > 3.14159265 {
		x -= 2 * 3.14159265
	}
	for x < -3.14159265 {
		x += 2 * 3.14159265
	}
	x2 := x * x
	return x * (1 - x2/6*(1-x2/20*(1-x2/42)))
}
func cosf(x float64) float64 { return sinf(x + 3.14159265/2) }

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

// --- Menu ----------------------------------------------------------------

type menuRow struct {
	rect  image.Rectangle
	label string
}

type Menu struct {
	rows     []menuRow
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{} }

func (m *Menu) RulesButton() image.Rectangle { return m.rulesBtn }

func (m *Menu) Draw(screenPt image.Point, f *Fonts) {
	screen := image.Pt(screenPt.X, usableH)
	ink.ClearScreen()

	title := ink.OpenFont(ink.DefaultFontBold, 60, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Hashiwokakero")
	ink.DrawString(image.Pt((screen.X-tw)/2, 70), "Hashiwokakero")
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
	"Mål: förbind alla öar med broar så att varje ös antal broar stämmer med dess nummer, och alla öar hänger ihop i ett enda nätverk.",
	"En bro går rakt (vågrätt eller lodrätt) mellan två öar som ligger på samma rad eller kolumn, utan någon annan ö emellan.",
	"Tryck på en ö för att markera den, tryck sedan på en angränsande ö för att lägga en bro mellan dem. Tryck igen för att lägga till en andra bro (max två mellan samma par), och en tredje gång för att ta bort broarna igen.",
	"Broar får aldrig korsa varandra.",
	"En ö med siffran 3 måste till slut ha exakt tre broar anslutna. När en ö har rätt antal broar fylls den i som markering.",
	"Alla pussel går att lösa med ren logik — du behöver aldrig gissa.",
	"Knappen Rensa tar bort alla broar om du vill börja om.",
}

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

// --- Splash screen -----------------------------------------------------------

type motifFunc func(box image.Rectangle)

func DrawSplash(screenPt image.Point, f *Fonts, title string, motif motifFunc) {
	screen := image.Pt(screenPt.X, usableH)
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

// drawSplashMotif draws 4 islands with single/double bridges between them —
// the essence of Hashiwokakero.
func drawSplashMotif(box image.Rectangle) {
	step := box.Dx() / 4
	pts := [4]image.Point{
		{box.Min.X + step, box.Min.Y + step},
		{box.Min.X + 3*step, box.Min.Y + step},
		{box.Min.X + step, box.Min.Y + 3*step},
		{box.Min.X + 3*step, box.Min.Y + 3*step},
	}
	r := step / 3
	// single bridge top row
	ink.DrawLine(pts[0], pts[1], ink.Black)
	// double bridge left column
	ink.DrawLine(image.Pt(pts[0].X-6, pts[0].Y), image.Pt(pts[2].X-6, pts[2].Y), ink.Black)
	ink.DrawLine(image.Pt(pts[0].X+6, pts[0].Y), image.Pt(pts[2].X+6, pts[2].Y), ink.Black)
	// single bridge bottom row
	ink.DrawLine(pts[2], pts[3], ink.Black)

	labels := []string{"1", "3", "3", "1"}
	for i, p := range pts {
		ink.FillArea(image.Rect(p.X-r, p.Y-r, p.X+r, p.Y+r), ink.White)
		ringDisc(p, r)
		drawCenteredString(image.Rect(p.X-r, p.Y-r, p.X+r, p.Y+r), labels[i], 28)
	}
}
