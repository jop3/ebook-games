package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"murar/game"
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
		Button: ink.OpenFont(ink.DefaultFontBold, 34, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 38, true),
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
	boardMargin     = 16
	topMargin       = 40
)

// Layout maps between screen pixels and the 9x9 board, plus the 8x8 wall
// groove grid overlaid on it.
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

// IntersectionCenter returns the screen point of wall-groove intersection
// (gx,gy), gx,gy in [0,WallGrid) — the corner shared by the 2x2 block of
// cells (gx,gy)..(gx+1,gy+1).
func (l *Layout) IntersectionCenter(gx, gy int) image.Point {
	return image.Pt(l.GridOrigin.X+(gx+1)*l.CellSize, l.GridOrigin.Y+(gy+1)*l.CellSize)
}

// IntersectionRect returns a tappable square centered on intersection
// (gx,gy), one cell wide, so grooves are comfortably tappable.
func (l *Layout) IntersectionRect(gx, gy int) image.Rectangle {
	c := l.IntersectionCenter(gx, gy)
	half := l.CellSize / 2
	return image.Rect(c.X-half, c.Y-half, c.X+half, c.Y+half)
}

// ScreenToIntersection returns the nearest wall-groove intersection to p, if
// p falls within its tap rectangle.
func (l *Layout) ScreenToIntersection(p image.Point) (gx, gy int, ok bool) {
	if l.CellSize == 0 {
		return 0, 0, false
	}
	rel := p.Sub(l.GridOrigin)
	gx = (rel.X+l.CellSize/2)/l.CellSize - 1
	gy = (rel.Y+l.CellSize/2)/l.CellSize - 1
	if gx < 0 || gx >= game.WallGrid || gy < 0 || gy >= game.WallGrid {
		return 0, 0, false
	}
	if !p.In(l.IntersectionRect(gx, gy)) {
		return 0, 0, false
	}
	return gx, gy, true
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
	drawCenteredString(l.StatusBar, text, 34)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

const wallThickness = 10

// DrawBoard renders the 9x9 grid, placed walls, the pending wall preview (if
// any, in build mode), both pawns, and — in move mode, on a human's turn —
// the currently legal destination cells.
func DrawBoard(l *Layout, a *app) {
	s := a.gs

	// Grid frame + lines.
	ink.DrawRect(l.BoardArea, ink.Black)
	for i := 1; i < game.Size; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.BoardArea.Min.Y), image.Pt(px, l.BoardArea.Max.Y), ink.LightGray)
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, py), image.Pt(l.BoardArea.Max.X, py), ink.LightGray)
	}

	// Legal pawn-move hints (move mode, human's turn only).
	if !a.buildMode && s.Phase == game.PhasePlaying && !s.AITurn() {
		for _, to := range game.LegalPawnMoves(&s.Board, s.Turn) {
			cell := l.CellToScreen(to.X, to.Y)
			cx, cy := (cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2
			r := l.CellSize / 9
			if r < 3 {
				r = 3
			}
			ink.FillArea(image.Rect(cx-r, cy-r, cx+r, cy+r), ink.DarkGray)
		}
	}

	// Placed walls (thick black bars across the groove).
	for y := 0; y < game.WallGrid; y++ {
		for x := 0; x < game.WallGrid; x++ {
			if s.Board.WallH[y][x] {
				drawWallBar(l, game.Wall{X: x, Y: y, Orient: game.Horizontal}, ink.Black)
			}
			if s.Board.WallV[y][x] {
				drawWallBar(l, game.Wall{X: x, Y: y, Orient: game.Vertical}, ink.Black)
			}
		}
	}

	// Pending wall preview (build mode only).
	if a.buildMode && a.pendingWall != nil {
		if a.wallRejected {
			drawWallBarHatched(l, *a.pendingWall)
		} else {
			drawWallBar(l, *a.pendingWall, ink.DarkGray)
		}
	}

	// Pawns. P1 ("Svart") = filled disc; P2 ("Vit") = hollow ring.
	drawPawn(l.CellToScreen(s.Board.Pawns[game.P1].X, s.Board.Pawns[game.P1].Y), true)
	drawPawn(l.CellToScreen(s.Board.Pawns[game.P2].X, s.Board.Pawns[game.P2].Y), false)

	// Briefly highlight the most recently placed wall.
	if s.LastWall != nil {
		w := *s.LastWall
		r := wallBarRect(l, w)
		ink.DrawRect(pad(r, -3), ink.Black)
	}
}

