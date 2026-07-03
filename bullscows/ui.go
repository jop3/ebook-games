package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"bullscows/game"
)

type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Digit  *ink.Font // big digits in keypad / entry
	Row    *ink.Font // history rows
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 36, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 40, true),
		Digit:  ink.OpenFont(ink.DefaultFontBold, 52, true),
		Row:    ink.OpenFont(ink.DefaultFontBold, 44, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Digit, f.Row} {
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
	usableH         = 1340 // ScreenSize().Y (1448) lies; below ~1360 wraps to top
	margin          = 40
	statusBarHeight = 96
	buttonBarHeight = 140
	entryHeight     = 130
	keypadHeight    = 430
)

// Layout partitions the screen into: status bar, scrolling history, current
// entry row, digit keypad, button bar.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	History   image.Rectangle
	Entry     image.Rectangle
	Keypad    image.Rectangle
	ButtonBar image.Rectangle
}

func NewLayout(screen image.Point) Layout {
	W := screen.X
	H := usableH
	l := Layout{Screen: image.Rect(0, 0, W, H)}
	l.StatusBar = image.Rect(0, 0, W, statusBarHeight)
	// Keypad and button bar hold tappable rects, so inset them horizontally by
	// margin: their internal gap padding must still land inside the safe zone
	// (x in [24, W-24]), and starting flush at x=0 leaves the outermost
	// key/button edge inside the margin.
	l.ButtonBar = image.Rect(margin, H-margin-buttonBarHeight, W-margin, H-margin)
	l.Keypad = image.Rect(margin, l.ButtonBar.Min.Y-keypadHeight, W-margin, l.ButtonBar.Min.Y)
	l.Entry = image.Rect(0, l.Keypad.Min.Y-entryHeight, W, l.Keypad.Min.Y)
	l.History = image.Rect(0, statusBarHeight, W, l.Entry.Min.Y)
	return l
}

// --- Rendering -------------------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

// Key is a tappable digit on the keypad.
type Key struct {
	Rect  image.Rectangle
	Digit int
}

func (k Key) Hit(p image.Point) bool { return p.In(k.Rect) }

func DrawStatus(l *Layout, text string, f *Fonts) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	// Centering the text within the full StatusBar (which starts at y=0) puts
	// its top edge above the y=40 safe margin; center it within a rect that
	// starts at the margin instead so the glyph top stays in the safe zone.
	drawCenteredString(image.Rect(l.StatusBar.Min.X, margin, l.StatusBar.Max.X, l.StatusBar.Max.Y), text, 38)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawHistory renders past guesses (most recent at the bottom). Each row shows
// the guessed digits and a Bulls/Cows summary using filled/hollow markers.
func DrawHistory(l *Layout, s *game.GameState, f *Fonts) {
	ink.FillArea(l.History, ink.White)
	rowH := 78
	n := len(s.Guesses)
	// How many rows fit; show the last that fit.
	maxRows := l.History.Dy() / rowH
	start := 0
	if n > maxRows {
		start = n - maxRows
	}
	f.Row.SetActive(ink.Black)
	y := l.History.Min.Y + 10
	for i := start; i < n; i++ {
		g := s.Guesses[i]
		row := image.Rect(l.History.Min.X+40, y, l.History.Max.X-40, y+rowH-10)
		drawGuessRow(row, g, f)
		y += rowH
	}
}

// drawGuessRow lays out one guess: the digits on the left, then bull markers
// (filled squares) and cow markers (hollow squares) on the right.
func drawGuessRow(row image.Rectangle, g game.Guess, f *Fonts) {
	// Digits.
	f.Row.SetActive(ink.Black)
	dx := row.Min.X
	for _, d := range g.Code {
		cell := image.Rect(dx, row.Min.Y, dx+56, row.Max.Y)
		drawCenteredString(cell, itoa(d), 44)
		dx += 60
	}
	// Bulls (filled) then cows (hollow) as small squares on the right.
	mx := row.Max.X
	sz := 34
	gap := 12
	my := (row.Min.Y + row.Max.Y) / 2
	// draw from right to left: cows first (outermost), then bulls
	for i := 0; i < g.Score.Cows; i++ {
		mx -= sz + gap
		r := image.Rect(mx, my-sz/2, mx+sz, my+sz/2)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
	}
	for i := 0; i < g.Score.Bulls; i++ {
		mx -= sz + gap
		r := image.Rect(mx, my-sz/2, mx+sz, my+sz/2)
		ink.FillArea(r, ink.Black)
	}
}

