package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"lgame/game"
)

// Fonts held open for the app lifetime (opened once — never per draw).
type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Small  *ink.Font
	Hint   *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 34, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 38, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 38, true),
		Small:  ink.OpenFont(ink.DefaultFont, 26, true),
		Hint:   ink.OpenFont(ink.DefaultFont, 30, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Small, f.Hint} {
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
	pickerHeight    = 260
	boardMargin     = 30
	topMargin       = 40
	gapAfterStatus  = 20
	gapBeforePicker = 20
	gapBeforeBtns   = 20
)

// Layout maps between screen pixels and the 4x4 board, and lays out the
// status bar, board, orientation picker, and button bar top-to-bottom (with
// the button bar built bottom-up so it always keeps its margin — see the
// gamedev guide's layout rules).
type Layout struct {
	Screen     image.Rectangle
	StatusBar  image.Rectangle
	PickerArea image.Rectangle
	ButtonBar  image.Rectangle

	BoardArea  image.Rectangle
	GridOrigin image.Point
	CellSize   int
}

func NewLayout(screen image.Point) Layout {
	H := usableH
	l := Layout{Screen: image.Rect(0, 0, screen.X, H)}
	l.StatusBar = image.Rect(0, topMargin, screen.X, topMargin+statusBarHeight)
	l.ButtonBar = image.Rect(0, H-topMargin-buttonBarHeight, screen.X, H-topMargin)
	l.PickerArea = image.Rect(boardMargin, l.ButtonBar.Min.Y-gapBeforeBtns-pickerHeight,
		screen.X-boardMargin, l.ButtonBar.Min.Y-gapBeforeBtns)

	avail := image.Rect(boardMargin, l.StatusBar.Max.Y+gapAfterStatus,
		screen.X-boardMargin, l.PickerArea.Min.Y-gapBeforePicker)
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

// --- Rendering primitives ------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

// anchorTarget is a tappable legal L-placement, hit-tested against the
// cell that placement's own topmost-leftmost occupied square lands on (see
// LegalLPlacements' doc comment: this cell is unique per candidate
// placement of a given orientation, so it's safe to use as the tap target).
type anchorTarget struct {
	Rect image.Rectangle
	Pl   game.Placement
}

// neutralTarget is a tappable cell during the optional neutral-move step:
// either one of the two neutral pieces (before a piece is selected) or one
// of the empty destination cells (after one is selected).
type neutralTarget struct {
	Rect image.Rectangle
	Cell image.Point
}

func DrawStatus(l *Layout, text string, f *Fonts) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	drawCenteredString(l.StatusBar, text, 34)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// DrawBoard renders the grid, the pieces, and — when it's the human's turn
// to place their L-piece with an orientation already chosen — the legal
// anchor-cell highlights, populating a.anchorTargets for tap dispatch.
// During the optional neutral-move step it similarly highlights either the
// two neutral pieces (nothing selected yet) or the empty destination cells
// (a neutral piece already selected), populating a.neutralTargets.
func DrawBoard(l *Layout, a *app) {
	gs := a.gs
	ink.DrawRect(l.BoardArea, ink.Black)
	ink.DrawRect(pad(l.BoardArea, 1), ink.Black)
	for i := 1; i < game.Size; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.BoardArea.Min.Y), image.Pt(px, l.BoardArea.Max.Y), ink.Black)
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, py), image.Pt(l.BoardArea.Max.X, py), ink.Black)
	}

	interactive := gs.Phase == game.PhasePlaying && !gs.AITurn()

	a.anchorTargets = a.anchorTargets[:0]
	if interactive && gs.Step == game.StepPlaceL && a.selectedOrient >= 0 {
		for _, pl := range game.LegalLPlacements(gs.Board, gs.Turn) {
			if pl.Orient != a.selectedOrient {
				continue
			}
			anchorCell := pl.Cells[0]
			r := l.CellToScreen(anchorCell.X, anchorCell.Y)
			a.anchorTargets = append(a.anchorTargets, anchorTarget{Rect: r, Pl: pl})
		}
	}

	a.neutralTargets = a.neutralTargets[:0]
	if interactive && gs.Step == game.StepNeutralOptional {
		if a.selectedNeutral == nil {
			for y := 0; y < game.Size; y++ {
				for x := 0; x < game.Size; x++ {
					if gs.Board.At(x, y) == game.Neutral {
						a.neutralTargets = append(a.neutralTargets,
							neutralTarget{Rect: l.CellToScreen(x, y), Cell: image.Pt(x, y)})
					}
				}
			}
		} else {
			for y := 0; y < game.Size; y++ {
				for x := 0; x < game.Size; x++ {
					if gs.Board.At(x, y) == game.Empty {
						a.neutralTargets = append(a.neutralTargets,
							neutralTarget{Rect: l.CellToScreen(x, y), Cell: image.Pt(x, y)})
					}
				}
			}
		}
	}

	// Anchor highlights (drawn under the pieces so grid + pieces stay crisp).
	for _, at := range a.anchorTargets {
		markCell(at.Rect, ink.LightGray)
	}
	if interactive && gs.Step == game.StepNeutralOptional {
		for _, nt := range a.neutralTargets {
			markCell(nt.Rect, ink.LightGray)
		}
	}

	// Pieces. Black L = filled squares; White L = outlined squares; neutral
	// pieces = filled discs (a different glyph family so they read as a
	// third, unaligned piece type rather than a player's own man).
	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			cell := l.CellToScreen(x, y)
			switch gs.Board.At(x, y) {
			case game.Black:
				drawLPieceCell(cell, true)
			case game.White:
				drawLPieceCell(cell, false)
			case game.Neutral:
				drawNeutralPiece(cell)
			}
		}
	}

	// Selected-neutral-piece highlight: a bold border around its cell.
	if interactive && gs.Step == game.StepNeutralOptional && a.selectedNeutral != nil {
		r := l.CellToScreen(a.selectedNeutral.X, a.selectedNeutral.Y)
		ink.DrawRect(pad(r, 3), ink.Black)
		ink.DrawRect(pad(r, 4), ink.Black)
	}
}

