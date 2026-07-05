package main

import (
	"image"
	"image/color"
	"math"

	ink "github.com/dennwc/inkview"

	"hexa/game"
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
		Status: ink.OpenFont(ink.DefaultFontBold, 30, true),
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

	statusBarHeight = 90
	buttonBarHeight = 140
	boardMargin     = 10
	topMargin       = 34
)

// Layout maps between screen pixels and the board's cube-coordinate hexes.
// Cell centers are placed with the standard "pointy-top" axial hex formula,
// using (q,r) = (X,Z) of the cube coordinate — the same mapping
// ringar/ui.go uses (see its Layout doc comment for why this produces a
// true regular hex tessellation for this Directions convention), just over
// a plain full hexagon of cells instead of YINSH's truncated 85-point board.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	ButtonBar image.Rectangle
	BoardArea image.Rectangle

	Size    int // hex circumradius in pixels
	order   []game.Hex
	centers map[game.Hex]image.Point
}

func hexUnit(p game.Hex) (float64, float64) {
	q := float64(p.X)
	r := float64(p.Z)
	px := math.Sqrt(3) * (q + r/2)
	py := 1.5 * r
	return px, py
}

func NewLayout(screen image.Point) Layout {
	H := usableH
	l := Layout{Screen: image.Rect(0, 0, screen.X, H)}
	l.StatusBar = image.Rect(0, topMargin, screen.X, topMargin+statusBarHeight)
	l.ButtonBar = image.Rect(0, H-topMargin-buttonBarHeight, screen.X, H-topMargin)

	avail := image.Rect(boardMargin, l.StatusBar.Max.Y+boardMargin,
		screen.X-boardMargin, l.ButtonBar.Min.Y-boardMargin)

	pts := game.AllPoints()
	minPx, maxPx := math.Inf(1), math.Inf(-1)
	minPy, maxPy := math.Inf(1), math.Inf(-1)
	unit := make(map[game.Hex][2]float64, len(pts))
	for _, p := range pts {
		px, py := hexUnit(p)
		unit[p] = [2]float64{px, py}
		minPx, maxPx = math.Min(minPx, px), math.Max(maxPx, px)
		minPy, maxPy = math.Min(minPy, py), math.Max(maxPy, py)
	}
	unitW := maxPx - minPx
	unitH := maxPy - minPy

	const hx = 0.866025404
	sizeW := float64(avail.Dx()) / (unitW + 2*hx)
	sizeH := float64(avail.Dy()) / (unitH + 2.0)
	size := math.Min(sizeW, sizeH)
	if size < 1 {
		size = 1
	}

	drawnW := size*unitW + 2*hx*size
	drawnH := size*unitH + 2*size
	left := size*minPx - hx*size
	top := size*minPy - size
	originX := float64(avail.Min.X) + (float64(avail.Dx())-drawnW)/2 - left
	originY := float64(avail.Min.Y) + (float64(avail.Dy())-drawnH)/2 - top

	l.Size = int(math.Round(size))
	l.order = pts
	l.centers = make(map[game.Hex]image.Point, len(pts))
	for _, p := range pts {
		u := unit[p]
		cx := int(math.Round(size*u[0] + originX))
		cy := int(math.Round(size*u[1] + originY))
		l.centers[p] = image.Pt(cx, cy)
	}
	l.BoardArea = avail
	return l
}

// Center returns the pixel center of board cell p.
func (l *Layout) Center(p game.Hex) image.Point { return l.centers[p] }

