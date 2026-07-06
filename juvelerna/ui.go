package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"juvelerna/game"
)

// Fonts held open for the app lifetime (opened once — never per draw).
type Fonts struct {
	Status *ink.Font
	Title  *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Small  *ink.Font
	Tiny   *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 32, true),
		Title:  ink.OpenFont(ink.DefaultFontBold, 56, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 32, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 36, true),
		Small:  ink.OpenFont(ink.DefaultFont, 24, true),
		Tiny:   ink.OpenFont(ink.DefaultFont, 19, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Title, f.Button, f.Menu, f.Small, f.Tiny} {
		if fn != nil {
			fn.Close()
		}
	}
}

func pad(r image.Rectangle, n int) image.Rectangle {
	return image.Rect(r.Min.X+n, r.Min.Y+n, r.Max.X-n, r.Max.Y-n)
}

func centerOf(r image.Rectangle) (int, int) {
	return (r.Min.X + r.Max.X) / 2, (r.Min.Y + r.Max.Y) / 2
}

// squareIn returns the largest square centered within box — used to keep
// gem-pattern art bounded and undistorted regardless of the aspect ratio of
// the box a caller happens to pass in.
func squareIn(box image.Rectangle) image.Rectangle {
	side := box.Dx()
	if box.Dy() < side {
		side = box.Dy()
	}
	if side <= 0 {
		return box
	}
	cx, cy := centerOf(box)
	return image.Rect(cx-side/2, cy-side/2, cx+side/2, cy+side/2)
}

func drawCenteredString(r image.Rectangle, s string, approxHeight int) {
	w := ink.StringWidth(s)
	x := r.Min.X + (r.Dx()-w)/2
	y := r.Min.Y + (r.Dy()-approxHeight)/2
	ink.DrawString(image.Pt(x, y), s)
}

