package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"grottan/story"
)

// Fonts opened once in Init and reused (never OpenFont inside Draw — guide §4).
type Fonts struct {
	Header *ink.Font
	Body   *ink.Font
	Button *ink.Font
	Label  *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Header: ink.OpenFont(ink.DefaultFontBold, 40, true),
		Body:   ink.OpenFont(ink.DefaultFont, 34, true),
		Button: ink.OpenFont(ink.DefaultFontBold, 30, true),
		Label:  ink.OpenFont(ink.DefaultFontBold, 26, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Header, f.Body, f.Button, f.Label} {
		if fn != nil {
			fn.Close()
		}
	}
}

// --- geometry (guide §5: lay out against the real 1340 drawable height) -------

const (
	usableH      = 1340
	topMargin    = 46
	sideMargin   = 24
	bottomMargin = 54
	headerH      = 72
	bh           = 74 // button height
	gap          = 14
	blockGap     = 12
	padX         = 20 // pill horizontal padding
	labelH       = 32
	lineH        = 46 // transcript line height
	scrollW      = 64 // width of the ▲/▼ scroll column
)

func pad(r image.Rectangle, n int) image.Rectangle {
	return image.Rect(r.Min.X+n, r.Min.Y+n, r.Max.X-n, r.Max.Y-n)
}

func drawCentered(r image.Rectangle, s string, glyphH int) {
	x := r.Min.X + (r.Dx()-ink.StringWidth(s))/2
	y := r.Min.Y + (r.Dy()-glyphH)/2
	ink.DrawString(image.Pt(x, y), s)
}

// --- game screen ------------------------------------------------------------

func (a *app) drawGame(sz image.Point) {
	W := sz.X
	H := usableH
	ink.ClearScreen()

	a.drawHeader(W)
	controlTop := a.drawControls(W, H)
	a.drawTranscript(W, topMargin+headerH+8, controlTop-8)

	if a.sayOpen {
		a.drawSayPopup(W, controlTop)
	} else {
		a.sayBtns = nil
	}

	if a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(image.Rect(0, 0, W, H))
	}
	a.updates++
}

func (a *app) drawHeader(W int) {
	top := image.Rect(0, topMargin, W, topMargin+headerH)
	a.fonts.Header.SetActive(ink.Black)
	name := story.RoomName(a.st)
	ink.DrawString(image.Pt(sideMargin, top.Min.Y+(headerH-40)/2), name)

	// "Meny" button, right-aligned.
	mw := 150
	mb := image.Rect(W-sideMargin-mw, top.Min.Y+4, W-sideMargin, top.Max.Y-4)
	drawButton(mb, "Meny", false, a.fonts.Button)
	a.menuBtn = mb

	ink.DrawLine(image.Pt(0, top.Max.Y), image.Pt(W, top.Max.Y), ink.Black)
}

// drawControls lays the exits / nouns / verbs / aux block bottom-anchored and
// returns the y of its top edge so the transcript can stop above it.
func (a *app) drawControls(W, H int) int {
	x0, x1 := sideMargin, W-sideMargin

	exitBlockH := labelH + 2*bh + gap
	nounBlockH := labelH + 2*bh + gap
	controlH := exitBlockH + nounBlockH + bh + bh + 3*blockGap
	top := H - bottomMargin - controlH

	y := top
	// Exits.
	a.exitBtns = a.drawGroup(x0, x1, y, "UTGÅNGAR", exitLabels(a.st), 2, groupExit)
	y += exitBlockH + blockGap
	// Nouns (room + carried objects).
	a.nounBtns = a.drawGroup(x0, x1, y, "FÖREMÅL", nounLabels(a.st), 2, groupNoun)
	y += nounBlockH + blockGap
	// Verb bar (fixed row of six).
	a.verbBtns = a.drawVerbBar(x0, x1, y)
	y += bh + blockGap
	// Aux row.
	a.auxBtns = a.drawAuxBar(x0, x1, y)

	return top
}

type groupKind int

