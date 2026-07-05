package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"mosaik/game"
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
		Button: ink.OpenFont(ink.DefaultFontBold, 34, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 38, true),
		Small:  ink.OpenFont(ink.DefaultFont, 26, true),
		Tiny:   ink.OpenFont(ink.DefaultFont, 22, true),
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

// --- Layout ------------------------------------------------------------

const (
	usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

	topMargin  = 14
	sideMargin = 20
	gapSm      = 8

	statusBarHeight = 60
	factoriesHeight = 168
	opponentHeight  = 140
	buttonBarHeight = 110
)

// faintTone/faintBorder render the "ghost" hints — an empty wall cell's
// target pattern, and an empty pattern-line slot's outline — deliberately
// much lighter than any real (placed) tile so the two are never confused at
// a glance.
var faintTone = color.Gray{Y: 0xb5}
var faintBorder = color.Gray{Y: 0xb5}
var highlightTone = color.Gray{Y: 0xe2}

// sourceHit is a tappable (factory-tile or center-chip) rect, tagged with
// the exact (source, color) Move fields tapping it selects.
type sourceHit struct {
	rect   image.Rectangle
	source int
	color  game.Color
}

// targetHit is a tappable pattern-line-row (or floor) rect on the currently
// full-size board, tagged with the Move.TargetLine it commits to.
type targetHit struct {
	rect   image.Rectangle
	target int
}

// Layout holds every screen region for Mosaik's drafting screen. Unlike a
// single board-grid game, there's no one cell mapping — factories, the
// active board's lines+wall+floor, and the compact opponent strip are each
// their own region, rebuilt every draw from the current screen size (the
// static bands here) and the current game state (the dynamic hit-test
// slices, populated by DrawFactories/DrawActiveBoard/DrawOpponentStrip).
type Layout struct {
	Screen        image.Rectangle
	StatusBar     image.Rectangle
	FactoriesArea image.Rectangle
	ActiveArea    image.Rectangle
	OpponentArea  image.Rectangle
	ButtonBar     image.Rectangle

	ShowOppBtn image.Rectangle

	sourceHits []sourceHit
	targetHits []targetHit
}

func NewLayout(screen image.Point) Layout {
	H := usableH
	l := Layout{Screen: image.Rect(0, 0, screen.X, H)}
	y := topMargin
	l.StatusBar = image.Rect(0, y, screen.X, y+statusBarHeight)
	y = l.StatusBar.Max.Y + gapSm
	l.FactoriesArea = image.Rect(0, y, screen.X, y+factoriesHeight)
	y = l.FactoriesArea.Max.Y + gapSm

	l.ButtonBar = image.Rect(0, H-topMargin-buttonBarHeight, screen.X, H-topMargin)
	l.OpponentArea = image.Rect(0, l.ButtonBar.Min.Y-gapSm-opponentHeight, screen.X, l.ButtonBar.Min.Y-gapSm)
	l.ActiveArea = image.Rect(0, y, screen.X, l.OpponentArea.Min.Y-gapSm)
	return l
}

// HitSource finds the (source, color) a tap at p would select — a specific
// tile swatch in a factory, or a specific color chip in the center.
func (l *Layout) HitSource(p image.Point) (source int, c game.Color, ok bool) {
	for _, h := range l.sourceHits {
		if p.In(h.rect) {
			return h.source, h.color, true
		}
	}
	return 0, 0, false
}

// HitTarget finds the pattern-line row (or the floor, -1) a tap at p would
// commit the currently-selected tiles to, on the full-size board.
func (l *Layout) HitTarget(p image.Point) (target int, ok bool) {
	for _, h := range l.targetHits {
		if p.In(h.rect) {
			return h.target, true
		}
	}
	return 0, false
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
		drawCenteredString(r, label, 34)
		buttons[i] = Button{Rect: r, Label: label}
	}
	return buttons
}

// --- Tile pattern art ------------------------------------------------------
//
// 5 distinct greyscale patterns, one per game.Color, built from different
// silhouette families (solid fill, ring, crossed hatch, one-way diagonal
// stripe, dot grid) so no two are confusable at chip size. Each has a
// "faint" (light-gray) variant used for the wall's empty-cell target hints.

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

