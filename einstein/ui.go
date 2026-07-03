package main

// ui.go — inkview-rendering av anteckningsrutnätet, ledtrådstext,
// svårighetsmeny och validering. Importerar cgo-`ink`; byggs i SDK-image.

import (
	"fmt"
	"image"
	"math/rand"

	ink "github.com/dennwc/inkview"
)

// screen tillstånd
type screen int

const (
	screenSplash screen = iota
	screenMenu
	screenPlay
	screenResult
	screenRules
)

// Game håller hela UI-tillståndet.
type Game struct {
	sw, sh int

	scr    screen
	puzzle Puzzle
	labels LabelSet
	notes  *NoteGrid
	rng    *rand.Rand

	// resultat
	resultCorrect bool

	// vald svårighet i menyn
	menuChoice Difficulty

	// träffytor (celler) för aktuell ritning: rektangel -> callback-data
	cellHits []cellHit
	btnHits  []btnHit

	// vald kategori-par att visa i rutnätet (medel/svår har flera par;
	// vi visar par mot ankarkategori 0 sida vid sida).
}

type cellHit struct {
	r              image.Rectangle
	ca, va, cb, vb int
}

type btnHit struct {
	r      image.Rectangle
	action string
	data   int
}

func NewGame() *Game {
	// Do NOT call ink.ScreenSize() here — this runs before ink.Run() inits the
	// screen, so it returns garbage. Seed with the Verse Pro default; Draw()
	// refreshes it from the real ScreenSize() every frame (post-init).
	g := &Game{
		sw:         1072,
		sh:         1448,
		scr:        screenSplash,
		rng:        rand.New(rand.NewSource(1)),
		menuChoice: Medium,
	}
	return g
}

// --- ink.App interface ---

func (g *Game) Init() error {
	// Queue the first Draw so hit rectangles are recorded before the first
	// tap (matches the other games; avoids a dead first tap).
	ink.Repaint()
	return nil
}
func (g *Game) Close() error { return nil }

func (g *Game) Draw() {
	// Refresh screen dimensions here — NOT in the constructor. NewGame() runs
	// before ink.Run() starts the inkview main loop, so ScreenSize() there
	// returns uninitialized (garbage) values and everything ends up mis-placed
	// off-screen. Querying in Draw() (post-init) gives the real 1072x1448.
	sz := ink.ScreenSize()
	if sz.X > 0 && sz.Y > 0 {
		g.sw, g.sh = sz.X, sz.Y
	}
	if g.sw <= 0 || g.sh <= 0 {
		g.sw, g.sh = 1072, 1448 // defensive fallback for the Verse Pro
	}

	ink.ClearScreen()
	switch g.scr {
	case screenSplash:
		g.drawSplash()
	case screenMenu:
		g.drawMenu()
	case screenPlay:
		g.drawPlay()
	case screenResult:
		g.drawResult()
	case screenRules:
		g.drawRules()
	}
	ink.FullUpdate()
}

func (g *Game) Orientation(o ink.Orientation) bool { return false }

func (g *Game) Key(e ink.KeyEvent) bool {
	if e.State != ink.KeyStateUp {
		return false
	}
	if e.Key == ink.KeyBack {
		switch g.scr {
		case screenPlay, screenResult, screenRules:
			g.scr = screenMenu
			ink.Repaint()
			return true
		}
	}
	return false
}

// På riktig PocketBook-hårdvara levereras fingertryck som EVT_POINTER*-events
// (App.Pointer()), INTE som EVT_TOUCH*. Vi hanterar därför Pointer som primär
// och Touch som reserv — båda dispatchar till samma handleTap.
func (g *Game) Pointer(e ink.PointerEvent) bool {
	if e.State != ink.PointerUp {
		return false
	}
	return g.handleTap(e.Point)
}

func (g *Game) Touch(e ink.TouchEvent) bool {
	if e.State != ink.TouchUp {
		return false
	}
	return g.handleTap(e.Point)
}

func (g *Game) handleTap(p image.Point) bool {
	switch g.scr {
	case screenSplash:
		g.scr = screenMenu
		ink.Repaint()
	case screenMenu:
		g.touchMenu(p)
	case screenPlay:
		g.touchPlay(p)
	case screenResult:
		g.touchResult(p)
	case screenRules:
		g.touchRules(p)
	}
	return true
}

