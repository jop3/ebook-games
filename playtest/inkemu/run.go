package ink

// App and the event loop. On device, Run() blocks in the C event loop and the
// C library calls back into Init/Draw/Pointer/... Here the loop is Go: Boot()
// wires an app to the emulator and returns a Harness that drives it exactly the
// way a finger would — through the real Pointer/Key/Draw path.

import (
	"fmt"
	"image"
)

// App is the interface every game's top-level struct implements. Identical to
// the real SDK's App so game code compiles unchanged.
type App interface {
	Init() error
	Close() error
	Draw()
	Key(e KeyEvent) bool
	Pointer(e PointerEvent) bool
	Touch(e TouchEvent) bool
	Orientation(o Orientation) bool
}

// Run wires the app and renders one frame, then returns. It exists only so a
// game's main() still compiles under the emulator; real play-testing uses Boot.
func Run(app App) error {
	_, err := Boot(app)
	return err
}

// Exit is a no-op in the emulator.
func Exit() {}

// SetErr is a no-op kept for API compatibility.
func SetErr(err error) {}

// Harness drives a booted app headlessly.
type Harness struct {
	app App
}

// Boot binds app to the emulator, runs Init, and renders the first frame.
func Boot(app App) (*Harness, error) {
	if app == nil {
		return nil, fmt.Errorf("ink.Boot: nil app")
	}
	dev.mu.Lock()
	dev.app = app
	dev.spans = nil
	dev.mu.Unlock()

	if err := app.Init(); err != nil {
		return nil, fmt.Errorf("app.Init: %w", err)
	}
	h := &Harness{app: app}
	h.drainRepaint()
	return h, nil
}

// Draw forces a redraw, refreshing the recorded text display list.
func (h *Harness) Draw() {
	dev.mu.Lock()
	dev.spans = dev.spans[:0]
	dev.needsDraw = false
	dev.drawCalls++
	dev.mu.Unlock()
	h.app.Draw()
}

// drainRepaint renders frames until the app stops asking for more. Some games
// do deferred work inside Draw (e.g. Othello computes the AI reply on the frame
// AFTER the player's move, then calls Repaint again), so a single draw is not
// enough. The cap guards against a game that repaints unconditionally.
func (h *Harness) drainRepaint() {
	for i := 0; i < 200; i++ {
		dev.mu.Lock()
		need := dev.needsDraw
		dev.mu.Unlock()
		if !need {
			return
		}
		h.Draw()
	}
}

// Tap simulates a finger tap (pointer down then up) at p and re-renders if the
// app requested a repaint. Returns whether the app handled the tap.
func (h *Harness) Tap(p image.Point) bool {
	h.app.Pointer(PointerEvent{Point: p, State: PointerDown})
	handled := h.app.Pointer(PointerEvent{Point: p, State: PointerUp})
	h.drainRepaint()
	return handled
}

// TapXY is Tap with loose coordinates.
func (h *Harness) TapXY(x, y int) bool { return h.Tap(image.Pt(x, y)) }

// TapRect taps the centre of a rectangle (the common case: a button/cell rect).
func (h *Harness) TapRect(r image.Rectangle) bool {
	return h.Tap(image.Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2))
}

// Press sends a hardware key (down then up).
func (h *Harness) Press(k Key) bool {
	h.app.Key(KeyEvent{Key: k, State: KeyStateDown})
	handled := h.app.Key(KeyEvent{Key: k, State: KeyStateUp})
	h.drainRepaint()
	return handled
}

// Back presses the hardware Back key (the standard "go up a screen" gesture).
func (h *Harness) Back() bool { return h.Press(KeyBack) }

// Rotate delivers an orientation change.
func (h *Harness) Rotate(o Orientation) {
	h.app.Orientation(o)
	h.drainRepaint()
}

// Texts returns the strings drawn in the most recent frame, with their boxes.
func (h *Harness) Texts() []TextSpan {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	out := make([]TextSpan, len(dev.spans))
	copy(out, dev.spans)
	return out
}

// FindText returns the first span exactly equal to s.
func (h *Harness) FindText(s string) (TextSpan, bool) {
	for _, sp := range h.Texts() {
		if sp.S == s {
			return sp, true
		}
	}
	return TextSpan{}, false
}

// FindTextContains returns the first span whose string contains sub.
func (h *Harness) FindTextContains(sub string) (TextSpan, bool) {
	for _, sp := range h.Texts() {
		if containsFold(sp.S, sub) {
			return sp, true
		}
	}
	return TextSpan{}, false
}

// TapText finds an on-screen label equal to s and taps its centre. Returns an
// error if no such label is on screen, so tests fail loudly on a UI change.
func (h *Harness) TapText(s string) error {
	sp, ok := h.FindText(s)
	if !ok {
		return fmt.Errorf("TapText: no on-screen text %q (visible: %v)", s, h.visibleStrings())
	}
	h.TapRect(sp.Rect)
	return nil
}

// TapTextContains taps the first label containing sub.
func (h *Harness) TapTextContains(sub string) error {
	sp, ok := h.FindTextContains(sub)
	if !ok {
		return fmt.Errorf("TapTextContains: no on-screen text containing %q (visible: %v)", sub, h.visibleStrings())
	}
	h.TapRect(sp.Rect)
	return nil
}

func (h *Harness) visibleStrings() []string {
	sps := h.Texts()
	out := make([]string, 0, len(sps))
	for _, sp := range sps {
		out = append(out, sp.S)
	}
	return out
}

// FullUpdates/PartialUpdates/DrawCount expose flush/draw counters for assertions.
func (h *Harness) FullUpdates() int    { dev.mu.Lock(); defer dev.mu.Unlock(); return dev.fullUpd }
func (h *Harness) PartialUpdates() int { dev.mu.Lock(); defer dev.mu.Unlock(); return dev.partialUpd }
func (h *Harness) DrawCount() int      { dev.mu.Lock(); defer dev.mu.Unlock(); return dev.drawCalls }

// Frame returns the current framebuffer image (for a screenshot).
func (h *Harness) Frame() image.Image {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	return dev.canvas()
}
