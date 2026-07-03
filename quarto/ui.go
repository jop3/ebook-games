package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"quarto/game"
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
		Menu:   ink.OpenFont(ink.DefaultFont, 40, true),
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

	statusBarHeight = 80
	buttonBarHeight = 130
	activeBarHeight = 150
	boardMargin     = 20
	topMargin       = 40
	poolCols        = 8 // pool of up to 16 pieces laid out 8 per row
)

// Layout maps between screen pixels and the 4x4 board, plus the piece pool.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	ActiveBar image.Rectangle
	PoolArea  image.Rectangle
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
	l.PoolArea = image.Rect(0, l.ButtonBar.Min.Y-boardMargin-2*poolRowHeight(screen.X),
		screen.X, l.ButtonBar.Min.Y-boardMargin)
	l.ActiveBar = image.Rect(0, l.PoolArea.Min.Y-boardMargin-activeBarHeight,
		screen.X, l.PoolArea.Min.Y-boardMargin)

	avail := image.Rect(boardMargin, l.StatusBar.Max.Y+boardMargin,
		screen.X-boardMargin, l.ActiveBar.Min.Y-boardMargin)
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

// poolRowHeight sizes each of the 2 pool rows (16 pieces / 8 cols = 2 rows).
func poolRowHeight(screenW int) int {
	cell := screenW / poolCols
	return cell
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

// --- Rendering -----------------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

// PoolButton is a tappable piece in the pool tray.
type PoolButton struct {
	Rect  image.Rectangle
	Piece game.Piece
}

func (b PoolButton) Hit(p image.Point) bool { return p.In(b.Rect) }

func DrawStatus(l *Layout, text string, f *Fonts) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	drawCenteredString(l.StatusBar, text, 34)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawBoard renders the 4x4 grid and any placed pieces.
func DrawBoard(l *Layout, s *game.GameState, f *Fonts) {
	ink.DrawRect(l.BoardArea, ink.Black)
	ink.DrawRect(pad(l.BoardArea, 1), ink.Black)
	for i := 1; i < game.Size; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.BoardArea.Min.Y), image.Pt(px, l.BoardArea.Max.Y), ink.Black)
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, py), image.Pt(l.BoardArea.Max.X, py), ink.Black)
	}

	// Highlight the winning line, if any.
	var winSet map[int]bool
	if s.Phase == game.PhaseWon {
		winSet = make(map[int]bool, 4)
		for _, idx := range s.WinLine {
			winSet[idx] = true
		}
	}

	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			p := s.Board.At(x, y)
			if p == game.NoPiece {
				continue
			}
			cell := l.CellToScreen(x, y)
			if winSet != nil && winSet[y*game.Size+x] {
				ink.FillArea(pad(cell, 4), ink.LightGray)
			}
			drawPiece(cell, p)
		}
	}
}

