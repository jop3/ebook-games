package main

import (
	"image"
	"image/color"
	"math"

	ink "github.com/dennwc/inkview"

	"sushi/game"
)

// Fonts held open for the app lifetime (opened once — never per draw).
type Fonts struct {
	Status *ink.Font
	Title  *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Small  *ink.Font
	Tiny   *ink.Font
	Mono   *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 34, true),
		Title:  ink.OpenFont(ink.DefaultFontBold, 40, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 36, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 38, true),
		Small:  ink.OpenFont(ink.DefaultFont, 26, true),
		Tiny:   ink.OpenFont(ink.DefaultFont, 22, true),
		Mono:   ink.OpenFont(ink.DefaultFontMono, 26, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Title, f.Button, f.Menu, f.Small, f.Tiny, f.Mono} {
		if fn != nil {
			fn.Close()
		}
	}
}

func pad(r image.Rectangle, n int) image.Rectangle {
	return image.Rect(r.Min.X+n, r.Min.Y+n, r.Max.X-n, r.Max.Y-n)
}

func center(r image.Rectangle) (int, int) {
	return (r.Min.X + r.Max.X) / 2, (r.Min.Y + r.Max.Y) / 2
}

func drawCenteredString(r image.Rectangle, s string, approxHeight int) {
	w := ink.StringWidth(s)
	x := r.Min.X + (r.Dx()-w)/2
	y := r.Min.Y + (r.Dy()-approxHeight)/2
	ink.DrawString(image.Pt(x, y), s)
}

