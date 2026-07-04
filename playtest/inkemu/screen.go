package ink

import "image"

// Repaint requests a redraw. On device this queues a Draw event; here it just
// sets a flag the Harness consumes after each injected input, then calls Draw.
func Repaint() {
	dev.mu.Lock()
	dev.needsDraw = true
	dev.mu.Unlock()
}

// The *Update calls present the framebuffer on device. Here they only bump
// counters so tests can assert a frame was flushed if they want to.
func FullUpdate() {
	dev.mu.Lock()
	dev.fullUpd++
	dev.mu.Unlock()
}

func SoftUpdate() { FullUpdate() }

func PartialUpdate(r image.Rectangle) {
	dev.mu.Lock()
	dev.partialUpd++
	dev.mu.Unlock()
}

func PartialUpdateBW(r image.Rectangle) { PartialUpdate(r) }

func SetOrientation(o Orientation) {
	dev.mu.Lock()
	dev.orient = o
	dev.mu.Unlock()
}

func SetDefaultOrientation(o Orientation) { SetOrientation(o) }

func SetGlobalOrientation(o Orientation) { SetOrientation(o) }

func GetOrientation() Orientation {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	return dev.orient
}

func GetGlobalOrientation() Orientation { return GetOrientation() }
