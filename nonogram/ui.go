package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"nonogram/game"
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

// Layout positions the clue margins and the cell grid. The left margin width and
// top margin height scale with the largest clue count so all numbers fit.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	ButtonBar image.Rectangle

	GridArea   image.Rectangle // the cell area only (excludes clue margins)
	GridOrigin image.Point
	CellSize   int
	W, H       int

	LeftMargin int // px width reserved for row clues
	TopMargin  int // px height reserved for column clues
	ClueUnit   int // px per clue number cell
}

func NewLayout(screenPt image.Point, puz *game.Puzzle) Layout {
	screen := image.Pt(screenPt.X, usableH)
	l := Layout{Screen: image.Rectangle{Max: screen}, W: puz.W, H: puz.H}
	l.StatusBar = image.Rect(0, 40, screen.X, 40+statusBarHeight)
	l.ButtonBar = image.Rect(0, screen.Y-buttonBarHeight-40, screen.X, screen.Y-40)

	avail := image.Rect(boardMargin, l.StatusBar.Max.Y+boardMargin,
		screen.X-boardMargin, l.ButtonBar.Min.Y-boardMargin)

	// Longest clue lists determine margin sizes.
	maxRowClue := 1
	for _, c := range puz.RowClues {
		if n := len(c.Display()); n > maxRowClue {
			maxRowClue = n
		}
	}
	maxColClue := 1
	for _, c := range puz.ColClues {
		if n := len(c.Display()); n > maxColClue {
			maxColClue = n
		}
	}

	// Solve cell size so that grid + margins fit both dimensions. We reserve
	// clueUnit per clue number; margin = maxClue*clueUnit. Use a cell size that
	// fits, then set clueUnit proportional but capped for readability.
	// Try the biggest cell size that fits width and height.
	cell := 0
	for c := 96; c >= 12; c-- {
		clueUnit := c * 3 / 4
		if clueUnit < 18 {
			clueUnit = 18
		}
		gw := c*puz.W + maxRowClue*clueUnit
		gh := c*puz.H + maxColClue*clueUnit
		if gw <= avail.Dx() && gh <= avail.Dy() {
			cell = c
			l.ClueUnit = clueUnit
			break
		}
	}
	if cell == 0 {
		cell = 12
		l.ClueUnit = 18
	}
	l.CellSize = cell
	l.LeftMargin = maxRowClue * l.ClueUnit
	l.TopMargin = maxColClue * l.ClueUnit

	gridW := cell * puz.W
	gridH := cell * puz.H
	totalW := gridW + l.LeftMargin
	totalH := gridH + l.TopMargin
	// Center the whole thing (clues + grid) in avail.
	ox := avail.Min.X + (avail.Dx()-totalW)/2
	oy := avail.Min.Y + (avail.Dy()-totalH)/2
	l.GridOrigin = image.Pt(ox+l.LeftMargin, oy+l.TopMargin)
	l.GridArea = image.Rect(l.GridOrigin.X, l.GridOrigin.Y,
		l.GridOrigin.X+gridW, l.GridOrigin.Y+gridH)
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

// DrawClues renders the row clues (left margin, right-aligned) and column clues
// (top margin, bottom-aligned).
func DrawClues(l *Layout, s *game.GameState, f *Fonts) {
	f.Clue.SetActive(ink.Black)
	u := l.ClueUnit

	// Row clues: to the left of each row, numbers right-aligned to the grid.
	for y := 0; y < l.H; y++ {
		clue := s.Puz.RowClues[y].Display()
		cell := l.CellToScreen(0, y)
		// place numbers ending at grid's left edge
		x1 := cell.Min.X
		for i := len(clue) - 1; i >= 0; i-- {
			x1 -= u
			r := image.Rect(x1, cell.Min.Y, x1+u, cell.Max.Y)
			drawCenteredString(r, itoa(clue[i]), 30)
		}
	}

	// Column clues: above each column, numbers bottom-aligned to the grid.
	for x := 0; x < l.W; x++ {
		clue := s.Puz.ColClues[x].Display()
		cell := l.CellToScreen(x, 0)
		y1 := cell.Min.Y
		for i := len(clue) - 1; i >= 0; i-- {
			y1 -= u
			r := image.Rect(cell.Min.X, y1, cell.Max.X, y1+u)
			drawCenteredString(r, itoa(clue[i]), 30)
		}
	}
}

// DrawGrid renders the cell grid with heavier lines every 5 cells, and the
// player's fill / X marks.
func DrawGrid(l *Layout, s *game.GameState, f *Fonts) {
	ink.DrawRect(l.GridArea, ink.Black)
	ink.DrawRect(pad(l.GridArea, 1), ink.Black)

	// Fills and marks first.
	for y := 0; y < l.H; y++ {
		for x := 0; x < l.W; x++ {
			cell := l.CellToScreen(x, y)
			switch s.Cells[y][x] {
			case game.StateFilled:
				ink.FillArea(pad(cell, 1), ink.Black)
			case game.StateMarked:
				drawX(cell)
			}
		}
	}

	// Grid lines (drawn over blanks; fills already cover their cells).
	for i := 1; i < l.W; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		var col color.Color = ink.LightGray
		if i%5 == 0 {
			col = ink.Black
		}
		ink.DrawLine(image.Pt(px, l.GridArea.Min.Y), image.Pt(px, l.GridArea.Max.Y), col)
	}
	for i := 1; i < l.H; i++ {
		py := l.GridOrigin.Y + i*l.CellSize
		var col color.Color = ink.LightGray
		if i%5 == 0 {
			col = ink.Black
		}
		ink.DrawLine(image.Pt(l.GridArea.Min.X, py), image.Pt(l.GridArea.Max.X, py), col)
	}
}

