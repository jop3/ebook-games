package ink

// Real TrueType text rendering so emulator layout matches the device: glyphs are
// drawn from the bundled Go fonts (proportional, full Latin incl. å ä ö), and
// StringWidth/CharWidth report true advances. DrawString positions by TOP-LEFT
// (baseline = y + ascent) to match the SDK contract, exactly as the guide's
// screenshot emulator does.

import (
	"image"
	"image/color"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

var (
	regularFont *sfnt.Font
	boldFont    *sfnt.Font
	fontOnce    sync.Once

	faceCache = map[faceKey]font.Face{}
	faceMu    sync.Mutex
)

type faceKey struct {
	size int
	bold bool
}

func loadFonts() {
	fontOnce.Do(func() {
		regularFont, _ = opentype.Parse(goregular.TTF)
		boldFont, _ = opentype.Parse(gobold.TTF)
	})
}

func faceFor(size int, bold bool) font.Face {
	loadFonts()
	if size <= 0 {
		size = 30
	}
	key := faceKey{size, bold}
	faceMu.Lock()
	defer faceMu.Unlock()
	if f, ok := faceCache[key]; ok {
		return f
	}
	src := regularFont
	if bold {
		src = boldFont
	}
	// Size in pixels: DPI 72 makes 1pt == 1px, so EM height ≈ size px, matching
	// the SDK's pixel-size OpenFont contract closely enough for layout checks.
	f, err := opentype.NewFace(src, &opentype.FaceOptions{
		Size:    float64(size),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil
	}
	faceCache[key] = f
	return f
}

// Font is an opened font at a fixed size/weight.
type Font struct {
	name string
	size int
	bold bool
	face font.Face
}

// OpenFont opens a font; bold is chosen when the name contains "Bold".
func OpenFont(name string, size int, aa bool) *Font {
	bold := containsFold(name, "Bold") || containsFold(name, "-b")
	return &Font{name: name, size: size, bold: bold, face: faceFor(size, bold)}
}

// SetActive makes this font the current drawing font and sets the text colour.
func (f *Font) SetActive(cl color.Color) {
	if f == nil {
		return
	}
	dev.mu.Lock()
	dev.curFont = f
	dev.curColor = cl
	dev.mu.Unlock()
}

// Close releases the font (no-op; faces are cached process-wide).
func (f *Font) Close() {}

func currentFace() (font.Face, color.Color) {
	if dev.curFont != nil && dev.curFont.face != nil {
		return dev.curFont.face, dev.curColor
	}
	return faceFor(30, false), dev.curColor
}

// DrawString draws s with its TOP-LEFT at p, using the current font/colour, and
// records the span for the play driver.
func DrawString(p image.Point, s string) {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	face, cl := currentFace()
	if face == nil {
		return
	}
	m := face.Metrics()
	ascent := m.Ascent.Ceil()
	height := (m.Ascent + m.Descent).Ceil()
	d := &font.Drawer{
		Dst:  dev.canvas(),
		Src:  image.NewUniform(cl),
		Face: face,
		Dot:  fixed.P(p.X, p.Y+ascent),
	}
	w := d.MeasureString(s).Ceil()
	d.DrawString(s)
	dev.spans = append(dev.spans, TextSpan{
		S:    s,
		Rect: image.Rect(p.X, p.Y, p.X+w, p.Y+height),
	})
}

// DrawStringR is right-aligned in the SDK; approximate by drawing left of p.
func DrawStringR(p image.Point, s string) {
	w := StringWidth(s)
	DrawString(image.Pt(p.X-w, p.Y), s)
}

// StringWidth measures s with the current font.
func StringWidth(s string) int {
	dev.mu.Lock()
	defer dev.mu.Unlock()
	face, _ := currentFace()
	if face == nil {
		return 0
	}
	return font.MeasureString(face, s).Ceil()
}

// CharWidth measures a single rune with the current font.
func CharWidth(c rune) int {
	return StringWidth(string(c))
}

// SetTextStrength is a no-op in the emulator.
func SetTextStrength(n int) {}

func containsFold(s, sub string) bool {
	ls, lsub := toLower(s), toLower(sub)
	if len(lsub) == 0 {
		return true
	}
	for i := 0; i+len(lsub) <= len(ls); i++ {
		if ls[i:i+len(lsub)] == lsub {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
