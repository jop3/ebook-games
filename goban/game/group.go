package game

import "image"

// Group returns every point connected to p by a chain of orthogonal
// neighbors sharing p's color (a flood fill). This works uniformly for a
// group of stones (Black or White) and for a connected empty region — the
// latter is what scoring uses to find territory.
func Group(b Board, p image.Point) []image.Point {
	color := b.At(p)
	seen := map[image.Point]bool{p: true}
	stack := []image.Point{p}
	group := []image.Point{p}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, n := range b.Neighbors(cur) {
			if seen[n] || b.At(n) != color {
				continue
			}
			seen[n] = true
			stack = append(stack, n)
			group = append(group, n)
		}
	}
	return group
}

// Liberties returns the distinct empty points orthogonally adjacent to any
// point in group.
func Liberties(b Board, group []image.Point) []image.Point {
	seen := map[image.Point]bool{}
	var libs []image.Point
	for _, p := range group {
		for _, n := range b.Neighbors(p) {
			if b.At(n) == Empty && !seen[n] {
				seen[n] = true
				libs = append(libs, n)
			}
		}
	}
	return libs
}

// BorderColors returns the set of non-Empty colors bordering every point in
// region (region is normally a connected empty area from Group). Used by
// scoring to tell single-owner territory from a neutral (dame) point that
// touches both colors.
func BorderColors(b Board, region []image.Point) map[Color]bool {
	borders := map[Color]bool{}
	for _, p := range region {
		for _, n := range b.Neighbors(p) {
			if c := b.At(n); c != Empty {
				borders[c] = true
			}
		}
	}
	return borders
}
