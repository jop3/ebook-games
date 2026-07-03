package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"jotto/game"
)

// --- Fonts -----------------------------------------------------------------

type Fonts struct {
	Status *ink.Font // status bar text
	Button *ink.Font // action buttons
	Menu   *ink.Font // menu labels
	Tile   *ink.Font // big letters in the guess grid
	Key    *ink.Font // keyboard letters
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 40, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 42, true),
		Tile:   ink.OpenFont(ink.DefaultFontBold, 64, true),
		Key:    ink.OpenFont(ink.DefaultFontBold, 40, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Tile, f.Key} {
		if fn != nil {
			fn.Close()
		}
	}
}

// --- small helpers ---------------------------------------------------------

func pad(r image.Rectangle, n int) image.Rectangle {
	return image.Rect(r.Min.X+n, r.Min.Y+n, r.Max.X-n, r.Max.Y-n)
}

func drawCenteredString(r image.Rectangle, s string, approxHeight int) {
	w := ink.StringWidth(s)
	x := r.Min.X + (r.Dx()-w)/2
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

// upperStr upper-cases a-z and å/ä/ö for display (device font renders these; in
// the emulator they show garbled, which is expected).
func upperStr(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			out = append(out, r-32)
		case r == 'å':
			out = append(out, 'Å')
		case r == 'ä':
			out = append(out, 'Ä')
		case r == 'ö':
			out = append(out, 'Ö')
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

// --- Layout ----------------------------------------------------------------

const (
	statusBarHeight = 100
	buttonBarHeight = 140

	// usableH is the real drawable height on the PB634. ink.ScreenSize()
	// reports 1448, but content below ~1360 wraps to the top of the screen
	// (guide §5). All vertical layout must derive from this, never from
	// ScreenSize().Y directly.
	usableH = 1340

	// safeMargin keeps interactive/label content clear of the screen edges
	// (top/bottom/left/right), matching the bounds-audit safe area.
	safeMargin = 40
)

// Layout partitions the screen into: status bar, guess grid, keyboard, button
// bar. All positions derive from the actual screen size (§5), stacked
// bottom-up from usableH-safeMargin so nothing lands in the edge margins.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	Grid      image.Rectangle
	Keyboard  image.Rectangle
	ButtonBar image.Rectangle
}

func NewLayout(screen image.Point) Layout {
	screen.Y = usableH // real drawable height; ScreenSize().Y (1448) lies (guide §5)
	l := Layout{Screen: image.Rectangle{Max: screen}}

	// Horizontal safe area: keep the keyboard (the widest interactive row)
	// clear of the left/right margins.
	left := safeMargin
	right := screen.X - safeMargin

	// Bottom-up: button bar sits above the bottom margin, inset from the
	// side margins too (its buttons are drawn flush with its own edges).
	l.ButtonBar = image.Rect(left, screen.Y-safeMargin-buttonBarHeight, right, screen.Y-safeMargin)

	// Keyboard: 3 letter rows, sized to fit its own band, stacked above the
	// button bar and inset from the side margins.
	keyRows := 3
	keyRowH := 118
	keyGapV := 16
	kbH := keyRows*keyRowH + (keyRows+1)*keyGapV
	l.Keyboard = image.Rect(left, l.ButtonBar.Min.Y-kbH, right, l.ButtonBar.Min.Y)

	// Status bar starts below the top margin; the grid fills the space
	// between it and the keyboard.
	l.StatusBar = image.Rect(0, safeMargin, screen.X, safeMargin+statusBarHeight)
	l.Grid = image.Rect(0, l.StatusBar.Max.Y, screen.X, l.Keyboard.Min.Y)
	return l
}

// --- Rendering primitives --------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

// Key is a tappable keyboard letter.
type Key struct {
	Rect   image.Rectangle
	Letter rune
}

func (k Key) Hit(p image.Point) bool { return p.In(k.Rect) }

func DrawStatus(l *Layout, text string, f *Fonts) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	drawCenteredString(l.StatusBar, text, 40)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawGrid renders the 6x5 board: past guesses with symbol feedback, the current
// entry row, and empty rows below.
func DrawGrid(l *Layout, s *game.GameState, f *Fonts) {
	ink.FillArea(l.Grid, ink.White)

	rows := game.MaxGuesses
	cols := game.WordLen
	gap := 16

	// Cell size fits both width and the grid's height.
	cellW := (l.Grid.Dx() - gap*(cols+1)) / cols
	cellH := (l.Grid.Dy() - gap*(rows+1)) / rows
	cell := cellW
	if cellH < cell {
		cell = cellH
	}
	if cell > 150 {
		cell = 150
	}

	boardW := cols*cell + (cols-1)*gap
	boardH := rows*cell + (rows-1)*gap
	x0 := l.Grid.Min.X + (l.Grid.Dx()-boardW)/2
	y0 := l.Grid.Min.Y + (l.Grid.Dy()-boardH)/2

	entry := []rune(s.EntryString())
	for r := 0; r < rows; r++ {
		y := y0 + r*(cell+gap)
		for c := 0; c < cols; c++ {
			x := x0 + c*(cell+gap)
			rect := image.Rect(x, y, x+cell, y+cell)

			if r < len(s.Guesses) {
				g := s.Guesses[r]
				gw := []rune(g.Word)
				drawFeedbackTile(rect, gw[c], g.Statuses[c], f)
			} else if r == len(s.Guesses) && !s.Over {
				// current entry row
				ink.DrawRect(rect, ink.Black)
				ink.DrawRect(pad(rect, 1), ink.Black)
				if c < len(entry) {
					f.Tile.SetActive(ink.Black)
					drawCenteredString(rect, string(toUpperRune(entry[c])), 64)
				}
			} else {
				// empty future row
				ink.DrawRect(rect, ink.LightGray)
			}
		}
	}
}

// drawFeedbackTile draws one scored letter using symbols (no colour):
//   - Correct: solid black tile, white letter.
//   - Present: white tile with a bold ring (double border), black letter.
//   - Absent:  light-grey outline, grey letter.
func drawFeedbackTile(rect image.Rectangle, letter rune, st game.Status, f *Fonts) {
	up := string(toUpperRune(letter))
	switch st {
	case game.Correct:
		ink.FillArea(rect, ink.Black)
		f.Tile.SetActive(ink.White)
		drawCenteredString(rect, up, 64)
	case game.Present:
		// bold ring: outer black rect + a second inset border to read as a ring
		ink.DrawRect(rect, ink.Black)
		ink.DrawRect(pad(rect, 1), ink.Black)
		ink.DrawRect(pad(rect, 2), ink.Black)
		ink.DrawRect(pad(rect, 10), ink.Black)
		f.Tile.SetActive(ink.Black)
		drawCenteredString(rect, up, 64)
	default: // Absent
		ink.DrawRect(rect, ink.LightGray)
		f.Tile.SetActive(ink.LightGray)
		drawCenteredString(rect, up, 64)
	}
}

func toUpperRune(r rune) rune {
	switch {
	case r >= 'a' && r <= 'z':
		return r - 32
	case r == 'å':
		return 'Å'
	case r == 'ä':
		return 'Ä'
	case r == 'ö':
		return 'Ö'
	}
	return r
}

// keyboardRows defines the on-screen QWERTY-ish Swedish layout (A-Ö split into
// three rows). The device renders å/ä/ö correctly; the emulator garbles them.
var keyboardRows = [][]rune{
	[]rune("qwertyuiopå"),
	[]rune("asdfghjklöä"),
	[]rune("zxcvbnm"),
}

// DrawKeyboard renders the letter keys and returns their hit rects. Keys already
// known to be absent are greyed; correct/present keys get a marker. Greyed keys
// are still tappable (the player may still want them), matching Wordle where the
// keyboard is a hint, not a lock.
func DrawKeyboard(l *Layout, s *game.GameState, f *Fonts) []Key {
	ink.FillArea(l.Keyboard, ink.White)
	ink.DrawLine(image.Pt(l.Keyboard.Min.X, l.Keyboard.Min.Y),
		image.Pt(l.Keyboard.Max.X, l.Keyboard.Min.Y), ink.Black)

	gapV := 16
	gapH := 12
	rowH := 118
	keys := make([]Key, 0, 30)

	for ri, row := range keyboardRows {
		n := len(row)
		// key width from the widest row (row 0/1 have 11) so columns align.
		maxCols := 11
		keyW := (l.Keyboard.Dx() - gapH*(maxCols+1)) / maxCols
		rowW := n*keyW + (n-1)*gapH
		x0 := l.Keyboard.Min.X + (l.Keyboard.Dx()-rowW)/2
		y0 := l.Keyboard.Min.Y + gapV + ri*(rowH+gapV)

		for ci, letter := range row {
			x := x0 + ci*(keyW+gapH)
			rect := image.Rect(x, y0, x+keyW, y0+rowH)
			drawKey(rect, letter, s, f)
			keys = append(keys, Key{Rect: rect, Letter: letter})
		}
	}
	return keys
}

func drawKey(rect image.Rectangle, letter rune, s *game.GameState, f *Fonts) {
	st, seen := s.LetterStatus(letter)
	up := string(toUpperRune(letter))
	if seen && st == game.Absent {
		// excluded letter: light grey box + grey glyph
		ink.DrawRect(rect, ink.LightGray)
		f.Key.SetActive(ink.LightGray)
		drawCenteredString(rect, up, 40)
		return
	}
	ink.DrawRect(rect, ink.Black)
	ink.DrawRect(pad(rect, 1), ink.Black)
	f.Key.SetActive(ink.Black)
	drawCenteredString(rect, up, 40)
	if seen && st == game.Correct {
		// small filled corner marker to flag a confirmed-correct letter
		m := image.Rect(rect.Max.X-18, rect.Min.Y+4, rect.Max.X-4, rect.Min.Y+18)
		ink.FillArea(m, ink.Black)
	} else if seen && st == game.Present {
		m := image.Rect(rect.Max.X-18, rect.Min.Y+4, rect.Max.X-4, rect.Min.Y+18)
		ink.DrawRect(m, ink.Black)
	}
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
		drawCenteredString(r, label, 38)
		buttons[i] = Button{Rect: r, Label: label}
	}
	return buttons
}

