package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"amazons/game"
)

// Fonts held open for the app lifetime (opened once — never per draw).
type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Small  *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 32, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 36, true),
		Small:  ink.OpenFont(ink.DefaultFont, 26, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Small} {
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
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	statusBarHeight = 96
	buttonBarHeight = 140
	boardMargin     = 12
	topMargin       = 40
)

// Layout maps between screen pixels and the 10x10 board.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	ButtonBar image.Rectangle

	BoardArea  image.Rectangle
	GridOrigin image.Point
	CellSize   int
}

func NewLayout(screen image.Point) Layout {
	H := usableH
	l := Layout{Screen: image.Rect(0, 0, screen.X, H)}
	l.StatusBar = image.Rect(0, topMargin, screen.X, topMargin+statusBarHeight)
	l.ButtonBar = image.Rect(0, H-topMargin-buttonBarHeight, screen.X, H-topMargin)

	avail := image.Rect(boardMargin, l.StatusBar.Max.Y+boardMargin,
		screen.X-boardMargin, l.ButtonBar.Min.Y-boardMargin)
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
	if x < 0 || x >= game.Size || y < 0 || y >= game.Size {
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

// DrawBoard renders the grid, burned squares, queens, the current
// selection/pending queen (if any), and its legal destinations for whichever
// half of the turn ("move" or "shoot") is currently active.
func DrawBoard(l *Layout, a *app) {
	s := a.gs
	// Grid frame + lines.
	ink.DrawRect(l.BoardArea, ink.Black)
	ink.DrawRect(pad(l.BoardArea, 1), ink.Black)
	for i := 1; i < game.Size; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.BoardArea.Min.Y), image.Pt(px, l.BoardArea.Max.Y), ink.Black)
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, py), image.Pt(l.BoardArea.Max.X, py), ink.Black)
	}

	// Burned squares (permanent, hatched blocks) — drawn before the
	// destination hints/queens so they always read as solid obstacles.
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			if s.Board.At(x, y) == game.Burned {
				drawBurned(l.CellToScreen(x, y))
			}
		}
	}

	// Legal destination hints for the active half of the turn.
	if s.Phase == game.PhasePlaying {
		var origin image.Point
		var show bool
		switch s.Step {
		case game.StepMove:
			if a.hasSelection {
				origin, show = a.selected, true
			}
		case game.StepShoot:
			origin, show = s.Pending, true
		}
		if show {
			for _, to := range s.Board.DestinationsFrom(origin) {
				cell := l.CellToScreen(to.X, to.Y)
				cx, cy := (cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2
				r := l.CellSize / 10
				if r < 2 {
					r = 2
				}
				ink.FillArea(image.Rect(cx-r, cy-r, cx+r, cy+r), ink.LightGray)
			}
		}
	}

	// Queens. Black = filled disc; White = hollow (ring) disc.
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			switch s.Board.At(x, y) {
			case game.QueenBlack:
				drawQueen(l.CellToScreen(x, y), true)
			case game.QueenWhite:
				drawQueen(l.CellToScreen(x, y), false)
			}
		}
	}

	// Highlight border: the selected queen (StepMove) or the queen that just
	// moved and is now awaiting its shot (StepShoot) — makes "which action is
	// next" unambiguous.
	if s.Phase == game.PhasePlaying {
		var highlight image.Point
		var show bool
		switch s.Step {
		case game.StepMove:
			highlight, show = a.selected, a.hasSelection
		case game.StepShoot:
			highlight, show = s.Pending, true
		}
		if show {
			r := l.CellToScreen(highlight.X, highlight.Y)
			ink.DrawRect(pad(r, 2), ink.Black)
			ink.DrawRect(pad(r, 3), ink.Black)
		}
	}
}