func drawLeftString(r image.Rectangle, s string, approxHeight int) {
	x := r.Min.X + 10
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

// --- Gem colors: names + greyscale patterns ---------------------------------
// The 5 game.Color values map 1:1 onto mosaik's pattern palette (solid fill,
// ring, cross-hatch, diagonal stripe, dot grid) so no two read as the same
// grey blob on e-ink, plus a Swedish gem name for labels.

func gemName(c game.Color) string {
	switch c {
	case game.ColorSolid:
		return "Diamant"
	case game.ColorRing:
		return "Safir"
	case game.ColorCross:
		return "Smaragd"
	case game.ColorStripe:
		return "Rubin"
	case game.ColorDot:
		return "Onyx"
	}
	return "?"
}

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

func drawDiagRun(box image.Rectangle, col color.Color, period, thickness int, reverse bool) {
	for y := box.Min.Y; y < box.Max.Y; y++ {
		runStart := -1
		for x := box.Min.X; x <= box.Max.X; x++ {
			on := false
			if x < box.Max.X {
				v := (x - box.Min.X) + (y - box.Min.Y)
				if reverse {
					v = (x - box.Min.X) - (y - box.Min.Y)
				}
				m := ((v % period) + period) % period
				on = m < thickness
			}
			if on && runStart == -1 {
				runStart = x
			} else if !on && runStart != -1 {
				ink.DrawLine(image.Pt(runStart, y), image.Pt(x-1, y), col)
				runStart = -1
			}
		}
	}
}

func drawDotsGrid(box image.Rectangle, col color.Color) {
	n := 3
	stepX := box.Dx() / (n + 1)
	stepY := box.Dy() / (n + 1)
	r := stepX / 3
	if r < 2 {
		r = 2
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= n; j++ {
			fillDiscAt(box.Min.X+i*stepX, box.Min.Y+j*stepY, r, col)
		}
	}
}

// drawGemPattern renders game.Color c's pattern inside box, in black (faint =
// false) or a light grey tone (faint = true — used for a "no cards of this
// color" placeholder, kept unused for now but mirroring mosaik's convention).
func drawGemPattern(box image.Rectangle, c game.Color, faint bool) {
	// squareIn: several call sites here (token/bank chips, bonus badges) pass
	// a WIDE, SHORT rectangle (not the roughly-square boxes mosaik always
	// used), and the Ring case in particular sizes its radius from box.Dx()
	// alone with no vertical clipping — on a wide box that draws a circle
	// far taller than the box, bleeding into neighboring rows. Always
	// confine the actual pattern to the centered square inscribed in box.
	box = squareIn(box)
	col := color.Color(ink.Black)
	if faint {
		col = color.Gray{Y: 0xb0}
	}
	switch c {
	case game.ColorSolid:
		ink.FillArea(pad(box, box.Dx()/6+1), col)
	case game.ColorRing:
		cx, cy := centerOf(box)
		rOuter := box.Dx()/2 - box.Dx()/8
		fillDiscAt(cx, cy, rOuter, col)
		fillDiscAt(cx, cy, rOuter-box.Dx()/5, ink.White)
	case game.ColorCross:
		drawDiagRun(box, col, 10, 3, false)
		drawDiagRun(box, col, 10, 3, true)
	case game.ColorStripe:
		drawDiagRun(box, col, 16, 7, false)
	case game.ColorDot:
		drawDotsGrid(box, col)
	}
}

// drawGoldGlyph draws the wildcard/gold token symbol: a bold 6-point
// asterisk (distinct from any of the 5 gem patterns and from the noble
// marker triangle).
// drawGoldGlyph draws the wildcard/gold token symbol: box filled solid
// black with a bold white "G" — deliberately the only solid-black swatch in
// the game, so gold never reads as a 6th gem color/pattern. Simpler and
// more legible at these small sizes than a hand-drawn line glyph (an
// earlier version drew a 3-line asterisk via hand-rolled trig, which
// rendered as an illegible blob).
func drawGoldGlyph(box image.Rectangle) {
	ink.FillArea(box, ink.Black)
	size := box.Dy() * 2 / 3
	if size < 10 {
		size = 10
	}
	gf := ink.OpenFont(ink.DefaultFontBold, size, true)
	gf.SetActive(ink.White)
	drawCenteredString(box, "G", size)
	gf.Close()
}

// --- Layout ------------------------------------------------------------

const (
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	topMargin  = 14
	sideMargin = 18
	gapSm      = 8

	statusBarHeight   = 50
	nobleRowHeight    = 100
	bankRowHeight     = 110
	tableauRowHeight  = 163
	tableauAreaHeight = tableauRowHeight * game.NumTiers
	activePanelHeight = 320
	opponentHeight    = 90
	buttonBarHeight   = 100
)

type bankHit struct {
	rect  image.Rectangle
	color game.Color
}

type nobleHit struct {
	rect image.Rectangle
	idx  int // index into gs.Nobles
}

type tableauHit struct {
	rect       image.Rectangle
	tier, slot int
}

type deckHit struct {
	rect image.Rectangle
	tier int
}

type reservedHit struct {
	rect image.Rectangle
	idx  int
}

type discardHit struct {
	rect   image.Rectangle
	color  game.Color
	isGold bool
}

// Layout holds every screen region for Juvelerna's board, rebuilt each draw
// from the current screen size (static bands) and populated hit-test slices
// (dynamic, from the current game state).
type Layout struct {
	Screen        image.Rectangle
	StatusBar     image.Rectangle
	NobleRow      image.Rectangle
	BankRow       image.Rectangle
	TableauArea   image.Rectangle
	ActivePanel   image.Rectangle
	OpponentStrip image.Rectangle
	ButtonBar     image.Rectangle

	ShowOppBtn image.Rectangle

	nobleHits    []nobleHit
	bankHits     []bankHit
	tableauHits  []tableauHit
	deckHits     []deckHit
	reservedHits []reservedHit
	discardHits  []discardHit
}

func NewLayout(screen image.Point) Layout {
	H := usableH
	l := Layout{Screen: image.Rect(0, 0, screen.X, H)}
	y := topMargin
	l.StatusBar = image.Rect(0, y, screen.X, y+statusBarHeight)
	y = l.StatusBar.Max.Y + gapSm
	l.NobleRow = image.Rect(0, y, screen.X, y+nobleRowHeight)
	y = l.NobleRow.Max.Y + gapSm
	l.BankRow = image.Rect(0, y, screen.X, y+bankRowHeight)
	y = l.BankRow.Max.Y + gapSm
	l.TableauArea = image.Rect(0, y, screen.X, y+tableauAreaHeight)
	y = l.TableauArea.Max.Y + gapSm
	l.ActivePanel = image.Rect(0, y, screen.X, y+activePanelHeight)
	y = l.ActivePanel.Max.Y + gapSm
	l.OpponentStrip = image.Rect(0, y, screen.X, y+opponentHeight)
	y = l.OpponentStrip.Max.Y + gapSm
	l.ButtonBar = image.Rect(0, y, screen.X, y+buttonBarHeight)
	return l
}

func (l *Layout) HitBank(p image.Point) (game.Color, bool) {
	for _, h := range l.bankHits {
		if p.In(h.rect) {
			return h.color, true
		}
	}
	return 0, false
}

func (l *Layout) HitNoble(p image.Point) (int, bool) {
	for _, h := range l.nobleHits {
		if p.In(h.rect) {
			return h.idx, true
		}
	}
	return 0, false
}

func (l *Layout) HitTableau(p image.Point) (tier, slot int, ok bool) {
	for _, h := range l.tableauHits {
		if p.In(h.rect) {
			return h.tier, h.slot, true
		}
	}
	return 0, 0, false
}

func (l *Layout) HitDeck(p image.Point) (int, bool) {
	for _, h := range l.deckHits {
		if p.In(h.rect) {
			return h.tier, true
		}
	}
	return 0, false
}

func (l *Layout) HitReserved(p image.Point) (int, bool) {
	for _, h := range l.reservedHits {
		if p.In(h.rect) {
			return h.idx, true
		}
	}
	return 0, false
}

func (l *Layout) HitDiscard(p image.Point) (game.Color, bool, bool) {
	for _, h := range l.discardHits {
		if p.In(h.rect) {
			return h.color, h.isGold, true
		}
	}
	return 0, false, false
}

// --- Buttons -------------------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

func DrawButtonBar(l *Layout, labels []string, f *Fonts) []Button {
	bar := l.ButtonBar
	ink.FillArea(bar, ink.White)
	ink.DrawLine(image.Pt(bar.Min.X, bar.Min.Y), image.Pt(bar.Max.X, bar.Min.Y), ink.Black)
	f.Button.SetActive(ink.Black)
	n := len(labels)
	if n == 0 {
		return nil
	}
	gap := 16
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
		drawCenteredString(r, label, 32)
		buttons[i] = Button{Rect: r, Label: label}
	}
	return buttons
}