// --- Meny (M4: svårighetsval) ---

func (g *Game) drawMenu() {
	g.btnHits = nil
	title := ink.OpenFont(ink.DefaultFontBold, 64, true)
	title.SetActive(ink.Black)
	ink.DrawString(image.Pt(g.sw/2-ink.StringWidth("Einsteins Gåta")/2, 160), "Einsteins Gåta")
	title.Close()

	sub := ink.OpenFont(ink.DefaultFont, 36, true)
	sub.SetActive(ink.Black)
	ink.DrawString(image.Pt(g.sw/2-ink.StringWidth("Välj svårighetsgrad")/2, 260), "Välj svårighetsgrad")
	sub.Close()

	opts := []struct {
		d     Difficulty
		label string
	}{
		{Easy, "Lätt  (3×3)"},
		{Medium, "Medel  (4×4)"},
		{Hard, "Svår  (5×5)"},
	}
	f := ink.OpenFont(ink.DefaultFontBold, 44, true)
	f.SetActive(ink.Black)
	// Knappbredd/-höjd härledda relativt skärmen med marginal till kanterna.
	// ScreenSize().Y (g.sh, ~1448) ljuger om den verkliga ritbara höjden —
	// under ~1360 wrappar innehåll till toppen. Lägg knapparna längst ner
	// bottom-up mot usableH i stället.
	const usableH = 1340
	margin := g.sw / 12
	bw := g.sw - 2*margin
	bh := g.sh / 12
	x := g.sw/2 - bw/2
	gap := bh / 2

	startW := g.sw / 3
	startH := g.sh / 11
	rh := startH

	// Stapla underifrån: Regler (botten), Starta, sedan svårighetsraderna.
	ry := usableH - margin - rh
	sy := ry - gap - startH
	lastRowY := sy - gap - bh // y för sista (nedersta) svårighetsraden

	y := lastRowY - (len(opts)-1)*(bh+gap)
	if y < 400 {
		y = 400 // behåll ett rimligt tak om det skulle bli för lågt
	}
	for _, o := range opts {
		r := image.Rect(x, y, x+bw, y+bh)
		ink.DrawRect(r, ink.Black)
		if o.d == g.menuChoice {
			ink.DrawRect(image.Rect(x-4, y-4, x+bw+4, y+bh+4), ink.Black)
		}
		ink.DrawString(image.Pt(g.sw/2-ink.StringWidth(o.label)/2, y+bh/2-10), o.label)
		g.btnHits = append(g.btnHits, btnHit{r: r, action: "difficulty", data: int(o.d)})
		y += bh + gap
	}
	f.Close()

	// Starta-knappen centrerad, håller marginal till nederkanten.
	sr := image.Rect(g.sw/2-startW/2, sy, g.sw/2+startW/2, sy+startH)
	ink.FillArea(sr, ink.Black)
	startLbl := "Starta"
	sf2 := ink.OpenFont(ink.DefaultFontBold, 48, true)
	sf2.SetActive(ink.White)
	ink.DrawString(image.Pt(g.sw/2-ink.StringWidth(startLbl)/2, sr.Min.Y+startH/2-24), startLbl)
	sf2.Close()
	g.btnHits = append(g.btnHits, btnHit{r: sr, action: "start"})

	// "Regler"-knapp under Starta (endast kontur, för att skilja från Starta).
	rr := image.Rect(g.sw/2-startW/2, ry, g.sw/2+startW/2, ry+rh)
	ink.DrawRect(rr, ink.Black)
	ink.DrawRect(image.Rect(rr.Min.X+1, rr.Min.Y+1, rr.Max.X-1, rr.Max.Y-1), ink.Black)
	rf := ink.OpenFont(ink.DefaultFontBold, 44, true)
	rf.SetActive(ink.Black)
	rlbl := "Regler"
	ink.DrawString(image.Pt(g.sw/2-ink.StringWidth(rlbl)/2, rr.Min.Y+rh/2-22), rlbl)
	rf.Close()
	g.btnHits = append(g.btnHits, btnHit{r: rr, action: "rules"})
}

