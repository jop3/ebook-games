package main

import (
	"fmt"
	"image"
	"strings"

	"anagram/game"

	ink "github.com/dennwc/inkview"
)

// layout margins / sizes derived from screen size at Draw time.
const (
	margin = 40

	// usableH is the real drawable height on the PB634. ink.ScreenSize()
	// reports 1448, but content below ~1360 wraps to the top of the screen
	// (guide §5). All vertical layout must derive from this, never from
	// ScreenSize().Y directly.
	usableH = 1340
)

// screenSize returns the screen size with Y clamped to the real drawable
// height (guide §5) — use this everywhere instead of ink.ScreenSize().
func screenSize() image.Point {
	sz := ink.ScreenSize()
	sz.Y = usableH
	return sz
}

// --- small drawing helpers ---

func (a *app) text(f *ink.Font, x, y int, s string) {
	f.SetActive(ink.Black)
	ink.DrawString(image.Pt(x, y), s)
}

func (a *app) textCentered(f *ink.Font, cx, y int, s string) {
	w := ink.StringWidth(s)
	a.text(f, cx-w/2, y, s)
}

func rect(x0, y0, x1, y1 int) rectangle { return rectangle{x0, y0, x1, y1} }

func (r rectangle) dx() int { return r.x1 - r.x0 }
func (r rectangle) dy() int { return r.y1 - r.y0 }

func toImgRect(r rectangle) image.Rectangle {
	return image.Rect(r.x0, r.y0, r.x1, r.y1)
}

// button draws a bordered box with a centered label and registers a hit.
func (a *app) button(f *ink.Font, r rectangle, label string, filled bool, action func()) {
	ir := toImgRect(r)
	if filled {
		ink.FillArea(ir, ink.Black)
		f.SetActive(ink.White)
	} else {
		ink.DrawRect(ir, ink.Black)
		ink.DrawRect(image.Rect(r.x0+2, r.y0+2, r.x1-2, r.y1-2), ink.Black)
		f.SetActive(ink.Black)
	}
	w := ink.StringWidth(label)
	// vertical centering: font ~ size tall; approximate.
	fh := fontHeight(f)
	tx := r.x0 + (r.x1-r.x0-w)/2
	ty := r.y0 + (r.y1-r.y0-fh)/2
	ink.DrawString(image.Pt(tx, ty), label)
	if action != nil {
		a.hits = append(a.hits, hit{r, action})
	}
	// restore black for subsequent text
	f.SetActive(ink.Black)
}

// fontHeight is a rough glyph height per font (matches the sizes we open).
func fontHeight(f *ink.Font) int {
	switch f {
	default:
		return 40
	}
}

// --- menu screen ---

func (a *app) drawMenu() {
	sz := screenSize()
	cx := sz.X / 2

	a.textCentered(a.fonts.title, cx, sz.Y/6, "ORDSKRAV")
	a.textCentered(a.fonts.body, cx, sz.Y/6+90, "Svenskt anagram-spel")

	// instructions
	lines := []string{
		"Bilda så många svenska ord",
		"du kan av de 7 bokstäverna.",
		"",
		"Tryck på bokstäver för att",
		"bygga ett ord, sedan LÄMNA IN.",
		"Poäng = ordets längd.",
		fmt.Sprintf("Ord måste vara minst %d bokstäver.", game.MinWordLen),
	}
	y := sz.Y/6 + 200
	for _, ln := range lines {
		a.textCentered(a.fonts.small, cx, y, ln)
		y += 48
	}

	// start button
	bw, bh := 560, 130
	bx := cx - bw/2
	by := sz.Y * 2 / 3
	a.button(a.fonts.button, rect(bx, by, bx+bw, by+bh), "SPELA", true, a.newRound)

	// rules button opens the full rules screen
	ry := by + bh + 30
	a.button(a.fonts.button, rect(bx, ry, bx+bw, ry+bh), "REGLER", false,
		func() { a.scr = screenRules })

	a.textCentered(a.fonts.small, cx, sz.Y-120, fmt.Sprintf("%d ord i ordlistan", a.dict.Size()))
}

