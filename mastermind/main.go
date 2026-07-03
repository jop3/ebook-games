package main

import (
	"image"
	"math/rand"
	"strconv"
	"time"

	ink "github.com/dennwc/inkview"
)

type screen int

const (
	screenSplash screen = iota
	screenMenu
	screenGame
	screenRules
	screenKnuth // "Enheten gissar" — device guesses, player gives feedback
)

// App implements ink.App for Mastermind.
type App struct {
	rng    *rand.Rand
	scr    screen
	game   *GameState
	draft  Guess  // current in-progress guess on the game screen
	sel    int    // selected color in the palette (0..Colors-1), -1 = none picked yet
	drawsN int    // number of Draw calls, used to trigger periodic FullUpdate
	fonts  *Fonts // typefaces opened once in Init, reused every Draw

	// cached hit rectangles built during Draw, consumed by Touch.
	menuButtons []button
	pegRects    []image.Rectangle
	paletteRect []image.Rectangle
	confirmBtn  button
	newBtn      button
	backBtn     button
	rulesBtn    button // "Regler" on the menu
	rulesBack   button // "Tillbaka" on the rules screen

	// Knuth ("Enheten gissar") mode state.
	knuth      *KnuthSolver
	kfbBlack   int  // player's in-progress black-feedback count for current guess
	kfbWhite   int  // player's in-progress white-feedback count
	aiPending  bool // true after a confirm: compute the next guess on next Draw

	// cached hit rectangles for the Knuth screen.
	knuthBtn    button // "Enheten gissar" on the menu
	kBlackMinus button
	kBlackPlus  button
	kWhiteMinus button
	kWhitePlus  button
	kConfirm    button
	kNew        button
	kBack       button
}

func newApp() *App {
	return &App{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
		scr: screenSplash,
		sel: 0,
	}
}

func (a *App) Init() error {
	a.fonts = InitFonts()
	// Queue the first Draw so the menu's hit rectangles (menuButtons) are
	// recorded before the user's first tap. Without this, the first tap lands
	// with no hit-rects and does nothing (you'd have to tap twice) — the other
	// games all call Repaint here.
	ink.Repaint()
	return nil
}

func (a *App) Close() error {
	if a.fonts != nil {
		a.fonts.Close()
		a.fonts = nil
	}
	return nil
}

func (a *App) Orientation(o ink.Orientation) bool { return false }

// Pointer receives finger taps on PocketBook hardware. Normal single taps
// arrive as EVT_POINTER* (not EVT_TOUCH*), so this is where interaction lands.
func (a *App) Pointer(e ink.PointerEvent) bool {
	if e.State != ink.PointerUp {
		return false
	}
	return a.handleTap(e.Point)
}

// handleTap dispatches a tap at point p to the active screen.
func (a *App) handleTap(p image.Point) bool {
	switch a.scr {
	case screenSplash:
		a.scr = screenMenu
		ink.Repaint()
		return true
	case screenMenu:
		return a.touchMenu(p)
	case screenGame:
		return a.touchGame(p)
	case screenKnuth:
		return a.touchKnuth(p)
	case screenRules:
		if a.rulesBack.hit(p) {
			a.scr = screenMenu
			ink.Repaint()
			return true
		}
	}
	return false
}

// Key handles hardware keys: Back returns to menu / exits.
func (a *App) Key(e ink.KeyEvent) bool {
	if e.State != ink.KeyStateUp {
		return false
	}
	if e.Key == ink.KeyBack {
		if a.scr == screenGame || a.scr == screenRules || a.scr == screenKnuth {
			a.scr = screenMenu
			ink.Repaint()
			return true
		}
		ink.Exit()
		return true
	}
	return false
}

// Touch is kept as a fallback for firmware that emits multitouch EVT_TOUCH*
// events instead of pointer events. It routes to the same logic.
func (a *App) Touch(e ink.TouchEvent) bool {
	if e.State != ink.TouchUp {
		return false
	}
	return a.handleTap(e.Point)
}