func (g *Game) touchMenu(p image.Point) {
	for _, b := range g.btnHits {
		if p.In(b.r) {
			switch b.action {
			case "difficulty":
				g.menuChoice = Difficulty(b.data)
				ink.Repaint()
			case "start":
				g.startGame(g.menuChoice)
			case "rules":
				g.scr = screenRules
				ink.Repaint()
			}
			return
		}
	}
}

func (g *Game) startGame(d Difficulty) {
	g.puzzle = Generate(d, g.rng)
	g.labels = DefaultLabels(g.puzzle.N, g.puzzle.Categories)
	g.notes = NewNoteGrid(g.puzzle.N, g.puzzle.Categories)
	g.scr = screenPlay
	ink.Repaint()
}

// --- Regler (hjälpskärm) ---

var rulesParagraphs = []string{
	"Mål: lista ut den enda korrekta kombinationen — vem/vad hör ihop med vad — enbart genom logik utifrån ledtrådarna.",
	"Varje kategori (t.ex. hus, färg, djur) har lika många värden. Exakt ett värde ur varje kategori hör ihop med varje rad. Inget värde används två gånger.",
	"Rutnätet är din anteckningsyta. Varje ruta korsar två värden. Tryck på en ruta för att växla dess märke:",
	"O = dessa två värden hör ihop.  X = de hör INTE ihop.  Tom = ännu oavgjort.",
	"Sätter du ett O bör alla andra rutor i samma rad och kolumn bli X — precis som i ett logikpussel.",
	"Läs ledtrådarna under rutnätet och pricka av det du kan sluta dig till. Varje pussel har exakt en lösning och kräver ingen gissning.",
	"När allt är ifyllt: tryck Lämna in för att se om din lösning stämmer.",
}

func (g *Game) drawSplash() {
	title := ink.OpenFont(ink.DefaultFontBold, 72, true)
	title.SetActive(ink.Black)
	ink.DrawString(image.Pt(g.sw/2-ink.StringWidth("Einsteins Gåta")/2, g.sh/6), "Einsteins Gåta")
	title.Close()

	// A small logic grid with a few O / X deduction marks — the game's core.
	const n = 3
	side := g.sw * 2 / 5
	cell := side / n
	ox := g.sw/2 - cell*n/2
	oy := g.sh/2 - cell*n/2
	for i := 0; i <= n; i++ {
		x := ox + i*cell
		ink.DrawLine(image.Pt(x, oy), image.Pt(x, oy+cell*n), ink.Black)
		ink.DrawLine(image.Pt(ox, oy+i*cell), image.Pt(ox+cell*n, oy+i*cell), ink.Black)
	}
	// Marks: O at (0,0) and (2,2); X at (1,0),(0,1),(2,1),(1,2).
	mark := ink.OpenFont(ink.DefaultFontBold, cell*2/3, true)
	mark.SetActive(ink.Black)
	put := func(col, row int, s string) {
		cx := ox + col*cell + cell/2 - ink.StringWidth(s)/2
		cy := oy + row*cell + cell/2 - cell/3
		ink.DrawString(image.Pt(cx, cy), s)
	}
	put(0, 0, "O")
	put(2, 2, "O")
	put(1, 0, "X")
	put(0, 1, "X")
	put(2, 1, "X")
	put(1, 2, "X")
	mark.Close()

	hint := ink.OpenFont(ink.DefaultFont, 34, true)
	hint.SetActive(ink.DarkGray)
	ht := "Tryck för att börja"
	ink.DrawString(image.Pt(g.sw/2-ink.StringWidth(ht)/2, g.sh*5/6), ht)
	hint.Close()
}

