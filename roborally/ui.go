package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"

	"roborally/game"
)

// Fonts held open for the app lifetime (opened once — never per draw).
type Fonts struct {
	Title  *ink.Font
	Big    *ink.Font
	Status *ink.Font
	Body   *ink.Font
	Card   *ink.Font
	Small  *ink.Font
}

func InitFonts() *Fonts {
	return &Fonts{
		Title:  ink.OpenFont(ink.DefaultFontBold, 72, true),
		Big:    ink.OpenFont(ink.DefaultFontBold, 44, true),
		Status: ink.OpenFont(ink.DefaultFontBold, 34, true),
		Body:   ink.OpenFont(ink.DefaultFont, 32, true),
		Card:   ink.OpenFont(ink.DefaultFontBold, 28, true),
		Small:  ink.OpenFont(ink.DefaultFont, 24, true),
	}
}

func (f *Fonts) Close() {
	for _, fn := range []*ink.Font{f.Title, f.Big, f.Status, f.Body, f.Card, f.Small} {
		if fn != nil {
			fn.Close()
		}
	}
}

const usableH = 1340 // ink.ScreenSize().Y (1448) lies; below ~1360 wraps to top

func pad(r image.Rectangle, n int) image.Rectangle {
	return image.Rect(r.Min.X+n, r.Min.Y+n, r.Max.X-n, r.Max.Y-n)
}

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

func drawCentered(r image.Rectangle, s string, approxH int) {
	w := ink.StringWidth(s)
	ink.DrawString(image.Pt(r.Min.X+(r.Dx()-w)/2, r.Min.Y+(r.Dy()-approxH)/2), s)
}

func drawBtn(r image.Rectangle, label string, f *ink.Font, enabled bool) {
	var col color.Color = ink.Black
	if !enabled {
		col = ink.DarkGray
	}
	ink.DrawRect(r, col)
	ink.DrawRect(pad(r, 1), col)
	f.SetActive(col)
	drawCentered(r, label, 34)
}

// --- Menu ------------------------------------------------------------------

type menuRow struct {
	rect image.Rectangle
	id   string
}

func DrawMenu(screen image.Point, f *Fonts, cfg config) ([]menuRow, image.Rectangle) {
	ink.ClearScreen()
	H := usableH

	f.Title.SetActive(ink.Black)
	drawCentered(image.Rect(0, 40, screen.X, 150), "Robo Rally", 72)

	rows := []struct {
		id, label string
	}{
		{"bana", "Bana: " + banaLabel(cfg.courseSel)},
		{"nai", "Motståndare: " + itoa(cfg.nAI)},
		{"ai", "Dator: " + cfg.ai.String()},
	}

	margin := 70
	rowW := screen.X - 2*margin
	rowH := 110
	top := 210
	var out []menuRow
	f.Big.SetActive(ink.Black)
	y := top
	for _, row := range rows {
		r := image.Rect(margin, y, margin+rowW, y+rowH-18)
		ink.DrawRect(r, ink.Black)
		ink.DrawRect(pad(r, 1), ink.Black)
		f.Big.SetActive(ink.Black)
		ink.DrawString(image.Pt(r.Min.X+26, r.Min.Y+(r.Dy()-44)/2), row.label)
		out = append(out, menuRow{rect: r, id: row.id})
		y += rowH
	}

	// Start button (large) and Regler below it, bottom-anchored.
	bmargin := 60
	regH := 90
	regBtn := image.Rect((screen.X-rowW/2)/1, 0, 0, 0) // placeholder, set below
	_ = regBtn
	rulesBtn := image.Rect((screen.X-rowW/2)/2, H-bmargin-regH, (screen.X+rowW/2)/2, H-bmargin)
	startH := 130
	startBtn := image.Rect(margin, rulesBtn.Min.Y-24-startH, margin+rowW, rulesBtn.Min.Y-24)
	drawBtn(startBtn, "Starta", f.Big, true)
	out = append(out, menuRow{rect: startBtn, id: "start"})
	drawBtn(rulesBtn, "Regler", f.Big, true)
	return out, rulesBtn
}