const (
	groupExit groupKind = iota
	groupNoun
)

type labelled struct {
	text string
	data int
}

// drawGroup draws a titled row of pill buttons that flow left-to-right and wrap
// to at most maxRows rows (extras are dropped, keeping the block height fixed).
func (a *app) drawGroup(x0, x1, top int, title string, items []labelled, maxRows int, kind groupKind) []button {
	a.fonts.Label.SetActive(ink.DarkGray)
	ink.DrawString(image.Pt(x0, top), title)

	a.fonts.Button.SetActive(ink.Black)
	rowTop := top + labelH
	x, row := x0, 0
	var out []button
	for _, it := range items {
		w := ink.StringWidth(it.text) + 2*padX
		if w > x1-x0 {
			w = x1 - x0
		}
		if x+w > x1 { // wrap
			row++
			if row >= maxRows {
				break
			}
			x = x0
		}
		r := image.Rect(x, rowTop+row*(bh+gap), x+w, rowTop+row*(bh+gap)+bh)
		drawButton(r, it.text, false, a.fonts.Button)
		out = append(out, button{Rect: r, Label: it.text, Data: it.data})
		x += w + gap
	}
	if len(items) == 0 {
		a.fonts.Body.SetActive(ink.LightGray)
		ink.DrawString(image.Pt(x0, rowTop+(bh-34)/2), placeholder(kind))
	}
	return out
}

func placeholder(kind groupKind) string {
	if kind == groupExit {
		return "(inga synliga utgångar)"
	}
	return "(inga föremål här)"
}

var verbDefs = []struct {
	label string
	verb  story.Verb
}{
	{"Titta", story.VerbLook},
	{"Undersök", story.VerbExamine},
	{"Ta", story.VerbTake},
	{"Släpp", story.VerbDrop},
	{"Lås upp", story.VerbUnlock},
	{"Tänd", story.VerbLight},
}

func (a *app) drawVerbBar(x0, x1, top int) []button {
	a.fonts.Button.SetActive(ink.Black)
	n := len(verbDefs)
	vgap := 10
	bw := (x1 - x0 - (n-1)*vgap) / n
	var out []button
	for i, vd := range verbDefs {
		x := x0 + i*(bw+vgap)
		r := image.Rect(x, top, x+bw, top+bh)
		armed := a.verbArmed && a.armedVerb == vd.verb
		drawButton(r, vd.label, armed, a.fonts.Button)
		out = append(out, button{Rect: r, Label: vd.label, Data: int(vd.verb), Armed: armed})
	}
	return out
}

func (a *app) drawAuxBar(x0, x1, top int) []button {
	a.fonts.Button.SetActive(ink.Black)
	labels := []labelled{{"Ryggsäck", auxInventory}}
	if len(story.KnownMagicWords(a.st)) > 0 {
		labels = append(labels, labelled{"Säg…", auxSay})
	}
	n := len(labels)
	vgap := 10
	bw := (x1 - x0 - (n-1)*vgap) / n
	if bw > 360 {
		bw = 360
	}
	var out []button
	for i, l := range labels {
		x := x0 + i*(bw+vgap)
		r := image.Rect(x, top, x+bw, top+bh)
		armed := l.data == auxSay && a.sayOpen
		drawButton(r, l.text, armed, a.fonts.Button)
		out = append(out, button{Rect: r, Label: l.text, Data: l.data, Armed: armed})
	}
	return out
}

// drawSayPopup floats the discovered magic words above the control block.
func (a *app) drawSayPopup(W, controlTop int) {
	words := story.KnownMagicWords(a.st)
	rowH := bh + gap
	boxH := labelH + len(words)*rowH + gap
	box := image.Rect(W/4, controlTop-boxH-8, 3*W/4, controlTop-8)
	ink.FillArea(box, ink.White)
	ink.DrawRect(box, ink.Black)
	ink.DrawRect(pad(box, 1), ink.Black)

	a.fonts.Label.SetActive(ink.DarkGray)
	ink.DrawString(image.Pt(box.Min.X+padX, box.Min.Y+6), "Säg ett ord:")

	a.fonts.Button.SetActive(ink.Black)
	var out []button
	y := box.Min.Y + labelH
	for _, m := range words {
		r := image.Rect(box.Min.X+padX, y, box.Max.X-padX, y+bh)
		drawButton(r, story.MagicWordText[m], false, a.fonts.Button)
		out = append(out, button{Rect: r, Label: story.MagicWordText[m], Data: int(m)})
		y += rowH
	}
	a.sayBtns = out
}