// DrawPool renders the "piece to place" slot and the remaining pool below the
// board, and returns tappable rects for each pool piece (only meaningful
// during StepGive).
func DrawPool(l *Layout, s *game.GameState, f *Fonts) []PoolButton {
	// Active-piece bar: label + the piece itself (if any).
	ink.FillArea(l.ActiveBar, ink.White)
	ink.DrawLine(image.Pt(l.ActiveBar.Min.X, l.ActiveBar.Min.Y),
		image.Pt(l.ActiveBar.Max.X, l.ActiveBar.Min.Y), ink.Black)
	f.Small.SetActive(ink.DarkGray)
	label := "Bricka att placera:"
	if s.Step == game.StepGive {
		label = "Bricka du gav bort:"
	}
	lw := ink.StringWidth(label)
	labelY := l.ActiveBar.Min.Y + 10
	ink.DrawString(image.Pt((l.Screen.Dx()-lw)/2, labelY), label)

	slotSize := l.ActiveBar.Dy() - 60
	if slotSize > 140 {
		slotSize = 140
	}
	cx := l.Screen.Dx() / 2
	slotY := labelY + 40
	slot := image.Rect(cx-slotSize/2, slotY, cx+slotSize/2, slotY+slotSize)
	if s.ActivePiece != game.NoPiece {
		ink.DrawRect(slot, ink.Black)
		drawPiece(slot, s.ActivePiece)
	} else {
		ink.DrawRect(slot, ink.LightGray)
	}

	// Pool tray: remaining pieces in a grid, tappable only during StepGive.
	ink.FillArea(l.PoolArea, ink.White)
	ink.DrawLine(image.Pt(l.PoolArea.Min.X, l.PoolArea.Min.Y),
		image.Pt(l.PoolArea.Max.X, l.PoolArea.Min.Y), ink.Black)

	n := len(s.Pool)
	if n == 0 {
		return nil
	}
	cols := poolCols
	rows := (n + cols - 1) / cols
	if rows < 1 {
		rows = 1
	}
	cellW := l.PoolArea.Dx() / cols
	cellH := l.PoolArea.Dy() / rows
	if cellH > cellW {
		cellH = cellW
	}
	totalW := cellW * cols
	originX := l.PoolArea.Min.X + (l.PoolArea.Dx()-totalW)/2
	totalH := cellH * rows
	originY := l.PoolArea.Min.Y + (l.PoolArea.Dy()-totalH)/2

	active := s.Step == game.StepGive
	buttons := make([]PoolButton, 0, n)
	for i, p := range s.Pool {
		col := i % cols
		row := i / cols
		r := image.Rect(originX+col*cellW, originY+row*cellH,
			originX+(col+1)*cellW, originY+(row+1)*cellH)
		cellR := pad(r, 4)
		if active {
			ink.DrawRect(cellR, ink.Black)
		} else {
			ink.DrawRect(cellR, ink.LightGray)
		}
		drawPiece(cellR, p)
		buttons = append(buttons, PoolButton{Rect: r, Piece: p})
	}
	return buttons
}

// drawPiece renders a Quarto piece inside cell using four independent visual
// channels so all 4 binary attributes read clearly in greyscale:
//   - Size:   big square (Tall) vs a smaller centered square (Short)
//   - Shape:  square outline (Square) vs circular outline (Round)
//   - Fill:   solid black fill (Solid) vs hollow/outline only (Hollow)
//   - Color:  a white "hole" dot in the center marks Dark; no dot marks Light
func drawPiece(cell image.Rectangle, p game.Piece) {
	tall := p&game.AttrTall != 0
	dark := p&game.AttrDark != 0
	square := p&game.AttrSquare != 0
	solid := p&game.AttrSolid != 0

	inset := cell.Dx() / 6
	if !tall {
		inset = cell.Dx() * 3 / 10
	}
	r := pad(cell, inset)
	if r.Dx() < 6 || r.Dy() < 6 {
		return
	}

	if square {
		if solid {
			ink.FillArea(r, ink.Black)
		} else {
			ink.DrawRect(r, ink.Black)
			ink.DrawRect(pad(r, 1), ink.Black)
		}
	} else {
		if solid {
			fillDisc(r, ink.Black)
		} else {
			drawRingDisc(r)
		}
	}

	if dark {
		// Small white "hole" dot in the center marks the Dark attribute.
		cx := (r.Min.X + r.Max.X) / 2
		cy := (r.Min.Y + r.Max.Y) / 2
		hr := r.Dx() / 6
		if hr < 3 {
			hr = 3
		}
		fillDisc(image.Rect(cx-hr, cy-hr, cx+hr, cy+hr), ink.White)
		// Outline the hole so it reads on a hollow/ring piece too.
		ink.DrawRect(image.Rect(cx-hr, cy-hr, cx+hr, cy+hr), ink.Black)
	}
}