// banaLabel names a course-selector value: the curated courses first, then the
// three "Slump" (random) tiers.
func banaLabel(sel int) string {
	nc := game.NumCurated()
	if sel < nc {
		tiers := []string{"Lätt", "Mellan", "Svår"}
		return itoa(sel+1) + " – " + tiers[int(game.CuratedTier(sel))]
	}
	slump := []string{"Slump Lätt", "Slump Mellan", "Slump Svår"}
	return slump[sel-nc]
}

// --- Board layout ----------------------------------------------------------

type BoardLayout struct {
	Origin image.Point
	Cell   int
	Area   image.Rectangle
}

func newBoardLayout(area image.Rectangle, w, h int) BoardLayout {
	cell := area.Dx() / w
	if c := area.Dy() / h; c < cell {
		cell = c
	}
	if cell < 1 {
		cell = 1
	}
	bw, bh := cell*w, cell*h
	origin := image.Pt(area.Min.X+(area.Dx()-bw)/2, area.Min.Y+(area.Dy()-bh)/2)
	return BoardLayout{Origin: origin, Cell: cell, Area: image.Rect(origin.X, origin.Y, origin.X+bw, origin.Y+bh)}
}

func (l BoardLayout) cell(x, y int) image.Rectangle {
	return image.Rect(l.Origin.X+x*l.Cell, l.Origin.Y+y*l.Cell,
		l.Origin.X+(x+1)*l.Cell, l.Origin.Y+(y+1)*l.Cell)
}

// --- Board rendering -------------------------------------------------------

func drawBoard(l BoardLayout, b *game.Board, robots []game.Robot, f *Fonts) {
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			drawTile(l.cell(x, y), b.At(image.Pt(x, y)), l.Cell, f)
		}
	}
	// Walls on top of tiles so shared edges read cleanly.
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			drawWalls(l.cell(x, y), b.At(image.Pt(x, y)))
		}
	}
	for i := range robots {
		r := &robots[i]
		if r.NextCheck > b.NCheck {
			continue // finished, off the board
		}
		drawRobot(l.cell(r.Pos.X, r.Pos.Y), r.ID, r.Facing, r.Alive, r.IsHuman, f)
	}
}

func drawTile(rect image.Rectangle, t *game.Tile, cell int, f *Fonts) {
	ink.DrawRect(rect, ink.LightGray)
	switch {
	case t.Kind == game.FloorPit:
		ink.FillArea(pad(rect, 3), ink.Black)
		// white X
		in := pad(rect, cell/4)
		ink.DrawLine(in.Min, in.Max, ink.White)
		ink.DrawLine(image.Pt(in.Max.X, in.Min.Y), image.Pt(in.Min.X, in.Max.Y), ink.White)
		return
	case t.Kind == game.FloorRepair:
		drawPlus(rect, cell)
	}
	if t.Belt != game.DirNone {
		drawBeltGlyph(rect, t.Belt, t.BeltExpress)
	}
	if t.Gear != game.GearNone {
		drawGearGlyph(rect, t.Gear, cell)
	}
	if t.Antenna {
		drawAntenna(rect, cell)
	}
	if t.Checkpoint != 0 {
		drawCheckpoint(rect, int(t.Checkpoint), f)
	}
	if t.StartDock != 0 {
		drawStartDock(rect, int(t.StartDock), cell, f)
	}
	if t.Laser != game.DirNone {
		drawLaserEmitter(rect, t.Laser, cell)
	}
}

func center(r image.Rectangle) image.Point {
	return image.Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2)
}