func (a *App) touchMenu(p image.Point) bool {
	if a.rulesBtn.hit(p) {
		a.scr = screenRules
		ink.Repaint()
		return true
	}
	if a.knuthBtn.hit(p) {
		a.startKnuth()
		return true
	}
	for i, b := range a.menuButtons {
		if b.hit(p) {
			a.startGame(Presets[i])
			return true
		}
	}
	return false
}

func (a *App) startGame(cfg Config) {
	a.game = NewGame(cfg, a.rng)
	a.draft = make(Guess, cfg.Pegs)
	for i := range a.draft {
		a.draft[i] = -1 // -1 = empty
	}
	a.sel = 0
	a.scr = screenGame
	ink.FullUpdate()
	ink.Repaint()
}

// knuthCfg is the configuration used for the "Enheten gissar" mode. It is the
// classic 4-peg / 6-color game — the only size where Knuth's minimax stays
// fast enough per move on the device (see KnuthAllowed / knuth.go).
var knuthCfg = Config{Name: "Klassisk", Pegs: 4, Colors: 6, MaxGuesses: 10, AllowRepeat: true}

func (a *App) startKnuth() {
	a.knuth = NewKnuthSolver(knuthCfg)
	a.kfbBlack = 0
	a.kfbWhite = 0
	a.aiPending = false
	a.scr = screenKnuth
	ink.FullUpdate()
	ink.Repaint()
}

// touchKnuth handles taps on the "Enheten gissar" screen: black/white
// steppers, Confirm (records feedback and asks the solver for the next guess),
// New (restart), and Meny (back).
func (a *App) touchKnuth(p image.Point) bool {
	if a.kBack.hit(p) {
		a.scr = screenMenu
		ink.Repaint()
		return true
	}
	if a.kNew.hit(p) {
		a.startKnuth()
		return true
	}

	s := a.knuth
	if s == nil {
		return false
	}
	// Once solved or contradictory, only New / Meny are active.
	if s.Solved() || s.Impossible() || a.aiPending {
		return false
	}

	pegs := s.CurrentGuess()
	n := len(pegs)

	if a.kBlackMinus.hit(p) {
		if a.kfbBlack > 0 {
			a.kfbBlack--
		}
		ink.Repaint()
		return true
	}
	if a.kBlackPlus.hit(p) {
		if a.kfbBlack+a.kfbWhite < n {
			a.kfbBlack++
		}
		ink.Repaint()
		return true
	}
	if a.kWhiteMinus.hit(p) {
		if a.kfbWhite > 0 {
			a.kfbWhite--
		}
		ink.Repaint()
		return true
	}
	if a.kWhitePlus.hit(p) {
		if a.kfbBlack+a.kfbWhite < n {
			a.kfbWhite++
		}
		ink.Repaint()
		return true
	}
	if a.kConfirm.hit(p) {
		// Record the feedback; the heavy minimax runs on the next Draw so the
		// UI can first paint a "Räknar…" hint (aiPending pattern).
		a.aiPending = true
		ink.FullUpdate()
		ink.Repaint()
		return true
	}
	return false
}

func (a *App) touchGame(p image.Point) bool {
	// Back / New buttons.
	if a.backBtn.hit(p) {
		a.scr = screenMenu
		ink.Repaint()
		return true
	}
	if a.newBtn.hit(p) {
		a.startGame(a.game.Cfg)
		return true
	}

	if a.game.Status == Playing {
		// Palette selection.
		for i, r := range a.paletteRect {
			if p.In(r) {
				a.sel = i
				ink.Repaint()
				return true
			}
		}
		// Peg placement: set the tapped peg to the selected color.
		for i, r := range a.pegRects {
			if p.In(r) {
				a.draft[i] = Color(a.sel)
				ink.Repaint()
				return true
			}
		}
		// Confirm.
		if a.confirmBtn.hit(p) && a.draftFull() {
			a.game.Submit(append(Guess(nil), a.draft...))
			for i := range a.draft {
				a.draft[i] = -1
			}
			ink.FullUpdate()
			ink.Repaint()
			return true
		}
	}
	return false
}

func (a *App) draftFull() bool {
	for _, c := range a.draft {
		if c < 0 {
			return false
		}
	}
	return true
}