func drawLeftString(r image.Rectangle, s string, approxHeight int) {
	x := r.Min.X + 16
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

func signed(n int) string {
	if n > 0 {
		return "+" + itoa(n)
	}
	return itoa(n)
}

// --- Layout ------------------------------------------------------------

const (
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	topMargin       = 20
	sideMargin      = 24
	statusBarHeight = 64
	tableauHeight   = 150
	chopBarHeight   = 84
	buttonBarHeight = 130
)

// Layout holds every screen region for the drafting screen. Unlike a board
// game, there's no cell grid to map — this is a fresh stacked-panel layout:
// status bar, "your tableau" summary, your hand grid, an (optionally shown)
// chopsticks button, opponents' compact tableaus, and the bottom button bar.
type Layout struct {
	Screen        image.Rectangle
	StatusBar     image.Rectangle
	TableauArea   image.Rectangle
	HandArea      image.Rectangle
	ChopBar       image.Rectangle
	OpponentsArea image.Rectangle
	ButtonBar     image.Rectangle
}

func NewLayout(screen image.Point) Layout {
	H := usableH
	l := Layout{Screen: image.Rect(0, 0, screen.X, H)}
	l.StatusBar = image.Rect(0, topMargin, screen.X, topMargin+statusBarHeight)
	l.ButtonBar = image.Rect(0, H-topMargin-buttonBarHeight, screen.X, H-topMargin)

	opponentsHeight := 176
	l.OpponentsArea = image.Rect(0, l.ButtonBar.Min.Y-opponentsHeight, screen.X, l.ButtonBar.Min.Y)
	l.ChopBar = image.Rect(0, l.OpponentsArea.Min.Y-chopBarHeight, screen.X, l.OpponentsArea.Min.Y)
	l.TableauArea = image.Rect(0, l.StatusBar.Max.Y, screen.X, l.StatusBar.Max.Y+tableauHeight)
	l.HandArea = image.Rect(0, l.TableauArea.Max.Y, screen.X, l.ChopBar.Min.Y)
	return l
}

// --- Buttons -------------------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

// DrawButtonBar lays out and draws a row of equal-width buttons inside bar.
func DrawButtonBar(bar *image.Rectangle, labels []string, f *Fonts) []Button {
	ink.FillArea(*bar, ink.White)
	ink.DrawLine(image.Pt(bar.Min.X, bar.Min.Y), image.Pt(bar.Max.X, bar.Min.Y), ink.Black)
	f.Button.SetActive(ink.Black)
	n := len(labels)
	if n == 0 {
		return nil
	}
	gap := 20
	usableW := bar.Dx() - 2*sideMargin
	bw := (usableW - gap*(n+1)) / n
	bh := bar.Dy() - 2*gap
	buttons := make([]Button, n)
	for i, label := range labels {
		x0 := bar.Min.X + sideMargin + gap + i*(bw+gap)
		y0 := bar.Min.Y + gap
		r := image.Rect(x0, y0, x0+bw, y0+bh)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawCenteredString(r, label, 36)
		buttons[i] = Button{Rect: r, Label: label}
	}
	return buttons
}

// --- Drawing primitives used by the card icons --------------------------

func fillDiscAt(cx, cy, r int, col color.Color) {
	if r <= 0 {
		return
	}
	rr := r * r
	for dy := -r; dy <= r; dy++ {
		hw := isqrt(rr - dy*dy)
		ink.DrawLine(image.Pt(cx-hw, cy+dy), image.Pt(cx+hw, cy+dy), col)
	}
}

func fillHalfDiscUp(cx, cy, r int, col color.Color) {
	rr := r * r
	for dy := -r; dy <= 0; dy++ {
		hw := isqrt(rr - dy*dy)
		ink.DrawLine(image.Pt(cx-hw, cy+dy), image.Pt(cx+hw, cy+dy), col)
	}
}

func fillEllipse(cx, cy, rx, ry int, col color.Color) {
	if rx <= 0 || ry <= 0 {
		return
	}
	for dy := -ry; dy <= ry; dy++ {
		t := 1 - float64(dy*dy)/float64(ry*ry)
		if t < 0 {
			continue
		}
		hw := int(float64(rx) * math.Sqrt(t))
		ink.DrawLine(image.Pt(cx-hw, cy+dy), image.Pt(cx+hw, cy+dy), col)
	}
}

func fillTriUp(cx, top, halfW, height int, col color.Color) {
	for dy := 0; dy <= height; dy++ {
		w := halfW * dy / height
		y := top + dy
		ink.DrawLine(image.Pt(cx-w, y), image.Pt(cx+w, y), col)
	}
}

// drawThickDiag draws a near-diagonal thick stroke by stacking several
// parallel 1px lines offset vertically — a good enough approximation at
// these icon sizes for lines that run mostly left-right (see munkar's
// similar drawLine helper for the same trick applied to its glyph cells).
func drawThickDiag(x0, y0, x1, y1, th int, col color.Color) {
	for o := -th / 2; o <= th/2; o++ {
		ink.DrawLine(image.Pt(x0, y0+o), image.Pt(x1, y1+o), col)
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

// --- Card icons ------------------------------------------------------------
//
// Eight simple greyscale line-art/fill glyphs, one per CardKind, deliberately
// built from different silhouette FAMILIES (blocky+dots, ring, oval+hatch,
// diagonal bars, dome, solid triangle, cup outline, thin parallel sticks) so
// that no two are confusable at card size. Nigiri and Maki additionally
// encode their subtype/count as a number of small pips, per the spec.

func drawCardIcon(box image.Rectangle, c game.Card) {
	switch c.Kind {
	case game.KindNigiri:
		drawNigiri(box, c.N)
	case game.KindMaki:
		drawMaki(box, c.N)
	case game.KindTempura:
		drawTempura(box)
	case game.KindSashimi:
		drawSashimi(box)
	case game.KindDumpling:
		drawDumpling(box)
	case game.KindWasabi:
		drawWasabi(box)
	case game.KindChopsticks:
		drawChopsticks(box)
	case game.KindPudding:
		drawPudding(box)
	}
}

// drawNigiri: a white rice block (black outline) topped by a black nori
// belt, with N small pips above it for egg/salmon/squid (1/2/3).
func drawNigiri(box image.Rectangle, n int) {
	w, h := box.Dx(), box.Dy()
	riceTop := box.Min.Y + h*48/100
	rice := image.Rect(box.Min.X+w/10, riceTop, box.Max.X-w/10, box.Max.Y-h/14)
	ink.FillArea(rice, ink.White)
	ink.DrawRect(rice, ink.Black)
	ink.DrawRect(pad(rice, 1), ink.Black)
	beltH := rice.Dy() * 30 / 100
	belt := image.Rect(rice.Min.X, rice.Min.Y, rice.Max.X, rice.Min.Y+beltH)
	ink.FillArea(belt, ink.Black)
	if n > 0 {
		cx, _ := center(box)
		cy := box.Min.Y + h*22/100
		r := w / 13
		if r < 3 {
			r = 3
		}
		gap := r * 3
		startX := cx - gap*(n-1)/2
		for i := 0; i < n; i++ {
			fillDiscAt(startX+i*gap, cy, r, ink.Black)
		}
	}
}

// drawMaki: a nori ring around white rice, with N pips at the center (1/2/3
// icon values).
func drawMaki(box image.Rectangle, n int) {
	cx, cy := center(box)
	rOuter := box.Dx() / 2
	if box.Dy()/2 < rOuter {
		rOuter = box.Dy() / 2
	}
	rOuter -= 2
	rInner := rOuter * 60 / 100
	fillDiscAt(cx, cy, rOuter, ink.Black)
	fillDiscAt(cx, cy, rInner, ink.White)
	if n > 0 {
		r := rInner / 4
		if r < 3 {
			r = 3
		}
		gap := r * 3
		switch n {
		case 1:
			fillDiscAt(cx, cy, r, ink.Black)
		case 2:
			fillDiscAt(cx-gap/2, cy, r, ink.Black)
			fillDiscAt(cx+gap/2, cy, r, ink.Black)
		default:
			fillDiscAt(cx, cy-gap/2, r, ink.Black)
			fillDiscAt(cx-gap/2, cy+gap/3, r, ink.Black)
			fillDiscAt(cx+gap/2, cy+gap/3, r, ink.Black)
		}
	}
}

// drawTempura: an outlined oval with 3 short diagonal hatch marks
// suggesting a battered, fried texture.
func drawTempura(box image.Rectangle) {
	cx, cy := center(box)
	rx := box.Dx() * 40 / 100
	ry := box.Dy() * 30 / 100
	fillEllipse(cx, cy, rx, ry, ink.Black)
	fillEllipse(cx, cy, rx-3, ry-3, ink.White)
	for _, dx := range []int{-rx / 2, 0, rx / 2} {
		ink.DrawLine(image.Pt(cx+dx-ry/3, cy-ry/2), image.Pt(cx+dx+ry/3, cy+ry/2), ink.Black)
	}
}

// drawSashimi: 3 fanned diagonal slices (thick parallel strokes), like
// fish sliced and laid out overlapping.
func drawSashimi(box image.Rectangle) {
	w, h := box.Dx(), box.Dy()
	th := h / 8
	if th < 3 {
		th = 3
	}
	for i := 0; i < 3; i++ {
		off := (i - 1) * h * 30 / 100
		x0 := box.Min.X + w*22/100
		y0 := box.Min.Y + h*72/100 + off
		x1 := box.Max.X - w*22/100
		y1 := box.Min.Y + h*28/100 + off
		drawThickDiag(x0, y0, x1, y1, th, ink.Black)
	}
}

// drawDumpling: a pleated dome (hollow half-disc outline) with a few
// radial pleat ticks along its curved top edge.
func drawDumpling(box image.Rectangle) {
	cx := (box.Min.X + box.Max.X) / 2
	baseY := box.Max.Y - box.Dy()/10
	r := box.Dx() * 40 / 100
	fillHalfDiscUp(cx, baseY, r, ink.Black)
	fillHalfDiscUp(cx, baseY, r-4, ink.White)
	for _, deg := range []float64{-55, -20, 20, 55} {
		rad := deg * math.Pi / 180
		x0 := cx + int(float64(r-4)*math.Sin(rad))
		y0 := baseY - int(float64(r-4)*math.Cos(rad))
		x1 := cx + int(float64(r+3)*math.Sin(rad))
		y1 := baseY - int(float64(r+3)*math.Cos(rad))
		ink.DrawLine(image.Pt(x0, y0), image.Pt(x1, y1), ink.Black)
	}
}

// drawWasabi: a solid filled triangle — the only solid-fill, sharp-cornered
// shape among the 8, so it can't be confused with any rounded card.
func drawWasabi(box image.Rectangle) {
	cx, _ := center(box)
	top := box.Min.Y + box.Dy()*15/100
	height := box.Dy() * 65 / 100
	halfW := box.Dx() * 34 / 100
	fillTriUp(cx, top, halfW, height, ink.Black)
}

// drawPudding: an outlined trapezoid cup (wider at the rim) with a rim line
// and a small swirl, standing in for a ramekin of custard.
func drawPudding(box image.Rectangle) {
	w, h := box.Dx(), box.Dy()
	cx, _ := center(box)
	topY := box.Min.Y + h/6
	botY := box.Max.Y - h/10
	topHW := w * 36 / 100
	botHW := w * 24 / 100
	tl := image.Pt(cx-topHW, topY)
	tr := image.Pt(cx+topHW, topY)
	bl := image.Pt(cx-botHW, botY)
	br := image.Pt(cx+botHW, botY)
	ink.DrawLine(tl, tr, ink.Black)
	ink.DrawLine(tl, bl, ink.Black)
	ink.DrawLine(tr, br, ink.Black)
	ink.DrawLine(bl, br, ink.Black)
	rimY := topY + h/10
	ink.DrawLine(image.Pt(cx-topHW+3, rimY), image.Pt(cx+topHW-3, rimY), ink.Black)
	midY := (topY + botY) / 2
	ink.DrawLine(image.Pt(cx-w/10, midY+h/12), image.Pt(cx+w/10, midY-h/12), ink.Black)
}

// drawChopsticks: two thin parallel diagonal sticks, tips together at the
// top right — a plain-line glyph, unlike every other (filled/outlined)
// icon, so it reads instantly as "different."
func drawChopsticks(box image.Rectangle) {
	w, h := box.Dx(), box.Dy()
	drawThickDiag(box.Min.X+w*22/100, box.Max.Y-h*10/100, box.Max.X-w*18/100, box.Min.Y+h*14/100, 4, ink.Black)
	drawThickDiag(box.Min.X+w*34/100, box.Max.Y-h*10/100, box.Max.X-w*6/100, box.Min.Y+h*22/100, 4, ink.Black)
}

// cardShortLabel is the compact Swedish caption drawn under a card's icon.
func cardShortLabel(c game.Card) string {
	switch c.Kind {
	case game.KindNigiri:
		switch c.N {
		case 1:
			return "Ägg"
		case 2:
			return "Lax"
		case 3:
			return "Bläckfisk"
		}
		return "Nigiri"
	case game.KindMaki:
		return "Maki " + itoa(c.N)
	case game.KindTempura:
		return "Tempura"
	case game.KindSashimi:
		return "Sashimi"
	case game.KindDumpling:
		return "Dumpling"
	case game.KindWasabi:
		return "Wasabi"
	case game.KindChopsticks:
		return "Ätpinnar"
	case game.KindPudding:
		return "Pudding"
	}
	return "?"
}

// --- Drafting screen ---------------------------------------------------

// DrawGameScreen renders the whole drafting screen and returns the tappable
// rects for the human's current hand, index-aligned with gs.Players[0].Hand.
func DrawGameScreen(l *Layout, a *app) []image.Rectangle {
	gs := a.gs
	f := a.fonts

	drawStatusBar(l, gs, f)
	drawTableauSummary(l, gs, f)
	handRects := drawHand(l, a)
	drawChopBar(l, a)
	drawOpponents(l, gs, f)
	return handRects
}

func drawStatusBar(l *Layout, gs *game.State, f *Fonts) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	cardsLeft := gs.HandLen()
	text := "Runda " + itoa(gs.Round) + "/" + itoa(game.NumRounds) +
		"   ·   " + itoa(cardsLeft) + " kort kvar   ·   Poäng " + itoa(gs.Players[0].Score)
	drawCenteredString(l.StatusBar, text, 34)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y), image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// tableauCounts groups a tableau by kind, returning (kind, representative N,
// count) triples in a fixed, stable display order.
type tableauChip struct {
	kind  game.CardKind
	n     int // representative subtype for the icon glyph (0 = generic)
	count int
}

var chipOrder = []game.CardKind{
	game.KindNigiri, game.KindWasabi, game.KindTempura, game.KindSashimi,
	game.KindDumpling, game.KindMaki, game.KindChopsticks, game.KindPudding,
}

func tableauChips(tableau []game.Card) []tableauChip {
	var chips []tableauChip
	for _, k := range chipOrder {
		count := 0
		for _, c := range tableau {
			if c.Kind == k {
				count++
			}
		}
		if count == 0 {
			continue
		}
		chips = append(chips, tableauChip{kind: k, count: count})
	}
	return chips
}

func drawTableauSummary(l *Layout, gs *game.State, f *Fonts) {
	area := l.TableauArea
	ink.FillArea(area, ink.White)
	f.Small.SetActive(ink.Black)
	labelR := image.Rect(area.Min.X+sideMargin, area.Min.Y, area.Max.X-sideMargin, area.Min.Y+34)
	drawLeftString(labelR, "Din uppläggning", 26)

	chips := tableauChips(gs.Players[0].Tableau)
	rowY := labelR.Max.Y + 6
	rowH := area.Max.Y - rowY - 6
	chipW := 118
	x := area.Min.X + sideMargin
	for _, ch := range chips {
		r := image.Rect(x, rowY, x+chipW-10, rowY+rowH)
		iconBox := image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+rowH*68/100)
		drawCardIcon(pad(iconBox, 6), game.Card{Kind: ch.kind})
		countR := image.Rect(r.Min.X, iconBox.Max.Y, r.Max.X, r.Max.Y)
		f.Small.SetActive(ink.Black)
		drawCenteredString(countR, "x"+itoa(ch.count), 26)
		x += chipW
	}
	if len(chips) == 0 {
		f.Small.SetActive(ink.DarkGray)
		drawLeftString(image.Rect(x, rowY, area.Max.X, rowY+rowH), "(inga kort ännu)", 26)
	}
	ink.DrawLine(image.Pt(area.Min.X, area.Max.Y), image.Pt(area.Max.X, area.Max.Y), ink.Black)
}

// maxHandCols is the sizing reference for hand-card width: card size stays
// the same from turn to turn regardless of how many are actually in hand
// (only the row/column COUNT for a given n varies), so the hand doesn't
// visibly resize itself as the round wears down.
const maxHandCols = 5

// layoutHandGrid arranges n cards into a grid of up to maxHandCols columns,
// wrapping to further rows as needed. Both the grid as a whole (vertically)
// and each individual row (horizontally, so a partial last row isn't left
// stranded against the left edge) are centered within area.
func layoutHandGrid(area image.Rectangle, n int) []image.Rectangle {
	if n == 0 {
		return nil
	}
	gap := 16
	cellW := (area.Dx() - (maxHandCols-1)*gap) / maxHandCols
	cellH := 190
	if cellH > area.Dy() {
		cellH = area.Dy()
	}
	cols := n
	if cols > maxHandCols {
		cols = maxHandCols
	}
	rows := (n + cols - 1) / cols
	gridH := rows*cellH + (rows-1)*gap
	originY := area.Min.Y + (area.Dy()-gridH)/2
	if originY < area.Min.Y {
		originY = area.Min.Y
	}

	out := make([]image.Rectangle, n)
	for row := 0; row < rows; row++ {
		rowCount := cols
		if row == rows-1 {
			if rem := n - row*cols; rem < rowCount {
				rowCount = rem
			}
		}
		rowW := rowCount*cellW + (rowCount-1)*gap
		rowX := area.Min.X + (area.Dx()-rowW)/2
		y0 := originY + row*(cellH+gap)
		for col := 0; col < rowCount; col++ {
			x0 := rowX + col*(cellW+gap)
			out[row*cols+col] = image.Rect(x0, y0, x0+cellW, y0+cellH)
		}
	}
	return out
}

func drawHand(l *Layout, a *app) []image.Rectangle {
	area := l.HandArea
	ink.FillArea(area, ink.White)
	hand := a.gs.Players[0].Hand
	rects := layoutHandGrid(pad(area, 4), len(hand))
	for i, r := range hand {
		selected := false
		for _, s := range a.selected {
			if s == i {
				selected = true
			}
		}
		cr := rects[i]
		ink.FillArea(cr, ink.White)
		if selected {
			ink.DrawRect(cr, ink.Black)
			ink.DrawRect(pad(cr, 2), ink.Black)
			ink.DrawRect(pad(cr, 3), ink.Black)
		} else {
			ink.DrawRect(cr, ink.Black)
		}
		iconBox := image.Rect(cr.Min.X, cr.Min.Y+4, cr.Max.X, cr.Min.Y+cr.Dy()*68/100)
		drawCardIcon(pad(iconBox, 8), r)
		a.fonts.Small.SetActive(ink.Black)
		labelR := image.Rect(cr.Min.X, iconBox.Max.Y, cr.Max.X, cr.Max.Y-4)
		drawCenteredString(labelR, cardShortLabel(r), 26)
	}
	return rects
}

func drawChopBar(l *Layout, a *app) {
	area := l.ChopBar
	ink.FillArea(area, ink.White)
	if !a.chopAvailable() {
		a.chopBtn = image.Rectangle{}
		return
	}
	r := pad(area, 12)
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 1), ink.Black)
	label := "Använd ätpinnar"
	if a.chopMode {
		label = "Ätpinnar: välj 2 kort (" + itoa(len(a.selected)) + "/2)"
	}
	a.fonts.Button.SetActive(ink.Black)
	drawCenteredString(r, label, 34)
	a.chopBtn = r
}