// drawBeltGlyph draws one or two chevrons pointing in the belt direction.
func drawBeltGlyph(rect image.Rectangle, d game.Dir, express bool) {
	c := center(rect)
	s := rect.Dx() / 5
	chevron := func(off int) {
		// tip and two wings, oriented by direction
		var tip, w1, w2 image.Point
		switch d {
		case game.N:
			tip = image.Pt(c.X, c.Y-s+off)
			w1 = image.Pt(c.X-s, c.Y+off)
			w2 = image.Pt(c.X+s, c.Y+off)
		case game.S:
			tip = image.Pt(c.X, c.Y+s+off)
			w1 = image.Pt(c.X-s, c.Y+off)
			w2 = image.Pt(c.X+s, c.Y+off)
		case game.E:
			tip = image.Pt(c.X+s+off, c.Y)
			w1 = image.Pt(c.X+off, c.Y-s)
			w2 = image.Pt(c.X+off, c.Y+s)
		default: // W
			tip = image.Pt(c.X-s+off, c.Y)
			w1 = image.Pt(c.X+off, c.Y-s)
			w2 = image.Pt(c.X+off, c.Y+s)
		}
		for o := 0; o <= 1; o++ {
			ink.DrawLine(tip.Add(image.Pt(0, o)), w1.Add(image.Pt(0, o)), ink.DarkGray)
			ink.DrawLine(tip.Add(image.Pt(0, o)), w2.Add(image.Pt(0, o)), ink.DarkGray)
		}
	}
	chevron(0)
	if express {
		back := s
		switch d {
		case game.N:
			chevron(back)
		case game.S:
			chevron(-back)
		case game.E:
			chevron(-back)
		case game.W:
			chevron(back)
		}
	}
}

func drawGearGlyph(rect image.Rectangle, g game.Gear, cell int) {
	r := pad(rect, cell/4)
	ink.DrawRect(r, ink.DarkGray)
	ink.DrawRect(pad(r, 1), ink.DarkGray)
	// a tick on one corner to suggest rotation direction
	c := center(rect)
	if g == game.GearCW {
		ink.DrawLine(image.Pt(r.Max.X, c.Y), image.Pt(r.Max.X, r.Min.Y), ink.Black)
		ink.DrawLine(image.Pt(r.Max.X, r.Min.Y), image.Pt(c.X, r.Min.Y), ink.Black)
	} else {
		ink.DrawLine(image.Pt(r.Min.X, c.Y), image.Pt(r.Min.X, r.Min.Y), ink.Black)
		ink.DrawLine(image.Pt(r.Min.X, r.Min.Y), image.Pt(c.X, r.Min.Y), ink.Black)
	}
}

func drawPlus(rect image.Rectangle, cell int) {
	c := center(rect)
	s := cell / 4
	for o := -1; o <= 1; o++ {
		ink.DrawLine(image.Pt(c.X-s, c.Y+o), image.Pt(c.X+s, c.Y+o), ink.DarkGray)
		ink.DrawLine(image.Pt(c.X+o, c.Y-s), image.Pt(c.X+o, c.Y+s), ink.DarkGray)
	}
}

func drawAntenna(rect image.Rectangle, cell int) {
	c := center(rect)
	top := image.Pt(c.X, rect.Min.Y+cell/6)
	ink.DrawLine(image.Pt(c.X, rect.Max.Y-cell/6), top, ink.Black)
	for i := 1; i <= 2; i++ {
		ink.DrawLine(top, image.Pt(top.X-i*4, top.Y-i*4), ink.Black)
		ink.DrawLine(top, image.Pt(top.X+i*4, top.Y-i*4), ink.Black)
	}
}

func drawCheckpoint(rect image.Rectangle, ord int, f *Fonts) {
	for o := 0; o <= 2; o++ {
		ink.DrawRect(pad(rect, o), ink.Black)
	}
	f.Big.SetActive(ink.Black)
	drawCentered(rect, itoa(ord), 44)
}

func drawLaserEmitter(rect image.Rectangle, d game.Dir, cell int) {
	c := center(rect)
	// small filled nub on the firing edge
	var nub image.Rectangle
	s := cell / 8
	switch d {
	case game.N:
		nub = image.Rect(c.X-s, rect.Max.Y-2*s, c.X+s, rect.Max.Y)
	case game.S:
		nub = image.Rect(c.X-s, rect.Min.Y, c.X+s, rect.Min.Y+2*s)
	case game.E:
		nub = image.Rect(rect.Min.X, c.Y-s, rect.Min.X+2*s, c.Y+s)
	default:
		nub = image.Rect(rect.Max.X-2*s, c.Y-s, rect.Max.X, c.Y+s)
	}
	ink.FillArea(nub, ink.Black)
}

