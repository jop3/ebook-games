package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"roborally/game"
)

const uiMargin = 30

func (a *app) drawStatusStrip(sz image.Point, text string) {
	strip := image.Rect(0, 40, sz.X, 118)
	ink.FillArea(strip, ink.White)
	a.fonts.Status.SetActive(ink.Black)
	w := ink.StringWidth(text)
	ink.DrawString(image.Pt((sz.X-w)/2, 54), text)
	ink.DrawLine(image.Pt(0, strip.Max.Y), image.Pt(sz.X, strip.Max.Y), ink.Black)
}

func (a *app) bottomBar(sz image.Point, left string, leftOK bool, right string) (lr, rr image.Rectangle) {
	H := usableH
	bh := 110
	y0 := H - 40 - bh
	gap := 24
	bw := (sz.X - 2*uiMargin - gap) / 2
	lr = image.Rect(uiMargin, y0, uiMargin+bw, y0+bh)
	rr = image.Rect(sz.X-uiMargin-bw, y0, sz.X-uiMargin, y0+bh)
	drawBtn(lr, left, a.fonts.Big, leftOK)
	drawBtn(rr, right, a.fonts.Big, true)
	return
}

func (a *app) humanStatus() string {
	r := &a.gs.Robots[0]
	goal := "Mål: " + itoa(int(r.NextCheck)) + "/" + itoa(int(a.gs.Board.NCheck))
	if r.NextCheck > a.gs.Board.NCheck {
		goal = "Mål: klart!"
	}
	return "Runda " + itoa(a.gs.Round) + "  ·  Skada " + itoa(r.Damage) + "  ·  " + goal
}

// --- Program screen --------------------------------------------------------

func (a *app) drawProgram(sz image.Point) {
	ink.ClearScreen()
	a.drawStatusStrip(sz, a.humanStatus())

	// Register row.
	regTop := 150
	regH := 160
	gap := 16
	regW := (sz.X - 2*uiMargin - 4*gap) / 5
	a.fonts.Small.SetActive(ink.Black)
	for r := 0; r < 5; r++ {
		x := uiMargin + r*(regW+gap)
		rect := image.Rect(x, regTop, x+regW, regTop+regH)
		a.regRects[r] = rect
		card := a.gs.Registers[0][r]
		if card == game.CardNone {
			ink.DrawRect(rect, ink.DarkGray)
			a.fonts.Small.SetActive(ink.DarkGray)
			drawCentered(rect, itoa(r+1), 24)
		} else {
			drawCardFace(rect, card, a.fonts, false)
		}
	}

	// Hand grid (up to 9 cards, 3 columns).
	hand := a.gs.Hands[0]
	a.handRects = a.handRects[:0]
	handTop := regTop + regH + 40
	cols := 3
	cardW := (sz.X - 2*uiMargin - (cols-1)*gap) / cols
	cardH := 150
	bottomY := usableH - 40 - 110 - 30
	for i := range hand {
		row := i / cols
		col := i % cols
		x := uiMargin + col*(cardW+gap)
		y := handTop + row*(cardH+gap)
		if y+cardH > bottomY {
			break
		}
		rect := image.Rect(x, y, x+cardW, y+cardH)
		a.handRects = append(a.handRects, rect)
		drawCardFace(rect, hand[i], a.fonts, a.gs.HandCardUsed(i))
	}

	a.korBtn, a.menyBtn = a.bottomBar(sz, "Kör", a.gs.ProgramComplete(), "Meny")
}

// --- Resolve screen --------------------------------------------------------

func (a *app) drawResolve(sz image.Point) {
	ink.ClearScreen()
	a.drawStatusStrip(sz, "Register "+itoa(a.gs.CurReg+1)+"/5  ·  "+a.humanStatus())

	// Board fills the space between the status strip and the log/bottom bar.
	logH := 70
	bottomTop := usableH - 40 - 110 - logH - 20
	area := image.Rect(uiMargin, 140, sz.X-uiMargin, bottomTop)
	a.layout = newBoardLayout(area, a.gs.Board.W, a.gs.Board.H)
	drawBoard(a.layout, a.gs.Board, a.gs.Robots, a.fonts)

	// Log line (most recent).
	a.fonts.Body.SetActive(ink.Black)
	if n := len(a.gs.Log); n > 0 {
		msg := a.gs.Log[n-1]
		w := ink.StringWidth(msg)
		ink.DrawString(image.Pt((sz.X-w)/2, bottomTop+24), msg)
	}

	a.nastaBtn, a.menyBtn = a.bottomBar(sz, "Nästa", true, "Meny")
}

// --- Done screen -----------------------------------------------------------

func (a *app) drawDone(sz image.Point) (again, meny image.Rectangle) {
	ink.ClearScreen()
	H := usableH
	a.fonts.Title.SetActive(ink.Black)
	banner := "Robot " + itoa(a.gs.Winner+1) + " vann!"
	if a.gs.Winner == 0 {
		banner = "Du vann!"
	}
	drawCentered(image.Rect(0, H/5, sz.X, H/5+90), banner, 72)

	// Standings: each robot's checkpoint progress.
	a.fonts.Big.SetActive(ink.Black)
	y := H/5 + 160
	for i := range a.gs.Robots {
		r := &a.gs.Robots[i]
		who := "Robot " + itoa(i+1)
		if r.IsHuman {
			who = "Du"
		}
		reached := int(r.NextCheck) - 1
		if reached < 0 {
			reached = 0
		}
		line := who + ": " + itoa(reached) + "/" + itoa(int(a.gs.Board.NCheck)) + " kontrollpunkter"
		w := ink.StringWidth(line)
		ink.DrawString(image.Pt((sz.X-w)/2, y), line)
		y += 70
	}

	again, meny = a.bottomBar(sz, "Spela igen", true, "Meny")
	return
}
