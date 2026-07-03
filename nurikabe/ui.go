package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"nurikabe/game"
)

type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Seed   *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 32, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 40, true),
		Seed:   ink.OpenFont(ink.DefaultFontBold, 34, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Seed} {
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
	usableH         = 1340
	statusBarHeight = 84
	buttonBarHeight = 140
	boardMargin     = 16
)

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

	cell := avail.Dx() / puz.W
	if hc := avail.Dy() / puz.H; hc < cell {
		cell = hc
	}
	l.Cell = cell

	gridW := cell * puz.W
	gridH := cell * puz.H
	ox := avail.Min.X + (avail.Dx()-gridW)/2
	oy := avail.Min.Y + (avail.Dy()-gridH)/2
	l.Origin = image.Pt(ox, oy)
	l.GridArea = image.Rect(ox, oy, ox+gridW, oy+gridH)
	return l
}

func (l *Layout) CellRect(x, y int) image.Rectangle {
	return image.Rect(
		l.Origin.X+x*l.Cell, l.Origin.Y+y*l.Cell,
		l.Origin.X+(x+1)*l.Cell, l.Origin.Y+(y+1)*l.Cell,
	)
}

func (l *Layout) ScreenToCell(p image.Point) (x, y int, ok bool) {
	if l.Cell == 0 {
		return 0, 0, false
	}
	rel := p.Sub(l.Origin)
	if rel.X < 0 || rel.Y < 0 {
		return 0, 0, false
	}
	x = rel.X / l.Cell
	y = rel.Y / l.Cell
	if x < 0 || x >= l.W || y < 0 || y >= l.H {
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
	drawCenteredString(l.StatusBar, text, 32)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawGrid renders seed cells (numbered, boxed), sea cells (filled black),
// island marks (small dot), and flags any 2x2-sea violation.
func DrawGrid(l *Layout, gs *game.GameState, f *Fonts) {
	ink.DrawRect(l.GridArea, ink.Black)
	ink.DrawRect(pad(l.GridArea, 1), ink.Black)

	violation := game.IsSea2x2Violation(gs)

	for y := 0; y < l.H; y++ {
		for x := 0; x < l.W; x++ {
			cell := l.CellRect(x, y)
			if sz, isSeed := gs.Puz.Seeds[[2]int{x, y}]; isSeed {
				ink.FillArea(pad(cell, 1), ink.White)
				ink.DrawRect(pad(cell, 3), ink.Black)
				f.Seed.SetActive(ink.Black)
				drawCenteredString(cell, itoa(sz), 30)
				continue
			}
			switch gs.Cells[y][x] {
			case game.StateSea:
				ink.FillArea(pad(cell, 1), ink.Black)
			case game.StateIsland:
				ink.FillArea(pad(cell, 1), ink.White)
				r := l.Cell / 6
				if r < 3 {
					r = 3
				}
				c := image.Pt(cell.Min.X+l.Cell/2, cell.Min.Y+l.Cell/2)
				fillDisc(c, r)
			default:
				ink.FillArea(pad(cell, 1), ink.White)
			}
		}
	}

	// Grid lines.
	for i := 1; i < l.W; i++ {
		px := l.Origin.X + i*l.Cell
		ink.DrawLine(image.Pt(px, l.GridArea.Min.Y), image.Pt(px, l.GridArea.Max.Y), ink.LightGray)
	}
	for i := 1; i < l.H; i++ {
		py := l.Origin.Y + i*l.Cell
		ink.DrawLine(image.Pt(l.GridArea.Min.X, py), image.Pt(l.GridArea.Max.X, py), ink.LightGray)
	}

	if violation {
		// Light hatch across the whole board as a mistake hint (a visible
		// pattern change, not just a color highlight, since color alone is
		// unreliable on e-ink).
		for i := 0; i < 6; i++ {
			y := l.GridArea.Min.Y + i*l.GridArea.Dy()/6
			ink.DrawLine(image.Pt(l.GridArea.Min.X, y), image.Pt(l.GridArea.Max.X, y), ink.DarkGray)
		}
	}
}

func fillDisc(c image.Point, r int) {
	for dy := -r; dy <= r; dy++ {
		half := isqrt(r*r - dy*dy)
		ink.DrawLine(image.Pt(c.X-half, c.Y+dy), image.Pt(c.X+half, c.Y+dy), ink.Black)
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

func (m *Menu) RulesButton() image.Rectangle { return m.rulesBtn }

func (m *Menu) Draw(screenPt image.Point, f *Fonts) {
	screen := image.Pt(screenPt.X, usableH)
	ink.ClearScreen()

	title := ink.OpenFont(ink.DefaultFontBold, 60, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Nurikabe")
	ink.DrawString(image.Pt((screen.X-tw)/2, 70), "Nurikabe")
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
	"Mål: måla varje ruta som antingen ö (vit) eller hav (svart) enligt siffrorna i rutnätet.",
	"Varje siffra är fröet till en ö: den ön ska till slut bestå av exakt så många sammanhängande vita rutor (räknat vågrätt/lodrätt), och innehålla precis den enda siffran.",
	"Två olika öar får aldrig ligga an mot varandra vågrätt eller lodrätt (diagonal kontakt är tillåten).",
	"Allt hav ska hänga ihop i en enda sammanhängande yta.",
	"Inget 2×2-block av rutor får bestå enbart av hav.",
	"Tryck på en ruta för att växla: tom → hav → ö-markering → tom. Fröets siffra går inte att ändra.",
	"Om ett 2×2-block med hav uppstår markeras rutnätet med strimmor som en varning.",
	"Knappen Rensa nollställer alla dina målningar (fröna påverkas inte).",
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

// drawSplashMotif draws a small grid with a couple of numbered white islands
// in a black sea.
func drawSplashMotif(box image.Rectangle) {
	const n = 5
	// 1 = sea, 0 = island
	sea := [n][n]int{
		{1, 1, 0, 1, 1},
		{1, 1, 0, 1, 1},
		{0, 0, 0, 1, 1},
		{1, 1, 1, 1, 0},
		{1, 1, 1, 0, 0},
	}
	seeds := map[[2]int]string{{2, 0}: "3", {4, 3}: "2"}
	cell := box.Dx() / n
	ox := box.Min.X + (box.Dx()-cell*n)/2
	oy := box.Min.Y + (box.Dy()-cell*n)/2
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			cr := image.Rect(ox+c*cell, oy+r*cell, ox+(c+1)*cell, oy+(r+1)*cell)
			if sea[r][c] == 1 {
				ink.FillArea(pad(cr, 2), ink.Black)
			}
		}
	}
	for pos, label := range seeds {
		cr := image.Rect(ox+pos[0]*cell, oy+pos[1]*cell, ox+(pos[0]+1)*cell, oy+(pos[1]+1)*cell)
		ink.FillArea(pad(cr, 2), ink.White)
		ink.DrawRect(pad(cr, 4), ink.Black)
		drawCenteredString(cr, label, cell/2)
	}
	for i := 0; i <= n; i++ {
		x := ox + i*cell
		ink.DrawLine(image.Pt(x, oy), image.Pt(x, oy+n*cell), ink.Black)
		y := oy + i*cell
		ink.DrawLine(image.Pt(ox, y), image.Pt(ox+n*cell, y), ink.Black)
	}
}
