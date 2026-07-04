package ink

// This file holds the emulator's global device state: the framebuffer, the
// current active font/colour, the recorded display list, and the bound App.
// The real SDK keeps equivalent state inside the C library; here it is plain Go
// so tests can inspect it.

import (
	"image"
	"image/color"
	"sync"
)

// Default PocketBook Verse Pro (PB634) portrait geometry. NOTE the guide's
// warning: ScreenSize().Y reports 1448 but only ~1340 is safely usable. We
// report the real 1448 (matching the device) and let games apply their own
// usable-height constant, exactly as they do on hardware.
const (
	defaultScreenW = 1072
	defaultScreenH = 1448
)

// TextSpan is one recorded DrawString call: the string and the box it occupies
// on screen (top-left origin, width from the real face metrics, height =
// ascent+descent). The play driver uses these to locate on-screen labels.
type TextSpan struct {
	S    string
	Rect image.Rectangle
}

// device is the emulator's single global state, mirroring the C library's
// global drawing context. Guarded by mu because Repaint may be called from the
// app while the harness reads state.
type device struct {
	mu sync.Mutex

	w, h   int
	fb     *image.RGBA // the framebuffer ("screen buffer")
	app    App
	orient Orientation

	curFont  *Font
	curColor color.Color

	spans      []TextSpan // display list of text drawn since last clear-tracking
	needsDraw  bool       // set by Repaint, consumed by the Harness
	drawCalls  int        // number of Draw() invocations (sanity/debug)
	fullUpd    int        // FullUpdate calls since boot
	partialUpd int        // PartialUpdate calls since boot
}

var dev = &device{
	w:        defaultScreenW,
	h:        defaultScreenH,
	curColor: color.Black,
}

// SetScreenSize overrides the emulated screen geometry (default 1072x1448).
// Emulator-only helper; the real SDK has no such function.
func SetScreenSize(w, h int) {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	dev.w, dev.h = w, h
	dev.fb = nil // force reallocation on next draw
}

func (d *device) canvas() *image.RGBA {
	if d.fb == nil || d.fb.Bounds().Dx() != d.w || d.fb.Bounds().Dy() != d.h {
		d.fb = image.NewRGBA(image.Rect(0, 0, d.w, d.h))
		// e-ink starts white.
		fillWhite(d.fb)
	}
	return d.fb
}

func fillWhite(img *image.RGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.SetRGBA(x, y, color.RGBA{0xff, 0xff, 0xff, 0xff})
		}
	}
}

// ScreenSize returns the emulated screen size.
func ScreenSize() image.Point {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	return image.Point{X: dev.w, Y: dev.h}
}

// Screen returns the full-screen rectangle.
func Screen() image.Rectangle {
	return image.Rectangle{Max: ScreenSize()}
}
