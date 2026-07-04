package main

import (
	"image"
	"image/color"

	ink "github.com/dennwc/inkview"
)

// A small procedural drawing toolkit for the vignettes: hatching, stipple,
// masonry texture, and arcs. Everything is deterministic (no RNG — jitter comes
// from a hash of the pixel coordinate) so a scene renders identically every
// frame, which matters on e-ink where a partial refresh must not make texture
// crawl. These primitives are what let the line art read as shaded illustration
// rather than bare outline while staying cheap and fully generated.

// rnd returns a stable pseudo-random value in [0,1) for a lattice point — used
// to jitter stipple so texture looks organic but never changes between frames.
func rnd(x, y, salt int) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263 + uint32(salt)*2246822519
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float64(h%10000) / 10000.0
}

// dot paints a small 2×2 mark (a single e-ink pixel reads too faint).
func dot(p image.Point, cl color.Color) {
	ink.FillArea(image.Rect(p.X, p.Y, p.X+2, p.Y+2), cl)
}

// hatchRect fills a rectangle with 45° parallel lines (slope −1), analytically
// clipped to the rect. up=true flips to slope +1 for cross-hatching.
func hatchRect(r image.Rectangle, step int, cl color.Color, up bool) {
	if up {
		// lines of constant (x - y)
		for c := r.Min.X - r.Max.Y; c <= r.Max.X-r.Min.Y; c += step {
			x0 := r.Min.X
			if c+r.Min.Y > x0 {
				x0 = c + r.Min.Y
			}
			x1 := r.Max.X
			if c+r.Max.Y < x1 {
				x1 = c + r.Max.Y
			}
			if x0 <= x1 {
				ink.DrawLine(image.Pt(x0, x0-c), image.Pt(x1, x1-c), cl)
			}
		}
		return
	}
	// lines of constant (x + y)
	for c := r.Min.X + r.Min.Y; c <= r.Max.X+r.Max.Y; c += step {
		x0 := r.Min.X
		if c-r.Max.Y > x0 {
			x0 = c - r.Max.Y
		}
		x1 := r.Max.X
		if c-r.Min.Y < x1 {
			x1 = c - r.Min.Y
		}
		if x0 <= x1 {
			ink.DrawLine(image.Pt(x0, c-x0), image.Pt(x1, c-x1), cl)
		}
	}
}

// crossHatch lays both diagonals for a denser shadow.
func crossHatch(r image.Rectangle, step int, cl color.Color) {
	hatchRect(r, step, cl, false)
	hatchRect(r, step, cl, true)
}

// stipple scatters dots on a jittered lattice wherever contains() holds — for
// organic fills (foliage, rock, mist). gap sets density; salt varies the field.
func stipple(r image.Rectangle, gap int, salt int, contains func(p image.Point) bool, cl color.Color) {
	for y := r.Min.Y; y < r.Max.Y; y += gap {
		for x := r.Min.X; x < r.Max.X; x += gap {
			jx := x + int(rnd(x, y, salt)*float64(gap))
			jy := y + int(rnd(x, y, salt+7)*float64(gap))
			p := image.Pt(jx, jy)
			if contains(p) {
				dot(p, cl)
			}
		}
	}
}

// brickWall textures a rectangle with courses of staggered blocks.
func brickWall(r image.Rectangle, courseH, brickW int, cl color.Color) {
	row := 0
	for y := r.Min.Y; y < r.Max.Y; y += courseH {
		y2 := y + courseH
		if y2 > r.Max.Y {
			y2 = r.Max.Y
		}
		ink.DrawLine(image.Pt(r.Min.X, y), image.Pt(r.Max.X, y), cl)
		off := 0
		if row%2 == 1 {
			off = brickW / 2
		}
		for x := r.Min.X + off; x < r.Max.X; x += brickW {
			ink.DrawLine(image.Pt(x, y), image.Pt(x, y2), cl)
		}
		row++
	}
}

// arc strokes an elliptical arc from d0° to d1° (0°=east, CCW), 2px thick.
func arc(c image.Point, rw, rh, d0, d1 int, cl color.Color) {
	const rad = 3.14159265358979 / 180.0
	var prev image.Point
	first := true
	step := 4
	if d1 < d0 {
		step = -4
	}
	for d := d0; (step > 0 && d <= d1) || (step < 0 && d >= d1); d += step {
		a := float64(d) * rad
		p := image.Pt(c.X+int(float64(rw)*cos(a)), c.Y-int(float64(rh)*sin(a)))
		if !first {
			ink.DrawLine(prev, p, cl)
			ink.DrawLine(prev.Add(image.Pt(0, 1)), p.Add(image.Pt(0, 1)), cl)
		}
		prev = p
		first = false
	}
}

// discOutline strokes a circle (thin).
func discOutline(c image.Point, r int, cl color.Color) {
	arc(c, r, r, 0, 360, cl)
}

// foliage draws a bumpy cloud outline of radius rad at c and stipples inside it,
// giving a tree canopy some body instead of a bare triangle.
func foliage(c image.Point, rad int, salt int) {
	// bumpy outline: several overlapping arcs around the perimeter.
	const tau = 6.28318530718
	lobes := 7
	var prev image.Point
	for i := 0; i <= lobes; i++ {
		a := float64(i) / float64(lobes) * tau
		rr := rad + int(float64(rad)/6*sinApprox(a*3))
		p := image.Pt(c.X+int(float64(rr)*cos(a)), c.Y-int(float64(rr)*sin(a)))
		if i > 0 {
			stroke(prev, p)
		}
		prev = p
	}
	stipple(image.Rect(c.X-rad, c.Y-rad, c.X+rad, c.Y+rad), 9, salt,
		func(p image.Point) bool {
			dx, dy := p.X-c.X, p.Y-c.Y
			return dx*dx+dy*dy < (rad-4)*(rad-4)
		}, ink.DarkGray)
}
