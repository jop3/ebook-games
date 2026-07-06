package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"geishorna/game"
)

// Fonts held open for the app lifetime (opened once in Init — never inside
// Draw; see POCKETBOOK_GAMEDEV_GUIDE §4).
type Fonts struct {
	Status   *ink.Font
	Title    *ink.Font
	Button   *ink.Font
	Menu     *ink.Font
	Small    *ink.Font
	Tiny     *ink.Font
	CardTag  *ink.Font
	CardSub  *ink.Font
	BigCount *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Status:   ink.OpenFont(ink.DefaultFontBold, 28, true),
		Title:    ink.OpenFont(ink.DefaultFontBold, 40, true),
		Button:   ink.OpenFont(ink.DefaultFontBold, 32, true),
		Menu:     ink.OpenFont(ink.DefaultFont, 38, true),
		Small:    ink.OpenFont(ink.DefaultFont, 26, true),
		Tiny:     ink.OpenFont(ink.DefaultFont, 22, true),
		CardTag:  ink.OpenFont(ink.DefaultFontBold, 40, true),
		CardSub:  ink.OpenFont(ink.DefaultFont, 20, true),
		BigCount: ink.OpenFont(ink.DefaultFontBold, 44, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Title, f.Button, f.Menu, f.Small, f.Tiny, f.CardTag, f.CardSub, f.BigCount} {
		if fn != nil {
			fn.Close()
		}
	}
}

// compSplits are the three ways to split four selected Competition cards
// (indices 0..3 into app.sel) into two unordered pairs.
var compSplits = [3][2][2]int{
	{{0, 1}, {2, 3}},
	{{0, 2}, {1, 3}},
	{{0, 3}, {1, 2}},
}

// --- small helpers ---------------------------------------------------------

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

// --- Layout ----------------------------------------------------------------

const (
	usableH = 1340 // ink.ScreenSize().Y over-reports; keep content above ~1360

	topMargin  = 30
	sideMargin = 20
	gap        = 12

	statusH = 64
	navH    = 110
	promptH = 150
	handH   = 176
)

// Layout holds every screen region for the game screen, laid out bottom-up so
// each element keeps its margin to the bottom edge (guide §5).
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	BoardArea image.Rectangle
	HandArea  image.Rectangle
	PromptBar image.Rectangle
	NavBar    image.Rectangle
}

func NewLayout(screen image.Point) Layout {
	H := usableH
	l := Layout{Screen: image.Rect(0, 0, screen.X, H)}
	l.StatusBar = image.Rect(0, topMargin, screen.X, topMargin+statusH)
	l.NavBar = image.Rect(0, H-topMargin-navH, screen.X, H-topMargin)
	l.PromptBar = image.Rect(0, l.NavBar.Min.Y-gap-promptH, screen.X, l.NavBar.Min.Y-gap)
	l.HandArea = image.Rect(0, l.PromptBar.Min.Y-gap-handH, screen.X, l.PromptBar.Min.Y-gap)
	l.BoardArea = image.Rect(0, l.StatusBar.Max.Y+gap, screen.X, l.HandArea.Min.Y-gap)
	return l
}

func (l *Layout) geishaRowRect(i int) image.Rectangle {
	n := game.NumGeishas
	rowH := l.BoardArea.Dy() / n
	y0 := l.BoardArea.Min.Y + i*rowH
	return image.Rect(sideMargin, y0+2, l.Screen.Dx()-sideMargin, y0+rowH-2)
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
		drawCenteredString(r, label, 32)
		buttons[i] = Button{Rect: r, Label: label}
	}
	return buttons
}

// drawTextButton draws a single bordered button and returns its rect. If dim,
// it is drawn greyed (inactive).
func drawTextButton(r image.Rectangle, label string, f *ink.Font, dim bool) image.Rectangle {
	col := color.Color(ink.Black)
	if dim {
		col = ink.LightGray
	}
	ink.FillArea(r, ink.White)
	ink.DrawRect(r, col)
	ink.DrawRect(pad(r, 1), col)
	f.SetActive(col)
	drawCenteredString(r, label, 30)
	return r
}

// --- Card chips ------------------------------------------------------------

