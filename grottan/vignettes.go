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

// pine draws a simple 3-tier conifer with its base at `base`.
func pine(base image.Point, h int) {
	stroke(image.Pt(base.X, base.Y), image.Pt(base.X, base.Y-h/6))
	tiers := 3
	tierH := h * 5 / 6 / tiers
	top := base.Y - h/6
	half := h / 4
	for i := 0; i < tiers; i++ {
		yb := top - i*tierH*3/4
		yt := yb - tierH
		hw := half - i*half/4
		stroke(image.Pt(base.X-hw, yb), image.Pt(base.X, yt))
		stroke(image.Pt(base.X, yt), image.Pt(base.X+hw, yb))
		stroke(image.Pt(base.X-hw, yb), image.Pt(base.X+hw, yb))
	}
}

// --- the cave's sample vignettes ---------------------------------------------

// vigRoad: the opening scene — a brick well house beside a forest and a stream.
func vigRoad(r image.Rectangle) {
	W, H := r.Dx(), r.Dy()
	groundY := r.Max.Y - H/6
	stroke(image.Pt(r.Min.X, groundY), image.Pt(r.Max.X, groundY))

	// The little brick building, right of centre.
	bw, bh := W/4, H*3/5
	bx1 := r.Max.X - bw - W/10
	by1 := groundY - bh
	body := image.Rect(bx1, by1, bx1+bw, groundY)
	ink.DrawRect(body, ink.Black)
	ink.DrawRect(pad(body, 1), ink.Black)
	apex := image.Pt(bx1+bw/2, by1-H/4)
	stroke(image.Pt(bx1-8, by1), apex)
	stroke(apex, image.Pt(bx1+bw+8, by1))
	door := image.Rect(bx1+bw/2-bw/8, groundY-bh/2, bx1+bw/2+bw/8, groundY)
	ink.DrawRect(door, ink.Black)

	// A couple of pines on the left.
	pine(image.Pt(r.Min.X+W/8, groundY), H*3/4)
	pine(image.Pt(r.Min.X+W/4, groundY), H*3/5)

	// The stream out of the building, down the gully.
	wavyLine(r.Min.X+W/3, bx1, groundY+H/12, 6, 6)
}

// vigWellhouse: inside the well house — a covered spring.
func vigWellhouse(r image.Rectangle) {
	W, H := r.Dx(), r.Dy()
	cx := r.Min.X + W/2
	groundY := r.Max.Y - H/8
	stroke(image.Pt(r.Min.X, groundY), image.Pt(r.Max.X, groundY))

	rw, rh := W/6, H/12
	ringY := groundY - H/3
	// Well shaft walls + rim.
	stroke(image.Pt(cx-rw, ringY), image.Pt(cx-rw, groundY))
	stroke(image.Pt(cx+rw, ringY), image.Pt(cx+rw, groundY))
	ellipse(image.Pt(cx, ringY), rw, rh)
	// Water surface.
	wavyLine(cx-rw+8, cx+rw-8, ringY+H/16, 4, 5)

	// Pitched roof on two posts.
	postT := groundY - H*3/5
	stroke(image.Pt(cx-rw, ringY), image.Pt(cx-rw, postT))
	stroke(image.Pt(cx+rw, ringY), image.Pt(cx+rw, postT))
	apex := image.Pt(cx, postT-H/5)
	stroke(image.Pt(cx-rw-14, postT), apex)
	stroke(apex, image.Pt(cx+rw+14, postT))
}

// vigGrate: the depression with the locked steel grate set into the dirt.
func vigGrate(r image.Rectangle) {
	W, H := r.Dx(), r.Dy()
	rimY := r.Min.Y + H/3
	dipY := r.Min.Y + H/2
	gx1, gx2 := r.Min.X+W*3/8, r.Min.X+W*5/8

	// Ground sloping into a shallow depression.
	stroke(image.Pt(r.Min.X, rimY), image.Pt(r.Min.X+W/4, rimY))
	stroke(image.Pt(r.Min.X+W/4, rimY), image.Pt(gx1, dipY))
	stroke(image.Pt(gx2, dipY), image.Pt(r.Min.X+W*3/4, rimY))
	stroke(image.Pt(r.Min.X+W*3/4, rimY), image.Pt(r.Max.X, rimY))

	// The grate (top-down: a barred rectangle) in the floor of the dip.
	grate := image.Rect(gx1, dipY, gx2, dipY+H/3)
	ink.DrawRect(grate, ink.Black)
	ink.DrawRect(pad(grate, 1), ink.Black)
	for i := 1; i < 5; i++ {
		x := gx1 + i*(gx2-gx1)/5
		stroke(image.Pt(x, grate.Min.Y), image.Pt(x, grate.Max.Y))
	}
	for i := 1; i < 3; i++ {
		y := grate.Min.Y + i*(grate.Dy())/3
		stroke(image.Pt(gx1, y), image.Pt(gx2, y))
	}

	// A dry streambed leading in from the left (dashed).
	for x := r.Min.X + W/12; x < gx1; x += 22 {
		ink.DrawLine(image.Pt(x, rimY-H/6), image.Pt(x+12, rimY-H/6), ink.LightGray)
	}
}

// vigMist: the Hall of Mists — pillars, a descending stair, wisps of mist.
func vigMist(r image.Rectangle) {
	W, H := r.Dx(), r.Dy()
	floorY := r.Max.Y - H/8
	stroke(image.Pt(r.Min.X, floorY), image.Pt(r.Max.X, floorY))

	// Mist wisps across the upper half.
	for i := 0; i < 3; i++ {
		wavyLine(r.Min.X+W/12, r.Max.X-W/12, r.Min.Y+H/6+i*H/9, 7, 5)
	}

	// Two pillars on the left.
	stroke(image.Pt(r.Min.X+W/6, floorY), image.Pt(r.Min.X+W/6, r.Min.Y+H/2))
	stroke(image.Pt(r.Min.X+W/3, floorY), image.Pt(r.Min.X+W/3, r.Min.Y+H/2))

	// A stone stair descending on the right.
	steps, sw, sh := 5, W/16, H/12
	sx := r.Min.X + W*7/12
	for i := 0; i < steps; i++ {
		x, y := sx+i*sw, floorY-i*sh
		stroke(image.Pt(x, y), image.Pt(x+sw, y))
		stroke(image.Pt(x+sw, y), image.Pt(x+sw, y-sh))
	}
}