// --- play screen ---

func (a *app) drawPlay() {
	sz := screenSize()
	r := a.round
	cx := sz.X / 2

	// header: score + target
	a.text(a.fonts.body, margin, margin, fmt.Sprintf("Poäng: %d", r.Score()))
	right := fmt.Sprintf("Hittade: %d/%d", len(r.Found()), r.TotalSolutions())
	a.text(a.fonts.body, sz.X-margin-ink.StringWidth(right), margin, right)

	// input row box
	inputY := margin + 90
	inputH := 110
	inBox := rect(margin, inputY, sz.X-margin, inputY+inputH)
	ink.DrawRect(toImgRect(inBox), ink.Black)
	in := strings.ToUpper(r.Input())
	if in == "" {
		a.fonts.big.SetActive(ink.LightGray)
		a.textCentered(a.fonts.big, cx, inputY+22, "…")
		a.fonts.big.SetActive(ink.Black)
	} else {
		a.textCentered(a.fonts.big, cx, inputY+22, spaced(in))
	}

	// letter tiles: single row of 7
	letters := r.Letters()
	n := len(letters)
	tilesY := inputY + inputH + 40
	gap := 18
	avail := sz.X - 2*margin
	tile := (avail - (n-1)*gap) / n
	if tile > 150 {
		tile = 150
	}
	totalW := n*tile + (n-1)*gap
	startX := (sz.X - totalW) / 2
	for i, ch := range letters {
		tx := startX + i*(tile+gap)
		tr := rect(tx, tilesY, tx+tile, tilesY+tile)
		used := r.LetterUsed(i)
		ir := toImgRect(tr)
		if used {
			// dimmed / hollow to show it's consumed
			ink.DrawRect(ir, ink.LightGray)
			a.fonts.big.SetActive(ink.LightGray)
		} else {
			ink.DrawRect(ir, ink.Black)
			ink.DrawRect(image.Rect(tr.x0+2, tr.y0+2, tr.x1-2, tr.y1-2), ink.Black)
			a.fonts.big.SetActive(ink.Black)
		}
		lbl := strings.ToUpper(string(ch))
		lw := ink.StringWidth(lbl)
		a.text0(a.fonts.big, tx+(tile-lw)/2, tilesY+(tile-56)/2, lbl)
		if !used {
			idx := i
			a.hits = append(a.hits, hit{tr, func() { a.tapLetter(idx) }})
		}
	}
	a.fonts.big.SetActive(ink.Black)

	// action buttons row
	btnY := tilesY + tile + 45
	btnH := 100
	a.threeButtons(margin, btnY, sz.X-margin, btnH,
		"BLANDA", func() { r.Shuffle(); a.status = "" },
		"RENSA", func() { r.Clear() },
		"RADERA", func() { r.Backspace() },
	)

	// submit + give up
	btn2Y := btnY + btnH + 25
	half := (sz.X - 2*margin - 30) / 2
	a.button(a.fonts.button, rect(margin, btn2Y, margin+half, btn2Y+btnH),
		"LÄMNA IN", true, a.submit)
	a.button(a.fonts.button, rect(margin+half+30, btn2Y, sz.X-margin, btn2Y+btnH),
		"GE UPP", false, func() { a.scr = screenGiveUp })

	// status line
	statusY := btn2Y + btnH + 30
	if a.status != "" {
		a.textCentered(a.fonts.small, cx, statusY, a.status)
	}

	// found words list
	listY := statusY + 55
	a.text(a.fonts.body, margin, listY, "Hittade ord:")
	a.drawFoundList(margin, listY+55, sz.X-margin, sz.Y-margin)
}

// text0 draws with the font's currently active color (no reset).
func (a *app) text0(f *ink.Font, x, y int, s string) {
	ink.DrawString(image.Pt(x, y), s)
}

