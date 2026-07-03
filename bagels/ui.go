package main

import (
	"image"
	"strings"

	ink "github.com/dennwc/inkview"

	"bagels/game"
)

type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Digit  *ink.Font // big digits in keypad / entry
	Row    *ink.Font // history-row digits
	Word   *ink.Font // history-row word feedback
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 36, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 40, true),
		Digit:  ink.OpenFont(ink.DefaultFontBold, 52, true),
		Row:    ink.OpenFont(ink.DefaultFontBold, 44, true),
		Word:   ink.OpenFont(ink.DefaultFont, 34, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Digit, f.Row, f.Word} {
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

// codeString renders a code as space-separated digits, e.g. "1 4 7".
func codeString(c game.Code) string {
	parts := make([]string, len(c))
	for i, d := range c {
		parts[i] = itoa(d)
	}
	return strings.Join(parts, " ")
}

// --- Layout ----------------------------------------------------------------

const (
	statusBarHeight = 96
	buttonBarHeight = 140
	entryHeight     = 130
	keypadHeight    = 430

	// usableH is the real drawable height on the PB634. ink.ScreenSize()
	// reports 1448, but content below ~1360 wraps to the top of the screen
	// (guide §5). All vertical layout must derive from this, never from
	// ScreenSize().Y directly.
	usableH = 1340

	// safeMargin keeps interactive UI (text/borders) out of the screen edges,
	// which the hardware doesn't render reliably (guide §5).
	safeMargin = 40
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
	screen.Y = usableH // real drawable height; ScreenSize().Y (1448) lies (guide §5)
	l := Layout{Screen: image.Rectangle{Max: screen}}
	// Content area is inset from the screen edges so text/borders never land
	// in the unreliable margin (guide §5). Stack bottom-anchored rows UP from
	// the true bottom (H - safeMargin), not the raw screen height.
	left, right := safeMargin, screen.X-safeMargin
	top, bottom := safeMargin, screen.Y-safeMargin

	l.StatusBar = image.Rect(left, top, right, top+statusBarHeight)
	l.ButtonBar = image.Rect(left, bottom-buttonBarHeight, right, bottom)
	l.Keypad = image.Rect(left, l.ButtonBar.Min.Y-keypadHeight, right, l.ButtonBar.Min.Y)
	l.Entry = image.Rect(left, l.Keypad.Min.Y-entryHeight, right, l.Keypad.Min.Y)
	l.History = image.Rect(left, l.StatusBar.Max.Y, right, l.Entry.Min.Y)
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
	drawCenteredString(l.StatusBar, text, 38)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawHistory renders past guesses (most recent at the bottom). Each row shows
// the guessed digits on the left and the sorted word feedback on the right.
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
	y := l.History.Min.Y + 10
	for i := start; i < n; i++ {
		g := s.Guesses[i]
		row := image.Rect(l.History.Min.X+40, y, l.History.Max.X-40, y+rowH-10)
		drawGuessRow(row, g, f)
		y += rowH
	}
}

// drawGuessRow lays out one guess: the digits on the left, then the sorted word
// feedback ("Fermi Pico …" or "Bagels") right-aligned.
func drawGuessRow(row image.Rectangle, g game.Guess, f *Fonts) {
	// Digits on the left.
	f.Row.SetActive(ink.Black)
	dx := row.Min.X
	for _, d := range g.Code {
		cell := image.Rect(dx, row.Min.Y, dx+56, row.Max.Y)
		drawCenteredString(cell, itoa(d), 44)
		dx += 60
	}
	// Word feedback, right-aligned.
	f.Word.SetActive(ink.Black)
	words := strings.Join(g.Feedback(), " ")
	ww := ink.StringWidth(words)
	wx := row.Max.X - ww
	if wx < dx+20 {
		wx = dx + 20
	}
	wy := row.Min.Y + (row.Dy()-34)/2
	ink.DrawString(image.Pt(wx, wy), words)
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
		avail := s.DigitAvailable(d) && !s.Over()
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

	title := ink.OpenFont(ink.DefaultFontBold, 60, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Bagels")
	ink.DrawString(image.Pt((screen.X-tw)/2, 70), "Bagels")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 34, true)
	sub.SetActive(ink.Black)
	subT := "Välj svårighetsgrad"
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
	"Mål: lista ut den hemliga koden på så få gissningar som möjligt.",
	"Koden består av olika siffror (inga upprepningar). Tryck siffrorna för att bygga din gissning och tryck sedan Gissa.",
	"Efter varje gissning får du ord som ledtråd:",
	"Fermi: en siffra som är rätt OCH på rätt plats.",
	"Pico: en siffra som är rätt men på fel plats.",
	"Bagels: ingen siffra i gissningen finns i koden.",
	"Orden sorteras, så ordningen avslöjar inte vilken siffra som stämmer. Exempel: koden är 1 4 7, du gissar 1 2 4 → Fermi Pico (utan att säga vilken som är vilken).",
	"Du har ett begränsat antal gissningar. Siffror du redan lagt gråas ut; tryck Sudda för att ta bort den senaste.",
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
// "tap to start" hint.
func DrawSplash(screen image.Point, f *Fonts, title string, motif motifFunc) {
	ink.ClearScreen()

	tf := ink.OpenFont(ink.DefaultFontBold, 72, true)
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

// drawSplashMotif draws a guess row "1 4 7" in boxes with the Fermi/Pico word
// feedback below — the heart of Bagels in simple graphics.
func drawSplashMotif(box image.Rectangle) {
	digits := []string{"1", "4", "7"}
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

	// Word feedback centered below the boxes.
	wordFont := ink.OpenFont(ink.DefaultFont, 44, true)
	wordFont.SetActive(ink.Black)
	words := "Pico Fermi"
	my := y0 + bw + box.Dy()/5
	ww := ink.StringWidth(words)
	ink.DrawString(image.Pt(box.Min.X+(box.Dx()-ww)/2, my), words)
	wordFont.Close()
}
