package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"studie/story"
)

// Per-room illustration band for the mystery. Same reusable primitive as the
// cave (a LocID→scene registry drawn in a framed strip below the header, on the
// shared drawing toolkit), but the scenes are authored for this story's register
// — fog, gaslight, a study with a body. A room without a scene simply shows none.
type vignette func(r image.Rectangle)

var vignettes = map[story.LocID]vignette{
	story.LOC_STREET: vigStreet,
	story.LOC_HALL:   vigHall,
	story.LOC_STUDY:  vigStudy,
}

func roomVignette(loc story.LocID) vignette { return vignettes[loc] }

const vigBandHeight = 240

func drawVignetteBand(r image.Rectangle, v vignette) {
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 1), ink.Black)
	v(pad(r, 20))
}

// stroke draws a 2–3px line by nudging a few 1px copies (the SDK has no width).
func stroke(p1, p2 image.Point) {
	ink.DrawLine(p1, p2, ink.Black)
	ink.DrawLine(p1.Add(image.Pt(1, 0)), p2.Add(image.Pt(1, 0)), ink.Black)
	ink.DrawLine(p1.Add(image.Pt(0, 1)), p2.Add(image.Pt(0, 1)), ink.Black)
}

// fog lays a low haze of stipple plus a couple of drifting gray strands.
func fog(r image.Rectangle, salt int) {
	stipple(r, 11, salt, func(p image.Point) bool { return rnd(p.X, p.Y, salt) > 0.7 }, ink.LightGray)
	for i := 0; i < 2; i++ {
		y := r.Min.Y + r.Dy()/3 + i*r.Dy()/4
		prev := image.Pt(r.Min.X, y)
		for x := r.Min.X; x <= r.Max.X; x += r.Dx() / 24 {
			off := int(6 * sinApprox(float64(x-r.Min.X)/float64(r.Dx())*9))
			p := image.Pt(x, y+off)
			ink.DrawLine(prev, p, ink.LightGray)
			prev = p
		}
	}
}

// vigStreet: a foggy gaslit street — a lamp, the lodging-house, a hansom.
func vigStreet(r image.Rectangle) {
	W, H := r.Dx(), r.Dy()
	groundY := r.Max.Y - H/6

	// A tall building silhouette on the right, with two lit windows.
	b := image.Rect(r.Max.X-W/3, r.Min.Y+H/8, r.Max.X, groundY)
	ink.DrawRect(b, ink.Black)
	crossHatch(b, 16, ink.LightGray)
	for _, wx := range []int{b.Min.X + W/12, b.Min.X + W/5} {
		win := image.Rect(wx, b.Min.Y+H/6, wx+W/16, b.Min.Y+H/6+H/8)
		ink.FillArea(win, ink.White)
		ink.DrawRect(win, ink.Black)
	}
	// The open door, a slab of dark.
	door := image.Rect(b.Min.X+W/9, groundY-H/3, b.Min.X+W/9+W/12, groundY)
	ink.FillArea(door, ink.Black)

	// A gas street-lamp on the left, casting a stippled halo.
	lx := r.Min.X + W/5
	stroke(image.Pt(lx, groundY), image.Pt(lx, r.Min.Y+H/4))
	lamp := image.Rect(lx-W/28, r.Min.Y+H/8, lx+W/28, r.Min.Y+H/4)
	ink.DrawRect(lamp, ink.Black)
	stroke(image.Pt(lamp.Min.X, lamp.Min.Y), image.Pt(lamp.Max.X, lamp.Max.Y))
	stroke(image.Pt(lamp.Max.X, lamp.Min.Y), image.Pt(lamp.Min.X, lamp.Max.Y))
	glowC := image.Pt(lx, (lamp.Min.Y+lamp.Max.Y)/2)
	rr := W / 6
	stipple(image.Rect(lx-rr, glowC.Y-H/5, lx+rr, glowC.Y+H/5), 10, 3,
		func(p image.Point) bool {
			dx, dy := p.X-glowC.X, p.Y-glowC.Y
			d := dx*dx + dy*dy
			return d < rr*rr && rnd(p.X, p.Y, 5) > float64(d)/float64(rr*rr)
		}, ink.LightGray)

	// A hansom cab waiting at the kerb (two wheels + a body).
	hx := r.Min.X + W/2
	discOutline(image.Pt(hx, groundY-H/10), H/10, ink.Black)
	discOutline(image.Pt(hx+W/8, groundY-H/10), H/10, ink.Black)
	cab := image.Rect(hx-H/12, groundY-H/3, hx+W/8+H/12, groundY-H/8)
	ink.DrawRect(cab, ink.Black)
	crossHatch(cab, 12, ink.LightGray)

	// Ground and fog over everything low.
	stroke(image.Pt(r.Min.X, groundY), image.Pt(r.Max.X, groundY))
	hatchRect(image.Rect(r.Min.X, groundY+2, r.Max.X, groundY+H/14), 16, ink.LightGray, false)
	fog(image.Rect(r.Min.X, r.Min.Y+H/6, r.Max.X, groundY), 1)
}