func (g *Game) drawRules() {
	g.btnHits = nil

	title := ink.OpenFont(ink.DefaultFontBold, 56, true)
	title.SetActive(ink.Black)
	ink.DrawString(image.Pt(g.sw/2-ink.StringWidth("Einsteins Gåta — Regler")/2, 70), "Einsteins Gåta — Regler")
	title.Close()

	body := ink.OpenFont(ink.DefaultFont, 34, true)
	body.SetActive(ink.Black)
	margin := 60
	maxW := g.sw - 2*margin
	y := 190
	lineH := 46
	paraGap := 24
	for _, p := range rulesParagraphs {
		for _, ln := range wrapText(p, maxW) {
			ink.DrawString(image.Pt(margin, y), ln)
			y += lineH
		}
		y += paraGap
	}
	body.Close()

	// Tillbaka-knapp. ScreenSize().Y ljuger om verklig ritbar höjd — håll
	// mot usableH, inte g.sh.
	const usableH = 1340
	bw := g.sw / 2
	bh := g.sh / 12
	r := image.Rect(g.sw/2-bw/2, usableH-bh-60, g.sw/2+bw/2, usableH-60)
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(image.Rect(r.Min.X+1, r.Min.Y+1, r.Max.X-1, r.Max.Y-1), ink.Black)
	bf := ink.OpenFont(ink.DefaultFontBold, 44, true)
	bf.SetActive(ink.Black)
	ink.DrawString(image.Pt(g.sw/2-ink.StringWidth("Tillbaka")/2, r.Min.Y+bh/2-22), "Tillbaka")
	bf.Close()
	g.btnHits = append(g.btnHits, btnHit{r: r, action: "menu"})
}

func (g *Game) touchRules(p image.Point) {
	for _, b := range g.btnHits {
		if p.In(b.r) && b.action == "menu" {
			g.scr = screenMenu
			ink.Repaint()
			return
		}
	}
}

// wrapText delar s i rader som inte är bredare än maxW pixlar för aktiv font.
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

// --- Spelvy (M2/M3): rutnät + ledtrådar ---

