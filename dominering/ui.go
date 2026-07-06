package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"dominering/game"
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
		Status: ink.OpenFont(ink.DefaultFontBold, 34, true),
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

func sideName(s game.Side) string {
	if s == game.V {
		return "Vertikal"
	}
	return "Horisontell"
}

// --- Layout ------------------------------------------------------------------

const (
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	statusBarHeight = 96
	buttonBarHeight = 140
	boardMargin     = 16
	topMargin       = 40
)

// Layout maps between screen pixels and the board (whose size can be 6 or 8,
// chosen on the menu), so — unlike a game with one fixed board size — it
// takes the size explicitly rather than reading a package constant.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	ButtonBar image.Rectangle

	BoardSize  int
	BoardArea  image.Rectangle
	GridOrigin image.Point
	CellSize   int
}

func NewLayout(screen image.Point, boardSize int) Layout {
	H := usableH
	l := Layout{Screen: image.Rect(0, 0, screen.X, H), BoardSize: boardSize}
	l.StatusBar = image.Rect(0, topMargin, screen.X, topMargin+statusBarHeight)
	l.ButtonBar = image.Rect(0, H-topMargin-buttonBarHeight, screen.X, H-topMargin)

	avail := image.Rect(boardMargin, l.StatusBar.Max.Y+boardMargin,
		screen.X-boardMargin, l.ButtonBar.Min.Y-boardMargin)
	side := avail.Dx()
	if avail.Dy() < side {
		side = avail.Dy()
	}
	cell := side / boardSize
	if cell < 1 {
		cell = 1
	}
	l.CellSize = cell
	boardPx := cell * boardSize
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
	if x < 0 || x >= l.BoardSize || y < 0 || y >= l.BoardSize {
		return 0, 0, false
	}
	return x, y, true
}

// --- Rendering ---------------------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

func DrawStatus(l *Layout, text string, f *Fonts) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	drawCenteredString(l.StatusBar, text, 34)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawBoard renders the grid, every domino placed so far (as a distinct
// 1x2 tile with a center dividing line, not just two shaded cells), the
// current selection (if any) and its highlighted ghost-partner cell(s).
func DrawBoard(l *Layout, a *app) {
	s := a.gs
	size := s.Board.Size

	// Grid frame + lines.
	ink.DrawRect(l.BoardArea, ink.Black)
	ink.DrawRect(pad(l.BoardArea, 1), ink.Black)
	for i := 1; i < size; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.BoardArea.Min.Y), image.Pt(px, l.BoardArea.Max.Y), ink.LightGray)
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, py), image.Pt(l.BoardArea.Max.X, py), ink.LightGray)
	}

	// Every domino placed so far, drawn as one tile spanning its two cells.
	for _, m := range s.Moves {
		drawDomino(l.CellToScreen(m.A.X, m.A.Y), l.CellToScreen(m.B.X, m.B.Y))
	}

	// Ghost preview: the selected (anchor) cell and its legal partner
	// cell(s), highlighted so the player can see exactly where the second
	// tap will land.
	if a.hasSelection {
		sel := l.CellToScreen(a.selected.X, a.selected.Y)
		ink.DrawRect(pad(sel, 2), ink.Black)
		ink.DrawRect(pad(sel, 3), ink.Black)
		for _, pt := range s.Board.PartnersFrom(s.Turn, a.selected) {
			ghost := pad(l.CellToScreen(pt.X, pt.Y), l.CellSize/10)
			ink.FillArea(ghost, ink.LightGray)
			ink.DrawRect(l.CellToScreen(pt.X, pt.Y), ink.DarkGray)
		}
	}

	// Briefly mark the two cells of the most recent move with a corner
	// notch, so the last move stands out among older dominoes.
	if s.HasLastMove {
		for _, p := range s.LastMove {
			markLastMove(l.CellToScreen(p.X, p.Y))
		}
	}
}

// drawDomino renders one placed 1x2 domino spanning cell rects ra and rb
// (adjacent, either stacked or side-by-side): a light-gray-filled tile with
// a bold outline and a center dividing line between its two halves — reads
// clearly on e-ink as a single physical piece, not two separate shaded
// cells.
func drawDomino(ra, rb image.Rectangle) image.Rectangle {
	tile := ra.Union(rb)
	inset := tile.Dx() / 16
	if inset < 2 {
		inset = 2
	}
	r := pad(tile, inset)
	ink.FillArea(r, ink.LightGray)
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 1), ink.Black)
	if ra.Min.X == rb.Min.X {
		// Vertical domino: divider is a horizontal line at the seam.
		my := (ra.Max.Y + rb.Min.Y) / 2
		ink.DrawLine(image.Pt(r.Min.X, my), image.Pt(r.Max.X, my), ink.Black)
	} else {
		// Horizontal domino: divider is a vertical line at the seam.
		mx := (ra.Max.X + rb.Min.X) / 2
		ink.DrawLine(image.Pt(mx, r.Min.Y), image.Pt(mx, r.Max.Y), ink.Black)
	}
	return r
}

