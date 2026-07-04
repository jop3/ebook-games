package main

import (
	"image"

	ink "github.com/dennwc/inkview"

	"grottan/story"
)

// A vignette is a small monochrome line-art scene drawn into the header band of
// the game screen for rooms that have one. It's the reusable "illustrated
// adventure" primitive: a themed story (mystery/horror) registers its own
// per-room vignettes against the same band, so the engine stays text-only while
// the presentation can be as illustrated as the art budget allows. Static line
// art is exactly what greyscale e-ink renders best.
//
// Vignettes live in the UI layer (they draw via ink), keeping the story package
// pure. A room with no registered vignette simply shows none, and the transcript
// gets the reclaimed space. Vignettes are suppressed in the dark — you can't see
// the room, so you shouldn't see its picture either.
type vignette func(r image.Rectangle)

var vignettes = map[story.LocID]vignette{
	story.LOC_START:    vigRoad,
	story.LOC_BUILDING: vigWellhouse,
	story.LOC_GRATE:    vigGrate,
	story.LOC_MISTHALL: vigMist,
}

func roomVignette(loc story.LocID) vignette { return vignettes[loc] }

// vigBandHeight is the height of the illustration strip below the header.
const vigBandHeight = 240

// drawVignetteBand frames the band and draws the scene inside a padded rect.
func drawVignetteBand(r image.Rectangle, v vignette) {
	ink.DrawRect(r, ink.Black)
	ink.DrawRect(pad(r, 1), ink.Black)
	v(pad(r, 20))
}

// --- shared drawing helpers --------------------------------------------------

// stroke draws a 2–3px line by nudging a few 1px copies (the SDK has no width).
func stroke(p1, p2 image.Point) {
	ink.DrawLine(p1, p2, ink.Black)
	ink.DrawLine(p1.Add(image.Pt(1, 0)), p2.Add(image.Pt(1, 0)), ink.Black)
	ink.DrawLine(p1.Add(image.Pt(0, 1)), p2.Add(image.Pt(0, 1)), ink.Black)
}

func ellipse(c image.Point, rw, rh int) {
	const tau = 6.28318530718
	var prev image.Point
	steps := 40
	for i := 0; i <= steps; i++ {
		a := float64(i) / float64(steps) * tau
		p := image.Pt(c.X+int(float64(rw)*cos(a)), c.Y+int(float64(rh)*sin(a)))
		if i > 0 {
			stroke(prev, p)
		}
		prev = p
	}
}

// wavyLine draws a horizontal sine ripple from x1 to x2 at height y (used for
// water and mist).
func wavyLine(x1, x2, y, amp, cycles int) {
	steps := 28
	prev := image.Pt(x1, y)
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := x1 + int(float64(x2-x1)*t)
		off := int(float64(amp) * sinApprox(t*float64(cycles)*3.14159265))
		p := image.Pt(x, y+off)
		ink.DrawLine(prev, p, ink.DarkGray)
		prev = p
	}
}

// drawTree draws a broadleaf tree: a stippled canopy of overlapping foliage
// clouds over a trunk, with a soft ground shadow.
func drawTree(base image.Point, h, salt int) {
	arc(image.Pt(base.X, base.Y), h/5, h/22, 180, 360, ink.LightGray) // ground shadow
	trunkW := h / 22
	if trunkW < 2 {
		trunkW = 2
	}
	ink.FillArea(image.Rect(base.X-trunkW, base.Y-h*2/5, base.X+trunkW, base.Y), ink.DarkGray)
	foliage(image.Pt(base.X, base.Y-h*2/3), h/3, salt)
	foliage(image.Pt(base.X-h/8, base.Y-h/2), h/5, salt+3)
	foliage(image.Pt(base.X+h/8, base.Y-h/2), h/5, salt+5)
}