// --- Status bar --------------------------------------------------------

func playerLabel(gs *game.GameState, side int) string {
	if gs.Mode == game.ModeAI {
		if side == 0 {
			return "Du"
		}
		return "Dator"
	}
	if side == 0 {
		return "Spelare 1"
	}
	return "Spelare 2"
}

func DrawStatus(l *Layout, gs *game.GameState, f *Fonts, msg string) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	text := msg
	if text == "" {
		turn := playerLabel(gs, gs.Turn)
		if gs.AITurn() {
			turn += " (dator)"
		}
		switch gs.Phase {
		case game.PhaseNobleChoice:
			text = turn + " väljer en adelsperson"
		case game.PhaseDiscard:
			text = turn + " måste kasta " + itoa(gs.DiscardNeeded) + " pollett(er)"
		default:
			text = turn + " drar   ·   " +
				playerLabel(gs, 0) + " " + itoa(gs.Players[0].Prestige) + "p – " +
				playerLabel(gs, 1) + " " + itoa(gs.Players[1].Prestige) + "p"
		}
	}
	drawCenteredString(l.StatusBar, text, 30)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y), image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// --- Noble row -----------------------------------------------------------

// DrawNobleRow renders every still-available noble as a small tile showing
// its required color+count badges. While gs.Phase == PhaseNobleChoice, only
// the pending nobles are tappable (drawn with a bold double border and a
// "Välj" hint); records those into l.nobleHits.
func DrawNobleRow(l *Layout, gs *game.GameState, f *Fonts) {
	area := pad(l.NobleRow, sideMargin/2)
	ink.FillArea(l.NobleRow, ink.White)
	l.nobleHits = l.nobleHits[:0]

	f.Small.SetActive(ink.Black)
	drawLeftString(image.Rect(area.Min.X, area.Min.Y, area.Min.X+140, area.Min.Y+24), "Adel (3p):", 20)

	n := len(gs.Nobles)
	if n == 0 {
		return
	}
	tilesArea := image.Rect(area.Min.X+150, area.Min.Y, area.Max.X, area.Max.Y)
	gap := 10
	tileW := (tilesArea.Dx() - (n-1)*gap) / n
	for i, noble := range gs.Nobles {
		x0 := tilesArea.Min.X + i*(tileW+gap)
		r := image.Rect(x0, tilesArea.Min.Y, x0+tileW, tilesArea.Max.Y)

		pendingIdx := -1
		if gs.Phase == game.PhaseNobleChoice {
			for k, ni := range gs.PendingNobles {
				if ni == i {
					pendingIdx = k
				}
			}
		}
		ink.DrawRect(r, ink.Black)
		if pendingIdx >= 0 {
			ink.DrawRect(pad(r, 2), ink.Black)
			ink.DrawRect(pad(r, 3), ink.Black)
			l.nobleHits = append(l.nobleHits, nobleHit{rect: r, idx: pendingIdx})
		}
		drawNobleBadges(pad(r, 6), noble)
	}
}

func drawNobleBadges(area image.Rectangle, n game.Noble) {
	var cols []game.Color
	for c := game.Color(0); c < game.NumColors; c++ {
		if n.Requirement[c] > 0 {
			cols = append(cols, c)
		}
	}
	if len(cols) == 0 {
		return
	}
	gap := 4
	bw := (area.Dx() - (len(cols)-1)*gap) / len(cols)
	f := ink.OpenFont(ink.DefaultFontBold, 18, true)
	defer f.Close()
	f.SetActive(ink.Black)
	for i, c := range cols {
		x0 := area.Min.X + i*(bw+gap)
		r := image.Rect(x0, area.Min.Y, x0+bw, area.Max.Y-20)
		drawGemPattern(pad(r, 3), c, false)
		ink.DrawRect(r, ink.Black)
		lbl := itoa(n.Requirement[c])
		lr := image.Rect(x0, area.Max.Y-20, x0+bw, area.Max.Y)
		drawCenteredString(lr, lbl, 18)
	}
}

// --- Bank row (5 gem piles + gold) -----------------------------------------

// DrawBankRow renders the 5 gem token piles (tappable) plus the gold pile
// (not directly tappable — it's never taken as a main action), highlighting
// any colors currently included in the active player's build-a-take
// selection.
func DrawBankRow(l *Layout, gs *game.GameState, sel selection) {
	area := pad(l.BankRow, sideMargin/2)
	ink.FillArea(l.BankRow, ink.White)
	l.bankHits = l.bankHits[:0]

	n := int(game.NumColors) + 1 // + gold
	gap := 10
	cw := (area.Dx() - (n-1)*gap) / n
	for i := game.Color(0); i < game.NumColors; i++ {
		x0 := area.Min.X + int(i)*(cw+gap)
		r := image.Rect(x0, area.Min.Y, x0+cw, area.Max.Y)
		selected := selCountColor(sel, i) > 0
		drawBankPile(r, i, gs.Bank[i], selected, selCountColor(sel, i))
		l.bankHits = append(l.bankHits, bankHit{rect: r, color: i})
	}
	// Gold pile (informational only).
	x0 := area.Min.X + int(game.NumColors)*(cw+gap)
	r := image.Rect(x0, area.Min.Y, x0+cw, area.Max.Y)
	ink.DrawRect(r, ink.Black)
	drawGoldGlyph(pad(r, 6))
	countR := image.Rect(r.Min.X, r.Max.Y-24, r.Max.X, r.Max.Y)
	ink.FillArea(countR, ink.White)
	drawCenteredString(countR, itoa(gs.BankGold), 20)
}