// drawDiagRun fills box with a repeating one-pixel-wide diagonal band
// pattern via per-row scanlines (never drawing outside box, so it needs no
// clipping support from the underlying line-drawing primitive): period is
// the stripe spacing and thickness how much of each period is "on".
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

// drawPattern renders game.Color c's pattern inside box, in black (faint =
// false, a real placed tile) or in a light gray tone (faint = true, an
// empty wall cell's target hint).
func drawPattern(box image.Rectangle, c game.Color, faint bool) {
	ink.FillArea(box, ink.White)
	col := color.Color(ink.Black)
	if faint {
		col = faintTone
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
		drawDiagRun(box, col, 14, 4, false)
		drawDiagRun(box, col, 14, 4, true)
	case game.ColorStripe:
		drawDiagRun(box, col, 22, 10, false)
	case game.ColorDot:
		drawDotsGrid(box, col)
	}
}

// colorName is the short Swedish label for color c, used on the round-end
// tiling breakdown (numbers alone aren't enough there — the reader needs to
// know WHICH pattern scored).
func colorName(c game.Color) string {
	switch c {
	case game.ColorSolid:
		return "Fyllda"
	case game.ColorRing:
		return "Ring"
	case game.ColorCross:
		return "Rutig"
	case game.ColorStripe:
		return "Randig"
	case game.ColorDot:
		return "Prickig"
	}
	return "?"
}

// drawTileChip draws a real, placed/available tile: its pattern plus a
// black border (doubled/tripled if selected, matching the other games' "in
// hand"/selection emphasis convention).
func drawTileChip(r image.Rectangle, c game.Color, selected bool) {
	drawPattern(pad(r, 3), c, false)
	ink.DrawRect(r, ink.Black)
	if selected {
		ink.DrawRect(pad(r, 2), ink.Black)
		ink.DrawRect(pad(r, 3), ink.Black)
	}
}

// drawWallCell draws one wall cell: its real pattern + solid black border if
// filled, or a faint target-pattern hint + faint border if still empty.
func drawWallCell(r image.Rectangle, filled bool, c game.Color) {
	if filled {
		drawTileChip(r, c, false)
		return
	}
	drawPattern(pad(r, 3), c, true)
	ink.DrawRect(r, faintBorder)
}

func drawGridLines(origin image.Point, cellSize, n int) {
	for i := 0; i <= n; i++ {
		x := origin.X + i*cellSize
		ink.DrawLine(image.Pt(x, origin.Y), image.Pt(x, origin.Y+n*cellSize), faintBorder)
		y := origin.Y + i*cellSize
		ink.DrawLine(image.Pt(origin.X, y), image.Pt(origin.X+n*cellSize, y), faintBorder)
	}
}

// --- Small game-state helpers (kept in ui.go: purely about presentation,
// not rules) --------------------------------------------------------------

func distinctColors(tiles []game.Color) []game.Color {
	var seen [game.NumColors]bool
	for _, t := range tiles {
		seen[t] = true
	}
	var out []game.Color
	for c := game.Color(0); c < game.NumColors; c++ {
		if seen[c] {
			out = append(out, c)
		}
	}
	return out
}

func countColorUI(tiles []game.Color, c game.Color) int {
	n := 0
	for _, t := range tiles {
		if t == c {
			n++
		}
	}
	return n
}

func lineColorOf(b *game.Board, i int) (game.Color, bool) {
	if len(b.Lines[i]) == 0 {
		return 0, false
	}
	return b.Lines[i][0], true
}

func activeLineCount(b *game.Board) int {
	n := 0
	for i := range b.Lines {
		if len(b.Lines[i]) > 0 {
			n++
		}
	}
	return n
}

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

// boardHeaderLabel is the possessive form of playerLabel used above the
// full-size board ("Din bricka", not the grammatically broken "Dus
// bricka" a naive playerLabel+"s bricka" concatenation would produce for
// "Du").
func boardHeaderLabel(gs *game.GameState, side int) string {
	if gs.Mode == game.ModeAI {
		if side == 0 {
			return "Din bricka"
		}
		return "Datorns bricka"
	}
	return playerLabel(gs, side) + " – bricka"
}

// selectionCount is how many tiles of the selected color sit in the
// selected source right now — used for the "N i handen" status line.
func selectionCount(gs *game.GameState, sel selection) int {
	var tiles []game.Color
	if sel.source == -1 {
		tiles = gs.Center
	} else {
		tiles = gs.Factories[sel.source]
	}
	return countColorUI(tiles, sel.color)
}

