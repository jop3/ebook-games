package ui

import (
	"image"

	ink "github.com/dennwc/inkview"

	"irad/game"
)

// Mode is the player configuration chosen in the menu. It spans the full
// 1–4 player range: ModeAI is one human versus the AI; Mode2/3/4 are hot-seat
// matches with that many human players.
type Mode int

const (
	ModeAI Mode = iota // 1 human + AI
	Mode2              // 2 humans, hot-seat
	Mode3              // 3 humans, hot-seat
	Mode4              // 4 humans, hot-seat
	modeCount
)

// modeLabel is the human-readable description of a mode.
func modeLabel(m Mode) string {
	switch m {
	case ModeAI:
		return "Läge: Mot AI"
	case Mode2:
		return "Läge: 2 spelare"
	case Mode3:
		return "Läge: 3 spelare"
	case Mode4:
		return "Läge: 4 spelare"
	default:
		return "Läge: ?"
	}
}

// MenuItem is one tappable row in the variant list.
type menuRow struct {
	rect  image.Rectangle
	label string
}

// PresetRowRects returns the hit rectangles of the preset rows (populated by
// Draw). Exposed for the headless emulator harness.
func (m *Menu) PresetRowRects() []image.Rectangle {
	rs := make([]image.Rectangle, len(m.presetRows))
	for i, r := range m.presetRows {
		rs[i] = r.rect
	}
	return rs
}

// PresetLabel returns the label of preset row i. For the emulator harness.
func (m *Menu) PresetLabel(i int) string {
	if i < 0 || i >= len(m.presetRows) {
		return ""
	}
	return m.presetRows[i].label
}

// Menu holds the start-screen state: variant selection, mode selector, and
// the custom-size steppers.
type Menu struct {
	Mode Mode

	// Custom variant parameters, edited via steppers.
	CustomW, CustomH, CustomWin int

	// Hit regions, recomputed each Draw.
	presetRows            []menuRow
	customRow             menuRow
	modeBtn               image.Rectangle
	startCustom           image.Rectangle
	rulesBtn              image.Rectangle
	stepW, stepH, stepWin [2]image.Rectangle // [0]=minus [1]=plus
}

// NewMenu returns a menu with sensible custom defaults.
func NewMenu() *Menu {
	return &Menu{
		Mode:    ModeAI,
		CustomW: 9, CustomH: 9, CustomWin: 5,
	}
}

// VsAI reports whether the selected mode plays against the AI.
func (m *Menu) VsAI() bool { return m.Mode == ModeAI }

// NumPlayers returns the human player count implied by the mode (AI mode has
// a single human seat plus the AI, handled as a two-player game).
func (m *Menu) NumPlayers() int {
	switch m.Mode {
	case Mode3:
		return 3
	case Mode4:
		return 4
	default:
		return 2
	}
}

const (
	customMinSize = 3
	customMaxSize = 19
	customMinWin  = 3
)

// clampCustom keeps custom parameters in valid ranges.
func (m *Menu) clampCustom() {
	clamp := func(v *int, lo, hi int) {
		if *v < lo {
			*v = lo
		}
		if *v > hi {
			*v = hi
		}
	}
	clamp(&m.CustomW, customMinSize, customMaxSize)
	clamp(&m.CustomH, customMinSize, customMaxSize)
	// Win length can't exceed the larger dimension.
	maxDim := m.CustomW
	if m.CustomH > maxDim {
		maxDim = m.CustomH
	}
	clamp(&m.CustomWin, customMinWin, maxDim)
}