// --- Menu ------------------------------------------------------------------

type Menu struct {
	playBtn  image.Rectangle
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{} }

func (m *Menu) PlayButton() image.Rectangle  { return m.playBtn }
func (m *Menu) RulesButton() image.Rectangle { return m.rulesBtn }

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()

	title := ink.OpenFont(ink.DefaultFontBold, 80, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Jotto")
	ink.DrawString(image.Pt((screen.X-tw)/2, screen.Y/6), "Jotto")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 36, true)
	sub.SetActive(ink.Black)
	subT := "Gissa det hemliga ordet"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, screen.Y/6+120), subT)
	sub.Close()

	margin := 60
	rowW := screen.X - 2*margin
	bw := rowW / 2
	y := screen.Y/2 + 40

	f.Menu.SetActive(ink.Black)
	m.playBtn = image.Rect((screen.X-bw)/2, y, (screen.X+bw)/2, y+130)
	ink.DrawRect(m.playBtn, ink.Black)
	ink.DrawRect(pad(m.playBtn, 1), ink.Black)
	drawCenteredString(m.playBtn, "Spela", 42)

	y += 180
	m.rulesBtn = image.Rect((screen.X-bw)/2, y, (screen.X+bw)/2, y+130)
	ink.DrawRect(m.rulesBtn, ink.Black)
	ink.DrawRect(pad(m.rulesBtn, 1), ink.Black)
	drawCenteredString(m.rulesBtn, "Regler", 42)
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
	"Mål: lista ut det hemliga svenska fembokstavsordet på högst sex gissningar.",
	"Skriv ett ord på tangentbordet och tryck Gissa. Gissningen måste vara ett riktigt ord i ordlistan, annars avvisas den.",
	"Efter varje gissning färgas varje bokstav:",
	"Fylld svart ruta: rätt bokstav på rätt plats.",
	"Ruta med ring runt bokstaven: bokstaven finns i ordet men på fel plats.",
	"Ljusgrå ruta: bokstaven finns inte i ordet.",
	"Dubbletter räknas som i Wordle: varje förekomst i facit kan bara markeras en gång.",
	"Bokstäver som visat sig saknas gråas ut på tangentbordet. Tryck Sudda för att ta bort senaste bokstaven.",
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

