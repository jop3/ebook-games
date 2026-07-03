package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"akari/game"
)

type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Number *ink.Font // wall clue numbers
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 32, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 40, true),
		Number: ink.OpenFont(ink.DefaultFontBold, 34, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Number} {
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

// Layout positions the cell grid, centered in the space between the status
// bar and the button bar.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	ButtonBar image.Rectangle

	GridArea   image.Rectangle
	GridOrigin image.Point
	CellSize   int
	W, H       int
}

func NewLayout(screenPt image.Point, b *game.Board) Layout {
	screen := image.Pt(screenPt.X, usableH)
	l := Layout{Screen: image.Rectangle{Max: screen}, W: b.W, H: b.H}
	l.StatusBar = image.Rect(0, 40, screen.X, 40+statusBarHeight)
	l.ButtonBar = image.Rect(0, screen.Y-buttonBarHeight-40, screen.X, screen.Y-40)

	avail := image.Rect(boardMargin, l.StatusBar.Max.Y+boardMargin,
		screen.X-boardMargin, l.ButtonBar.Min.Y-boardMargin)

	cell := min(avail.Dx()/b.W, avail.Dy()/b.H)
	if cell < 12 {
		cell = 12
	}
	l.CellSize = cell

	gridW := cell * b.W
	gridH := cell * b.H
	ox := avail.Min.X + (avail.Dx()-gridW)/2
	oy := avail.Min.Y + (avail.Dy()-gridH)/2
	l.GridOrigin = image.Pt(ox, oy)
	l.GridArea = image.Rect(ox, oy, ox+gridW, oy+gridH)
	return l
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
	if x < 0 || x >= l.W || y < 0 || y >= l.H {
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
	drawCenteredString(l.StatusBar, text, 32)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawGrid renders the wall/white cells, lit-cell wash, bulbs, dot marks and
// wall clue numbers, plus a conflict X on any bulb that sees another bulb.
func DrawGrid(l *Layout, s *game.GameState, f *Fonts) {
	ink.DrawRect(l.GridArea, ink.Black)
	ink.DrawRect(pad(l.GridArea, 1), ink.Black)

	lit := s.LitGrid()
	conflict := s.ConflictBulbs()

	for y := 0; y < l.H; y++ {
		for x := 0; x < l.W; x++ {
			cell := l.CellToScreen(x, y)
			c := s.Board.Cells[y][x]
			switch c.Kind {
			case game.Wall:
				ink.FillArea(pad(cell, 1), ink.Black)
				if c.Number >= 0 {
					f.Number.SetActive(ink.White)
					drawCenteredString(cell, itoa(c.Number), 34)
				}
			case game.White:
				if lit[y][x] {
					ink.FillArea(pad(cell, 1), ink.LightGray)
				}
				switch s.Marks[y][x] {
				case game.MarkBulb:
					drawBulb(cell)
					if conflict[y][x] {
						drawConflictMark(cell)
					}
				case game.MarkDot:
					drawDot(cell)
				}
			}
		}
	}

	// Grid lines.
	for i := 1; i < l.W; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.GridArea.Min.Y), image.Pt(px, l.GridArea.Max.Y), ink.DarkGray)
	}
	for i := 1; i < l.H; i++ {
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.GridArea.Min.X, py), image.Pt(l.GridArea.Max.X, py), ink.DarkGray)
	}
}

// drawBulb draws a filled sun-like disc for a placed bulb.
func drawBulb(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/5)
	cx, cy := (r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2
	rad := r.Dx() / 2
	drawDisc(image.Pt(cx, cy), rad, ink.Black)
}

// drawDisc fills an approximate circle using horizontal scanlines (no
// dedicated ellipse primitive in the ink API).
func drawDisc(center image.Point, radius int, col color.Color) {
	for dy := -radius; dy <= radius; dy++ {
		// dx^2 + dy^2 <= r^2
		dxMax := 0
		for dx := radius; dx >= 0; dx-- {
			if dx*dx+dy*dy <= radius*radius {
				dxMax = dx
				break
			}
		}
		y := center.Y + dy
		ink.DrawLine(image.Pt(center.X-dxMax, y), image.Pt(center.X+dxMax, y), col)
	}
}