func selCountColor(sel selection, c game.Color) int {
	n := 0
	for _, sc := range sel.bankColors {
		if sc == c {
			n++
		}
	}
	return n
}

func drawBankPile(r image.Rectangle, c game.Color, count int, selected bool, selCount int) {
	swatch := image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y-26)
	drawGemPattern(pad(swatch, 6), c, false)
	ink.DrawRect(r, ink.Black)
	if selected {
		ink.DrawRect(pad(r, 2), ink.Black)
		ink.DrawRect(pad(r, 3), ink.Black)
	}
	countR := image.Rect(r.Min.X, r.Max.Y-26, r.Max.X, r.Max.Y)
	ink.FillArea(countR, ink.White)
	label := itoa(count)
	if selCount > 0 {
		label += " (" + itoa(selCount) + " vald)"
		if selCount > 1 {
			label = itoa(count) + " (" + itoa(selCount) + " valda)"
		}
	}
	drawCenteredString(countR, label, 20)
}

// --- Tableau: 3 tiers x (4 face-up cards + 1 deck-back) --------------------

// DrawTableau renders the 3 tiers of the tableau (highest tier on top),
// each row showing its 4 face-up cards plus a face-down "deck" icon (with
// the remaining card count) that lets the active player reserve blindly.
// Records every tappable card/deck icon into l.tableauHits/l.deckHits.
func DrawTableau(l *Layout, gs *game.GameState, sel selection) {
	ink.FillArea(l.TableauArea, ink.White)
	l.tableauHits = l.tableauHits[:0]
	l.deckHits = l.deckHits[:0]

	for row := 0; row < game.NumTiers; row++ {
		tier := game.NumTiers - 1 - row // tier 3 (index 2) drawn first/top
		rowArea := image.Rect(l.TableauArea.Min.X, l.TableauArea.Min.Y+row*tableauRowHeight,
			l.TableauArea.Max.X, l.TableauArea.Min.Y+(row+1)*tableauRowHeight)
		drawTableauRow(l, pad(rowArea, 4), tier, gs, sel)
	}
}

func drawTableauRow(l *Layout, area image.Rectangle, tier int, gs *game.GameState, sel selection) {
	labelW := 40
	labelR := image.Rect(area.Min.X, area.Min.Y, area.Min.X+labelW, area.Max.Y)
	tf := ink.OpenFont(ink.DefaultFontBold, 26, true)
	tf.SetActive(ink.Black)
	drawCenteredString(labelR, "T"+itoa(tier+1), 26)
	tf.Close()

	slotsArea := image.Rect(labelR.Max.X+6, area.Min.Y, area.Max.X, area.Max.Y)
	n := game.TableauSlots + 1 // + deck
	gap := 8
	cw := (slotsArea.Dx() - (n-1)*gap) / n
	for s := 0; s < game.TableauSlots; s++ {
		x0 := slotsArea.Min.X + s*(cw+gap)
		r := image.Rect(x0, slotsArea.Min.Y, x0+cw, slotsArea.Max.Y)
		card := gs.Tableau[tier][s]
		selected := sel.kind == selCard && !sel.fromReserved && sel.tier == tier && sel.slot == s
		drawCardCell(r, card, selected)
		if card.Tier != 0 {
			l.tableauHits = append(l.tableauHits, tableauHit{rect: r, tier: tier, slot: s})
		}
	}
	// Deck-back icon (5th slot).
	x0 := slotsArea.Min.X + game.TableauSlots*(cw+gap)
	r := image.Rect(x0, slotsArea.Min.Y, x0+cw, slotsArea.Max.Y)
	left := len(gs.Decks[tier])
	drawDeckBack(r, left, sel.kind == selDeck && sel.tier == tier)
	if left > 0 {
		l.deckHits = append(l.deckHits, deckHit{rect: r, tier: tier})
	}
}

func drawDeckBack(r image.Rectangle, left int, selected bool) {
	ink.FillArea(r, ink.White)
	drawDiagRun(pad(r, 4), ink.DarkGray, 14, 6, false)
	ink.DrawRect(r, ink.Black)
	if selected {
		ink.DrawRect(pad(r, 2), ink.Black)
		ink.DrawRect(pad(r, 3), ink.Black)
	}
	countR := image.Rect(r.Min.X, r.Max.Y-26, r.Max.X, r.Max.Y)
	ink.FillArea(countR, ink.White)
	ink.DrawRect(countR, ink.Black)
	drawCenteredString(countR, itoa(left), 20)
}

