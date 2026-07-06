package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"expeditionen/game"
)

// Fonts held open for the app lifetime (opened once in Init — never inside
// Draw; see POCKETBOOK_GAMEDEV_GUIDE.md §4).
type Fonts struct {
	Status   *ink.Font
	Title    *ink.Font
	Button   *ink.Font
	Menu     *ink.Font
	Small    *ink.Font
	Tiny     *ink.Font
	CardRank *ink.Font
	CardSuit *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Status:   ink.OpenFont(ink.DefaultFontBold, 32, true),
		Title:    ink.OpenFont(ink.DefaultFontBold, 40, true),
		Button:   ink.OpenFont(ink.DefaultFontBold, 36, true),
		Menu:     ink.OpenFont(ink.DefaultFont, 38, true),
		Small:    ink.OpenFont(ink.DefaultFont, 26, true),
		Tiny:     ink.OpenFont(ink.DefaultFont, 22, true),
		CardRank: ink.OpenFont(ink.DefaultFontBold, 30, true),
		CardSuit: ink.OpenFont(ink.DefaultFont, 20, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Title, f.Button, f.Menu, f.Small, f.Tiny, f.CardRank, f.CardSuit} {
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
	x := r.Min.X + 12
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

// cardLabel is the short text drawn on a card chip: "I" for an investment
// card (plain ASCII — the guide warns glyphs like stars/arrows can render as
// a broken box on-device), or its number rank otherwise.
func cardLabel(c game.Card) string {
	if c.IsInvestment() {
		return "I"
	}
	return itoa(int(c.Rank))
}

// --- Layout --------------------------------------------------------------

const (
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	topMargin  = 40
	sideMargin = 24
	gap        = 14

	statusBarHeight = 70
	aiSummaryHeight = 32
	discardHeight   = 118
	handHeight      = 232
	actionBarHeight = 100
	navBarHeight    = 120
)

// Layout holds every screen region for the game screen, laid out bottom-up
// (per the guide's §5 rule) so every element keeps its margin to the bottom
// edge regardless of screen quirks.
type Layout struct {
	Screen     image.Rectangle
	StatusBar  image.Rectangle
	AISummary  image.Rectangle
	BoardArea  image.Rectangle
	DiscardBar image.Rectangle
	HandArea   image.Rectangle
	ActionBar  image.Rectangle
	NavBar     image.Rectangle
}

func NewLayout(screen image.Point) Layout {
	H := usableH
	l := Layout{Screen: image.Rect(0, 0, screen.X, H)}

	l.StatusBar = image.Rect(0, topMargin, screen.X, topMargin+statusBarHeight)
	l.AISummary = image.Rect(0, l.StatusBar.Max.Y, screen.X, l.StatusBar.Max.Y+aiSummaryHeight)

	l.NavBar = image.Rect(0, H-topMargin-navBarHeight, screen.X, H-topMargin)
	l.ActionBar = image.Rect(0, l.NavBar.Min.Y-gap-actionBarHeight, screen.X, l.NavBar.Min.Y-gap)
	l.HandArea = image.Rect(0, l.ActionBar.Min.Y-gap-handHeight, screen.X, l.ActionBar.Min.Y-gap)
	l.DiscardBar = image.Rect(0, l.HandArea.Min.Y-gap-discardHeight, screen.X, l.HandArea.Min.Y-gap)

	l.BoardArea = image.Rect(0, l.AISummary.Max.Y+gap, screen.X, l.DiscardBar.Min.Y-gap)
	return l
}

// boardRowRect returns the rect for suit i's (0-based, in game.AllSuits
// order) expedition row within l.BoardArea.
func (l *Layout) boardRowRect(i int) image.Rectangle {
	rowH := l.BoardArea.Dy() / game.NumSuits
	y0 := l.BoardArea.Min.Y + i*rowH
	return image.Rect(sideMargin, y0, l.Screen.Dx()-sideMargin, y0+rowH)
}

// --- Buttons ---------------------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

func DrawButtonBar(bar image.Rectangle, labels []string, f *Fonts) []Button {
	ink.FillArea(bar, ink.White)
	ink.DrawLine(image.Pt(bar.Min.X, bar.Min.Y), image.Pt(bar.Max.X, bar.Min.Y), ink.Black)
	f.Button.SetActive(ink.Black)
	n := len(labels)
	if n == 0 {
		return nil
	}
	btnGap := 20
	usableW := bar.Dx() - 2*sideMargin
	bw := (usableW - btnGap*(n+1)) / n
	bh := bar.Dy() - 2*btnGap
	buttons := make([]Button, n)
	for i, label := range labels {
		x0 := bar.Min.X + sideMargin + btnGap + i*(bw+btnGap)
		y0 := bar.Min.Y + btnGap
		r := image.Rect(x0, y0, x0+bw, y0+bh)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawCenteredString(r, label, 36)
		buttons[i] = Button{Rect: r, Label: label}
	}
	return buttons
}

// --- Card chip rendering -----------------------------------------------

// drawCardChip draws one card as a small bordered box: the suit's 1-letter
// abbreviation near the top, and its rank ("I" for investment) large and
// centered below. selected draws a doubled border.
func drawCardChip(r image.Rectangle, c game.Card, f *Fonts, selected bool) {
	ink.FillArea(r, ink.White)
	ink.DrawRect(r, ink.Black)
	if selected {
		ink.DrawRect(pad(r, 2), ink.Black)
		ink.DrawRect(pad(r, 3), ink.Black)
	}
	f.CardSuit.SetActive(ink.Black)
	suitR := image.Rect(r.Min.X, r.Min.Y+2, r.Max.X, r.Min.Y+r.Dy()*32/100)
	drawCenteredString(suitR, c.Suit.Abbrev(), 20)
	f.CardRank.SetActive(ink.Black)
	rankR := image.Rect(r.Min.X, suitR.Max.Y, r.Max.X, r.Max.Y-2)
	drawCenteredString(rankR, cardLabel(c), 30)
}

// drawEmptySlot draws a dashed-looking empty placeholder (a thin single
// border, no fill) for an empty discard pile or an unplayed expedition.
func drawEmptySlot(r image.Rectangle, label string, f *Fonts) {
	ink.FillArea(r, ink.White)
	ink.DrawRect(r, ink.DarkGray)
	f.Tiny.SetActive(ink.DarkGray)
	drawCenteredString(r, label, 22)
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

// --- Game screen ------------------------------------------------------

// DrawGameScreen renders the whole game screen and returns every tappable
// region: the 5 board-row rects (used only for the selection highlight, not
// tapping — plays go through the Spela/Kasta buttons), the 6 draw-source
// rects (5 discard piles + the deck, in that order), and the human's
// tappable hand-card rects.
type gameRects struct {
	boardRows  [game.NumSuits]image.Rectangle
	drawSlots  [game.NumSuits + 1]image.Rectangle // 0..4 = discard piles (AllSuits order), 5 = deck
	handCards  []image.Rectangle
	playBtn    image.Rectangle
	discardBtn image.Rectangle
	haveAction bool // whether playBtn/discardBtn are currently live (a hand card is selected)
}

func DrawGameScreen(l *Layout, a *app) gameRects {
	var gr gameRects
	gs := a.gs
	f := a.fonts

	drawStatusBar(l, gs, f)
	drawAISummary(l, gs, f)
	gr.boardRows = drawBoard(l, gs, f, a.selected)

	if gs.Phase == game.PhaseDone {
		drawFinalBreakdown(l, gs, f)
	} else {
		gr.drawSlots = drawDiscardBar(l, gs, f, a.canDraw())
		gr.handCards = drawHand(l, a)
		gr.playBtn, gr.discardBtn, gr.haveAction = drawActionBar(l, a)
	}
	return gr
}

func drawStatusBar(l *Layout, gs *game.State, f *Fonts) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	text := statusText(gs)
	drawCenteredString(l.StatusBar, text, 32)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y), image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

func statusText(gs *game.State) string {
	score := "Du " + itoa(gs.Players[game.HumanIdx].Score()) + " – Dator " + itoa(gs.Players[game.AIIdx].Score())
	if gs.Phase == game.PhaseDone {
		switch w := gs.Winner(); {
		case len(w) > 1:
			return "Oavgjort!  " + score
		case w[0] == game.HumanIdx:
			return "Du vann!  " + score
		default:
			return "Datorn vann!  " + score
		}
	}
	if gs.AITurn() {
		if gs.AwaitingDraw() {
			return "Datorns tur (drar)   ·   " + score
		}
		return "Datorns tur   ·   " + score
	}
	if gs.AwaitingDraw() {
		return "Din tur – välj varifrån du drar   ·   " + score
	}
	return "Din tur – välj ett kort   ·   " + score
}

// drawAISummary shows a compact, always-public one-line summary of the AI's
// expeditions (card counts per suit + running score) — the AI's ROWS are
// public information exactly like real cards on the table; only its HAND is
// hidden, and this line never shows that.
func drawAISummary(l *Layout, gs *game.State, f *Fonts) {
	ink.FillArea(l.AISummary, ink.White)
	f.Tiny.SetActive(ink.Black)
	line := "Dator: "
	for i, s := range game.AllSuits {
		if i > 0 {
			line += "  "
		}
		row := gs.Players[game.AIIdx].Rows[s]
		line += s.Abbrev() + itoa(len(row))
	}
	line += "   (poäng " + itoa(gs.Players[game.AIIdx].Score()) + ")"
	drawLeftString(image.Rect(l.AISummary.Min.X+sideMargin, l.AISummary.Min.Y, l.AISummary.Max.X-sideMargin, l.AISummary.Max.Y), line, 22)
}

// drawBoard renders the human's 5 expedition rows: suit label, the ascending
// run of played cards, and the row's live score. selectedSuit (if >= 0)
// gets a highlighted border, hinting where the currently-selected hand card
// would go.
func drawBoard(l *Layout, gs *game.State, f *Fonts, selected int) [game.NumSuits]image.Rectangle {
	var rows [game.NumSuits]image.Rectangle
	var selectedSuit game.Suit = -1
	if selected >= 0 && selected < len(gs.Players[game.HumanIdx].Hand) {
		selectedSuit = gs.Players[game.HumanIdx].Hand[selected].Suit
	}
	for i, s := range game.AllSuits {
		r := l.boardRowRect(i)
		rows[i] = r
		ink.FillArea(r, ink.White)
		border := pad(r, 2)
		if s == selectedSuit {
			ink.DrawRect(border, ink.Black)
			ink.DrawRect(pad(border, 1), ink.Black)
		} else {
			ink.DrawRect(border, ink.DarkGray)
		}

		labelR := image.Rect(border.Min.X+6, border.Min.Y, border.Min.X+90, border.Max.Y)
		f.Small.SetActive(ink.Black)
		drawLeftString(labelR, s.String(), 26)

		scoreW := 110
		scoreR := image.Rect(border.Max.X-scoreW, border.Min.Y, border.Max.X-6, border.Max.Y)
		row := gs.Players[game.HumanIdx].Rows[s]
		f.Small.SetActive(ink.Black)
		drawCenteredString(scoreR, itoa(game.Score(row)), 26)

		chipArea := image.Rect(labelR.Max.X+6, border.Min.Y+4, scoreR.Min.X-6, border.Max.Y-4)
		drawExpeditionChips(chipArea, row, f)
	}
	return rows
}

// drawExpeditionChips renders row's cards left-to-right, small enough that
// even a full 12-card expedition fits without scrolling.
func drawExpeditionChips(area image.Rectangle, row []game.Card, f *Fonts) {
	if len(row) == 0 {
		drawEmptySlot(pad(area, 2), "(orörd)", f)
		return
	}
	const maxCards = 12
	chipGap := 4
	chipW := (area.Dx() - chipGap*(maxCards-1)) / maxCards
	if chipW > 56 {
		chipW = 56
	}
	if chipW < 22 {
		chipW = 22
	}
	chipH := area.Dy()
	if chipH > 78 {
		chipH = 78
	}
	y0 := area.Min.Y + (area.Dy()-chipH)/2
	x := area.Min.X
	for _, c := range row {
		r := image.Rect(x, y0, x+chipW, y0+chipH)
		drawCardChip(r, c, f, false)
		x += chipW + chipGap
	}
}

// drawDiscardBar renders the 5 discard piles plus the deck as 6 equal tiles.
// active controls whether they're currently valid tap targets (visually,
// via a bold border) — true only when the human owes a draw.
func drawDiscardBar(l *Layout, gs *game.State, f *Fonts, active bool) [game.NumSuits + 1]image.Rectangle {
	var slots [game.NumSuits + 1]image.Rectangle
	area := l.DiscardBar
	ink.FillArea(area, ink.White)
	ink.DrawLine(image.Pt(area.Min.X, area.Min.Y), image.Pt(area.Max.X, area.Min.Y), ink.Black)

	n := game.NumSuits + 1
	tileGap := 12
	usableW := area.Dx() - 2*sideMargin
	tw := (usableW - tileGap*(n-1)) / n
	th := area.Dy() - 12
	y0 := area.Min.Y + 6

	drawTile := func(i int, label string, top *game.Card, count int) image.Rectangle {
		x0 := area.Min.X + sideMargin + i*(tw+tileGap)
		r := image.Rect(x0, y0, x0+tw, y0+th)
		ink.FillArea(r, ink.White)
		var bcol color.Color = ink.DarkGray
		if active {
			bcol = ink.Black
		}
		ink.DrawRect(r, bcol)
		if active {
			ink.DrawRect(pad(r, 1), bcol)
		}
		labelR := image.Rect(r.Min.X, r.Min.Y+2, r.Max.X, r.Min.Y+22)
		f.Tiny.SetActive(ink.Black)
		drawCenteredString(labelR, label, 18)
		bodyR := image.Rect(r.Min.X, labelR.Max.Y, r.Max.X, r.Max.Y-2)
		if top != nil {
			drawCardChip(pad(bodyR, 4), *top, f, false)
		} else {
			f.Small.SetActive(ink.DarkGray)
			drawCenteredString(bodyR, "–", 26)
		}
		if count > 0 {
			f.Tiny.SetActive(ink.Black)
			countR := image.Rect(r.Min.X, r.Max.Y-20, r.Max.X, r.Max.Y)
			drawCenteredString(countR, "("+itoa(count)+")", 18)
		}
		return r
	}

	for i, s := range game.AllSuits {
		pile := gs.Discards[s]
		var top *game.Card
		if len(pile) > 0 {
			top = &pile[len(pile)-1]
		}
		slots[i] = drawTile(i, s.Abbrev(), top, len(pile))
	}
	slots[game.NumSuits] = drawTile(game.NumSuits, "Kortlek", nil, len(gs.Deck))
	return slots
}

const maxHandCols = 4

// layoutHandGrid arranges n cards into up to maxHandCols columns, wrapping
// to further rows as needed, centered within area (mirrors sushi's
// layoutHandGrid).
func layoutHandGrid(area image.Rectangle, n int) []image.Rectangle {
	if n == 0 {
		return nil
	}
	cellGap := 14
	cellW := (area.Dx() - (maxHandCols-1)*cellGap) / maxHandCols
	cols := n
	if cols > maxHandCols {
		cols = maxHandCols
	}
	rows := (n + cols - 1) / cols
	cellH := area.Dy() / rows
	if cellH > 110 {
		cellH = 110
	}
	gridH := rows*cellH + (rows-1)*cellGap
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
		rowW := rowCount*cellW + (rowCount-1)*cellGap
		rowX := area.Min.X + (area.Dx()-rowW)/2
		y0 := originY + row*(cellH+cellGap)
		for col := 0; col < rowCount; col++ {
			x0 := rowX + col*(cellW+cellGap)
			out[row*cols+col] = image.Rect(x0, y0, x0+cellW, y0+cellH)
		}
	}
	return out
}

func drawHand(l *Layout, a *app) []image.Rectangle {
	area := pad(l.HandArea, 4)
	ink.FillArea(l.HandArea, ink.White)
	hand := a.gs.Players[game.HumanIdx].Hand
	rects := layoutHandGrid(area, len(hand))
	for i, c := range hand {
		drawCardChip(rects[i], c, a.fonts, i == a.selected)
	}
	return rects
}

// drawActionBar shows the Spela/Kasta buttons when a hand card is selected
// and it's the human's turn to choose an action, otherwise a short hint
// (also surfacing the last illegal-play rejection, if any).
func drawActionBar(l *Layout, a *app) (playBtn, discardBtn image.Rectangle, have bool) {
	area := l.ActionBar
	ink.FillArea(area, ink.White)
	gs := a.gs

	if !gs.HumanTurn() {
		f := a.fonts.Small
		f.SetActive(ink.DarkGray)
		drawCenteredString(area, "Väntar på datorn …", 26)
		return image.Rectangle{}, image.Rectangle{}, false
	}
	if gs.AwaitingDraw() {
		f := a.fonts.Small
		f.SetActive(ink.Black)
		drawCenteredString(area, "Dra ett kort: kortleken eller toppen av en kasthög ovan", 26)
		return image.Rectangle{}, image.Rectangle{}, false
	}
	if a.selected < 0 {
		f := a.fonts.Small
		f.SetActive(ink.DarkGray)
		msg := "Tryck på ett kort i handen"
		if a.playRejected {
			msg = "Ogiltigt drag — välj ett annat kort eller läge"
		}
		drawCenteredString(area, msg, 26)
		return image.Rectangle{}, image.Rectangle{}, false
	}

	btnTop := area.Min.Y + 12
	if a.playRejected {
		// A card is still selected (the rejected play), but a warning strip
		// now sits above the buttons instead of them filling the whole bar.
		warnR := image.Rect(area.Min.X+sideMargin, area.Min.Y+2, area.Max.X-sideMargin, area.Min.Y+30)
		a.fonts.Tiny.SetActive(ink.Black)
		drawCenteredString(warnR, "Ogiltigt drag för Spela — testa Kasta istället", 22)
		btnTop = warnR.Max.Y + 6
	}

	half := (area.Dx() - 2*sideMargin - gap) / 2
	pr := image.Rect(area.Min.X+sideMargin, btnTop, area.Min.X+sideMargin+half, area.Max.Y-12)
	dr := image.Rect(pr.Max.X+gap, btnTop, area.Max.X-sideMargin, area.Max.Y-12)
	a.fonts.Button.SetActive(ink.Black)
	ink.DrawRect(pr, ink.Black)
	ink.DrawRect(pad(pr, 1), ink.Black)
	drawCenteredString(pr, "Spela", 36)
	ink.DrawRect(dr, ink.Black)
	ink.DrawRect(pad(dr, 1), ink.Black)
	drawCenteredString(dr, "Kasta", 36)
	return pr, dr, true
}

// drawFinalBreakdown replaces the discard/hand/action areas at PhaseDone
// with a per-suit score table for both players.
func drawFinalBreakdown(l *Layout, gs *game.State, f *Fonts) {
	area := image.Rect(0, l.DiscardBar.Min.Y, l.Screen.Dx(), l.ActionBar.Max.Y)
	ink.FillArea(area, ink.White)
	ink.DrawLine(image.Pt(area.Min.X, area.Min.Y), image.Pt(area.Max.X, area.Min.Y), ink.Black)

	f.Small.SetActive(ink.Black)
	headerR := image.Rect(sideMargin, area.Min.Y+8, area.Max.X-sideMargin, area.Min.Y+40)
	drawLeftString(headerR, "Expedition        Du     Dator", 26)

	rowH := 40
	y := headerR.Max.Y + 4
	for _, s := range game.AllSuits {
		hs := game.Score(gs.Players[game.HumanIdx].Rows[s])
		as := game.Score(gs.Players[game.AIIdx].Rows[s])
		line := padRight(s.String(), 14) + padLeft(itoa(hs), 8) + padLeft(itoa(as), 8)
		r := image.Rect(sideMargin, y, area.Max.X-sideMargin, y+rowH)
		f.Small.SetActive(ink.Black)
		drawLeftString(r, line, 26)
		y += rowH
	}
	totalLine := padRight("Totalt", 14) + padLeft(itoa(gs.Players[game.HumanIdx].Score()), 8) + padLeft(itoa(gs.Players[game.AIIdx].Score()), 8)
	r := image.Rect(sideMargin, y+6, area.Max.X-sideMargin, y+6+rowH)
	f.Status.SetActive(ink.Black)
	drawLeftString(r, totalLine, 30)
}

func padRight(s string, n int) string {
	for len(s) < n {
		s += " "
	}
	return s
}

func padLeft(s string, n int) string {
	for len(s) < n {
		s = " " + s
	}
	return s
}

// --- Menu ------------------------------------------------------------------

type Menu struct {
	startBtn image.Rectangle
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{} }

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	H := usableH

	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Expeditionen")
	ink.DrawString(image.Pt((screen.X-tw)/2, 80), "Expeditionen")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 30, true)
	sub.SetActive(ink.Black)
	subT := "Du mot datorn — dolda händer"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 190), subT)
	sub.Close()

	margin := 60
	rowW := screen.X - 2*margin

	startH := 130
	sr := image.Rect(margin, 340, margin+rowW, 340+startH)
	ink.DrawRect(sr, ink.Black)
	ink.DrawRect(pad(sr, 1), ink.Black)
	f.Menu.SetActive(ink.Black)
	drawCenteredString(sr, "Starta spel", 40)
	m.startBtn = sr

	rbW := rowW / 2
	rbH := 100
	rb := image.Rect((screen.X-rbW)/2, H-margin-rbH, (screen.X+rbW)/2, H-margin)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb
}