// drawCardChip draws one item card: the geisha's tag letter large and centered
// with its charm value small below. selected doubles the border; dim greys it.
func drawCardChip(r image.Rectangle, c game.Card, f *Fonts, selected, dim bool) {
	border := color.Color(ink.Black)
	if dim {
		border = ink.LightGray
	}
	ink.FillArea(r, ink.White)
	ink.DrawRect(r, border)
	if selected {
		ink.DrawRect(pad(r, 2), ink.Black)
		ink.DrawRect(pad(r, 3), ink.Black)
	}
	f.CardTag.SetActive(border)
	tagR := image.Rect(r.Min.X, r.Min.Y+r.Dy()*10/100, r.Max.X, r.Min.Y+r.Dy()*62/100)
	drawCenteredString(tagR, c.Tag(), 40)
	f.CardSub.SetActive(border)
	subR := image.Rect(r.Min.X, tagR.Max.Y, r.Max.X, r.Max.Y-4)
	drawCenteredString(subR, "charm "+itoa(c.Charm()), 20)
}

// --- Game screen entry -----------------------------------------------------

type actBtn struct {
	rect image.Rectangle
	live bool
}

type gameRects struct {
	geishaRows  [game.NumGeishas]image.Rectangle
	handCards   []image.Rectangle
	actionBtns  [game.NumActions]actBtn
	confirmBtn  image.Rectangle
	confirmLive bool
	cancelBtn   image.Rectangle
	pairBtns    [3]image.Rectangle
	offerCards  []image.Rectangle
	offerGroups []image.Rectangle
	continueBtn image.Rectangle
}

func DrawGameScreen(l *Layout, a *app) gameRects {
	var gr gameRects
	gs := a.gs
	f := a.fonts

	drawStatusBar(l, a)
	gr.geishaRows = drawBoard(l, gs, f)

	switch gs.Phase {
	case game.PhaseRoundEnd:
		gr.continueBtn = drawRoundEnd(l, gs, f)
	case game.PhaseMatchEnd:
		drawMatchEnd(l, gs, f)
	default:
		if gs.HumanTurn() && a.mode == modeChoose && gs.Pending != nil {
			gr.offerCards, gr.offerGroups = drawOffer(l, gs, f)
			drawChoosePrompt(l, gs, f)
		} else {
			gr.handCards = drawHand(l, a)
			a.fillLowerControls(l, &gr)
		}
	}
	return gr
}

// fillLowerControls renders the prompt bar for the current human/AI mode and
// records the tappable regions.
func (a *app) fillLowerControls(l *Layout, gr *gameRects) {
	gs := a.gs
	f := a.fonts
	area := l.PromptBar
	ink.FillArea(area, ink.White)
	ink.DrawLine(image.Pt(area.Min.X, area.Min.Y), image.Pt(area.Max.X, area.Min.Y), ink.Black)

	if !gs.HumanTurn() {
		f.Small.SetActive(ink.DarkGray)
		drawCenteredString(area, "Datorns tur …", 26)
		return
	}

	switch a.mode {
	case modeAction:
		gr.actionBtns = drawActionButtons(area, gs, f)
	case modeSelect:
		a.drawSelectPrompt(area, gr)
	case modeCompPair:
		a.drawCompPairPrompt(area, gr)
	}
}

func drawStatusBar(l *Layout, a *app) {
	gs := a.gs
	f := a.fonts
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	drawCenteredString(l.StatusBar, statusText(gs), 28)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y), image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

func statusText(gs *game.State) string {
	hg, hc := gs.Standing(game.HumanIdx)
	ag, ac := gs.Standing(game.AIIdx)
	stand := "Du " + itoa(hc) + "p·" + itoa(hg) + "g   Dator " + itoa(ac) + "p·" + itoa(ag) + "g"
	var lead string
	switch gs.Phase {
	case game.PhaseMatchEnd:
		if gs.MatchWinner == game.HumanIdx {
			lead = "Du vann matchen!"
		} else {
			lead = "Datorn vann matchen!"
		}
	case game.PhaseRoundEnd:
		lead = "Runda " + itoa(gs.Round) + " klar"
	default:
		switch {
		case gs.AITurn():
			lead = "Datorns tur"
		case gs.Pending != nil:
			lead = "Välj ur erbjudandet"
		default:
			lead = "Runda " + itoa(gs.Round) + " · din tur"
		}
	}
	return lead + "    " + stand
}