// drawCardCell renders one development card (or an empty outline if the
// slot has been drawn empty because its deck ran dry): a bonus-color swatch
// in the top-left corner, its prestige-point number in the top-right corner
// (the spec's splash motif detail, echoed here on every real card), and a
// small grid of nonzero cost badges below.
func drawCardCell(r image.Rectangle, card game.Card, selected bool) {
	ink.FillArea(r, ink.White)
	ink.DrawRect(r, ink.Black)
	if selected {
		ink.DrawRect(pad(r, 2), ink.Black)
		ink.DrawRect(pad(r, 3), ink.Black)
	}
	if card.Tier == 0 {
		return // empty slot (deck exhausted)
	}
	corner := 30
	swatch := image.Rect(r.Min.X+4, r.Min.Y+4, r.Min.X+4+corner, r.Min.Y+4+corner)
	drawGemPattern(swatch, card.Color, false)
	ink.DrawRect(swatch, ink.Black)

	if card.Points > 0 {
		f := ink.OpenFont(ink.DefaultFontBold, 26, true)
		f.SetActive(ink.Black)
		pr := image.Rect(r.Max.X-34, r.Min.Y+2, r.Max.X-4, r.Min.Y+32)
		drawCenteredString(pr, itoa(card.Points), 26)
		f.Close()
	}

	costArea := image.Rect(r.Min.X+4, r.Min.Y+corner+10, r.Max.X-4, r.Max.Y-4)
	drawCostBadges(costArea, card.Cost)
}

// drawCostBadges lays out one small swatch+count badge per nonzero cost
// color, in a 2-column grid.
func drawCostBadges(area image.Rectangle, cost [game.NumColors]int) {
	var cols []game.Color
	for c := game.Color(0); c < game.NumColors; c++ {
		if cost[c] > 0 {
			cols = append(cols, c)
		}
	}
	if len(cols) == 0 {
		f := ink.OpenFont(ink.DefaultFont, 20, true)
		f.SetActive(ink.DarkGray)
		drawCenteredString(area, "Gratis", 20)
		f.Close()
		return
	}
	const gridCols = 2
	rows := (len(cols) + gridCols - 1) / gridCols
	gap := 4
	bw := (area.Dx() - (gridCols-1)*gap) / gridCols
	bh := area.Dy() / rows
	if bh > 34 {
		bh = 34
	}
	f := ink.OpenFont(ink.DefaultFontBold, 18, true)
	defer f.Close()
	f.SetActive(ink.Black)
	for i, c := range cols {
		col, row := i%gridCols, i/gridCols
		x0 := area.Min.X + col*(bw+gap)
		y0 := area.Min.Y + row*(bh+gap)
		r := image.Rect(x0, y0, x0+bw, y0+bh)
		swatch := image.Rect(r.Min.X, r.Min.Y, r.Min.X+bh, r.Max.Y)
		drawGemPattern(pad(swatch, 3), c, false)
		ink.DrawRect(swatch, ink.Black)
		numR := image.Rect(swatch.Max.X+2, r.Min.Y, r.Max.X, r.Max.Y)
		drawCenteredString(numR, itoa(cost[c]), 18)
	}
}

// --- Active player panel (full-size) ---------------------------------------