func (a *app) drawFoundList(x0, y0, x1, y1 int) {
	found := a.round.Found()
	if len(found) == 0 {
		a.fonts.small.SetActive(ink.LightGray)
		a.text0(a.fonts.small, x0, y0, "(inga ännu)")
		a.fonts.small.SetActive(ink.Black)
		return
	}
	// multi-column layout
	colW := 240
	cols := (x1 - x0) / colW
	if cols < 1 {
		cols = 1
	}
	lineH := 44
	rowsPerCol := (y1 - y0) / lineH
	if rowsPerCol < 1 {
		rowsPerCol = 1
	}
	max := cols * rowsPerCol
	a.fonts.small.SetActive(ink.Black)
	for i, w := range found {
		if i >= max {
			a.text0(a.fonts.small, x0+(cols-1)*colW, y0+(rowsPerCol-1)*lineH, "…")
			break
		}
		col := i / rowsPerCol
		row := i % rowsPerCol
		s := fmt.Sprintf("%s (%d)", strings.ToUpper(w), len([]rune(w)))
		a.text0(a.fonts.small, x0+col*colW, y0+row*lineH, s)
	}
}

// threeButtons lays out three equal buttons across [x0,x1].
func (a *app) threeButtons(x0, y, x1, h int, l1 string, f1 func(), l2 string, f2 func(), l3 string, f3 func()) {
	gap := 25
	w := (x1 - x0 - 2*gap) / 3
	a.button(a.fonts.button, rect(x0, y, x0+w, y+h), l1, false, f1)
	a.button(a.fonts.button, rect(x0+w+gap, y, x0+2*w+gap, y+h), l2, false, f2)
	a.button(a.fonts.button, rect(x0+2*(w+gap), y, x1, y+h), l3, false, f3)
}

// --- give up screen ---

func (a *app) drawGiveUp() {
	sz := screenSize()
	cx := sz.X / 2
	r := a.round

	a.textCentered(a.fonts.title, cx, margin, "RESULTAT")
	a.textCentered(a.fonts.body, cx, margin+100,
		fmt.Sprintf("Poäng: %d   Ord: %d/%d", r.Score(), len(r.Found()), r.TotalSolutions()))
	a.textCentered(a.fonts.small, cx, margin+160,
		"Basord: "+strings.ToUpper(r.BaseWord()))

	// missed words (longer ones)
	a.text(a.fonts.body, margin, margin+230, "Några ord du missade:")
	missed := r.MissedWords(30)
	y := margin + 300
	colW := 300
	cols := (sz.X - 2*margin) / colW
	if cols < 1 {
		cols = 1
	}
	lineH := 48
	rows := (sz.Y - y - 260) / lineH
	if rows < 1 {
		rows = 1
	}
	a.fonts.small.SetActive(ink.Black)
	for i, w := range missed {
		if i >= cols*rows {
			break
		}
		col := i / rows
		row := i % rows
		a.text0(a.fonts.small, margin+col*colW, y+row*lineH,
			fmt.Sprintf("%s (%d)", strings.ToUpper(w), len([]rune(w))))
	}

	// buttons
	bh := 120
	by := sz.Y - margin - bh
	half := (sz.X - 2*margin - 30) / 2
	a.button(a.fonts.button, rect(margin, by, margin+half, by+bh),
		"NYTT SPEL", true, a.newRound)
	a.button(a.fonts.button, rect(margin+half+30, by, sz.X-margin, by+bh),
		"MENY", false, func() { a.scr = screenMenu })
}

// --- actions ---

func (a *app) tapLetter(i int) {
	a.round.Tap(i)
	a.status = ""
}

func (a *app) submit() {
	res, word := a.round.Submit()
	W := strings.ToUpper(word)
	switch res {
	case game.ResultAccepted:
		a.status = fmt.Sprintf("Rätt! %s +%d poäng", W, len([]rune(word)))
	case game.ResultAlreadyFound:
		a.status = W + " – redan hittat"
	case game.ResultNotAWord:
		a.status = W + " – finns inte i ordlistan"
	case game.ResultTooShort:
		a.status = fmt.Sprintf("För kort – minst %d bokstäver", game.MinWordLen)
	case game.ResultNotFormable:
		a.status = "Går inte att bilda"
	case game.ResultNotFormableEmpty:
		a.status = "Bygg ett ord först"
	}
}

