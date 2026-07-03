package main

import (
	"image"
	"strconv"

	ink "github.com/dennwc/inkview"
	"sudoku/game"
)

// Draw renders the whole screen. ScreenSize is read here (guide rule 3).
func (a *app) Draw() {
	ink.ClearScreen()
	switch a.screen {
	case screenSplash:
		a.drawSplash()
	case screenMenu:
		a.drawMenu()
	case screenPlay:
		a.drawPlay(computeLayout())
	case screenRules:
		a.drawRules()
	}
	ink.FullUpdate()
}

// menuButtonRect lays out the difficulty buttons on the menu.
func menuButtonRect(i int) image.Rectangle {
	s := ink.ScreenSize()
	H := usableH
	w := s.X * 2 / 3
	h := H / 12
	gap := H / 30
	x := (s.X - w) / 2
	y := H/3 + i*(h+gap)
	return image.Rect(x, y, x+w, y+h)
}

var menuLabels = []string{"Latt", "Medel", "Svar"}

func (a *app) drawMenu() {
	s := ink.ScreenSize()
	H := usableH
	// Title.
	a.f.title.SetActive(ink.Black)
	title := "SUDOKU"
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((s.X-tw)/2, H/6), title)

	a.f.small.SetActive(ink.Black)
	sub := "Valj svarighetsgrad"
	sw := ink.StringWidth(sub)
	ink.DrawString(image.Pt((s.X-sw)/2, H/6+90), sub)

	for i, lbl := range menuLabels {
		r := menuButtonRect(i)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(ink.Pad(r, 3), ink.Black)
		centerText(a.f.button, r, lbl, ink.Black, 40)
	}

	// "Regler" button below the difficulty buttons opens the rules screen.
	rb := menuButtonRect(len(menuLabels))
	ink.DrawRect(rb, ink.Black)
	ink.DrawRect(ink.Pad(rb, 3), ink.Black)
	centerText(a.f.button, rb, "Regler", ink.Black, 40)
	a.menuRules = rb
}

func (a *app) drawPlay(l layout) {
	s := ink.ScreenSize()
	headY := usableH / 40
	if headY < 40 {
		headY = 40
	}

	// Title band: difficulty + status message.
	a.f.small.SetActive(ink.Black)
	head := "Sudoku - " + a.diff.String()
	ink.DrawString(image.Pt(l.gridOrigin.X, headY), head)
	if a.message != "" {
		mw := ink.StringWidth(a.message)
		ink.DrawString(image.Pt(s.X-l.gridOrigin.X-mw, headY), a.message)
	}

	conf := map[game.Cell]bool{}
	if a.showConf {
		conf = a.board.Conflicts()
	}

	// Cell backgrounds: highlight the selected cell.
	if a.selR >= 0 && a.selC >= 0 {
		ink.FillArea(l.cellRect(a.selR, a.selC), ink.LightGray)
	}

	// Digits + notes.
	for r := 0; r < game.N; r++ {
		for c := 0; c < game.N; c++ {
			rect := l.cellRect(r, c)
			v := a.board[r][c]
			// Subtle conflict / wrong marker: dark corner wedge.
			if a.showConf {
				wrong := conf[game.Cell{Row: r, Col: c}]
				if !wrong && v != 0 && a.board.IsComplete() && v != a.puzzle.Solution[r][c] {
					wrong = true
				}
				if wrong {
					a.drawConflictMark(rect)
				}
			}
			if v != 0 {
				f := a.f.cell
				f.SetActive(ink.Black)
				str := strconv.Itoa(v)
				w := ink.StringWidth(str)
				x := rect.Min.X + (rect.Dx()-w)/2
				y := rect.Min.Y + (rect.Dy()-56)/2
				// Given clues are drawn bold-black; player entries the same
				// on greyscale, but we underline givens for distinction.
				ink.DrawString(image.Pt(x, y), str)
				if a.puzzle.Given[r][c] {
					yb := rect.Max.Y - rect.Dy()/8
					ink.DrawLine(image.Pt(rect.Min.X+rect.Dx()/3, yb),
						image.Pt(rect.Max.X-rect.Dx()/3, yb), ink.Black)
				}
			} else if a.notes[r][c] != 0 {
				a.drawNotes(rect, a.notes[r][c])
			}
		}
	}

	a.drawGridLines(l)
	a.drawNumberPad(l)
	a.drawActions(l)
}

