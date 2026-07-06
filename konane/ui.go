package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"konane/game"
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

// --- Rendering -----------------------------------------------------------------

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

// DrawBoard renders the grid, stones, the current selection/chain highlight
// (if any), and a brief marker over cells captured so far this turn.
func DrawBoard(l *Layout, a *app) {
	s := a.gs
	// Grid frame + lines.
	ink.DrawRect(l.BoardArea, ink.Black)
	ink.DrawRect(pad(l.BoardArea, 1), ink.Black)
	for i := 1; i < game.Size; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.BoardArea.Min.Y), image.Pt(px, l.BoardArea.Max.Y), ink.Black)
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, py), image.Pt(l.BoardArea.Max.X, py), ink.Black)
	}

	// Opening-phase highlights: the removable stone(s) for whichever
	// half-step is active right now.
	switch s.Phase {
	case game.PhaseOpeningBlackRemove:
		for _, p := range game.CenterRemovalOptions() {
			highlightCell(l, p)
		}
	case game.PhaseOpeningWhiteRemove:
		for _, p := range s.OpeningWhiteOptions() {
			highlightCell(l, p)
		}
	case game.PhasePlaying:
		// Legal jump-destination hints, either from the pending selection or
		// from the stone currently mid-chain.
		var from image.Point
		show := false
		if s.ChainActive {
			from, show = s.ChainFrom, true
		} else if a.hasSelection {
			from, show = a.selected, true
		}
		if show {
			for _, j := range s.Board.LegalJumpsFrom(from, s.Turn) {
				highlightCell(l, j.To)
			}
		}
	}

	// Stones. Black = filled disc; White = hollow (ring) disc.
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			switch s.Board.At(x, y) {
			case game.Black:
				drawStone(l.CellToScreen(x, y), true)
			case game.White:
				drawStone(l.CellToScreen(x, y), false)
			}
		}
	}

	// Selection / chain-source highlight: a bold border around the relevant
	// square.
	if s.Phase == game.PhasePlaying {
		if s.ChainActive {
			r := l.CellToScreen(s.ChainFrom.X, s.ChainFrom.Y)
			ink.DrawRect(pad(r, 2), ink.Black)
			ink.DrawRect(pad(r, 3), ink.Black)
		} else if a.hasSelection {
			r := l.CellToScreen(a.selected.X, a.selected.Y)
			ink.DrawRect(pad(r, 2), ink.Black)
			ink.DrawRect(pad(r, 3), ink.Black)
		}
	}

	// Briefly mark cells captured so far this turn (already empty; the
	// marker vanishes on its own once the next turn's first jump replaces
	// LastCaptured, so there is no separate timer to manage).
	for _, p := range s.LastCaptured {
		drawCaptureMark(l.CellToScreen(p.X, p.Y))
	}
}