func (a *App) Draw() {
	a.drawsN++
	ink.ClearScreen()
	switch a.scr {
	case screenSplash:
		a.drawSplash()
	case screenMenu:
		a.drawMenu()
	case screenGame:
		a.drawGame()
	case screenKnuth:
		a.drawKnuth()
	case screenRules:
		a.drawRules()
	}

	// aiPending: the screen was drawn once showing "Räknar…"; now do the heavy
	// minimax and queue another Draw with the fresh guess. This keeps the UI
	// from freezing between the tap and the result.
	if a.aiPending && a.scr == screenKnuth {
		a.aiPending = false
		a.knuth.Feedback(Feedback{Black: a.kfbBlack, White: a.kfbWhite})
		a.kfbBlack = 0
		a.kfbWhite = 0
		ink.FullUpdate()
		ink.Repaint()
		return
	}
	// Periodic full refresh to clear e-ink ghosting.
	if a.drawsN%8 == 0 {
		ink.FullUpdate()
	}
}

func (a *App) drawMenu() {
	f := a.fonts
	drawTitle(f.TitleBig, "Huvudbry: Mastermind", titleY)
	drawTitle(f.MenuSub, "Välj svårighetsgrad", titleY+80)

	sz := ink.ScreenSize()
	a.menuButtons = a.menuButtons[:0]
	top := 320
	bw := sz.X - 2*margin
	bh := 160
	gap := 40
	for i, cfg := range Presets {
		y := top + i*(bh+gap)
		rect := image.Rect(margin, y, margin+bw, y+bh)
		b := button{rect: rect, label: cfg.Name, hot: true}
		b.draw(f.Button)
		// Sub-line with details.
		detail := strconv.Itoa(cfg.Pegs) + " pinnar, " + strconv.Itoa(cfg.Colors) +
			" färger, " + strconv.Itoa(cfg.MaxGuesses) + " försök"
		if !cfg.AllowRepeat {
			detail += ", inga uppr."
		}
		drawTextAt(f.Detail, image.Pt(rect.Min.X+30, rect.Min.Y+90), detail)
		a.menuButtons = append(a.menuButtons, b)
	}

	// "Regler" and "Enheten gissar" buttons below the presets, side by side.
	rbTop := top + len(Presets)*(bh+gap) + 10
	half := (bw - gap) / 2
	rb := image.Rect(margin, rbTop, margin+half, rbTop+110)
	a.rulesBtn = button{rect: rb, label: "Regler", hot: true}
	a.rulesBtn.draw(f.Button)

	kb := image.Rect(margin+half+gap, rbTop, margin+2*half+gap, rbTop+110)
	a.knuthBtn = button{rect: kb, label: "Enheten gissar", hot: true}
	a.knuthBtn.draw(f.Button)
}

// rulesParagraphs is the Mastermind rules text, one entry per paragraph.
var rulesParagraphs = []string{
	"Mål: knäck den hemliga koden av färgade pinnar på så få försök som möjligt.",
	"Datorn slumpar en dold kod. Varje färg visas som ett eget svartvitt mönster (e-läsaren saknar färg) — samma mönster i paletten som på brädet.",
	"Så gissar du: välj en färg i paletten och tryck på en position i raden. Fyll hela raden och tryck Bekräfta.",
	"Efter varje gissning får du ledtrådar:",
	"Fylld markör: rätt färg på rätt plats.",
	"Ihålig markör: rätt färg men fel plats.",
	"Ledtrådarna säger INTE vilken pinne som är rätt — bara hur många. Använd flera gissningar för att ringa in koden.",
	"I vissa svårighetsgrader kan samma färg förekomma flera gånger. Det står i varje varibeskrivning på menyn.",
}

