package ui

import (
	"image"

	"irad/game"
)

// Layout maps between screen pixels and board cells. It is recomputed
// whenever the board or screen size changes (orientation, new variant).
type Layout struct {
	Screen      image.Rectangle
	BoardArea   image.Rectangle // square region the grid is drawn in
	BoardOrigin image.Point     // top-left pixel of cell (0,0)
	CellSize    int
	Cols, Rows  int

	StatusBar image.Rectangle
	ButtonBar image.Rectangle
}

const (
	statusBarHeight = 100
	buttonBarHeight = 140
	// boardMargin keeps the board's outer edge clear of the safe-area side
	// margin (x ∈ [24, 1048] on the PB634); 16 was too tight and let the
	// board rect touch x=16.
	boardMargin = 26

	// usableH is the real drawable height on the PB634. ink.ScreenSize()
	// reports 1448, but content below ~1360 wraps to the top of the
	// screen on hardware. All vertical layout must use this instead of
	// screen.Y (see POCKETBOOK_GAMEDEV_GUIDE.md §5).
	usableH = 1340
)

// NewLayout computes the layout for a board on a given screen size. Vertical
// layout is computed against the real usable height (1340), not the raw
// screen.Y from ink.ScreenSize() (which over-reports at 1448).
func NewLayout(screen image.Point, b *game.Board) Layout {
	h := usableH
	l := Layout{
		Screen: image.Rectangle{Max: image.Pt(screen.X, h)},
		Cols:   b.Width,
		Rows:   b.Height,
	}

	l.StatusBar = image.Rect(0, 0, screen.X, statusBarHeight)
	l.ButtonBar = image.Rect(0, h-buttonBarHeight, screen.X, h)

	// Region available for the board, between the two bars.
	avail := image.Rect(0, statusBarHeight, screen.X, h-buttonBarHeight)
	avail = pad(avail, boardMargin)

	availW := avail.Dx()
	availH := avail.Dy()

	// Largest square cell that fits both dimensions.
	cw := availW / b.Width
	ch := availH / b.Height
	cell := cw
	if ch < cell {
		cell = ch
	}
	if cell < 1 {
		cell = 1
	}
	l.CellSize = cell

	gridW := cell * b.Width
	gridH := cell * b.Height

	// Center the grid within the available area.
	l.BoardOrigin = image.Pt(
		avail.Min.X+(availW-gridW)/2,
		avail.Min.Y+(availH-gridH)/2,
	)
	l.BoardArea = image.Rect(
		l.BoardOrigin.X, l.BoardOrigin.Y,
		l.BoardOrigin.X+gridW, l.BoardOrigin.Y+gridH,
	)
	return l
}

// CellToScreen returns the pixel rectangle of cell (x,y).
func (l *Layout) CellToScreen(x, y int) image.Rectangle {
	return image.Rect(
		l.BoardOrigin.X+x*l.CellSize,
		l.BoardOrigin.Y+y*l.CellSize,
		l.BoardOrigin.X+(x+1)*l.CellSize,
		l.BoardOrigin.Y+(y+1)*l.CellSize,
	)
}

// ScreenToCell resolves a pixel point to a board cell. ok is false if the
// point lies outside the grid.
func (l *Layout) ScreenToCell(p image.Point) (x, y int, ok bool) {
	if l.CellSize == 0 {
		return 0, 0, false
	}
	rel := p.Sub(l.BoardOrigin)
	if rel.X < 0 || rel.Y < 0 {
		return 0, 0, false
	}
	x = rel.X / l.CellSize
	y = rel.Y / l.CellSize
	if x < 0 || x >= l.Cols || y < 0 || y >= l.Rows {
		return 0, 0, false
	}
	return x, y, true
}

func pad(r image.Rectangle, n int) image.Rectangle {
	return image.Rect(r.Min.X+n, r.Min.Y+n, r.Max.X-n, r.Max.Y-n)
}