func drawOpponents(l *Layout, gs *game.State, f *Fonts) {
	area := l.OpponentsArea
	ink.FillArea(area, ink.White)
	ink.DrawLine(image.Pt(area.Min.X, area.Min.Y), image.Pt(area.Max.X, area.Min.Y), ink.Black)
	f.Tiny.SetActive(ink.Black)
	labelR := image.Rect(area.Min.X+sideMargin, area.Min.Y+4, area.Max.X-sideMargin, area.Min.Y+30)
	drawLeftString(labelR, "Motståndarnas uppläggningar", 22)

	y := labelR.Max.Y + 2
	lineH := 32
	for i := 1; i < gs.NumPlayers; i++ {
		p := gs.Players[i]
		line := "AI " + itoa(i+1) + ":  " + summarizeTableau(p.Tableau) + "   (Poäng " + itoa(p.Score) + ", Pudding " + itoa(p.Pudding) + ")"
		r := image.Rect(area.Min.X+sideMargin, y, area.Max.X-sideMargin, y+lineH)
		f.Tiny.SetActive(ink.Black)
		drawLeftString(r, line, 22)
		y += lineH
	}
}

// summarizeTableau renders a compact one-line, kind-grouped summary of a
// tableau in plain text — deliberately just counts, never the cards
// themselves in any more revealing form, and NEVER anything from a hand:
// opponents' hands are hidden information and this function is never given
// one.
func summarizeTableau(tableau []game.Card) string {
	chips := tableauChips(tableau)
	if len(chips) == 0 {
		return "(inga kort ännu)"
	}
	s := ""
	for i, ch := range chips {
		if i > 0 {
			s += "  "
		}
		s += kindAbbrev(ch.kind) + " " + itoa(ch.count)
	}
	return s
}

