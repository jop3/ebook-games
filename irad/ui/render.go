package ui

import (
	"image"

	ink "github.com/dennwc/inkview"

	"irad/game"
)

// Fonts held open for the lifetime of the app. Opened in InitFonts.
type Fonts struct {
	Status *ink.Font
	Button *ink.Font
	Menu   *ink.Font
	Mark   *ink.Font // for X / O glyphs on small boards
}

// InitFonts opens the fonts used across screens. Call once after the app's
// Init. Sizes are tuned for the Verse Pro's 1072px-wide portrait screen.
func InitFonts() *Fonts {
	return &Fonts{
		Status: ink.OpenFont(ink.DefaultFontBold, 44, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 40, true),
		Menu:   ink.OpenFont(ink.DefaultFont, 40, true),
		Mark:   ink.OpenFont(ink.DefaultFontBold, 40, true),
	}
}

// Close releases the fonts.
func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Status, f.Button, f.Menu, f.Mark} {
		fn.Close()
	}
}

// drawCenteredString centers s horizontally within r and vertically by a
// rough baseline offset, using the currently active font.
func drawCenteredString(r image.Rectangle, s string, approxHeight int) {
	w := ink.StringWidth(s)
	x := r.Min.X + (r.Dx()-w)/2
	y := r.Min.Y + (r.Dy()-approxHeight)/2
	ink.DrawString(image.Pt(x, y), s)
}

// DrawBoard renders the grid, stones, blocked cells, and the selection
// highlight (moving phase). It does not call any screen-update function;
// callers decide between PartialUpdate and FullUpdate.
func DrawBoard(l *Layout, gs *game.GameState, f *Fonts) {
	b := &gs.Board

	// Grid lines.
	ink.DrawRect(l.BoardArea, ink.Black)
	for x := 1; x < b.Width; x++ {
		px := l.BoardOrigin.X + x*l.CellSize
		ink.DrawLine(
			image.Pt(px, l.BoardArea.Min.Y),
			image.Pt(px, l.BoardArea.Max.Y),
			ink.Black,
		)
	}
	for y := 1; y < b.Height; y++ {
		py := l.BoardOrigin.Y + y*l.CellSize
		ink.DrawLine(
			image.Pt(l.BoardArea.Min.X, py),
			image.Pt(l.BoardArea.Max.X, py),
			ink.Black,
		)
	}

	f.Mark.SetActive(ink.Black)
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			i := b.Idx(x, y)
			cell := l.CellToScreen(x, y)
			switch {
			case b.Blocked[i]:
				drawBlocked(cell)
			case b.Cells[i] == game.Player1:
				drawStoneX(cell)
			case b.Cells[i] == game.Player2:
				drawStoneO(cell)
			case b.Cells[i] == game.Player3:
				drawStoneTriangle(cell)
			case b.Cells[i] == game.Player4:
				drawStoneSquare(cell)
			}
		}
	}

	// Selection highlight in moving phase.
	if gs.Phase == game.PhaseMoving && gs.Selected >= 0 {
		sx, sy := b.XY(gs.Selected)
		ink.InvertArea(pad(l.CellToScreen(sx, sy), 4))
	}
}

// drawStoneX draws Player1's mark: a filled disc-like X (cross) inset in cell.
func drawStoneX(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/5)
	ink.DrawLine(r.Min, r.Max, ink.Black)
	ink.DrawLine(image.Pt(r.Max.X, r.Min.Y), image.Pt(r.Min.X, r.Max.Y), ink.Black)
	// Thicken by drawing parallel lines.
	for o := 1; o <= 2; o++ {
		ink.DrawLine(image.Pt(r.Min.X+o, r.Min.Y), image.Pt(r.Max.X, r.Max.Y-o), ink.Black)
		ink.DrawLine(image.Pt(r.Min.X, r.Min.Y+o), image.Pt(r.Max.X-o, r.Max.Y), ink.Black)
		ink.DrawLine(image.Pt(r.Max.X-o, r.Min.Y), image.Pt(r.Min.X, r.Max.Y-o), ink.Black)
		ink.DrawLine(image.Pt(r.Max.X, r.Min.Y+o), image.Pt(r.Min.X+o, r.Max.Y), ink.Black)
	}
}