func (m *Menu) StartButton() image.Rectangle { return m.startBtn }
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
	"Mål: högst poäng när kortleken tar slut.",
	"5 expeditioner: Öknen, Djungeln, Havet, Vulkanen, Polaren. Varje expedition har 12 kort: 3 investeringskort (märkta I) och nio talkort, 2 till 10.",
	"På din tur gör du ETT av två saker: spela ett kort till din EGEN expedition för det kortets färg, eller lägg kortet med framsidan upp på den expeditionens kasthög. Dra sedan exakt ett kort — antingen från kortleken eller från toppen av VILKEN SOM HELST kasthög.",
	"Turordning inom en expedition: talkort måste spelas i STIGANDE ordning (samma värde som senast spelat är också tillåtet, men aldrig lägre).",
	"Investeringsregeln: alla investeringskort för en expedition måste spelas INNAN något talkort i den expeditionen. Så fort ett talkort har spelats i en expedition får inga fler investeringskort läggas dit.",
	"Poäng per expedition: en orörd expedition (inga kort spelade) ger 0 poäng. Annars: (summan av talkorten − 20) × (1 + antal spelade investeringskort). Har du spelat 8 kort eller fler i den expeditionen läggs 20 poäng till DÄREFTER — alltså efter multiplikationen, inte innan.",
	"Exempel: summa 25, 1 investeringskort, 9 kort spelade → (25−20)×(1+1) = 10, plus 20 i bonus = 30 poäng. En expedition du knappt startat kan alltså ge MINUSPOÄNG om summan hamnar under 20 — investeringskort förstärker även ett minus.",
	"Kasthögarna är synliga för båda spelarna, precis som en egen expeditions kort — bara handen är dold.",
	"Spelet slutar när kortleken tar slut. Den med högst sammanlagd poäng över alla 5 expeditioner vinner.",
	"Baserat på Lost Cities (Kosmos / Reiner Knizia).",
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
	paraGap := 12
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