func (g *Game) drawPlay() {
	g.cellHits = nil
	g.btnHits = nil
	n := g.puzzle.N
	cats := g.puzzle.Categories
	anchor := 0

	// Alla positioner härleds RELATIVT skärmstorleken så inget spiller
	// utanför [0,sw]x[0,sh]. Marginal hålls mot alla kanter.
	// OBS: ScreenSize().Y (g.sh, ~1448) ljuger om den verkliga ritbara
	// höjden — under ~1360 wrappar innehåll till toppen. Knappraden längst
	// ner hålls därför mot usableH, inte g.sh.
	const usableH = 1340
	margin := g.sw / 20 // ~54 px vid 1072 bred
	top := margin

	// Knapprad längst ner med marginal (spegel av blackbox-mönstret).
	barH := g.sh / 14 // ~103 px
	barTop := usableH - margin - barH

	// Titel överst.
	titleH := 40
	tf := ink.OpenFont(ink.DefaultFontBold, titleH, true)
	tf.SetActive(ink.Black)
	title := fmt.Sprintf("Anteckningar — %s", diffName(g.puzzle.Difficulty))
	ink.DrawString(image.Pt(margin, top), title)
	tf.Close()

	// --- Vertikal budget -------------------------------------------------
	// Ordning uppifrån: titel, kolumnrubriker, rutnät(+etiketter), ledtrådar,
	// knapprad. Ledtrådsblocket reserveras FÖRST så rutnätet aldrig ritas in
	// under det, och rutnätet skalas till det som blir kvar.
	headerH := 30 // höjd för kolumnrubriker ovanför rutnätet
	gridTop := top + titleH + margin/2 + headerH
	clueLineH := 30
	// Önskad höjd för ledtrådstexten (rubrik + alla rader), men max halva
	// den kvarvarande ytan så rutnätet alltid får plats.
	wantClueH := (len(g.puzzle.Clues) + 1) * clueLineH
	avail := barTop - gridTop - margin/2
	maxClueH := avail / 2
	clueBlockH := wantClueH
	if clueBlockH > maxClueH {
		clueBlockH = maxClueH
	}

	gridAreaTop := gridTop
	gridAreaBottom := barTop - clueBlockH - margin/2

	// Etikettkolumnen till vänster: radvärden (Röd, Grön, ...). Härled bredd
	// från skärmen; på svår behövs plats för "Blue Master"/"Pall Mall".
	labelW := g.sw / 5 // ~214 px vid 1072
	if labelW < 150 {
		labelW = 150
	}
	gridAreaLeft := margin + labelW
	gridAreaRight := g.sw - margin

	// Antal rader i rutnätet: (cats-1) block, varje med n rader.
	rows := (cats - 1) * n
	// Cellstorlek skalad så rutnätet ryms både i bredd och höjd.
	cw := (gridAreaRight - gridAreaLeft) / n
	ch := (gridAreaBottom - gridAreaTop) / rows
	cell := cw
	if ch < cell {
		cell = ch
	}
	if cell < 1 {
		cell = 1
	}

	// Vänsterjustera rutnätet direkt efter etikettkolumnen (inget stort
	// tomrum), men centrera vertikalt i tillgänglig höjd.
	gridH := cell * rows
	x0 := gridAreaLeft
	y0 := gridAreaTop + (gridAreaBottom-gridAreaTop-gridH)/2
	if y0 < gridAreaTop {
		y0 = gridAreaTop
	}

	// Radetikettfont skalas så n värden per block ryms inom blockhöjden och
	// aldrig överlappar. Basstorlek 22, krymps vid små celler.
	valFontSize := 22
	if cell < 44 {
		valFontSize = 18
	}
	if cell < 32 {
		valFontSize = 16
	}
	f := ink.OpenFont(ink.DefaultFont, valFontSize, true)
	f.SetActive(ink.Black)

	// Teckenbredd för att beskära radetiketter så de ryms i etikettkolumnen.
	labelMax := labelW / (valFontSize/2 + 1)
	if labelMax < 5 {
		labelMax = 5
	}

	// Kolumnrubriker: ankarkategorins värden, ovanför rutnätet.
	colMax := cell / (valFontSize/2 + 1)
	if colMax < 2 {
		colMax = 2
	}
	for v := 0; v < n; v++ {
		cx := x0 + v*cell
		lbl := g.labels.val(anchor, v)
		ink.DrawString(image.Pt(cx+2, y0-headerH+2), truncate(lbl, colMax))
	}

	// Rita block: varje icke-ankarkategori ger n rader. Etiketterna knyts till
	// SAMMA y som rutnätsraden så de alltid ligger i linje och inte överlappar.
	y := y0
	for c := 0; c < cats; c++ {
		if c == anchor {
			continue
		}
		for rv := 0; rv < n; rv++ {
			ry := y + rv*cell
			// radvärdesetikett, vertikalt centrerad i cellen
			ink.DrawString(image.Pt(margin, ry+cell/2-valFontSize/2), truncate(g.labels.val(c, rv), labelMax))
			for av := 0; av < n; av++ {
				cx := x0 + av*cell
				r := image.Rect(cx, ry, cx+cell, ry+cell)
				ink.DrawRect(r, ink.Black)
				sym := markSymbol(g.notes.Get(anchor, av, c, rv))
				if sym != "" {
					ink.DrawString(image.Pt(cx+cell/2-valFontSize/3, ry+cell/2-valFontSize/2), sym)
				}
				g.cellHits = append(g.cellHits, cellHit{r: r, ca: anchor, va: av, cb: c, vb: rv})
			}
		}
		y += n * cell
	}
	f.Close()

	// Ledtrådstext, bunden i det reserverade blocket ovanför knappraden.
	cf := ink.OpenFont(ink.DefaultFont, 24, true)
	cf.SetActive(ink.Black)
	cy := gridAreaBottom + margin/2
	ink.DrawString(image.Pt(margin, cy), "Ledtrådar:")
	cy += clueLineH
	clueMax := (g.sw - 2*margin) / 12         // grov teckengräns för radbredd
	clueStop := barTop - clueLineH - margin/2 // håll tydlig lucka till knappraden
	for i, c := range g.puzzle.Clues {
		if cy > clueStop {
			ink.DrawString(image.Pt(margin, cy), "…")
			break
		}
		line := fmt.Sprintf("%d. %s", i+1, ClueText(g.labels, c))
		ink.DrawString(image.Pt(margin, cy), truncate(line, clueMax))
		cy += clueLineH
	}
	cf.Close()

	// Knapprad längst ner: "Meny" till vänster, "Lämna in" till höger.
	menuW := g.sw / 4
	submitW := g.sw / 3

	mr := image.Rect(margin, barTop, margin+menuW, barTop+barH)
	ink.DrawRect(mr, ink.Black)
	mf := ink.OpenFont(ink.DefaultFontBold, 36, true)
	mf.SetActive(ink.Black)
	ink.DrawString(image.Pt(mr.Min.X+(menuW-ink.StringWidth("Meny"))/2, mr.Min.Y+barH/2-20), "Meny")
	mf.Close()
	g.btnHits = append(g.btnHits, btnHit{r: mr, action: "menu"})

	br := image.Rect(g.sw-margin-submitW, barTop, g.sw-margin, barTop+barH)
	ink.FillArea(br, ink.Black)
	bf := ink.OpenFont(ink.DefaultFontBold, 40, true)
	bf.SetActive(ink.White)
	ink.DrawString(image.Pt(br.Min.X+(submitW-ink.StringWidth("Lämna in"))/2, br.Min.Y+barH/2-24), "Lämna in")
	bf.Close()
	g.btnHits = append(g.btnHits, btnHit{r: br, action: "submit"})
}