func kindAbbrev(k game.CardKind) string {
	switch k {
	case game.KindNigiri:
		return "Nig"
	case game.KindWasabi:
		return "Was"
	case game.KindTempura:
		return "Tem"
	case game.KindSashimi:
		return "Sas"
	case game.KindDumpling:
		return "Dum"
	case game.KindMaki:
		return "Mak"
	case game.KindChopsticks:
		return "Ätp"
	case game.KindPudding:
		return "Pud"
	}
	return "?"
}

// --- Round-end / game-end screens ---------------------------------------

func playerName(i int) string {
	if i == 0 {
		return "Du"
	}
	return "AI " + itoa(i+1)
}

// DrawRoundEnd renders the breakdown for the round that just finished and
// returns the "Nästa runda" button rect plus a (single-entry) Meny button.
func DrawRoundEnd(l *Layout, a *app) (image.Rectangle, []Button) {
	gs := a.gs
	f := a.fonts
	screen := l.Screen
	H := usableH

	f.Title.SetActive(ink.Black)
	title := "Runda " + itoa(gs.Round) + " avslutad"
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.Dx()-tw)/2, 50), title)

	tableBottom := drawScoreTable(image.Rect(sideMargin, 160, screen.Dx()-sideMargin, H-260), gs, f)

	f.Tiny.SetActive(ink.DarkGray)
	legend := "N=Nigiri  T=Tempura  S=Sashimi  D=Dumpling  M=Maki  (Pudding räknas först i slutet)"
	lw := ink.StringWidth(legend)
	ink.DrawString(image.Pt((screen.Dx()-lw)/2, tableBottom+30), legend)

	bar := image.Rect(0, H-topMargin-buttonBarHeight, screen.Dx(), H-topMargin)
	labels := []string{"Meny", "Nästa runda"}
	if gs.Round >= game.NumRounds {
		labels = []string{"Meny", "Se slutresultat"}
	}
	btns := DrawButtonBar(&bar, labels, f)
	var next image.Rectangle
	var rest []Button
	for _, b := range btns {
		if b.Label == "Nästa runda" || b.Label == "Se slutresultat" {
			next = b.Rect
		} else {
			rest = append(rest, b)
		}
	}
	return next, rest
}