// --- splash screen ---

// drawSplash renders the start screen: big title, a motif of lettered tiles
// (reusing the play screen's letter-tile look), and a "tap to start" hint.
func (a *app) drawSplash() {
	sz := screenSize()
	cx := sz.X / 2

	a.textCentered(a.fonts.title, cx, sz.Y/6, "ORDSKRAV")

	// Centered square motif box, chess-app style.
	side := sz.X * 3 / 5
	box := rect((sz.X-side)/2, (sz.Y-side)/2, (sz.X+side)/2, (sz.Y+side)/2)
	a.drawSplashMotif(box)

	// grey "tap to start" hint low on the screen.
	a.fonts.small.SetActive(ink.DarkGray)
	ht := "Tryck för att börja"
	a.text0(a.fonts.small, cx-ink.StringWidth(ht)/2, sz.Y*5/6, ht)
	a.fonts.small.SetActive(ink.Black)
}

// drawSplashMotif draws three lettered tiles ("O R D") using the same bordered
// box style as the play-screen letter tiles.
func (a *app) drawSplashMotif(box rectangle) {
	letters := []string{"O", "R", "D"}
	n := len(letters)
	tile := box.dx() / 4
	gap := tile / 4
	totalW := n*tile + (n-1)*gap
	x0 := box.x0 + (box.dx()-totalW)/2
	y0 := box.y0 + (box.dy()-tile)/2

	a.fonts.title.SetActive(ink.Black)
	for i, ch := range letters {
		tx := x0 + i*(tile+gap)
		tr := rect(tx, y0, tx+tile, y0+tile)
		ir := toImgRect(tr)
		ink.DrawRect(ir, ink.Black)
		ink.DrawRect(image.Rect(tr.x0+2, tr.y0+2, tr.x1-2, tr.y1-2), ink.Black)
		lw := ink.StringWidth(ch)
		a.text0(a.fonts.title, tx+(tile-lw)/2, y0+(tile-64)/2, ch)
	}
	a.fonts.title.SetActive(ink.Black)
}

// --- rules screen ---

var rulesParagraphs = []string{
	"Mål: bilda så många giltiga svenska ord som möjligt av de 7 slumpade bokstäverna.",
	"Tryck på bokstäverna för att bygga ett ord i inmatningsraden och tryck sedan LÄMNA IN för att kontrollera det.",
	"Poäng = ordets längd. Ett ord måste vara minst 3 bokstäver långt för att räknas.",
	"Varje bokstav får bara användas så många gånger som den visas bland de sju brickorna.",
	"RENSA tömmer inmatningsraden. RADERA tar bort den senaste bokstaven. BLANDA blandar om bokstäverna.",
	"GE UPP avslutar rundan och visar några av de ord du missade.",
	"Målet är att hitta så många ord och så hög poäng som möjligt innan du ger upp.",
}

// drawRules renders the rules text with a "Tillbaka" button.
func (a *app) drawRules() {
	sz := screenSize()
	cx := sz.X / 2

	a.textCentered(a.fonts.title, cx, 60, "REGLER")

	m := 60
	maxW := sz.X - 2*m
	y := 200
	lineH := 46
	paraGap := 26
	a.fonts.body.SetActive(ink.Black)
	for _, p := range rulesParagraphs {
		for _, ln := range wrapText(p, maxW) {
			a.text0(a.fonts.body, m, y, ln)
			y += lineH
		}
		y += paraGap
	}

	// centered "Tillbaka" button above the bottom edge.
	bw, bh := sz.X/2, 110
	br := rect((sz.X-bw)/2, sz.Y-bh-50, (sz.X+bw)/2, sz.Y-50)
	a.button(a.fonts.button, br, "TILLBAKA", true, func() { a.scr = screenMenu })
}

// wrapText greedily word-wraps s to maxW pixels, measured with the currently
// active body font via ink.StringWidth.
func wrapText(s string, maxW int) []string {
	var lines []string
	cur := ""
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

// splitWords splits s on single spaces, dropping empties.
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

// spaced inserts thin spacing between characters for readability.
func spaced(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}
