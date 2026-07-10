package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"isola/game"
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

// Layout maps between screen pixels and the 8x8 board.
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

// DrawBoard renders the grid, the missing (hatched) tiles, both pawns, and —
// depending on whose turn/step it is — either the highlighted move
// destinations or the highlighted removable tiles (with the mover's own new
// position marked as excluded from removal).
func DrawBoard(l *Layout, a *app) {
	s := a.gs
	b := &s.Board

	// Grid frame + lines.
	ink.DrawRect(l.BoardArea, ink.Black)
	ink.DrawRect(pad(l.BoardArea, 1), ink.Black)
	for i := 1; i < game.Size; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.BoardArea.Min.Y), image.Pt(px, l.BoardArea.Max.Y), ink.Black)
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, py), image.Pt(l.BoardArea.Max.X, py), ink.Black)
	}

	// Missing tiles: hatch fill.
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			if !b.IsPresent(x, y) {
				drawHatch(l.CellToScreen(x, y))
			}
		}
	}

	// Highlight the set of cells legal to tap right now (only during a
	// human turn; the AI's turn shows no hints).
	humansTurn := s.Phase == game.PhasePlaying && !s.AITurn()
	if humansTurn {
		switch s.Step {
		case game.StepMove:
			for _, d := range b.LegalMoves(s.Turn) {
				fillLight(l.CellToScreen(d.X, d.Y))
			}
		case game.StepRemove:
			for _, r := range b.LegalTileRemovals(s.PendingTo) {
				fillLight(l.CellToScreen(r.X, r.Y))
			}
			// Mark the mover's new position as explicitly NOT removable.
			drawExcluded(l.CellToScreen(s.PendingTo.X, s.PendingTo.Y))
		}
	}

	// Pawns. Black = filled disc; White = hollow (ring) disc.
	drawMan(l.CellToScreen(b.BlackPawn.X, b.BlackPawn.Y), true)
	drawMan(l.CellToScreen(b.WhitePawn.X, b.WhitePawn.Y), false)

	// Briefly mark the most recently removed tile (it is already missing —
	// drawHatch above already rendered it — so just outline it) so the
	// player can see what just vanished.
	if s.HasLast {
		r := l.CellToScreen(s.LastRemoved.X, s.LastRemoved.Y)
		ink.DrawRect(pad(r, 2), ink.Black)
	}
}

// fillLight tints a cell to show it is a legal tap target right now (a move
// destination, or — during the removal step — a removable tile).
func fillLight(cell image.Rectangle) {
	ink.FillArea(pad(cell, 2), ink.LightGray)
}

// drawExcluded overlays a bold X on the mover's own new position during the
// removal step, showing it is the one tile that cannot be removed.
func drawExcluded(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/4)
	ink.DrawLine(image.Pt(r.Min.X, r.Min.Y), image.Pt(r.Max.X, r.Max.Y), ink.Black)
	ink.DrawLine(image.Pt(r.Max.X, r.Min.Y), image.Pt(r.Min.X, r.Max.Y), ink.Black)
}

// drawHatch fills a cell with a dense diagonal hatch pattern, marking a
// removed (missing) tile — visually distinct from the sparser single-line
// "excluded" marker and from the plain LightGray move/removal highlight.
func drawHatch(cell image.Rectangle) {
	ink.FillArea(cell, ink.White)
	step := cell.Dx() / 4
	if step < 6 {
		step = 6
	}
	for x := cell.Min.X - cell.Dy(); x < cell.Max.X; x += step {
		p1 := image.Pt(x, cell.Min.Y)
		p2 := image.Pt(x+cell.Dy(), cell.Max.Y)
		clipDiagonal(p1, p2, cell)
	}
	ink.DrawRect(cell, ink.DarkGray)
}

// clipDiagonal draws the portion of the line p1->p2 (a 45-degree diagonal)
// that falls within cell, clipping crudely by walking the diagonal in unit
// steps — cheap and adequate at on-screen cell sizes.
func clipDiagonal(p1, p2 image.Point, cell image.Rectangle) {
	dx, dy := 1, 1
	if p2.X < p1.X {
		dx = -1
	}
	x, y := p1.X, p1.Y
	var start, end image.Point
	started := false
	for {
		if x >= cell.Min.X && x < cell.Max.X && y >= cell.Min.Y && y < cell.Max.Y {
			if !started {
				start = image.Pt(x, y)
				started = true
			}
			end = image.Pt(x, y)
		}
		if x == p2.X && y == p2.Y {
			break
		}
		x += dx
		y += dy
	}
	if started {
		ink.DrawLine(start, end, ink.DarkGray)
	}
}

