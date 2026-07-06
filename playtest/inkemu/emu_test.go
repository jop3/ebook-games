package ink

// Self-tests for the emulator itself: device semantics (Boot resets state and
// always draws a first frame), drawing-primitive correctness, clipping, text
// span recording, and the harness input paths. These run with a plain
// `go test` in this directory — no game, no workspace overlay needed.

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

// stubApp is a minimal App whose behaviour each test scripts via closures.
type stubApp struct {
	init    func()
	draw    func()
	pointer func(PointerEvent) bool
	touch   func(TouchEvent) bool
	key     func(KeyEvent) bool
}

func (a *stubApp) Init() error {
	if a.init != nil {
		a.init()
	}
	return nil
}
func (a *stubApp) Close() error { return nil }
func (a *stubApp) Draw() {
	if a.draw != nil {
		a.draw()
	}
}
func (a *stubApp) Key(e KeyEvent) bool {
	if a.key != nil {
		return a.key(e)
	}
	return false
}
func (a *stubApp) Pointer(e PointerEvent) bool {
	if a.pointer != nil {
		return a.pointer(e)
	}
	return false
}
func (a *stubApp) Touch(e TouchEvent) bool {
	if a.touch != nil {
		return a.touch(e)
	}
	return false
}
func (a *stubApp) Orientation(o Orientation) bool { return false }

func mustBoot(t *testing.T, a App) *Harness {
	t.Helper()
	h, err := Boot(a)
	if err != nil {
		t.Fatalf("Boot: %v", err)
	}
	return h
}

func pxAt(t *testing.T, h *Harness, x, y int) color.RGBA {
	t.Helper()
	img := h.Frame().(*image.RGBA)
	return img.RGBAAt(x, y)
}

var (
	white = color.RGBA{0xff, 0xff, 0xff, 0xff}
	black = color.RGBA{0x00, 0x00, 0x00, 0xff}
)

// Boot must render a first frame even when Init never calls Repaint — on
// hardware the OS always sends a show event after Init.
func TestBootDrawsWithoutRepaint(t *testing.T) {
	a := &stubApp{draw: func() {
		f := OpenFont(DefaultFont, 30, true)
		f.SetActive(Black)
		DrawString(image.Pt(10, 10), "hello")
	}}
	h := mustBoot(t, a)
	if h.DrawCount() != 1 {
		t.Fatalf("DrawCount = %d, want 1", h.DrawCount())
	}
	if _, ok := h.FindText("hello"); !ok {
		t.Fatalf("first frame not rendered: no spans recorded")
	}
}

// Booting a second app must start from power-on state: white screen, zeroed
// counters, no pending repaint inherited from the previous app.
func TestBootResetsDeviceState(t *testing.T) {
	first := &stubApp{draw: func() {
		FillArea(Screen(), Black)
		FullUpdate()
	}}
	h1 := mustBoot(t, first)
	if h1.FullUpdates() != 1 {
		t.Fatalf("first app FullUpdates = %d, want 1", h1.FullUpdates())
	}
	Repaint() // leave a pending repaint behind, as an interrupted app might

	quiet := &stubApp{}
	h := mustBoot(t, quiet)
	if got := pxAt(t, h, 5, 5); got != white {
		t.Errorf("screen not reset to white after re-Boot: got %v", got)
	}
	if h.DrawCount() != 1 || h.FullUpdates() != 0 || h.PartialUpdates() != 0 {
		t.Errorf("counters not reset: draws=%d full=%d partial=%d",
			h.DrawCount(), h.FullUpdates(), h.PartialUpdates())
	}
}

// An app that requests a repaint from every Draw is an infinite redraw loop on
// device; the harness must fail loudly rather than silently stop draining.
func TestRunawayRepaintPanics(t *testing.T) {
	a := &stubApp{draw: func() { Repaint() }}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("Boot of an unconditionally-repainting app did not panic")
		}
		if !strings.Contains(r.(string), "unconditionally") {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	_, _ = Boot(a)
}

