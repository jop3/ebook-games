package ui

import (
	"image"

	ink "github.com/dennwc/inkview"
)

// MotifFunc draws the game's line-art centered in the given box.
type MotifFunc func(box image.Rectangle)

// DrawSplash renders the start screen: the game title, a large line-art motif,
// and a "tap to start" hint — echoing the built-in chess app's opening screen.
//
// Vertical layout uses usableH (1340), not screen.Y from ink.ScreenSize()
// (which over-reports at 1448 — see POCKETBOOK_GAMEDEV_GUIDE.md §5).
func DrawSplash(screen image.Point, f *Fonts, title string, motif MotifFunc) {
	ink.ClearScreen()
	h := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 80, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, h/6), title)
	tf.Close()

	side := screen.X * 3 / 5
	box := image.Rect((screen.X-side)/2, (h-side)/2,
		(screen.X+side)/2, (h+side)/2)
	motif(box)

	hint := ink.OpenFont(ink.DefaultFont, 34, true)
	hint.SetActive(ink.DarkGray)
	ht := "Tryck för att börja"
	hw := ink.StringWidth(ht)
	ink.DrawString(image.Pt((screen.X-hw)/2, h*5/6), ht)
	hint.Close()
}

// SplashMotif draws the four player marks X O △ □ in a 2x2 arrangement — the
// game's pieces, in simple line graphics.
func SplashMotif(box image.Rectangle) {
	cx := (box.Min.X + box.Max.X) / 2
	cy := (box.Min.Y + box.Max.Y) / 2
	r := box.Dx() / 6
	off := box.Dx() / 4
	tl := image.Pt(cx-off, cy-off)
	tr := image.Pt(cx+off, cy-off)
	bl := image.Pt(cx-off, cy+off)
	br := image.Pt(cx+off, cy+off)

	drawMarkX(tl, r)
	drawMarkO(tr, r)
	drawMarkTriangle(bl, r)
	drawMarkSquare(br, r)
}

func thickLineSplash(a, b image.Point) {
	for o := -2; o <= 2; o++ {
		ink.DrawLine(image.Pt(a.X, a.Y+o), image.Pt(b.X, b.Y+o), ink.Black)
		ink.DrawLine(image.Pt(a.X+o, a.Y), image.Pt(b.X+o, b.Y), ink.Black)
	}
}

func drawMarkX(c image.Point, r int) {
	thickLineSplash(image.Pt(c.X-r, c.Y-r), image.Pt(c.X+r, c.Y+r))
	thickLineSplash(image.Pt(c.X+r, c.Y-r), image.Pt(c.X-r, c.Y+r))
}

func drawMarkO(c image.Point, r int) {
	// thick ring by drawing many chords
	for dy := -r; dy <= r; dy++ {
		w := 0
		for w*w+dy*dy <= r*r {
			w++
		}
		// outer edge pixels only (ring thickness ~ r/4)
		inner := r - r/4
		iw := 0
		if inner > 0 {
			for iw*iw+dy*dy <= inner*inner {
				iw++
			}
		}
		ink.DrawLine(image.Pt(c.X-w, c.Y+dy), image.Pt(c.X-iw, c.Y+dy), ink.Black)
		ink.DrawLine(image.Pt(c.X+iw, c.Y+dy), image.Pt(c.X+w, c.Y+dy), ink.Black)
	}
}

func drawMarkTriangle(c image.Point, r int) {
	top := image.Pt(c.X, c.Y-r)
	left := image.Pt(c.X-r, c.Y+r)
	right := image.Pt(c.X+r, c.Y+r)
	thickLineSplash(top, left)
	thickLineSplash(left, right)
	thickLineSplash(right, top)
}

func drawMarkSquare(c image.Point, r int) {
	tl := image.Pt(c.X-r, c.Y-r)
	tr := image.Pt(c.X+r, c.Y-r)
	bl := image.Pt(c.X-r, c.Y+r)
	br := image.Pt(c.X+r, c.Y+r)
	thickLineSplash(tl, tr)
	thickLineSplash(tr, br)
	thickLineSplash(br, bl)
	thickLineSplash(bl, tl)
}
