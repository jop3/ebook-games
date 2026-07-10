package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"shong/game"
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

// --- Layout ------------------------------------------------------------------

const (
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	statusBarHeight = 96
	buttonBarHeight = 140
	boardMargin     = 24
	topMargin       = 40
)

// Layout maps between screen pixels and the 4x6 board. The board is narrower
// than it is tall, so — unlike the square boards in this codebase's other
// games — width and height are handled independently; on this device's
// portrait screen the board ends up height-constrained, leaving generous
// side margins.
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
	cell := avail.Dx() / game.Cols
	if h := avail.Dy() / game.Rows; h < cell {
		cell = h
	}
	if cell < 1 {
		cell = 1
	}
	l.CellSize = cell
	boardW, boardH := cell*game.Cols, cell*game.Rows
	l.GridOrigin = image.Pt(
		avail.Min.X+(avail.Dx()-boardW)/2,
		avail.Min.Y+(avail.Dy()-boardH)/2,
	)
	l.BoardArea = image.Rect(l.GridOrigin.X, l.GridOrigin.Y,
		l.GridOrigin.X+boardW, l.GridOrigin.Y+boardH)
	return l
}

// CellToScreen maps board coordinates to the screen with the vertical axis
// FLIPPED: board y=0 is Black's back rank, and the rules ("Svart börjar
// nederst") put Black at the bottom of the screen — matching every sibling
// game where the human's side is the near edge.
func (l *Layout) CellToScreen(x, y int) image.Rectangle {
	fy := game.Rows - 1 - y
	return image.Rect(
		l.GridOrigin.X+x*l.CellSize,
		l.GridOrigin.Y+fy*l.CellSize,
		l.GridOrigin.X+(x+1)*l.CellSize,
		l.GridOrigin.Y+(fy+1)*l.CellSize,
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
	y = game.Rows - 1 - rel.Y/l.CellSize // inverse of CellToScreen's flip
	if x < 0 || x >= game.Cols || y < 0 || y >= game.Rows {
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
	drawCenteredString(l.StatusBar, text, 36)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawBoard renders the grid, pieces, the current selection (if any), and its
// legal destinations.
func DrawBoard(l *Layout, a *app) {
	s := a.gs
	// Grid frame + lines.
	ink.DrawRect(l.BoardArea, ink.Black)
	ink.DrawRect(pad(l.BoardArea, 1), ink.Black)
	for i := 1; i < game.Cols; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.BoardArea.Min.Y), image.Pt(px, l.BoardArea.Max.Y), ink.Black)
	}
	for i := 1; i < game.Rows; i++ {
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, py), image.Pt(l.BoardArea.Max.X, py), ink.Black)
	}

	// Legal destination hints for a selected piece.
	if a.hasSelection {
		for _, to := range s.Board.DestinationsFrom(a.selected) {
			cell := l.CellToScreen(to.X, to.Y)
			cx, cy := (cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2
			r := l.CellSize / 10
			if r < 3 {
				r = 3
			}
			ink.FillArea(image.Rect(cx-r, cy-r, cx+r, cy+r), ink.LightGray)
		}
	}

	// Pieces.
	for y := 0; y < game.Rows; y++ {
		for x := 0; x < game.Cols; x++ {
			if p := s.Board.At(x, y); p != nil {
				drawPiece(l.CellToScreen(x, y), p)
			}
		}
	}

	// Selection highlight: a bold border around the selected piece's square.
	if a.hasSelection {
		r := l.CellToScreen(a.selected.X, a.selected.Y)
		ink.DrawRect(pad(r, 2), ink.Black)
		ink.DrawRect(pad(r, 3), ink.Black)
	}

	// Briefly mark the square captured by the most recent move.
	if s.LastCaptured != nil {
		drawCaptureMark(l.CellToScreen(s.LastCaptured.X, s.LastCaptured.Y))
	}
}

