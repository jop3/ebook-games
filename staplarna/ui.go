package main

import (
	"image"
	"image/color"
	"math"

	ink "github.com/dennwc/inkview"

	"staplarna/game"
)

// Fonts held open for the app lifetime (opened once — never per draw).
type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Small  *ink.Font
	Badge  *ink.Font // tiny font for the stack-height corner numeral
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 34, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 38, true),
		Small:  ink.OpenFont(ink.DefaultFont, 26, true),
		Badge:  ink.OpenFont(ink.DefaultFontBold, 22, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Small, f.Badge} {
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

// --- Layout --------------------------------------------------------------

const (
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	statusBarHeight = 96
	setupBarHeight  = 116
	buttonBarHeight = 140
	boardMargin     = 10
	topMargin       = 34
)

// Layout maps between screen pixels and the board's 61 cube-coordinate
// points. Point centers are placed with the standard "pointy-top" axial hex
// formula, using (q,r) = (X,Z) of the cube coordinate — the same mapping
// ringar/ui.go uses, since Staplarna's Directions (copied from ringar, see
// game/coords.go) are identical, so this produces a true regular hex
// tessellation in which neighbouring board points are always visually
// adjacent, edge-sharing hexagons.
//
// The SetupBar area is always reserved (whether or not the game is currently
// in PhaseSetup) so the board never resizes/jumps when the game leaves the
// setup phase — during play it is simply left blank.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	SetupBar  image.Rectangle
	ButtonBar image.Rectangle
	BoardArea image.Rectangle

	Size    int // hex circumradius in pixels
	order   []game.Point
	centers map[game.Point]image.Point
}

// hexUnit returns the UNIT (size=1) pixel position of cube point p under the
// axial mapping (q,r) = (p.X, p.Z).
func hexUnit(p game.Point) (float64, float64) {
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
	l.SetupBar = image.Rect(0, l.StatusBar.Max.Y, screen.X, l.StatusBar.Max.Y+setupBarHeight)
	l.ButtonBar = image.Rect(0, H-topMargin-buttonBarHeight, screen.X, H-topMargin)

	avail := image.Rect(boardMargin, l.SetupBar.Max.Y+boardMargin,
		screen.X-boardMargin, l.ButtonBar.Min.Y-boardMargin)

	pts := game.AllPoints()
	minPx, maxPx := math.Inf(1), math.Inf(-1)
	minPy, maxPy := math.Inf(1), math.Inf(-1)
	unit := make(map[game.Point][2]float64, len(pts))
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
	l.centers = make(map[game.Point]image.Point, len(pts))
	for _, p := range pts {
		u := unit[p]
		cx := int(math.Round(size*u[0] + originX))
		cy := int(math.Round(size*u[1] + originY))
		l.centers[p] = image.Pt(cx, cy)
	}
	l.BoardArea = avail
	return l
}

// Center returns the pixel center of board point p.
func (l *Layout) Center(p game.Point) image.Point { return l.centers[p] }