// drawRingDisc draws a circular outline only (hollow round piece).
func drawRingDisc(r image.Rectangle) {
	fillDisc(r, ink.Black)
	inner := pad(r, r.Dx()/6+2)
	fillDisc(inner, ink.White)
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

// --- Menu ----------------------------------------------------------------

type menuChoice struct {
	mode    game.Mode
	aiLevel int
	label   string
}

type menuRow struct {
	rect   image.Rectangle
	choice menuChoice
}

type Menu struct {
	rows     []menuRow
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{} }

// choices lists the game modes offered on the start screen.
var choices = []menuChoice{
	{game.ModeHotseat, 0, "2 spelare (hot-seat)"},
	{game.ModeAI, 2, "Mot dator – Lätt"},
	{game.ModeAI, 4, "Mot dator – Medel"},
	{game.ModeAI, 6, "Mot dator – Svår"},
}

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	H := usableH

	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Quarto!")
	ink.DrawString(image.Pt((screen.X-tw)/2, 70), "Quarto!")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 34, true)
	sub.SetActive(ink.Black)
	subT := "Välj spelläge"
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
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	// Mode rows fill the space between the subtitle and the Regler button.
	f.Menu.SetActive(ink.Black)
	rowH := 130
	top := 300
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
		drawLeftString(r, c.label, 40)
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

// rulesParagraphs is the rules text for Quarto!, one entry per paragraph.
var rulesParagraphs = []string{
	"Mål: bygg en rad, kolumn eller diagonal av fyra brickor som alla delar minst en egenskap.",
	"Det finns 16 unika brickor. Varje bricka har fyra egenskaper: hög/låg, mörk/ljus, fyrkantig/rund, solid/ihålig (vit prick i mitten = mörk).",
	"Brickorna delas mellan spelarna — ingen äger några brickor. I stället väljer motståndaren vilken bricka du måste placera.",
	"Ditt drag har två steg: 1) placera brickan du fick av motståndaren på en ledig ruta, 2) välj en bricka ur den kvarvarande poolen och ge den till motståndaren.",
	"Du vinner så fort en rad, kolumn eller diagonal med fyra brickor blir komplett OCH alla fyra delar minst en egenskap — t.ex. alla höga, eller alla ljusa.",
	"Viktigt: det räcker att brickorna delar EN egenskap. Fyra korta brickor vinner lika gärna som fyra höga — även om de i övrigt är helt olika.",
	"Den som placerar den avgörande brickan vinner, även om det var motståndaren som valde ut den brickan åt dig — tänk efter innan du ger bort en bricka!",
	"Blir brädet fullt utan någon vinstlinje är det oavgjort.",
}

// DrawRules renders the scrolling rules text with a back button and returns
// the back button rect.
func DrawRules(screen image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	ink.ClearScreen()
	H := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 56, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, 60), title)
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

	body := ink.OpenFont(ink.DefaultFont, 32, true)
	body.SetActive(ink.Black)
	bodyMargin := 60
	maxW := screen.X - 2*bodyMargin
	y := 160
	lineH := 42
	paraGap := 20
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

	// Motif occupies a centered square in the middle of the screen.
	side := screen.X * 3 / 5
	box := image.Rect((screen.X-side)/2, (H-side)/2,
		(screen.X+side)/2, (H+side)/2)
	motif(box)

	hint := ink.OpenFont(ink.DefaultFont, 34, true)
	hint.SetActive(ink.DarkGray)
	ht := "Tryck för att börja"
	hw := ink.StringWidth(ht)
	ink.DrawString(image.Pt((screen.X-hw)/2, H*5/6), ht)
	hint.Close()
}

// drawSplashMotif draws four distinct Quarto pieces in a row, chosen to show
// contrasts across all four attributes: big-solid-square(-dark), small-ring-
// circle(-light), big-ring-square(-dark), small-solid-circle(-light).
func drawSplashMotif(box image.Rectangle) {
	cy := (box.Min.Y + box.Max.Y) / 2
	cellW := box.Dx() / 4
	pieces := []game.Piece{
		game.AttrTall | game.AttrSolid | game.AttrSquare | game.AttrDark,
		game.AttrDark,
		game.AttrTall | game.AttrSquare,
		game.AttrSolid,
	}
	for i, p := range pieces {
		cx := box.Min.X + cellW*i + cellW/2
		size := cellW * 7 / 10
		cell := image.Rect(cx-size/2, cy-size/2, cx+size/2, cy+size/2)
		drawPiece(cell, p)
	}
}