// drawBoard renders the seven geisha rows with each side's item count and the
// persistent favor marker.
func drawBoard(l *Layout, gs *game.State, f *Fonts) [game.NumGeishas]image.Rectangle {
	var rows [game.NumGeishas]image.Rectangle
	for i := 0; i < game.NumGeishas; i++ {
		r := l.geishaRowRect(i)
		rows[i] = r
		drawGeishaRow(r, i, gs, f)
	}
	return rows
}

func drawGeishaRow(r image.Rectangle, i int, gs *game.State, f *Fonts) {
	aiN := gs.Players[game.AIIdx].Field[i]
	huN := gs.Players[game.HumanIdx].Field[i]
	holder := gs.Favor[i]

	cellW := 110
	aiCell := image.Rect(r.Min.X, r.Min.Y, r.Min.X+cellW, r.Max.Y)
	huCell := image.Rect(r.Max.X-cellW, r.Min.Y, r.Max.X, r.Max.Y)
	center := image.Rect(aiCell.Max.X, r.Min.Y, huCell.Min.X, r.Max.Y)

	ink.FillArea(r, ink.White)
	ink.DrawRect(r, ink.LightGray)

	drawCountCell(aiCell, aiN, holder == game.AIIdx, aiN > huN, f)
	drawCountCell(huCell, huN, holder == game.HumanIdx, huN > aiN, f)

	// Center: tag + name, charm, and a neutral hint for the marker.
	f.Small.SetActive(ink.Black)
	nameR := image.Rect(center.Min.X+8, center.Min.Y+6, center.Max.X-8, center.Min.Y+center.Dy()/2)
	drawCenteredString(nameR, game.Geishas[i].Tag+"  "+game.Geishas[i].Name, 26)
	f.Tiny.SetActive(ink.DarkGray)
	charmR := image.Rect(center.Min.X+8, nameR.Max.Y, center.Max.X-8, center.Max.Y-4)
	holderTxt := "charm " + itoa(game.Geishas[i].Charm)
	switch holder {
	case game.AIIdx:
		holderTxt += "   (datorns gunst)"
	case game.HumanIdx:
		holderTxt += "   (din gunst)"
	}
	drawCenteredString(charmR, holderTxt, 22)
}

// drawCountCell renders one side's item count. held fills the cell (this side
// owns the favor marker); leads underlines it (this side leads the round so far).
func drawCountCell(cell image.Rectangle, n int, held, leads bool, f *Fonts) {
	inner := pad(cell, 3)
	if held {
		ink.FillArea(inner, ink.LightGray)
		ink.DrawRect(inner, ink.Black)
		ink.DrawRect(pad(inner, 1), ink.Black)
	} else {
		ink.DrawRect(inner, ink.LightGray)
	}
	f.BigCount.SetActive(ink.Black)
	drawCenteredString(inner, itoa(n), 44)
	if leads && n > 0 {
		y := inner.Max.Y - 8
		ux := inner.Min.X + inner.Dx()/4
		ink.DrawLine(image.Pt(ux, y), image.Pt(inner.Max.X-inner.Dx()/4, y), ink.Black)
	}
}

// --- hand ------------------------------------------------------------------

const maxHandCols = 4

func layoutRow(area image.Rectangle, n, maxCols, cellCap int) []image.Rectangle {
	if n == 0 {
		return nil
	}
	cellGap := 14
	cols := n
	if cols > maxCols {
		cols = maxCols
	}
	rows := (n + cols - 1) / cols
	cellW := (area.Dx() - (maxCols-1)*cellGap) / maxCols
	cellH := area.Dy()/rows - cellGap
	if cellH > cellCap {
		cellH = cellCap
	}
	if cellW > cellCap*3/4 {
		cellW = cellCap * 3 / 4
	}
	gridH := rows*cellH + (rows-1)*cellGap
	originY := area.Min.Y + (area.Dy()-gridH)/2
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
	rects := layoutRow(area, len(hand), maxHandCols, 150)
	selectable := a.gs.HumanTurn() && (a.mode == modeSelect || a.mode == modeCompPair)
	for i, c := range hand {
		sel := false
		for _, s := range a.sel {
			if s == i {
				sel = true
				break
			}
		}
		drawCardChip(rects[i], c, a.fonts, sel, !selectable && a.mode != modeAction)
	}
	return rects
}

// --- prompt-bar renderers --------------------------------------------------

