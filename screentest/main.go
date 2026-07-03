// Command screentest pinpoints the exact wrap boundary (real drawable height).
// It draws fine labeled lines from y=1300 to y=1447 every 10px. Whatever is
// still at the BOTTOM is inside the real buffer; the first value that jumps to
// the TOP marks the boundary.
package main

import (
	"fmt"
	"image"

	ink "github.com/dennwc/inkview"
)

type app struct{ f *ink.Font }

func (a *app) Init() error {
	a.f = ink.OpenFont(ink.DefaultFontBold, 36, true)
	ink.Repaint()
	return nil
}
func (a *app) Close() error { return nil }

func (a *app) Draw() {
	ink.ClearScreen()
	a.f.SetActive(ink.Black)
	ink.DrawString(image.Pt(80, 200), "Vilket ar det HOGSTA y langst ner?")

	// Fine lines every 10px in the suspect zone. Alternate x so labels don't
	// overlap. The real height is the highest y still shown at the bottom.
	for y := 1300; y <= 1447; y += 10 {
		ink.DrawLine(image.Pt(0, y), image.Pt(1072, y), ink.Black)
		x := 100
		if (y/10)%2 == 1 {
			x = 620
		}
		ink.DrawString(image.Pt(x, y-40), fmt.Sprintf("y=%d", y))
	}
	ink.FullUpdate()
}

func (a *app) Key(e ink.KeyEvent) bool { return false }
func (a *app) Pointer(e ink.PointerEvent) bool {
	if e.State == ink.PointerUp {
		ink.Exit()
	}
	return true
}
func (a *app) Touch(e ink.TouchEvent) bool        { return false }
func (a *app) Orientation(o ink.Orientation) bool { ink.Repaint(); return true }

func main() {
	if err := ink.Run(&app{}); err != nil {
		panic(err)
	}
}