// drawGridLines draws thin lines for every cell and thick lines on the
// 3x3 box boundaries.
func (a *app) drawGridLines(l layout) {
	g := l.gridOrigin
	end := image.Pt(g.X+l.gridSize, g.Y+l.gridSize)
	for i := 0; i <= game.N; i++ {
		x := g.X + i*l.cellSize
		y := g.Y + i*l.cellSize
		ink.DrawLine(image.Pt(x, g.Y), image.Pt(x, end.Y), ink.Black)
		ink.DrawLine(image.Pt(g.X, y), image.Pt(end.X, y), ink.Black)
		if i%3 == 0 {
			// Thicken box borders by drawing 2 neighbouring lines.
			ink.DrawLine(image.Pt(x-1, g.Y), image.Pt(x-1, end.Y), ink.Black)
			ink.DrawLine(image.Pt(x+1, g.Y), image.Pt(x+1, end.Y), ink.Black)
			ink.DrawLine(image.Pt(g.X, y-1), image.Pt(end.X, y-1), ink.Black)
			ink.DrawLine(image.Pt(g.X, y+1), image.Pt(end.X, y+1), ink.Black)
		}
	}
}

func (a *app) drawNotes(rect image.Rectangle, mask uint16) {
	a.f.pencil.SetActive(ink.Black)
	third := rect.Dx() / 3
	for d := 1; d <= 9; d++ {
		if mask&(1<<uint(d)) == 0 {
			continue
		}
		i := d - 1
		cx := rect.Min.X + (i%3)*third + third/2
		cy := rect.Min.Y + (i/3)*third + third/6
		s := strconv.Itoa(d)
		w := ink.StringWidth(s)
		ink.DrawString(image.Pt(cx-w/2, cy), s)
	}
}

// drawConflictMark draws a subtle small filled triangle in the top-right
// corner of a cell to flag a conflict without shouting.
func (a *app) drawConflictMark(rect image.Rectangle) {
	sz := rect.Dx() / 5
	x0 := rect.Max.X - sz
	for i := 0; i < sz; i++ {
		ink.DrawLine(image.Pt(x0+i, rect.Min.Y), image.Pt(rect.Max.X, rect.Min.Y+i), ink.Black)
	}
}

func (a *app) drawNumberPad(l layout) {
	for d := 1; d <= 9; d++ {
		r := l.numButtonRect(d)
		ink.DrawRect(r, ink.Black)
		centerText(a.f.button, r, strconv.Itoa(d), ink.Black, 40)
	}
}

func (a *app) drawActions(l layout) {
	labels := a.actionLabels()
	for i, lbl := range labels {
		r := l.actionButtonRect(i, len(labels))
		ink.DrawRect(r, ink.Black)
		if i == 0 && a.noteMode { // highlight active note mode
			ink.FillArea(ink.Pad(r, 2), ink.LightGray)
			ink.DrawRect(r, ink.Black)
		}
		centerText(a.f.button, r, lbl, ink.Black, 40)
	}
}

func (a *app) actionLabels() []string {
	note := "Anteckn"
	if a.noteMode {
		note = "Anteckn*"
	}
	return []string{note, "Sudda", "Klar?", "Ny"}
}

// --- tap dispatch -----------------------------------------------------

func (a *app) handleTap(p image.Point) bool {
	switch a.screen {
	case screenSplash:
		// Any tap advances to the menu.
		a.screen = screenMenu
		a.repaint()
		return true
	case screenMenu:
		return a.tapMenu(p)
	case screenPlay:
		return a.tapPlay(p)
	case screenRules:
		if p.In(a.rulesBack) {
			a.screen = screenMenu
			a.repaint()
			return true
		}
	}
	return false
}

func (a *app) tapMenu(p image.Point) bool {
	if p.In(a.menuRules) {
		a.screen = screenRules
		a.repaint()
		return true
	}
	for i := range menuLabels {
		if p.In(menuButtonRect(i)) {
			a.newGame(game.Difficulty(i))
			a.repaint()
			return true
		}
	}
	return false
}

func (a *app) tapPlay(p image.Point) bool {
	l := computeLayout()

	// Grid cells.
	grid := image.Rect(l.gridOrigin.X, l.gridOrigin.Y,
		l.gridOrigin.X+l.gridSize, l.gridOrigin.Y+l.gridSize)
	if p.In(grid) {
		c := (p.X - l.gridOrigin.X) / l.cellSize
		r := (p.Y - l.gridOrigin.Y) / l.cellSize
		if r >= 0 && r < game.N && c >= 0 && c < game.N {
			a.selR, a.selC = r, c
			a.repaint()
			return true
		}
	}

	// Number buttons.
	for d := 1; d <= 9; d++ {
		if p.In(l.numButtonRect(d)) {
			a.placeDigit(d)
			a.repaint()
			return true
		}
	}

	// Action buttons.
	labels := a.actionLabels()
	for i := range labels {
		if p.In(l.actionButtonRect(i, len(labels))) {
			switch i {
			case 0:
				a.noteMode = !a.noteMode
			case 1:
				a.erase()
			case 2:
				a.check()
			case 3:
				a.screen = screenMenu
			}
			a.repaint()
			return true
		}
	}
	return false
}

// --- Splash screen ----------------------------------------------------