func drawActionButtons(area image.Rectangle, gs *game.State, f *Fonts) [game.NumActions]actBtn {
	var btns [game.NumActions]actBtn
	n := game.NumActions
	btnGap := 16
	usableW := area.Dx() - 2*sideMargin
	bw := (usableW - btnGap*(n-1)) / n
	bh := area.Dy() - 2*btnGap
	y0 := area.Min.Y + btnGap
	for i := 0; i < n; i++ {
		a := game.Action(i)
		x0 := area.Min.X + sideMargin + i*(bw+btnGap)
		r := image.Rect(x0, y0, x0+bw, y0+bh)
		live := gs.Available(game.HumanIdx, a)
		label := a.Name() + " (" + itoa(a.Cards()) + ")"
		drawTextButton(r, label, f.Button, !live)
		btns[i] = actBtn{rect: r, live: live}
	}
	return btns
}

func (a *app) drawSelectPrompt(area image.Rectangle, gr *gameRects) {
	f := a.fonts
	need := a.selAction.Cards()
	got := len(a.sel)
	msg := selectHint(a.selAction, got, need)
	if a.rejected {
		msg = "Ogiltigt val — försök igen"
	}
	f.Small.SetActive(ink.Black)
	msgR := image.Rect(area.Min.X+sideMargin, area.Min.Y+8, area.Max.X-sideMargin, area.Min.Y+46)
	drawCenteredString(msgR, msg, 26)

	btnY := msgR.Max.Y + 8
	half := (area.Dx() - 2*sideMargin - gap) / 2
	cancelR := image.Rect(area.Min.X+sideMargin, btnY, area.Min.X+sideMargin+half, area.Max.Y-12)
	confirmR := image.Rect(cancelR.Max.X+gap, btnY, area.Max.X-sideMargin, area.Max.Y-12)

	gr.cancelBtn = drawTextButton(cancelR, "Ångra", f.Button, false)
	confirmLabel := "Bekräfta"
	if a.selAction == game.Competition {
		confirmLabel = "Fortsätt"
	}
	live := got == need
	gr.confirmBtn = drawTextButton(confirmR, confirmLabel, f.Button, !live)
	gr.confirmLive = live
}

func selectHint(a game.Action, got, need int) string {
	base := ""
	switch a {
	case game.Secret:
		base = "Välj 1 kort att gömma"
	case game.TradeOff:
		base = "Välj 2 kort att kasta"
	case game.Gift:
		base = "Välj 3 kort — datorn tar 1"
	case game.Competition:
		base = "Välj 4 kort — delas i två par"
	}
	return base + "   (" + itoa(got) + "/" + itoa(need) + ")"
}

func (a *app) drawCompPairPrompt(area image.Rectangle, gr *gameRects) {
	f := a.fonts
	hand := a.gs.Players[game.HumanIdx].Hand
	f.Small.SetActive(ink.Black)
	msgR := image.Rect(area.Min.X+sideMargin, area.Min.Y+6, area.Max.X-sideMargin, area.Min.Y+40)
	drawCenteredString(msgR, "Välj hur paren delas — datorn tar ett par", 26)

	btnY := msgR.Max.Y + 6
	n := 3
	btnGap := 14
	usableW := area.Dx() - 2*sideMargin
	bw := (usableW - btnGap*(n+1)) / n
	for i := 0; i < n; i++ {
		x0 := area.Min.X + sideMargin + btnGap + i*(bw+btnGap)
		r := image.Rect(x0, btnY, x0+bw, area.Max.Y-12)
		sp := compSplits[i]
		lbl := pairLabel(hand, a.sel, sp)
		gr.pairBtns[i] = drawTextButton(r, lbl, f.Small, false)
	}
	// No cancel here: the four cards are already committed to Competition; the
	// only remaining choice is which of the three splits to offer.
}

func pairLabel(hand []game.Card, sel []int, sp [2][2]int) string {
	a1 := hand[sel[sp[0][0]]].Tag()
	a2 := hand[sel[sp[0][1]]].Tag()
	b1 := hand[sel[sp[1][0]]].Tag()
	b2 := hand[sel[sp[1][1]]].Tag()
	return a1 + a2 + " | " + b1 + b2
}

// --- offer (responding to the AI) ------------------------------------------

