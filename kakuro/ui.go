package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"kakuro/game"
)

type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Digit  *ink.Font
	Clue   *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 32, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 40, true),
		Digit:  ink.OpenFont(ink.DefaultFontBold, 36, true),
		Clue:   ink.OpenFont(ink.DefaultFontBold, 20, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Digit, f.Clue} {
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

const usableH = 1340 // real drawable height; ScreenSize().Y (1448) lies

type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle

	GridOrigin image.Point
	Cell       int
	W, H       int

	PadTop  int
	PadKeys []Button // 1..9
	Clear   Button
	Menu    Button
}

func NewLayout(screenPt image.Point, puz *game.Puzzle) Layout {
	screen := image.Pt(screenPt.X, usableH)
	l := Layout{Screen: image.Rectangle{Max: screen}, W: puz.W, H: puz.H}
	l.StatusBar = image.Rect(0, 40, screen.X, 40+84)

	margin := screen.X / 24
	bottomMargin := 40
	gap := screen.X / 60

	// Pad geometry first, bottom-up: Clear+Meny row, then 1..9 pad row above.
	btnH := screen.X / 12
	actionTop := screen.Y - bottomMargin - btnH
	padTop := actionTop - gap - btnH
	l.PadTop = padTop

	avail := screen.X - 2*margin
	cell := avail / puz.W
	if maxH := padTop - gap - (l.StatusBar.Max.Y + margin); cell*puz.H > maxH && puz.H > 0 {
		cell = maxH / puz.H
	}
	if cell < 20 {
		cell = 20
	}
	l.Cell = cell
	gridW := cell * puz.W
	gridH := cell * puz.H
	ox := (screen.X - gridW) / 2
	oy := l.StatusBar.Max.Y + (padTop-gap-l.StatusBar.Max.Y-gridH)/2
	if oy < l.StatusBar.Max.Y+margin {
		oy = l.StatusBar.Max.Y + margin
	}
	l.GridOrigin = image.Pt(ox, oy)

	// 1..9 keypad.
	padW := (screen.X - 2*margin - 8*gap) / 9
	l.PadKeys = make([]Button, 9)
	for i := 0; i < 9; i++ {
		x0 := margin + i*(padW+gap)
		r := image.Rect(x0, padTop, x0+padW, padTop+btnH)
		l.PadKeys[i] = Button{Rect: r, Label: itoa(i + 1)}
	}

	// Clear + Meny row.
	actW := (screen.X - 2*margin - gap) / 2
	l.Clear = Button{Rect: image.Rect(margin, actionTop, margin+actW, actionTop+btnH), Label: "Sudda"}
	l.Menu = Button{Rect: image.Rect(margin+actW+gap, actionTop, screen.X-margin, actionTop+btnH), Label: "Meny"}

	return l
}

func (l *Layout) CellRect(row, col int) image.Rectangle {
	x := l.GridOrigin.X + col*l.Cell
	y := l.GridOrigin.Y + row*l.Cell
	return image.Rect(x, y, x+l.Cell, y+l.Cell)
}

func (l *Layout) ScreenToCell(p image.Point) (row, col int, ok bool) {
	if l.Cell == 0 {
		return 0, 0, false
	}
	rel := p.Sub(l.GridOrigin)
	if rel.X < 0 || rel.Y < 0 {
		return 0, 0, false
	}
	col = rel.X / l.Cell
	row = rel.Y / l.Cell
	if row < 0 || row >= l.H || col < 0 || col >= l.W {
		return 0, 0, false
	}
	return row, col, true
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

// DrawGrid renders block cells (with diagonal split + up to 2 clue numbers),
// entry cells (with the player's digit, highlighted if selected), and marks
// any run currently in violation.
func DrawGrid(l *Layout, gs *game.GameState, f *Fonts, selRow, selCol int, hasSel bool) {
	for r := 0; r < l.H; r++ {
		for c := 0; c < l.W; c++ {
			cell := gs.Puz.Grid[r][c]
			rect := l.CellRect(r, c)
			switch cell.Kind {
			case game.KindBlock:
				ink.FillArea(pad(rect, 1), ink.Black)
				if cell.DownClue > 0 || cell.RightClue > 0 {
					ink.DrawLine(rect.Min, rect.Max, ink.White)
					f.Clue.SetActive(ink.White)
					if cell.RightClue > 0 {
						r2 := image.Rect(rect.Min.X+rect.Dx()/2, rect.Max.Y-rect.Dy()/2, rect.Max.X, rect.Max.Y)
						drawCenteredString(r2, itoa(cell.RightClue), 18)
					}
					if cell.DownClue > 0 {
						r2 := image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X-rect.Dx()/2, rect.Min.Y+rect.Dy()/2)
						drawCenteredString(r2, itoa(cell.DownClue), 18)
					}
				}
			case game.KindEntry:
				ink.DrawRect(rect, ink.Black)
				if hasSel && r == selRow && c == selCol {
					ink.FillArea(pad(rect, 2), ink.LightGray)
				} else {
					ink.FillArea(pad(rect, 2), ink.White)
				}
				ink.DrawRect(rect, ink.Black)
				if cell.Value != 0 {
					f.Digit.SetActive(ink.Black)
					drawCenteredString(rect, itoa(cell.Value), 34)
				}
			}
		}
	}
}