// drawDot draws a small "known empty" memory mark.
func drawDot(cell image.Rectangle) {
	cx, cy := (cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2
	rad := cell.Dx() / 8
	if rad < 3 {
		rad = 3
	}
	drawDisc(image.Pt(cx, cy), rad, ink.DarkGray)
}

// drawConflictMark overlays a small X on a bulb that sees another bulb.
func drawConflictMark(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/6)
	ink.DrawLine(image.Pt(r.Min.X, r.Min.Y), image.Pt(r.Max.X, r.Max.Y), ink.White)
	ink.DrawLine(image.Pt(r.Max.X, r.Min.Y), image.Pt(r.Min.X, r.Max.Y), ink.White)
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
	titleText := "Akari"
	tw := ink.StringWidth(titleText)
	ink.DrawString(image.Pt((screen.X-tw)/2, 70), titleText)
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
	"Mål: placera lampor så att alla vita rutor lyser upp.",
	"En lampa lyser upp sin egen ruta och rakt ut åt alla fyra håll tills den träffar en svart vägg eller rutnätets kant.",
	"Två lampor får aldrig lysa på varandra. Om de gör det markeras båda med ett vitt kryss så att du ser konflikten.",
	"Siffror på svarta väggar (0–4) anger exakt hur många lampor som måste stå i de angränsande rutorna (upp, ner, vänster, höger). En vägg utan siffra har inget krav.",
	"Tryck på en vit ruta för att växla: tom → lampa → punkt (minnesanteckning \"säkert tom\") → tom.",
	"Du har vunnit när alla vita rutor lyser, inga lampor lyser på varandra, och alla nummererade väggar stämmer.",
	"Alla pussel går att lösa med ren logik — du behöver aldrig gissa.",
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

// drawSplashMotif draws a small Akari grid: a couple of walls, a couple of
// bulbs and their light rays (grey lines) crossing the board.
func drawSplashMotif(box image.Rectangle) {
	const n = 5
	// 0 = white, 1 = wall, 2 = bulb
	pat := [n][n]int{
		{0, 0, 1, 0, 0},
		{0, 0, 0, 0, 0},
		{2, 0, 0, 0, 2},
		{0, 0, 0, 0, 0},
		{0, 0, 1, 0, 0},
	}
	cell := box.Dx() / n
	ox := box.Min.X + (box.Dx()-cell*n)/2
	oy := box.Min.Y + (box.Dy()-cell*n)/2

	cellRect := func(r, c int) image.Rectangle {
		return image.Rect(ox+c*cell, oy+r*cell, ox+(c+1)*cell, oy+(r+1)*cell)
	}

	// Light rays first (grey), so walls/bulbs draw crisply on top.
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			if pat[r][c] != 2 {
				continue
			}
			// Ray to the right and left, stopping at a wall.
			for dc := 1; c+dc < n && pat[r][c+dc] != 1; dc++ {
				cr := cellRect(r, c+dc)
				ink.DrawLine(image.Pt(cr.Min.X, (cr.Min.Y+cr.Max.Y)/2),
					image.Pt(cr.Max.X, (cr.Min.Y+cr.Max.Y)/2), ink.LightGray)
			}
			for dc := 1; c-dc >= 0 && pat[r][c-dc] != 1; dc++ {
				cr := cellRect(r, c-dc)
				ink.DrawLine(image.Pt(cr.Min.X, (cr.Min.Y+cr.Max.Y)/2),
					image.Pt(cr.Max.X, (cr.Min.Y+cr.Max.Y)/2), ink.LightGray)
			}
			// Ray up and down.
			for dr := 1; r+dr < n && pat[r+dr][c] != 1; dr++ {
				cr := cellRect(r+dr, c)
				ink.DrawLine(image.Pt((cr.Min.X+cr.Max.X)/2, cr.Min.Y),
					image.Pt((cr.Min.X+cr.Max.X)/2, cr.Max.Y), ink.LightGray)
			}
			for dr := 1; r-dr >= 0 && pat[r-dr][c] != 1; dr++ {
				cr := cellRect(r-dr, c)
				ink.DrawLine(image.Pt((cr.Min.X+cr.Max.X)/2, cr.Min.Y),
					image.Pt((cr.Min.X+cr.Max.X)/2, cr.Max.Y), ink.LightGray)
			}
		}
	}

	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			cr := cellRect(r, c)
			switch pat[r][c] {
			case 1:
				ink.FillArea(pad(cr, 2), ink.Black)
			case 2:
				drawBulb(cr)
			}
		}
	}

	// Grid lines over the whole square.
	for i := 0; i <= n; i++ {
		x := ox + i*cell
		ink.DrawLine(image.Pt(x, oy), image.Pt(x, oy+n*cell), ink.Black)
		y := oy + i*cell
		ink.DrawLine(image.Pt(ox, y), image.Pt(ox+n*cell, y), ink.Black)
	}
}