// PointAt returns the board point whose center is nearest tap point q, if q
// is within roughly one hex's worth of distance of it.
func (l *Layout) PointAt(q image.Point) (game.Point, bool) {
	if l.Size <= 0 {
		return game.Point{}, false
	}
	bestD := l.Size * l.Size * 3
	var best game.Point
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

// hexCellRect returns a square bounding box centered on c, used to place a
// piece glyph inside board point p's hex (an easy approximation that keeps
// every glyph comfortably inside the hex without touching its edges).
func hexCellRect(c image.Point, r int) image.Rectangle {
	return image.Rect(c.X-r, c.Y-r, c.X+r, c.Y+r)
}

// --- Rendering -------------------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

func DrawStatus(l *Layout, a *app, f *Fonts) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	top := image.Rect(l.StatusBar.Min.X, l.StatusBar.Min.Y, l.StatusBar.Max.X, l.StatusBar.Min.Y+52)
	drawCenteredString(top, a.statusText(), 32)
	if a.gs.Phase != game.PhaseDone {
		f.Small.SetActive(ink.Black)
		bottom := image.Rect(l.StatusBar.Min.X, l.StatusBar.Min.Y+52, l.StatusBar.Max.X, l.StatusBar.Max.Y)
		drawCenteredString(bottom, a.materialText(), 26)
	}
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// Chip is a tappable type-selector button in the setup bar.
type Chip struct {
	Rect image.Rectangle
	Type game.PieceType
}

// DrawSetupSelector renders the 3 Tzaar/Tzarra/Tott chips (shape + remaining
// count for the side currently placing) during PhaseSetup, and returns the
// tappable chips (only for types still available to place). Outside
// PhaseSetup it just clears the bar to white and returns nil.
func DrawSetupSelector(l *Layout, a *app) []Chip {
	ink.FillArea(l.SetupBar, ink.White)
	ink.DrawLine(image.Pt(l.SetupBar.Min.X, l.SetupBar.Max.Y),
		image.Pt(l.SetupBar.Max.X, l.SetupBar.Max.Y), ink.Black)
	s := a.gs
	if s.Phase != game.PhaseSetup {
		return nil
	}
	turn := s.Turn
	margin := 24
	gap := 16
	n := len(game.AllTypes)
	usableW := l.SetupBar.Dx() - 2*margin
	cw := (usableW - gap*(n-1)) / n
	y0 := l.SetupBar.Min.Y + 8
	ch := l.SetupBar.Dy() - 16

	chips := make([]Chip, 0, n)
	for i, t := range game.AllTypes {
		x0 := l.SetupBar.Min.X + margin + i*(cw+gap)
		r := image.Rect(x0, y0, x0+cw, y0+ch)
		remaining := s.RemainingCount(turn, t)

		ink.DrawRect(r, ink.Black)
		if a.setupType == t {
			ink.DrawRect(pad(r, 1), ink.Black)
			ink.DrawRect(pad(r, 2), ink.Black)
		}

		glyphSide := ch - 16
		glyphBox := image.Rect(r.Min.X+8, r.Min.Y+8, r.Min.X+8+glyphSide, r.Min.Y+8+glyphSide)
		drawTypeGlyph(glyphBox, t, true)

		f := a.fonts.Small
		f.SetActive(ink.Black)
		txt := itoa(remaining)
		tw := ink.StringWidth(txt)
		tx := r.Max.X - tw - 14
		if tx < glyphBox.Max.X+6 {
			tx = glyphBox.Max.X + 6
		}
		ink.DrawString(image.Pt(tx, r.Min.Y+(r.Dy()-26)/2), txt)

		if remaining > 0 {
			chips = append(chips, Chip{Rect: r, Type: t})
		}
	}
	return chips
}

// DrawBoard renders the hex grid, every stack (shape by type, corner numeral
// for height > 1), the current selection (if any) with its legal exact-
// distance landing hexes, and a brief marker over the most recent move.
func DrawBoard(l *Layout, a *app) {
	s := a.gs
	for _, p := range l.order {
		drawHexOutline(l.Center(p), l.Size)
	}

	for _, p := range l.order {
		st, ok := s.Board.At(p)
		if !ok {
			continue
		}
		cell := hexCellRect(l.Center(p), l.Size)
		drawTypeGlyph(cell, st.Type, st.Owner == game.Black)
		if st.Height > 1 {
			drawHeightBadge(cell, st.Height, a.fonts.Badge)
		}
	}

	// Legal destination hints for a selected stack (exact-distance landings),
	// drawn AFTER the pieces so hints on occupied cells stay visible: the
	// glyphs are opaque and would fully cover a dot underneath, leaving
	// capture/merge destinations looking unmarked. Empty landings get a grey
	// dot; occupied ones a bold hex ring around the piece.
	if s.Phase == game.PhasePlaying && a.hasSelection {
		hintR := l.Size * 3 / 10
		if hintR < 2 {
			hintR = 2
		}
		for _, to := range game.DestinationsFrom(s.Board, a.selected) {
			if _, occupied := s.Board.At(to); occupied {
				drawBoldHex(l.Center(to), l.Size, 2)
			} else {
				fillCircle(l.Center(to), hintR, ink.LightGray)
			}
		}
	}

	// Selected stack: bold double outline.
	if s.Phase == game.PhasePlaying && a.hasSelection {
		drawBoldHex(l.Center(a.selected), l.Size, 3)
	}

	// Brief highlight of the most recent move's destination.
	if s.HasLast {
		drawBoldHex(l.Center(s.LastTo), l.Size, 1)
	}
}

// drawTypeGlyph draws a piece's shape (Tzaar = large circle, Tzarra = square,
// Tott = triangle) inside cell: a solid black fill for Black, a hollow
// (white-centered) fill for White — the same "thick=Black, thin/hollow=
// White" convention used for every disc/marker/ring elsewhere in this house.
func drawTypeGlyph(cell image.Rectangle, t game.PieceType, black bool) {
	switch t {
	case game.Tzaar:
		drawCirclePiece(cell, black)
	case game.Tzarra:
		drawSquarePiece(cell, black)
	default:
		drawTrianglePiece(cell, black)
	}
}

func drawCirclePiece(cell image.Rectangle, black bool) {
	r := pad(cell, cell.Dx()/8)
	fillDisc(r, ink.Black)
	if !black {
		inner := pad(r, r.Dx()/8+2)
		fillDisc(inner, ink.White)
	}
}

func drawSquarePiece(cell image.Rectangle, black bool) {
	r := pad(cell, cell.Dx()/6)
	ink.FillArea(r, ink.Black)
	if !black {
		inner := pad(r, r.Dx()/8+2)
		ink.FillArea(inner, ink.White)
	}
}

func drawTrianglePiece(cell image.Rectangle, black bool) {
	r := pad(cell, cell.Dx()/8)
	fillTriangle(triCorners(r), ink.Black)
	if !black {
		inner := pad(r, r.Dx()/7+2)
		fillTriangle(triCorners(inner), ink.White)
	}
}

// triCorners returns the 3 corners of an upward-pointing triangle inscribed
// in r: apex at top-center, base corners at the bottom-left/right.
func triCorners(r image.Rectangle) [3]image.Point {
	apex := image.Pt((r.Min.X+r.Max.X)/2, r.Min.Y)
	left := image.Pt(r.Min.X, r.Max.Y)
	right := image.Pt(r.Max.X, r.Max.Y)
	return [3]image.Point{apex, left, right}
}

// fillTriangle fills the triangle defined by pts using horizontal scanlines
// (a standard sorted-by-Y edge-walk fill), the polygon analogue of
// fillDisc/fillCircle used for every round piece elsewhere in this house.
func fillTriangle(pts [3]image.Point, col color.Color) {
	p := pts
	if p[0].Y > p[1].Y {
		p[0], p[1] = p[1], p[0]
	}
	if p[1].Y > p[2].Y {
		p[1], p[2] = p[2], p[1]
	}
	if p[0].Y > p[1].Y {
		p[0], p[1] = p[1], p[0]
	}
	lerpX := func(a, b image.Point, y int) int {
		if a.Y == b.Y {
			return a.X
		}
		return a.X + (y-a.Y)*(b.X-a.X)/(b.Y-a.Y)
	}
	for y := p[0].Y; y <= p[2].Y; y++ {
		xLong := lerpX(p[0], p[2], y)
		var xShort int
		if y <= p[1].Y {
			if p[0].Y == p[1].Y {
				xShort = p[1].X
			} else {
				xShort = lerpX(p[0], p[1], y)
			}
		} else {
			xShort = lerpX(p[1], p[2], y)
		}
		x0, x1 := xLong, xShort
		if x0 > x1 {
			x0, x1 = x1, x0
		}
		ink.DrawLine(image.Pt(x0, y), image.Pt(x1, y), col)
	}
}

// drawHeightBadge overlays a small boxed numeral at a piece's top-right
// corner showing its stack height, used whenever height > 1 (a lone piece,
// height 1, shows no badge at all).
func drawHeightBadge(cell image.Rectangle, height int, f *ink.Font) {
	bw := cell.Dx() * 2 / 5
	if bw < 22 {
		bw = 22
	}
	bh := bw * 3 / 4
	badge := image.Rect(cell.Max.X-bw, cell.Min.Y, cell.Max.X, cell.Min.Y+bh)
	ink.FillArea(badge, ink.White)
	ink.DrawRect(badge, ink.Black)
	f.SetActive(ink.Black)
	txt := itoa(height)
	tw := ink.StringWidth(txt)
	ink.DrawString(image.Pt(badge.Min.X+(badge.Dx()-tw)/2, badge.Min.Y+1), txt)
}

// drawBoldHex draws n concentric hex outlines 2px apart, centered at c —
// used for selection/highlight emphasis (thicker = more emphasis).
func drawBoldHex(c image.Point, r, n int) {
	for i := 0; i < n; i++ {
		rr := r - i*2
		if rr < 2 {
			break
		}
		drawHexOutline(c, rr)
	}
}

// --- Hex + shape primitives (adapted from ringar/ui.go's identical hex
// drawing helpers — the shared house convention for hex rendering; copied
// rather than imported since each game module is self-contained). ---------

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

func fillDisc(r image.Rectangle, col color.Color) {
	fillCircle(image.Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2), minInt(r.Dx(), r.Dy())/2, col)
}

