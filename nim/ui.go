package main

import (
	"fmt"
	"image"

	ink "github.com/dennwc/inkview"

	"nim/game"
)

// Fonts holds every (typeface,size) opened ONCE in Init and reused. Never call
// OpenFont inside Draw (guide §4).
type Fonts struct {
	Title *ink.Font // large heading
	Big   *ink.Font // buttons / turn indicator
	Med   *ink.Font // labels
	Small *ink.Font // hints
}

// InitFonts opens all fonts. Call once from app.Init.
func InitFonts() *Fonts {
	return &Fonts{
		Title: ink.OpenFont(ink.DefaultFontBold, 72, true),
		Big:   ink.OpenFont(ink.DefaultFontBold, 44, true),
		Med:   ink.OpenFont(ink.DefaultFont, 34, true),
		Small: ink.OpenFont(ink.DefaultFont, 26, true),
	}
}

// Close releases the fonts (from app.Close).
func (f *Fonts) Close() {
	for _, ff := range []*ink.Font{f.Title, f.Big, f.Med, f.Small} {
		if ff != nil {
			ff.Close()
		}
	}
}

// usableH is the real drawable height on the PB634. ink.ScreenSize() reports
// 1448, but content below ~1360 wraps to the top of the screen (guide §5).
// All vertical layout must be derived from this, never from ScreenSize().Y.
const usableH = 1340

// Button is a tappable rectangular region with a label.
type Button struct {
	Rect  image.Rectangle
	Label string
	ID    string // action id
}

// Hit reports whether p is inside the button.
func (b Button) Hit(p image.Point) bool { return p.In(b.Rect) }

// drawButton renders a bordered button with centered label.
func drawButton(b Button, f *ink.Font, filled bool) {
	if filled {
		ink.FillArea(b.Rect, ink.Black)
		f.SetActive(ink.White)
	} else {
		ink.DrawRect(b.Rect, ink.Black)
		ink.DrawRect(ink.Pad(b.Rect, 1), ink.Black)
		f.SetActive(ink.Black)
	}
	w := ink.StringWidth(b.Label)
	tx := b.Rect.Min.X + (b.Rect.Dx()-w)/2
	ty := b.Rect.Min.Y + (b.Rect.Dy()-fontHeight(f))/2
	ink.DrawString(image.Pt(tx, ty), b.Label)
}

// fontHeight is a rough glyph height for vertical centering (device fonts don't
// expose metrics; approximate from a capital letter width scale).
func fontHeight(f *ink.Font) int {
	// StringWidth("M") ~ font advance; height is ~1.4x that for these fonts.
	return ink.StringWidth("M") * 3 / 2
}

// drawCentered draws s centered in the horizontal band [x0,x1) at top y.
func drawCentered(f *ink.Font, x0, x1, y int, s string) {
	w := ink.StringWidth(s)
	ink.DrawString(image.Pt(x0+(x1-x0-w)/2, y), s)
}

// --- Menu ---

// Menu holds the current menu selections and computes button hit regions.
type Menu struct {
	Variant   game.Variant
	Mode      game.Mode
	PresetIdx int
	// cached button regions from the last Draw (for hit-testing).
	buttons []Button
}

// NewMenu returns a menu with sensible defaults.
func NewMenu() *Menu {
	return &Menu{Variant: game.Normal, Mode: game.SoloAI, PresetIdx: 0}
}

// Buttons returns the hit regions from the last Draw.
func (m *Menu) Buttons() []Button { return m.buttons }