// --- Status bar ------------------------------------------------------------

func DrawStatus(l *Layout, gs *game.GameState, f *Fonts, msg string) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	text := msg
	if text == "" {
		turn := playerLabel(gs, gs.Turn)
		if gs.AITurn() {
			turn += " (dator)"
		}
		text = "Runda " + itoa(gs.RoundNum+1) + "   ·   " + turn + " drar   ·   " +
			playerLabel(gs, 0) + " " + itoa(gs.Boards[0].Score) + " – " +
			playerLabel(gs, 1) + " " + itoa(gs.Boards[1].Score)
	}
	drawCenteredString(l.StatusBar, text, 30)
	ink.DrawLine(image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y), image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y), ink.Black)
}

// --- Factories + center ------------------------------------------------------

// DrawFactories renders the 5 factory displays (each up to 4 tiles in a 2x2
// grid) and the center pool (one chip per distinct color present, plus a
// small marker glyph while the start marker is still unclaimed), and
// records every tappable tile/chip into l.sourceHits.
func DrawFactories(l *Layout, gs *game.GameState, sel selection) {
	area := pad(l.FactoriesArea, sideMargin/2)
	ink.FillArea(l.FactoriesArea, ink.White)
	l.sourceHits = l.sourceHits[:0]

	n := game.NumFactories + 1 // + center
	gap := 10
	boxW := (area.Dx() - (n-1)*gap) / n
	x := area.Min.X
	for fi := 0; fi < game.NumFactories; fi++ {
		box := image.Rect(x, area.Min.Y, x+boxW, area.Max.Y)
		drawFactoryBox(l, box, fi, gs.Factories[fi], sel)
		x += boxW + gap
	}
	centerBox := image.Rect(x, area.Min.Y, area.Max.X, area.Max.Y)
	drawCenterBox(l, centerBox, gs, sel)
}

func drawFactoryBox(l *Layout, box image.Rectangle, fi int, tiles []game.Color, sel selection) {
	ink.DrawRect(box, ink.Black)
	inner := pad(box, 8)
	gap := 5
	cw := (inner.Dx() - gap) / 2
	ch := (inner.Dy() - gap) / 2
	for i := 0; i < 4; i++ {
		col, row := i%2, i/2
		r := image.Rect(
			inner.Min.X+col*(cw+gap), inner.Min.Y+row*(ch+gap),
			inner.Min.X+col*(cw+gap)+cw, inner.Min.Y+row*(ch+gap)+ch)
		if i >= len(tiles) {
			continue
		}
		c := tiles[i]
		selected := sel.active && sel.source == fi && sel.color == c
		drawTileChip(r, c, selected)
		l.sourceHits = append(l.sourceHits, sourceHit{rect: r, source: fi, color: c})
	}
}

// drawCenterBox renders the center pool as one chip per distinct color
// present (with an "xN" count), plus the start marker if unclaimed — this
// scales cleanly regardless of how many tiles have piled up (unlike a
// literal one-chip-per-tile grid, which would get unreadable fast since the
// center can hold far more than a factory's fixed 4).
func drawCenterBox(l *Layout, box image.Rectangle, gs *game.GameState, sel selection) {
	ink.DrawRect(box, ink.Black)
	inner := pad(box, 8)
	present := distinctColors(gs.Center)
	items := len(present)
	if gs.CenterHasStart {
		items++
	}
	if items == 0 {
		f := ink.OpenFont(ink.DefaultFont, 22, true)
		f.SetActive(ink.DarkGray)
		drawCenteredString(box, "Mitten", 22)
		f.Close()
		return
	}
	cols := items
	if cols > 3 {
		cols = 3
	}
	rows := (items + cols - 1) / cols
	gap := 5
	cw := (inner.Dx() - (cols-1)*gap) / cols
	ch := (inner.Dy() - (rows-1)*gap) / rows

	idx := 0
	cellRect := func() image.Rectangle {
		c, r := idx%cols, idx/cols
		idx++
		return image.Rect(
			inner.Min.X+c*(cw+gap), inner.Min.Y+r*(ch+gap),
			inner.Min.X+c*(cw+gap)+cw, inner.Min.Y+r*(ch+gap)+ch)
	}

	if gs.CenterHasStart {
		drawMarkerChip(cellRect())
	}
	for _, c := range present {
		r := cellRect()
		selected := sel.active && sel.source == -1 && sel.color == c
		drawTileChip(r, c, selected)
		drawCountBadge(r, countColorUI(gs.Center, c))
		l.sourceHits = append(l.sourceHits, sourceHit{rect: r, source: -1, color: c})
	}
}

