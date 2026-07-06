package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"chomp/game"
)

// Fonts held open for the app lifetime (opened once — never per draw).
type Fonts struct {
	Status *ink.Font
	Hint   *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Small  *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 36, true),
		Hint:   ink.OpenFont(ink.DefaultFont, 28, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 36, true),
		Small:  ink.OpenFont(ink.DefaultFont, 28, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Hint, f.Button, f.Menu, f.Small} {
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

// --- Layout ----------------------------------------------------------------

const (
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	statusBarHeight = 130 // two lines: turn/winner + the AI-honesty hint
	buttonBarHeight = 140
	boardMargin     = 16
	topMargin       = 40
)

// Layout maps between screen pixels and the current game's Rows x Cols grid.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	ButtonBar image.Rectangle

	BoardArea  image.Rectangle
	GridOrigin image.Point
	CellSize   int
	Rows, Cols int
}

func NewLayout(screen image.Point, rows, cols int) Layout {
	H := usableH
	l := Layout{Screen: image.Rect(0, 0, screen.X, H), Rows: rows, Cols: cols}
	l.StatusBar = image.Rect(0, topMargin, screen.X, topMargin+statusBarHeight)
	l.ButtonBar = image.Rect(0, H-topMargin-buttonBarHeight, screen.X, H-topMargin)

	avail := image.Rect(boardMargin, l.StatusBar.Max.Y+boardMargin,
		screen.X-boardMargin, l.ButtonBar.Min.Y-boardMargin)
	cellW := avail.Dx() / cols
	cellH := avail.Dy() / rows
	cell := cellW
	if cellH < cell {
		cell = cellH
	}
	if cell < 1 {
		cell = 1
	}
	l.CellSize = cell
	boardW, boardH := cell*cols, cell*rows
	l.GridOrigin = image.Pt(
		avail.Min.X+(avail.Dx()-boardW)/2,
		avail.Min.Y+(avail.Dy()-boardH)/2,
	)
	l.BoardArea = image.Rect(l.GridOrigin.X, l.GridOrigin.Y,
		l.GridOrigin.X+boardW, l.GridOrigin.Y+boardH)
	return l
}

// CellToScreen returns the screen rectangle for board cell (r,c).
func (l *Layout) CellToScreen(r, c int) image.Rectangle {
	return image.Rect(
		l.GridOrigin.X+c*l.CellSize,
		l.GridOrigin.Y+r*l.CellSize,
		l.GridOrigin.X+(c+1)*l.CellSize,
		l.GridOrigin.Y+(r+1)*l.CellSize,
	)
}

// ScreenToCell maps a tapped point back to a (row, col) board cell.
func (l *Layout) ScreenToCell(p image.Point) (r, c int, ok bool) {
	if l.CellSize == 0 {
		return 0, 0, false
	}
	rel := p.Sub(l.GridOrigin)
	if rel.X < 0 || rel.Y < 0 {
		return 0, 0, false
	}
	c = rel.X / l.CellSize
	r = rel.Y / l.CellSize
	if r < 0 || r >= l.Rows || c < 0 || c >= l.Cols {
		return 0, 0, false
	}
	return r, c, true
}

// --- Rendering ---------------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

// playerLabel names a player for on-screen text, adapting to the opponent
// mode: hot-seat says "Spelare 1/2", vs-AI says "Du"/"AI:n".
func playerLabel(gs *game.GameState, p game.Player) string {
	if gs.Opponent == game.OpponentAI {
		if p == game.P0 {
			return "Du"
		}
		return "AI:n"
	}
	if p == game.P0 {
		return "Spelare 1"
	}
	return "Spelare 2"
}

func statusText(gs *game.GameState) string {
	if gs.Phase == game.PhaseDone {
		return playerLabel(gs, gs.Winner) + " vinner!"
	}
	who := playerLabel(gs, gs.Turn)
	if gs.Turn == game.P0 {
		return who + " drar"
	}
	return who + " drar"
}

