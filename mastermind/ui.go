package main

import (
	"image"
	"image/color"
	"strconv"

	ink "github.com/dennwc/inkview"
)

// Layout constants. Screen is nominally 1072x1448 portrait, but the real
// drawable height is only ~1340 (content below ~1360 wraps to the top on
// hardware) — usableH is the effective height ALL vertical layout must use.
const (
	margin      = 40
	titleY      = 70
	statusY     = 150
	historyTop  = 210
	rowH        = 96
	pegGap      = 16
	feedbackGap = 40
	usableH     = 1340
)

// Fonts holds every typeface/size the UI uses, opened once for the app
// lifetime. Opening a font is expensive on e-ink, so doing it per draw call
// (once per peg / row / button / label) made every frame slow and dropped
// taps. These are opened in App.Init and closed in App.Close.
type Fonts struct {
	TitleBig  *ink.Font // bold 56 — menu title
	TitleGame *ink.Font // bold 44 — game title
	Heading   *ink.Font // bold 40 — status line / menu subtitle emphasis
	MenuSub   *ink.Font // regular 40 — menu subtitle
	Button    *ink.Font // bold 40 — button labels
	PegDigit  *ink.Font // bold 34 — color number overlaid on a peg
	Detail    *ink.Font // regular 30 — palette / preset detail text
}

// InitFonts opens all fonts used across screens.
func InitFonts() *Fonts {
	return &Fonts{
		TitleBig:  ink.OpenFont(ink.DefaultFontBold, 56, true),
		TitleGame: ink.OpenFont(ink.DefaultFontBold, 44, true),
		Heading:   ink.OpenFont(ink.DefaultFontBold, 40, true),
		MenuSub:   ink.OpenFont(ink.DefaultFont, 40, true),
		Button:    ink.OpenFont(ink.DefaultFontBold, 40, true),
		PegDigit:  ink.OpenFont(ink.DefaultFontBold, 34, true),
		Detail:    ink.OpenFont(ink.DefaultFont, 30, true),
	}
}

// Close releases the fonts.
func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{
		f.TitleBig, f.TitleGame, f.Heading, f.MenuSub, f.Button, f.PegDigit, f.Detail,
	} {
		if fn != nil {
			fn.Close()
		}
	}
}

// drawPeg renders a single peg of the given color at rectangle r. Since the
// display is grayscale, each color is drawn as a distinct pattern: a filled
// circle with a bold digit (1..8) plus a per-color fill style, so colors stay
// visually separable even for the color-blind / low-contrast e-ink panel.
func drawPeg(r image.Rectangle, c Color, filled bool, digit *ink.Font) {
	// Outer ring.
	ink.DrawRect(r, ink.Black)
	inner := ink.Pad(r, 3)
	ink.DrawRect(inner, ink.Black)

	if !filled {
		return
	}

	switch c % 8 {
	case 0: // solid black
		ink.FillArea(ink.Pad(r, 6), ink.Black)
	case 1: // ring (white center)
		ink.FillArea(ink.Pad(r, 6), ink.Black)
		ink.FillArea(ink.Pad(r, 16), ink.White)
	case 2: // dark gray fill
		ink.FillArea(ink.Pad(r, 6), ink.DarkGray)
	case 3: // light gray fill
		ink.FillArea(ink.Pad(r, 6), ink.LightGray)
	case 4: // horizontal bars
		ink.FillArea(ink.Pad(r, 6), ink.White)
		sz := r.Size()
		step := 12
		for y := r.Min.Y + 10; y < r.Max.Y-8; y += step {
			ink.FillArea(image.Rect(r.Min.X+8, y, r.Max.X-8, y+5), ink.Black)
		}
		_ = sz
	case 5: // vertical bars
		ink.FillArea(ink.Pad(r, 6), ink.White)
		step := 12
		for x := r.Min.X + 10; x < r.Max.X-8; x += step {
			ink.FillArea(image.Rect(x, r.Min.Y+8, x+5, r.Max.Y-8), ink.Black)
		}
	case 6: // cross / plus
		ink.FillArea(ink.Pad(r, 6), ink.White)
		cx := (r.Min.X + r.Max.X) / 2
		cy := (r.Min.Y + r.Max.Y) / 2
		ink.FillArea(image.Rect(r.Min.X+10, cy-5, r.Max.X-10, cy+5), ink.Black)
		ink.FillArea(image.Rect(cx-5, r.Min.Y+10, cx+5, r.Max.Y-10), ink.Black)
	case 7: // checker / diagonal
		ink.FillArea(ink.Pad(r, 6), ink.White)
		p1 := image.Pt(r.Min.X+8, r.Min.Y+8)
		p2 := image.Pt(r.Max.X-8, r.Max.Y-8)
		ink.DrawLine(p1, p2, ink.Black)
		ink.DrawLine(image.Pt(r.Min.X+8, r.Max.Y-8), image.Pt(r.Max.X-8, r.Min.Y+8), ink.Black)
	}

	// Overlay the color number so it is unambiguous.
	if digit != nil {
		// Use inverse-friendly color: draw digit in white on dark fills, black otherwise.
		lbl := c % 8
		if lbl == 0 || lbl == 2 {
			digit.SetActive(ink.White)
		} else {
			digit.SetActive(ink.Black)
		}
		s := strconv.Itoa(int(c) + 1)
		w := ink.StringWidth(s)
		cx := (r.Min.X+r.Max.X)/2 - w/2
		cy := (r.Min.Y+r.Max.Y)/2 - 20
		ink.DrawString(image.Pt(cx, cy), s)
	}
}