// Draw renders the menu and records button hit regions.
func (m *Menu) Draw(size image.Point, f *Fonts) {
	ink.ClearScreen()
	W := size.X
	m.buttons = m.buttons[:0]

	// Title.
	f.Title.SetActive(ink.Black)
	drawCentered(f.Title, 0, W, 120, "NIM")
	f.Small.SetActive(ink.Black)
	drawCentered(f.Small, 0, W, 210, "Ta sista stickan")

	margin := 90
	bx0, bx1 := margin, W-margin
	bh := 110
	y := 300
	gap := 40

	// Variant toggle.
	vLabel := "Variant: " + m.Variant.String()
	if m.Variant == game.Normal {
		vLabel = "Variant: Normal (sista vinner)"
	} else {
		vLabel = "Variant: Misère (sista förlorar)"
	}
	b := Button{Rect: image.Rect(bx0, y, bx1, y+bh), Label: vLabel, ID: "variant"}
	drawButton(b, f.Med, false)
	m.buttons = append(m.buttons, b)
	y += bh + gap

	// Mode toggle.
	mLabel := "Läge: Solo mot AI"
	if m.Mode == game.TwoPlayer {
		mLabel = "Läge: 2 spelare"
	}
	b = Button{Rect: image.Rect(bx0, y, bx1, y+bh), Label: mLabel, ID: "mode"}
	drawButton(b, f.Med, false)
	m.buttons = append(m.buttons, b)
	y += bh + gap

	// Preset toggle.
	pLabel := "Högar: " + game.Presets[m.PresetIdx].Name
	b = Button{Rect: image.Rect(bx0, y, bx1, y+bh), Label: pLabel, ID: "preset"}
	drawButton(b, f.Med, false)
	m.buttons = append(m.buttons, b)
	y += bh + gap*2

	// Start button (filled).
	b = Button{Rect: image.Rect(bx0, y, bx1, y+bh+20), Label: "SPELA", ID: "start"}
	drawButton(b, f.Big, true)
	m.buttons = append(m.buttons, b)
	y += bh + 20 + gap

	// Regler button (outlined) opens the full rules screen.
	rbW := (bx1 - bx0) / 2
	rbx0 := bx0 + ((bx1-bx0)-rbW)/2
	b = Button{Rect: image.Rect(rbx0, y, rbx0+rbW, y+bh), Label: "Regler", ID: "rules"}
	drawButton(b, f.Med, false)
	m.buttons = append(m.buttons, b)

	// Footer hint.
	f.Small.SetActive(ink.Black)
	drawCentered(f.Small, 0, W, size.Y-90, "AI:n spelar perfekt — kan du slå den?")
}

// --- Game layout ---

// Layout stores the on-screen geometry of the current game board so taps can be
// mapped back to (pile, action). Recomputed every Draw from ScreenSize.
type Layout struct {
	PileRects []image.Rectangle // tappable area per pile column
	Minus     Button
	Plus      Button
	Confirm   Button
	MenuBtn   Button
}

// GameView renders the game screen and returns the layout for hit-testing.
type GameView struct{}