func (a *app) drawTranscript(W, top, bottom int) {
	area := image.Rect(sideMargin, top, W-sideMargin-scrollW, bottom)
	a.fonts.Body.SetActive(ink.Black)
	maxW := area.Dx()

	// Wrap the logical transcript into display lines.
	var lines []string
	for _, entry := range a.log {
		if entry == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, wrapText(entry, maxW)...)
	}

	visN := area.Dy() / lineH
	maxScroll := len(lines) - visN
	if maxScroll < 0 {
		maxScroll = 0
	}
	if a.stickTail {
		a.scroll = maxScroll
	}
	if a.scroll > maxScroll {
		a.scroll = maxScroll
	}
	if a.scroll < 0 {
		a.scroll = 0
	}

	y := area.Min.Y
	for i := a.scroll; i < len(lines) && i < a.scroll+visN; i++ {
		if lines[i] != "" {
			ink.DrawString(image.Pt(area.Min.X, y), lines[i])
		}
		y += lineH
	}

	// Scroll buttons ▲ / ▼ in the right column.
	colX0 := W - sideMargin - scrollW + 8
	colX1 := W - sideMargin
	up := image.Rect(colX0, top, colX1, top+bh)
	dn := image.Rect(colX0, bottom-bh, colX1, bottom)
	drawButton(up, "▲", false, a.fonts.Button)
	drawButton(dn, "▼", false, a.fonts.Button)
	a.scrollUp, a.scrollDown = up, dn
}

// --- exit / noun label sources ----------------------------------------------

func exitLabels(s *story.State) []labelled {
	var out []labelled
	for _, e := range story.Exits(s) {
		out = append(out, labelled{text: e.Label, data: int(e.Motion)})
	}
	return out
}

func nounLabels(s *story.State) []labelled {
	var out []labelled
	for _, id := range story.VisibleObjects(s) {
		out = append(out, labelled{text: nounWord(id), data: int(id)})
	}
	for _, id := range story.CarriedObjects(s) {
		out = append(out, labelled{text: "•" + nounWord(id), data: int(id)})
	}
	return out
}

// nounWord is the short, capitalized button label for an object.
func nounWord(id story.ObjID) string {
	w := story.Objects[id].Words
	if len(w) == 0 {
		return "?"
	}
	return capitalize(w[0])
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	b := []rune(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 32
	}
	return string(b)
}

// --- shared button drawing --------------------------------------------------

func drawButton(r image.Rectangle, label string, armed bool, f *ink.Font) {
	if armed {
		ink.FillArea(r, ink.Black)
		f.SetActive(ink.White)
	} else {
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		f.SetActive(ink.Black)
	}
	// Fit the label; shrink obvious overflows by ellipsizing (guide §5a).
	s := label
	for ink.StringWidth(s) > r.Dx()-12 && len(s) > 1 {
		s = s[:len(s)-1]
	}
	if s != label && len(s) > 1 {
		s = s[:len(s)-1] + "…"
	}
	drawCentered(r, s, 30)
	f.SetActive(ink.Black)
}

// --- menu -------------------------------------------------------------------

type menuAction int

const (
	menuContinue menuAction = iota
	menuNew
	menuRules
)