// honestyHint mirrors nim's solo-mode honesty line: while it's the human's
// turn against the AI, say plainly whether they hold the theoretical win —
// the AI is unbeatable with perfect play, so this is the one place the app
// can still be encouraging without ever bluffing.
func honestyHint(gs *game.GameState) string {
	if gs.Phase != game.PhasePlaying || gs.Opponent != game.OpponentAI || gs.Turn != game.P0 {
		return ""
	}
	if gs.MoverCanWin() {
		return "Läget: du kan vinna med perfekt spel"
	}
	return "Läget: AI:n har ett vinstläge just nu"
}

func DrawStatus(l *Layout, a *app) {
	ink.FillArea(l.StatusBar, ink.White)
	a.fonts.Status.SetActive(ink.Black)
	line1 := image.Rect(l.StatusBar.Min.X, l.StatusBar.Min.Y, l.StatusBar.Max.X, l.StatusBar.Min.Y+70)
	drawCenteredString(line1, statusText(a.gs), 36)
	if hint := honestyHint(a.gs); hint != "" {
		a.fonts.Hint.SetActive(ink.DarkGray)
		line2 := image.Rect(l.StatusBar.Min.X, l.StatusBar.Min.Y+70, l.StatusBar.Max.X, l.StatusBar.Max.Y)
		drawCenteredString(line2, hint, 28)
	}
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawBoard renders every remaining cell as a filled "chocolate square" and
// leaves eaten cells blank — no animation, a direct before/after redraw
// makes the staircase shape visible at a glance. The poisoned cell (0,0),
// while still present, is marked with a small crossbones-style hazard mark.
func DrawBoard(l *Layout, gs *game.GameState) {
	// A light outline of the ORIGINAL rectangle keeps the board's footprint
	// stable across the whole game, even as cells disappear.
	ink.DrawRect(l.BoardArea, ink.LightGray)

	for r := 0; r < l.Rows; r++ {
		for c := 0; c < l.Cols; c++ {
			cell := l.CellToScreen(r, c)
			if !gs.Board.Has(r, c) {
				continue // eaten: leave it blank.
			}
			if r == 0 && c == 0 {
				drawPoisonCell(cell)
				continue
			}
			drawChocolateCell(cell)
		}
	}
}

// drawChocolateCell draws one remaining "piece" of the bar: a filled square
// with a black border and a smaller inset border, evoking a segmented
// chocolate square.
func drawChocolateCell(cell image.Rectangle) {
	inner := pad(cell, cell.Dx()/12+2)
	ink.FillArea(inner, ink.LightGray)
	ink.DrawRect(inner, ink.Black)
	ink.DrawRect(pad(inner, inner.Dx()/6+2), ink.Black)
}

// drawPoisonCell marks the top-left poisoned square: filled dark with a
// simple crossbones-style hazard mark (plain line art only — no font glyph,
// per the guide's warning that non-ASCII symbols can render broken on
// device).
func drawPoisonCell(cell image.Rectangle) {
	ink.FillArea(cell, ink.DarkGray)
	ink.DrawRect(cell, ink.Black)
	r := pad(cell, cell.Dx()/4)
	ink.DrawLine(image.Pt(r.Min.X, r.Min.Y), image.Pt(r.Max.X, r.Max.Y), ink.White)
	ink.DrawLine(image.Pt(r.Max.X, r.Min.Y), image.Pt(r.Min.X, r.Max.Y), ink.White)
	cx, cy := (cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2
	rad := cell.Dx() / 8
	if rad < 3 {
		rad = 3
	}
	fillDisc(image.Rect(cx-rad, cy-rad, cx+rad, cy+rad), ink.White)
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
		drawCenteredString(r, label, 38)
		buttons[i] = Button{Rect: r, Label: label}
	}
	return buttons
}

// --- Menu --------------------------------------------------------------

type menuChoice struct {
	opponent game.Opponent
	label    string
}

type menuRow struct {
	rect   image.Rectangle
	choice menuChoice
}

// opponentChoices lists the opponent options offered on the start screen.
var opponentChoices = []menuChoice{
	{game.OpponentHotseat, "2 spelare (hot-seat)"},
	{game.OpponentAI, "Mot en perfekt AI"},
}

type Menu struct {
	SizeIdx int // index into game.Sizes

	sizeBtns [3]image.Rectangle
	rows     []menuRow
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{SizeIdx: 0} }

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	H := usableH

	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Chomp")
	ink.DrawString(image.Pt((screen.X-tw)/2, 56), "Chomp")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 30, true)
	sub.SetActive(ink.Black)
	subT := "Välj bräda och läge"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 140), subT)
	sub.Close()

	// Board-size toggle: three buttons side by side; the selected one gets a
	// bold (double) border.
	margin := 60
	rowW := screen.X - 2*margin
	toggleY := 190
	toggleH := 84
	gap := 16
	n := len(game.Sizes)
	btnW := (rowW - gap*(n-1)) / n
	f.Menu.SetActive(ink.Black)
	for i, sz := range game.Sizes {
		x0 := margin + i*(btnW+gap)
		r := image.Rect(x0, toggleY, x0+btnW, toggleY+toggleH)
		ink.DrawRect(r, ink.Black)
		if i == m.SizeIdx {
			ink.DrawRect(pad(r, 3), ink.Black)
			ink.DrawRect(pad(r, 4), ink.Black)
		}
		drawCenteredString(r, sz.Name, 36)
		m.sizeBtns[i] = r
	}
	f.Small.SetActive(ink.DarkGray)
	sel := game.Sizes[m.SizeIdx]
	dims := itoa(sel.Rows) + "x" + itoa(sel.Cols) + " bräda — samma perfekta AI oavsett storlek"
	dw := ink.StringWidth(dims)
	ink.DrawString(image.Pt((screen.X-dw)/2, toggleY+toggleH+8), dims)

	// Bottom-anchored "Regler" button, stacked up from H-margin.
	rbW := rowW / 2
	rbH := 100
	rb := image.Rect((screen.X-rbW)/2, H-margin-rbH, (screen.X+rbW)/2, H-margin)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	f.Menu.SetActive(ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	// Opponent rows fill the space between the toggle/hint and the Regler
	// button.
	rowH := 116
	top := toggleY + toggleH + 46
	bottom := rb.Min.Y - 30
	rn := len(opponentChoices)
	avail := bottom - top
	if avail < rowH*rn {
		rowH = avail / rn
	}
	y := top

	m.rows = m.rows[:0]
	for _, c := range opponentChoices {
		r := image.Rect(margin, y, margin+rowW, y+rowH-18)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawLeftString(r, c.label, 38)
		m.rows = append(m.rows, menuRow{rect: r, choice: c})
		y += rowH
	}
}