func drawWalls(rect image.Rectangle, t *game.Tile) {
	th := 4
	if t.HasWall(game.N) {
		ink.FillArea(image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+th), ink.Black)
	}
	if t.HasWall(game.S) {
		ink.FillArea(image.Rect(rect.Min.X, rect.Max.Y-th, rect.Max.X, rect.Max.Y), ink.Black)
	}
	if t.HasWall(game.W) {
		ink.FillArea(image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+th, rect.Max.Y), ink.Black)
	}
	if t.HasWall(game.E) {
		ink.FillArea(image.Rect(rect.Max.X-th, rect.Min.Y, rect.Max.X, rect.Max.Y), ink.Black)
	}
}

// isqrt is an integer square root (floor).
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

// fillDisc fills an approximate circle inscribed in r.
func fillDisc(r image.Rectangle, col color.Color) {
	cx, cy := center(r).X, center(r).Y
	rad := r.Dx() / 2
	if r.Dy()/2 < rad {
		rad = r.Dy() / 2
	}
	rr := rad * rad
	for dy := -rad; dy <= rad; dy++ {
		hw := isqrt(rr - dy*dy)
		ink.DrawLine(image.Pt(cx-hw, cy+dy), image.Pt(cx+hw, cy+dy), col)
	}
}

// fillTriangle fills the triangle a-b-c (black) by scanline.
func fillTriangle(a, b, c image.Point) {
	minY := a.Y
	if b.Y < minY {
		minY = b.Y
	}
	if c.Y < minY {
		minY = c.Y
	}
	maxY := a.Y
	if b.Y > maxY {
		maxY = b.Y
	}
	if c.Y > maxY {
		maxY = c.Y
	}
	edges := [3][2]image.Point{{a, b}, {b, c}, {c, a}}
	for y := minY; y <= maxY; y++ {
		var xs []int
		for _, e := range edges {
			p, q := e[0], e[1]
			if p.Y == q.Y {
				continue
			}
			if (y >= p.Y && y < q.Y) || (y >= q.Y && y < p.Y) {
				x := p.X + (q.X-p.X)*(y-p.Y)/(q.Y-p.Y)
				xs = append(xs, x)
			}
		}
		if len(xs) >= 2 {
			lo, hi := xs[0], xs[1]
			if lo > hi {
				lo, hi = hi, lo
			}
			ink.DrawLine(image.Pt(lo, y), image.Pt(hi, y), ink.Black)
		}
	}
}

// fillNose draws a black arrowhead protruding from the token toward facing.
func fillNose(cell, body image.Rectangle, facing game.Dir) {
	c := center(body)
	rad := body.Dx() / 2
	reach := rad + cell.Dx()/7
	base := rad * 3 / 4
	dir := facing.Step()
	perpX, perpY := -dir.Y, dir.X
	tip := image.Pt(c.X+dir.X*reach, c.Y+dir.Y*reach)
	b1 := image.Pt(c.X+dir.X*base/2+perpX*base, c.Y+dir.Y*base/2+perpY*base)
	b2 := image.Pt(c.X+dir.X*base/2-perpX*base, c.Y+dir.Y*base/2-perpY*base)
	fillTriangle(tip, b1, b2)
}