// markCell draws a small centered square marker inside cell, used to
// highlight a legal tap target without obscuring the grid lines.
func markCell(cell image.Rectangle, col color.Color) {
	r := pad(cell, cell.Dx()/3)
	ink.FillArea(r, col)
}

// drawLPieceCell draws one occupied cell of an L-piece: a solid black
// square for Black's piece, or a black-outlined (hollow) square for White's.
func drawLPieceCell(cell image.Rectangle, black bool) {
	r := pad(cell, cell.Dx()/10)
	if black {
		ink.FillArea(r, ink.Black)
		return
	}
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 1), ink.Black)
	ink.DrawRect(pad(r, 2), ink.Black)
}

// drawNeutralPiece draws a neutral single-cell piece as a filled disc — a
// different glyph family from the L-pieces' squares, so it reads as its own
// (unaligned) piece type at a glance.
func drawNeutralPiece(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/4)
	fillDisc(r, ink.Black)
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

// --- Orientation picker --------------------------------------------------

// DrawOrientationPicker draws the 8 L-orientation icons in a 4x2 grid (the
// top row is the 4 rotations of the base shape, the bottom row their mirror
// image's 4 rotations — see game.LOrientations), returning their screen
// rects indexed 0-7 to match game.LOrientations' indices. Orientations with
// zero currently-legal placements are drawn dimmed (not disabled outright —
// tapping one just shows no anchor highlights, which is enough feedback).
func DrawOrientationPicker(area image.Rectangle, f *Fonts, gs *game.GameState, selected int) [8]image.Rectangle {
	var rects [8]image.Rectangle
	cols, rows := 4, 2
	cw := area.Dx() / cols
	ch := area.Dy() / rows

	legalByOrient := [8]bool{}
	for _, pl := range game.LegalLPlacements(gs.Board, gs.Turn) {
		legalByOrient[pl.Orient] = true
	}

	for i := 0; i < 8; i++ {
		col := i % cols
		row := i / cols
		r := image.Rect(area.Min.X+col*cw, area.Min.Y+row*ch,
			area.Min.X+(col+1)*cw, area.Min.Y+(row+1)*ch)
		box := pad(r, 8)
		rects[i] = box
		ink.DrawRect(box, ink.Black)
		if i == selected {
			ink.DrawRect(pad(box, 1), ink.Black)
			ink.DrawRect(pad(box, 2), ink.Black)
		}
		var col2 color.Color = ink.Black
		if !legalByOrient[i] {
			col2 = ink.DarkGray
		}
		drawOrientIcon(pad(box, 6), game.LOrientations[i], col2)
	}
	return rects
}