// drawMan draws a pawn inside a cell. black=true renders a solid disc; false
// renders a hollow (white) disc with a black outline so both read on e-ink.
func drawMan(cell image.Rectangle, black bool) {
	r := pad(cell, cell.Dx()/8)
	fillDisc(r, ink.Black)
	if !black {
		inner := pad(r, r.Dx()/8+2)
		fillDisc(inner, ink.White)
	}
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
	rows     []menuRow
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{} }

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	H := usableH

	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Isola")
	ink.DrawString(image.Pt((screen.X-tw)/2, 56), "Isola")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 30, true)
	sub.SetActive(ink.Black)
	subT := "Välj läge"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 150), subT)
	sub.Close()

	margin := 60
	rowW := screen.X - 2*margin

	// Bottom-anchored "Regler" button, stacked up from H-margin.
	rbW := rowW / 2
	rbH := 100
	rb := image.Rect((screen.X-rbW)/2, H-margin-rbH, (screen.X+rbW)/2, H-margin)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	f.Menu.SetActive(ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	// Opponent rows fill the space between the subtitle and the Regler
	// button.
	rowH := 130
	top := 240
	bottom := rb.Min.Y - 30
	n := len(opponentChoices)
	avail := bottom - top
	if avail < rowH*n {
		rowH = avail / n
	}
	y := top

	m.rows = m.rows[:0]
	for _, c := range opponentChoices {
		r := image.Rect(margin, y, margin+rowW, y+rowH-22)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawLeftString(r, c.label, 38)
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

// rulesParagraphs is the rules text for Isola, one entry per paragraph.
var rulesParagraphs = []string{
	"Mål: du vinner så fort din motståndare, på sin egen tur, inte har något lagligt drag att göra.",
	"Brädet är 8x8 och alla 64 rutor finns kvar från början. Varje spelare har en enda spelpjäs, som börjar på var sin sida av brädet.",
	"En tur har två steg. Först flyttar du din pjäs: den rör sig som en dam i schack, hur långt som helst i någon av de åtta riktningarna (vågrätt, lodrätt eller diagonalt).",
	"Din pjäs får aldrig hoppa över en saknad ruta eller motståndarens pjäs, och kan aldrig landa på en saknad ruta eller på motståndaren — den stannar alltid precis innan.",
	"Sedan tar du bort exakt en ruta från brädet: valfri kvarvarande ruta utom just den du nu står på. Rutan du precis lämnade får du gärna ta bort.",
	"En borttagen ruta är borta för resten av spelet — ingen pjäs kan någonsin flytta till den eller passera över den igen. Brädet krymper alltså tur för tur.",
	"Du vinner så fort motståndaren, på sin tur, inte kan göra något enda lagligt drag — även om båda pjäserna fortfarande finns kvar på brädet.",
	"Tryck på en av de gråmarkerade rutorna för att flytta dit. Tryck sedan på en av de kvarvarande rutorna (allt utom ett kryss över rutan du just landade på) för att ta bort den.",
	"Datorns styrka bygger på rörlighet: den räknar hur många drag den själv har jämfört med hur många motståndaren har kvar, och försöker maximera den skillnaden varje tur.",
	"Isola (även känt som Isolation) är ett klassiskt abstrakt brädspel, känt sedan 1970-talet.",
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

// drawSplashMotif draws a small 5x3 Isola board sample: a solid (Black) pawn
// in one corner, a ring (White) pawn in the opposite corner, and a scatter
// of hatched (removed) tiles in between — the "shrinking board" idea in one
// glance.
func drawSplashMotif(box image.Rectangle) {
	const cols, rows = 5, 3
	cellW := box.Dx() / cols
	cellH := box.Dy() / rows
	cell := cellW
	if cellH < cell {
		cell = cellH
	}
	boardW, boardH := cell*cols, cell*rows
	ox := box.Min.X + (box.Dx()-boardW)/2
	oy := box.Min.Y + (box.Dy()-boardH)/2

	at := func(cx, cy int) image.Rectangle {
		return image.Rect(ox+cx*cell, oy+cy*cell, ox+(cx+1)*cell, oy+(cy+1)*cell)
	}

	// Grid frame + lines.
	board := image.Rect(ox, oy, ox+boardW, oy+boardH)
	ink.DrawRect(board, ink.Black)
	for i := 1; i < cols; i++ {
		x := ox + i*cell
		ink.DrawLine(image.Pt(x, board.Min.Y), image.Pt(x, board.Max.Y), ink.Black)
	}
	for i := 1; i < rows; i++ {
		y := oy + i*cell
		ink.DrawLine(image.Pt(board.Min.X, y), image.Pt(board.Max.X, y), ink.Black)
	}

	// A scatter of removed (hatched) tiles between the two pawns.
	removed := [][2]int{{1, 0}, {2, 1}, {3, 0}, {1, 2}, {3, 2}}
	for _, rc := range removed {
		drawHatch(at(rc[0], rc[1]))
	}

	// Pawns in opposite corners.
	drawMan(at(0, rows-1), true)  // Black, bottom-left
	drawMan(at(cols-1, 0), false) // White, top-right
}