func DrawKeypad(l *Layout, f *Fonts, usedInSelection map[int]bool) []Button {
	f.Button.SetActive(ink.Black)
	var buttons []Button
	for _, b := range l.PadKeys {
		ink.DrawRect(b.Rect, ink.Black)
		ink.DrawRect(pad(b.Rect, 1), ink.Black)
		if usedInSelection[atoi(b.Label)] {
			ink.FillArea(pad(b.Rect, 3), ink.LightGray)
			ink.DrawRect(b.Rect, ink.Black)
		}
		drawCenteredString(b.Rect, b.Label, 36)
		buttons = append(buttons, b)
	}
	for _, b := range []Button{l.Clear, l.Menu} {
		ink.DrawRect(b.Rect, ink.Black)
		ink.DrawRect(pad(b.Rect, 1), ink.Black)
		drawCenteredString(b.Rect, b.Label, 38)
		buttons = append(buttons, b)
	}
	return buttons
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
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
	tw := ink.StringWidth("Kakuro")
	ink.DrawString(image.Pt((screen.X-tw)/2, 70), "Kakuro")
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
	"Mål: fyll i alla vita rutor med siffrorna 1-9 så att varje \"ord\" (en sammanhängande rad eller kolumn av vita rutor) summerar till talet som står i den svarta rutan intill.",
	"Talet uppe till höger i en svart ruta gäller den lodräta raden av vita rutor under den. Talet nere till vänster gäller den vågräta raden till höger.",
	"Ingen siffra får upprepas inom samma rad eller kolumn (samma \"ord\").",
	"Tryck på en vit ruta för att välja den, tryck sedan på en siffra 1-9 i knappraden för att fylla i den. Sudda tar bort siffran i den valda rutan.",
	"Alla pussel går att lösa med logik: använd summan och regeln om inga upprepningar för att räkna ut vilka siffror som måste stå var.",
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

// drawSplashMotif draws a small Kakuro corner: one diagonal-clue block cell
// feeding a short run of digit cells.
func drawSplashMotif(box image.Rectangle) {
	n := 4
	cell := box.Dx() / n
	ox := box.Min.X + (box.Dx()-cell*n)/2
	oy := box.Min.Y + (box.Dy()-cell*n)/2
	grid := [4][4]bool{
		{false, false, true, true},
		{false, true, true, false},
		{true, true, false, false},
		{false, false, false, false},
	}
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			cr := image.Rect(ox+c*cell, oy+r*cell, ox+(c+1)*cell, oy+(r+1)*cell)
			if grid[r][c] {
				ink.DrawRect(pad(cr, 2), ink.Black)
			} else {
				ink.FillArea(pad(cr, 2), ink.Black)
			}
		}
	}
	for i := 0; i <= n; i++ {
		x := ox + i*cell
		ink.DrawLine(image.Pt(x, oy), image.Pt(x, oy+n*cell), ink.Black)
		y := oy + i*cell
		ink.DrawLine(image.Pt(ox, y), image.Pt(ox+n*cell, y), ink.Black)
	}
}