func drawChoosePrompt(l *Layout, gs *game.State, f *Fonts) {
	area := l.PromptBar
	ink.FillArea(area, ink.White)
	ink.DrawLine(image.Pt(area.Min.X, area.Min.Y), image.Pt(area.Max.X, area.Min.Y), ink.Black)
	f.Small.SetActive(ink.Black)
	msg := "Datorn erbjuder — tryck på det du tar"
	if gs.Pending.Action == game.Gift {
		msg = "Datorn ger dig 3 kort — tryck på det du behåller (datorn får resten)"
	} else {
		msg = "Datorn delar 4 kort i två par — tryck på paret du tar"
	}
	drawCenteredString(area, msg, 26)
}

// drawOffer renders the AI's open offer in the hand area and returns the
// tappable rects (offerCards for Gift, offerGroups for Competition).
func drawOffer(l *Layout, gs *game.State, f *Fonts) (cards, groups []image.Rectangle) {
	area := pad(l.HandArea, 4)
	ink.FillArea(l.HandArea, ink.White)
	pend := gs.Pending
	if pend.Action == game.Gift {
		rects := layoutRow(area, len(pend.Cards), 3, 150)
		for i, c := range pend.Cards {
			drawCardChip(rects[i], c, f, false, false)
		}
		return rects, nil
	}
	// Competition: two group boxes, each holding two chips.
	groups = make([]image.Rectangle, 2)
	boxGap := 30
	boxW := (area.Dx() - boxGap) / 2
	boxH := area.Dy() - 8
	for g := 0; g < 2; g++ {
		x0 := area.Min.X + g*(boxW+boxGap)
		box := image.Rect(x0, area.Min.Y+4, x0+boxW, area.Min.Y+4+boxH)
		ink.DrawRect(box, ink.Black)
		ink.DrawRect(pad(box, 1), ink.Black)
		groups[g] = box
		inner := pad(box, 10)
		chipW := (inner.Dx() - 14) / 2
		if chipW > 120 {
			chipW = 120
		}
		chipH := inner.Dy()
		if chipH > 150 {
			chipH = 150
		}
		cy := inner.Min.Y + (inner.Dy()-chipH)/2
		totalW := 2*chipW + 14
		cx := inner.Min.X + (inner.Dx()-totalW)/2
		for k := 0; k < 2; k++ {
			cr := image.Rect(cx, cy, cx+chipW, cy+chipH)
			drawCardChip(cr, pend.Groups[g][k], f, false, false)
			cx += chipW + 14
		}
	}
	return nil, groups
}

// --- round-end / match-end -------------------------------------------------

func drawRoundEnd(l *Layout, gs *game.State, f *Fonts) image.Rectangle {
	area := image.Rect(0, l.HandArea.Min.Y, l.Screen.Dx(), l.PromptBar.Max.Y)
	ink.FillArea(area, ink.White)
	ink.DrawLine(image.Pt(area.Min.X, area.Min.Y), image.Pt(area.Max.X, area.Min.Y), ink.Black)

	res := gs.Result
	f.Small.SetActive(ink.Black)
	y := area.Min.Y + 14
	hs := "Din hemliga: " + secretLabel(res.HasHumanSecret, res.HumanSecret)
	as := "Datorns hemliga: " + secretLabel(res.HasAISecret, res.AISecret)
	drawLeftString(image.Rect(area.Min.X+sideMargin, y, area.Max.X-sideMargin, y+32), hs, 26)
	y += 38
	drawLeftString(image.Rect(area.Min.X+sideMargin, y, area.Max.X-sideMargin, y+32), as, 26)

	btnW := area.Dx() / 2
	btnH := 84
	r := image.Rect((area.Dx()-btnW)/2, area.Max.Y-btnH-14, (area.Dx()+btnW)/2, area.Max.Y-14)
	next := "Nästa runda"
	if gs.MatchWinner >= 0 {
		next = "Visa resultat"
	}
	return drawTextButton(r, next, f.Button, false)
}

func secretLabel(has bool, c game.Card) string {
	if !has {
		return "(ingen)"
	}
	return c.Tag() + " (charm " + itoa(c.Charm()) + ")"
}