func fillCircle(c image.Point, rad int, col color.Color) {
	rr := rad * rad
	for dy := -rad; dy <= rad; dy++ {
		hw := isqrt(rr - dy*dy)
		ink.DrawLine(image.Pt(c.X-hw, c.Y+dy), image.Pt(c.X+hw, c.Y+dy), col)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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
	tw := ink.StringWidth("Staplarna")
	ink.DrawString(image.Pt((screen.X-tw)/2, 56), "Staplarna")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 30, true)
	sub.SetActive(ink.Black)
	subT := "Välj läge"
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
	f.Button.SetActive(ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	// Opponent rows fill the space between the subtitle and the Regler button.
	f.Menu.SetActive(ink.Black)
	rowH := 140
	top := 210
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

// rulesParagraphs is the rules text for Staplarna, one entry per paragraph,
// ordered with the most essential content FIRST (goal, board, setup, core
// movement rule) — DrawRules truncates whatever doesn't fit rather than
// scrolling, so required content up front guarantees it always renders even
// on a screen too small to show every paragraph in full.
var rulesParagraphs = []string{
	"Mål: din motståndare har noll kvar av NÅGON av de tre pjästyperna (Tzaar, Tzarra eller Tott) — eller din motståndare saknar lagligt drag.",
	"Baserat på TZAAR (Kris Burm, GIPF-projektet).",
	"Brädet är ett sexhörnigt fält med 61 rutor (5 rutor per kant).",
	"Placeringsfas: Svart och Vit turas om att placera ut sina 30 pjäser var (6 Tzaar, 9 Tzarra, 15 Tott) på tomma rutor, i valfritt mönster. Svart börjar.",
	"Flyttfas: välj en av dina staplar. Den flyttar rakt i en av de 6 riktningarna EXAKT så många rutor som stapelns höjd — aldrig kortare, aldrig längre. En ensam pjäs (höjd 1) flyttar alltså bara en ruta.",
	"En stapel får aldrig passera genom en annan stapel, vare sig egen eller fiendens — varje ruta fram till (men inte medräknat) målrutan måste vara helt tom.",
	"Landar du på en tom ruta: vanlig flytt. Landar du på en egen stapel: de slås ihop till en enda, högre stapel. Landar du på en fiendestapel som INTE är högre än din egen: den fångas och tas bort i sin helhet — även pjäser begravda längre ner i stapeln.",
	"Fångst är INTE obligatoriskt: en vanlig flytt utan fångst är fortfarande laglig även om en fångst är möjlig någon annanstans på brädet.",
	"Placeringsfas: tryck på en av de tre pjästyperna längst upp för att välja vilken du placerar härnäst (antal kvar visas på rutan), tryck sedan på en tom ruta.",
	"Flyttfas: tryck på en av dina staplar för att välja den — lagliga mål, på exakt rätt avstånd, markeras. Tryck sedan på ett av dem för att flytta dit. Tryck på samma stapel igen för att avmarkera.",
	"Pjästyp visas som form: Tzaar = stor cirkel, Tzarra = fyrkant, Tott = triangel. En liten siffra i hörnet visar stapelns höjd när den är mer än 1.",
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
	y := 140
	lineH := 36
	paraGap := 14
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

type motifFunc func(box image.Rectangle)

// DrawSplash renders the start screen: the game title, a large line-art motif,
// and a "tap to start" hint — echoing the built-in chess app's opening screen.
func DrawSplash(screen image.Point, f *Fonts, title string, motif motifFunc) {
	ink.ClearScreen()
	H := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 76, true)
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

// drawSplashMotif draws TZAAR's core idea at a glance: three small hexes
// holding pieces of the three different shapes/heights — a Tzaar (circle,
// height 1), a Tzarra (square, height 2, corner numeral), and a Tott
// (triangle, height 1) — with a capture-distance arrow from the height-2
// stack to the piece exactly 2 hexes away, showing that a stack moves/
// captures EXACTLY as far as it is tall.
func drawSplashMotif(box image.Rectangle) {
	cy := (box.Min.Y + box.Max.Y) / 2
	cx := (box.Min.X + box.Max.X) / 2
	r := box.Dx() / 10
	gap := r * 9 / 4
	centers := [3]image.Point{{cx - 2*gap, cy}, {cx, cy}, {cx + 2*gap, cy}}
	for _, c := range centers {
		drawHexOutline(c, r)
	}

	drawCirclePiece(hexCellRect(centers[0], r), true)
	drawSquarePiece(hexCellRect(centers[1], r), false)
	bf := ink.OpenFont(ink.DefaultFontBold, 24, true)
	drawHeightBadge(hexCellRect(centers[1], r), 2, bf)
	bf.Close()
	drawTrianglePiece(hexCellRect(centers[2], r), true)

	// Capture-distance arrow: from the height-2 stack to the piece 2 hexes
	// away, since that stack's only legal move/capture distance is exactly 2.
	arrowY := cy - r*2
	tip := image.Pt(centers[2].X-r/2, arrowY)
	ink.DrawLine(image.Pt(centers[1].X, arrowY), tip, ink.Black)
	drawArrowHead(tip, true, r)
}

// drawArrowHead draws a small chevron ("›" if pointRight, "‹" otherwise)
// whose tip sits at c — used to cap the splash motif's capture-distance line.
func drawArrowHead(c image.Point, pointRight bool, size int) {
	dx := size
	if !pointRight {
		dx = -dx
	}
	tailA := image.Pt(c.X-dx/2, c.Y-size/2)
	tailB := image.Pt(c.X-dx/2, c.Y+size/2)
	ink.DrawLine(c, tailA, ink.Black)
	ink.DrawLine(c, tailB, ink.Black)
}
