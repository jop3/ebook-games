package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"hertigen/game"
)

// Fonts held open for the app lifetime (opened once — never per draw).
type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Small  *ink.Font
	Menu   *ink.Font
	Tile   *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 34, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 32, true),
		Small:  ink.OpenFont(ink.DefaultFont, 26, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 38, true),
		Tile:   ink.OpenFont(ink.DefaultFontBold, 30, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Small, f.Menu, f.Tile} {
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

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func signInt(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	default:
		return 0
	}
}

// --- Layout ------------------------------------------------------------------

const (
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	statusBarHeight = 96
	buttonBarHeight = 150
	boardMargin     = 16
	topMargin       = 40
)

// Layout maps between screen pixels and the 6x6 board.
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

// tileAbbrev is the 2-letter code drawn inside a board tile square.
func tileAbbrev(t game.TileType) string {
	switch t {
	case game.Duke:
		return "HE"
	case game.Footman:
		return "FO"
	case game.Knight:
		return "RI"
	case game.Rider:
		return "RY"
	case game.DiagGuard:
		return "DV"
	case game.Catapult:
		return "KA"
	case game.Champion:
		return "KM"
	default:
		return "?"
	}
}

// drawTile renders one occupied square: a filled square for Black, a hollow
// (outlined) square for White — same "solid vs. hollow" convention the other
// games in this repo use to tell sides apart on greyscale e-ink — with the
// tile type's 2-letter code in the middle and a small corner mark whenever
// the tile is currently showing its FaceB pattern (no mark = FaceA).
func drawTile(cell image.Rectangle, t *game.Tile, f *Fonts) {
	r := pad(cell, cell.Dx()/10)
	if t.Side == game.Black {
		ink.FillArea(r, ink.Black)
		f.Tile.SetActive(ink.White)
	} else {
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		f.Tile.SetActive(ink.Black)
	}
	drawCenteredString(r, tileAbbrev(t.Type), 26)
	if t.Face == game.FaceB {
		m := r.Dx() / 6
		corner := image.Rect(r.Min.X, r.Min.Y, r.Min.X+m, r.Min.Y+m)
		if t.Side == game.Black {
			ink.FillArea(corner, ink.White)
		} else {
			ink.FillArea(corner, ink.Black)
		}
	}
}

// drawMoveHint marks a candidate relocate/recruit destination: a small
// filled dot, same convention as hasami's legal-destination hint.
func drawMoveHint(cell image.Rectangle) {
	cx, cy := (cell.Min.X+cell.Max.X)/2, (cell.Min.Y+cell.Max.Y)/2
	r := cell.Dx() / 9
	if r < 3 {
		r = 3
	}
	fillDisc(image.Rect(cx-r, cy-r, cx+r, cy+r), ink.LightGray)
}

// drawStrikeHint marks a candidate Strike target: the enemy tile sitting
// there is captured WITHOUT the striker relocating onto it, so it gets a
// distinct hollow double-ring instead of the plain move dot.
func drawStrikeHint(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/5)
	ink.DrawRect(r, ink.DarkGray)
	ink.DrawRect(pad(r, 3), ink.DarkGray)
}