func TestFillAndClearAndInvert(t *testing.T) {
	r := image.Rect(100, 200, 110, 210)
	a := &stubApp{draw: func() {
		FillArea(r, Black)
		// Off-screen fill must clamp, not crash.
		FillArea(image.Rect(-50, -50, 10, 10), DarkGray)
		FillArea(image.Rect(2000, 2000, 3000, 3000), Black)
	}}
	h := mustBoot(t, a)
	if got := pxAt(t, h, 105, 205); got != black {
		t.Errorf("inside fill = %v, want black", got)
	}
	if got := pxAt(t, h, 99, 205); got != white {
		t.Errorf("left of fill = %v, want white", got)
	}
	if got := pxAt(t, h, 105, 210); got != white {
		t.Errorf("below fill (exclusive Max) = %v, want white", got)
	}
	if got := pxAt(t, h, 5, 5); (got != color.RGBA{0x55, 0x55, 0x55, 0xff}) {
		t.Errorf("clamped off-screen fill = %v, want dark gray", got)
	}

	a2 := &stubApp{draw: func() {
		FillArea(r, Black)
		InvertArea(image.Rect(105, 200, 115, 210)) // half over black, half over white
	}}
	h = mustBoot(t, a2)
	if got := pxAt(t, h, 107, 205); got != white {
		t.Errorf("inverted black = %v, want white", got)
	}
	if got := pxAt(t, h, 112, 205); got != black {
		t.Errorf("inverted white = %v, want black", got)
	}
}

func TestDrawRectIsOnePixelOutline(t *testing.T) {
	r := image.Rect(50, 50, 60, 60)
	h := mustBoot(t, &stubApp{draw: func() { DrawRect(r, Black) }})
	if got := pxAt(t, h, 50, 55); got != black {
		t.Errorf("left edge = %v, want black", got)
	}
	if got := pxAt(t, h, 59, 55); got != black {
		t.Errorf("right edge (Max-1) = %v, want black", got)
	}
	if got := pxAt(t, h, 55, 55); got != white {
		t.Errorf("interior = %v, want white", got)
	}
}

func TestDrawLineEndpoints(t *testing.T) {
	h := mustBoot(t, &stubApp{draw: func() {
		DrawLine(image.Pt(10, 10), image.Pt(30, 20), Black)
	}})
	if got := pxAt(t, h, 10, 10); got != black {
		t.Errorf("line start = %v, want black", got)
	}
	if got := pxAt(t, h, 30, 20); got != black {
		t.Errorf("line end = %v, want black", got)
	}
}

func TestSetClipRestrictsDrawing(t *testing.T) {
	clip := image.Rect(0, 0, 200, 200)
	h := mustBoot(t, &stubApp{draw: func() {
		SetClip(clip)
		FillArea(image.Rect(150, 150, 300, 300), Black) // half in, half out
		DrawLine(image.Pt(150, 250), image.Pt(300, 250), Black)
		f := OpenFont(DefaultFont, 40, true)
		f.SetActive(Black)
		DrawString(image.Pt(150, 400), "clippedtext") // entirely outside the clip
		SetClip(Screen()) // full-screen clip = no clipping again
		FillArea(image.Rect(500, 500, 510, 510), Black)
	}})
	if got := pxAt(t, h, 160, 160); got != black {
		t.Errorf("inside clip = %v, want black", got)
	}
	if got := pxAt(t, h, 250, 250); got != white {
		t.Errorf("fill outside clip = %v, want white", got)
	}
	if got := pxAt(t, h, 220, 250); got != white {
		t.Errorf("line outside clip = %v, want white", got)
	}
	// The span is still recorded (the SDK reports the string as drawn), but no
	// pixels may land outside the clip.
	sp, ok := h.FindText("clippedtext")
	if !ok {
		t.Fatalf("clipped DrawString did not record a span")
	}
	img := h.Frame().(*image.RGBA)
	for y := sp.Rect.Min.Y; y < sp.Rect.Max.Y; y++ {
		for x := sp.Rect.Min.X; x < sp.Rect.Max.X; x++ {
			if img.RGBAAt(x, y) != white {
				t.Fatalf("clipped text painted pixel at %d,%d", x, y)
			}
		}
	}
	if got := pxAt(t, h, 505, 505); got != black {
		t.Errorf("fill after clearing clip = %v, want black", got)
	}
}

func TestTextSpansAndSwedishFolding(t *testing.T) {
	h := mustBoot(t, &stubApp{draw: func() {
		f := OpenFont(DefaultFontBold, 36, true)
		f.SetActive(Black)
		DrawString(image.Pt(100, 300), "VÄLKOMMEN ÅÄÖ")
	}})
	sp, ok := h.FindTextContains("välkommen åäö")
	if !ok {
		t.Fatalf("case folding failed for Swedish text; visible: %v", h.visibleStrings())
	}
	if sp.Rect.Min.X != 100 || sp.Rect.Min.Y != 300 {
		t.Errorf("span origin = %v, want (100,300)", sp.Rect.Min)
	}
	if sp.Rect.Dx() <= 0 || sp.Rect.Dy() <= 0 {
		t.Errorf("span has no area: %v", sp.Rect)
	}
	w := func() int {
		f := OpenFont(DefaultFontBold, 36, true)
		f.SetActive(Black)
		return StringWidth("VÄLKOMMEN ÅÄÖ")
	}()
	if sp.Rect.Dx() != w {
		t.Errorf("span width %d != StringWidth %d", sp.Rect.Dx(), w)
	}
}