func (a *app) drawMenu(sz image.Point) {
	ink.ClearScreen()
	W := sz.X

	a.fonts.Header.SetActive(ink.Black)
	title := ink.OpenFont(ink.DefaultFontBold, 84, true)
	title.SetActive(ink.Black)
	tw := ink.StringWidth("Grottan")
	ink.DrawString(image.Pt((W-tw)/2, 150), "Grottan")
	title.Close()

	a.fonts.Body.SetActive(ink.DarkGray)
	sub := "Ett äventyr i Colossal Cave"
	ink.DrawString(image.Pt((W-ink.StringWidth(sub))/2, 270), sub)

	var items []labelled
	if a.hasSave {
		items = append(items, labelled{"Fortsätt", int(menuContinue)})
	}
	items = append(items, labelled{"Nytt spel", int(menuNew)})
	items = append(items, labelled{"Regler", int(menuRules)})

	a.fonts.Button.SetActive(ink.Black)
	bw := W / 2
	x0 := (W - bw) / 2
	y := 420
	rowH := 120
	var out []button
	for _, it := range items {
		r := image.Rect(x0, y, x0+bw, y+bh+10)
		drawButton(r, it.text, false, a.fonts.Button)
		out = append(out, button{Rect: r, Label: it.text, Data: it.data})
		y += rowH
	}
	a.menuBtns = out
}

// --- rules ------------------------------------------------------------------

var rulesParagraphs = []string{
	"Du utforskar Colossal Cave — den klassiska grottan full av gångar, föremål och hemligheter. Målet i denna del: ta dig ner i grottan, hitta guldklimpen och andra skatter, och lär känna vägarna.",
	"Så här styr du (ingen text att skriva):",
	"• Tryck på en UTGÅNG för att gå åt det hållet.",
	"• Tryck på ett VERB (t.ex. Ta) så tänds det. Tryck sedan på ett FÖREMÅL för att utföra handlingen. Tryck på verbet igen för att ångra.",
	"• Titta och Ryggsäck utförs direkt — de behöver inget föremål.",
	"• Föremål märkta med • bär du på dig; övriga ligger i rummet.",
	"• Säg… visar magiska ord du har upptäckt. Prova dem!",
	"• Grottan är mörk. Du behöver en tänd lampa för att se — annars kan du falla.",
	"Spelet sparas automatiskt efter varje drag. Välj Fortsätt i menyn för att återuppta.",
	"Baserat på Colossal Cave Adventure av Will Crowther & Don Woods, via Open Adventure (Eric S. Raymond), BSD-2-Clause.",
}

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

// DrawRules renders the Swedish rules with a back button and returns its rect.
func DrawRules(sz image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	ink.ClearScreen()
	W := sz.X

	tf := ink.OpenFont(ink.DefaultFontBold, 56, true)
	tf.SetActive(ink.Black)
	ink.DrawString(image.Pt((W-ink.StringWidth(title))/2, 40), title)
	tf.Close()

	body := ink.OpenFont(ink.DefaultFont, 30, true)
	body.SetActive(ink.Black)
	margin := 54
	maxW := W - 2*margin
	y := 140
	rlineH := 42
	paraGap := 16
	for _, p := range paragraphs {
		for _, ln := range wrapText(p, maxW) {
			ink.DrawString(image.Pt(margin, y), ln)
			y += rlineH
		}
		y += paraGap
	}
	body.Close()

	bhh := 100
	bw := W / 2
	bottom := usableH - bottomMargin
	r := image.Rect((W-bw)/2, bottom-bhh, (W+bw)/2, bottom)
	drawButton(r, "Tillbaka", false, f.Button)
	return r
}

// --- splash -----------------------------------------------------------------

type motifFunc func(box image.Rectangle)

func DrawSplash(sz image.Point, f *Fonts, title string, motif motifFunc) {
	ink.ClearScreen()
	W, Hs := sz.X, sz.Y

	tf := ink.OpenFont(ink.DefaultFontBold, 80, true)
	tf.SetActive(ink.Black)
	ink.DrawString(image.Pt((W-ink.StringWidth(title))/2, Hs/6), title)
	tf.Close()

	side := W * 3 / 5
	box := image.Rect((W-side)/2, (usableH-side)/2, (W+side)/2, (usableH+side)/2)
	motif(box)

	hint := ink.OpenFont(ink.DefaultFont, 34, true)
	hint.SetActive(ink.DarkGray)
	ht := "Tryck för att börja"
	ink.DrawString(image.Pt((W-ink.StringWidth(ht))/2, usableH*5/6), ht)
	hint.Close()
}

