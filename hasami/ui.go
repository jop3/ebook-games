package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"hasami/game"
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
		Status: ink.OpenFont(ink.DefaultFontBold, 36, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 38, true),
		Small:  ink.OpenFont(ink.DefaultFont, 28, true),
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

// --- Layout ----------------------------------------------------------------

const (
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	statusBarHeight = 96
	buttonBarHeight = 140
	boardMargin     = 16
	topMargin       = 40
)

// Layout maps between screen pixels and the 9x9 board.
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

// --- Rendering -------------------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

func DrawStatus(l *Layout, text string, f *Fonts) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	drawCenteredString(l.StatusBar, text, 36)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawBoard renders the grid, men, the current selection (if any), its legal
// destinations, and a brief marker over cells captured by the last move.
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

	// Legal destination hints for a selected man.
	if a.hasSelection {
		for _, to := range s.Board.DestinationsFrom(a.selected) {
			cell := l.CellToScreen(to.X, to.Y)
			cx, cy := (cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2
			r := l.CellSize / 10
			if r < 2 {
				r = 2
			}
			ink.FillArea(image.Rect(cx-r, cy-r, cx+r, cy+r), ink.LightGray)
		}
	}

	// Men. Black = filled disc; White = hollow (ring) disc.
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			switch s.Board.At(x, y) {
			case game.Black:
				drawMan(l.CellToScreen(x, y), true)
			case game.White:
				drawMan(l.CellToScreen(x, y), false)
			}
		}
	}

	// Selection highlight: a bold border around the selected man's square.
	if a.hasSelection {
		r := l.CellToScreen(a.selected.X, a.selected.Y)
		ink.DrawRect(pad(r, 2), ink.Black)
		ink.DrawRect(pad(r, 3), ink.Black)
	}

	// Briefly mark cells captured by the most recent move (they are already
	// empty; the marker vanishes on its own once the next move overwrites
	// LastCaptured, so there is no separate timer to manage).
	for _, p := range s.LastCaptured {
		drawCaptureMark(l.CellToScreen(p.X, p.Y))
	}
}

// drawMan draws a man inside a cell. black=true renders a solid disc; false
// renders a hollow (white) disc with a black outline so both read on e-ink.
func drawMan(cell image.Rectangle, black bool) {
	r := pad(cell, cell.Dx()/8)
	fillDisc(r, ink.Black)
	if !black {
		inner := pad(r, r.Dx()/8+2)
		fillDisc(inner, ink.White)
	}
}

// drawCaptureMark overlays a diagonal cross on a (now-empty) cell to briefly
// flag it as just-captured — e-ink has no real animation, so a single-frame
// distinct marker stands in for one.
func drawCaptureMark(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/4)
	ink.DrawLine(image.Pt(r.Min.X, r.Min.Y), image.Pt(r.Max.X, r.Max.Y), ink.DarkGray)
	ink.DrawLine(image.Pt(r.Max.X, r.Min.Y), image.Pt(r.Min.X, r.Max.Y), ink.DarkGray)
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
	aiDepth  int
	label    string
}

type menuRow struct {
	rect   image.Rectangle
	choice menuChoice
}

// opponentChoices lists the opponent options offered on the start screen.
var opponentChoices = []menuChoice{
	{game.OpponentHotseat, 0, "2 spelare (hot-seat)"},
	{game.OpponentAI, game.DepthEasy, "Mot dator – Lätt"},
	{game.OpponentAI, game.DepthMedium, "Mot dator – Medel"},
	{game.OpponentAI, game.DepthHard, "Mot dator – Svår"},
}