// drawPiece draws p's glyph (Triangel △, Kvadrat □, X, or a Kung "house"
// shape) centered in the cell, then two small corner markers: bottom-left is
// the side marker (filled disc = Black, hollow ring = White — since e-ink
// has no color, this is what tells the two sides apart on every glyph);
// top-right is either an "eye" dot (once the piece has made its first move
// and is now long, i.e. Moved) or, for a King, a tiny glyph showing its
// current move mode (a "+" for orthogonal, a "x" for diagonal).
func drawPiece(cell image.Rectangle, p *game.Piece) {
	inner := pad(cell, cell.Dx()/6)
	switch p.Kind {
	case game.Triangle:
		fillTriangleUp(inner, ink.Black)
	case game.Square:
		ink.FillArea(inner, ink.Black)
	case game.Ex:
		drawExGlyph(inner)
	case game.King:
		fillKingGlyph(inner, ink.Black)
	}

	markSize := cell.Dx() / 14
	if markSize < 5 {
		markSize = 5
	}
	pad4 := markSize + cell.Dx()/24 + 2

	// Side marker, bottom-left corner.
	sc := image.Pt(cell.Min.X+pad4, cell.Max.Y-pad4)
	sRect := image.Rect(sc.X-markSize, sc.Y-markSize, sc.X+markSize, sc.Y+markSize)
	fillDisc(sRect, ink.Black)
	if p.Side == game.White {
		fillDisc(pad(sRect, markSize/3+1), ink.White)
	}

	// State marker, top-right corner.
	ec := image.Pt(cell.Max.X-pad4, cell.Min.Y+pad4)
	if p.Kind == game.King {
		drawKingModeGlyph(ec, markSize, p.Ortho)
	} else if p.Moved {
		eRect := image.Rect(ec.X-markSize, ec.Y-markSize, ec.X+markSize, ec.Y+markSize)
		fillDisc(eRect, ink.Black)
	}
}

// fillTriangleUp fills an upward-pointing triangle (apex at top-center, base
// spanning the full width at the bottom) inscribed in r.
func fillTriangleUp(r image.Rectangle, col color.Color) {
	apexX := (r.Min.X + r.Max.X) / 2
	apexY := r.Min.Y
	h := r.Max.Y - r.Min.Y
	halfW := r.Dx() / 2
	if h <= 0 {
		return
	}
	for y := apexY; y <= r.Max.Y; y++ {
		w := halfW * (y - apexY) / h
		ink.DrawLine(image.Pt(apexX-w, y), image.Pt(apexX+w, y), col)
	}
}

// fillKingGlyph draws the Kung as a simple "house/crown" silhouette: a
// triangular peak sitting on a square base — visually distinct from a plain
// Triangel or Kvadrat while built from the same primitives.
func fillKingGlyph(r image.Rectangle, col color.Color) {
	peakH := r.Dy() * 2 / 5
	baseTop := r.Min.Y + peakH
	ink.FillArea(image.Rect(r.Min.X, baseTop, r.Max.X, r.Max.Y), col)
	fillTriangleUp(image.Rect(r.Min.X, r.Min.Y, r.Max.X, baseTop), col)
}

// drawExGlyph draws an X as two thick crossed diagonal strokes.
func drawExGlyph(r image.Rectangle) {
	thickness := r.Dx() / 6
	if thickness < 4 {
		thickness = 4
	}
	drawThickDiagLine(r.Min, r.Max, thickness)
	drawThickDiagLine(image.Pt(r.Min.X, r.Max.Y), image.Pt(r.Max.X, r.Min.Y), thickness)
}

// drawThickDiagLine fakes a thick line between two 45-degree-diagonal points
// by drawing several parallel lines, offset horizontally.
func drawThickDiagLine(p1, p2 image.Point, thickness int) {
	for i := -thickness / 2; i <= thickness/2; i++ {
		ink.DrawLine(image.Pt(p1.X+i, p1.Y), image.Pt(p2.X+i, p2.Y), ink.Black)
	}
}

// drawKingModeGlyph draws a tiny "+" (orthogonal mode) or "x" (diagonal
// mode) centered at c, showing which move-set the King currently uses.
func drawKingModeGlyph(c image.Point, size int, ortho bool) {
	if ortho {
		ink.DrawLine(image.Pt(c.X-size, c.Y), image.Pt(c.X+size, c.Y), ink.Black)
		ink.DrawLine(image.Pt(c.X, c.Y-size), image.Pt(c.X, c.Y+size), ink.Black)
	} else {
		ink.DrawLine(image.Pt(c.X-size, c.Y-size), image.Pt(c.X+size, c.Y+size), ink.Black)
		ink.DrawLine(image.Pt(c.X-size, c.Y+size), image.Pt(c.X+size, c.Y-size), ink.Black)
	}
}