// drawMarkerChip draws the start-player marker: a small filled triangle,
// visually unlike any of the 5 tile patterns so it's never mistaken for a
// drafting choice (it isn't one — it's claimed automatically).
func drawMarkerChip(r image.Rectangle) {
	ink.DrawRect(r, faintBorder)
	cx, _ := centerOf(r)
	top := r.Min.Y + r.Dy()/4
	bot := r.Max.Y - r.Dy()/4
	halfW := r.Dx() / 4
	for y := top; y <= bot; y++ {
		w := halfW * (y - top) / (bot - top)
		ink.DrawLine(image.Pt(cx-w, y), image.Pt(cx+w, y), ink.Black)
	}
}

// drawCountBadge draws a small "xN" label in the bottom-right corner of a
// center chip.
func drawCountBadge(r image.Rectangle, n int) {
	f := ink.OpenFont(ink.DefaultFontBold, 20, true)
	f.SetActive(ink.Black)
	s := "x" + itoa(n)
	w := ink.StringWidth(s)
	ink.FillArea(image.Rect(r.Max.X-w-6, r.Max.Y-22, r.Max.X-1, r.Max.Y-1), ink.White)
	ink.DrawString(image.Pt(r.Max.X-w-4, r.Max.Y-22), s)
	f.Close()
}

// --- Active board (full-size): pattern lines + wall + floor -----------------

const boardCellSize = 96

// DrawActiveBoard renders side's board full-size: pattern lines on the
// left (right-aligned so the wall-adjacent end is always flush against the
// wall, per row), the 5x5 wall on the right (empty cells showing a faint
// hint of their fixed target pattern), and the floor line spanning the full
// width below. Records every tappable row (and the floor) into
// l.targetHits.
func DrawActiveBoard(l *Layout, gs *game.GameState, side int, f *Fonts, legalTargets map[int]bool, sel selection) {
	area := l.ActiveArea
	ink.FillArea(area, ink.White)
	b := &gs.Boards[side]
	l.targetHits = l.targetHits[:0]

	// The board (header + lines/wall + floor) is centered as ONE block
	// within the available area, rather than anchored to its top — cell
	// size is generous enough that the content doesn't fill the whole
	// band, and centering spreads the leftover space evenly above and
	// below instead of leaving it all stranded beneath the floor line.
	headerH := 40
	headerGap := 8
	cell := boardCellSize
	linesW := cell * game.WallSize
	wallW := cell * game.WallSize
	gapLW := 22
	blockW := linesW + gapLW + wallW
	blockH := cell * game.WallSize
	floorGap := 14
	floorH := 84
	contentH := headerH + headerGap + blockH + floorGap + floorH
	top := area.Min.Y + (area.Dy()-contentH)/2
	if top < area.Min.Y {
		top = area.Min.Y
	}

	header := image.Rect(area.Min.X, top, area.Max.X, top+headerH)
	f.Menu.SetActive(ink.Black)
	label := boardHeaderLabel(gs, side) + "   ·   Poäng " + itoa(b.Score)
	drawCenteredString(header, label, 32)

	blockX := area.Min.X + (area.Dx()-blockW)/2
	blockY := header.Max.Y + headerGap

	linesOrigin := image.Pt(blockX, blockY)
	wallOrigin := image.Pt(blockX+linesW+gapLW, blockY)

	// Pattern lines.
	for i := 0; i < game.WallSize; i++ {
		rowY := linesOrigin.Y + i*cell
		firstCol := game.WallSize - 1 - i
		filledCount := len(b.Lines[i])
		rowRect := image.Rect(linesOrigin.X+firstCol*cell, rowY, linesOrigin.X+linesW, rowY+cell)
		if sel.active && legalTargets[i] {
			ink.FillArea(pad(rowRect, 1), highlightTone)
		}
		c, hasColor := lineColorOf(b, i)
		for s := 0; s <= i; s++ {
			col := firstCol + s
			r := pad(image.Rect(linesOrigin.X+col*cell, rowY, linesOrigin.X+(col+1)*cell, rowY+cell), 4)
			filled := hasColor && s >= (i+1-filledCount)
			if filled {
				drawTileChip(r, c, false)
			} else {
				ink.FillArea(r, ink.White)
				ink.DrawRect(r, ink.Black)
			}
		}
		l.targetHits = append(l.targetHits, targetHit{rect: rowRect, target: i})
	}

	// Wall.
	for r := 0; r < game.WallSize; r++ {
		for c := 0; c < game.WallSize; c++ {
			cellRect := pad(image.Rect(
				wallOrigin.X+c*cell, wallOrigin.Y+r*cell,
				wallOrigin.X+(c+1)*cell, wallOrigin.Y+(r+1)*cell), 4)
			drawWallCell(cellRect, b.Wall[r][c], game.ColorAt(r, c))
		}
	}
	drawGridLines(wallOrigin, cell, game.WallSize)

	// Floor line, spanning the full lines+wall block width.
	floorY := blockY + blockH + floorGap
	floorRect := image.Rect(blockX, floorY, blockX+blockW, floorY+floorH)
	if sel.active && legalTargets[-1] {
		ink.FillArea(pad(floorRect, 1), highlightTone)
	}
	drawFloor(floorRect, b)
	l.targetHits = append(l.targetHits, targetHit{rect: floorRect, target: -1})
}