// DrawActivePanel renders side's full board: tokens+gold (tappable for
// discard when discardable is true), owned card-bonuses, reserved cards
// (tappable to select for purchase when selectable is true), and prestige.
func DrawActivePanel(l *Layout, gs *game.GameState, side int, f *Fonts, sel selection, discardable, selectable bool) {
	area := l.ActivePanel
	ink.FillArea(area, ink.White)
	ink.DrawLine(image.Pt(area.Min.X, area.Min.Y), image.Pt(area.Max.X, area.Min.Y), ink.Black)
	p := &gs.Players[side]
	if !discardable {
		l.discardHits = l.discardHits[:0]
	}
	l.reservedHits = l.reservedHits[:0]

	// Every row below the header reserves its OWN small caption inside a
	// fixed-height "label gap" that precedes it — captions are never drawn
	// overlapping the row above (an earlier version placed a caption in the
	// gap BETWEEN rows without reserving enough of it, which visibly
	// overlapped the previous row's content).
	const (
		headerH   = 40
		rowGap    = 6
		tokRowH   = 64
		labelGapH = 22 // caption height + its own padding, reserved before a row
		bonusRowH = 44
	)

	headerR := image.Rect(sideMargin, area.Min.Y+4, area.Max.X-sideMargin, area.Min.Y+4+headerH)
	f.Status.SetActive(ink.Black)
	label := boardHeaderLabel(gs, side) + "   ·   Prestige " + itoa(p.Prestige)
	drawLeftString(headerR, label, 32)

	// Tokens + gold row. The count digits below each swatch use a small
	// dedicated font, set once here — every draw call below (token, gold,
	// bonus chips) inherits it since nothing else activates a font in
	// between (an earlier version left the header's large font active,
	// rendering the counts oversized and overflowing into the row below).
	f.Tiny.SetActive(ink.Black)
	tokY := headerR.Max.Y + rowGap
	tokArea := image.Rect(sideMargin, tokY, area.Max.X-sideMargin, tokY+tokRowH)
	n := int(game.NumColors) + 1
	gap := 8
	cw := (tokArea.Dx() - (n-1)*gap) / n
	for c := game.Color(0); c < game.NumColors; c++ {
		x0 := tokArea.Min.X + int(c)*(cw+gap)
		r := image.Rect(x0, tokArea.Min.Y, x0+cw, tokArea.Max.Y)
		drawTokenChip(r, c, p.Tokens[c])
		if discardable && p.Tokens[c] > 0 {
			l.discardHits = append(l.discardHits, discardHit{rect: r, color: c})
		}
	}
	x0 := tokArea.Min.X + int(game.NumColors)*(cw+gap)
	r := image.Rect(x0, tokArea.Min.Y, x0+cw, tokArea.Max.Y)
	drawGoldChip(r, p.Gold)
	if discardable && p.Gold > 0 {
		l.discardHits = append(l.discardHits, discardHit{rect: r, isGold: true})
	}

	// Bonuses row: its caption fills the reserved label gap right after the
	// token row, THEN the row itself starts.
	bonusLabelY := tokArea.Max.Y + rowGap
	f.Tiny.SetActive(ink.Black)
	drawLeftString(image.Rect(sideMargin, bonusLabelY, sideMargin+400, bonusLabelY+labelGapH), "Rabatter (från köpta kort):", 16)
	bonusY := bonusLabelY + labelGapH
	bonusArea := image.Rect(sideMargin, bonusY, area.Max.X-sideMargin, bonusY+bonusRowH)
	bw := (bonusArea.Dx() - (int(game.NumColors)-1)*gap) / int(game.NumColors)
	for c := game.Color(0); c < game.NumColors; c++ {
		x0 := bonusArea.Min.X + int(c)*(bw+gap)
		r := image.Rect(x0, bonusArea.Min.Y, x0+bw, bonusArea.Max.Y)
		drawBonusChip(r, c, p.Bonuses[c])
	}

	// Reserved cards row: same "caption in its own reserved gap" pattern,
	// then the row fills the rest of the panel down to its bottom margin.
	resLabelY := bonusArea.Max.Y + rowGap
	f.Tiny.SetActive(ink.Black)
	drawLeftString(image.Rect(sideMargin, resLabelY, sideMargin+400, resLabelY+labelGapH), "Reserverade kort:", 16)
	resY := resLabelY + labelGapH
	resArea := image.Rect(sideMargin, resY, area.Max.X-sideMargin, area.Max.Y-6)
	if len(p.Reserved) == 0 {
		return
	}
	rgap := 10
	rw := (resArea.Dx() - (game.MaxReserved-1)*rgap) / game.MaxReserved
	for i, card := range p.Reserved {
		x0 := resArea.Min.X + i*(rw+rgap)
		r := image.Rect(x0, resArea.Min.Y, x0+rw, resArea.Max.Y)
		selected := selectable && sel.kind == selCard && sel.fromReserved && sel.reservedIdx == i
		drawCardCell(r, card, selected)
		if selectable {
			l.reservedHits = append(l.reservedHits, reservedHit{rect: r, idx: i})
		}
	}
}

func boardHeaderLabel(gs *game.GameState, side int) string {
	if gs.Mode == game.ModeAI {
		if side == 0 {
			return "Ditt bräde"
		}
		return "Datorns bräde"
	}
	return playerLabel(gs, side) + " – bräde"
}

func drawTokenChip(r image.Rectangle, c game.Color, count int) {
	swatch := image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y-22)
	drawGemPattern(pad(swatch, 4), c, false)
	ink.DrawRect(r, ink.Black)
	countR := image.Rect(r.Min.X, r.Max.Y-22, r.Max.X, r.Max.Y)
	ink.FillArea(countR, ink.White)
	drawCenteredString(countR, itoa(count), 18)
}

func drawGoldChip(r image.Rectangle, count int) {
	swatch := image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y-22)
	ink.DrawRect(swatch, ink.Black)
	drawGoldGlyph(pad(swatch, 4))
	countR := image.Rect(r.Min.X, r.Max.Y-22, r.Max.X, r.Max.Y)
	ink.FillArea(countR, ink.White)
	drawCenteredString(countR, itoa(count), 18)
}

func drawBonusChip(r image.Rectangle, c game.Color, count int) {
	drawGemPattern(pad(r, 4), c, count == 0)
	ink.DrawRect(r, ink.Black)
	if count > 0 {
		badge := image.Rect(r.Max.X-20, r.Max.Y-20, r.Max.X, r.Max.Y)
		ink.FillArea(badge, ink.White)
		ink.DrawRect(badge, ink.Black)
		drawCenteredString(badge, itoa(count), 16)
	}
}

// --- Opponent compact strip --------------------------------------------------