// markLastMove draws a small corner tick in each corner of a just-placed
// domino cell, distinguishing the most recent move from older ones already
// on the board (no real animation is possible on e-ink).
func markLastMove(cell image.Rectangle) {
	n := cell.Dx() / 6
	if n < 4 {
		n = 4
	}
	c := image.Pt((cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2)
	ink.DrawLine(image.Pt(c.X-n, c.Y), image.Pt(c.X+n, c.Y), ink.DarkGray)
	ink.DrawLine(image.Pt(c.X, c.Y-n), image.Pt(c.X, c.Y+n), ink.DarkGray)
}

// fillDisc approximates a filled circle inside rect r using horizontal spans
// (used only by the splash motif's small decorative marks, if any).
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

// --- Menu ----------------------------------------------------------------------

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
	size int // game.SizeStandard or game.SizeSmall

	sizeBtns [2]image.Rectangle // [0]=8x8 (Vanlig), [1]=6x6 (Lätt)
	rows     []menuRow
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{size: game.SizeStandard} }

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	H := usableH

	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Dominering")
	ink.DrawString(image.Pt((screen.X-tw)/2, 56), "Dominering")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 30, true)
	sub.SetActive(ink.Black)
	subT := "Välj brädstorlek och läge"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 140), subT)
	sub.Close()

	// Board-size toggle: two buttons side by side; the selected one is drawn
	// with a bold (double) border.
	margin := 60
	rowW := screen.X - 2*margin
	toggleY := 190
	toggleH := 84
	gap := 20
	btnW := (rowW - gap) / 2
	labels := [2]string{"8x8 (Vanlig)", "6x6 (Lätt)"}
	f.Menu.SetActive(ink.Black)
	for i := 0; i < 2; i++ {
		x0 := margin + i*(btnW+gap)
		r := image.Rect(x0, toggleY, x0+btnW, toggleY+toggleH)
		ink.DrawRect(r, ink.Black)
		selected := (i == 0 && m.size == game.SizeStandard) || (i == 1 && m.size == game.SizeSmall)
		if selected {
			ink.DrawRect(pad(r, 3), ink.Black)
			ink.DrawRect(pad(r, 4), ink.Black)
		}
		drawCenteredString(r, labels[i], 36)
		m.sizeBtns[i] = r
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

// TapSizeToggle handles a tap on the 8x8/6x6 toggle. Returns true (and
// updates the selected size) if the tap hit one of the two buttons.
func (m *Menu) TapSizeToggle(p image.Point) bool {
	if p.In(m.sizeBtns[0]) {
		m.size = game.SizeStandard
		return true
	}
	if p.In(m.sizeBtns[1]) {
		m.size = game.SizeSmall
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

// --- Rules screen --------------------------------------------------------------

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

// rulesParagraphs is the rules text for Dominering, one entry per paragraph.
var rulesParagraphs = []string{
	"Mål: den som INTE kan lägga en bricka på sin tur förlorar (\"normalt spel\"). Det är alltså inte sista draget som avgör — det är att sitta helt utan lagligt drag.",
	"Brädet är 8x8 (\"Vanlig\") eller 6x6 (\"Lätt\"), helt tomt från början. Spelarna turas om att lägga en 1x2-bricka (en domino) på två lediga rutor. Vertikal (V) börjar alltid.",
	"Vertikal (V) får ENDAST lägga sin bricka stående — den täcker två rutor i samma kolumn, den ena rakt ovanför den andra. Horisontell (H) får ENDAST lägga sin bricka liggande — den täcker två rutor i samma rad, sida vid sida. Ingen får någonsin lägga i den andra riktningen, oavsett hur brädet ser ut.",
	"En bricka som väl lagts flyttas eller tas aldrig bort. Brädet krymper alltså för varje drag, tills en spelare på sin tur inte längre hittar två lediga rutor i just sin egen riktning — den spelaren förlorar direkt, och motståndaren vinner.",
	"Tryck på en tom ruta för att välja den som ankare — den (eller de) ruta(or) som skulle fullborda en laglig bricka i din riktning markeras automatiskt. Tryck på en markerad ruta för att bekräfta placeringen, eller tryck på den valda rutan igen för att avmarkera.",
	"Mot dator: du spelar alltid Vertikal, datorn spelar alltid Horisontell. Svårighetsgraden avgör hur djupt datorn räknar framåt — men eftersom brädet krymper spelar datorn ofta mycket starkt i slutspelet, oavsett nivå.",
	"Domineering är ett kombinatoriskt matematiskt spel, uppfunnet av Göran Andersson och populariserat av matematikern John Horton Conway.",
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

// drawSplashMotif draws a small 4x4 grid mid-game: a couple of vertical
// dominoes and a horizontal one already placed (each rendered exactly like
// the in-game tiles, with a center dividing line), with the rest of the
// grid left empty — a miniature "board so far" snapshot.
func drawSplashMotif(box image.Rectangle) {
	const n = 4
	side := box.Dx()
	if box.Dy() < side {
		side = box.Dy()
	}
	cell := side / n
	origin := image.Pt(box.Min.X+(box.Dx()-cell*n)/2, box.Min.Y+(box.Dy()-cell*n)/2)
	grid := image.Rect(origin.X, origin.Y, origin.X+cell*n, origin.Y+cell*n)

	ink.DrawRect(grid, ink.Black)
	ink.DrawRect(pad(grid, 1), ink.Black)
	for i := 1; i < n; i++ {
		px := origin.X + i*cell
		ink.DrawLine(image.Pt(px, grid.Min.Y), image.Pt(px, grid.Max.Y), ink.LightGray)
		py := origin.Y + i*cell
		ink.DrawLine(image.Pt(grid.Min.X, py), image.Pt(grid.Max.X, py), ink.LightGray)
	}

	cellRect := func(x, y int) image.Rectangle {
		return image.Rect(origin.X+x*cell, origin.Y+y*cell, origin.X+(x+1)*cell, origin.Y+(y+1)*cell)
	}

	// Two vertical dominoes (columns 0 and 3) and one horizontal domino
	// (row 3, columns 1-2) — a plausible mid-game snapshot.
	drawDomino(cellRect(0, 0), cellRect(0, 1))
	drawDomino(cellRect(3, 1), cellRect(3, 2))
	drawDomino(cellRect(1, 3), cellRect(2, 3))
}