// drawStartDock marks a starting square with corner brackets and its number, so
// empty start positions are visible on the board.
func drawStartDock(rect image.Rectangle, num, cell int, f *Fonts) {
	r := pad(rect, cell/8)
	n := cell / 4
	corners := [4][3]image.Point{
		{r.Min, image.Pt(r.Min.X + n, r.Min.Y), image.Pt(r.Min.X, r.Min.Y + n)},
		{image.Pt(r.Max.X, r.Min.Y), image.Pt(r.Max.X - n, r.Min.Y), image.Pt(r.Max.X, r.Min.Y + n)},
		{image.Pt(r.Min.X, r.Max.Y), image.Pt(r.Min.X + n, r.Max.Y), image.Pt(r.Min.X, r.Max.Y - n)},
		{r.Max, image.Pt(r.Max.X - n, r.Max.Y), image.Pt(r.Max.X, r.Max.Y - n)},
	}
	for _, c := range corners {
		for o := 0; o <= 1; o++ {
			ink.DrawLine(c[0].Add(image.Pt(0, o)), c[1].Add(image.Pt(0, o)), ink.DarkGray)
			ink.DrawLine(c[0].Add(image.Pt(o, 0)), c[2].Add(image.Pt(o, 0)), ink.DarkGray)
		}
	}
	f.Small.SetActive(ink.DarkGray)
	ink.DrawString(image.Pt(r.Min.X+n/2, center(rect).Y-14), "S"+itoa(num))
}

// drawRobot draws a robot as a round token with a prominent heading arrowhead
// and a per-robot emblem — a shape deliberately distinct from the square
// checkpoint/gear boxes so a robot never reads as a board element, and each
// robot is told apart by its emblem (not just a number):
//
//	1 (you)  solid black disc      3  striped disc
//	2        ring (hollow)         4  bullseye (ring + centre dot)
//
// A small id number sits in the cell corner as a secondary aid.
func drawRobot(rect image.Rectangle, id int, facing game.Dir, alive, human bool, f *Fonts) {
	body := pad(rect, rect.Dx()/7)
	if !alive {
		// faint hollow token for a destroyed robot awaiting respawn
		fillDisc(body, ink.LightGray)
		fillDisc(pad(body, body.Dx()/6), ink.White)
		return
	}
	// The heading arrowhead protrudes past the token — drawn first so the token
	// body sits on top of its base (a clean "piece with a pointed front").
	fillNose(rect, body, facing)
	ring := func() {
		fillDisc(body, ink.Black)
		fillDisc(pad(body, body.Dx()/7+2), ink.White)
	}
	switch id {
	case 0: // you: solid disc
		fillDisc(body, ink.Black)
	case 1: // ring
		ring()
	case 2: // striped disc: black disc cut by white bars
		fillDisc(body, ink.Black)
		bar := body.Dy() / 7
		for _, y0 := range []int{body.Min.Y + body.Dy()/3, body.Min.Y + 2*body.Dy()/3} {
			for dy := 0; dy < bar; dy++ {
				ink.DrawLine(image.Pt(body.Min.X, y0+dy), image.Pt(body.Max.X, y0+dy), ink.White)
			}
		}
	case 3: // bullseye: ring with a solid centre dot
		ring()
		fillDisc(pad(body, body.Dx()*5/16), ink.Black)
	}
	// small id number in the top-left corner of the cell (secondary aid)
	f.Small.SetActive(ink.DarkGray)
	ink.DrawString(image.Pt(rect.Min.X+4, rect.Min.Y+2), itoa(id+1))
}

func drawNose(body image.Rectangle, facing game.Dir, col color.Color) {
	c := center(body)
	s := body.Dx() / 4
	var a, b, tip image.Point
	switch facing {
	case game.N:
		tip = image.Pt(c.X, body.Min.Y)
		a = image.Pt(c.X-s, body.Min.Y+s)
		b = image.Pt(c.X+s, body.Min.Y+s)
	case game.S:
		tip = image.Pt(c.X, body.Max.Y)
		a = image.Pt(c.X-s, body.Max.Y-s)
		b = image.Pt(c.X+s, body.Max.Y-s)
	case game.E:
		tip = image.Pt(body.Max.X, c.Y)
		a = image.Pt(body.Max.X-s, c.Y-s)
		b = image.Pt(body.Max.X-s, c.Y+s)
	default:
		tip = image.Pt(body.Min.X, c.Y)
		a = image.Pt(body.Min.X+s, c.Y-s)
		b = image.Pt(body.Min.X+s, c.Y+s)
	}
	for o := 0; o <= 2; o++ {
		ink.DrawLine(tip, a.Add(image.Pt(0, o)), col)
		ink.DrawLine(tip, b.Add(image.Pt(0, o)), col)
		ink.DrawLine(a, b, col)
	}
}