// drawFeedback renders black/white feedback dots to the right of a guess row.
func drawFeedback(x, y int, fb Feedback, pegs int) {
	r := 12
	gap := 8
	cx := x
	cy := y
	drawn := 0
	perRow := (pegs + 1) / 2
	place := func(fill color.Color, outline bool) {
		col := drawn % perRow
		rowN := drawn / perRow
		px := cx + col*(r*2+gap)
		py := cy + rowN*(r*2+gap)
		rect := image.Rect(px, py, px+r*2, py+r*2)
		if outline {
			ink.DrawRect(rect, ink.Black)
		} else {
			ink.FillArea(rect, fill)
		}
		drawn++
	}
	for i := 0; i < fb.Black; i++ {
		place(ink.Black, false)
	}
	for i := 0; i < fb.White; i++ {
		place(ink.White, true) // hollow outline = "white" peg
	}
}

// drawTitle draws a centered title string using an already-open font.
func drawTitle(f *ink.Font, s string, y int) {
	if f == nil {
		return
	}
	f.SetActive(ink.Black)
	sz := ink.ScreenSize()
	w := ink.StringWidth(s)
	ink.DrawString(image.Pt(sz.X/2-w/2, y), s)
}

// drawTextAt draws left-aligned text using an already-open font.
func drawTextAt(f *ink.Font, p image.Point, s string) {
	if f == nil {
		return
	}
	f.SetActive(ink.Black)
	ink.DrawString(p, s)
}

// button describes a tappable rectangle.
type button struct {
	rect  image.Rectangle
	label string
	hot   bool // if false, drawn disabled (grayed)
}

func (b button) draw(f *ink.Font) {
	ink.DrawRect(b.rect, ink.Black)
	ink.DrawRect(ink.Pad(b.rect, 2), ink.Black)
	var cl color.Color = ink.Black
	if !b.hot {
		cl = ink.LightGray
	}
	if f != nil {
		f.SetActive(cl)
		w := ink.StringWidth(b.label)
		cx := (b.rect.Min.X+b.rect.Max.X)/2 - w/2
		cy := (b.rect.Min.Y+b.rect.Max.Y)/2 - 24
		ink.DrawString(image.Pt(cx, cy), b.label)
	}
}

func (b button) hit(p image.Point) bool {
	return b.hot && p.In(b.rect)
}