// Draw renders the menu and records hit regions. Caller follows with a
// FullUpdate.
func (m *Menu) Draw(screen image.Point, f *Fonts) {
	m.clampCustom()
	ink.ClearScreen()

	title := ink.OpenFont(ink.DefaultFontBold, 56, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("I rad")
	ink.DrawString(image.Pt((screen.X-tw)/2, 40), "I rad")
	title.Close()

	f.Menu.SetActive(ink.Black)

	y := 140
	rowH := 84
	margin := 50
	rowW := screen.X - 2*margin

	m.presetRows = m.presetRows[:0]
	for _, p := range game.Presets {
		r := image.Rect(margin, y, margin+rowW, y+rowH-16)
		ink.DrawRect(r, ink.Black)
		drawLeftString(r, p.Name, 40)
		m.presetRows = append(m.presetRows, menuRow{rect: r, label: p.Name})
		y += rowH
	}

	// Custom row with steppers.
	cr := image.Rect(margin, y, margin+rowW, y+rowH-16)
	ink.DrawRect(cr, ink.Black)
	m.customRow = menuRow{rect: cr, label: "Anpassad"}
	label := "Anpassad: " + itoa(m.CustomW) + "x" + itoa(m.CustomH) + ", " + itoa(m.CustomWin) + " i rad"
	drawLeftString(cr, label, 40)
	y += rowH

	// Stepper buttons for W, H, Win.
	m.stepW = m.drawStepper(margin, y, "Bredd", m.CustomW, f)
	y += 80
	m.stepH = m.drawStepper(margin, y, "Höjd", m.CustomH, f)
	y += 80
	m.stepWin = m.drawStepper(margin, y, "Vinst", m.CustomWin, f)
	y += 80

	startC := image.Rect(margin, y, margin+rowW, y+rowH-16)
	ink.DrawRect(startC, ink.Black)
	ink.DrawRect(pad(startC, 1), ink.Black)
	drawCenteredString(startC, "Starta Anpassad", 40)
	m.startCustom = startC
	y += rowH + 10

	// Mode selector (tap to cycle AI / 2 / 3 / 4 players).
	ob := image.Rect(margin, y, margin+rowW, y+rowH-16)
	ink.DrawRect(ob, ink.Black)
	ink.DrawRect(pad(ob, 1), ink.Black)
	drawCenteredString(ob, modeLabel(m.Mode)+"  (tryck för att byta)", 40)
	m.modeBtn = ob
	y += rowH + 10

	// "Regler" button opens the rules screen.
	rbW := rowW / 2
	rb := image.Rect((screen.X-rbW)/2, y, (screen.X+rbW)/2, y+rowH-16)
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(pad(rb, 1), ink.Black)
	drawCenteredString(rb, "Regler", 40)
	m.rulesBtn = rb
}

func (m *Menu) drawStepper(x, y int, name string, val int, f *Fonts) [2]image.Rectangle {
	f.Menu.SetActive(ink.Black)
	ink.DrawString(image.Pt(x, y+16), name+": "+itoa(val))

	bw := 64
	minus := image.Rect(x+340, y, x+340+bw, y+bw)
	plus := image.Rect(x+340+bw+20, y, x+340+2*bw+20, y+bw)
	ink.DrawRect(minus, ink.Black)
	ink.DrawRect(plus, ink.Black)
	drawCenteredString(minus, "-", 40)
	drawCenteredString(plus, "+", 40)
	return [2]image.Rectangle{minus, plus}
}

// MenuAction is the result of a menu touch.
type MenuAction struct {
	StartPreset *game.Preset // non-nil: start this preset
	Redraw      bool         // state changed, redraw menu
	OpenRules   bool         // open the rules screen
}

// HandleTouch processes a touch on the menu, returning the action to take.
func (m *Menu) HandleTouch(p image.Point) MenuAction {
	// Steppers.
	if p.In(m.stepW[0]) {
		m.CustomW--
		return MenuAction{Redraw: true}
	}
	if p.In(m.stepW[1]) {
		m.CustomW++
		return MenuAction{Redraw: true}
	}
	if p.In(m.stepH[0]) {
		m.CustomH--
		return MenuAction{Redraw: true}
	}
	if p.In(m.stepH[1]) {
		m.CustomH++
		return MenuAction{Redraw: true}
	}
	if p.In(m.stepWin[0]) {
		m.CustomWin--
		return MenuAction{Redraw: true}
	}
	if p.In(m.stepWin[1]) {
		m.CustomWin++
		return MenuAction{Redraw: true}
	}
	if p.In(m.modeBtn) {
		m.Mode = (m.Mode + 1) % modeCount
		return MenuAction{Redraw: true}
	}
	if p.In(m.startCustom) {
		m.clampCustom()
		preset := game.CustomPreset(m.CustomW, m.CustomH, m.CustomWin)
		return MenuAction{StartPreset: &preset}
	}
	if p.In(m.rulesBtn) {
		return MenuAction{OpenRules: true}
	}
	for i := range m.presetRows {
		if p.In(m.presetRows[i].rect) {
			preset := game.Presets[i]
			return MenuAction{StartPreset: &preset}
		}
	}
	return MenuAction{}
}

func drawLeftString(r image.Rectangle, s string, approxHeight int) {
	x := r.Min.X + 24
	y := r.Min.Y + (r.Dy()-approxHeight)/2
	ink.DrawString(image.Pt(x, y), s)
}

// itoa is a tiny non-allocating-ish integer formatter to avoid importing
// strconv across the UI package surface (keeps deps minimal).
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