// drawQueen draws a queen inside a cell. black=true renders a solid disc;
// false renders a hollow (white) disc with a black outline so both read on
// e-ink.
func drawQueen(cell image.Rectangle, black bool) {
	r := pad(cell, cell.Dx()/8)
	fillDisc(r, ink.Black)
	if !black {
		inner := pad(r, r.Dx()/8+2)
		fillDisc(inner, ink.White)
	}
	// A small center mark (crown dot) distinguishes a queen from a plain
	// hasami-style "man" disc used elsewhere in this game library.
	cx, cy := (cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2
	dot := cell.Dx() / 10
	if dot < 2 {
		dot = 2
	}
	if black {
		fillDisc(image.Rect(cx-dot, cy-dot, cx+dot, cy+dot), ink.White)
	} else {
		fillDisc(image.Rect(cx-dot, cy-dot, cx+dot, cy+dot), ink.Black)
	}
}

// drawBurned renders a permanently burned square as a filled, diagonally
// hatched block — visually distinct from an empty cell (plain grid square)
// and from a queen (a disc/ring).
func drawBurned(cell image.Rectangle) {
	ink.FillArea(pad(cell, 1), ink.LightGray)
	n := 4
	for i := -n; i <= n; i++ {
		off := cell.Dx() * i / n
		// Diagonal lines parallel to the falling diagonal, clipped to the
		// cell by construction (each segment runs corner-to-corner-ish
		// within the square, offset along the top/left edges).
		var p1, p2 image.Point
		if off >= 0 {
			p1 = image.Pt(cell.Min.X, cell.Min.Y+off)
			p2 = image.Pt(cell.Max.X-off, cell.Max.Y)
		} else {
			p1 = image.Pt(cell.Min.X-off, cell.Min.Y)
			p2 = image.Pt(cell.Max.X, cell.Max.Y+off)
		}
		ink.DrawLine(p1, p2, ink.DarkGray)
	}
	ink.DrawRect(cell, ink.Black)
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

// --- Menu ------------------------------------------------------------------

type menuChoice struct {
	opponent game.Opponent
	label    string
}

type menuRow struct {
	rect   image.Rectangle
	choice menuChoice
}

// opponentChoices lists the opponent options offered on the start screen.
// Hot-seat is listed FIRST and is the recommended, primary mode — the AI is
// a shallow territory heuristic (see game/ai.go) and is labeled honestly as
// weak/experimental, never as a strong opponent.
var opponentChoices = []menuChoice{
	{game.OpponentHotseat, "2 spelare (hot-seat) – rekommenderas"},
	{game.OpponentAI, "Mot dator (svag, experimentell)"},
}

type Menu struct {
	rows     []menuRow
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{} }

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	H := usableH

	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Amazons")
	ink.DrawString(image.Pt((screen.X-tw)/2, 56), "Amazons")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 30, true)
	sub.SetActive(ink.Black)
	subT := "Välj läge"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 140), subT)
	sub.Close()

	// Bottom-anchored "Regler" button, stacked up from H-margin.
	margin := 60
	rowW := screen.X - 2*margin
	rbW := rowW / 2
	rbH := 100
	rb := image.Rect((screen.X-rbW)/2, H-margin-rbH, (screen.X+rbW)/2, H-margin)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	// Opponent rows fill the space between the subtitle and the Regler
	// button.
	rowH := 140
	top := 210
	bottom := rb.Min.Y - 30
	n := len(opponentChoices)
	avail := bottom - top
	if avail < rowH*n {
		rowH = avail / n
	}
	y := top

	f.Menu.SetActive(ink.Black)
	m.rows = m.rows[:0]
	for _, c := range opponentChoices {
		r := image.Rect(margin, y, margin+rowW, y+rowH-24)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawLeftString(r, c.label, 36)
		m.rows = append(m.rows, menuRow{rect: r, choice: c})
		y += rowH
	}
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

// --- Rules screen ------------------------------------------------------------

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