// DrawBoard renders the grid, tiles, the current selection (if any) with its
// legal destinations/strikes (or, in recruit-square mode, the legal
// placement squares), and a brief marker over cells captured by the last
// action.
func DrawBoard(l *Layout, a *app) {
	s := a.gs
	ink.DrawRect(l.BoardArea, ink.Black)
	ink.DrawRect(pad(l.BoardArea, 1), ink.Black)
	for i := 1; i < game.Size; i++ {
		px := l.GridOrigin.X + i*l.CellSize
		ink.DrawLine(image.Pt(px, l.BoardArea.Min.Y), image.Pt(px, l.BoardArea.Max.Y), ink.Black)
		py := l.GridOrigin.Y + i*l.CellSize
		ink.DrawLine(image.Pt(l.BoardArea.Min.X, py), image.Pt(l.BoardArea.Max.X, py), ink.Black)
	}

	switch a.mode {
	case selTile:
		for _, act := range s.Board.ActionsFrom(a.selected) {
			cell := l.CellToScreen(act.To.X, act.To.Y)
			if act.Kind == game.ActStrike {
				drawStrikeHint(cell)
			} else {
				drawMoveHint(cell)
			}
		}
	case selRecruitSquare:
		for _, act := range s.Board.RecruitActions(s.Turn, s.Reserve[s.Turn]) {
			if act.Recruit != a.recruitType {
				continue
			}
			drawMoveHint(l.CellToScreen(act.To.X, act.To.Y))
		}
	}

	for y := 0; y < game.Size; y++ {
		for x := 0; x < game.Size; x++ {
			if t := s.Board.At(x, y); t != nil {
				drawTile(l.CellToScreen(x, y), t, a.fonts)
			}
		}
	}

	if a.mode != selNone {
		r := l.CellToScreen(a.selected.X, a.selected.Y)
		ink.DrawRect(pad(r, 2), ink.Black)
		ink.DrawRect(pad(r, 3), ink.Black)
	}

	// Unlike hasami's custodial captures, a Hertigen ActRelocate that
	// captures does so BY LANDING: the captured cell ends up occupied by
	// the mover, not empty (only a Strike leaves its target cell genuinely
	// empty). Only mark cells that are actually empty now, so the cross
	// never gets drawn on top of the tile that just landed there.
	for _, p := range s.LastCaptured {
		if s.Board.At(p.X, p.Y) == nil {
			drawCaptureMark(l.CellToScreen(p.X, p.Y))
		}
	}
}

// drawCaptureMark overlays a diagonal cross on a (now-empty) cell to briefly
// flag it as just-captured.
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

// DrawButtonBar renders one row of equally-sized buttons in l.ButtonBar and
// returns their tap rects. Used both for the steady-state ["Ny","Meny"] bar
// and for the dynamic recruit-flow bars (reserve type names, "Rekrytera",
// "Avbryt").
func DrawButtonBar(l *Layout, labels []string, font *ink.Font) []Button {
	ink.FillArea(l.ButtonBar, ink.White)
	ink.DrawLine(image.Pt(l.ButtonBar.Min.X, l.ButtonBar.Min.Y),
		image.Pt(l.ButtonBar.Max.X, l.ButtonBar.Min.Y), ink.Black)
	font.SetActive(ink.Black)
	n := len(labels)
	if n == 0 {
		return nil
	}
	gap := 16
	sideMargin := 20
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
		drawCenteredString(r, label, 32)
		buttons[i] = Button{Rect: r, Label: label}
	}
	return buttons
}

// drawNavButtons renders a row of buttons anchored to the bottom of the
// (Layout-less) rules/legend screens — the same visual language as
// DrawButtonBar, but positioned directly from the raw screen size since
// those screens have no board Layout.
func drawNavButtons(screen image.Point, labels []string, f *Fonts) []Button {
	H := usableH
	margin := 40
	bh := 110
	gap := 20
	n := len(labels)
	if n == 0 {
		return nil
	}
	usableW := screen.X - 2*margin
	totalGap := gap * (n + 1)
	bw := (usableW - totalGap) / n
	y0 := H - margin - bh
	f.Button.SetActive(ink.Black)
	out := make([]Button, n)
	for i, label := range labels {
		x0 := margin + gap + i*(bw+gap)
		r := image.Rect(x0, y0, x0+bw, y0+bh)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawCenteredString(r, label, 34)
		out[i] = Button{Rect: r, Label: label}
	}
	return out
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
	tw := ink.StringWidth("Hertigen")
	ink.DrawString(image.Pt((screen.X-tw)/2, 56), "Hertigen")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 30, true)
	sub.SetActive(ink.Black)
	subT := "Välj motståndare"
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
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb

	// Opponent rows fill the space between the subtitle and the Regler
	// button.
	rowH := 130
	top := 230
	bottom := rb.Min.Y - 30
	n := len(opponentChoices)
	avail := bottom - top
	if avail < rowH*n {
		rowH = avail / n
	}
	y := top

	f.Menu.SetActive(ink.Black)
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