func (a *App) drawSplash() {
	f := a.fonts
	sz := ink.ScreenSize()

	tf := ink.OpenFont(ink.DefaultFontBold, 72, true)
	tf.SetActive(ink.Black)
	ink.DrawString(image.Pt(sz.X/2-ink.StringWidth("Mastermind")/2, sz.Y/6), "Mastermind")
	tf.Close()

	// A centered row of four code pegs with distinct greyscale patterns, plus a
	// small feedback-peg row beneath — the essence of Mastermind.
	cy := sz.Y/2 - sz.Y/16
	r := sz.X / 12
	gap := r
	n := 4
	totalW := n*2*r + (n-1)*gap
	x := sz.X/2 - totalW/2 + r
	for i := 0; i < n; i++ {
		splashPeg(image.Pt(x, cy), r, i)
		x += 2*r + gap
	}

	// Feedback pegs (two filled, two hollow) below.
	fy := cy + 2*r + r
	fr := r / 2
	fgap := fr
	ftotal := n*2*fr + (n-1)*fgap
	fx := sz.X/2 - ftotal/2 + fr
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			splashDisc(fx, fy, fr, true)
		} else {
			splashRing(fx, fy, fr)
		}
		fx += 2*fr + fgap
	}

	hint := ink.OpenFont(ink.DefaultFont, 34, true)
	hint.SetActive(ink.LightGray)
	ht := "Tryck för att börja"
	ink.DrawString(image.Pt(sz.X/2-ink.StringWidth(ht)/2, sz.Y*5/6), ht)
	hint.Close()
	_ = f
}

// splashPeg draws a code peg with a distinct pattern per style index.
func splashPeg(c image.Point, r, style int) {
	switch style % 4 {
	case 0:
		splashDisc(c.X, c.Y, r, true) // solid
	case 1:
		splashRing(c.X, c.Y, r) // ring
	case 2:
		splashDisc(c.X, c.Y, r, true) // solid with white dot
		splashDisc(c.X, c.Y, r/3, false)
	case 3:
		// half-filled: ring plus a filled inner core
		splashRing(c.X, c.Y, r)
		splashDisc(c.X, c.Y, r/2, true)
	}
}

// splashDisc fills (black=true) or clears (black=false) a disc.
func splashDisc(cx, cy, rad int, black bool) {
	col := ink.Black
	if !black {
		col = ink.White
	}
	rr := rad * rad
	for dy := -rad; dy <= rad; dy++ {
		w := 0
		for w*w+dy*dy <= rr {
			w++
		}
		ink.DrawLine(image.Pt(cx-w, cy+dy), image.Pt(cx+w, cy+dy), col)
	}
}

// splashRing draws a black ring (thickness ~ rad/4).
func splashRing(cx, cy, rad int) {
	inner := rad - rad/4
	if inner < 1 {
		inner = 1
	}
	rr := rad * rad
	ir := inner * inner
	for dy := -rad; dy <= rad; dy++ {
		w := 0
		for w*w+dy*dy <= rr {
			w++
		}
		iw := 0
		for iw*iw+dy*dy <= ir {
			iw++
		}
		ink.DrawLine(image.Pt(cx-w, cy+dy), image.Pt(cx-iw, cy+dy), ink.Black)
		ink.DrawLine(image.Pt(cx+iw, cy+dy), image.Pt(cx+w, cy+dy), ink.Black)
	}
}

func (a *App) drawRules() {
	f := a.fonts
	sz := ink.ScreenSize()
	drawTitle(f.TitleGame, "Mastermind — Regler", titleY)

	margin := 60
	maxW := sz.X - 2*margin
	y := 200
	lineH := 46
	paraGap := 22
	f.Detail.SetActive(ink.Black)
	for _, p := range rulesParagraphs {
		for _, ln := range wrapText(p, maxW) {
			drawTextAt(f.Detail, image.Pt(margin, y), ln)
			y += lineH
		}
		y += paraGap
	}

	// Back button.
	bw := sz.X / 2
	rb := image.Rect(sz.X/2-bw/2, usableH-160, sz.X/2+bw/2, usableH-50)
	a.rulesBack = button{rect: rb, label: "Tillbaka", hot: true}
	a.rulesBack.draw(f.Button)
}

// wrapText breaks s into lines no wider than maxW px using the active font.
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