// DrawEntry renders the current in-progress guess as boxes, one per slot.
func DrawEntry(l *Layout, s *game.GameState, f *Fonts) {
	ink.FillArea(l.Entry, ink.White)
	ink.DrawLine(image.Pt(l.Entry.Min.X, l.Entry.Min.Y),
		image.Pt(l.Entry.Max.X, l.Entry.Min.Y), ink.Black)

	n := s.Cfg.Length
	box := 90
	gap := 20
	totalW := n*box + (n-1)*gap
	x0 := l.Entry.Min.X + (l.Entry.Dx()-totalW)/2
	y0 := l.Entry.Min.Y + (l.Entry.Dy()-box)/2
	f.Digit.SetActive(ink.Black)
	for i := 0; i < n; i++ {
		r := image.Rect(x0+i*(box+gap), y0, x0+i*(box+gap)+box, y0+box)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		if i < len(s.Entry) {
			drawCenteredString(r, itoa(s.Entry[i]), 52)
		}
	}
}

// DrawKeypad renders the 0-9 digit keys and returns their hit rects. Digits
// already used in the current entry are greyed and non-tappable.
func DrawKeypad(l *Layout, s *game.GameState, f *Fonts) []Key {
	ink.FillArea(l.Keypad, ink.White)
	ink.DrawLine(image.Pt(l.Keypad.Min.X, l.Keypad.Min.Y),
		image.Pt(l.Keypad.Max.X, l.Keypad.Min.Y), ink.Black)

	// Two rows: 0-4, 5-9.
	cols := 5
	rows := 2
	gap := 18
	kw := (l.Keypad.Dx() - gap*(cols+1)) / cols
	kh := (l.Keypad.Dy() - gap*(rows+1)) / rows

	keys := make([]Key, 0, 10)
	for d := 0; d < 10; d++ {
		r := d / cols
		c := d % cols
		x0 := l.Keypad.Min.X + gap + c*(kw+gap)
		y0 := l.Keypad.Min.Y + gap + r*(kh+gap)
		rect := image.Rect(x0, y0, x0+kw, y0+kh)
		avail := s.DigitAvailable(d) && !s.Solved
		if avail {
			ink.DrawRect(rect, ink.Black)
			ink.DrawRect(pad(rect, 1), ink.Black)
			f.Digit.SetActive(ink.Black)
		} else {
			ink.DrawRect(rect, ink.LightGray)
			f.Digit.SetActive(ink.LightGray)
		}
		drawCenteredString(rect, itoa(d), 52)
		if avail {
			keys = append(keys, Key{Rect: rect, Digit: d})
		}
	}
	return keys
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
	totalGap := gap * (n + 1)
	bw := (l.ButtonBar.Dx() - totalGap) / n
	bh := l.ButtonBar.Dy() - 2*gap
	buttons := make([]Button, n)
	for i, label := range labels {
		x0 := l.ButtonBar.Min.X + gap + i*(bw+gap)
		y0 := l.ButtonBar.Min.Y + gap
		r := image.Rect(x0, y0, x0+bw, y0+bh)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawCenteredString(r, label, 36)
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

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	W, H := screen.X, usableH

	title := ink.OpenFont(ink.DefaultFontBold, 60, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Bulls & Cows")
	ink.DrawString(image.Pt((W-tw)/2, 70), "Bulls & Cows")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 34, true)
	sub.SetActive(ink.Black)
	subT := "Välj svårighetsgrad"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((W-sw)/2, 170), subT)
	sub.Close()

	f.Menu.SetActive(ink.Black)
	rowH := 130
	rowMargin := 60
	rowW := W - 2*rowMargin

	// Stack the difficulty rows plus the "Regler" button bottom-up so they
	// never spill past the real usable height, regardless of preset count.
	n := len(game.Presets)
	rbH := 100
	blockH := n*rowH + 30 + rbH
	y := H - margin - blockH
	if minY := 250; y < minY {
		y = minY // keep clear of the title/subtitle
	}

	m.rows = m.rows[:0]
	for _, p := range game.Presets {
		r := image.Rect(rowMargin, y, rowMargin+rowW, y+rowH-20)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawLeftString(r, p.Name, 40)
		m.rows = append(m.rows, menuRow{rect: r, label: p.Name})
		y += rowH
	}

	// "Regler" button opens the full rules screen.
	rbW := rowW / 2
	rb := image.Rect((W-rbW)/2, y+30, (W+rbW)/2, y+30+rbH)
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
	"Mål: lista ut den hemliga koden på så få gissningar som möjligt.",
	"Koden består av olika siffror (inga upprepningar). Tryck siffrorna för att bygga din gissning och tryck sedan Gissa.",
	"Efter varje gissning får du besked:",
	"Bull (fylld ruta ■): rätt siffra på rätt plats.",
	"Cow (ihålig ruta □): rätt siffra men på fel plats.",
	"Exempel: koden är 1 2 3 4 och du gissar 1 4 5 6 → en bull (1:an) och en cow (4:an).",
	"Siffror du redan lagt i gissningen gråas ut på knappsatsen. Tryck Sudda för att ta bort den senaste.",
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
	btnY := usableH - margin - bh
	r := image.Rect((screen.X-bw)/2, btnY, (screen.X+bw)/2, btnY+bh)
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
	W, H := screen.X, usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 72, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((W-tw)/2, H/6), title)
	tf.Close()

	side := W * 3 / 5
	box := image.Rect((W-side)/2, (H-side)/2,
		(W+side)/2, (H+side)/2)
	motif(box)

	hint := ink.OpenFont(ink.DefaultFont, 34, true)
	hint.SetActive(ink.DarkGray)
	ht := "Tryck för att börja"
	hw := ink.StringWidth(ht)
	ink.DrawString(image.Pt((W-hw)/2, H*5/6), ht)
	hint.Close()
}

