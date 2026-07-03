package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"
	"sudoku/game"
)

// usableH is the REAL drawable height of the device. ink.ScreenSize().Y
// reports 1448, but content below ~1360 wraps around to the top of the
// screen on actual hardware. All vertical layout must be computed
// against this value, never against ScreenSize().Y.
const usableH = 1340

// layout holds all pixel geometry for the current screen size. It is
// recomputed from ink.ScreenSize() inside Draw (NEVER in the
// constructor — ScreenSize returns garbage before ink.Run).
type layout struct {
	screen image.Rectangle

	gridOrigin image.Point // top-left of the 9x9 grid
	cellSize   int         // side of one cell
	gridSize   int         // side of the whole grid

	// Number pad + action buttons live in a bar below the grid.
	padTop     int
	btnW, btnH int
	btnGap     int
	padOrigin  image.Point // top-left of the 1..9 pad area
}

func computeLayout() layout {
	s := ink.ScreenSize()
	H := usableH
	var l layout
	l.screen = image.Rect(0, 0, s.X, H)

	margin := s.X / 24
	avail := s.X - 2*margin
	l.cellSize = avail / game.N
	l.gridSize = l.cellSize * game.N

	l.btnGap = s.X / 60
	l.btnW = (l.gridSize - 8*l.btnGap) / 9
	l.btnH = l.btnW + l.btnW/3

	// Stack bottom-anchored UI UP from H-margin: action row, then the
	// number pad row above it, then the grid fills what's left above a
	// top title band.
	bottomMargin := 40
	actionTop := H - bottomMargin - l.btnH
	padTop := actionTop - l.btnGap - l.btnH
	l.padTop = padTop

	titleBand := H / 12
	l.gridOrigin = image.Pt((s.X-l.gridSize)/2, titleBand)
	// If the grid would overlap the button area (small usable height),
	// shrink the cell size so the grid ends just above the pad. The
	// button geometry (which is derived from gridSize) must be
	// recomputed afterwards too, otherwise stale (larger) button widths
	// combined with the new (smaller/shifted) padOrigin push the
	// rightmost buttons off the right edge of the screen.
	if l.gridOrigin.Y+l.gridSize+H/40 > padTop {
		maxGridSize := padTop - H/40 - titleBand
		if maxGridSize > 0 {
			l.cellSize = maxGridSize / game.N
			l.gridSize = l.cellSize * game.N
			l.gridOrigin = image.Pt((s.X-l.gridSize)/2, titleBand)

			l.btnW = (l.gridSize - 8*l.btnGap) / 9
			l.btnH = l.btnW + l.btnW/3
			// btnH changed, so the bottom-anchored rows must be
			// re-stacked from the bottom too.
			actionTop = H - bottomMargin - l.btnH
			padTop = actionTop - l.btnGap - l.btnH
			l.padTop = padTop
		}
	}

	l.padOrigin = image.Pt(l.gridOrigin.X, l.padTop)
	return l
}

// cellRect returns the pixel rectangle of grid cell (row,col).
func (l layout) cellRect(row, col int) image.Rectangle {
	x := l.gridOrigin.X + col*l.cellSize
	y := l.gridOrigin.Y + row*l.cellSize
	return image.Rect(x, y, x+l.cellSize, y+l.cellSize)
}

// numButtonRect returns the rectangle for number button d (1..9).
func (l layout) numButtonRect(d int) image.Rectangle {
	i := d - 1
	x := l.padOrigin.X + i*(l.btnW+l.btnGap)
	y := l.padOrigin.Y
	return image.Rect(x, y, x+l.btnW, y+l.btnH)
}

// actionRow returns the y-band top for the row of action buttons
// (Erase, Check, New/Difficulty). It sits under the number row.
func (l layout) actionTop() int {
	return l.padOrigin.Y + l.btnH + l.btnGap
}

// actionButtonRect splits the grid width into `n` equal buttons and
// returns the i-th (0-based) on the action row.
func (l layout) actionButtonRect(i, n int) image.Rectangle {
	totalGap := (n - 1) * l.btnGap
	w := (l.gridSize - totalGap) / n
	x := l.padOrigin.X + i*(w+l.btnGap)
	y := l.actionTop()
	return image.Rect(x, y, x+w, y+l.btnH)
}

// --- drawing helpers -------------------------------------------------

func centerText(f *ink.Font, r image.Rectangle, s string, cl color.Color, size int) {
	f.SetActive(cl)
	w := ink.StringWidth(s)
	x := r.Min.X + (r.Dx()-w)/2
	// Approximate vertical centering: text origin is top-left-ish; nudge
	// down so the glyph sits mid-box.
	y := r.Min.Y + (r.Dy()-size)/2
	ink.DrawString(image.Pt(x, y), s)
}
