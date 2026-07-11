package main

import (
	"image"

	ink "github.com/dennwc/inkview"
)

// Fonts held open for the app lifetime (opened once in Init — never per frame,
// which on e-ink is slow and drops taps). Same set as spelbutiken.
type Fonts struct {
	Title  *ink.Font // screen title
	Head   *ink.Font // book names
	Small  *ink.Font // status / secondary lines
	Button *ink.Font // bottom-bar buttons
	Splash *ink.Font // big splash title
}

func InitFonts() *Fonts {
	return &Fonts{
		Title:  ink.OpenFont(ink.DefaultFontBold, 48, true),
		Head:   ink.OpenFont(ink.DefaultFontBold, 38, true),
		Small:  ink.OpenFont(ink.DefaultFont, 28, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 34, true),
		Splash: ink.OpenFont(ink.DefaultFontBold, 76, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Title, f.Head, f.Small, f.Button, f.Splash} {
		if fn != nil {
			fn.Close()
		}
	}
}

// --- geometry helpers ---------------------------------------------------------

// usableH is the real drawable height; ScreenSize().Y reports 1448 but content
// below ~1360 wraps to the top of the panel (guide §5). All vertical layout
// anchors to this.
const usableH = 1340

// topMargin keeps content clear of the device's status strip (guide §5a).
const topMargin = 46

const (
	titleBarH  = 110 + topMargin
	rowH       = 108
	buttonBarH = 130
	// swipeMin is the vertical finger travel that counts as a scroll gesture
	// instead of a tap (guide §5a).
	swipeMin = 110
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

// fmtSize renders a byte count the Swedish way ("2,4 MB").
func fmtSize(n int64) string {
	switch {
	case n >= 1<<30:
		return deci(n, 1<<30) + " GB"
	case n >= 1<<20:
		return deci(n, 1<<20) + " MB"
	case n >= 1<<10:
		return deci(n, 1<<10) + " kB"
	default:
		return itoa(int(n)) + " B"
	}
}

// deci formats n/unit with one decimal and a decimal comma.
func deci(n, unit int64) string {
	tenths := n * 10 / unit
	whole := tenths / 10
	frac := tenths % 10
	if frac == 0 {
		return itoa(int(whole))
	}
	return itoa(int(whole)) + "," + itoa(int(frac))
}

// --- bottom button bar ----------------------------------------------------------

type Button struct {
	Rect  image.Rectangle
	Label string
	ID    string
}

func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

// drawButtonBar renders a row of buttons at the bottom of the usable area and
// returns their rects.
func drawButtonBar(screen image.Point, f *Fonts, labels, ids []string) []Button {
	bar := image.Rect(0, usableH-buttonBarH, screen.X, usableH)
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

// --- splash ---------------------------------------------------------------------

// drawSplash paints the launch screen: big title, a book-under-a-cloud motif
// (the app's whole idea in one picture), and the tap hint. Chess-app style,
// like every other app in the library (guide §10).
func drawSplash(screen image.Point, f *Fonts) {
	f.Splash.SetActive(ink.Black)
	title := "Boksynk"
	ink.DrawString(image.Pt((screen.X-ink.StringWidth(title))/2, usableH/6), title)

	side := screen.X * 3 / 5
	box := image.Rect((screen.X-side)/2, usableH/2-side/2, (screen.X+side)/2, usableH/2+side/2)
	drawSplashMotif(box)

	f.Small.SetActive(ink.DarkGray)
	hint := "Tryck för att börja"
	ink.DrawString(image.Pt((screen.X-ink.StringWidth(hint))/2, usableH*5/6), hint)
}

// drawSplashMotif draws a cloud with a down arrow feeding an open book —
// monochrome line art only.
func drawSplashMotif(box image.Rectangle) {
	w, h := box.Dx(), box.Dy()

	// Cloud: three overlapping outlined "puffs" (rectangles with the corners
	// knocked off read as a cloud at e-ink resolution; keep it simple —
	// concentric rounded blocks).
	cw := w * 3 / 5
	ch := h / 5
	cx := box.Min.X + (w-cw)/2
	cy := box.Min.Y + h/12
	cloud := image.Rect(cx, cy, cx+cw, cy+ch)
	ink.DrawRect(cloud, ink.Black)
	ink.DrawRect(pad(cloud, 1), ink.Black)
	// a smaller puff on top
	puff := image.Rect(cx+cw/4, cy-ch/2, cx+cw*3/4, cy)
	ink.DrawRect(puff, ink.Black)
	ink.DrawRect(pad(puff, 1), ink.Black)
	// erase the seam so the two boxes read as one shape
	ink.FillArea(image.Rect(puff.Min.X+2, cy-2, puff.Max.X-2, cy+2), ink.White)

	// Arrow from cloud down to the book.
	ax := box.Min.X + w/2
	ay0 := cloud.Max.Y + h/24
	ay1 := box.Min.Y + h*11/20
	for d := -2; d <= 2; d++ {
		ink.DrawLine(image.Pt(ax+d, ay0), image.Pt(ax+d, ay1), ink.Black)
	}
	hw := w / 14
	for i := 0; i <= hw; i++ {
		ink.DrawLine(image.Pt(ax-hw+i, ay1-hw+i), image.Pt(ax-hw+i, ay1-hw+i+3), ink.Black)
		ink.DrawLine(image.Pt(ax+hw-i, ay1-hw+i), image.Pt(ax+hw-i, ay1-hw+i+3), ink.Black)
	}

	// Open book: two page panels meeting at a spine, with text lines.
	bw := w * 4 / 5
	bh := h / 4
	bx := box.Min.X + (w-bw)/2
	by := box.Max.Y - bh - h/12
	spine := bx + bw/2
	left := image.Rect(bx, by, spine, by+bh)
	right := image.Rect(spine, by, bx+bw, by+bh)
	ink.DrawRect(left, ink.Black)
	ink.DrawRect(pad(left, 1), ink.Black)
	ink.DrawRect(right, ink.Black)
	ink.DrawRect(pad(right, 1), ink.Black)
	// text lines on each page
	for i := 1; i <= 3; i++ {
		y := by + i*bh/4
		ink.DrawLine(image.Pt(bx+16, y), image.Pt(spine-16, y), ink.DarkGray)
		ink.DrawLine(image.Pt(spine+16, y), image.Pt(bx+bw-16, y), ink.DarkGray)
	}
}