// drawSplashMotif renders the spec's requested motif: one expedition row
// showing an ascending run of cards, with a small score readout.
func drawSplashMotif(box image.Rectangle) {
	f := &Fonts{
		CardRank: ink.OpenFont(ink.DefaultFontBold, 40, true),
		CardSuit: ink.OpenFont(ink.DefaultFont, 26, true),
		Small:    ink.OpenFont(ink.DefaultFont, 30, true),
	}
	defer f.Close()

	ranks := []game.Rank{3, 5, 7, 9}
	cx, cy := (box.Min.X+box.Max.X)/2, (box.Min.Y+box.Max.Y)/2
	cw, ch := box.Dx()/6, box.Dx()/6*13/10
	rowY := cy - ch/2
	totalW := len(ranks)*cw + (len(ranks)-1)*16
	x := cx - totalW/2
	for _, rk := range ranks {
		r := image.Rect(x, rowY, x+cw, rowY+ch)
		drawCardChip(r, game.Card{Suit: game.SuitOken, Rank: rk}, f, false)
		x += cw + 16
	}
	f.Small.SetActive(ink.Black)
	scoreR := image.Rect(box.Min.X, rowY+ch+20, box.Max.X, rowY+ch+60)
	drawCenteredString(scoreR, "Poäng: "+itoa(game.Score(ranksToRow(ranks))), 30)
}

func ranksToRow(ranks []game.Rank) []game.Card {
	row := make([]game.Card, len(ranks))
	for i, r := range ranks {
		row[i] = game.Card{Suit: game.SuitOken, Rank: r}
	}
	return row
}