// DrawGameEnd renders the final banner: total ranking, the Pudding
// breakdown, and Ny omgång / Meny buttons.
func DrawGameEnd(l *Layout, a *app) []Button {
	gs := a.gs
	f := a.fonts
	screen := l.Screen
	H := usableH

	f.Title.SetActive(ink.Black)
	winners := gs.Winner()
	var title string
	if len(winners) > 1 {
		title = "Oavgjort!"
	} else if winners[0] == 0 {
		title = "Du vinner!"
	} else {
		title = playerName(winners[0]) + " vinner!"
	}
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.Dx()-tw)/2, 50), title)

	tableBottom := drawScoreTable(image.Rect(sideMargin, 160, screen.Dx()-sideMargin, H-320), gs, f)

	f.Small.SetActive(ink.Black)
	y := tableBottom + 50
	puddingLine := "Pudding: "
	for i := range gs.Players {
		if i > 0 {
			puddingLine += "   "
		}
		adj := 0
		if gs.LastPudding != nil {
			adj = gs.LastPudding[i]
		}
		puddingLine += playerName(i) + " " + itoa(gs.Players[i].Pudding) + " (" + signed(adj) + ")"
	}
	pw := ink.StringWidth(puddingLine)
	if pw > screen.Dx()-2*sideMargin {
		f.Tiny.SetActive(ink.Black)
	}
	ink.DrawString(image.Pt((screen.Dx()-ink.StringWidth(puddingLine))/2, y), puddingLine)

	bar := image.Rect(0, H-topMargin-buttonBarHeight, screen.Dx(), H-topMargin)
	return DrawButtonBar(&bar, []string{"Ny omgång", "Meny"}, f)
}