// floorTrackLabels mirrors game's floor penalty track for the UI's static
// per-slot caption.
var floorTrackLabels = [game.FloorSlots]string{"-1", "-1", "-2", "-2", "-2", "-3", "-3"}

// drawFloor renders the 7-slot floor line: the marker (if present) and each
// floor tile fill the leftmost slots in order; every slot shows its fixed
// penalty value underneath.
func drawFloor(area image.Rectangle, b *game.Board) {
	f := ink.OpenFont(ink.DefaultFont, 20, true)
	defer f.Close()
	f.SetActive(ink.DarkGray)
	labelR := image.Rect(area.Min.X, area.Min.Y, area.Min.X+70, area.Max.Y)
	drawLeftString(labelR, "Golv", 22)

	slotsArea := image.Rect(labelR.Max.X+6, area.Min.Y, area.Max.X, area.Max.Y)
	gap := 5
	slotW := (slotsArea.Dx() - (game.FloorSlots-1)*gap) / game.FloorSlots
	tileH := slotsArea.Dy() * 62 / 100
	// Tile chips must stay SQUARE (never stretched into a wide rectangle):
	// drawPattern's circular/dot glyphs assume a roughly square box, and a
	// squashed box would render the Ring pattern as a clipped oval instead
	// of a circle. Center a square of side min(slotW, tileH) in each slot.
	chipSize := slotW
	if tileH < chipSize {
		chipSize = tileH
	}

	items := make([]*game.Color, 0, game.FloorSlots)
	if b.HasMarker {
		items = append(items, nil) // nil marks the start-marker slot
	}
	for i := range b.Floor {
		c := b.Floor[i]
		items = append(items, &c)
	}

	for i := 0; i < game.FloorSlots; i++ {
		x0 := slotsArea.Min.X + i*(slotW+gap)
		cx0 := x0 + (slotW-chipSize)/2
		tileRect := image.Rect(cx0, slotsArea.Min.Y, cx0+chipSize, slotsArea.Min.Y+chipSize)
		if i < len(items) {
			if items[i] == nil {
				drawMarkerChip(tileRect)
			} else {
				drawTileChip(tileRect, *items[i], false)
			}
		} else {
			ink.DrawRect(tileRect, faintBorder)
		}
		labelRect := image.Rect(x0, tileRect.Max.Y, x0+slotW, slotsArea.Max.Y)
		f.SetActive(ink.DarkGray)
		drawCenteredString(labelRect, floorTrackLabels[i], 20)
	}
}

// --- Opponent compact strip --------------------------------------------------