// DrawGame renders piles, selection UI, turn indicator and buttons. selPile is
// the currently selected pile (-1 = none), selCount is how many sticks are
// marked for removal. status is the line under the title. Returns the layout.
func DrawGame(size image.Point, f *Fonts, gs *game.GameState, selPile, selCount int, status string, aiWinning bool) Layout {
	ink.ClearScreen()
	W, H := size.X, size.Y
	var lay Layout

	// Header.
	f.Big.SetActive(ink.Black)
	title := "Spelare 1"
	if gs.Turn == 1 {
		if gs.Mode == game.SoloAI {
			title = "AI"
		} else {
			title = "Spelare 2"
		}
	}
	if !gs.Over {
		drawCentered(f.Big, 0, W, 60, "Tur: "+title)
	} else {
		wl := "Spelare 1 vinner!"
		if gs.Winner == 1 {
			if gs.Mode == game.SoloAI {
				wl = "AI vinner!"
			} else {
				wl = "Spelare 2 vinner!"
			}
		}
		drawCentered(f.Big, 0, W, 60, wl)
	}

	// Honesty line: when solo, show whether AI holds a theoretical win.
	f.Small.SetActive(ink.Black)
	if status != "" {
		drawCentered(f.Small, 0, W, 140, status)
	} else if gs.Mode == game.SoloAI && !gs.Over {
		s := "Läget: du kan vinna med perfekt spel"
		// From the human's perspective (turn 0). If it's the human's turn and
		// the mover is NOT winning, the AI holds the win.
		if gs.Turn == 0 && !gs.IsWinningForMover() {
			s = "Läget: AI:n har vinstläge just nu"
		}
		drawCentered(f.Small, 0, W, 140, s)
	}

	// --- Vertical layout derived from screen height with top/bottom margins ---
	// Built bottom-up so the button rows always sit fully on-screen with a
	// clear margin. Order from the bottom: hint line, Confirm/Meny row,
	// controls (− count +) row. The board fills what remains below the header.
	margin := H / 24 // ~60px top/bottom margin at 1448
	bh := 100        // button height for the controls row
	rowGap := 24     // gap between stacked rows

	hintY := H - margin         // baseline for the bottom hint line
	btnH := bh + 10             // Confirm/Meny slightly taller
	btnY := hintY - 40 - btnH   // Confirm/Meny row top (40px above hint)
	ctrlY := btnY - rowGap - bh // controls (−/+/count) row top

	boardTop := 220
	boardBot := ctrlY - rowGap // board ends just above the controls
	sideMargin := 30
	boardX0, boardX1 := sideMargin, W-sideMargin
	nPiles := len(gs.Piles)
	colW := (boardX1 - boardX0) / nPiles
	lay.PileRects = make([]image.Rectangle, nPiles)

	// Max sticks in any pile for scaling stick height.
	maxSticks := 1
	for _, n := range gs.Piles {
		if n > maxSticks {
			maxSticks = n
		}
	}

	for i, n := range gs.Piles {
		cx0 := boardX0 + i*colW
		cx1 := cx0 + colW
		col := image.Rect(cx0, boardTop, cx1, boardBot)
		lay.PileRects[i] = col

		// Highlight selected pile.
		if i == selPile {
			ink.DrawRect(col, ink.Black)
			ink.DrawRect(ink.Pad(col, 2), ink.Black)
			ink.DrawRect(ink.Pad(col, 4), ink.Black)
		} else {
			ink.DrawRect(col, ink.LightGray)
		}

		// Draw sticks as vertical bars, bottom-aligned, grouped in this column.
		drawPile(f, col, n, maxSticks, i == selPile, selCount)

		// Pile label (index + count) at top of column.
		f.Small.SetActive(ink.Black)
		lbl := fmt.Sprintf("Hög %d: %d", i+1, n)
		drawCentered(f.Small, cx0, cx1, boardTop+8, lbl)
	}

	// --- Controls row: [ - ] [ count ] [ + ]  (ctrlY/bh computed above) ---
	third := W / 3
	lay.Minus = Button{Rect: image.Rect(40, ctrlY, third-10, ctrlY+bh), Label: "−", ID: "minus"}
	lay.Plus = Button{Rect: image.Rect(2*third+10, ctrlY, W-40, ctrlY+bh), Label: "+", ID: "plus"}
	drawButton(lay.Minus, f.Big, false)
	drawButton(lay.Plus, f.Big, false)

	// count display in the middle
	f.Big.SetActive(ink.Black)
	cnt := "–"
	if selPile >= 0 {
		cnt = fmt.Sprintf("Ta %d", selCount)
	}
	drawCentered(f.Big, third, 2*third, ctrlY+bh/2-30, cnt)

	// Confirm + Menu buttons (btnY/btnH computed above).
	half := W / 2
	confirmEnabled := selPile >= 0 && selCount >= 1 && !gs.Over && !(gs.Mode == game.SoloAI && gs.Turn == 1)
	lay.Confirm = Button{Rect: image.Rect(40, btnY, half-15, btnY+btnH), Label: "Bekräfta", ID: "confirm"}
	lay.MenuBtn = Button{Rect: image.Rect(half+15, btnY, W-40, btnY+btnH), Label: "Meny", ID: "menu"}
	drawButton(lay.Confirm, f.Big, confirmEnabled)
	drawButton(lay.MenuBtn, f.Med, false)

	// Hint line, anchored to the bottom margin (fully on-screen).
	f.Small.SetActive(ink.Black)
	drawCentered(f.Small, 0, W, hintY, "Tryck på en hög, välj antal, bekräfta")

	return lay
}

// drawPile renders n vertical stick-bars inside col, bottom-aligned. When the
// pile is selected, the top selCount sticks are drawn filled/marked to preview
// the removal.
func drawPile(f *Fonts, col image.Rectangle, n, maxSticks int, selected bool, selCount int) {
	if n == 0 {
		f.Med.SetActive(ink.LightGray)
		drawCentered(f.Med, col.Min.X, col.Max.X, col.Min.Y+col.Dy()/2, "tom")
		return
	}
	// Area reserved for sticks (leave room for the label at top).
	top := col.Min.Y + 60
	bot := col.Max.Y - 20
	avail := bot - top
	// Stick height scaled so the tallest pile fills the area.
	gap := 10
	stickH := (avail - gap*(maxSticks-1)) / maxSticks
	if stickH < 6 {
		stickH = 6
	}
	if stickH > 40 {
		stickH = 40
	}
	stickW := col.Dx() * 3 / 5
	sx0 := col.Min.X + (col.Dx()-stickW)/2

	// Draw from the bottom up.
	for k := 0; k < n; k++ {
		y1 := bot - k*(stickH+gap)
		y0 := y1 - stickH
		r := image.Rect(sx0, y0, sx0+stickW, y1)
		// The top selCount sticks (highest indices) are "to be removed".
		toRemove := selected && k >= n-selCount
		if toRemove {
			// marked: draw as hollow with an X-ish cross so it reads "removing".
			ink.DrawRect(r, ink.Black)
			ink.DrawLine(r.Min, r.Max, ink.Black)
			ink.DrawLine(image.Pt(r.Max.X, r.Min.Y), image.Pt(r.Min.X, r.Max.Y), ink.Black)
		} else {
			ink.FillArea(r, ink.Black)
		}
	}
}