// drawSplashMotif draws a column of 4 guess tiles spelling J O T T O, each in a
// different feedback style (solid, ring, empty) — the game's iconography.
func drawSplashMotif(box image.Rectangle) {
	letters := []rune{'J', 'O', 'T', 'O'}
	styles := []game.Status{game.Correct, game.Present, game.Absent, game.Correct}
	n := len(letters)
	gap := box.Dx() / 12
	cell := (box.Dx() - gap*(n-1)) / n
	if cell > box.Dy() {
		cell = box.Dy()
	}
	totalW := n*cell + (n-1)*gap
	x0 := box.Min.X + (box.Dx()-totalW)/2
	y0 := box.Min.Y + (box.Dy()-cell)/2

	mf := ink.OpenFont(ink.DefaultFontBold, cell*3/5, true)
	for i := 0; i < n; i++ {
		rect := image.Rect(x0+i*(cell+gap), y0, x0+i*(cell+gap)+cell, y0+cell)
		up := string(letters[i])
		switch styles[i] {
		case game.Correct:
			ink.FillArea(rect, ink.Black)
			mf.SetActive(ink.White)
		case game.Present:
			ink.DrawRect(rect, ink.Black)
			ink.DrawRect(pad(rect, 1), ink.Black)
			ink.DrawRect(pad(rect, 2), ink.Black)
			ink.DrawRect(pad(rect, 10), ink.Black)
			mf.SetActive(ink.Black)
		default:
			ink.DrawRect(rect, ink.LightGray)
			mf.SetActive(ink.LightGray)
		}
		drawCenteredString(rect, up, cell*3/5)
	}
	mf.Close()
}
