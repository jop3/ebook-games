package game

// Preset is a named, data-driven game configuration. Classic games are
// rows in the Presets table rather than separate code paths.
type Preset struct {
	Name       string
	Width      int
	Height     int
	WinLength  int
	DropMode   bool
	PieceLimit int                   // 0 = unlimited
	Blocked    func(w, h int) []bool // nil = no obstacles
}

// Presets is the menu of built-in variants. "Anpassad" (custom) is handled
// separately in the menu via steppers and is not listed here.
var Presets = []Preset{
	{"Tre i rad", 3, 3, 3, false, 0, nil},
	{"Tre i kvarn", 3, 3, 3, false, 3, nil},
	{"Fyra i rad", 7, 6, 4, true, 0, nil},
	{"Fem i rad", 13, 13, 5, false, 0, nil},
	{"Fem i rad, vertikal", 7, 10, 5, true, 0, nil},
	{"Hinder - Korset", 13, 13, 5, false, 0, crossPattern},
	{"Hinder - Ramen", 13, 13, 5, false, 0, framePattern},
}

// crossPattern blocks the central row and column.
func crossPattern(w, h int) []bool {
	blocked := make([]bool, w*h)
	cx, cy := w/2, h/2
	for y := 0; y < h; y++ {
		blocked[y*w+cx] = true
	}
	for x := 0; x < w; x++ {
		blocked[cy*w+x] = true
	}
	return blocked
}

// framePattern blocks the outermost ring of cells.
func framePattern(w, h int) []bool {
	blocked := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x == 0 || y == 0 || x == w-1 || y == h-1 {
				blocked[y*w+x] = true
			}
		}
	}
	return blocked
}

// CustomPreset builds an unlimited free-placement preset of the given size.
// Used by the "Anpassad" menu entry.
func CustomPreset(width, height, winLength int) Preset {
	return Preset{
		Name:      "Anpassad",
		Width:     width,
		Height:    height,
		WinLength: winLength,
	}
}