// highlightCell fills a small centered square in cell (x,y) to mark it as a
// legal tap target (an opening-removal option or a jump destination).
func highlightCell(l *Layout, p image.Point) {
	cell := l.CellToScreen(p.X, p.Y)
	cx, cy := (cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2
	r := l.CellSize / 10
	if r < 2 {
		r = 2
	}
	ink.FillArea(image.Rect(cx-r, cy-r, cx+r, cy+r), ink.LightGray)
}

// drawStone draws a stone inside a cell. black=true renders a solid disc;
// false renders a hollow (white) disc with a black outline so both read on
// e-ink.
func drawStone(cell image.Rectangle, black bool) {
	r := pad(cell, cell.Dx()/8)
	fillDisc(r, ink.Black)
	if !black {
		inner := pad(r, r.Dx()/8+2)
		fillDisc(inner, ink.White)
	}
}

// drawCaptureMark overlays a diagonal cross on a (now-empty) cell to briefly
// flag it as just-captured — e-ink has no real animation, so a single-frame
// distinct marker stands in for one.
func drawCaptureMark(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/4)
	ink.DrawLine(image.Pt(r.Min.X, r.Min.Y), image.Pt(r.Max.X, r.Max.Y), ink.DarkGray)
	ink.DrawLine(image.Pt(r.Max.X, r.Min.Y), image.Pt(r.Min.X, r.Max.Y), ink.DarkGray)
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
	tw := ink.StringWidth("Konane")
	ink.DrawString(image.Pt((screen.X-tw)/2, 56), "Konane")
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
	f.Menu.SetActive(ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	// Opponent rows fill the space between the subtitle and the Regler
	// button.
	rowH := 116
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

// rulesParagraphs is the full Swedish rules text for Konane.
var rulesParagraphs = []string{
	"Mål: Konane har bara ett vinstvillkor — får motståndaren inget lagligt hopp alls på sin tur, förlorar hen omedelbart.",
	"Brädet är 8x8 och är från början helt fyllt av svarta och vita stenar i ett rutmönster, precis som ett fullsatt schackbräde. Det finns inga tomma rutor förrän öppningen tar bort de två första stenarna.",
	"Öppning (sker bara en enda gång, i spelets allra första drag): Svart tar bort en av de två mittersta stenarna. Vit tar sedan bort en av sina egna stenar som ligger direkt intill (uppåt, nedåt, till vänster eller höger om) den lucka Svart just skapade.",
	"Efter öppningen finns det bara EN sorts drag i hela spelet: hopp. Det förekommer inga vanliga steg-drag alls, och det går aldrig att passa.",
	"Ett hopp: flytta en av dina stenar rakt vågrätt eller lodrätt, över en angränsande fiendesten, till den tomma rutan direkt bortom den. Fiendestenen som hoppades över tas bort från brädet.",
	"En sten får fortsätta hoppa flera gånger i samma drag — en hoppkedja — så länge varje nytt hopp är lagligt utifrån den ruta stenen just landat på. Du väljer själv hur långt du vill fortsätta kedjan; du är aldrig tvungen att ta den längsta möjliga kedjan.",
	"Du MÅSTE göra minst det första hoppet i ditt drag om något lagligt hopp finns någonstans på brädet för dig, oavsett med vilken sten. Har du inget lagligt hopp alls när det blir din tur förlorar du omedelbart — det finns ingen möjlighet att passa.",
	"Tryck på en egen sten som har ett lagligt hopp för att välja den — möjliga landningsrutor markeras. Tryck på en av dem för att hoppa. Kan kedjan fortsätta kan du trycka på nästa markerade ruta för att hoppa igen, eller trycka på \"Klart\" för att avsluta draget i förtid och lämna kvar resten av kedjan olagd.",
	"Konane är ett traditionellt hawaiiskt brädspel — namnet betyder ungefär \"stenhoppning\" — som historiskt spelades utomhus på ett bräde ristat direkt i lavasten, med svarta lavastenar och vita korallbitar.",
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
	y := 140
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

// --- Splash screen ---------------------------------------------------------

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

// drawSplashMotif draws a small checkerboard corner (like the real starting
// board) with the traditional opening gap left empty and a jump arrow arcing
// over one stone into that gap — Konane's signature opening move.
func drawSplashMotif(box image.Rectangle) {
	const n = 4 // a small demo grid, not the full 8x8
	side := box.Dx()
	if box.Dy() < side {
		side = box.Dy()
	}
	cell := side / n
	origin := image.Pt(box.Min.X+(box.Dx()-cell*n)/2, box.Min.Y+(box.Dy()-cell*n)/2)

	grid := image.Rect(origin.X, origin.Y, origin.X+cell*n, origin.Y+cell*n)
	ink.DrawRect(grid, ink.Black)
	for i := 1; i < n; i++ {
		x := origin.X + i*cell
		ink.DrawLine(image.Pt(x, grid.Min.Y), image.Pt(x, grid.Max.Y), ink.Black)
		y := origin.Y + i*cell
		ink.DrawLine(image.Pt(grid.Min.X, y), image.Pt(grid.Max.X, y), ink.Black)
	}

	gap := image.Pt(1, 1)      // the empty opening gap
	over := image.Pt(1, 2)     // the stone that gets jumped
	jumper := image.Pt(1, 3)   // the stone that jumps, two cells below the gap
	cellCenter := func(p image.Point) image.Point {
		return image.Pt(origin.X+p.X*cell+cell/2, origin.Y+p.Y*cell+cell/2)
	}

	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			p := image.Pt(x, y)
			if p == gap {
				continue // leave the gap empty
			}
			r := pad(image.Rect(origin.X+x*cell, origin.Y+y*cell, origin.X+(x+1)*cell, origin.Y+(y+1)*cell), cell/8)
			if (x+y)%2 == 0 {
				fillDisc(r, ink.Black)
			} else {
				fillDisc(r, ink.Black)
				fillDisc(pad(r, r.Dx()/8+2), ink.White)
			}
		}
	}

	// Jump arrow: an arc from the jumper's cell, up over the jumped stone,
	// landing in the gap.
	from := cellCenter(jumper)
	to := cellCenter(gap)
	apex := image.Pt((from.X+to.X)/2-cell/2, cellCenter(over).Y)
	ink.DrawLine(from, apex, ink.Black)
	ink.DrawLine(apex, to, ink.Black)
	// Arrowhead at the landing point.
	ink.DrawLine(to, image.Pt(to.X-cell/5, to.Y-cell/6), ink.Black)
	ink.DrawLine(to, image.Pt(to.X+cell/6, to.Y-cell/5), ink.Black)
}