// DrawOpponentStrip renders `side`'s board as a one-line compact summary
// (prestige, owned-card count, reserved count, token total) plus the
// "Visa motståndare"/"Mitt bräde" toggle. Records the toggle button rect
// into l.ShowOppBtn.
func DrawOpponentStrip(l *Layout, gs *game.GameState, side int, showOpp bool, f *Fonts) {
	area := l.OpponentStrip
	ink.FillArea(area, ink.White)
	ink.DrawLine(image.Pt(area.Min.X, area.Min.Y), image.Pt(area.Max.X, area.Min.Y), ink.Black)
	p := &gs.Players[side]

	btnW, btnH := 260, 60
	btn := image.Rect(area.Max.X-btnW-sideMargin, area.Min.Y+(area.Dy()-btnH)/2,
		area.Max.X-sideMargin, area.Min.Y+(area.Dy()-btnH)/2+btnH)
	ink.DrawRect(btn, ink.Black)
	ink.DrawRect(pad(btn, 1), ink.Black)
	label := "Visa motståndare"
	if showOpp {
		label = "Mitt bräde"
	}
	toggleFont := ink.OpenFont(ink.DefaultFontBold, 24, true)
	toggleFont.SetActive(ink.Black)
	drawCenteredString(btn, label, 24)
	toggleFont.Close()
	l.ShowOppBtn = btn

	textArea := image.Rect(sideMargin, area.Min.Y, btn.Min.X-16, area.Max.Y)
	f.Status.SetActive(ink.Black)
	nameLine := playerLabel(gs, side) + "   ·   Prestige " + itoa(p.Prestige)
	drawLeftString(image.Rect(textArea.Min.X, area.Min.Y+6, textArea.Max.X, area.Min.Y+42), nameLine, 30)
	f.Tiny.SetActive(ink.Black)
	infoLine := "Kort: " + itoa(len(p.Cards)) + "   ·   Reserverade: " + itoa(len(p.Reserved)) +
		"   ·   Polletter: " + itoa(p.TokensTotal()) + "   ·   Adel: " + itoa(len(p.Nobles))
	drawLeftString(image.Rect(textArea.Min.X, area.Min.Y+46, textArea.Max.X, area.Min.Y+70), infoLine, 20)
}

// --- Winner banner -----------------------------------------------------------

func winnerText(gs *game.GameState) string {
	switch gs.Winner() {
	case 0:
		return playerLabel(gs, 0) + " vinner!"
	case 1:
		return playerLabel(gs, 1) + " vinner!"
	default:
		return "Oavgjort!"
	}
}

func DrawWinner(l *Layout, gs *game.GameState, f *Fonts) {
	area := image.Rect(0, l.NobleRow.Min.Y, l.Screen.Max.X, l.BankRow.Max.Y)
	ink.FillArea(area, ink.White)

	f.Title.SetActive(ink.Black)
	text := winnerText(gs)
	tw := ink.StringWidth(text)
	ink.DrawString(image.Pt((l.Screen.Dx()-tw)/2, area.Min.Y+20), text)

	f.Menu.SetActive(ink.Black)
	scoreLine := playerLabel(gs, 0) + ": " + itoa(gs.Players[0].Prestige) + "p (" + itoa(len(gs.Players[0].Cards)) + " kort)     " +
		playerLabel(gs, 1) + ": " + itoa(gs.Players[1].Prestige) + "p (" + itoa(len(gs.Players[1].Cards)) + " kort)"
	sw := ink.StringWidth(scoreLine)
	ink.DrawString(image.Pt((l.Screen.Dx()-sw)/2, area.Min.Y+100), scoreLine)
}

// --- Menu ------------------------------------------------------------------

type menuChoice struct {
	mode    game.Mode
	aiLevel int
	label   string
}

type menuRow struct {
	rect   image.Rectangle
	choice menuChoice
}