// drawOrientIcon renders one orientation's shape inside box using a fixed
// 3x3 mini-grid (the largest bounding box any orientation needs is 3x2 or
// 2x3). The subcell size is SQUARE (the smaller of box's two dimensions,
// divided by 3) and the whole 3x3 grid is centered within box — box itself
// is usually wide-and-short (a picker tile), and stretching the subcells to
// fill it non-uniformly would flatten every orientation into an
// unrecognizable row of wide bars instead of a readable L shape.
func drawOrientIcon(box image.Rectangle, shape []image.Point, col color.Color) {
	const gridN = 3
	side := box.Dx()
	if box.Dy() < side {
		side = box.Dy()
	}
	cell := side / gridN
	gridPx := cell * gridN
	origin := image.Pt(
		box.Min.X+(box.Dx()-gridPx)/2,
		box.Min.Y+(box.Dy()-gridPx)/2,
	)
	w, h := 0, 0
	for _, p := range shape {
		if p.X+1 > w {
			w = p.X + 1
		}
		if p.Y+1 > h {
			h = p.Y + 1
		}
	}
	offX := (gridN - w) / 2
	offY := (gridN - h) / 2
	for _, p := range shape {
		cx := p.X + offX
		cy := p.Y + offY
		r := image.Rect(origin.X+cx*cell, origin.Y+cy*cell, origin.X+(cx+1)*cell, origin.Y+(cy+1)*cell)
		ink.FillArea(pad(r, 2), col)
	}
}