// drawSplash renders the launch screen: big title, a Sudoku motif box, and a
// grey "tap to start" hint. Reached first on launch; any tap -> menu.
func (a *app) drawSplash() {
	s := ink.ScreenSize()
	H := usableH

	a.f.splash.SetActive(ink.Black)
	title := "SUDOKU"
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((s.X-tw)/2, H/6), title)

	side := s.X * 3 / 5
	box := image.Rect((s.X-side)/2, (H-side)/2, (s.X+side)/2, (H+side)/2)
	a.drawSplashMotif(box)

	a.f.rules.SetActive(ink.DarkGray)
	hint := "Tryck for att borja"
	hw := ink.StringWidth(hint)
	ink.DrawString(image.Pt((s.X-hw)/2, H*5/6), hint)
}

// drawSplashMotif draws a single 3x3 Sudoku box with a few clue digits, the
// game's own line-art in monochrome.
func (a *app) drawSplashMotif(box image.Rectangle) {
	// Use the smaller side so the 3x3 stays square.
	side := box.Dx()
	if box.Dy() < side {
		side = box.Dy()
	}
	cell := side / 3
	grid := cell * 3
	ox := box.Min.X + (box.Dx()-grid)/2
	oy := box.Min.Y + (box.Dy()-grid)/2

	// Cell lines.
	for i := 0; i <= 3; i++ {
		x := ox + i*cell
		y := oy + i*cell
		ink.DrawLine(image.Pt(x, oy), image.Pt(x, oy+grid), ink.Black)
		ink.DrawLine(image.Pt(ox, y), image.Pt(ox+grid, y), ink.Black)
	}
	// Thicken the outer box border.
	ink.DrawRect(image.Rect(ox, oy, ox+grid, oy+grid), ink.Black)
	ink.DrawRect(ink.Pad(image.Rect(ox, oy, ox+grid, oy+grid), 1), ink.Black)
	ink.DrawRect(ink.Pad(image.Rect(ox, oy, ox+grid, oy+grid), 2), ink.Black)

	// A few clue digits scattered in the box (like a real Sudoku box).
	clues := [3][3]int{
		{5, 0, 3},
		{0, 7, 0},
		{2, 0, 9},
	}
	a.f.motif.SetActive(ink.Black)
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			d := clues[r][c]
			if d == 0 {
				continue
			}
			rect := image.Rect(ox+c*cell, oy+r*cell, ox+(c+1)*cell, oy+(r+1)*cell)
			centerText(a.f.motif, rect, strconv.Itoa(d), ink.Black, 60)
		}
	}
}

// --- Rules screen -----------------------------------------------------

var rulesParagraphs = []string{
	"Mal: fyll hela rutnatet sa att varje rad, varje kolumn och varje 3x3-box innehaller siffrorna 1-9 exakt en gang.",
	"Vissa celler ar redan ifyllda (understrukna) - de ar fasta ledtradar och kan inte andras.",
	"Tryck pa en cell for att markera den. Tryck sedan en siffra 1-9 for att fylla i den. Tryck samma siffra igen for att ta bort den.",
	"Anteckningslage: tryck Anteckn for att slaa pa smaa anteckningar (pencil marks). Da fyller siffrorna i sma noteringar i cellen i stallet for en stor siffra - bra for att komma ihag mojliga val.",
	"Sudda tar bort siffran eller anteckningarna i den markerade cellen.",
	"Klar? kontrollerar bradet: konflikter markeras, och nar allt ar ifyllt sags om losningen ar ratt.",
	"Ny gar tillbaka till menyn dar du valjer svarighetsgrad: Latt, Medel eller Svar.",
}

// drawRules renders the full Swedish rules with a "Tillbaka" button, storing
// the button rect for tap handling.
func (a *app) drawRules() {
	s := ink.ScreenSize()

	a.f.title.SetActive(ink.Black)
	title := "Regler"
	tw := ink.StringWidth(title)
	ink.DrawString(image.Pt((s.X-tw)/2, 60), title)

	a.f.rules.SetActive(ink.Black)
	margin := 60
	maxW := s.X - 2*margin
	y := 200
	lineH := 46
	paraGap := 24
	for _, p := range rulesParagraphs {
		for _, ln := range wrapText(p, maxW) {
			ink.DrawString(image.Pt(margin, y), ln)
			y += lineH
		}
		y += paraGap
	}

	bh := 110
	bw := s.X / 2
	H := usableH
	r := image.Rect((s.X-bw)/2, H-bh-40, (s.X+bw)/2, H-40)
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(ink.Pad(r, 3), ink.Black)
	centerText(a.f.button, r, "Tillbaka", ink.Black, 40)
	a.rulesBack = r
}

// wrapText greedily word-wraps s to maxW pixels, measured with the currently
// active font via ink.StringWidth. Avoids importing "strings".
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