// rulesParagraphs is the rules text for Hertigen, one entry per paragraph.
// The tile roster described here is this game's own original invention (see
// the design-note doc comment atop game/tiles.go) — not a transcription of
// the real The Duke's tile set, which the spec this game is built from
// explicitly flags as unverified from memory.
var rulesParagraphs = []string{
	"Mål: fånga motståndarens Hertig.",
	"Brädet är 6x6. Varje sida har en Hertig, en Fotknekt och en Kämpe på sin hemrad; Riddare, Ryttare, Diagonalvakt och Katapult börjar i reserven. Svart börjar.",
	"Varje bricka är dubbelsidig med ett eget tryckt rörelsemönster på vardera sidan. Vilket mönster som gäller styrs av vilken sida som råkar vara uppåt just nu.",
	"Mönstren: Flytt (glid dit, blockeras av allt i vägen, rutan måste vara tom), Hopp (hoppar dit rakt av, struntar i allt i vägen, tar ev. fiende genom att landa), Slag (slår bort en fiende på den rutan UTAN att själv flytta dit) och Flytt/Slag (glider dit, tomt eller med fiende att slå bort).",
	"Vändning: varje gång en bricka agerar vänds den till sin ANDRA sida efteråt, så nästa gång gäller det andra mönstret. Gäller även Hertigen.",
	"Rekrytera (istället för att flytta): placera en reservtrupp på en tom ruta intill din Hertigs nuvarande ruta. Det räknas som hela ditt drag.",
	"Tryck på en egen bricka för att välja den — lagliga mål markeras (prick=flytt, dubbel ram=slag). Tryck på ett mål för att utföra draget. Är Hertigen vald visas \"Rekrytera\": tryck den, välj trupptyp, tryck sedan en markerad ruta.",
	"Se \"Pjäslegend\" för en bildöversikt av alla sju brickornas båda mönster.",
	"Baserat på The Duke (Catalyst Game Labs) — vi lånar bara grundmekaniken, inte pjäsnamn, mönster eller uppställning.",
}

// DrawRules renders the (fixed, non-scrolling) rules text plus a small
// bottom nav bar ("Pjäslegend" / "Tillbaka") and returns the nav buttons.
func DrawRules(screen image.Point, f *Fonts, title string, paragraphs []string) []Button {
	ink.ClearScreen()
	H := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 52, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, 46), title)
	tf.Close()

	nav := drawNavButtons(screen, []string{"Pjäslegend", "Tillbaka"}, f)
	limit := nav[0].Rect.Min.Y - 20

	body := ink.OpenFont(ink.DefaultFont, 27, true)
	body.SetActive(ink.Black)
	bodyMargin := 46
	maxW := screen.X - 2*bodyMargin
	y := 128
	lineH := 36
	paraGap := 14
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

	_ = H
	return nav
}

// --- Legend (pattern reference) screen --------------------------------------

// legendTypes lists every tile type shown on the legend screen: the Duke
// first, then every troop type in the same stable order used elsewhere.
var legendTypes = append([]game.TileType{game.Duke}, game.TroopTypes...)

// kindLabel is the short code drawn at each printed offset in a pattern
// grid, matching the key printed at the top of the legend screen.
func kindLabel(k game.MoveKind) string {
	switch k {
	case game.MoveOnly:
		return "F"
	case game.Jump:
		return "H"
	case game.Strike:
		return "S"
	case game.MoveOrStrike:
		return "F/S"
	default:
		return "?"
	}
}