func drawMatchEnd(l *Layout, gs *game.State, f *Fonts) {
	area := image.Rect(0, l.HandArea.Min.Y, l.Screen.Dx(), l.PromptBar.Max.Y)
	ink.FillArea(area, ink.White)
	ink.DrawLine(image.Pt(area.Min.X, area.Min.Y), image.Pt(area.Max.X, area.Min.Y), ink.Black)

	f.Title.SetActive(ink.Black)
	banner := "Datorn vann matchen"
	if gs.MatchWinner == game.HumanIdx {
		banner = "Du vann matchen!"
	}
	drawCenteredString(image.Rect(area.Min.X, area.Min.Y+16, area.Max.X, area.Min.Y+72), banner, 40)

	hg, hc := gs.Standing(game.HumanIdx)
	ag, ac := gs.Standing(game.AIIdx)
	f.Small.SetActive(ink.Black)
	line := "Du: " + itoa(hg) + " geishor / " + itoa(hc) + " charm    Dator: " + itoa(ag) + " / " + itoa(ac)
	drawCenteredString(image.Rect(area.Min.X, area.Min.Y+80, area.Max.X, area.Min.Y+120), line, 26)
	f.Tiny.SetActive(ink.DarkGray)
	drawCenteredString(image.Rect(area.Min.X, area.Max.Y-40, area.Max.X, area.Max.Y-8), "Tryck \"Ny\" för en ny match", 22)
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
	tw := ink.StringWidth("Geishorna")
	ink.DrawString(image.Pt((screen.X-tw)/2, 80), "Geishorna")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 30, true)
	sub.SetActive(ink.Black)
	subT := "Du mot datorn — vinn geishornas gunst"
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
	drawCenteredString(sr, "Starta match", 40)
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

// --- Rules -----------------------------------------------------------------

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
	"Mål: vinn geishornas gunst. Först till gunst av 4 geishor ELLER geishor med minst 11 charm (av 21) vinner matchen. Gunstmarkörer ligger kvar mellan rundorna.",
	"7 geishor med charm 2,2,2,3,3,4,5. Varje geisha har lika många föremålskort som sin charm — 21 kort totalt. Varje runda läggs 1 kort undan dolt; båda får 6 kort.",
	"Varje runda har du fyra brickor, en användning var (du väljer ordning). Före varje handling drar du 1 kort.",
	"Hemlig (1 kort): läggs dolt, räknas för dig vid rundans slut.",
	"Kasta (2 kort): läggs bort ur rundan, räknas för ingen.",
	"Gåva (3 kort): du visar 3 kort, datorn tar 1, du behåller 2.",
	"Tävling (4 kort): du visar 4 kort i två par, datorn tar ett par, du behåller det andra.",
	"Rundslut: vänd hemliga kort. För varje geisha vinner den som lagt FLEST föremålskort av hennes typ hennes gunst — markören flyttas dit. Lika: markören står kvar (ingen vinner den).",
	"Baserat på Hanamikoji (Kota Nakayama / EmperorS4).",
}

func DrawRules(screen image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	ink.ClearScreen()
	H := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 56, true)
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

	body := ink.OpenFont(ink.DefaultFont, 27, true)
	body.SetActive(ink.Black)
	bodyMargin := 44
	maxW := screen.X - 2*bodyMargin
	y := 120
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

// --- Splash ----------------------------------------------------------------

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

// drawSplashMotif renders two geisha "tracks" with a few item cards leaning
// toward one side (the spec's requested motif).
func drawSplashMotif(box image.Rectangle) {
	f := &Fonts{
		CardTag: ink.OpenFont(ink.DefaultFontBold, 44, true),
		CardSub: ink.OpenFont(ink.DefaultFont, 22, true),
		Small:   ink.OpenFont(ink.DefaultFont, 30, true),
	}
	defer f.Close()

	cx := (box.Min.X + box.Max.X) / 2
	cy := (box.Min.Y + box.Max.Y) / 2
	cw, ch := box.Dx()/8, box.Dx()/8*13/10

	// A short row of item cards, tilted (offset) toward the right side.
	tags := []int{0, 3, 5} // Pärlan, Fjädern, Bläcket
	totalW := len(tags)*cw + (len(tags)-1)*18
	x := cx - totalW/2 + 30
	for k, g := range tags {
		yoff := k * 8
		r := image.Rect(x, cy-ch/2+yoff, x+cw, cy+ch/2+yoff)
		drawCardChip(r, game.Card{Geisha: g}, f, false, false)
		x += cw + 18
	}
	// Two favor "tracks" as bracket lines on each side.
	f.Small.SetActive(ink.Black)
	drawCenteredString(image.Rect(box.Min.X, cy+ch, box.Max.X, cy+ch+50), "Vinn geishornas gunst", 30)
}
