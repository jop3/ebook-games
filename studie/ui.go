package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"studie/story"
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

	// A per-room illustration band sits between the header and the transcript,
	// when the current room has one and it can be seen (not in the dark).
	transTop := topMargin + headerH + 8
	if v := roomVignette(a.st.Loc); v != nil && !story.IsDark(a.st) {
		band := image.Rect(sideMargin, transTop, W-sideMargin, transTop+vigBandHeight)
		drawVignetteBand(band, v)
		transTop = band.Max.Y + 8
	}
	a.drawTranscript(W, transTop, controlTop-8)

	if a.updates == 0 || a.updates%fullUpdateEvery == 0 {
		ink.FullUpdate()
	} else {
		ink.PartialUpdate(image.Rect(0, 0, W, H))
	}
	a.updates++
}

func (a *app) drawHeader(W int) {
	top := image.Rect(0, topMargin, W, topMargin+headerH)

	// "Meny" and "Blocket" (the notebook) buttons, right-aligned.
	bw := 150
	bgap := 10
	menu := image.Rect(W-sideMargin-bw, top.Min.Y+4, W-sideMargin, top.Max.Y-4)
	book := image.Rect(menu.Min.X-bgap-bw, top.Min.Y+4, menu.Min.X-bgap, top.Max.Y-4)
	drawButton(menu, "Meny", false, a.fonts.Button)
	drawButton(book, "Blocket", false, a.fonts.Button)
	a.menuBtn = menu
	a.bookBtn = book

	// Room name, clipped so it never runs under the buttons.
	a.fonts.Header.SetActive(ink.Black)
	name := story.RoomName(a.st)
	// Trim by runes, not bytes — a byte chop can split ö and leave invalid UTF-8.
	for rs := []rune(name); ink.StringWidth(name) > book.Min.X-sideMargin-16 && len(rs) > 1; {
		rs = rs[:len(rs)-1]
		name = string(rs)
	}
	ink.DrawString(image.Pt(sideMargin, top.Min.Y+(headerH-40)/2), name)

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
	{"Undersök", story.VerbExamine},
	{"Titta", story.VerbLook},
	{"Ta", story.VerbTake},
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

// auxInventory is the payload for the sole aux button (the inventory).
const auxInventory = 0

func (a *app) drawAuxBar(x0, x1, top int) []button {
	a.fonts.Button.SetActive(ink.Black)
	bw := 360
	r := image.Rect(x0, top, x0+bw, top+bh)
	drawButton(r, "Ryggsäck", false, a.fonts.Button)
	return []button{{Rect: r, Label: "Ryggsäck", Data: auxInventory}}
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
		// A gated destination (e.g. the chemist) stays hidden until the
		// deduction that reveals the lead has been made.
		if req, gated := story.GatedExits[e.Dest]; gated && !s.Deductions[req] {
			continue
		}
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
	// Truncate by runes, not bytes, so a trimmed å/ä/ö can't leave invalid
	// UTF-8 behind.
	s := label
	if rs := []rune(label); ink.StringWidth(s) > r.Dx()-12 {
		for len(rs) > 1 && ink.StringWidth(string(rs)+"…") > r.Dx()-12 {
			rs = rs[:len(rs)-1]
		}
		s = string(rs) + "…"
	}
	drawCentered(r, s, 30)
	f.SetActive(ink.Black)
}

// --- notebook screen (the deduction pad, spec §10b) -------------------------

// clueLabel is the short Swedish tag shown for a clue in the notebook list
// (the full observation was printed to the transcript when it was examined).
var clueLabel = map[string]string{
	"body":     "Kroppen — inget sår, doft av mandel",
	"ring":     "Glasringen på skrivbordet",
	"clock":    "Klockan, ställd två timmar fel",
	"letter":   "Den brända brevlappen: '…liggare'",
	"boot":     "Fotavtrycket i kolet",
	"latch":    "Fönsterhaspen, uppbruten utifrån",
	"visitor":  "Fru Hudds vittnesmål",
	"cabtime":  "Kuskens vittnesmål",
	"ledger":   "Apotekets liggare",
	"tincture": "Den mörka flaskan (mandel)",
}

// deductionLabel is the short tag for a completed deduction.
var deductionLabel = map[string]string{
	"entry":  "Hur mördaren kom in",
	"poison": "Att det var gift",
	"motive": "Motivet",
	"timing": "Tiden för dådet",
}

// deductionOrder fixes the display order of secured deductions (maps iterate
// randomly, which would make the list jitter between frames on e-ink).
var deductionOrder = []string{"entry", "poison", "timing", "motive"}

// nbRowH is the compact clue-row height (the list can hold up to ten clues).
const nbRowH = 56

// drawNotebook renders the detective's pad: the gathered clues (tap two to
// combine), the last conclusion drawn, the deductions secured, and — at the
// bottom — the "Anklaga…" and "Tillbaka" buttons. Returns the Tillbaka rect.
func (a *app) drawNotebook(sz image.Point) image.Rectangle {
	ink.ClearScreen()
	W := sz.X

	tf := ink.OpenFont(ink.DefaultFontBold, 52, true)
	tf.SetActive(ink.Black)
	ink.DrawString(image.Pt((W-ink.StringWidth("Anteckningsbok"))/2, 34), "Anteckningsbok")
	tf.Close()

	a.fonts.Body.SetActive(ink.DarkGray)
	prog := "Slutsatser: " + itoa(story.DeductionCount(a.st)) + "/" + itoa(story.TotalDeductions()) +
		"   ·   tryck på två ledtrådar för att dra en slutsats"
	ink.DrawString(image.Pt((W-ink.StringWidth(prog))/2, 98), prog)

	x0 := sideMargin
	x1 := W - sideMargin
	y := 148

	// Clue list (compact rows).
	a.fonts.Label.SetActive(ink.DarkGray)
	ink.DrawString(image.Pt(x0, y), "LEDTRÅDAR")
	y += labelH
	a.clueBtns = nil
	if len(a.st.Clues) == 0 {
		a.fonts.Body.SetActive(ink.LightGray)
		ink.DrawString(image.Pt(x0, y), "(inga ännu — undersök platsen och hör vittnena)")
		y += nbRowH
	}
	for i, c := range a.st.Clues {
		r := image.Rect(x0, y, x1, y+nbRowH-6)
		selected := a.selClue == i
		lbl := clueLabel[c.ID]
		if lbl == "" {
			lbl = c.Text
		}
		drawButton(r, itoa(i+1)+".  "+lbl, selected, a.fonts.Label)
		a.clueBtns = append(a.clueBtns, button{Rect: r, Label: lbl, Data: i})
		y += nbRowH
	}

	// Secured deductions, one compact line.
	if story.DeductionCount(a.st) > 0 {
		y += 6
		a.fonts.Label.SetActive(ink.DarkGray)
		ink.DrawString(image.Pt(x0, y), "FASTSTÄLLT")
		y += labelH
		a.fonts.Body.SetActive(ink.Black)
		for _, id := range deductionOrder {
			if a.st.Deductions[id] {
				ink.DrawString(image.Pt(x0, y), "•  "+deductionLabel[id])
				y += lineH - 4
			}
		}
	}

	// Last conclusion drawn (wrapped), above the buttons.
	buttonsTop := usableH - bottomMargin - 100
	if a.nbMsg != "" {
		a.fonts.Label.SetActive(ink.DarkGray)
		ink.DrawString(image.Pt(x0, y+6), "SLUTSATS")
		y += labelH + 6
		a.fonts.Body.SetActive(ink.Black)
		for _, ln := range wrapText(a.nbMsg, x1-x0) {
			if y+lineH > buttonsTop { // never draw under the buttons
				break
			}
			ink.DrawString(image.Pt(x0, y), ln)
			y += lineH
		}
	}

	// Bottom: Anklaga… (left) and Tillbaka (right).
	backH := 90
	bottom := usableH - bottomMargin
	gapB := 20
	half := (x1 - x0 - gapB) / 2
	acc := image.Rect(x0, bottom-backH, x0+half, bottom)
	back := image.Rect(x1-half, bottom-backH, x1, bottom)
	drawButton(acc, "Anklaga…", false, a.fonts.Button)
	drawButton(back, "Tillbaka", false, a.fonts.Button)
	a.accuseBtn = acc
	return back
}

// --- accusation screen (spec §10d) ------------------------------------------

// drawAccuse renders the three-part charge (culprit / method / motive), each a
// list of selectable options, plus Anklaga (submit) and Tillbaka. A short
// refusal (wrong pillar or unsupported charge) shows inline; a correct, fully
// supported charge routes to the resolution screen instead. Returns the
// Tillbaka rect.
func (a *app) drawAccuse(sz image.Point) image.Rectangle {
	ink.ClearScreen()
	W := sz.X

	tf := ink.OpenFont(ink.DefaultFontBold, 52, true)
	tf.SetActive(ink.Black)
	ink.DrawString(image.Pt((W-ink.StringWidth("Anklagelse"))/2, 30), "Anklagelse")
	tf.Close()

	a.fonts.Body.SetActive(ink.DarkGray)
	sub := "Peka ut den skyldige, metoden och motivet."
	ink.DrawString(image.Pt((W-ink.StringWidth(sub))/2, 92), sub)

	x0 := sideMargin
	x1 := W - sideMargin
	y := 146

	a.accCulpritBtns, y = a.drawChoiceGroup(x0, x1, y, "DEN SKYLDIGE", story.Culprits, a.accCulprit)
	a.accMethodBtns, y = a.drawChoiceGroup(x0, x1, y, "METOD", story.Methods, a.accMethod)
	a.accMotiveBtns, y = a.drawChoiceGroup(x0, x1, y, "MOTIV", story.Motives, a.accMotive)

	// Inline refusal message (short), if any.
	buttonsTop := usableH - bottomMargin - 90
	if a.accResult != "" {
		a.fonts.Body.SetActive(ink.Black)
		yy := y + 4
		for _, ln := range wrapText(a.accResult, x1-x0) {
			if yy+lineH > buttonsTop {
				break
			}
			ink.DrawString(image.Pt(x0, yy), ln)
			yy += lineH
		}
	}

	// Bottom buttons.
	backH := 90
	bottom := usableH - bottomMargin
	gapB := 20
	half := (x1 - x0 - gapB) / 2
	ready := a.accCulprit != "" && a.accMethod != "" && a.accMotive != ""
	sub2 := image.Rect(x0, bottom-backH, x0+half, bottom)
	back := image.Rect(x1-half, bottom-backH, x1, bottom)
	drawButton(sub2, "Anklaga", false, a.fonts.Button)
	if !ready {
		ink.DrawRect(sub2, ink.LightGray) // hint: not yet complete
	}
	drawButton(back, "Tillbaka", false, a.fonts.Button)
	a.accSubmit = sub2
	return back
}

// drawChoiceGroup draws a titled column of single-select option buttons and
// returns the buttons plus the y below the group.
func (a *app) drawChoiceGroup(x0, x1, top int, title string, choices []story.Choice, sel string) ([]button, int) {
	a.fonts.Label.SetActive(ink.DarkGray)
	ink.DrawString(image.Pt(x0, top), title)
	y := top + labelH
	rowH := 60
	var out []button
	for _, c := range choices {
		r := image.Rect(x0, y, x1, y+rowH-8)
		drawButton(r, c.Label, sel == c.ID, a.fonts.Button)
		out = append(out, button{Rect: r, Label: c.ID})
		y += rowH
	}
	return out, y + 8
}

// drawResolution shows the closing scene on a solved case.
func (a *app) drawResolution(sz image.Point) image.Rectangle {
	ink.ClearScreen()
	W := sz.X

	tf := ink.OpenFont(ink.DefaultFontBold, 60, true)
	tf.SetActive(ink.Black)
	ink.DrawString(image.Pt((W-ink.StringWidth("Fallet löst"))/2, 60), "Fallet löst")
	tf.Close()

	body := ink.OpenFont(ink.DefaultFont, 34, true)
	body.SetActive(ink.Black)
	margin := 54
	y := 180
	for _, ln := range wrapText(a.resolution, W-2*margin) {
		ink.DrawString(image.Pt(margin, y), ln)
		y += 48
	}
	body.Close()

	backH := 100
	bw := W / 2
	bottom := usableH - bottomMargin
	r := image.Rect((W-bw)/2, bottom-backH, (W+bw)/2, bottom)
	drawButton(r, "Till menyn", false, a.fonts.Button)
	return r
}

// itoa is a tiny integer formatter (avoids importing strconv in the UI).
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

	drawFitTitle(W, 150, "En Studie i Grått", 72)

	a.fonts.Body.SetActive(ink.DarkGray)
	sub := "Ett fall i Londons dimma"
	ink.DrawString(image.Pt((W-ink.StringWidth(sub))/2, 250), sub)

	var items []labelled
	if a.hasSave {
		items = append(items, labelled{"Fortsätt fallet", int(menuContinue)})
	}
	items = append(items, labelled{"Nytt fall", int(menuNew)})
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
	"En man är död i ett låst rum, mitt i den gula dimman. Du är detektiven. Målet: samla ledtrådar, väg dem mot varandra, och lös fallet.",
	"Så här styr du (ingen text att skriva):",
	"• Tryck på en UTGÅNG för att förflytta dig mellan platserna.",
	"• Tryck på VERBET Undersök och sedan på ett FÖREMÅL för att granska det. Viktiga fynd skrivs in i din anteckningsbok.",
	"• Titta och Ryggsäck utförs direkt.",
	"• Öppna BLOCKET (uppe till höger). Där listas dina ledtrådar. Tryck på två av dem för att dra en SLUTSATS — stämmer de överens klarnar bilden, annars händer inget.",
	"• Samla alla slutsatser för att förstå hur, varför och av vem dådet begicks.",
	"Fallet sparas automatiskt. Välj Fortsätt fallet i menyn för att återuppta.",
	"Ett originalfall, fritt inspirerat av Sherlock Holmes debut (A Study in Scarlet, 1887, som är fri) — men med egen text, egna personer och egen intrig.",
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

// drawFitTitle draws a centered bold title at y, shrinking the font from the
// preferred size until it fits within the screen width.
func drawFitTitle(W, y int, title string, prefSize int) {
	for _, size := range []int{prefSize, prefSize - 8, prefSize - 16, prefSize - 24} {
		tf := ink.OpenFont(ink.DefaultFontBold, size, true)
		tf.SetActive(ink.Black)
		w := ink.StringWidth(title)
		if w <= W-2*sideMargin || size == prefSize-24 {
			ink.DrawString(image.Pt((W-w)/2, y), title)
			tf.Close()
			return
		}
		tf.Close()
	}
}

func DrawSplash(sz image.Point, f *Fonts, title string, motif motifFunc) {
	ink.ClearScreen()
	W, Hs := sz.X, sz.Y

	drawFitTitle(W, Hs/6, title, 76)

	side := W * 3 / 5
	box := image.Rect((W-side)/2, (usableH-side)/2, (W+side)/2, (usableH+side)/2)
	motif(box)

	hint := ink.OpenFont(ink.DefaultFont, 34, true)
	hint.SetActive(ink.DarkGray)
	ht := "Tryck för att börja"
	ink.DrawString(image.Pt((W-ink.StringWidth(ht))/2, usableH*5/6), ht)
	hint.Close()
}

// drawSplashMotif draws the mystery's emblem: a magnifying glass held over a
// boot-print in the dust — the detective's gaze on the one telling clue.
func drawSplashMotif(box image.Rectangle) {
	W, H := box.Dx(), box.Dy()

	// The boot-print, lower left, pressed into a hatched patch of dust.
	pcx := box.Min.X + W*3/8
	pcy := box.Max.Y - H/4
	dust := image.Rect(pcx-W/5, pcy-H/12, pcx+W/5, pcy+H/8)
	hatchRect(dust, 14, ink.LightGray, false)
	// sole
	sole := image.Rect(pcx-W/14, pcy-H/12, pcx+W/14, pcy+H/12)
	ink.FillArea(sole, ink.Black)
	// heel
	heel := image.Rect(pcx-W/16, pcy+H/12, pcx+W/16, pcy+H/12+H/16)
	ink.FillArea(heel, ink.Black)
	// tread marks (white notches)
	for i := 1; i < 4; i++ {
		yy := sole.Min.Y + i*sole.Dy()/4
		ink.DrawLine(image.Pt(sole.Min.X, yy), image.Pt(sole.Max.X, yy), ink.White)
	}

	// The magnifying glass, hovering upper right over the print.
	gcx := box.Min.X + W*5/8
	gcy := box.Min.Y + H*3/8
	rad := W / 5
	// lens ring (double) and inner rim
	discOutline(image.Pt(gcx, gcy), rad, ink.Black)
	discOutline(image.Pt(gcx, gcy), rad-3, ink.Black)
	discOutline(image.Pt(gcx, gcy), rad-14, ink.LightGray)
	// a glint across the glass
	arc(image.Pt(gcx, gcy), rad-24, rad-24, 120, 170, ink.LightGray)
	// handle angling down-right
	hx1 := gcx + int(float64(rad)*0.7)
	hy1 := gcy + int(float64(rad)*0.7)
	hx2 := hx1 + W/6
	hy2 := hy1 + H/6
	for o := -3; o <= 3; o++ {
		ink.DrawLine(image.Pt(hx1+o, hy1), image.Pt(hx2+o, hy2), ink.Black)
	}
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
