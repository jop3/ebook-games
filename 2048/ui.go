package main

import (
	"image"
	"image/color"
	"strconv"

	ink "github.com/dennwc/inkview"

	"twenty48/game"
)

// Fonts held open for the app lifetime (opened once — never per draw).
type Fonts struct {
	Title   *ink.Font
	Status  *ink.Font
	Button  *ink.Font
	Menu    *ink.Font
	Small   *ink.Font
	TileBig *ink.Font // 1-2 digit tile values
	TileMed *ink.Font // 3 digit
	TileSml *ink.Font // 4+ digit
	Banner  *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Title:   ink.OpenFont(ink.DefaultFontBold, 80, true),
		Status:  ink.OpenFont(ink.DefaultFontBold, 38, true),
		Button:  ink.OpenFont(ink.DefaultFontBold, 38, true),
		Menu:    ink.OpenFont(ink.DefaultFont, 40, true),
		Small:   ink.OpenFont(ink.DefaultFont, 28, true),
		TileBig: ink.OpenFont(ink.DefaultFontBold, 64, true),
		TileMed: ink.OpenFont(ink.DefaultFontBold, 50, true),
		TileSml: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Banner:  ink.OpenFont(ink.DefaultFontBold, 48, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Title, f.Status, f.Button, f.Menu, f.Small, f.TileBig, f.TileMed, f.TileSml, f.Banner} {
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

func itoa(n int) string { return strconv.Itoa(n) }

// --- Layout ------------------------------------------------------------------

const (
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	statusBarHeight = 96
	buttonBarHeight = 140
	arrowBarHeight  = 110
	boardMargin     = 24
	topMargin       = 40
)

// Layout maps between screen pixels and the 4x4 board, plus the tap-arrow bar.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	ArrowBar  image.Rectangle
	ButtonBar image.Rectangle

	BoardArea  image.Rectangle
	GridOrigin image.Point
	CellSize   int

	arrows [4]arrowBtn // Left, Right, Up, Down
}

type arrowBtn struct {
	rect  image.Rectangle
	dir   game.Dir
	label string
}

func NewLayout(screen image.Point) Layout {
	H := usableH
	l := Layout{Screen: image.Rect(0, 0, screen.X, H)}
	l.StatusBar = image.Rect(0, topMargin, screen.X, topMargin+statusBarHeight)
	l.ButtonBar = image.Rect(0, H-topMargin-buttonBarHeight, screen.X, H-topMargin)
	l.ArrowBar = image.Rect(0, l.ButtonBar.Min.Y-boardMargin-arrowBarHeight, screen.X, l.ButtonBar.Min.Y-boardMargin)

	avail := image.Rect(boardMargin, l.StatusBar.Max.Y+boardMargin,
		screen.X-boardMargin, l.ArrowBar.Min.Y-boardMargin)
	side := avail.Dx()
	if avail.Dy() < side {
		side = avail.Dy()
	}
	cell := side / game.Size
	if cell < 1 {
		cell = 1
	}
	l.CellSize = cell
	boardPx := cell * game.Size
	l.GridOrigin = image.Pt(
		avail.Min.X+(avail.Dx()-boardPx)/2,
		avail.Min.Y+(avail.Dy()-boardPx)/2,
	)
	l.BoardArea = image.Rect(l.GridOrigin.X, l.GridOrigin.Y,
		l.GridOrigin.X+boardPx, l.GridOrigin.Y+boardPx)

	// Arrow bar: 4 equal buttons, "Vänster" / ▲ / ▼ / "Höger" — plain words for
	// left/right since ◄► glyphs render as a broken box on-device (guide §5a),
	// while ▲▼ do render.
	labels := []struct {
		label string
		dir   game.Dir
	}{
		{"Vänster", game.Left},
		{"▲", game.Up},
		{"▼", game.Down},
		{"Höger", game.Right},
	}
	gap := 16
	sideMargin := 24
	usableW := l.ArrowBar.Dx() - 2*sideMargin
	n := len(labels)
	bw := (usableW - gap*(n+1)) / n
	for i, lb := range labels {
		x0 := l.ArrowBar.Min.X + sideMargin + gap + i*(bw+gap)
		r := image.Rect(x0, l.ArrowBar.Min.Y, x0+bw, l.ArrowBar.Max.Y)
		l.arrows[i] = arrowBtn{rect: r, dir: lb.dir, label: lb.label}
	}
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

// ArrowHit returns the direction of the tap-arrow button hit at p, if any.
func (l *Layout) ArrowHit(p image.Point) (game.Dir, bool) {
	for _, a := range l.arrows {
		if p.In(a.rect) {
			return a.dir, true
		}
	}
	return 0, false
}

// --- Rendering -----------------------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

func DrawStatus(l *Layout, s *game.GameState, f *Fonts) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	text := "Poäng: " + itoa(s.Score) + "    Bästa: " + itoa(s.Best)
	drawCenteredString(l.StatusBar, text, 38)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawBoard renders the 4x4 grid and every tile's value.
func DrawBoard(l *Layout, b game.Board, f *Fonts) {
	ink.DrawRect(l.BoardArea, ink.Black)
	ink.DrawRect(pad(l.BoardArea, 1), ink.Black)
	for i := 1; i < game.Size; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.BoardArea.Min.Y), image.Pt(px, l.BoardArea.Max.Y), ink.Black)
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, py), image.Pt(l.BoardArea.Max.X, py), ink.Black)
	}

	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			v := b.At(x, y)
			if v == 0 {
				continue
			}
			cell := pad(l.CellToScreen(x, y), 4)
			drawTile(cell, v, f)
		}
	}
}