// drawStream draws a little brook: two rippled banks with cross glints.
func drawStream(x1, x2, y int) {
	wavyLine(x1, x2, y, 6, 6)
	wavyLine(x1, x2, y+8, 5, 6)
	for x := x1 + 15; x < x2; x += 40 {
		ink.DrawLine(image.Pt(x, y-4), image.Pt(x+8, y-4), ink.LightGray)
	}
}

// drawPillar draws a fluted stone column with a capital, base, and shaded side.
func drawPillar(base image.Point, h, w int) {
	shaft := image.Rect(base.X-w, base.Y-h, base.X+w, base.Y)
	ink.DrawRect(shaft, ink.Black)
	for i := -1; i <= 1; i++ { // fluting
		x := base.X + i*w/2
		ink.DrawLine(image.Pt(x, base.Y-h+8), image.Pt(x, base.Y-8), ink.LightGray)
	}
	hatchRect(image.Rect(base.X+w/3, base.Y-h+8, base.X+w, base.Y-8), 9, ink.LightGray, false)
	capB := image.Rect(base.X-w-6, base.Y-h-12, base.X+w+6, base.Y-h)
	ink.DrawRect(capB, ink.Black)
	ink.DrawRect(pad(capB, 1), ink.Black)
	bas := image.Rect(base.X-w-6, base.Y-12, base.X+w+6, base.Y)
	ink.DrawRect(bas, ink.Black)
	ink.DrawRect(pad(bas, 1), ink.Black)
}

// --- the cave's sample vignettes ---------------------------------------------

// vigRoad: the opening scene — a brick well house, a moonlit forest, a stream.
func vigRoad(r image.Rectangle) {
	W, H := r.Dx(), r.Dy()
	groundY := r.Max.Y - H/6

	// Moon with a crater, in clear sky above the hills.
	moon := image.Pt(r.Min.X+W*9/20, r.Min.Y+H/6)
	discOutline(moon, H/12, ink.DarkGray)
	discOutline(image.Pt(moon.X+H/40, moon.Y-H/50), H/44, ink.LightGray)

	// Distant hills.
	arc(image.Pt(r.Min.X+W*2/5, groundY), W/4, H/4, 0, 180, ink.LightGray)
	arc(image.Pt(r.Min.X+W*3/5, groundY), W/5, H/6, 0, 180, ink.LightGray)

	// Ground with a strip of soil texture.
	stroke(image.Pt(r.Min.X, groundY), image.Pt(r.Max.X, groundY))
	hatchRect(image.Rect(r.Min.X, groundY+2, r.Max.X, groundY+H/12), 16, ink.LightGray, false)

	// The brick well house, right of centre.
	bw, bh := W/4, H*3/5
	bx1 := r.Max.X - bw - W/12
	by1 := groundY - bh
	body := image.Rect(bx1, by1, bx1+bw, groundY)
	ink.DrawRect(body, ink.Black)
	ink.DrawRect(pad(body, 1), ink.Black)
	brickWall(pad(body, 3), H/14, W/16, ink.LightGray)
	// Gable roof with shingle courses.
	apex := image.Pt(bx1+bw/2, by1-H/4)
	stroke(image.Pt(bx1-10, by1), apex)
	stroke(apex, image.Pt(bx1+bw+10, by1))
	stroke(image.Pt(bx1-10, by1), image.Pt(bx1+bw+10, by1))
	for i := 1; i < 4; i++ {
		yy := by1 - i*(H/4)/4
		hw := (bw + 20) * (4 - i) / 8
		stroke(image.Pt(apex.X-hw, yy), image.Pt(apex.X+hw, yy))
	}
	// Dark doorway (hatched interior) and a paned window.
	door := image.Rect(bx1+bw/2-bw/8, groundY-bh/2, bx1+bw/2+bw/8, groundY)
	ink.DrawRect(door, ink.Black)
	crossHatch(pad(door, 2), 8, ink.DarkGray)
	win := image.Rect(bx1+bw-bw/3, by1+bh/6, bx1+bw-bw/12, by1+bh/6+bh/6)
	ink.DrawRect(win, ink.Black)
	stroke(image.Pt((win.Min.X+win.Max.X)/2, win.Min.Y), image.Pt((win.Min.X+win.Max.X)/2, win.Max.Y))
	stroke(image.Pt(win.Min.X, (win.Min.Y+win.Max.Y)/2), image.Pt(win.Max.X, (win.Min.Y+win.Max.Y)/2))

	// Foreground trees and the stream out of the building.
	drawTree(image.Pt(r.Min.X+W/8, groundY), H*3/4, 11)
	drawTree(image.Pt(r.Min.X+W/4+W/40, groundY), H*3/5, 23)
	drawStream(r.Min.X+W/3, bx1, groundY+H/14)
}