// TapSizeToggle handles a tap on the Lätt/Medel/Svår toggle. Returns true
// (and updates SizeIdx) if the tap hit one of the buttons.
func (m *Menu) TapSizeToggle(p image.Point) bool {
	for i, r := range m.sizeBtns {
		if p.In(r) {
			m.SizeIdx = i
			return true
		}
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

// wrapText breaks s into lines no wider than maxW pixels for the active font.
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

// splitWords splits on spaces (small helper to avoid importing strings).
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

// rulesParagraphs is the rules text for Chomp, one entry per paragraph.
var rulesParagraphs = []string{
	"Mål: tvinga din motståndare att äta den förgiftade rutan.",
	"Brädet är en rektangel av rutor — en chokladkaka. Rutan längst upp till vänster (rad 0, kolumn 0) är förgiftad.",
	"På din tur väljer du EN kvarvarande ruta och äter den. Alla kvarvarande rutor på SAMMA rad eller längre ner, OCH SAMMA kolumn eller längre åt höger, försvinner också — som att bryta av chokladen snett ner och åt höger från din ruta.",
	"Den som tvingas äta den förgiftade rutan FÖRLORAR direkt. Du kan välja den själv om du vill ge upp.",
	"Bräddstorlek väljs på menyn: Lätt (4x4), Medel (5x6) eller Svår (6x7). Det gör spelet längre och knepigare — inte AI:n svagare, den spelar lika perfekt oavsett storlek.",
	"Spela hot-seat mot en vän, eller mot datorn. AI:n räknar ut alla möjliga spel i förväg och spelar perfekt — går inte att lura, precis som Nims AI. Mot AI:n visar en rad om du just då har ett vinstläge.",
	"Tryck på en kvarvarande ruta för att äta den (och allt den tar med sig). Ingen animation — brädet uppdateras direkt.",
	"Chomp är ett klassiskt matematiskt spel, även kallat \"Chocolate Chomp\", beskrivet av bland andra Fred Schuh och David Gale.",
}

// DrawRules renders the scrolling rules text with a back button and returns
// the back button rect.
func DrawRules(screen image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	ink.ClearScreen()
	H := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 56, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, 50), title)
	tf.Close()

	// Back button, bottom-anchored.
	margin := 40
	bh := 110
	bw := screen.X / 2
	r := image.Rect((screen.X-bw)/2, H-margin-bh, (screen.X+bw)/2, H-margin)
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 1), ink.Black)
	f.Button.SetActive(ink.Black)
	drawCenteredString(r, "Tillbaka", 38)

	body := ink.OpenFont(ink.DefaultFont, 30, true)
	body.SetActive(ink.Black)
	bodyMargin := 50
	maxW := screen.X - 2*bodyMargin
	y := 150
	lineH := 40
	paraGap := 18
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