// drawSplashMotif draws the cave-mouth / grate icon: an arched opening with
// vertical grate bars and a hanging lantern — the game's central image.
func drawSplashMotif(box image.Rectangle) {
	cx := (box.Min.X + box.Max.X) / 2
	w := box.Dx() * 3 / 4
	archLeft := cx - w/2
	archRight := cx + w/2
	baseY := box.Max.Y - box.Dy()/6
	springY := box.Min.Y + box.Dy()/3 // where the arch starts to curve
	radius := w / 2

	thick := func(p1, p2 image.Point) {
		for o := -1; o <= 1; o++ {
			ink.DrawLine(p1.Add(image.Pt(o, 0)), p2.Add(image.Pt(o, 0)), ink.Black)
		}
	}

	// Cave mouth: two jambs and a semicircular arch.
	thick(image.Pt(archLeft, baseY), image.Pt(archLeft, springY))
	thick(image.Pt(archRight, baseY), image.Pt(archRight, springY))
	prev := image.Pt(archLeft, springY)
	for deg := 0; deg <= 180; deg += 6 {
		rad := float64(deg) * 3.14159265 / 180.0
		x := cx - int(float64(radius)*cos(rad))
		y := springY - int(float64(radius)*sin(rad))
		p := image.Pt(x, y)
		thick(prev, p)
		prev = p
	}
	thick(prev, image.Pt(archRight, springY))

	// Vertical grate bars across the opening.
	bars := 5
	for i := 1; i <= bars; i++ {
		x := archLeft + i*w/(bars+1)
		topY := springY - isqrt(radius*radius-(x-cx)*(x-cx))
		thick(image.Pt(x, topY), image.Pt(x, baseY))
	}
	// Two crossbars.
	thick(image.Pt(archLeft, springY+box.Dy()/12), image.Pt(archRight, springY+box.Dy()/12))
	thick(image.Pt(archLeft, baseY-box.Dy()/12), image.Pt(archRight, baseY-box.Dy()/12))

	// Ground line.
	thick(image.Pt(archLeft-w/8, baseY), image.Pt(archRight+w/8, baseY))

	// A small lantern glyph hanging above the arch.
	lx, ly := cx, box.Min.Y+box.Dy()/8
	ink.DrawLine(image.Pt(lx, box.Min.Y), image.Pt(lx, ly), ink.Black)
	lamp := image.Rect(lx-box.Dx()/22, ly, lx+box.Dx()/22, ly+box.Dy()/12)
	ink.DrawRect(lamp, ink.Black)
	ink.DrawRect(pad(lamp, 1), ink.Black)
	ink.FillArea(pad(lamp, lamp.Dx()/3), ink.Black)
}

// small math helpers (no float import churn in the hot path).
func cos(x float64) float64 { return sinApprox(x + 1.5707963) }
func sin(x float64) float64 { return sinApprox(x) }

// sinApprox is a compact Taylor/Bhaskara sine good enough for icon line-art.
func sinApprox(x float64) float64 {
	// reduce to [-pi, pi]
	const pi = 3.14159265358979
	for x > pi {
		x -= 2 * pi
	}
	for x < -pi {
		x += 2 * pi
	}
	// Bhaskara I approximation.
	neg := false
	if x < 0 {
		x = -x
		neg = true
	}
	y := 16 * x * (pi - x) / (5*pi*pi - 4*x*(pi-x))
	if neg {
		return -y
	}
	return y
}

func isqrt(n int) int {
	if n <= 0 {
		return 0
	}
	x := n
	y := (x + 1) / 2
	for y < x {
		x = y
		y = (x + n/x) / 2
	}
	return x
}