type Menu struct {
	winMode game.WinMode

	modeBtns [2]image.Rectangle // [0]=Fångst, [1]=Fem i rad
	rows     []menuRow
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{winMode: game.ModeCapture} }

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	H := usableH

	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Hasami")
	ink.DrawString(image.Pt((screen.X-tw)/2, 56), "Hasami")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 30, true)
	sub.SetActive(ink.Black)
	subT := "Välj läge"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 140), subT)
	sub.Close()

	// Win-mode toggle: two buttons side by side; the selected one is drawn
	// with a bold (double) border.
	margin := 60
	rowW := screen.X - 2*margin
	toggleY := 190
	toggleH := 84
	gap := 20
	btnW := (rowW - gap) / 2
	labels := [2]string{"Fångst", "Fem i rad"}
	f.Menu.SetActive(ink.Black)
	for i := 0; i < 2; i++ {
		x0 := margin + i*(btnW+gap)
		r := image.Rect(x0, toggleY, x0+btnW, toggleY+toggleH)
		ink.DrawRect(r, ink.Black)
		selected := (i == 0 && m.winMode == game.ModeCapture) || (i == 1 && m.winMode == game.ModeFiveInRow)
		if selected {
			ink.DrawRect(pad(r, 3), ink.Black)
			ink.DrawRect(pad(r, 4), ink.Black)
		}
		drawCenteredString(r, labels[i], 36)
		m.modeBtns[i] = r
	}

	// Bottom-anchored "Regler" button, stacked up from H-margin.
	rbW := rowW / 2
	rbH := 100
	rb := image.Rect((screen.X-rbW)/2, H-margin-rbH, (screen.X+rbW)/2, H-margin)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	// Opponent rows fill the space between the toggle and the Regler button.
	rowH := 116
	top := toggleY + toggleH + 30
	bottom := rb.Min.Y - 30
	n := len(opponentChoices)
	avail := bottom - top
	if avail < rowH*n {
		rowH = avail / n
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

// TapModeToggle handles a tap on the Fångst/Fem i rad toggle. Returns true
// (and updates the selected mode) if the tap hit one of the two buttons.
func (m *Menu) TapModeToggle(p image.Point) bool {
	if p.In(m.modeBtns[0]) {
		m.winMode = game.ModeCapture
		return true
	}
	if p.In(m.modeBtns[1]) {
		m.winMode = game.ModeFiveInRow
		return true
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

// --- Rules screen ----------------------------------------------------------

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

// rulesParagraphs is the rules text for Hasami, one entry per paragraph.
var rulesParagraphs = []string{
	"Mål (Fångst): reducera motståndaren till en enda man.",
	"Brädet är 9x9. Varje spelare har 9 man på sin närmsta rad — Svart längst ner, Vit längst upp. Svart börjar.",
	"En man flyttar som ett torn: hur långt som helst rakt (vågrätt eller lodrätt), aldrig diagonalt, och aldrig genom en annan man.",
	"Fångst: bildar din flytt en sammanhängande rad av en eller flera fiendemän, inklämd mellan två av dina egna, tas hela raden bort. En enda flytt kan fånga i flera riktningar samtidigt.",
	"Hörnfångst: en fiendeman i ett hörn tas när du besätter de två rutor som ligger jämte det hörnet.",
	"Säkert att gå in: att flytta din egen man till luckan mellan två fiendemän är INTE självfångst — fångst sker bara på den som just flyttat, aldrig på den egna, nyss flyttade mannen.",
	"Valfritt läge, Fem i rad: först att bilda en obruten rad av 5 egna man, vågrätt eller lodrätt, någonstans utanför sin egen startrad, vinner.",
	"Tryck på en egen man för att välja den — lediga rutor den kan nå markeras. Tryck sedan på en av dem för att flytta dit. Tryck på samma man igen för att avmarkera.",
	"Hasami shogi (\"sax-shogi\") är ett traditionellt japanskt brädspel, känt sedan mycket lång tid tillbaka.",
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

// --- Splash screen ---------------------------------------------------------

// motifFunc draws the game's line-art centered in the given box.
type motifFunc func(box image.Rectangle)

// DrawSplash renders the start screen: the game title, a large line-art motif,
// and a "tap to start" hint — echoing the built-in chess app's opening screen.
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

// drawSplashMotif draws the game's sandwich-capture idea as a short row of
// three men — solid, ring, solid (● ○ ●) — with small inward arrows above
// showing the two Black men about to bracket the White one between them.
func drawSplashMotif(box image.Rectangle) {
	cy := (box.Min.Y + box.Max.Y) / 2
	cx := (box.Min.X + box.Max.X) / 2
	r := box.Dx() / 8
	gap := r * 3
	centers := [3]image.Point{{cx - gap, cy}, {cx, cy}, {cx + gap, cy}}
	solid := [3]bool{true, false, true}
	for i, c := range centers {
		cell := image.Rect(c.X-r, c.Y-r, c.X+r, c.Y+r)
		if solid[i] {
			fillDisc(cell, ink.Black)
		} else {
			fillDisc(cell, ink.Black)
			fillDisc(pad(cell, r/4+3), ink.White)
		}
	}
	// Small inward-pointing arrows above the two Black men, indicating the
	// squeeze that captures the White man between them.
	arrowY := cy - r*2
	drawInwardArrow(image.Pt(cx-gap, arrowY), true, r)
	drawInwardArrow(image.Pt(cx+gap, arrowY), false, r)
}

// drawInwardArrow draws a small chevron ("›" if pointRight, "‹" otherwise)
// centered at c, its point offset toward the pointing direction and its two
// tails splayed out behind it — used above the splash motif's outer men to
// show them squeezing inward on the man between them.
func drawInwardArrow(c image.Point, pointRight bool, size int) {
	dx := size
	if !pointRight {
		dx = -dx
	}
	tip := image.Pt(c.X+dx/2, c.Y)
	tailA := image.Pt(c.X-dx/2, c.Y-size/2)
	tailB := image.Pt(c.X-dx/2, c.Y+size/2)
	ink.DrawLine(tip, tailA, ink.Black)
	ink.DrawLine(tip, tailB, ink.Black)
}