// --- Rules screen ----------------------------------------------------------

// wrapText greedily word-wraps s to lines no wider than maxW, measured with the
// currently active font via ink.StringWidth.
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

// splitWords splits s on spaces, dropping empties.
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

// rulesParagraphs is the full Swedish rules text for Nim.
var rulesParagraphs = []string{
	"Nim spelas med flera högar av stickor. Spelarna turas om att dra. På din tur väljer du EN hög och tar bort en eller flera stickor från just den högen.",
	"Du måste ta minst en sticka, och du kan aldrig ta från fler än en hög i samma drag.",
	"Det finns två varianter:",
	"Normal: den som tar den SISTA stickan VINNER.",
	"Misère: den som tar den SISTA stickan FÖRLORAR.",
	"Lägen: Solo mot AI, där datorn spelar perfekt (optimalt) — eller 2 spelare på samma enhet (hot-seat), där ni turas om.",
	"I solo-läget är spelet ärligt: raden under rubriken visar när AI:n har ett vinstläge och när du själv kan vinna med perfekt spel.",
	"Så här drar du: tryck på en hög för att välja den, ställ in antalet stickor med − och +, och tryck sedan Bekräfta.",
}

// DrawRules renders the rules text with a "Tillbaka" button and returns its rect
// for hit-testing.
func DrawRules(size image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	ink.ClearScreen()
	W, H := size.X, size.Y

	// Title (bold, centered near the top).
	f.Big.SetActive(ink.Black)
	drawCentered(f.Big, 0, W, 70, title)

	// Body paragraphs, word-wrapped.
	f.Med.SetActive(ink.Black)
	margin := 60
	maxW := W - 2*margin
	y := 200
	lineH := 46
	paraGap := 24
	for _, p := range paragraphs {
		for _, ln := range wrapText(p, maxW) {
			ink.DrawString(image.Pt(margin, y), ln)
			y += lineH
		}
		y += paraGap
	}

	// "Tillbaka" button, centered above the bottom margin.
	bh := 110
	bw := W / 2
	r := image.Rect((W-bw)/2, H-bh-40, (W+bw)/2, H-40)
	back := Button{Rect: r, Label: "Tillbaka", ID: "back"}
	drawButton(back, f.Big, false)
	return r
}

// --- Splash screen ---------------------------------------------------------

// motifFunc draws the game's line-art motif inside the given box.
type motifFunc func(box image.Rectangle)

// DrawSplash renders the launch screen: big title, a centered line-art motif,
// and a grey "tap to start" hint.
func DrawSplash(size image.Point, f *Fonts, title string, motif motifFunc) {
	ink.ClearScreen()
	W, H := size.X, size.Y

	// Title.
	f.Title.SetActive(ink.Black)
	drawCentered(f.Title, 0, W, H/6, title)

	// Motif box: a centered square.
	side := W * 3 / 5
	box := image.Rect((W-side)/2, (H-side)/2, (W+side)/2, (H+side)/2)
	motif(box)

	// Hint.
	f.Med.SetActive(ink.DarkGray)
	drawCentered(f.Med, 0, W, H*5/6, "Tryck för att börja")
}

// drawSplashMotif draws two piles of matchsticks (3 and 5), reusing Nim's
// vertical-stick style — the heart of the game in simple line-art.
func drawSplashMotif(box image.Rectangle) {
	piles := []int{3, 5}
	n := len(piles)
	colW := box.Dx() / n

	// Scale sticks so the tallest pile fills the box height.
	maxSticks := 1
	for _, p := range piles {
		if p > maxSticks {
			maxSticks = p
		}
	}
	top := box.Min.Y + 20
	bot := box.Max.Y - 20
	avail := bot - top
	gap := 16
	stickH := (avail - gap*(maxSticks-1)) / maxSticks
	if stickH > 50 {
		stickH = 50
	}
	stickW := colW * 2 / 5

	for i, p := range piles {
		cx0 := box.Min.X + i*colW
		sx0 := cx0 + (colW-stickW)/2
		for k := 0; k < p; k++ {
			y1 := bot - k*(stickH+gap)
			y0 := y1 - stickH
			ink.FillArea(image.Rect(sx0, y0, sx0+stickW, y1), ink.Black)
		}
	}
}
