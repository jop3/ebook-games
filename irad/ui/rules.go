package ui

import (
	"image"

	ink "github.com/dennwc/inkview"
)

// wrapText breaks s into lines no wider than maxW pixels for the active font.
func wrapText(s string, maxW int) []string {
	var lines []string
	var cur string
	for _, word := range splitWords(s) {
		try := word
		if cur != "" {
			try = cur + " " + word
		}
		if ink.StringWidth(try) > maxW && cur != "" {
			lines = append(lines, cur)
			cur = word
		} else {
			cur = try
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func splitWords(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ' ' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// RulesParagraphs is the rules text for "I rad".
var RulesParagraphs = []string{
	"Mål: var först med att lägga X antal av dina markeringar i en obruten rad — vågrätt, lodrätt eller diagonalt. Hur många som krävs står i variantens namn (t.ex. 3, 4 eller 5 i rad).",
	"Spelarna turas om att trycka på en tom ruta för att lägga sin markering. Markeringar: Spelare 1 = X, 2 = O, 3 = △, 4 = □.",
	"På menyn väljer du en färdig variant eller bygger en egen med Anpassad (bredd, höjd och hur många i rad). Läge-knappen växlar mellan mot dator och 2–4 spelare (hot-seat).",
	"I vissa varianter kan brädet ha blockerade rutor eller andra regler — men grundidén är alltid densamma: bilda din rad före motståndarna.",
	"Blir brädet fullt utan någon vinnande rad slutar partiet oavgjort.",
}

// DrawRules renders the rules text with a back button and returns its rect.
//
// The back button is bottom-anchored against usableH (1340), not screen.Y
// from ink.ScreenSize() (which over-reports at 1448 — see
// POCKETBOOK_GAMEDEV_GUIDE.md §5).
func DrawRules(screen image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	ink.ClearScreen()
	h := usableH

	tf := ink.OpenFont(ink.DefaultFontBold, 56, true)
	tf.SetActive(ink.Black)
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((screen.X-tw)/2, 60), title)
	tf.Close()

	body := ink.OpenFont(ink.DefaultFont, 34, true)
	body.SetActive(ink.Black)
	margin := 60
	maxW := screen.X - 2*margin
	y := 180
	lineH := 46
	paraGap := 24
	for _, p := range paragraphs {
		for _, ln := range wrapText(p, maxW) {
			ink.DrawString(image.Pt(margin, y), ln)
			y += lineH
		}
		y += paraGap
	}
	body.Close()

	bh := 110
	bw := screen.X / 2
	r := image.Rect((screen.X-bw)/2, h-bh-40, (screen.X+bw)/2, h-40)
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 1), ink.Black)
	f.Button.SetActive(ink.Black)
	drawCenteredString(r, "Tillbaka", 38)
	return r
}