// drawStoneO draws Player2's mark: a ring (filled outer square minus inner).
func drawStoneO(cell image.Rectangle) {
	outer := pad(cell, cell.Dx()/5)
	ink.FillArea(outer, ink.Black)
	inner := pad(outer, outer.Dx()/4)
	ink.FillArea(inner, ink.White)
}

// drawStoneTriangle draws Player3's mark: an upward triangle outline.
func drawStoneTriangle(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/5)
	apex := image.Pt((r.Min.X+r.Max.X)/2, r.Min.Y)
	left := image.Pt(r.Min.X, r.Max.Y)
	right := image.Pt(r.Max.X, r.Max.Y)
	// Draw each edge thrice (parallel offsets) to thicken the stroke.
	for o := 0; o <= 2; o++ {
		dy := image.Pt(0, o)
		ink.DrawLine(apex.Add(dy), left, ink.Black)
		ink.DrawLine(apex.Add(dy), right, ink.Black)
		ink.DrawLine(left.Sub(dy), right.Sub(dy), ink.Black)
	}
}

// drawStoneSquare draws Player4's mark: a thick square ring.
func drawStoneSquare(cell image.Rectangle) {
	r := pad(cell, cell.Dx()/5)
	for o := 0; o <= 2; o++ {
		ink.DrawRect(pad(r, o), ink.Black)
	}
}

// drawBlocked fills a blocked cell with a gray dither so it reads as off.
func drawBlocked(cell image.Rectangle) {
	ink.FillArea(pad(cell, 2), ink.DarkGray)
}

// statusTextTop is the fixed y the status text is drawn at, kept at/below
// the safe top margin (40) instead of vertically centering in StatusBar
// (which would place it at y=28, inside the margin).
const statusTextTop = 40

// DrawStatus renders the top status bar text.
func DrawStatus(l *Layout, text string, f *Fonts) {
	ink.FillArea(l.StatusBar, ink.White)
	f.Status.SetActive(ink.Black)
	w := ink.StringWidth(text)
	x := l.StatusBar.Min.X + (l.StatusBar.Dx()-w)/2
	ink.DrawString(image.Pt(x, statusTextTop), text)
	ink.DrawLine(
		image.Pt(l.StatusBar.Min.X, l.StatusBar.Max.Y),
		image.Pt(l.StatusBar.Max.X, l.StatusBar.Max.Y),
		ink.Black,
	)
}

// Button is a labelled tappable rectangle.
type Button struct {
	Rect  image.Rectangle
	Label string
}

// Hit reports whether p falls inside the button.
func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

// DrawButtonBar lays out and draws the given buttons evenly across the bottom
// bar, returning their hit rectangles.
func DrawButtonBar(l *Layout, labels []string, f *Fonts) []Button {
	ink.FillArea(l.ButtonBar, ink.White)
	ink.DrawLine(
		image.Pt(l.ButtonBar.Min.X, l.ButtonBar.Min.Y),
		image.Pt(l.ButtonBar.Max.X, l.ButtonBar.Min.Y),
		ink.Black,
	)

	f.Button.SetActive(ink.Black)
	n := len(labels)
	if n == 0 {
		return nil
	}
	// gap keeps button edges clear of the safe-area side margin (x=24); 20
	// let the outer buttons touch x=20.
	gap := 26
	totalGap := gap * (n + 1)
	bw := (l.ButtonBar.Dx() - totalGap) / n
	bh := l.ButtonBar.Dy() - 2*gap

	buttons := make([]Button, n)
	for i, label := range labels {
		x0 := l.ButtonBar.Min.X + gap + i*(bw+gap)
		y0 := l.ButtonBar.Min.Y + gap
		r := image.Rect(x0, y0, x0+bw, y0+bh)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawCenteredString(r, label, 40)
		buttons[i] = Button{Rect: r, Label: label}
	}
	return buttons
}
