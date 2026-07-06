package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"stadskarnan/game"
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
		Button: ink.OpenFont(ink.DefaultFontBold, 36, true),
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

	topMargin       = 46
	bottomMargin    = 40
	statusBarHeight = 80
	buttonBarHeight = 118
	trayHeight      = 160
	trayCols        = 7
	trayRows        = 2
	boardMargin     = 14
	gap             = 14
)

// Layout maps between screen pixels and the 10x10 board, plus the fixed
// per-piece-ID tray slot rectangles.
type Layout struct {
	Screen    image.Rectangle
	StatusBar image.Rectangle
	Tray      image.Rectangle
	ButtonBar image.Rectangle

	BoardArea  image.Rectangle
	GridOrigin image.Point
	CellSize   int

	TrayRects [game.NumPieces]image.Rectangle
}

func NewLayout(screen image.Point) Layout {
	H := usableH
	l := Layout{Screen: image.Rect(0, 0, screen.X, H)}

	l.ButtonBar = image.Rect(0, H-bottomMargin-buttonBarHeight, screen.X, H-bottomMargin)
	trayBottom := l.ButtonBar.Min.Y - gap
	l.Tray = image.Rect(0, trayBottom-trayHeight, screen.X, trayBottom)
	l.StatusBar = image.Rect(0, topMargin, screen.X, topMargin+statusBarHeight)

	avail := image.Rect(boardMargin, l.StatusBar.Max.Y+gap,
		screen.X-boardMargin, l.Tray.Min.Y-gap)
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

	// Fixed 7x2 tray grid: 13 of the 14 slots are used, in piece-ID order,
	// so a piece's position never moves as pieces around it get placed.
	trayInner := pad(l.Tray, 8)
	slotGap := 6
	slotW := (trayInner.Dx() - slotGap*(trayCols-1)) / trayCols
	slotH := (trayInner.Dy() - slotGap*(trayRows-1)) / trayRows
	for id := 0; id < game.NumPieces; id++ {
		row := id / trayCols
		col := id % trayCols
		x0 := trayInner.Min.X + col*(slotW+slotGap)
		y0 := trayInner.Min.Y + row*(slotH+slotGap)
		l.TrayRects[id] = image.Rect(x0, y0, x0+slotW, y0+slotH)
	}
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

// DrawBoard renders the grid, every placed piece (Black solid, White
// hollow, Cathedral hatched), sealed cells (dashed outline — permanently
// unusable), legal placement anchors for the current selection (or, during
// the Cathedral phase, every legal Cathedral anchor), and a brief flash over
// cells affected by the most recent placement.
func DrawBoard(l *Layout, a *app) {
	s := a.gs
	ink.DrawRect(l.BoardArea, ink.Black)
	ink.DrawRect(pad(l.BoardArea, 1), ink.Black)
	for i := 1; i < game.Size; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.BoardArea.Min.Y), image.Pt(px, l.BoardArea.Max.Y), ink.LightGray)
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, py), image.Pt(l.BoardArea.Max.X, py), ink.LightGray)
	}

	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			cell := l.CellToScreen(x, y)
			switch s.Board.At(x, y) {
			case game.Black:
				drawBuilding(cell, true)
			case game.White:
				drawBuilding(cell, false)
			case game.Cathedral:
				drawCathedralCell(cell)
			default:
				if s.Board.IsSealed(x, y) {
					drawSealedMark(cell)
				}
			}
		}
	}

	// Legal placement anchors.
	if s.Phase == game.PhaseCathedral {
		for _, p := range game.LegalCathedralPlacements(&s.Board) {
			drawAnchorHint(l.CellToScreen(p.Anchor.X, p.Anchor.Y))
		}
	} else if s.Phase == game.PhasePlaying && !s.AITurn() && a.selectedPiece >= 0 {
		for _, p := range game.LegalPlacementsForOrientation(&s.Board, a.selectedPiece, a.orientIdx) {
			drawAnchorHint(l.CellToScreen(p.Anchor.X, p.Anchor.Y))
		}
	}

	// Briefly flag cells captured or newly sealed by the most recent
	// placement (e-ink has no real animation; a distinct one-frame marker
	// stands in for one, and vanishes on its own once the next placement
	// replaces the list).
	for _, p := range s.LastCaptured {
		drawCaptureMark(l.CellToScreen(p.X, p.Y))
	}
}