func wallBarRect(l *Layout, w game.Wall) image.Rectangle {
	t := wallThickness
	switch w.Orient {
	case game.Horizontal:
		x0 := l.GridOrigin.X + w.X*l.CellSize
		x1 := l.GridOrigin.X + (w.X+2)*l.CellSize
		yLine := l.GridOrigin.Y + (w.Y+1)*l.CellSize
		return image.Rect(x0, yLine-t/2, x1, yLine+t/2)
	default: // Vertical
		y0 := l.GridOrigin.Y + w.Y*l.CellSize
		y1 := l.GridOrigin.Y + (w.Y+2)*l.CellSize
		xLine := l.GridOrigin.X + (w.X+1)*l.CellSize
		return image.Rect(xLine-t/2, y0, xLine+t/2, y1)
	}
}

func drawWallBar(l *Layout, w game.Wall, col color.Color) {
	ink.FillArea(wallBarRect(l, w), col)
}

// drawWallBarHatched draws a wall preview as a dashed bar — the brief visual
// cue that the last placement attempt at this spot was rejected.
func drawWallBarHatched(l *Layout, w game.Wall) {
	r := wallBarRect(l, w)
	switch w.Orient {
	case game.Horizontal:
		dash := 10
		for x := r.Min.X; x < r.Max.X; x += dash * 2 {
			x1 := x + dash
			if x1 > r.Max.X {
				x1 = r.Max.X
			}
			ink.FillArea(image.Rect(x, r.Min.Y, x1, r.Max.Y), ink.DarkGray)
		}
	default:
		dash := 10
		for y := r.Min.Y; y < r.Max.Y; y += dash * 2 {
			y1 := y + dash
			if y1 > r.Max.Y {
				y1 = r.Max.Y
			}
			ink.FillArea(image.Rect(r.Min.X, y, r.Max.X, y1), ink.DarkGray)
		}
	}
}

// drawPawn draws a pawn inside a cell. black=true renders a solid disc;
// false renders a hollow (ring) disc so both read clearly on e-ink.
func drawPawn(cell image.Rectangle, black bool) {
	r := pad(cell, cell.Dx()/6)
	fillDisc(r, ink.Black)
	if !black {
		inner := pad(r, r.Dx()/8+3)
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
	gap := 16
	sideMargin := 20
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
		// The drawn text may be fitted/ellipsized to the button's width, but
		// the button's hit-dispatch Label always stays the original string
		// so handleButton's exact-match switch never breaks on truncation.
		drawCenteredString(r, fitLabel(label, bw-16, f), 34)
		buttons[i] = Button{Rect: r, Label: label}
	}
	return buttons
}

// fitLabel falls back to the Small font (returned as-is; the caller always
// draws with f.Button, so this only trims text that would overflow — Murar's
// button labels are short enough in practice that this rarely engages) and,
// as a last resort, ellipsizes.
func fitLabel(label string, maxW int, f *Fonts) string {
	if ink.StringWidth(label) <= maxW {
		return label
	}
	for len(label) > 1 && ink.StringWidth(label+"…") > maxW {
		label = label[:len(label)-1]
	}
	return label + "…"
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
	tw := ink.StringWidth("Murar")
	ink.DrawString(image.Pt((screen.X-tw)/2, 56), "Murar")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 28, true)
	sub.SetActive(ink.DarkGray)
	subT := "Datorn spelar okej — inte perfekt"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 140), subT)
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

	// Opponent rows fill the space between the subtitle and the Regler button.
	top := 200
	bottom := rb.Min.Y - 30
	n := len(opponentChoices)
	rowH := 130
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