// drawPatternGrid draws one face's printed pattern as a small schematic: the
// tile itself in the center of a radius-2 grid, one line per straight
// (orthogonal/diagonal) direction reaching to the edge of the grid with the
// move-kind code at its tip (a ray's exact reach is capped visually at the
// grid edge — direction and kind are what a player needs to recognize, not
// the precise distance), and the kind code plotted directly at the (dx,dy)
// offset for non-straight entries (i.e. the Riddare's knight-shaped leaps).
// Which face (A/B) this is gets a single shared column header drawn once by
// the caller (DrawLegend) instead of a per-grid tag, so it never collides
// with a corner glyph.
func drawPatternGrid(box image.Rectangle, entries []game.PatternEntry, f *Fonts) {
	ink.DrawRect(box, ink.LightGray)
	const radius = 2
	n := radius*2 + 1
	cell := box.Dx() / n
	if box.Dy()/n < cell {
		cell = box.Dy() / n
	}
	center := image.Pt(box.Min.X+box.Dx()/2, box.Min.Y+box.Dy()/2)
	tb := image.Rect(center.X-cell/2, center.Y-cell/2, center.X+cell/2, center.Y+cell/2)
	ink.FillArea(tb, ink.Black)

	type dirKey struct{ x, y int }
	dirs := map[dirKey]game.MoveKind{}
	var extra []game.PatternEntry
	for _, e := range entries {
		if e.Dx == 0 && e.Dy == 0 {
			continue
		}
		straight := e.Dx == 0 || e.Dy == 0 || absInt(e.Dx) == absInt(e.Dy)
		if straight {
			dirs[dirKey{signInt(e.Dx), signInt(e.Dy)}] = e.Kind
		} else {
			extra = append(extra, e)
		}
	}
	for d, kind := range dirs {
		tip := image.Pt(center.X+d.x*cell*radius, center.Y+d.y*cell*radius)
		ink.DrawLine(center, tip, ink.Black)
		drawKindGlyph(tip, kind, f)
	}
	for _, e := range extra {
		dx, dy := e.Dx, e.Dy
		if dx > radius {
			dx = radius
		}
		if dx < -radius {
			dx = -radius
		}
		if dy > radius {
			dy = radius
		}
		if dy < -radius {
			dy = -radius
		}
		p := image.Pt(center.X+dx*cell, center.Y+dy*cell)
		drawKindGlyph(p, e.Kind, f)
	}
}

func drawKindGlyph(p image.Point, kind game.MoveKind, f *Fonts) {
	label := kindLabel(kind)
	f.Small.SetActive(ink.Black)
	w := ink.StringWidth(label)
	ink.DrawString(image.Pt(p.X-w/2, p.Y-13), label)
}

// DrawLegend renders every tile type's two printed faces side by side (Face
// A / Face B) so players don't have to memorize ~14 patterns from the rules
// text alone. Returns the nav buttons ("Tillbaka" back to the rules screen).
func DrawLegend(screen image.Point, f *Fonts) []Button {
	ink.ClearScreen()

	title := "Pjäslegend"
	tf := ink.OpenFont(ink.DefaultFontBold, 52, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, 36), title)
	tf.Close()

	capFont := ink.OpenFont(ink.DefaultFont, 25, true)
	capFont.SetActive(ink.DarkGray)
	capText := "F = Flytt   H = Hopp   S = Slag   F/S = Flytt eller slag"
	cw := ink.StringWidth(capText)
	ink.DrawString(image.Pt((screen.X-cw)/2, 96), capText)
	capFont.Close()

	nav := drawNavButtons(screen, []string{"Tillbaka"}, f)
	bottom := nav[0].Rect.Min.Y - 16

	margin := 36
	labelW := 230
	gridGap := 40
	top := 172 // leaves room for the "Sida A"/"Sida B" column header below the caption
	nTypes := len(legendTypes)
	rowH := (bottom - top) / nTypes
	gridSize := rowH - 16
	if gridSize > 130 {
		gridSize = 130
	}
	if gridSize < 40 {
		gridSize = 40
	}

	gridAX0 := margin + labelW
	gridBX0 := gridAX0 + gridSize + gridGap

	// Column headers, drawn once above the first row — naming which face
	// each column shows, instead of a per-grid tag that would collide with
	// a corner glyph on tile types whose pattern reaches every corner.
	hf := ink.OpenFont(ink.DefaultFont, 24, true)
	hf.SetActive(ink.Black)
	drawCenteredString(image.Rect(gridAX0, top-30, gridAX0+gridSize, top-2), "Sida A", 24)
	drawCenteredString(image.Rect(gridBX0, top-30, gridBX0+gridSize, top-2), "Sida B", 24)
	hf.Close()

	f.Small.SetActive(ink.Black)
	for i, t := range legendTypes {
		y := top + i*rowH
		rowRect := image.Rect(margin, y, screen.X-margin, y+rowH-8)

		labelR := image.Rect(rowRect.Min.X, rowRect.Min.Y, rowRect.Min.X+labelW, rowRect.Max.Y)
		drawLeftString(labelR, t.Name(), 28)

		gy := rowRect.Min.Y + (rowRect.Dy()-gridSize)/2
		gridA := image.Rect(gridAX0, gy, gridAX0+gridSize, gy+gridSize)
		gridB := image.Rect(gridBX0, gy, gridBX0+gridSize, gy+gridSize)
		drawPatternGrid(gridA, game.Patterns(t, game.FaceA), f)
		drawPatternGrid(gridB, game.Patterns(t, game.FaceB), f)

		ink.DrawLine(image.Pt(rowRect.Min.X, rowRect.Max.Y), image.Pt(rowRect.Max.X, rowRect.Max.Y), ink.LightGray)
	}

	return nav
}