func (a *App) drawGame() {
	g := a.game
	cfg := g.Cfg
	sz := ink.ScreenSize()
	f := a.fonts

	drawTitle(f.TitleGame, "Mastermind — "+cfg.Name, titleY)

	// Status line.
	var status string
	switch g.Status {
	case Playing:
		status = "Försök " + strconv.Itoa(len(g.History)+1) + " av " + strconv.Itoa(cfg.MaxGuesses)
	case Won:
		status = "Du vann på " + strconv.Itoa(len(g.History)) + " försök!"
	case Lost:
		status = "Förlust — koden var: " + secretString(g.Secret)
	}
	drawTitle(f.Heading, status, statusY)

	pegW := (sz.X - 2*margin - 240) / maxPegs(cfg.Pegs)
	if pegW > rowH-16 {
		pegW = rowH - 16
	}

	if g.Status != Playing {
		// Game over: history is the only content above the button row, so it
		// gets the full space between the status line and the bottom-anchored
		// buttons. Size rows to fit MaxGuesses rows (the worst case, so the
		// layout doesn't shift as guesses accumulate) within that space.
		bottomY := usableH - 320
		histRowH := rowH
		if avail := bottomY - historyTop; cfg.MaxGuesses > 0 {
			if fit := avail / cfg.MaxGuesses; fit < histRowH {
				histRowH = fit
			}
		}
		hPegW := histRowH - 16
		if hPegW > pegW {
			hPegW = pegW
		}
		y := historyTop
		for _, h := range g.History {
			x := margin
			for _, c := range h.Guess {
				r := image.Rect(x, y, x+hPegW, y+hPegW)
				drawPeg(r, c, true, f.PegDigit)
				x += hPegW + pegGap
			}
			drawFeedback(sz.X-220, y+8, h.Feedback, cfg.Pegs)
			y += histRowH
		}
		a.drawGameButtons(sz, f)
		return
	}

	// Playing: reserve bottom-anchored space for the draft row, palette, and
	// button row FIRST (against the real usable height, not ScreenSize().Y),
	// then size the history rows to fit whatever remains above it — so the
	// history never pushes the draft/palette/buttons off-screen no matter how
	// many guesses (up to MaxGuesses) have been made.
	btnH := 120
	rowGap := 30
	backY := usableH - margin - 120
	btnY := backY - rowGap - btnH

	palRows := (cfg.Colors + colsPerPaletteRow(sz.X) - 1) / colsPerPaletteRow(sz.X)
	palH := palRows*(96+pegGap) + 30 + 30 // palette rows + label + gap above buttons
	draftH := pegW + 40 + 20              // draft pegs + gaps + divider

	draftPaletteTop := btnY - rowGap - palH - draftH
	histRowH := rowH
	if avail := draftPaletteTop - historyTop; cfg.MaxGuesses > 0 {
		if fit := avail / cfg.MaxGuesses; fit < histRowH {
			histRowH = fit
		}
		if histRowH < 1 {
			histRowH = 1
		}
	}
	hPegW := histRowH - 16
	if hPegW > pegW {
		hPegW = pegW
	}
	if hPegW < 1 {
		hPegW = 1
	}

	y := historyTop
	for _, h := range g.History {
		x := margin
		for _, c := range h.Guess {
			r := image.Rect(x, y, x+hPegW, y+hPegW)
			drawPeg(r, c, true, f.PegDigit)
			x += hPegW + pegGap
		}
		drawFeedback(sz.X-220, y+8, h.Feedback, cfg.Pegs)
		y += histRowH
	}

	// Draft row (current guess being built), drawn just below history with a rule.
	ink.DrawLine(image.Pt(margin, y+4), image.Pt(sz.X-margin, y+4), ink.Black)
	y += 20
	a.pegRects = a.pegRects[:0]
	x := margin
	for i, c := range a.draft {
		r := image.Rect(x, y, x+pegW, y+pegW)
		if c < 0 {
			// empty slot.
			ink.DrawRect(r, ink.Black)
			ink.DrawRect(ink.Pad(r, 3), ink.LightGray)
		} else {
			drawPeg(r, c, true, f.PegDigit)
		}
		a.pegRects = append(a.pegRects, r)
		x += pegW + pegGap
		_ = i
	}
	y += pegW + 40

	// Palette.
	drawTextAt(f.Detail, image.Pt(margin, y-6), "Palett (vald är markerad):")
	y += 30
	a.paletteRect = a.paletteRect[:0]
	palW := 96
	px := margin
	py := y
	for i := 0; i < cfg.Colors; i++ {
		if px+palW > sz.X-margin {
			px = margin
			py += palW + pegGap
		}
		r := image.Rect(px, py, px+palW, py+palW)
		drawPeg(r, Color(i), true, f.PegDigit)
		if i == a.sel {
			// Selection marker: thick border.
			ink.DrawRect(ink.Pad(r, -6), ink.Black)
			ink.DrawRect(ink.Pad(r, -8), ink.Black)
		}
		a.paletteRect = append(a.paletteRect, r)
		px += palW + pegGap
	}

	// Button row: bottom-anchored against the real usable height (1340, not
	// ScreenSize().Y) so it stays on-screen even when history/palette content
	// above grows tall (large boards, many guesses) — never stack top-down off
	// a content-dependent y.
	bw := 360
	a.confirmBtn = button{
		rect:  image.Rect(margin, btnY, margin+bw, btnY+btnH),
		label: "Bekräfta",
		hot:   a.draftFull(),
	}
	a.confirmBtn.draw(f.Button)

	a.newBtn = button{rect: image.Rect(sz.X-margin-260, btnY, sz.X-margin, btnY+btnH), label: "Ny kod", hot: true}
	a.newBtn.draw(f.Button)
	a.backBtn = button{rect: image.Rect(margin, backY, margin+260, backY+120), label: "Meny", hot: true}
	a.backBtn.draw(f.Button)
}