func TestDrawStringRRightAligns(t *testing.T) {
	h := mustBoot(t, &stubApp{draw: func() {
		f := OpenFont(DefaultFont, 30, true)
		f.SetActive(Black)
		DrawStringR(image.Pt(800, 100), "right")
	}})
	sp, ok := h.FindText("right")
	if !ok {
		t.Fatalf("DrawStringR recorded no span")
	}
	if sp.Rect.Max.X != 800 {
		t.Errorf("right edge = %d, want 800", sp.Rect.Max.X)
	}
}

func TestTapAndTapText(t *testing.T) {
	var got []PointerEvent
	a := &stubApp{
		draw: func() {
			f := OpenFont(DefaultFont, 30, true)
			f.SetActive(Black)
			DrawString(image.Pt(400, 600), "Meny")
		},
		pointer: func(e PointerEvent) bool { got = append(got, e); return true },
	}
	h := mustBoot(t, a)
	if err := h.TapText("Meny"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].State != PointerDown || got[1].State != PointerUp {
		t.Fatalf("tap events = %+v, want down+up", got)
	}
	sp, _ := h.FindText("Meny")
	if !got[1].Point.In(sp.Rect) {
		t.Errorf("tap at %v missed label rect %v", got[1].Point, sp.Rect)
	}
	if err := h.TapText("no such label"); err == nil {
		t.Errorf("TapText on a missing label should error")
	}
}

func TestTouchPath(t *testing.T) {
	var got []TouchEvent
	a := &stubApp{touch: func(e TouchEvent) bool { got = append(got, e); return true }}
	h := mustBoot(t, a)
	if !h.TouchXY(123, 456) {
		t.Fatalf("TouchXY not handled")
	}
	if len(got) != 2 || got[0].State != TouchDown || got[1].State != TouchUp {
		t.Fatalf("touch events = %+v, want down+up", got)
	}
	if got[0].Point != image.Pt(123, 456) {
		t.Errorf("touch point = %v", got[0].Point)
	}
}

func TestPressAndRepaintDrain(t *testing.T) {
	frames := 0
	pending := 0
	a := &stubApp{
		draw: func() {
			frames++
			if pending > 0 { // deferred work: keep asking for frames
				pending--
				Repaint()
			}
		},
		key: func(e KeyEvent) bool {
			if e.State == KeyStateUp {
				pending = 3
				Repaint()
				return true
			}
			return false
		},
	}
	h := mustBoot(t, a)
	if !h.Back() {
		t.Fatalf("Back not handled")
	}
	// 1 boot frame + 4 drained frames (initial repaint + 3 chained).
	if frames != 5 {
		t.Errorf("frames = %d, want 5", frames)
	}
	if h.DrawCount() != frames {
		t.Errorf("DrawCount %d != frames %d", h.DrawCount(), frames)
	}
}

func TestFrameIsACopy(t *testing.T) {
	h := mustBoot(t, &stubApp{})
	img := h.Frame().(*image.RGBA)
	FillArea(image.Rect(0, 0, 10, 10), Black)
	if img.RGBAAt(5, 5) != white {
		t.Errorf("Frame aliases the live framebuffer")
	}
}

func TestSetScreenSize(t *testing.T) {
	SetScreenSize(600, 800)
	defer SetScreenSize(defaultScreenW, defaultScreenH)
	h := mustBoot(t, &stubApp{draw: func() { FillArea(Screen(), Black) }})
	if got := ScreenSize(); got != image.Pt(600, 800) {
		t.Fatalf("ScreenSize = %v", got)
	}
	if got := h.Frame().Bounds().Max; got != image.Pt(600, 800) {
		t.Fatalf("framebuffer bounds = %v", got)
	}
}

func BenchmarkFullFrameFill(b *testing.B) {
	dev.mu.Lock()
	dev.reset()
	dev.mu.Unlock()
	for i := 0; i < b.N; i++ {
		ClearScreen()
		FillArea(image.Rect(0, 0, defaultScreenW, defaultScreenH), Black)
	}
}