// PointAt returns the board cell whose center is nearest tap point q, if q
// is within a generous radius of it.
func (l *Layout) PointAt(q image.Point) (game.Hex, bool) {
	if l.Size <= 0 {
		return game.Hex{}, false
	}
	bestD := l.Size * l.Size * 3
	var best game.Hex
	found := false
	for _, p := range l.order {
		c := l.centers[p]
		dx, dy := q.X-c.X, q.Y-c.Y
		d := dx*dx + dy*dy
		if d < bestD {
			bestD, best, found = d, p, true
		}
	}
	return best, found
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
	drawCenteredString(l.StatusBar, text, 30)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawBoard renders the full hex field, every tile, the legal-move ghosts
// for whichever action the human can currently take, the current
// selection, and — once the game is over — the winning shape's highlight.
func DrawBoard(l *Layout, a *app) {
	s := a.gs

	for _, p := range l.order {
		drawHexOutline(l.Center(p), l.Size)
	}

	// Legal-move ghosts: the placement frontier, or (once a tile is
	// selected in the movement phase) that tile's legal destinations.
	// Suppressed during the AI's own turn so nothing suggests a move the
	// human isn't the one making right now.
	if !s.AITurn() {
		switch s.Phase {
		case game.PhasePlacement:
			for _, p := range game.PlaceMoves(s.Board.Tiles) {
				ghostHex(l.Center(p), l.Size)
			}
		case game.PhaseMoving:
			if a.hasSelection {
				for _, m := range game.MoveMoves(s.Board.Tiles, s.Turn, s.Advanced) {
					if m.From == a.selected {
						ghostHex(l.Center(m.To), l.Size)
					}
				}
			}
		}
	}

	tileR := l.Size * 78 / 100
	for _, p := range l.order {
		side := s.Board.At(p)
		if side == game.None {
			continue
		}
		drawTile(l.Center(p), tileR, side == game.Black)
	}

	// Current selection (movement phase): a bold ring drawn in the gap
	// between the tile's own fill and the cell border, PLUS a ring
	// stepping OUTWARD past the cell border — the outward step matters
	// because a black tile's fill already reaches most of the way to the
	// cell edge, so a highlight confined to that narrow gap reads poorly;
	// stepping outside the cell guarantees visible black-on-white contrast
	// regardless of the tile's own color or how densely packed its
	// neighbours are.
	if s.Phase == game.PhaseMoving && a.hasSelection {
		drawBoldHex(l.Center(a.selected), l.Size+6, 5)
	}

	// The completed shape: bold outline on all 6 winning cells.
	if s.Phase == game.PhaseDone && s.WinKind != game.ShapeNone {
		for _, p := range s.WinCells {
			drawBoldHex(l.Center(p), l.Size, 4)
		}
	}
}

// ghostHex marks an empty legal-move cell with a small faint dot, echoing
// every other game's LightGray legal-move hint.
func ghostHex(c image.Point, size int) {
	r := size/3 + 2
	fillCircle(c, r, ink.LightGray)
}

// drawTile draws a tile: a solid black hex for Black ("you" — the local
// human is always Black, matching every other game in this house), or a
// bold hex OUTLINE (black hex, hollowed out to a smaller white hex inside —
// the same "thick outer, hollow inner" convention every other game in this
// house uses to render White) for White ("the opponent") — two fills that
// stay clearly distinct from each other AND from the plain thin outlines
// drawn for every empty board cell.
func drawTile(c image.Point, r int, black bool) {
	if black {
		fillHex(c, r, ink.Black)
		return
	}
	fillHex(c, r, ink.Black)
	inner := r - r/3 - 2
	if inner < 1 {
		inner = 1
	}
	fillHex(c, inner, ink.White)
}

// drawBoldHex draws n concentric hex outlines 2px apart, centered at c —
// used for selection/win-highlight emphasis (thicker = more emphasis).
func drawBoldHex(c image.Point, r, n int) {
	for i := 0; i < n; i++ {
		rr := r - i*2
		if rr < 2 {
			break
		}
		drawHexOutline(c, rr)
	}
}

// --- Hex primitives (adapted from hex/ui.go's drawHexOutline/hexCorners/
// fillCircle — the shared house convention for hex rendering; copied
// rather than imported since each game module is self-contained, exactly
// as fillDisc/isqrt are independently copied into every other game in this
// repo). fillHex is new here: Six's tiles are filled hexes, not discs. -----

// drawHexOutline draws a pointy-top hexagon outline centered at c.
func drawHexOutline(c image.Point, r int) {
	pts := hexCorners(c, r)
	for i := 0; i < 6; i++ {
		ink.DrawLine(pts[i], pts[(i+1)%6], ink.Black)
	}
}

// hexCorners returns the 6 corners of a pointy-top hexagon.
func hexCorners(c image.Point, r int) [6]image.Point {
	hx := r * 866 / 1000
	hy := r / 2
	return [6]image.Point{
		{c.X, c.Y - r},
		{c.X + hx, c.Y - hy},
		{c.X + hx, c.Y + hy},
		{c.X, c.Y + r},
		{c.X - hx, c.Y + hy},
		{c.X - hx, c.Y - hy},
	}
}

// fillHex fills a pointy-top hexagon of circumradius r centered at c, via
// horizontal scanlines whose half-width tapers linearly outside the hex's
// vertical-sided middle band — the hex analogue of fillCircle's scanline
// disc fill.
func fillHex(c image.Point, r int, col color.Color) {
	if r < 1 {
		r = 1
	}
	hx := r * 866 / 1000
	hy := r / 2
	slantSpan := r - hy
	for dy := -r; dy <= r; dy++ {
		ady := dy
		if ady < 0 {
			ady = -ady
		}
		var halfw int
		if ady <= hy {
			halfw = hx
		} else if slantSpan > 0 {
			halfw = hx * (r - ady) / slantSpan
		}
		if halfw < 0 {
			halfw = 0
		}
		y := c.Y + dy
		ink.DrawLine(image.Pt(c.X-halfw, y), image.Pt(c.X+halfw, y), col)
	}
}

func fillCircle(c image.Point, rad int, col color.Color) {
	rr := rad * rad
	for dy := -rad; dy <= rad; dy++ {
		hw := isqrt(rr - dy*dy)
		ink.DrawLine(image.Pt(c.X-hw, c.Y+dy), image.Pt(c.X+hw, c.Y+dy), col)
	}
}

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
	{game.OpponentAI, game.DepthEasy, "Mot dator - Lätt"},
	{game.OpponentAI, game.DepthMedium, "Mot dator - Medel"},
	{game.OpponentAI, game.DepthHard, "Mot dator - Svår"},
}

