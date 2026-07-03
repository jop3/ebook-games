package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"lasordning/series"
)

// editView lets the user set a book's position in its series. The chosen number
// is written back to books_impl.numinseries on save.
type editView struct {
	book   series.Book
	series string
	number int
	err    string
}

func (v *editView) draw(screen image.Point, f *Fonts) []Button {
	ink.ClearScreen()

	f.Title.SetActive(ink.Black)
	ink.DrawString(image.Pt(24, 24), "Ändra ordning")
	ink.DrawLine(image.Pt(0, titleBarH), image.Pt(screen.X, titleBarH), ink.Black)

	// Book title + series.
	f.Head.SetActive(ink.Black)
	ink.DrawString(image.Pt(24, titleBarH+40),
		ellipsize2(f.Head, v.book.DisplayTitle(), screen.X-48))
	f.Small.SetActive(ink.DarkGray)
	ink.DrawString(image.Pt(24, titleBarH+96),
		ellipsize2(f.Small, "Serie: "+v.series, screen.X-48))

	// Big number in the middle with − / + controls.
	cy := screen.Y/2 - 40
	box := image.Rect(screen.X/2-120, cy-100, screen.X/2+120, cy+100)
	ink.DrawRect(box, ink.Black)
	f.Big.SetActive(ink.Black)
	label := itoa(v.number)
	if v.number == 0 {
		label = "–"
	}
	drawCentered(box, label, 120)

	f.Small.SetActive(ink.DarkGray)
	hint := "Bokens nummer i serien (0 = onumrerad)"
	drawCentered(image.Rect(0, box.Max.Y+20, screen.X, box.Max.Y+70), hint, 28)

	if v.err != "" {
		f.Small.SetActive(ink.Black)
		drawCentered(image.Rect(0, box.Max.Y+80, screen.X, box.Max.Y+130),
			"Fel: "+v.err, 28)
	}

	labels := []string{"−", "+", "Spara", "Avbryt"}
	ids := []string{"minus", "plus", "save", "cancel"}
	return drawButtonBar(screen, f, labels, ids)
}