// drawX draws a light X to mark a cell the player believes is blank.
func drawX(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/4)
	ink.DrawLine(image.Pt(r.Min.X, r.Min.Y), image.Pt(r.Max.X, r.Max.Y), ink.DarkGray)
	ink.DrawLine(image.Pt(r.Max.X, r.Min.Y), image.Pt(r.Min.X, r.Max.Y), ink.DarkGray)
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
	tw := ink.StringWidth("Nonogram")
	ink.DrawString(image.Pt((screen.X-tw)/2, 70), "Nonogram")
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
	"Mål: fyll i rutorna så att den dolda bilden framträder.",
	"Siffrorna vid varje rad och kolumn anger längden på de ifyllda blocken, i ordning. \"3 1\" betyder ett block på tre rutor, sedan (minst en tom ruta) och sedan ett block på en ruta.",
	"Tryck på en ruta för att växla: fylld → X (säkert tom) → tom. X är bara din egen minnesanteckning.",
	"Använd siffrorna i både rader och kolumner tillsammans för att lista ut vilka rutor som måste vara fyllda.",
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

// drawSplashMotif draws a small nonogram grid whose filled cells form a heart —
// a picture emerging from the grid, the essence of the game.
func drawSplashMotif(box image.Rectangle) {
	const n = 7
	pat := [n][n]int{
		{0, 1, 1, 0, 1, 1, 0},
		{1, 1, 1, 1, 1, 1, 1},
		{1, 1, 1, 1, 1, 1, 1},
		{1, 1, 1, 1, 1, 1, 1},
		{0, 1, 1, 1, 1, 1, 0},
		{0, 0, 1, 1, 1, 0, 0},
		{0, 0, 0, 1, 0, 0, 0},
	}
	cell := box.Dx() / n
	ox := box.Min.X + (box.Dx()-cell*n)/2
	oy := box.Min.Y + (box.Dy()-cell*n)/2
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			cr := image.Rect(ox+c*cell, oy+r*cell, ox+(c+1)*cell, oy+(r+1)*cell)
			if pat[r][c] == 1 {
				ink.FillArea(pad(cr, 2), ink.Black)
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