// --- Splash screen -----------------------------------------------------------

// motifFunc draws the game's line-art centered in the given box.
type motifFunc func(box image.Rectangle)

// DrawSplash renders the start screen: the game title, a large line-art
// motif, and a "tap to start" hint.
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

// drawSplashMotif draws a single Duke tile with its move-pattern arrows
// overlaid, like a legend entry: solid lines to the 4 orthogonal neighbours
// (Face A's pattern) and dashed lines to the 4 diagonal neighbours (Face
// B's pattern) — both drawn together to show off the flip mechanic that
// alternates the Duke between the two at a glance.
func drawSplashMotif(box image.Rectangle) {
	center := image.Pt((box.Min.X+box.Max.X)/2, (box.Min.Y+box.Max.Y)/2)
	tileR := box.Dx() / 10
	tb := image.Rect(center.X-tileR, center.Y-tileR, center.X+tileR, center.Y+tileR)
	ink.FillArea(tb, ink.Black)

	lf := ink.OpenFont(ink.DefaultFontBold, tileR, true)
	lf.SetActive(ink.White)
	label := "H"
	lw := ink.StringWidth(label)
	ink.DrawString(image.Pt(center.X-lw/2, center.Y-tileR*3/5), label)
	lf.Close()

	reach := box.Dx() * 2 / 5
	orthoDirs := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	diagDirs := [4][2]int{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
	for _, d := range orthoDirs {
		drawArrow(center, d, reach, false)
	}
	for _, d := range diagDirs {
		drawArrow(center, d, reach, true)
	}
}

// drawArrow draws a line (solid, or dashed if dashed is true) from center
// out to distance reach in direction d, capped with a small arrowhead.
func drawArrow(center image.Point, d [2]int, reach int, dashed bool) {
	tip := image.Pt(center.X+d[0]*reach, center.Y+d[1]*reach)
	if dashed {
		const steps = 8
		for i := 0; i < steps; i += 2 {
			a := lerpPt(center, tip, float64(i)/float64(steps))
			b := lerpPt(center, tip, float64(i+1)/float64(steps))
			ink.DrawLine(a, b, ink.Black)
		}
	} else {
		ink.DrawLine(center, tip, ink.Black)
	}
	drawArrowHead(tip, d, reach/8)
}

func lerpPt(a, b image.Point, t float64) image.Point {
	return image.Pt(a.X+int(float64(b.X-a.X)*t), a.Y+int(float64(b.Y-a.Y)*t))
}

// drawArrowHead draws a small chevron at tip pointing along d (whose
// components are each -1, 0 or 1 — an axis or diagonal unit direction).
func drawArrowHead(tip image.Point, d [2]int, size int) {
	if size < 6 {
		size = 6
	}
	back := image.Pt(tip.X-d[0]*size, tip.Y-d[1]*size)
	perp := [2]int{-d[1], d[0]}
	tailA := image.Pt(back.X+perp[0]*size/2, back.Y+perp[1]*size/2)
	tailB := image.Pt(back.X-perp[0]*size/2, back.Y-perp[1]*size/2)
	ink.DrawLine(tip, tailA, ink.Black)
	ink.DrawLine(tip, tailB, ink.Black)
}