// DrawOpponentStrip renders the OTHER board (side) as a compact mini-wall +
// score/line summary, with the "Visa motståndare"/"Mitt bräde" toggle
// button. Records the toggle button rect into l.ShowOppBtn.
func DrawOpponentStrip(l *Layout, gs *game.GameState, side int, showOpp bool, f *Fonts) {
	area := l.OpponentArea
	ink.FillArea(area, ink.White)
	ink.DrawLine(image.Pt(area.Min.X, area.Min.Y), image.Pt(area.Max.X, area.Min.Y), ink.Black)
	b := &gs.Boards[side]

	miniCell := 15
	miniSize := miniCell * game.WallSize
	miniOrigin := image.Pt(area.Min.X+sideMargin, area.Min.Y+(area.Dy()-miniSize)/2)
	for r := 0; r < game.WallSize; r++ {
		for c := 0; c < game.WallSize; c++ {
			cellRect := image.Rect(
				miniOrigin.X+c*miniCell, miniOrigin.Y+r*miniCell,
				miniOrigin.X+(c+1)*miniCell, miniOrigin.Y+(r+1)*miniCell)
			if b.Wall[r][c] {
				ink.FillArea(pad(cellRect, 1), ink.Black)
			} else {
				ink.DrawRect(cellRect, faintBorder)
			}
		}
	}

	btnW, btnH := 280, 68
	btn := image.Rect(area.Max.X-btnW-sideMargin, area.Min.Y+(area.Dy()-btnH)/2,
		area.Max.X-sideMargin, area.Min.Y+(area.Dy()-btnH)/2+btnH)
	ink.DrawRect(btn, ink.Black)
	ink.DrawRect(pad(btn, 1), ink.Black)
	label := "Visa motståndare"
	if showOpp {
		label = "Mitt bräde"
	}
	// A dedicated, slightly smaller bold font for this specific button: at
	// the normal Button size, "Visa motståndare" is wider than any button
	// used elsewhere in this game and would overflow its box.
	toggleFont := ink.OpenFont(ink.DefaultFontBold, 26, true)
	toggleFont.SetActive(ink.Black)
	drawCenteredString(btn, label, 26)
	toggleFont.Close()
	l.ShowOppBtn = btn

	textArea := image.Rect(miniOrigin.X+miniSize+20, area.Min.Y, btn.Min.X-16, area.Max.Y)
	f.Small.SetActive(ink.Black)
	nameLine := playerLabel(gs, side) + "   ·   Poäng " + itoa(b.Score)
	drawLeftString(image.Rect(textArea.Min.X, area.Min.Y+16, textArea.Max.X, area.Min.Y+46), nameLine, 26)
	floorInfo := "Golv: " + itoa(len(b.Floor))
	if b.HasMarker {
		floorInfo += " + markör"
	}
	infoLine := "Rader påbörjade: " + itoa(activeLineCount(b)) + "/5   ·   " + floorInfo
	f.Tiny.SetActive(ink.Black)
	drawLeftString(image.Rect(textArea.Min.X, area.Min.Y+50, textArea.Max.X, area.Min.Y+78), infoLine, 22)
}

// --- Round-end wall-tiling screen -------------------------------------------

func DrawTiling(l *Layout, gs *game.GameState, f *Fonts) []Button {
	screen := l.Screen
	f.Title.SetActive(ink.Black)
	title := "Runda " + itoa(gs.RoundNum+1) + " — plattor på väggen"
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.Dx()-tw)/2, 40), title)

	colW := screen.Dx() / 2
	drawTilingColumn(image.Rect(sideMargin, 130, colW-10, usableH-200), gs, 0, f)
	drawTilingColumn(image.Rect(colW+10, 130, screen.Dx()-sideMargin, usableH-200), gs, 1, f)

	return DrawButtonBar(l, []string{"Fortsätt"}, f)
}