// drawScoreTable renders one line per player with a monospace font for
// clean column alignment: category totals for the round just scored
// (LastRoundScores), plus the running grand Total. Returns the Y coordinate
// just below the last row drawn, so callers can place what comes next
// (a legend, the Pudding line) right underneath instead of guessing a fixed
// offset from the bottom of the screen.
func drawScoreTable(area image.Rectangle, gs *game.State, f *Fonts) int {
	ink.FillArea(area, ink.White)
	f.Mono.SetActive(ink.Black)
	lineH := 48
	y := area.Min.Y
	header := padName("") + " N   T   S   D   M  =Rond  Tot"
	ink.DrawString(image.Pt(area.Min.X, y), header)
	y += lineH + 10
	for i, p := range gs.Players {
		if i >= len(gs.LastRoundScores) {
			break
		}
		rs := gs.LastRoundScores[i]
		line := padName(playerName(i)) +
			padNum(rs.Nigiri) + padNum(rs.Tempura) + padNum(rs.Sashimi) +
			padNum(rs.Dumpling) + padNum(rs.Maki) + "  " + padNum(rs.Total()) +
			"  " + padNum(p.Score)
		ink.DrawString(image.Pt(area.Min.X, y), line)
		y += lineH
		if y > area.Max.Y {
			break
		}
	}
	return y
}