// rulesParagraphs is the rules text for Murar (Quoridor), one entry per
// paragraph.
var rulesParagraphs = []string{
	"Mål: nå en valfri ruta på motsatta kortsidan av brädet — sidan mitt emot din egen startsida.",
	"Brädet är 9x9. Varje spelare har en pjäs, centrerad på sin egen kortsida, och 10 murar i förråd. Svart börjar.",
	"Varje tur gör du EN sak: flyttar din pjäs, eller bygger en mur.",
	"Flytta pjäs: ett steg rakt — upp, ner, vänster eller höger — till en ledig ruta som inte är murblockerad.",
	"Hoppa över motståndaren: står motståndaren precis intill dig i den riktning du vill gå, och rutan bortom är ledig och inte murblockerad, hoppar du dit direkt.",
	"Diagonalt hopp (undantaget): är rutan bortom istället blockerad — av en mur, eller för att motståndaren står vid bordets bortre kant — kliver du diagonalt till en ledig, icke murblockerad ruta jämte motståndaren istället.",
	"Bygg mur: två rutor lång, i skåran mellan rutorna, vågrätt eller lodrätt. Får inte överlappa eller korsa en mur som redan ligger i samma skärningspunkt — och får ALDRIG stänga av varenda väg till målet för någon spelare, vare sig din egen eller motståndarens.",
	"Vinst: först till motsatta kortsidan vinner.",
	"Tryck \"Bygg mur\"/\"Flytta pjäs\" för att växla läge. I murläge: tryck för att förhandsgranska en mur, tryck igen för att bygga den, eller \"Rotera\" för att vända den. Ogiltiga placeringar avvisas tyst.",
	"Datorn (Lätt/Medel/Svår) spelar okej — inte perfekt. Den värderar drag efter avstånd till mål och överväger bara murar nära motståndarens just nu kortaste väg, inte varenda tänkbar mur.",
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

	body := ink.OpenFont(ink.DefaultFont, 27, true)
	body.SetActive(ink.Black)
	bodyMargin := 46
	maxW := screen.X - 2*bodyMargin
	y := 140
	lineH := 36
	paraGap := 14
	limit := r.Min.Y - 16
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

// drawSplashMotif draws a small 5x5 grid with a filled ("Svart") pawn
// centered on the bottom row, a ring ("Vit") pawn centered on the top row,
// and one short wall segment across the groove between them — Murar's
// elevator pitch in one small picture.
func drawSplashMotif(box image.Rectangle) {
	const cols, rows = 5, 5
	cell := box.Dx() / cols
	if box.Dy()/rows < cell {
		cell = box.Dy() / rows
	}
	gw, gh := cell*cols, cell*rows
	ox := box.Min.X + (box.Dx()-gw)/2
	oy := box.Min.Y + (box.Dy()-gh)/2

	for i := 0; i <= cols; i++ {
		x := ox + i*cell
		ink.DrawLine(image.Pt(x, oy), image.Pt(x, oy+gh), ink.Black)
	}
	for j := 0; j <= rows; j++ {
		y := oy + j*cell
		ink.DrawLine(image.Pt(ox, y), image.Pt(ox+gw, y), ink.Black)
	}

	bottomCell := image.Rect(ox+2*cell, oy+4*cell, ox+3*cell, oy+5*cell)
	drawPawn(bottomCell, true)

	topCell := image.Rect(ox+2*cell, oy, ox+3*cell, oy+cell)
	drawPawn(topCell, false)

	// A short wall segment across the groove between row 2 and row 3,
	// spanning 2 cell-lengths (columns 1-3).
	wallY := oy + 3*cell
	thickness := cell / 4
	if thickness < 5 {
		thickness = 5
	}
	ink.FillArea(image.Rect(ox+1*cell, wallY-thickness/2, ox+3*cell, wallY+thickness/2), ink.Black)
}
