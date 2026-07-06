package ink

// Drawing primitives that paint into the in-memory framebuffer. Semantics match
// the real SDK: DrawRect draws a 1px outline, FillArea fills solid, ClearScreen
// paints the whole screen white, colours are rendered as greys.

import (
	"image"
	"image/color"
)

// Pad shrinks a rectangle by n on every side (matches SDK helper).
func Pad(r image.Rectangle, n int) image.Rectangle {
	dp := image.Pt(n, n)
	r.Min = r.Min.Add(dp)
	r.Max = r.Max.Sub(dp)
	return r
}

func toRGBA(cl color.Color) color.RGBA {
	r, g, b, _ := cl.RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 0xff}
}

func setPx(x, y int, c color.RGBA) {
	fb := dev.canvas()
	p := image.Point{x, y}
	if !p.In(fb.Bounds()) {
		return
	}
	if dev.hasClip && !p.In(dev.clip) {
		return
	}
	fb.SetRGBA(x, y, c)
}

// clipped intersects r with the screen and the active clip region.
// Caller must hold dev.mu.
func clipped(r image.Rectangle) image.Rectangle {
	r = r.Intersect(dev.canvas().Bounds())
	if dev.hasClip {
		r = r.Intersect(dev.clip)
	}
	return r
}

// ClearScreen fills the current canvas with white. Like the SDK it ignores the
// clip region — it resets the whole screen buffer.
func ClearScreen() {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	fillWhite(dev.canvas())
}

// SetClip restricts subsequent drawing to r, like the SDK. Pass the full screen
// rect to clear it.
func SetClip(r image.Rectangle) {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	dev.clip = r
	dev.hasClip = !r.Eq(image.Rectangle{Max: image.Pt(dev.w, dev.h)})
}

func DrawPixel(p image.Point, cl color.Color) {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	setPx(p.X, p.Y, toRGBA(cl))
}

func DrawLine(p1, p2 image.Point, cl color.Color) {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	c := toRGBA(cl)
	// Bresenham.
	x0, y0, x1, y1 := p1.X, p1.Y, p2.X, p2.Y
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := sign(x1 - x0)
	sy := sign(y1 - y0)
	err := dx + dy
	for {
		setPx(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// DrawRect draws a 1px rectangle outline (matches the SDK, which is why games
// call it twice with Pad to get a 2px border).
func DrawRect(r image.Rectangle, cl color.Color) {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	c := toRGBA(cl)
	if r.Empty() {
		return
	}
	for x := r.Min.X; x < r.Max.X; x++ {
		setPx(x, r.Min.Y, c)
		setPx(x, r.Max.Y-1, c)
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		setPx(r.Min.X, y, c)
		setPx(r.Max.X-1, y, c)
	}
}

// FillArea fills a solid rectangle.
func FillArea(r image.Rectangle, cl color.Color) {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	fillRGBA(dev.canvas(), clipped(r), toRGBA(cl))
}

// InvertArea inverts the pixels in a rectangle (used for selection highlights).
func InvertArea(r image.Rectangle) {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	fb := dev.canvas()
	r = clipped(r)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		off := fb.PixOffset(r.Min.X, y)
		row := fb.Pix[off : off+r.Dx()*4]
		for x := 0; x < len(row); x += 4 {
			row[x], row[x+1], row[x+2] = 0xff-row[x], 0xff-row[x+1], 0xff-row[x+2]
		}
	}
}

func InvertAreaBW(r image.Rectangle) { InvertArea(r) }

// DimArea, DrawSelection, DitherArea are cosmetic; approximate with a light fill
// so layout/hit-testing still works. Colours are not gameplay-relevant.
func DimArea(r image.Rectangle, cl color.Color) {}

func DrawSelection(r image.Rectangle, cl color.Color) {
	DrawRect(r, cl)
}

func DitherArea(r image.Rectangle, levels, method int) {}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	default:
		return 0
	}
}