// --- Splash screen -------------------------------------------------------

// motifFunc draws the game's line-art centered in the given box.
type motifFunc func(box image.Rectangle)

// DrawSplash renders the start screen: the game title, a large line-art
// motif, and a "tap to start" hint — echoing the built-in chess app's
// opening screen.
func DrawSplash(screen image.Point, f *Fonts, title string, motif motifFunc) {
	ink.ClearScreen()
	H := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 80, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, H/6), title)
	tf.Close()

	// Motif occupies a centered box in the middle of the screen.
	w := screen.X * 4 / 5
	h := screen.X * 2 / 5
	box := image.Rect((screen.X-w)/2, (H-h)/2, (screen.X+w)/2, (H+h)/2)
	motif(box)

	hint := ink.OpenFont(ink.DefaultFont, 34, true)
	hint.SetActive(ink.DarkGray)
	ht := "Tryck för att börja"
	hw := ink.StringWidth(ht)
	ink.DrawString(image.Pt((screen.X-hw)/2, H*5/6), ht)
	hint.Close()
}

// drawSplashMotif draws a small chocolate-bar grid with a corner "bite"
// already taken out of it (a staircase shape, exactly the game's own board
// representation) and the poisoned top-left square marked with the same
// crossbones hazard mark used in the real game.
func drawSplashMotif(box image.Rectangle) {
	// A hand-picked staircase shape for the motif: 4 columns wide, 4 rows
	// tall, with a bite taken out of the bottom-right.
	rows := []int{4, 4, 2, 1}
	cols := 4
	cell := box.Dx() / cols
	if h := box.Dy() / len(rows); h < cell {
		cell = h
	}
	boardW, boardH := cell*cols, cell*len(rows)
	origin := image.Pt(box.Min.X+(box.Dx()-boardW)/2, box.Min.Y+(box.Dy()-boardH)/2)

	for r, n := range rows {
		for c := 0; c < cols; c++ {
			x0 := origin.X + c*cell
			y0 := origin.Y + r*cell
			cellRect := image.Rect(x0, y0, x0+cell, y0+cell)
			if c >= n {
				continue // eaten: leave blank, showing the bite/staircase shape.
			}
			if r == 0 && c == 0 {
				drawPoisonCell(cellRect)
				continue
			}
			drawChocolateCell(cellRect)
		}
	}
}