var choices = []menuChoice{
	{game.ModeHotseat, 0, "2 spelare (hot-seat)"},
	{game.ModeAI, game.DepthEasy, "Mot dator – Lätt"},
	{game.ModeAI, game.DepthMedium, "Mot dator – Medel"},
	{game.ModeAI, game.DepthHard, "Mot dator – Svår"},
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
	tw := ink.StringWidth("Juvelerna")
	ink.DrawString(image.Pt((screen.X-tw)/2, 66), "Juvelerna")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 32, true)
	sub.SetActive(ink.Black)
	subT := "Välj spelläge"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 160), subT)
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
	top := 280
	bottom := rb.Min.Y - 30
	n := len(choices)
	avail := bottom - top
	if avail < rowH*n {
		rowH = avail / n
	}
	y := top
	m.rows = m.rows[:0]
	for _, c := range choices {
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

// rulesParagraphs is the full Swedish rules text, covering every gotcha
// called out in the spec: the take-2-needs->=4-before rule, noble auto-claim
// vs. active-player choice when several qualify, and the "finish the round"
// end-game timing.
var rulesParagraphs = []string{
	"Mål: bygg upp juvelrabatter genom att köpa utvecklingskort och samla prestige. Först till minst 15 prestige (kontrollerat i slutet av en spelares tur) utlöser spelets slut.",
	"Banken har 5 sorters juvelpolletter (4 av varje i ett 2-spelarparti) plus 5 guld-polletter (jokrar). 3 rader med utvecklingskort ligger framme: tier 1 (billigast), tier 2 och tier 3 (dyrast), 4 kort synliga per rad plus en dold hög du kan reservera blint från.",
	"På din tur gör du EXAKT EN av fyra saker: (1) ta 1 pollett vardera av 3 OLIKA färger, (2) ta 2 polletter av SAMMA färg — men bara om den färgens hög hade MINST 4 polletter INNAN du tog (inte bara 2), (3) reservera ett kort — synligt eller blint från en hög — (max 3 reserverade samtidigt), du får dessutom 1 guldpollett om banken har någon kvar, eller (4) köp ett kort, från bordet eller från din egen reserv.",
	"Betalning: varje köpt kort ger en permanent rabatt på 1 pollett av sin egen färg för alla framtida köp. Du betalar med polletter och/eller dina rabatter; guld fungerar som vilken färg som helst.",
	"Handgräns: har du fler än 10 polletter totalt (juveler + guld) i slutet av din tur måste du kasta tillbaka till banken tills du har högst 10.",
	"Adelspersoner: 3 ligger framme, värda 3 prestige var. En adelsperson besöker automatiskt en spelare vars köpta korts färgrabatter uppfyller kraven, i slutet av turen. Om FLERA adelspersoner uppfylls samtidigt väljer DEN AKTIVA SPELAREN själv vilken — det sker inte automatiskt på första träff.",
	"Spelets slut: så fort en spelare når minst 15 prestige i slutet av sin tur är det avgjort — men rundan spelas färdigt (den andra spelaren får sin tur också) innan spelet stannar. Högst prestige vinner; vid lika poäng vinner den med FÄRRE köpta utvecklingskort (effektivast).",
	"Tryck på polletter i banken för att bygga ett uttag (samma färg två gånger = ta 2, tre olika färger = ta 3), tryck sedan Ta. Tryck på ett kort i bordet eller din egen reserv för att välja det, tryck sedan Reservera eller Köp. Tryck Rensa för att avbryta ett val.",
	"Baserat på Splendor (Marc André, Space Cowboys). Kortens exakta kostnader/poäng i den här versionen är egna, nya värden — inte kopierade från originalets kortlek.",
}

func DrawRules(screen image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	ink.ClearScreen()
	H := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 52, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, 40), title)
	tf.Close()

	margin := 40
	bh := 100
	bw := screen.X / 2
	r := image.Rect((screen.X-bw)/2, H-margin-bh, (screen.X+bw)/2, H-margin)
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 1), ink.Black)
	f.Button.SetActive(ink.Black)
	drawCenteredString(r, "Tillbaka", 32)

	body := ink.OpenFont(ink.DefaultFont, 23, true)
	body.SetActive(ink.Black)
	bodyMargin := 36
	maxW := screen.X - 2*bodyMargin
	y := 120
	lineH := 30
	paraGap := 10
	limit := r.Min.Y - 14
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

func DrawSplash(screen image.Point, f *Fonts, title string, motif motifFunc) {
	ink.ClearScreen()
	H := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 80, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, H/6), title)
	tf.Close()

	side := screen.X * 3 / 5
	box := image.Rect((screen.X-side)/2, (H-side)/2, (screen.X+side)/2, (H+side)/2)
	motif(box)

	hint := ink.OpenFont(ink.DefaultFont, 34, true)
	hint.SetActive(ink.DarkGray)
	ht := "Tryck för att börja"
	hw := ink.StringWidth(ht)
	ink.DrawString(image.Pt((screen.X-hw)/2, H*5/6), ht)
	hint.Close()
}

// drawSplashMotif shows the spec's chosen splash image: a small stack of 2-3
// gem tokens next to a development-card outline with its prestige-point
// corner — the two icons this whole game revolves around.
func drawSplashMotif(box image.Rectangle) {
	cy := box.Min.Y + box.Dy()/2
	tokR := box.Dx() / 10

	// 3 stacked/offset gem tokens on the left half.
	centers := []image.Point{
		{box.Min.X + box.Dx()/5, cy + tokR},
		{box.Min.X + box.Dx()/5 + tokR, cy - tokR/2},
		{box.Min.X + box.Dx()/5 + tokR*2, cy + tokR},
	}
	patterns := []func(image.Rectangle){
		func(r image.Rectangle) { drawGemPattern(r, game.ColorSolid, false) },
		func(r image.Rectangle) { drawGemPattern(r, game.ColorRing, false) },
		func(r image.Rectangle) { drawGemPattern(r, game.ColorStripe, false) },
	}
	for i, c := range centers {
		r := image.Rect(c.X-tokR, c.Y-tokR, c.X+tokR, c.Y+tokR)
		patterns[i](r)
		ink.DrawRect(r, ink.Black)
	}

	// A development-card outline on the right half, with a filled corner
	// (its prestige-point badge) and its own bonus-color swatch.
	cardW := box.Dx() * 2 / 5
	cardH := box.Dy() * 3 / 5
	cardX := box.Max.X - cardW - box.Dx()/12
	cardY := cy - cardH/2
	card := image.Rect(cardX, cardY, cardX+cardW, cardY+cardH)
	ink.DrawRect(card, ink.Black)
	ink.DrawRect(pad(card, 1), ink.Black)

	corner := cardW / 3
	pointCorner := image.Rect(card.Max.X-corner, card.Min.Y, card.Max.X, card.Min.Y+corner)
	ink.FillArea(pointCorner, ink.Black)
	f := ink.OpenFont(ink.DefaultFontBold, corner*2/3, true)
	f.SetActive(ink.White)
	drawCenteredString(pointCorner, "3", corner*2/3)
	f.Close()

	swatch := image.Rect(card.Min.X+8, card.Min.Y+8, card.Min.X+8+corner*2/3, card.Min.Y+8+corner*2/3)
	drawGemPattern(swatch, game.ColorCross, false)
	ink.DrawRect(swatch, ink.Black)
}