// colsPerPaletteRow returns how many 96px palette swatches fit per row given
// screen width sz.X, matching the wrap logic in the palette drawing loop.
func colsPerPaletteRow(screenW int) int {
	palW := 96
	usableW := screenW - 2*margin
	n := (usableW + pegGap) / (palW + pegGap)
	if n < 1 {
		n = 1
	}
	return n
}

func (a *App) drawGameButtons(sz image.Point, f *Fonts) {
	y := usableH - 320
	a.newBtn = button{rect: image.Rect(margin, y, margin+320, y+120), label: "Spela igen", hot: true}
	a.newBtn.draw(f.Button)
	a.backBtn = button{rect: image.Rect(sz.X-margin-320, y, sz.X-margin, y+120), label: "Meny", hot: true}
	a.backBtn.draw(f.Button)
	// Disable stale interactive rects.
	a.pegRects = a.pegRects[:0]
	a.paletteRect = a.paletteRect[:0]
	a.confirmBtn = button{}
}

// drawKnuth renders the "Enheten gissar" screen: the device's current guess as
// pegs, black/white feedback steppers, and Confirm / Ny / Meny buttons.
func (a *App) drawKnuth() {
	f := a.fonts
	sz := ink.ScreenSize()
	s := a.knuth

	drawTitle(f.TitleGame, "Enheten gissar", titleY)

	if s == nil {
		return
	}

	// Status line.
	var status string
	switch {
	case s.Solved():
		status = "Löst på " + strconv.Itoa(s.Turn()) + "!"
	case s.Impossible():
		status = "Omöjlig feedback"
	default:
		status = "Gissning " + strconv.Itoa(s.Turn()) +
			"  (" + strconv.Itoa(s.Remaining()) + " möjliga)"
	}
	drawTitle(f.Heading, status, statusY)

	// The device's current guess as a row of pegs, centered.
	guess := s.CurrentGuess()
	n := len(guess)
	pegW := 150
	totalW := n*pegW + (n-1)*pegGap
	x := sz.X/2 - totalW/2
	y := 260
	for _, c := range guess {
		r := image.Rect(x, y, x+pegW, y+pegW)
		drawPeg(r, c, true, f.PegDigit)
		x += pegW + pegGap
	}
	y += pegW + 50

	if s.Impossible() {
		f.Detail.SetActive(ink.Black)
		msg := "Din feedback motsäger sig själv. Tryck Ny för att börja om."
		for _, ln := range wrapText(msg, sz.X-2*margin) {
			drawTextAt(f.Detail, image.Pt(margin, y), ln)
			y += 46
		}
		a.drawKnuthEndButtons(sz, f)
		return
	}
	if s.Solved() {
		f.Detail.SetActive(ink.Black)
		drawTextAt(f.Heading, image.Pt(margin, y),
			"Enheten knäckte din kod på "+strconv.Itoa(s.Turn())+" gissningar.")
		a.drawKnuthEndButtons(sz, f)
		return
	}

	if a.aiPending {
		drawTitle(f.Heading, "Räknar…", y+40)
		// Clear interactive rects so nothing is tappable while computing.
		a.kBlackMinus, a.kBlackPlus = button{}, button{}
		a.kWhiteMinus, a.kWhitePlus = button{}, button{}
		a.kConfirm = button{}
		a.drawKnuthEndButtons(sz, f)
		return
	}

	// Instruction.
	f.Detail.SetActive(ink.Black)
	drawTextAt(f.Detail, image.Pt(margin, y-10), "Ange din feedback:")
	y += 40

	// Feedback steppers. Each row: label + sample marker, minus, value, plus.
	a.kBlackMinus, a.kBlackPlus = a.drawStepper(sz, f, y, "Svarta (rätt plats)", a.kfbBlack, true)
	y += 170
	a.kWhiteMinus, a.kWhitePlus = a.drawStepper(sz, f, y, "Vita (fel plats)", a.kfbWhite, false)
	y += 200

	// Confirm button (full width-ish), then Ny / Meny below.
	cbW := sz.X - 2*margin
	a.kConfirm = button{rect: image.Rect(margin, y, margin+cbW, y+120), label: "Bekräfta", hot: true}
	a.kConfirm.draw(f.Button)
	y += 150
	a.drawKnuthEndButtonsAt(sz, f, y)
}