type Menu struct {
	advanced bool

	modeBtns [2]image.Rectangle // [0]=Standard, [1]=Avancerat
	rows     []menuRow
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{} }

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	H := usableH

	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Hexa")
	ink.DrawString(image.Pt((screen.X-tw)/2, 50), "Hexa")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 28, true)
	sub.SetActive(ink.Black)
	subT := "Välj läge"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 138), subT)
	sub.Close()

	margin := 60
	rowW := screen.X - 2*margin

	// Standard/Avancerat toggle: two buttons side by side; the selected one
	// gets a bold (double) border — same convention as hasami's Fångst/Fem
	// i rad toggle.
	toggleY := 180
	toggleH := 78
	gap := 20
	btnW := (rowW - gap) / 2
	labels := [2]string{"Standard", "Avancerat"}
	f.Menu.SetActive(ink.Black)
	for i := 0; i < 2; i++ {
		x0 := margin + i*(btnW+gap)
		r := image.Rect(x0, toggleY, x0+btnW, toggleY+toggleH)
		ink.DrawRect(r, ink.Black)
		selected := (i == 0 && !m.advanced) || (i == 1 && m.advanced)
		if selected {
			ink.DrawRect(pad(r, 3), ink.Black)
			ink.DrawRect(pad(r, 4), ink.Black)
		}
		drawCenteredString(r, labels[i], 34)
		m.modeBtns[i] = r
	}

	// Bottom-anchored "Regler" button, stacked up from H-margin.
	rbW := rowW / 2
	rbH := 96
	rb := image.Rect((screen.X-rbW)/2, H-margin-rbH, (screen.X+rbW)/2, H-margin)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	f.Button.SetActive(ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	// Opponent rows fill the space between the toggle and the Regler
	// button.
	rowH := 108
	top := toggleY + toggleH + 26
	bottom := rb.Min.Y - 26
	n := len(opponentChoices)
	avail := bottom - top
	if avail < rowH*n {
		rowH = avail / n
	}
	y := top

	m.rows = m.rows[:0]
	for _, c := range opponentChoices {
		r := image.Rect(margin, y, margin+rowW, y+rowH-16)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawLeftString(r, c.label, 36)
		m.rows = append(m.rows, menuRow{rect: r, choice: c})
		y += rowH
	}
}