func padName(s string) string {
	for len(s) < 8 {
		s += " "
	}
	return s
}

func padNum(n int) string {
	s := itoa(n)
	for len(s) < 4 {
		s = " " + s
	}
	return s
}

// --- Menu ------------------------------------------------------------------

type menuRow struct {
	rect image.Rectangle
	n    int
}

// Menu is the start screen: pick a player count (2-5; always 1 human + the
// rest AI, since hidden simultaneous hands rule out hot-seat) or open Regler.
type Menu struct {
	rows     []menuRow
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{} }

var playerCountLabels = []string{"2 spelare", "3 spelare", "4 spelare", "5 spelare"}

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	H := usableH

	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Sushi")
	ink.DrawString(image.Pt((screen.X-tw)/2, 70), "Sushi")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 32, true)
	sub.SetActive(ink.Black)
	subT := "Du + datorspelare — välj antal spelare"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 170), subT)
	sub.Close()

	margin := 60
	rowW := screen.X - 2*margin
	rbW := rowW / 2
	rbH := 100
	rb := image.Rect((screen.X-rbW)/2, H-margin-rbH, (screen.X+rbW)/2, H-margin)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	f.Menu.SetActive(ink.Black)
	rowH := 130
	top := 300
	bottom := rb.Min.Y - 30
	n := len(playerCountLabels)
	avail := bottom - top
	if avail < rowH*n {
		rowH = avail / n
	}
	y := top
	m.rows = m.rows[:0]
	for i, label := range playerCountLabels {
		r := image.Rect(margin, y, margin+rowW, y+rowH-20)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawLeftString(r, label, 40)
		m.rows = append(m.rows, menuRow{rect: r, n: i + 2})
		y += rowH
	}
}