// drawBuilding renders a placed building cell: Black solid, White hollow —
// square blocks (not discs), reading as distinct city buildings.
func drawBuilding(cell image.Rectangle, black bool) {
	r := pad(cell, cell.Dx()/8+1)
	ink.FillArea(r, ink.Black)
	if !black {
		ink.FillArea(pad(r, r.Dx()/6+2), ink.White)
	}
}

// drawCathedralCell fills a Cathedral-owned cell with DarkGray, distinct
// from either player's building color on this greyscale-only display.
func drawCathedralCell(cell image.Rectangle) {
	ink.FillArea(pad(cell, cell.Dx()/8+1), ink.DarkGray)
}

// drawSealedMark outlines a permanently-blocked empty cell with a dashed
// square (short line segments), the same "no more building here" motif used
// on the splash screen.
func drawSealedMark(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/4)
	dash := 5
	for x := r.Min.X; x < r.Max.X; x += dash * 2 {
		x2 := x + dash
		if x2 > r.Max.X {
			x2 = r.Max.X
		}
		ink.DrawLine(image.Pt(x, r.Min.Y), image.Pt(x2, r.Min.Y), ink.LightGray)
		ink.DrawLine(image.Pt(x, r.Max.Y), image.Pt(x2, r.Max.Y), ink.LightGray)
	}
	for y := r.Min.Y; y < r.Max.Y; y += dash * 2 {
		y2 := y + dash
		if y2 > r.Max.Y {
			y2 = r.Max.Y
		}
		ink.DrawLine(image.Pt(r.Min.X, y), image.Pt(r.Min.X, y2), ink.LightGray)
		ink.DrawLine(image.Pt(r.Max.X, y), image.Pt(r.Max.X, y2), ink.LightGray)
	}
}

// drawAnchorHint marks a cell as a legal tap target for the current
// placement (its shape's anchor cell).
func drawAnchorHint(cell image.Rectangle) {
	cx, cy := (cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2
	r := cell.Dx() / 6
	if r < 3 {
		r = 3
	}
	ink.FillArea(image.Rect(cx-r, cy-r, cx+r, cy+r), ink.DarkGray)
}

// drawCaptureMark overlays a diagonal cross on a (now-empty, sealed) cell to
// briefly flag it as just-captured.
func drawCaptureMark(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/4)
	ink.DrawLine(image.Pt(r.Min.X, r.Min.Y), image.Pt(r.Max.X, r.Max.Y), ink.DarkGray)
	ink.DrawLine(image.Pt(r.Max.X, r.Min.Y), image.Pt(r.Min.X, r.Max.Y), ink.DarkGray)
}

// DrawTray renders one slot per building piece ID (fixed 7x2 grid, stable
// positions). A piece no longer available (already placed and not since
// captured back) is skipped (left blank); an available piece shows a small
// line-art rendering of its current shape (its base orientation, unless it
// is the selected piece, in which case its CURRENTLY chosen rotation is
// shown so rotating is visible before committing); the selected slot gets a
// bold double border.
func DrawTray(l *Layout, a *app) {
	s := a.gs
	hand := s.Hand(s.Turn)
	for id := 0; id < game.NumPieces; id++ {
		r := l.TrayRects[id]
		if !hand[id] {
			continue
		}
		cells := game.Pieces[id].Cells
		if id == a.selectedPiece {
			orients := game.Orientations(game.Pieces[id].Cells)
			if a.orientIdx >= 0 && a.orientIdx < len(orients) {
				cells = orients[a.orientIdx]
			}
		}
		ink.DrawRect(r, ink.Black)
		if id == a.selectedPiece {
			ink.DrawRect(pad(r, 2), ink.Black)
		}
		drawPieceIcon(pad(r, 6), cells)
	}
}