func (g *Game) touchPlay(p image.Point) {
	for _, b := range g.btnHits {
		if p.In(b.r) {
			switch b.action {
			case "submit":
				correct, complete := g.notes.CheckAgainst(g.puzzle.Solution, 0)
				g.resultCorrect = correct && complete
				g.scr = screenResult
				ink.Repaint()
			case "menu":
				g.scr = screenMenu
				ink.Repaint()
			}
			return
		}
	}
	for _, h := range g.cellHits {
		if p.In(h.r) {
			g.notes.Cycle(h.ca, h.va, h.cb, h.vb)
			ink.PartialUpdate(h.r)
			ink.Repaint()
			return
		}
	}
}

// --- Resultatvy ---

func (g *Game) drawResult() {
	g.btnHits = nil
	msg := "Fel — försök igen!"
	if g.resultCorrect {
		msg = "Rätt! Pusslet är löst."
	}
	f := ink.OpenFont(ink.DefaultFontBold, 60, true)
	f.SetActive(ink.Black)
	ink.DrawString(image.Pt(g.sw/2-ink.StringWidth(msg)/2, g.sh/2-140), msg)
	f.Close()

	// Visa facit. Marginal till kanterna, rader beskärs så de ryms i bredd.
	margin := g.sw / 20
	sf := ink.OpenFont(ink.DefaultFont, 30, true)
	sf.SetActive(ink.Black)
	lineMax := (g.sw - 2*margin) / 13
	y := g.sh/2 - 40
	for pos := 0; pos < g.puzzle.N; pos++ {
		line := fmt.Sprintf("Plats %d:", pos+1)
		for c := 0; c < g.puzzle.Categories; c++ {
			v := int(g.puzzle.Solution.Assignment[c][pos])
			line += " " + g.labels.val(c, v)
		}
		ink.DrawString(image.Pt(margin, y), truncate(line, lineMax))
		y += 40
	}
	sf.Close()

	// Knapp längst ner, håller marginal till nederkanten. ScreenSize().Y
	// ljuger om verklig ritbar höjd — håll mot usableH, inte g.sh.
	const usableH = 1340
	bw, bh := g.sw/3, g.sh/13
	by := y + 60
	if by+bh > usableH-margin {
		by = usableH - margin - bh
	}
	br := image.Rect(g.sw/2-bw/2, by, g.sw/2+bw/2, by+bh)
	ink.FillArea(br, ink.Black)
	bf := ink.OpenFont(ink.DefaultFontBold, 44, true)
	bf.SetActive(ink.White)
	ink.DrawString(image.Pt(g.sw/2-ink.StringWidth("Till meny")/2, br.Min.Y+bh/2-22), "Till meny")
	bf.Close()
	g.btnHits = append(g.btnHits, btnHit{r: br, action: "menu"})
}

func (g *Game) touchResult(p image.Point) {
	for _, b := range g.btnHits {
		if p.In(b.r) && b.action == "menu" {
			g.scr = screenMenu
			ink.Repaint()
			return
		}
	}
}

// --- hjälpfunktioner ---

func markSymbol(m Mark) string {
	switch m {
	case Possible:
		return "O"
	case Impossible:
		return "X"
	}
	return ""
}

func diffName(d Difficulty) string {
	switch d {
	case Easy:
		return "Lätt"
	case Medium:
		return "Medel"
	default:
		return "Svår"
	}
}

func truncate(s string, max int) string {
	if max < 1 {
		max = 1
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