// drawTile fills a tile's cell with a grey level bucketed by magnitude (the
// number is always the primary distinguisher; the fill is a secondary,
// greyscale-only hint) and draws its value centered, sized to fit the cell.
func drawTile(cell image.Rectangle, v int, f *Fonts) {
	var fill color.Color
	var textColor color.Color
	switch {
	case v < 8:
		fill, textColor = ink.White, ink.Black
	case v < 64:
		fill, textColor = ink.LightGray, ink.Black
	case v < 512:
		fill, textColor = ink.DarkGray, ink.Black
	default:
		fill, textColor = ink.Black, ink.White
	}
	ink.FillArea(cell, fill)
	ink.DrawRect(cell, ink.Black)

	s := itoa(v)
	font, h := f.TileBig, 64
	if len(s) == 3 {
		font, h = f.TileMed, 50
	} else if len(s) >= 4 {
		font, h = f.TileSml, 38
	}
	font.SetActive(textColor)
	drawCenteredString(cell, s, h)
}

// DrawArrowBar renders the four swipe-fallback buttons.
func DrawArrowBar(l *Layout, f *Fonts) {
	f.Button.SetActive(ink.Black)
	for _, a := range l.arrows {
		ink.DrawRect(a.rect, ink.Black)
		ink.DrawRect(pad(a.rect, 1), ink.Black)
		drawCenteredString(a.rect, a.label, 38)
	}
}

func DrawButtonBar(l *Layout, labels []string, f *Fonts) []Button {
	DrawArrowBar(l, f)

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
		drawCenteredString(r, label, 38)
		buttons[i] = Button{Rect: r, Label: label}
	}
	return buttons
}

// DrawBanner draws a translucent-look (inverted-border) banner across the
// middle of the board announcing win/game-over.
func DrawBanner(l *Layout, f *Fonts, text string) {
	bw := l.BoardArea.Dx() - 40
	bh := 140
	cy := (l.BoardArea.Min.Y + l.BoardArea.Max.Y) / 2
	r := image.Rect(l.BoardArea.Min.X+20, cy-bh/2, l.BoardArea.Min.X+20+bw, cy+bh/2)
	ink.FillArea(r, ink.White)
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 3), ink.Black)
	f.Banner.SetActive(ink.Black)
	drawCenteredString(r, text, 48)
}

// --- Menu ----------------------------------------------------------------------

