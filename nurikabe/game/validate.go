package game

// validate.go: the four independent Nurikabe constraints, each checkable on
// a full boolean grid (true = sea). Used both to certify a generated
// solution and (in principle) to flag mistakes live in the UI.

// ValidateSolution reports whether a full sea/island assignment satisfies
// all four Nurikabe constraints given the puzzle's seeds.
func ValidateSolution(w, h int, sea [][]bool, seeds map[[2]int]int) bool {
	return islandsOK(w, h, sea, seeds) && !hasSea2x2(w, h, sea) && seaConnected(w, h, sea)
}

// islandsOK checks: every island (connected white region) contains exactly
// one seed, and its size matches that seed's number; and no two islands
// touch orthogonally (equivalent to: each white region has exactly one seed
// with the right size, since a region touching another would merge under
// flood fill and immediately fail the "exactly one seed" check... EXCEPT
// orthogonal adjacency between two single-seed regions that are NOT merged
// by flood fill can't happen — if they're orthogonally adjacent they ARE the
// same flood-fill region. So "each white region has exactly one correctly
// sized seed" is sufficient and also implies no two distinct islands touch.
func islandsOK(w, h int, sea [][]bool, seeds map[[2]int]int) bool {
	visited := make([][]bool, h)
	for y := range visited {
		visited[y] = make([]bool, w)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if sea[y][x] || visited[y][x] {
				continue
			}
			// Flood fill this white region.
			region := floodFill(w, h, x, y, func(xx, yy int) bool { return !sea[yy][xx] }, visited)
			seedCount := 0
			var seedSize int
			for _, p := range region {
				if sz, ok := seeds[p]; ok {
					seedCount++
					seedSize = sz
				}
			}
			if seedCount != 1 {
				return false
			}
			if len(region) != seedSize {
				return false
			}
		}
	}
	return true
}

// hasSea2x2 reports whether any 2x2 block is entirely sea.
func hasSea2x2(w, h int, sea [][]bool) bool {
	for y := 0; y+1 < h; y++ {
		for x := 0; x+1 < w; x++ {
			if sea[y][x] && sea[y][x+1] && sea[y+1][x] && sea[y+1][x+1] {
				return true
			}
		}
	}
	return false
}

// seaConnected reports whether all sea cells form one connected region (a
// puzzle with zero sea cells is trivially connected).
func seaConnected(w, h int, sea [][]bool) bool {
	var start [2]int
	found := false
	total := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if sea[y][x] {
				total++
				if !found {
					start = [2]int{x, y}
					found = true
				}
			}
		}
	}
	if total == 0 {
		return true
	}
	visited := make([][]bool, h)
	for y := range visited {
		visited[y] = make([]bool, w)
	}
	region := floodFill(w, h, start[0], start[1], func(xx, yy int) bool { return sea[yy][xx] }, visited)
	return len(region) == total
}

// floodFill returns all cells connected to (x0,y0) (inclusive) for which
// pred(x,y) holds, marking them in visited (shared across calls so islandsOK
// can skip already-visited regions).
func floodFill(w, h, x0, y0 int, pred func(x, y int) bool, visited [][]bool) [][2]int {
	if visited[y0][x0] {
		return nil
	}
	var region [][2]int
	stack := [][2]int{{x0, y0}}
	visited[y0][x0] = true
	dirs := [4][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		region = append(region, p)
		for _, d := range dirs {
			nx, ny := p[0]+d[0], p[1]+d[1]
			if nx < 0 || nx >= w || ny < 0 || ny >= h || visited[ny][nx] {
				continue
			}
			if !pred(nx, ny) {
				continue
			}
			visited[ny][nx] = true
			stack = append(stack, [2]int{nx, ny})
		}
	}
	return region
}
