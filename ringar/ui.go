package main

import (
	"image"
	"image/color"
	"math"

	ink "github.com/dennwc/inkview"

	"ringar/game"
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

	statusBarHeight = 90
	buttonBarHeight = 140
	boardMargin     = 10
	topMargin       = 34
)

// Layout maps between screen pixels and the board's 85 cube-coordinate
// points. Point centers are placed with the standard "pointy-top" axial hex
// formula, using (q,r) = (X,Z) of the cube coordinate — chosen specifically
// because Ringar's 6 neighbour directions (game.Directions) turn out to
// match the 6 standard axial neighbour offsets exactly under that mapping
// (verified in coords_test.go's adjacency-symmetry check), so this produces
// a true regular hex tessellation in which neighbouring board points are
// always visually adjacent, edge-sharing hexagons — not an approximation.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	ButtonBar image.Rectangle
	BoardArea image.Rectangle

	Size    int // hex circumradius in pixels (also the ring-move ray spacing)
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
	l.ButtonBar = image.Rect(0, H-topMargin-buttonBarHeight, screen.X, H-topMargin)

	avail := image.Rect(boardMargin, l.StatusBar.Max.Y+boardMargin,
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

	// A pointy-top hex of circumradius `size` sticks out an extra size*0.866
	// horizontally and a full extra `size` vertically beyond its center, on
	// each side of the cluster's bounding box of centers.
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
// is within one hex's worth of distance of it.
func (l *Layout) PointAt(q image.Point) (game.Point, bool) {
	if l.Size <= 0 {
		return game.Point{}, false
	}
	bestD := l.Size * l.Size * 3 // generous radius: about 1.7x the hex circumradius
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

// DrawBoard renders the 85-point hex grid, rings, markers, and every
// selection/legal-move/pending-row highlight for the current game state.
func DrawBoard(l *Layout, a *app) {
	s := a.gs
	ringR := l.Size * 82 / 100
	markerR := l.Size * 42 / 100
	if ringR < 3 {
		ringR = 3
	}
	if markerR < 2 {
		markerR = 2
	}

	for _, p := range l.order {
		drawHexOutline(l.Center(p), l.Size)
	}

	// Legal destination hints for a selected ring.
	if s.Phase == game.PhasePlaying && a.hasSelection {
		hintR := markerR/3 + 2
		for _, d := range game.RingMoves(s.Board, a.selected) {
			fillCircle(l.Center(d), hintR, ink.LightGray)
		}
	}

	// Pending-row highlighting: the whole completed run gets a bold outline;
	// once a 5-marker window is fixed (immediately, for a run of exactly 5,
	// or after the player taps to pick one on a longer run) those 5 points
	// get an extra-bold outline, and every ring the claiming side could
	// still choose to remove is flagged too.
	if s.Phase == game.PhaseRowPending {
		for _, p := range s.PendingRun {
			drawBoldHex(l.Center(p), l.Size, 2)
		}
		if s.PendingWindow != nil {
			for _, p := range s.PendingWindow {
				drawBoldHex(l.Center(p), l.Size, 4)
			}
			for p, side := range s.Board.Rings {
				if side == s.PendingSide {
					drawBoldHex(l.Center(p), ringR+6, 1)
				}
			}
		}
	}

	// Rings, then markers (they never share a point, order doesn't matter,
	// but drawing rings first keeps the smaller marker discs crisp).
	for p, side := range s.Board.Rings {
		drawRing(l.Center(p), ringR, side == game.Black)
	}
	for p, side := range s.Board.Markers {
		drawMarker(l.Center(p), markerR, side == game.Black)
	}

	// Selected ring: bold double outline.
	if s.Phase == game.PhasePlaying && a.hasSelection {
		drawBoldHex(l.Center(a.selected), l.Size, 3)
	}

	// Brief highlight of the marker trail flipped by the most recent move.
	if s.HasLast {
		for _, p := range s.LastFlipped {
			drawBoldHex(l.Center(p), markerR+6, 1)
		}
	}
}

// drawRing draws a ring piece: always an annulus (a ring's defining shape is
// its hole, distinguishing it at a glance from a marker), with a thick band
// for Black and a thin band for White — the same "thick=Black, thin=White"
// convention used for every disc/marker in this house, just applied to a
// ring shape instead of a solid disc.
func drawRing(c image.Point, r int, black bool) {
	band := r / 3
	if !black {
		band = 3
	}
	if band < 2 {
		band = 2
	}
	inner := r - band
	if inner < 1 {
		inner = 1
	}
	fillCircle(c, r, ink.Black)
	fillCircle(c, inner, ink.White)
}

// drawMarker draws a marker (flat disc): solid black, or a hollow disc with
// a thin black rim for White — identical to every other game's disc/man in
// this house.
func drawMarker(c image.Point, r int, black bool) {
	fillCircle(c, r, ink.Black)
	if !black {
		inner := r - r/8 - 2
		if inner < 1 {
			inner = 1
		}
		fillCircle(c, inner, ink.White)
	}
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

// --- Hex primitives (adapted from hex/ui.go's drawHexOutline/hexCorners/
// fillCircle/thickLine — the shared house convention for hex rendering;
// copied rather than imported since each game module is self-contained,
// exactly as fillDisc/isqrt are independently copied into every other game
// in this repo). ---------------------------------------------------------

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

func thickLine(a, b image.Point, col color.Color, w int) {
	for o := 0; o < w; o++ {
		ink.DrawLine(image.Pt(a.X, a.Y+o), image.Pt(b.X, b.Y+o), col)
		ink.DrawLine(image.Pt(a.X+o, a.Y), image.Pt(b.X+o, b.Y), col)
	}
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
// Per the spec, the built-in AI is explicitly framed as casual — hot-seat
// two-player is Ringar's primary experience — so there is a single "Mot
// dator" row rather than a tiered difficulty ladder like the other games.
var opponentChoices = []menuChoice{
	{game.OpponentHotseat, "2 spelare (hot-seat)"},
	{game.OpponentAI, "Mot dator (avslappnad motståndare)"},
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
	tw := ink.StringWidth("Ringar")
	ink.DrawString(image.Pt((screen.X-tw)/2, 56), "Ringar")
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
	f.Button.SetActive(ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	// Opponent rows fill the space between the subtitle and the Regler button.
	f.Menu.SetActive(ink.Black)
	rowH := 140
	top := 220
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

// rulesParagraphs is the rules text for Ringar, one entry per paragraph.
// rulesParagraphs is ordered with the most essential content FIRST — the win
// condition, the house's YINSH credit, and the honest "casual AI" framing —
// since DrawRules truncates whatever doesn't fit rather than scrolling;
// putting required content up front guarantees it always renders even on a
// screen too small to show every paragraph in full.
var rulesParagraphs = []string{
	"Mål: ta bort 3 av dina egna ringar från brädet — den som gör det först vinner.",
	"Baserat på YINSH (Kris Burm, GIPF-projektet).",
	"\"Mot dator\" är en avslappnad motståndare, inte tänkt att vara stark — hot-seat med två spelare är Ringars huvudsakliga spelsätt.",
	"Brädet är ett sexhörnigt fält med 85 punkter, med linjer i 3 riktningar (axlar).",
	"Placeringsfas: Vit och Svart turas om att placera ut sina 5 ringar var på tomma punkter. Vit börjar.",
	"Flyttfas: välj en ring. En markör i din färg släpps på ringens punkt, och ringen glider rakt längs en axel till en tom punkt — hur långt som helst över tomma punkter.",
	"Ringen får även hoppa över en rad markörer, men måste då landa på första lediga punkten direkt efter dem, och får aldrig hoppa över eller landa på en annan ring.",
	"Varje markör som hoppas över vänds till din färg — som i Othello, fast som en direkt följd av ringens glidning.",
	"Rad: fem markörer av samma färg i rad längs en axel låter ägaren ta bort dem plus en av sina egna ringar.",
	"Blir raden sex eller fler markörer lång väljer du vilka fem intilliggande som tas bort — tryck på en markör i raden. Resten blir kvar i spel.",
	"Ringen du offrar kan vara vilken som helst av dina egna, var som helst — färre ringar ger sämre rörlighet.",
	"Fullbordar ett drag rader för båda samtidigt löser den som drog sin egen rad först.",
	"Tryck på en ring för att välja den (tillåtna mål markeras), tryck sedan på ett mål för att flytta dit.",
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

// drawSplashMotif draws a seven-hex honeycomb flower (echoing hex's splash
// idea) with 2 rings and a short trail of black/white markers, with one row
// highlighted — Ringar's core idea (ring move -> marker flip -> row) at a
// glance.
func drawSplashMotif(box image.Rectangle) {
	cx := (box.Min.X + box.Max.X) / 2
	cy := (box.Min.Y + box.Max.Y) / 2
	r := box.Dx() / 8
	const hx = 866
	dx := r * hx / 1000 * 2
	dy := r * 3 / 2

	// A center hex plus the ring of six neighbors around it.
	type off struct{ x, y int }
	ring := []off{{0, -1}, {1, -1}, {1, 0}, {0, 1}, {-1, 1}, {-1, 0}}
	centers := []image.Point{{cx, cy}}
	for _, o := range ring {
		hxp := cx + o.x*dx + o.y*(dx/2)
		hyp := cy + o.y*dy
		centers = append(centers, image.Pt(hxp, hyp))
	}
	for _, c := range centers {
		drawHexOutline(c, r)
	}

	// One highlighted row: the bottom-left neighbour, center, and top-right
	// neighbour form a straight axis line through the flower. A doubled hex
	// outline is nearly invisible at this small motif scale (it mostly just
	// overdraws the ordinary outline), so the highlight is instead a bold
	// connecting line straight through the 3 centers, drawn UNDER the
	// rings/markers so it reads as the row's axis without obscuring them.
	rowIdx := []int{5, 0, 2} // ring[4]=(-1,1) at index 5, center at 0, ring[1]=(1,-1) at index 2
	thickLine(centers[rowIdx[0]], centers[rowIdx[2]], ink.Black, 3)

	// A ring at the center, a ring at one corner, and a short black/white
	// marker trail along the highlighted row.
	drawRing(centers[0], r*7/10, true)
	drawRing(centers[3], r*7/10, false)
	drawMarker(centers[5], r*4/10, true)
	drawMarker(centers[2], r*4/10, false)
}