// DrawNeutralHint draws a short instruction in the picker's screen area
// during the optional neutral-move step (which has no orientation picker of
// its own — that area is reused for this hint instead).
func DrawNeutralHint(area image.Rectangle, f *Fonts, hasSelection bool) {
	f.Hint.SetActive(ink.DarkGray)
	var lines []string
	if hasSelection {
		lines = wrapText("Tryck på en tom ruta dit brickan ska flyttas.", area.Dx()-20)
	} else {
		lines = wrapText("Valfritt: tryck på en neutral bricka (●) för att flytta den, eller tryck Klar för att avstå.", area.Dx()-20)
	}
	for i, ln := range lines {
		ink.DrawString(image.Pt(area.Min.X+10, area.Min.Y+20+i*38), ln)
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
	aiDepth  int
	label    string
}

type menuRow struct {
	rect   image.Rectangle
	choice menuChoice
}

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
	tw := ink.StringWidth("L-spelet")
	ink.DrawString(image.Pt((screen.X-tw)/2, 56), "L-spelet")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 30, true)
	sub.SetActive(ink.Black)
	subT := "Välj motståndare"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 150), subT)
	sub.Close()

	margin := 60
	rowW := screen.X - 2*margin

	rbW := rowW / 2
	rbH := 100
	rb := image.Rect((screen.X-rbW)/2, H-margin-rbH, (screen.X+rbW)/2, H-margin)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	f.Menu.SetActive(ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	rowH := 130
	top := 240
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
		drawCenteredString(r, c.label, 38)
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

// rulesParagraphs is the full rules text for L-spelet (the L-Game),
// mirroring the Swedish style/tone of hasami's rulesParagraphs.
var rulesParagraphs = []string{
	"Mål: se till att motståndaren INTE har någon laglig plats att lägga sin L-bit på när det blir dennes tur. Då förlorar motståndaren omedelbart.",
	"Brädet är 4x4 rutor. Varje spelare har en egen L-formad bit på fyra rutor, som kan vridas och speglas i totalt åtta olika lägen. Dessutom finns två neutrala enrutebrickor som båda spelarna delar på.",
	"På din tur MÅSTE du lyfta upp din egen L-bit och lägga ner den på en ny plats, i valfritt av de åtta lägena — så länge den nya platsen skiljer sig från den gamla (minst en ruta måste ändras) och inte täcker någon annan bricka. Detta drag är obligatoriskt: finns det någon laglig ny plats alls för din L-bit, måste du flytta den dit.",
	"Sedan får du VALFRITT flytta en av de två neutrala brickorna till vilken tom ruta som helst på brädet, utan några riktningsbegränsningar. Du kan också välja att inte röra någon neutral bricka — tryck då på Klar.",
	"Vinstvillkor: om du, precis när det blir din tur, INTE har någon enda laglig plats att lägga din L-bit på (brädet som det faktiskt ser ut, innan du skulle ha flyttat), förlorar du direkt. Spelet slutar där och då — du får inte ens försöka.",
	"Spela: tryck på en av de åtta små ikonerna för att välja vridning/spegling på din L-bit. Lagliga platser markeras direkt på brädet — tryck på en av de markerade rutorna för att lägga biten där. Grå ikoner saknar just nu någon laglig plats.",
	"Efter att L-biten är placerad: tryck på en av de två neutrala brickorna (●) för att välja den, tryck sedan på valfri tom ruta för att flytta den dit — eller tryck Klar för att avstå.",
	"L-spelet uppfanns av Edward de Bono: litet bräde, djupt spel.",
}

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

	body := ink.OpenFont(ink.DefaultFont, 28, true)
	body.SetActive(ink.Black)
	bodyMargin := 50
	maxW := screen.X - 2*bodyMargin
	y := 130
	lineH := 34
	paraGap := 12
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

// --- Splash screen -----------------------------------------------------

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

// drawSplashMotif draws the spec's splash motif: a 4x4 grid with an L-piece
// filled in one corner and 2 small neutral dots elsewhere on the grid.
func drawSplashMotif(box image.Rectangle) {
	side := box.Dx()
	if box.Dy() < side {
		side = box.Dy()
	}
	origin := image.Pt(box.Min.X+(box.Dx()-side)/2, box.Min.Y+(box.Dy()-side)/2)
	cell := side / game.Size

	grid := image.Rect(origin.X, origin.Y, origin.X+cell*game.Size, origin.Y+cell*game.Size)
	ink.DrawRect(grid, ink.Black)
	for i := 1; i < game.Size; i++ {
		px := origin.X + i*cell
		ink.DrawLine(image.Pt(px, grid.Min.Y), image.Pt(px, grid.Max.Y), ink.Black)
		py := origin.Y + i*cell
		ink.DrawLine(image.Pt(grid.Min.X, py), image.Pt(grid.Max.X, py), ink.Black)
	}

	cellRect := func(x, y int) image.Rectangle {
		return image.Rect(origin.X+x*cell, origin.Y+y*cell, origin.X+(x+1)*cell, origin.Y+(y+1)*cell)
	}
	// L-piece in the top-left corner (base orientation: a 3-long bar with a
	// foot at one end).
	for _, p := range game.LOrientations[0] {
		ink.FillArea(pad(cellRect(p.X, p.Y), cell/8), ink.Black)
	}
	// Two small neutral dots elsewhere on the grid.
	fillDisc(pad(cellRect(3, 0), cell/4), ink.Black)
	fillDisc(pad(cellRect(0, 3), cell/4), ink.Black)
}