// drawPieceIcon draws a small line-art rendering of a piece's shape (a list
// of normalized cell offsets) scaled to fit inside box.
func drawPieceIcon(box image.Rectangle, cells []game.Offset) {
	maxX, maxY := game.BoundingBox(cells)
	cols, rows := maxX+1, maxY+1
	cell := box.Dx() / cols
	if h := box.Dy() / rows; h < cell {
		cell = h
	}
	if cell < 2 {
		cell = 2
	}
	offX := box.Min.X + (box.Dx()-cell*cols)/2
	offY := box.Min.Y + (box.Dy()-cell*rows)/2
	for _, c := range cells {
		r := image.Rect(offX+c[0]*cell, offY+c[1]*cell, offX+(c[0]+1)*cell, offY+(c[1]+1)*cell)
		ink.FillArea(pad(r, 1), ink.Black)
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
	sideGap := 20
	sideMargin := 24
	usableW := l.ButtonBar.Dx() - 2*sideMargin
	totalGap := sideGap * (n + 1)
	bw := (usableW - totalGap) / n
	bh := l.ButtonBar.Dy() - 2*sideGap
	buttons := make([]Button, n)
	for i, label := range labels {
		x0 := l.ButtonBar.Min.X + sideMargin + sideGap + i*(bw+sideGap)
		y0 := l.ButtonBar.Min.Y + sideGap
		r := image.Rect(x0, y0, x0+bw, y0+bh)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawCenteredString(r, label, 36)
		buttons[i] = Button{Rect: r, Label: label}
	}
	return buttons
}

// --- Menu ------------------------------------------------------------------

type Menu struct {
	rows     [2]image.Rectangle // [0]=hotseat, [1]=vs AI
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{} }

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	H := usableH

	title := ink.OpenFont(ink.DefaultFontBold, 60, true)
	title.SetActive(ink.Black)
	titleText := "Stadskärnan"
	tw := ink.StringWidth(titleText)
	ink.DrawString(image.Pt((screen.X-tw)/2, 56), titleText)
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 28, true)
	sub.SetActive(ink.Black)
	subT := "Bygg stadskärnan och stäng in motståndaren"
	sw := ink.StringWidth(subT)
	ink.DrawString(image.Pt((screen.X-sw)/2, 130), subT)
	sub.Close()

	margin := 60
	rowW := screen.X - 2*margin

	rbW := rowW / 2
	rbH := 100
	rb := image.Rect((screen.X-rbW)/2, H-margin-rbH, (screen.X+rbW)/2, H-margin)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	f.Button.SetActive(ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	labels := [2]string{"2 spelare (hot-seat)", "Mot dator (övning)"}
	top := 200
	bottom := rb.Min.Y - 30
	rowH := 140
	n := 2
	avail := bottom - top
	if avail < rowH*n {
		rowH = avail / n
	}
	// Center the option rows in the space between the subtitle and the
	// Regler button, rather than always packing them against the top (the
	// available gap varies with paragraph/font sizes).
	y := top + (avail-rowH*n)/3
	f.Menu.SetActive(ink.Black)
	for i := 0; i < n; i++ {
		r := image.Rect(margin, y, margin+rowW, y+rowH-24)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawLeftString(r, labels[i], 38)
		m.rows[i] = r
		y += rowH
	}
}

func (m *Menu) HandleTouch(p image.Point) (game.Opponent, bool) {
	if p.In(m.rows[0]) {
		return game.OpponentHotseat, true
	}
	if p.In(m.rows[1]) {
		return game.OpponentAI, true
	}
	return 0, false
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

// rulesParagraphs is the Swedish rules text for Stadskärnan, kept tight
// enough (short paragraphs, no repetition) to fit above the back button at
// 1072x1340 even at 10 paragraphs — verified in the emulator.
var rulesParagraphs = []string{
	"Mål: få ut mer av din egen byggnadsyta på brädet än motståndaren.",
	"Brädet är 10x10. Svart placerar först den neutrala Katedralen (fem rutor, ingen ägare). Sedan är det Vits tur.",
	"Varje spelare har 13 byggnader (1–4 rutor). Turas om att placera EN bit per drag, på en ledig ruta, i valfri rotation eller spegling.",
	"Efter varje drag kontrolleras alla lediga områden. Ett område som når brädets kant är alltid öppet — det kan aldrig stängas in.",
	"Ett område som INTE når kanten, och vars hela gräns är EN färgs byggnader (och/eller Katedralen), blir instängt: motståndarens byggnader däri tas bort och läggs i dennes förråd för att byggas igen; finns inga sådana däri förseglas området istället — för alltid.",
	"Saknar du en placerbar bit på din tur passar du. Detta kontrolleras på nytt varje drag, så ett senare drag kan öppna plats igen.",
	"Spelet slutar när ingen kan placera fler bitar. Vinnaren är den med FÄRST kvarvarande rutor i sitt förråd.",
	"Tryck på en bit i raden längst ner för att välja den — lediga rutor markeras. Tryck \"Rotera\" för att vända/spegla, tryck sedan en markerad ruta för att bygga.",
	"Mot dator: en enkel övningsnivå som väljer draget med bäst ytemarginal, med en kort titt på motståndarens bästa svar. Ingen stark spelare — hot-seat är huvudläget.",
	"Baserat på Cathedral (Robert P. Moore) — bitarnas former och namn här är egna.",
}

func DrawRules(screen image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	ink.ClearScreen()
	H := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 46, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, 34), title)
	tf.Close()

	margin := 30
	bh := 92
	bw := screen.X / 2
	r := image.Rect((screen.X-bw)/2, H-margin-bh, (screen.X+bw)/2, H-margin)
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 1), ink.Black)
	f.Button.SetActive(ink.Black)
	drawCenteredString(r, "Tillbaka", 34)

	body := ink.OpenFont(ink.DefaultFont, 26, true)
	body.SetActive(ink.Black)
	bodyMargin := 44
	maxW := screen.X - 2*bodyMargin
	y := 100
	lineH := 33
	paraGap := 13
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