type menuChoice struct {
	target int
	label  string
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

var choices = []menuChoice{
	{2048, "Spela (mål: 2048)"},
	{1024, "Kortare spel (mål: 1024)"},
	{4096, "Längre spel (mål: 4096)"},
}

func (m *Menu) Draw(screen image.Point, f *Fonts, best int) {
	ink.ClearScreen()
	H := usableH

	f.Title.SetActive(ink.Black)
	tw := ink.StringWidth("2048")
	ink.DrawString(image.Pt((screen.X-tw)/2, 70), "2048")

	sub := "Bästa poäng: " + itoa(best)
	f.Small.SetActive(ink.Black)
	sw := ink.StringWidth(sub)
	ink.DrawString(image.Pt((screen.X-sw)/2, 190), sub)

	margin := 60
	rowW := screen.X - 2*margin
	rbW := rowW / 2
	rbH := 100
	rb := image.Rect((screen.X-rbW)/2, H-margin-rbH, (screen.X+rbW)/2, H-margin)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	f.Button.SetActive(ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	f.Menu.SetActive(ink.Black)
	rowH := 130
	top := 280
	bottom := rb.Min.Y - 30
	n := len(choices)
	avail := bottom - top
	if avail < rowH*n {
		rowH = avail / n
	}
	y := top

	m.rows = m.rows[:0]
	for _, c := range choices {
		r := image.Rect(margin, y, margin+rowW, y+rowH-20)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawCenteredString(r, c.label, 40)
		m.rows = append(m.rows, menuRow{rect: r, choice: c})
		y += rowH
	}
}

func (m *Menu) HandleTouch(p image.Point) (int, bool) {
	for i := range m.rows {
		if p.In(m.rows[i].rect) {
			return m.rows[i].choice.target, true
		}
	}
	return 0, false
}

func (m *Menu) RulesButton() image.Rectangle { return m.rulesBtn }

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
	"Mål: skapa en bricka med värdet 2048 genom att slå ihop brickor.",
	"Svep upp, ner, vänster eller höger (eller tryck på pilarna) för att flytta alla brickor så långt det går åt det hållet.",
	"När två brickor med samma värde möts slås de ihop till en bricka med det dubbla värdet. Varje bricka kan bara slås ihop en gång per drag.",
	"Efter varje drag som ändrar brädet dyker en ny bricka (oftast en 2:a, ibland en 4:a) upp på en slumpmässig tom ruta. Ett svep som inte ändrar något räknas inte som ett drag.",
	"Poängen ökar med värdet av varje ny sammanslagen bricka. Din bästa poäng sparas mellan omgångarna.",
	"Spelet är slut när brädet är fullt och inga två intilliggande brickor har samma värde, så att inget drag längre är möjligt.",
	"När du når 2048 kan du välja att fortsätta spela mot en högre poäng.",
}

func DrawRules(screen image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	ink.ClearScreen()
	H := usableH

	f.Title.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, 60), title)

	margin := 40
	bh := 110
	bw := screen.X / 2
	r := image.Rect((screen.X-bw)/2, H-margin-bh, (screen.X+bw)/2, H-margin)
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 1), ink.Black)
	f.Button.SetActive(ink.Black)
	drawCenteredString(r, "Tillbaka", 38)

	f.Small.SetActive(ink.Black)
	bodyMargin := 60
	maxW := screen.X - 2*bodyMargin
	y := 180
	lineH := 40
	paraGap := 20
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

	return r
}

// --- Splash screen -----------------------------------------------------------

type motifFunc func(box image.Rectangle)

func DrawSplash(screen image.Point, f *Fonts, title string, motif motifFunc) {
	ink.ClearScreen()
	H := usableH

	f.Title.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, H/6), title)

	side := screen.X * 3 / 5
	box := image.Rect((screen.X-side)/2, (H-side)/2,
		(screen.X+side)/2, (H+side)/2)
	f.TileBig.SetActive(ink.Black)
	motif(box)

	f.Small.SetActive(ink.DarkGray)
	ht := "Tryck för att börja"
	hw := ink.StringWidth(ht)
	ink.DrawString(image.Pt((screen.X-hw)/2, H*5/6), ht)
}

// drawSplashMotif draws a 2x2 of merging tiles: 2, 4, 8, 16 — the splash
// pattern the spec calls for.
func drawSplashMotif(box image.Rectangle) {
	cell := box.Dx() / 2
	values := [4]int{2, 4, 8, 16}
	for i, v := range values {
		row, col := i/2, i%2
		r := image.Rect(box.Min.X+col*cell, box.Min.Y+row*cell,
			box.Min.X+(col+1)*cell, box.Min.Y+(row+1)*cell)
		r = pad(r, cell/12)
		ink.FillArea(r, ink.LightGray)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		s := itoa(v)
		w := ink.StringWidth(s)
		x := r.Min.X + (r.Dx()-w)/2
		y := r.Min.Y + (r.Dy()-64)/2
		ink.DrawString(image.Pt(x, y), s)
	}
}