// --- Card faces ------------------------------------------------------------

func drawCardFace(rect image.Rectangle, c game.Card, f *Fonts, dim bool) {
	var col color.Color = ink.Black
	if dim {
		col = ink.LightGray
	}
	ink.DrawRect(rect, col)
	ink.DrawRect(pad(rect, 1), col)
	drawCardGlyph(pad(rect, 8), c, col)
	f.Small.SetActive(col)
	lbl := c.Label()
	w := ink.StringWidth(lbl)
	ink.DrawString(image.Pt(rect.Min.X+(rect.Dx()-w)/2, rect.Max.Y-32), lbl)
}

func drawCardGlyph(r image.Rectangle, c game.Card, col color.Color) {
	cx := (r.Min.X + r.Max.X) / 2
	top := r.Min.Y + 8
	bot := r.Max.Y - 40
	mid := (top + bot) / 2
	arrowUp := func(x, yTip, yTail int) {
		for o := 0; o <= 2; o++ {
			ink.DrawLine(image.Pt(x+o, yTip), image.Pt(x+o, yTail), col)
		}
		ink.DrawLine(image.Pt(x, yTip), image.Pt(x-10, yTip+14), col)
		ink.DrawLine(image.Pt(x, yTip), image.Pt(x+10, yTip+14), col)
	}
	switch c {
	case game.Move1, game.Move2, game.Move3:
		arrowUp(cx, top, bot)
	case game.BackUp:
		// down arrow
		for o := 0; o <= 2; o++ {
			ink.DrawLine(image.Pt(cx+o, top), image.Pt(cx+o, bot), col)
		}
		ink.DrawLine(image.Pt(cx, bot), image.Pt(cx-10, bot-14), col)
		ink.DrawLine(image.Pt(cx, bot), image.Pt(cx+10, bot-14), col)
	case game.RotR:
		ink.DrawLine(image.Pt(cx, bot), image.Pt(cx, mid), col)
		ink.DrawLine(image.Pt(cx, mid), image.Pt(r.Max.X-12, mid), col)
		ink.DrawLine(image.Pt(r.Max.X-12, mid), image.Pt(r.Max.X-24, mid-10), col)
		ink.DrawLine(image.Pt(r.Max.X-12, mid), image.Pt(r.Max.X-24, mid+10), col)
	case game.RotL:
		ink.DrawLine(image.Pt(cx, bot), image.Pt(cx, mid), col)
		ink.DrawLine(image.Pt(cx, mid), image.Pt(r.Min.X+12, mid), col)
		ink.DrawLine(image.Pt(r.Min.X+12, mid), image.Pt(r.Min.X+24, mid-10), col)
		ink.DrawLine(image.Pt(r.Min.X+12, mid), image.Pt(r.Min.X+24, mid+10), col)
	case game.UTurn:
		ink.DrawLine(image.Pt(cx-12, bot), image.Pt(cx-12, top+10), col)
		ink.DrawLine(image.Pt(cx-12, top+10), image.Pt(cx+12, top+10), col)
		ink.DrawLine(image.Pt(cx+12, top+10), image.Pt(cx+12, bot), col)
		ink.DrawLine(image.Pt(cx+12, bot), image.Pt(cx+2, bot-12), col)
		ink.DrawLine(image.Pt(cx+12, bot), image.Pt(cx+22, bot-12), col)
	}
}

// --- Rules & splash --------------------------------------------------------

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