// vigWellhouse: inside the well house — a covered stone spring with a bucket.
func vigWellhouse(r image.Rectangle) {
	W, H := r.Dx(), r.Dy()
	cx := r.Min.X + W/2
	groundY := r.Max.Y - H/8
	stroke(image.Pt(r.Min.X, groundY), image.Pt(r.Max.X, groundY))
	hatchRect(image.Rect(r.Min.X, groundY+2, r.Max.X, groundY+H/14), 18, ink.LightGray, true)

	rw, rh := W/6, H/12
	ringY := groundY - H/3
	// Stone shaft with block texture.
	shaft := image.Rect(cx-rw, ringY, cx+rw, groundY)
	brickWall(shaft, H/12, rw/2, ink.LightGray)
	stroke(image.Pt(cx-rw, ringY), image.Pt(cx-rw, groundY))
	stroke(image.Pt(cx+rw, ringY), image.Pt(cx+rw, groundY))
	// Rim + dark water with a highlight.
	ellipse(image.Pt(cx, ringY), rw, rh)
	ellipse(image.Pt(cx, ringY), rw-4, rh-2)
	stipple(image.Rect(cx-rw, ringY-rh, cx+rw, ringY+rh), 8, 2,
		func(p image.Point) bool {
			dx := p.X - cx
			dy := (p.Y - ringY) * rw / rh
			return dx*dx+dy*dy < (rw-8)*(rw-8)
		}, ink.LightGray)
	wavyLine(cx-rw+10, cx+rw-10, ringY, 3, 5)

	// Roof on two posts with a crossbeam and plank courses.
	postT := groundY - H*3/5
	stroke(image.Pt(cx-rw, ringY), image.Pt(cx-rw, postT))
	stroke(image.Pt(cx+rw, ringY), image.Pt(cx+rw, postT))
	beamY := postT + H/24
	stroke(image.Pt(cx-rw-12, beamY), image.Pt(cx+rw+12, beamY))
	apex := image.Pt(cx, postT-H/5)
	stroke(image.Pt(cx-rw-16, postT), apex)
	stroke(apex, image.Pt(cx+rw+16, postT))
	for i := 1; i < 3; i++ {
		yy := postT - i*(H/5)/3
		hw := (rw + 16) * (3 - i) / 3
		stroke(image.Pt(apex.X-hw, yy), image.Pt(apex.X+hw, yy))
	}

	// Bucket on a rope from the beam.
	bucketY := ringY - rh - H/12
	stroke(image.Pt(cx, beamY), image.Pt(cx, bucketY))
	bucket := image.Rect(cx-W/28, bucketY, cx+W/28, bucketY+H/14)
	ink.DrawRect(bucket, ink.Black)
	ink.DrawRect(pad(bucket, 1), ink.Black)
	arc(image.Pt(cx, bucketY), W/28, H/24, 0, 180, ink.Black)
}