// drawCaptureMark frames the square where the most recent move captured an
// enemy piece by displacement (the mover now sits there) in a thin grey
// border — e-ink has no real animation, so a single-frame distinct marker
// stands in for one; it vanishes on its own once the next move replaces
// LastCaptured, so there is no separate timer to manage.
func drawCaptureMark(cell image.Rectangle) {
	ink.DrawRect(pad(cell, cell.Dx()/16), ink.DarkGray)
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

// --- Menu --------------------------------------------------------------------

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
	tw := ink.StringWidth("Shong")
	ink.DrawString(image.Pt((screen.X-tw)/2, 70), "Shong")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 32, true)
	sub.SetActive(ink.Black)
	subT := "Välj läge"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 170), subT)
	sub.Close()

	// Bottom-anchored "Regler" button, stacked up from H-margin.
	margin := 60
	rowW := screen.X - 2*margin
	rbW := rowW / 2
	rbH := 100
	rb := image.Rect((screen.X-rbW)/2, H-margin-rbH, (screen.X+rbW)/2, H-margin)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	// Activate the long-lived menu font BEFORE drawing anything more: the
	// `sub` font closed above must never be the active face when text is
	// measured/drawn (use-after-free in the C library on device).
	f.Menu.SetActive(ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	// Opponent rows fill the space between the subtitle and the Regler button.
	rowH := 130
	top := 300
	bottom := rb.Min.Y - 30
	n := len(opponentChoices)
	avail := bottom - top
	if avail < rowH*n {
		rowH = avail / n
	}
	y := top

	m.rows = m.rows[:0]
	for _, c := range opponentChoices {
		r := image.Rect(margin, y, margin+rowW, y+rowH-20)
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

// rulesParagraphs is the rules text for Shong, one entry per paragraph.
var rulesParagraphs = []string{
	"Mål: fånga motståndarens Kung, eller för din egen Kung ända till motståndarens bortre rad.",
	"Brädet är 4 rutor brett och 6 rutor högt. Varje spelare har fyra pjäser på sin egen bortre rad: X, Triangel, Kvadrat och Kung. Svart börjar nederst, Vit högst upp.",
	"Triangel flyttar diagonalt. Kvadrat flyttar rakt, vågrätt eller lodrätt. X flyttar åt alla åtta håll. Kungen flyttar exakt ett steg och växlar — varje gång den SJÄLV flyttar — mellan Triangelns diagonala drag och Kvadratens raka drag.",
	"Triangel, Kvadrat och X gör ett kort drag (1 ruta) första gången de flyttar. Därefter är pjäsen \"lång\" och flyttar alltid exakt 2 rutor, aldrig bara 1. En prick i pjäsens hörn visar att den blivit lång.",
	"Ingen pjäs får hoppa över en annan. Vägen — även mellanrutan på ett långt drag — måste vara helt fri. Landar du på en fiendepjäs slås den ut.",
	"Tryck på en egen pjäs för att välja den — tillåtna rutor markeras. Tryck sedan på en av dem för att flytta dit. Tryck på samma pjäs igen för att avmarkera.",
	"Baserat på Shong (Higher Plain Games) — fritt abstrakt spel, med egen grafik i den här versionen.",
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

// drawSplashMotif draws the four piece types in a row — Triangel, Kvadrat, X,
// Kung — echoing the built-in chess app's splash. The Triangel carries an
// "eye" dot to hint at the short/long move mechanic, matching how a moved
// piece is marked on the real board.
func drawSplashMotif(box image.Rectangle) {
	cy := (box.Min.Y + box.Max.Y) / 2
	cx := (box.Min.X + box.Max.X) / 2
	r := box.Dx() / 10
	gap := r * 3
	centers := [4]image.Point{
		{X: cx - gap*3/2, Y: cy}, {X: cx - gap/2, Y: cy},
		{X: cx + gap/2, Y: cy}, {X: cx + gap*3/2, Y: cy},
	}
	kinds := [4]game.Kind{game.Triangle, game.Square, game.Ex, game.King}
	for i, c := range centers {
		cell := image.Rect(c.X-r, c.Y-r, c.X+r, c.Y+r)
		switch kinds[i] {
		case game.Triangle:
			fillTriangleUp(cell, ink.Black)
		case game.Square:
			ink.FillArea(cell, ink.Black)
		case game.Ex:
			drawExGlyph(cell)
		case game.King:
			fillKingGlyph(cell, ink.Black)
		}
	}
	// Eye dot on the Triangel, hinting at the short/long mechanic.
	eyeC := image.Pt(centers[0].X+r, centers[0].Y-r)
	eyeR := r / 3
	fillDisc(image.Rect(eyeC.X-eyeR, eyeC.Y-eyeR, eyeC.X+eyeR, eyeC.Y+eyeR), ink.Black)
}