// drawStepper draws one feedback row with a minus/value/plus control and a
// sample feedback marker (filled for black, hollow for white). Returns the
// minus and plus button rects.
func (a *App) drawStepper(sz image.Point, f *Fonts, y int, label string, val int, filled bool) (button, button) {
	// Sample marker + label on the left.
	mr := image.Rect(margin, y+10, margin+40, y+50)
	if filled {
		ink.FillArea(mr, ink.Black)
	} else {
		ink.DrawRect(mr, ink.Black)
		ink.DrawRect(ink.Pad(mr, 2), ink.Black)
	}
	drawTextAt(f.Detail, image.Pt(margin+60, y+8), label)

	// Stepper on a second line: [ - ]  value  [ + ]
	sy := y + 60
	bw := 110
	minus := button{rect: image.Rect(margin, sy, margin+bw, sy+90), label: "–", hot: true}
	minus.draw(f.TitleBig)

	// Value box.
	vx := margin + bw + 40
	vr := image.Rect(vx, sy, vx+110, sy+90)
	ink.DrawRect(vr, ink.Black)
	f.TitleBig.SetActive(ink.Black)
	vs := strconv.Itoa(val)
	vw := ink.StringWidth(vs)
	ink.DrawString(image.Pt((vr.Min.X+vr.Max.X)/2-vw/2, vr.Min.Y+16), vs)

	px := vx + 110 + 40
	plus := button{rect: image.Rect(px, sy, px+bw, sy+90), label: "+", hot: true}
	plus.draw(f.TitleBig)

	return minus, plus
}

// drawKnuthEndButtons draws Ny / Meny at a fixed spot near the bottom.
func (a *App) drawKnuthEndButtons(sz image.Point, f *Fonts) {
	a.drawKnuthEndButtonsAt(sz, f, usableH-320)
}

func (a *App) drawKnuthEndButtonsAt(sz image.Point, f *Fonts, y int) {
	a.kNew = button{rect: image.Rect(margin, y, margin+320, y+120), label: "Ny", hot: true}
	a.kNew.draw(f.Button)
	a.kBack = button{rect: image.Rect(sz.X-margin-320, y, sz.X-margin, y+120), label: "Meny", hot: true}
	a.kBack.draw(f.Button)
}

func maxPegs(p int) int {
	if p < 1 {
		return 1
	}
	return p
}

func secretString(s Secret) string {
	out := ""
	for i, c := range s {
		if i > 0 {
			out += " "
		}
		out += strconv.Itoa(int(c) + 1)
	}
	return out
}

func main() {
	if err := ink.Run(newApp()); err != nil {
		panic(err)
	}
}