// vigGrate: the depression with the locked steel grate set into rock.
func vigGrate(r image.Rectangle) {
	W, H := r.Dx(), r.Dy()
	rimY := r.Min.Y + H/3
	dipY := r.Min.Y + H/2
	gx1, gx2 := r.Min.X+W*3/8, r.Min.X+W*5/8

	// Rocky slopes into the depression, hatched for rock.
	stroke(image.Pt(r.Min.X, rimY), image.Pt(r.Min.X+W/4, rimY))
	stroke(image.Pt(r.Min.X+W/4, rimY), image.Pt(gx1, dipY))
	stroke(image.Pt(gx2, dipY), image.Pt(r.Min.X+W*3/4, rimY))
	stroke(image.Pt(r.Min.X+W*3/4, rimY), image.Pt(r.Max.X, rimY))
	hatchRect(image.Rect(r.Min.X+W/4, rimY, gx1, dipY), 12, ink.LightGray, true)
	hatchRect(image.Rect(gx2, rimY, r.Min.X+W*3/4, dipY), 12, ink.LightGray, false)
	stipple(image.Rect(r.Min.X, rimY-H/10, r.Max.X, rimY), 14, 4,
		func(p image.Point) bool { return rnd(p.X, p.Y, 9) > 0.6 }, ink.LightGray)

	// The grate over darkness: a shadow fill under the barred rectangle.
	grate := image.Rect(gx1, dipY, gx2, dipY+H/3)
	crossHatch(grate, 7, ink.DarkGray)
	ink.DrawRect(grate, ink.Black)
	ink.DrawRect(pad(grate, 1), ink.Black)
	for i := 1; i < 6; i++ {
		x := gx1 + i*(gx2-gx1)/6
		stroke(image.Pt(x, grate.Min.Y), image.Pt(x, grate.Max.Y))
	}
	for i := 1; i < 3; i++ {
		y := grate.Min.Y + i*grate.Dy()/3
		stroke(image.Pt(gx1, y), image.Pt(gx2, y))
	}

	// Dry streambed leading in, and a scatter of pebbles.
	for x := r.Min.X + W/12; x < gx1-6; x += 22 {
		ink.DrawLine(image.Pt(x, rimY-H/6), image.Pt(x+12, rimY-H/6), ink.LightGray)
	}
	stipple(image.Rect(gx1-W/8, dipY-8, gx2+W/8, dipY+8), 18, 2,
		func(p image.Point) bool { return rnd(p.X, p.Y, 3) > 0.7 }, ink.DarkGray)
}

// vigMist: the Hall of Mists — fluted pillars, a shaded stair, layered mist.
func vigMist(r image.Rectangle) {
	W, H := r.Dx(), r.Dy()
	floorY := r.Max.Y - H/8
	stroke(image.Pt(r.Min.X, floorY), image.Pt(r.Max.X, floorY))
	hatchRect(image.Rect(r.Min.X, floorY+2, r.Max.X, floorY+H/14), 20, ink.LightGray, false)

	// Two columns on the left.
	drawPillar(image.Pt(r.Min.X+W/6, floorY), H*3/5, W/18)
	drawPillar(image.Pt(r.Min.X+W/3, floorY), H*3/5, W/18)

	// A stone stair descending on the right, risers in shadow.
	steps, sw, sh := 5, W/16, H/12
	sx := r.Min.X + W*7/12
	for i := 0; i < steps; i++ {
		x, y := sx+i*sw, floorY-i*sh
		ink.FillArea(image.Rect(x, y-3, x+sw, y), ink.Black) // tread edge
		hatchRect(image.Rect(x+sw-4, y-sh, x+sw, y), 5, ink.DarkGray, false)
		stroke(image.Pt(x+sw, y), image.Pt(x+sw, y-sh))
		stroke(image.Pt(x, y), image.Pt(x+sw, y))
	}

	// Layered mist: wavy strands plus a faint stipple haze between them.
	for i := 0; i < 3; i++ {
		wavyLine(r.Min.X+W/12, r.Max.X-W/12, r.Min.Y+H/6+i*H/9, 7, 5)
	}
	stipple(image.Rect(r.Min.X+W/12, r.Min.Y+H/8, r.Max.X-W/12, r.Min.Y+H/2), 12, 6,
		func(p image.Point) bool { return rnd(p.X, p.Y, 1) > 0.75 }, ink.LightGray)
}