func drawTilingColumn(area image.Rectangle, gs *game.GameState, side int, f *Fonts) {
	tr := gs.LastTiling[side]
	f.Menu.SetActive(ink.Black)
	drawLeftString(image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Min.Y+40), playerLabel(gs, side), 32)
	y := area.Min.Y + 52

	f.Small.SetActive(ink.Black)
	if len(tr.Placements) == 0 {
		drawLeftString(image.Rect(area.Min.X, y, area.Max.X, y+30), "(inga fullbordade rader)", 26)
		y += 36
	}
	for _, p := range tr.Placements {
		line := colorName(p.Color) + ", rad " + itoa(p.Row+1) + ": +" + itoa(p.Points) + "p"
		drawLeftString(image.Rect(area.Min.X, y, area.Max.X, y+30), line, 26)
		y += 36
	}
	if tr.FloorPenalty > 0 {
		drawLeftString(image.Rect(area.Min.X, y, area.Max.X, y+30), "Golvrad: -"+itoa(tr.FloorPenalty)+"p", 26)
		y += 36
	}
	y += 14
	f.Status.SetActive(ink.Black)
	drawLeftString(image.Rect(area.Min.X, y, area.Max.X, y+38), "Poäng: "+itoa(tr.ScoreBefore)+" → "+itoa(tr.ScoreAfter), 32)
}

// --- End-game bonus screen ---------------------------------------------------

func DrawBonus(l *Layout, gs *game.GameState, f *Fonts) []Button {
	screen := l.Screen
	f.Title.SetActive(ink.Black)
	title := "Spelet slut — bonuspoäng"
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.Dx()-tw)/2, 40), title)

	colW := screen.Dx() / 2
	drawBonusColumn(image.Rect(sideMargin, 130, colW-10, usableH-200), gs, 0, f)
	drawBonusColumn(image.Rect(colW+10, 130, screen.Dx()-sideMargin, usableH-200), gs, 1, f)

	return DrawButtonBar(l, []string{"Fortsätt"}, f)
}

func drawBonusColumn(area image.Rectangle, gs *game.GameState, side int, f *Fonts) {
	bo := gs.Bonuses[side]
	f.Menu.SetActive(ink.Black)
	drawLeftString(image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Min.Y+40), playerLabel(gs, side), 32)
	y := area.Min.Y + 52

	f.Small.SetActive(ink.Black)
	lines := []string{
		"Fullbordade rader: " + itoa(bo.Rows) + " x2 = " + itoa(bo.Rows*2),
		"Fullbordade kolumner: " + itoa(bo.Cols) + " x7 = " + itoa(bo.Cols*7),
		"Fullbordade färger: " + itoa(bo.Colors) + " x10 = " + itoa(bo.Colors*10),
	}
	for _, ln := range lines {
		drawLeftString(image.Rect(area.Min.X, y, area.Max.X, y+30), ln, 26)
		y += 36
	}
	y += 14
	f.Status.SetActive(ink.Black)
	drawLeftString(image.Rect(area.Min.X, y, area.Max.X, y+38), "Bonus: +"+itoa(bo.Total), 32)
	y += 46
	drawLeftString(image.Rect(area.Min.X, y, area.Max.X, y+42), "Slutpoäng: "+itoa(gs.Boards[side].Score), 36)
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