// --- Splash screen ---------------------------------------------------------

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

// drawSplashMotif draws a small board corner: the cross-shaped Cathedral
// piece (dark gray), one small building (a solid black block), and a dashed
// outline around an empty pocket — suggesting the enclosure mechanic.
func drawSplashMotif(box image.Rectangle) {
	cols, rows := 5, 4
	cell := box.Dx() / cols
	if h := box.Dy() / rows; h < cell {
		cell = h
	}
	ox := box.Min.X + (box.Dx()-cell*cols)/2
	oy := box.Min.Y + (box.Dy()-cell*rows)/2
	cellAt := func(cx, cy int) image.Rectangle {
		return image.Rect(ox+cx*cell, oy+cy*cell, ox+(cx+1)*cell, oy+(cy+1)*cell)
	}

	// Board corner grid.
	grid := image.Rect(ox, oy, ox+cols*cell, oy+rows*cell)
	ink.DrawRect(grid, ink.Black)
	for i := 1; i < cols; i++ {
		x := ox + i*cell
		ink.DrawLine(image.Pt(x, grid.Min.Y), image.Pt(x, grid.Max.Y), ink.LightGray)
	}
	for i := 1; i < rows; i++ {
		y := oy + i*cell
		ink.DrawLine(image.Pt(grid.Min.X, y), image.Pt(grid.Max.X, y), ink.LightGray)
	}

	// Cathedral cross, top-left area.
	cross := [][2]int{{1, 0}, {0, 1}, {1, 1}, {2, 1}, {1, 2}}
	for _, c := range cross {
		ink.FillArea(pad(cellAt(c[0], c[1]), cell/6+1), ink.DarkGray)
	}

	// One small building, bottom-right area.
	ink.FillArea(pad(cellAt(4, 3), cell/8+1), ink.Black)

	// Dashed outline suggesting an enclosed pocket around the building.
	pocket := pad(image.Rect(cellAt(3, 2).Min.X, cellAt(3, 2).Min.Y, cellAt(4, 3).Max.X, cellAt(4, 3).Max.Y), cell/6)
	dash := cell / 6
	if dash < 4 {
		dash = 4
	}
	for x := pocket.Min.X; x < pocket.Max.X; x += dash * 2 {
		x2 := x + dash
		if x2 > pocket.Max.X {
			x2 = pocket.Max.X
		}
		ink.DrawLine(image.Pt(x, pocket.Min.Y), image.Pt(x2, pocket.Min.Y), ink.Black)
		ink.DrawLine(image.Pt(x, pocket.Max.Y), image.Pt(x2, pocket.Max.Y), ink.Black)
	}
	for y := pocket.Min.Y; y < pocket.Max.Y; y += dash * 2 {
		y2 := y + dash
		if y2 > pocket.Max.Y {
			y2 = pocket.Max.Y
		}
		ink.DrawLine(image.Pt(pocket.Min.X, y), image.Pt(pocket.Min.X, y2), ink.Black)
		ink.DrawLine(image.Pt(pocket.Max.X, y), image.Pt(pocket.Max.X, y2), ink.Black)
	}
}