var rulesParagraphs = []string{
	"Mål: kör din robot till varje kontrollpunkt i rätt ordning (1, 2, 3 …) före datorrobotarna.",
	"Varje runda får du kort. Lägg fem av dem i de fem registren — det är din hemliga plan. Datorn programmerar samtidigt och ser inte dina kort, precis som du inte ser deras.",
	"Registren körs ett i taget. Den robot som är närmast prioritetsantennen rör sig först. Robotar knuffar varandra — rakt ner i hål eller ut över kanten!",
	"Efter varje register agerar brädet: transportband (dubbel pil = expressband, flyttar två) för dig, kugghjul vrider dig, vägglasrar och robotlasrar ger skada.",
	"Faller du i ett hål eller ut över kanten återuppstår du vid din senaste kontrollpunkt med två skadepoäng. Mer skada = färre kort nästa runda.",
	"På brädet: robotarna är runda pjäser med en nos som visar riktningen. Varje robot har ett eget mönster — 1 (du) fylld, 2 ring, 3 randig, 4 måltavla. Kontrollpunkter är numrerade rutor; startrutor är markerade med hörn och S1–S4.",
	"Tryck på ett handkort för att lägga det i nästa register. Tryck på ett register för att ta bort kortet. Tryck Kör när alla fem är fyllda.",
}

type motifFunc func(box image.Rectangle)

func DrawSplash(screen image.Point, f *Fonts, title string, motif motifFunc) {
	ink.ClearScreen()
	H := usableH
	f.Title.SetActive(ink.Black)
	drawCentered(image.Rect(0, H/6-40, screen.X, H/6+40), title, 72)
	side := screen.X * 3 / 5
	box := image.Rect((screen.X-side)/2, (H-side)/2, (screen.X+side)/2, (H+side)/2)
	motif(box)
	f.Body.SetActive(ink.DarkGray)
	drawCentered(image.Rect(0, H*5/6-30, screen.X, H*5/6+30), "Tryck för att börja", 32)
}

func drawSplashMotif(box image.Rectangle) {
	// a robot on a tile, a belt double-chevron, and a checkpoint flag
	cell := box.Dx() / 3
	robotCell := image.Rect(box.Min.X, box.Min.Y, box.Min.X+cell, box.Min.Y+cell)
	drawTileFrame(robotCell)
	drawNoseBox(robotCell, game.E)
	belt := image.Rect(box.Min.X+cell, box.Min.Y, box.Min.X+2*cell, box.Min.Y+cell)
	drawTileFrame(belt)
	drawBeltGlyph(belt, game.E, true)
	cp := image.Rect(box.Max.X-cell, box.Max.Y-cell, box.Max.X, box.Max.Y)
	for o := 0; o <= 2; o++ {
		ink.DrawRect(pad(cp, o), ink.Black)
	}
	c := center(cp)
	ink.DrawLine(image.Pt(c.X, cp.Min.Y+10), image.Pt(c.X, cp.Max.Y-10), ink.Black)
	ink.FillArea(image.Rect(c.X, cp.Min.Y+10, c.X+cell/3, cp.Min.Y+10+cell/4), ink.Black)
}

func drawTileFrame(r image.Rectangle) { ink.DrawRect(r, ink.Black) }

func drawNoseBox(rect image.Rectangle, facing game.Dir) {
	body := pad(rect, rect.Dx()/4)
	ink.FillArea(body, ink.Black)
	drawNose(body, facing, ink.White)
}

func DrawRules(screen image.Point, f *Fonts, title string, paragraphs []string) image.Rectangle {
	ink.ClearScreen()
	H := usableH
	f.Big.SetActive(ink.Black)
	drawCentered(image.Rect(0, 40, screen.X, 110), title, 44)

	margin := 40
	bh := 100
	bw := screen.X / 2
	back := image.Rect((screen.X-bw)/2, H-margin-bh, (screen.X+bw)/2, H-margin)
	drawBtn(back, "Tillbaka", f.Big, true)

	f.Body.SetActive(ink.Black)
	bm := 60
	maxW := screen.X - 2*bm
	y := 150
	lineH := 44
	paraGap := 22
	limit := back.Min.Y - 20
	for _, p := range paragraphs {
		for _, ln := range wrapText(p, maxW) {
			if y+lineH > limit {
				break
			}
			ink.DrawString(image.Pt(bm, y), ln)
			y += lineH
		}
		y += paraGap
	}
	return back
}