// vigHall: the narrow gaslit hall — a cold hearth with the burned letter, a
// door to the study, a back stair down to the yard.
func vigHall(r image.Rectangle) {
	W, H := r.Dx(), r.Dy()
	floorY := r.Max.Y - H/8
	stroke(image.Pt(r.Min.X, floorY), image.Pt(r.Max.X, floorY))

	// A fireplace on the left with cold ashes (dense stipple).
	fp := image.Rect(r.Min.X+W/12, floorY-H*3/5, r.Min.X+W/12+W/4, floorY)
	ink.DrawRect(fp, ink.Black)
	ink.DrawRect(pad(fp, 1), ink.Black)
	mouth := pad(fp, 12)
	mouth.Min.Y = fp.Min.Y + fp.Dy()/3
	crossHatch(mouth, 9, ink.DarkGray)
	stipple(image.Rect(mouth.Min.X, mouth.Max.Y-H/10, mouth.Max.X, mouth.Max.Y), 7, 2,
		func(p image.Point) bool { return rnd(p.X, p.Y, 4) > 0.5 }, ink.LightGray)

	// A gas sconce on the wall, glowing faintly.
	sx := r.Min.X + W/2
	sconce := image.Rect(sx-W/40, r.Min.Y+H/6, sx+W/40, r.Min.Y+H/6+H/10)
	ink.DrawRect(sconce, ink.Black)
	stipple(image.Rect(sx-W/12, r.Min.Y+H/12, sx+W/12, r.Min.Y+H/3), 10, 6,
		func(p image.Point) bool {
			dx, dy := p.X-sx, p.Y-(r.Min.Y+H/6)
			return dx*dx+dy*dy < (W/12)*(W/12) && rnd(p.X, p.Y, 7) > 0.5
		}, ink.LightGray)

	// A door to the study (right) and a descending stair before it.
	door := image.Rect(r.Max.X-W/4, floorY-H*3/5, r.Max.X-W/12, floorY)
	ink.DrawRect(door, ink.Black)
	ink.DrawRect(pad(door, 1), ink.Black)
	knob := image.Pt(door.Max.X-W/40, (door.Min.Y+door.Max.Y)/2)
	ink.FillArea(image.Rect(knob.X-3, knob.Y-3, knob.X+3, knob.Y+3), ink.Black)
	for i := 0; i < 3; i++ {
		y := floorY - i*H/16
		x := door.Min.X - W/16 - i*W/20
		stroke(image.Pt(x, y), image.Pt(x+W/16, y))
		stroke(image.Pt(x, y), image.Pt(x, y+H/16))
	}
}

// vigStudy: the victim's study — a slumped figure over a desk, a foggy window,
// a stopped clock on the shelf.
func vigStudy(r image.Rectangle) {
	W, H := r.Dx(), r.Dy()
	floorY := r.Max.Y - H/8
	stroke(image.Pt(r.Min.X, floorY), image.Pt(r.Max.X, floorY))

	// A tall window, right, with fog pressing at the glass.
	win := image.Rect(r.Max.X-W/3, r.Min.Y+H/10, r.Max.X-W/12, floorY-H/6)
	ink.DrawRect(win, ink.Black)
	ink.DrawRect(pad(win, 1), ink.Black)
	crossHatch(pad(win, 3), 16, ink.LightGray)
	stroke(image.Pt((win.Min.X+win.Max.X)/2, win.Min.Y), image.Pt((win.Min.X+win.Max.X)/2, win.Max.Y))
	stroke(image.Pt(win.Min.X, (win.Min.Y+win.Max.Y)/2), image.Pt(win.Max.X, (win.Min.Y+win.Max.Y)/2))

	// The desk, centre-left, with two squared papers.
	deskTop := floorY - H/3
	desk := image.Rect(r.Min.X+W/10, deskTop, r.Min.X+W/10+W/2, deskTop+H/22)
	ink.FillArea(desk, ink.Black)
	stroke(image.Pt(desk.Min.X+W/24, desk.Max.Y), image.Pt(desk.Min.X+W/24, floorY))
	stroke(image.Pt(desk.Max.X-W/24, desk.Max.Y), image.Pt(desk.Max.X-W/24, floorY))
	for i := 0; i < 2; i++ {
		pp := image.Rect(desk.Min.X+W/8+i*W/9, deskTop-H/40, desk.Min.X+W/8+i*W/9+W/12, deskTop)
		ink.FillArea(pp, ink.White)
		ink.DrawRect(pp, ink.Black)
	}

	// The slumped figure: head and shoulders fallen across the desk.
	hx := desk.Min.X + W/6
	discOutline(image.Pt(hx, deskTop-H/16), H/14, ink.Black)               // head
	arc(image.Pt(hx+W/12, deskTop-H/14), W/8, H/12, 160, 300, ink.Black)   // shoulder/back
	stroke(image.Pt(hx+W/6, deskTop-H/22), image.Pt(hx+W/3, deskTop-H/60)) // outflung arm

	// A stopped mantel clock on a shelf, upper left, hands set askew.
	clk := image.Pt(r.Min.X+W/8, r.Min.Y+H/5)
	discOutline(clk, H/12, ink.Black)
	stroke(clk, image.Pt(clk.X+H/20, clk.Y-H/30))
	stroke(clk, image.Pt(clk.X-H/40, clk.Y+H/24))
	stroke(image.Pt(clk.X-W/8, clk.Y+H/12), image.Pt(clk.X+W/8, clk.Y+H/12)) // shelf
}