func DrawWinner(l *Layout, gs *game.GameState, f *Fonts) []Button {
	screen := l.Screen

	f.Title.SetActive(ink.Black)
	text := winnerText(gs)
	tw := ink.StringWidth(text)
	ink.DrawString(image.Pt((screen.Dx()-tw)/2, 100), text)

	f.Menu.SetActive(ink.Black)
	scoreLine := playerLabel(gs, 0) + ": " + itoa(gs.Boards[0].Score) + "     " +
		playerLabel(gs, 1) + ": " + itoa(gs.Boards[1].Score)
	sw := ink.StringWidth(scoreLine)
	ink.DrawString(image.Pt((screen.Dx()-sw)/2, 230), scoreLine)

	return DrawButtonBar(l, []string{"Ny", "Meny"}, f)
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

type Menu struct {
	rows     []menuRow
	rulesBtn image.Rectangle
}

func NewMenu() *Menu { return &Menu{} }

var choices = []menuChoice{
	{game.ModeHotseat, 0, "2 spelare (hot-seat)"},
	{game.ModeAI, game.DepthEasy, "Mot dator – Lätt"},
	{game.ModeAI, game.DepthMedium, "Mot dator – Medel"},
	{game.ModeAI, game.DepthHard, "Mot dator – Svår"},
}

func (m *Menu) Draw(screen image.Point, f *Fonts) {
	ink.ClearScreen()
	H := usableH

	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Mosaik")
	ink.DrawString(image.Pt((screen.X-tw)/2, 70), "Mosaik")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 34, true)
	sub.SetActive(ink.Black)
	subT := "Välj spelläge"
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

var rulesParagraphs = []string{
	"Mål: flest poäng när spelet slutar.",
	"2 spelare. 5 fabriker fylls med 4 slumpade plattor var (5 mönster istället för färger); resten ligger i en mitten-hög.",
	"Din tur: ta ALLA plattor av ETT mönster från EN fabrik (resten till mitten), eller alla av ett mönster från mitten. Den som FÖRST tar från mitten en runda får startmarkören på sin golvrad (minuspoäng) men börjar nästa runda.",
	"Lägg plattorna på EN av dina 5 mönsterrader (rad i rymmer i+1 st, bara ETT mönster per rad, bara om väggrutan för det mönstret på den raden är tom). Det som inte får plats går till golvraden — även sådant du väljer att lägga där direkt.",
	"Tomma fabriker+mitten: varje FULLBORDAD rad flyttar en platta till sin fasta väggplats och ger poäng (resten kastas); ofullbordade rader väntar.",
	"Poäng: ensam platta (inga grannar) = 1p. Annars den sammanhängande VÅGRÄTA radens längd plus den LODRÄTA radens längd (båda med den nya plattan) — riktning utan granne ger 0, inte 1.",
	"Golvraden, 7 platser: -1,-1,-2,-2,-2,-3,-3 (poäng kan aldrig bli under 0). Töms varje runda.",
	"Spelet slutar när en runda med en FULLBORDAD vågrät rad har lagts klart (inte mitt i rundan). Bonus: +2/rad, +7/kolumn, +10/mönster helt på väggen. Mest poäng vinner; lika = flest fullbordade rader, annars delad seger.",
	"Tryck på en platta/mönster för att välja, tryck sedan på en rad (eller golvraden) för att lägga.",
	"\"Visa motståndare\" visar andra bordet i fullstorlek — öppen information, inget döljs.",
	"Baserat på Azul (Michael Kiesling, Next Move/Plan B Games).",
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

	body := ink.OpenFont(ink.DefaultFont, 25, true)
	body.SetActive(ink.Black)
	bodyMargin := 40
	maxW := screen.X - 2*bodyMargin
	y := 130
	lineH := 33
	paraGap := 11
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

	// Matches the other games' splash proportions (side = 3/5 of the
	// screen width, vertically centered with no extra shift) — a bigger
	// box or an added vertical shift here previously overlapped the title
	// text, silently painting over it.
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

// drawSplashMotif shows a 5x5 wall, partly filled with the 5 distinct tile
// patterns, with one complete row highlighted (a thick frame) — echoing the
// spec's chosen splash image: "a wall partly filled ... one complete
// scoring row highlighted".
func drawSplashMotif(box image.Rectangle) {
	cell := box.Dx() / game.WallSize
	if box.Dy()/game.WallSize < cell {
		cell = box.Dy() / game.WallSize
	}
	size := cell * game.WallSize
	origin := image.Pt(box.Min.X+(box.Dx()-size)/2, box.Min.Y+(box.Dy()-size)/2)

	// A representative partial fill: all of row 0 (the highlighted, complete
	// scoring row), plus a few scattered tiles elsewhere so the empty cells'
	// faint hints are visible too.
	filled := map[[2]int]bool{
		{0, 0}: true, {0, 1}: true, {0, 2}: true, {0, 3}: true, {0, 4}: true,
		{1, 2}: true, {2, 0}: true, {2, 1}: true, {3, 3}: true,
	}
	for r := 0; r < game.WallSize; r++ {
		for c := 0; c < game.WallSize; c++ {
			cellRect := pad(image.Rect(
				origin.X+c*cell, origin.Y+r*cell,
				origin.X+(c+1)*cell, origin.Y+(r+1)*cell), 4)
			drawWallCell(cellRect, filled[[2]int{r, c}], game.ColorAt(r, c))
		}
	}
	drawGridLines(origin, cell, game.WallSize)

	// Thick frame around the completed row (row 0) to call it out as the
	// scoring highlight.
	rowRect := image.Rect(origin.X-4, origin.Y-4, origin.X+size+4, origin.Y+cell+4)
	ink.DrawRect(rowRect, ink.Black)
	ink.DrawRect(pad(rowRect, 1), ink.Black)
}
