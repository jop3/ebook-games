package main

import (
	"image"

	ink "github.com/dennwc/inkview"
)

// Fonts held open for the app lifetime (opened once in Init — never per frame,
// which on e-ink is slow and drops taps).
type Fonts struct {
	Title   *ink.Font // screen titles
	Head    *ink.Font // series / author headings in the list
	Body    *ink.Font // book rows
	Small   *ink.Font // secondary / source labels
	Button  *ink.Font // bottom-bar buttons
	ButtonS *ink.Font // smaller button font for long labels
	Big     *ink.Font // the big number on the edit screen
}

func InitFonts() *Fonts {
	return &Fonts{
		Title:   ink.OpenFont(ink.DefaultFontBold, 48, true),
		Head:    ink.OpenFont(ink.DefaultFontBold, 40, true),
		Body:    ink.OpenFont(ink.DefaultFont, 36, true),
		Small:   ink.OpenFont(ink.DefaultFont, 28, true),
		Button:  ink.OpenFont(ink.DefaultFontBold, 34, true),
		ButtonS: ink.OpenFont(ink.DefaultFontBold, 26, true),
		Big:     ink.OpenFont(ink.DefaultFontBold, 120, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Title, f.Head, f.Body, f.Small, f.Button, f.ButtonS, f.Big} {
		if fn != nil {
			fn.Close()
		}
	}
}

// --- geometry helpers -------------------------------------------------------

func pad(r image.Rectangle, n int) image.Rectangle {
	return image.Rect(r.Min.X+n, r.Min.Y+n, r.Max.X-n, r.Max.Y-n)
}

func drawCentered(r image.Rectangle, s string, approxH int) {
	w := ink.StringWidth(s)
	ink.DrawString(image.Pt(r.Min.X+(r.Dx()-w)/2, r.Min.Y+(r.Dy()-approxH)/2), s)
}

func drawLeft(r image.Rectangle, s string, approxH int) {
	ink.DrawString(image.Pt(r.Min.X+24, r.Min.Y+(r.Dy()-approxH)/2), s)
}

// drawWrappedCentered word-wraps s to the width of r (using the Body font) and
// draws the lines centered near the top of r. Used for the multi-line status
// message that lists which online sources were checked.
func drawWrappedCentered(r image.Rectangle, s string, f *Fonts) {
	f.Body.SetActive(ink.Black)
	lines := wrapText(s, r.Dx())
	lineH := 46
	y := r.Min.Y
	for _, ln := range lines {
		w := ink.StringWidth(ln)
		ink.DrawString(image.Pt(r.Min.X+(r.Dx()-w)/2, y), ln)
		y += lineH
	}
}

// wrapText greedily wraps s to maxW pixels using the currently-active font.
func wrapText(s string, maxW int) []string {
	words := splitWords(s)
	var lines []string
	cur := ""
	for _, w := range words {
		try := w
		if cur != "" {
			try = cur + " " + w
		}
		if ink.StringWidth(try) <= maxW {
			cur = try
		} else {
			if cur != "" {
				lines = append(lines, cur)
			}
			cur = w
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
		if r == ' ' || r == '\n' {
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

// ellipsize trims s with a trailing "…" so it fits within maxW pixels using the
// currently-active font.
func ellipsize(s string, maxW int) string {
	if ink.StringWidth(s) <= maxW {
		return s
	}
	r := []rune(s)
	for len(r) > 1 {
		r = r[:len(r)-1]
		if ink.StringWidth(string(r)+"…") <= maxW {
			return string(r) + "…"
		}
	}
	return "…"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// topMargin is breathing room reserved at the very top of every screen. Content
// hugging y=0 sat under the device's status strip and was hard to tap; this
// pushes titles (and the top divider) down so nothing sits kant-i-kant with the
// screen edge.
const topMargin = 46

// --- bottom button bar ------------------------------------------------------

const buttonBarH = 130

type Button struct {
	Rect  image.Rectangle
	Label string
	ID    string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

// drawButtonBar renders a row of buttons at the bottom and returns their rects.
// ids must be the same length as labels. Each label is fitted to its button:
// the normal button font is used if it fits, else a smaller one, else the text
// is ellipsized — so a long label like "Hamta hela serien" never overflows into
// the neighbouring button.
func drawButtonBar(screen image.Point, f *Fonts, labels, ids []string) []Button {
	bar := image.Rect(0, screen.Y-buttonBarH, screen.X, screen.Y)
	ink.FillArea(bar, ink.White)
	ink.DrawLine(image.Pt(bar.Min.X, bar.Min.Y), image.Pt(bar.Max.X, bar.Min.Y), ink.Black)
	n := len(labels)
	if n == 0 {
		return nil
	}
	gap := 20
	bw := (bar.Dx() - gap*(n+1)) / n
	bh := bar.Dy() - 2*gap
	out := make([]Button, n)
	for i, label := range labels {
		x0 := bar.Min.X + gap + i*(bw+gap)
		r := image.Rect(x0, bar.Min.Y+gap, x0+bw, bar.Min.Y+gap+bh)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		drawButtonLabel(r, label, f)
		out[i] = Button{Rect: r, Label: label, ID: ids[i]}
	}
	return out
}

// drawButtonLabel draws label centered in r, choosing the largest button font
// that fits (with an inner padding), ellipsizing only if even the small font
// overflows.
func drawButtonLabel(r image.Rectangle, label string, f *Fonts) {
	maxW := r.Dx() - 16
	// try normal, then small
	f.Button.SetActive(ink.Black)
	if ink.StringWidth(label) <= maxW {
		drawCentered(r, label, 34)
		return
	}
	f.ButtonS.SetActive(ink.Black)
	if ink.StringWidth(label) <= maxW {
		drawCentered(r, label, 26)
		return
	}
	drawCentered(r, ellipsize(label, maxW), 26)
}
