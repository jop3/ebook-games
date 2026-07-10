package main

import (
	"image"

	ink "github.com/dennwc/inkview"
)

// Fonts held open for the app lifetime (opened once in Init — never per frame,
// which on e-ink is slow and drops taps). Same set as lasordning.
type Fonts struct {
	Title  *ink.Font // screen title
	Head   *ink.Font // game names
	Small  *ink.Font // status / secondary lines
	Button *ink.Font // bottom-bar buttons
}

func InitFonts() *Fonts {
	return &Fonts{
		Title:  ink.OpenFont(ink.DefaultFontBold, 48, true),
		Head:   ink.OpenFont(ink.DefaultFontBold, 40, true),
		Small:  ink.OpenFont(ink.DefaultFont, 28, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 34, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Title, f.Head, f.Small, f.Button} {
		if fn != nil {
			fn.Close()
		}
	}
}

// --- geometry helpers ---------------------------------------------------------

// topMargin keeps content clear of the device's status strip (guide §5).
const topMargin = 46

const (
	titleBarH  = 110 + topMargin
	rowH       = 108
	buttonBarH = 130
)

func pad(r image.Rectangle, n int) image.Rectangle {
	return image.Rect(r.Min.X+n, r.Min.Y+n, r.Max.X-n, r.Max.Y-n)
}

func drawCentered(r image.Rectangle, s string, approxH int) {
	w := ink.StringWidth(s)
	ink.DrawString(image.Pt(r.Min.X+(r.Dx()-w)/2, r.Min.Y+(r.Dy()-approxH)/2), s)
}

// ellipsize trims s with a trailing "…" so it fits within maxW pixels using the
// currently-active font. Rune-safe: never splits an å/ä/ö.
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

// wrapText greedily word-wraps s to maxW pixels using the active font.
func wrapText(s string, maxW int) []string {
	var lines []string
	cur := ""
	for _, w := range splitWords(s) {
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

// --- bottom button bar ----------------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
	ID    string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

// drawButtonBar renders a row of buttons at the bottom and returns their rects.
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
		f.Button.SetActive(ink.Black)
		drawCentered(r, ellipsize(label, r.Dx()-16), 34)
		out[i] = Button{Rect: r, Label: label, ID: ids[i]}
	}
	return out
}