// rulesParagraphs is the rules text for Amazons, one entry per paragraph.
// Honest about the built-in AI being a weak/exploratory heuristic, not a
// strong player — hot-seat is the recommended way to play.
var rulesParagraphs = []string{
	"Mål: du vinner så fort motståndaren inte längre kan göra ett enda drag på sin tur. Den som gör det sista draget vinner.",
	"Brädet är 10x10. Varje spelare har 4 drottningar: Svart står överst (kolumn 3 och 6 på rad 0, kolumn 0 och 9 på rad 3), Vit står nederst, spegelvänt. Svart börjar.",
	"Ett drag har två steg. Steg 1: flytta en av dina drottningar som en schackdrottning — hur långt som helst rakt eller diagonalt, men aldrig genom en annan drottning eller en bränd ruta.",
	"Steg 2: från din drottnings NYA ruta skjuter du en pil — samma sorts drag (rakt eller diagonalt, hur långt som helst) — till en ledig ruta. Den rutan blir permanent bränd.",
	"En bränd ruta är stängd för alltid, för båda spelarna: ingen drottning kan flytta in på den eller förbi den, och ingen pil kan flyga in på den eller förbi den. Precis som en ockuperad ruta.",
	"Det finns inga fångster i det här spelet — brickor tas aldrig bort. Brädet fylls sakta med bränt land tills en spelare blir helt instängd.",
	"Tryck på en av dina drottningar för att välja den — lediga rutor den kan nå markeras. Tryck på en av dem för att flytta dit. Tryck på samma drottning igen för att avmarkera.",
	"När drottningen har flyttat markeras nu ledig rutor den kan skjuta pilen till FRÅN DEN NYA RUTAN. Tryck på en av dem för att avsluta draget.",
	"Datorns motdrag är en enkel, uttalat SVAG och experimentell 'territorium'-uppskattning (den räknar ut vem som når fler rutor på färre drottningdrag) — inte en djup sökning. Den gör inga misstag med flit, men spela gärna två spelare (hot-seat) i stället för en riktig utmaning.",
	"Amazons uppfanns 1988 av Walter Zamkauskas i Argentina och är ett fritt tillgängligt, akademiskt abstrakt spel utan ägare eller licens.",
}

// DrawRules renders the scrolling rules text with a back button and returns the
// back button rect.
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

	body := ink.OpenFont(ink.DefaultFont, 28, true)
	body.SetActive(ink.Black)
	bodyMargin := 50
	maxW := screen.X - 2*bodyMargin
	y := 150
	lineH := 38
	paraGap := 16
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

// --- Splash screen -----------------------------------------------------------

// motifFunc draws the game's line-art centered in the given box.
type motifFunc func(box image.Rectangle)

// DrawSplash renders the start screen: the game title, a large line-art
// motif, and a "tap to start" hint — echoing the built-in chess app's opening
// screen.
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

// drawSplashMotif draws the game's core idea per the spec: a queen mid-move,
// with a small arrow landing on a hatched "burned" square. Three points
// across the box: (1) a faint ring marking where the queen started, (2) a
// solid queen disc where it lands, connected by an arrow, and (3) a hatched
// burned square further along, connected to (2) by a second arrow — the shot.
func drawSplashMotif(box image.Rectangle) {
	cy := (box.Min.Y + box.Max.Y) / 2
	unit := box.Dx() / 6

	p1 := image.Pt(box.Min.X+unit, cy)          // where the queen started
	p2 := image.Pt(box.Min.X+3*unit, cy-unit/2) // where it lands
	p3 := image.Pt(box.Min.X+5*unit, cy-unit)   // the burned square it shoots

	// (1) Ghost ring: the queen's original square, now empty.
	ringR := unit * 3 / 5
	ring := image.Rect(p1.X-ringR, p1.Y-ringR, p1.X+ringR, p1.Y+ringR)
	ink.DrawRect(ring, ink.DarkGray)

	// Move arrow: p1 -> p2.
	drawArrow(p1, p2)

	// (2) The queen, now standing at its new square.
	qR := unit * 3 / 5
	drawQueen(image.Rect(p2.X-qR, p2.Y-qR, p2.X+qR, p2.Y+qR), true)

	// Shot arrow: p2 -> p3.
	drawArrow(p2, p3)

	// (3) The burned square the arrow lands on.
	bR := unit * 3 / 5
	drawBurned(image.Rect(p3.X-bR, p3.Y-bR, p3.X+bR, p3.Y+bR))
}

// drawArrow draws a straight line from a to b with a small arrowhead at b.
func drawArrow(a, b image.Point) {
	ink.DrawLine(a, b, ink.Black)
	dx, dy := b.X-a.X, b.Y-a.Y
	length := isqrt(dx*dx + dy*dy)
	if length == 0 {
		return
	}
	// Unit vector along a->b, scaled for a small arrowhead.
	ux, uy := dx*24/length, dy*24/length
	// Perpendicular vector.
	px, py := -uy, ux
	tailA := image.Pt(b.X-ux+px/2, b.Y-uy+py/2)
	tailB := image.Pt(b.X-ux-px/2, b.Y-uy-py/2)
	ink.DrawLine(b, tailA, ink.Black)
	ink.DrawLine(b, tailB, ink.Black)
}