// TapAdvancedToggle handles a tap on the Standard/Avancerat toggle. Returns
// true (and updates the selected mode) if the tap hit one of the two
// buttons.
func (m *Menu) TapAdvancedToggle(p image.Point) bool {
	if p.In(m.modeBtns[0]) {
		m.advanced = false
		return true
	}
	if p.In(m.modeBtns[1]) {
		m.advanced = true
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

// rulesParagraphs is the rules text for Hexa, one entry per paragraph,
// ordered with the most essential content FIRST — the win condition, the
// original-game credit, and the honest "casual AI" framing — since
// DrawRules truncates whatever doesn't fit rather than scrolling.
var rulesParagraphs = []string{
	"Mål: forma en av tre figurer med SEX av dina egna hexbrickor — en rak Linje, en Triangel, eller en Hexagon-ring runt en mittcell.",
	"Baserat på Six (Steffen Spiele, Steffen Mühlhäuser).",
	"\"Mot dator\" är en avslappnad motståndare, inte tänkt att vara stark — hot-seat med två spelare är Hexas huvudsakliga spelsätt.",
	"I originalspelet växer mosaiken fritt utan bräde. Den här versionen använder ett stort men begränsat sexkantigt fält — i praktiken tar ett parti sällan slut vid kanten.",
	"Placeringsfas: Svart och Vit har 21 brickor var. Turas om att lägga en bricka så att den ligger an mot minst en bricka som redan ligger — hela mosaiken måste hela tiden hänga ihop. Svart börjar.",
	"Linje: sex brickor i rad, längs en av fältets 3 riktningar.",
	"Triangel: sex brickor i en triangelform (rader om 3, 2 och 1), i valfri riktning.",
	"Hexagon: sex brickor som omger en mittcell — mittcellen kan vara tom eller vilken färg som helst, bara de sex runt om räknas.",
	"Flyttfas: när alla 42 brickor är utlagda utan vinnare turas spelarna i stället om att flytta en egen bricka till en annan tom plats i mosaikens kant. Mosaiken måste hänga ihop även efter flytten.",
	"Avancerat (valfritt): en flytt FÅR spränga mosaiken i bitar. Alla brickor i en bit som inte innehåller den flyttade brickan tas då bort helt. Den som får 5 eller färre brickor kvar kan aldrig mer forma en figur och förlorar.",
	"Tryck på en ledig, markerad ruta för att placera. I flyttfasen: tryck på en egen bricka (dess tillåtna mål markeras), tryck sedan på målet. Tryck på samma bricka igen för att avmarkera.",
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
	bodyMargin := 50
	maxW := screen.X - 2*bodyMargin
	y := 130
	lineH := 35
	paraGap := 13
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

// drawSplashMotif draws the three winning shapes in miniature, each as 6
// solid black hexes: a line of 6, a triangle of 6, and a hexagon-ring of 6
// (its empty center left as a thin outline only) — Hexa's core idea at a
// glance.
func drawSplashMotif(box image.Rectangle) {
	cy := (box.Min.Y + box.Max.Y) / 2
	r := box.Dx() / 16
	if r < 6 {
		r = 6
	}
	dx := r * 866 / 1000 * 2 // horizontal distance between axial neighbours
	dy := r * 3 / 2          // vertical distance between axial neighbours

	groupW := box.Dx() / 3
	xs := [3]int{box.Min.X + groupW/2, box.Min.X + groupW + groupW/2, box.Min.X + 2*groupW + groupW/2}

	// Linje: 6 hexes in a straight row.
	{
		cx := xs[0]
		start := cx - dx*5/2
		for i := 0; i < 6; i++ {
			fillHex(image.Pt(start+i*dx, cy), r, ink.Black)
		}
	}

	// Triangel: rows of 3, 2, 1 (bottom to top), each hex edge-adjacent to
	// its neighbours exactly like game.triangleTemplate.
	{
		cx := xs[1]
		row0Y := cy + r
		row1Y := row0Y - dy
		row2Y := row1Y - dy
		row0X := cx - dx
		row1X := cx - dx/2
		row2X := cx
		for i := 0; i < 3; i++ {
			fillHex(image.Pt(row0X+i*dx, row0Y), r, ink.Black)
		}
		for i := 0; i < 2; i++ {
			fillHex(image.Pt(row1X+i*dx, row1Y), r, ink.Black)
		}
		fillHex(image.Pt(row2X, row2Y), r, ink.Black)
	}

	// Hexagon: a ring of 6 around an empty (outline-only) center.
	{
		cx := xs[2]
		drawHexOutline(image.Pt(cx, cy), r)
		type off struct{ x, y int }
		ring := []off{{0, -1}, {1, -1}, {1, 0}, {0, 1}, {-1, 1}, {-1, 0}}
		for _, o := range ring {
			hxp := cx + o.x*dx + o.y*(dx/2)
			hyp := cy + o.y*dy
			fillHex(image.Pt(hxp, hyp), r, ink.Black)
		}
	}
}