func (m *Menu) HandleTouch(p image.Point) (int, bool) {
	for _, row := range m.rows {
		if p.In(row.rect) {
			return row.n, true
		}
	}
	return 0, false
}

func (m *Menu) RulesButton() image.Rectangle { return m.rulesBtn }

// --- Rules screen ------------------------------------------------------

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

var rulesParagraphs = []string{
	"Mål: flest poäng efter 3 ronder plus Pudding.",
	"1 människa mot 2-4 datorspelare. Alla får en hand kort. Varje drag väljer alla SAMTIDIGT ett kort att behålla ur sin hand, och skickar resten vidare till nästa spelare. Så fortsätter det tills alla händer är tomma.",
	"Nigiri: Ägg 1p, Lax 2p, Bläckfisk 3p.",
	"Wasabi: tredubblar poängen för nästa Nigiri du lägger EFTER wasabin (Ägg 3p, Lax 6p, Bläckfisk 9p). En Wasabi som läggs efter en Nigiri gäller inte för den.",
	"Tempura: varje PAR ger 5p. Ett udda ensamt Tempura-kort ger 0p.",
	"Sashimi: varje FULLSTÄNDIGT set om 3 ger 10p. Ofullständiga set (1-2 kort) ger 0p.",
	"Dumplingar: 1/2/3/4/5+ kort ger 1/3/6/10/15p.",
	"Maki-rullar: flest maki-ikon denna rond ger 6p, näst flest ger 3p (delas lika, avrundat nedåt, vid delad plats — och vid delad förstaplats uteblir tvåan helt). Med bara 2 spelare ges ingen tvåa-poäng alls.",
	"Ätpinnar: när du redan har ett outnyttjat Ätpinnar-kort i din uppläggning kan du på ett senare drag ta TVÅ kort ur handen istället för ett — Ätpinnar-kortet läggs då tillbaka i din hand och kan användas igen senare.",
	"Pudding räknas bara EN gång, i slutet av spelet (inte varje rond): flest Pudding ger +6, färst ger -6 (delas lika vid delad plats). Med bara 2 spelare uteblir -6 helt — bara ledaren får sitt plus.",
	"Du ser bara motståndarnas UPPLAGDA kort, aldrig deras händer — precis som runt ett riktigt bord.",
	"Baserat på Sushi Go! (Gamewright).",
}

func DrawRules(screen image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	ink.ClearScreen()
	H := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 56, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, 44), title)
	tf.Close()

	margin := 40
	bh := 110
	bw := screen.X / 2
	r := image.Rect((screen.X-bw)/2, H-margin-bh, (screen.X+bw)/2, H-margin)
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 1), ink.Black)
	f.Button.SetActive(ink.Black)
	drawCenteredString(r, "Tillbaka", 36)

	body := ink.OpenFont(ink.DefaultFont, 27, true)
	body.SetActive(ink.Black)
	bodyMargin := 44
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

// --- Splash screen -------------------------------------------------------

type motifFunc func(box image.Rectangle)

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

// drawSplashMotif shows three distinct sushi icons in a row — Nigiri, Maki,
// Dumpling — reusing the exact same drawCardIcon glyphs seen in play.
func drawSplashMotif(box image.Rectangle) {
	cx, cy := center(box)
	// Keep each icon roughly card-shaped (a touch taller than wide, like the
	// in-hand cards) instead of stretching it across the whole (much taller
	// than wide) splash box.
	cell := box.Dx() / 4
	if box.Dy() < cell {
		cell = box.Dy()
	}
	cellW := cell
	cellH := cell * 12 / 10
	gap := cellW + cellW/3
	cells := []image.Rectangle{
		image.Rect(cx-gap-cellW/2, cy-cellH/2, cx-gap+cellW/2, cy+cellH/2),
		image.Rect(cx-cellW/2, cy-cellH/2, cx+cellW/2, cy+cellH/2),
		image.Rect(cx+gap-cellW/2, cy-cellH/2, cx+gap+cellW/2, cy+cellH/2),
	}
	drawCardIcon(cells[0], game.Card{Kind: game.KindNigiri, N: 2})
	drawCardIcon(cells[1], game.Card{Kind: game.KindMaki, N: 3})
	drawCardIcon(cells[2], game.Card{Kind: game.KindDumpling})
}