// drawSplashMotif draws a guess row "1 2 3 4" in boxes with bull/cow markers
// below — the heart of Bulls & Cows in simple graphics.
func drawSplashMotif(box image.Rectangle) {
	digits := []string{"1", "2", "3", "4"}
	n := len(digits)
	bw := box.Dx() / 6
	gap := bw / 3
	totalW := n*bw + (n-1)*gap
	x0 := box.Min.X + (box.Dx()-totalW)/2
	y0 := box.Min.Y + box.Dy()/6

	bigDigit := ink.OpenFont(ink.DefaultFontBold, 64, true)
	bigDigit.SetActive(ink.Black)
	for i, d := range digits {
		r := image.Rect(x0+i*(bw+gap), y0, x0+i*(bw+gap)+bw, y0+bw)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawCenteredString(r, d, 64)
	}
	bigDigit.Close()

	// Feedback markers: two filled (bulls) and two hollow (cows), centered below.
	my := y0 + bw + box.Dy()/6
	sz := bw / 2
	mgap := sz / 2
	kinds := []bool{true, false, true, false} // filled, hollow, ...
	mw := n*sz + (n-1)*mgap
	mx := box.Min.X + (box.Dx()-mw)/2
	for i, filled := range kinds {
		r := image.Rect(mx+i*(sz+mgap), my, mx+i*(sz+mgap)+sz, my+sz)
		if filled {
			ink.FillArea(r, ink.Black)
		} else {
			ink.DrawRect(r, ink.Black)
			ink.DrawRect(pad(r, 1), ink.Black)
		}
	}
}
