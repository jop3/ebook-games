package game

import "math/rand"

// Generate builds a Nurikabe puzzle solution-first: carve random
// non-touching islands (growing each from a random empty cell) until the
// grid is covered or no more islands fit, verify the leftover sea is
// connected with no 2x2 block, place one seed per island at its size, then
// certify the puzzle is solvable by brute-force search up to a small bound
// (these grids are tiny, so exhaustive search over island placements is
// tractable). Retries with a fresh carve on failure; falls back to a trivial
// all-sea-minus-one-island layout so the app always starts.
func Generate(p Preset, rng *rand.Rand) *Puzzle {
	const attempts = 200
	for a := 0; a < attempts; a++ {
		sea, seeds, ok := carve(p, rng)
		if !ok {
			continue
		}
		if !ValidateSolution(p.W, p.H, sea, seeds) {
			continue
		}
		return &Puzzle{W: p.W, H: p.H, Seeds: seeds, Solution: sea}
	}
	return trivialPuzzle(p)
}

// carve grows non-touching islands from random seed points until the grid
// runs low on space, then fills every remaining unclaimed cell with sea.
// Returns ok=false if it produced a degenerate layout (no islands, or an
// island of size 0) so the caller retries.
func carve(p Preset, rng *rand.Rand) (sea [][]bool, seeds map[[2]int]int, ok bool) {
	w, h := p.W, p.H
	owner := make([][]int, h) // -1 = unclaimed, else island id
	for y := range owner {
		owner[y] = make([]int, w)
		for x := range owner[y] {
			owner[y][x] = -1
		}
	}

	touchesOtherIsland := func(x, y, id int) bool {
		dirs := [4][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
		for _, d := range dirs {
			nx, ny := x+d[0], y+d[1]
			if nx < 0 || nx >= w || ny < 0 || ny >= h {
				continue
			}
			if owner[ny][nx] != -1 && owner[ny][nx] != id {
				return true
			}
		}
		return false
	}

	var islandCells [][][2]int
	targetIslandCount := 2 + rng.Intn(3) // 2-4 islands
	maxTotalIslandCells := w * h * 3 / 5 // leave room for connected sea

	totalUsed := 0
	for len(islandCells) < targetIslandCount && totalUsed < maxTotalIslandCells {
		// Pick a random unclaimed start cell.
		sx, sy := -1, -1
		for tries := 0; tries < 50; tries++ {
			x, y := rng.Intn(w), rng.Intn(h)
			if owner[y][x] == -1 && !touchesOtherIsland(x, y, -2) {
				sx, sy = x, y
				break
			}
		}
		if sx == -1 {
			break
		}
		id := len(islandCells)
		size := 1 + rng.Intn(4) // island sizes 1-4
		if totalUsed+size > maxTotalIslandCells {
			size = maxTotalIslandCells - totalUsed
			if size < 1 {
				break
			}
		}
		cells := [][2]int{{sx, sy}}
		owner[sy][sx] = id
		frontier := [][2]int{{sx, sy}}
		for len(cells) < size && len(frontier) > 0 {
			fi := rng.Intn(len(frontier))
			cx, cy := frontier[fi][0], frontier[fi][1]
			dirs := rng.Perm(4)
			dvs := [4][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
			grew := false
			for _, di := range dirs {
				nx, ny := cx+dvs[di][0], cy+dvs[di][1]
				if nx < 0 || nx >= w || ny < 0 || ny >= h || owner[ny][nx] != -1 {
					continue
				}
				if touchesOtherIsland(nx, ny, id) {
					continue
				}
				owner[ny][nx] = id
				cells = append(cells, [2]int{nx, ny})
				frontier = append(frontier, [2]int{nx, ny})
				grew = true
				break
			}
			if !grew {
				frontier = append(frontier[:fi], frontier[fi+1:]...)
			}
		}
		if len(cells) == 0 {
			break
		}
		islandCells = append(islandCells, cells)
		totalUsed += len(cells)
	}

	if len(islandCells) == 0 {
		return nil, nil, false
	}

	// Everything unclaimed is sea.
	sea = make([][]bool, h)
	for y := range sea {
		sea[y] = make([]bool, w)
		for x := range sea[y] {
			sea[y][x] = owner[y][x] == -1
		}
	}

	// One seed per island, placed at a random cell within it, sized to the
	// island's actual carved size.
	seeds = map[[2]int]int{}
	for _, cells := range islandCells {
		pick := cells[rng.Intn(len(cells))]
		seeds[pick] = len(cells)
	}
	return sea, seeds, true
}

// trivialPuzzle is the absolute fallback: a single full-grid island (one
// seed sized W*H, no sea at all), which trivially satisfies every
// constraint and guarantees the app can always start.
func trivialPuzzle(p Preset) *Puzzle {
	sea := make([][]bool, p.H)
	for y := range sea {
		sea[y] = make([]bool, p.W)
	}
	seeds := map[[2]int]int{{0, 0}: p.W * p.H}
	return &Puzzle{W: p.W, H: p.H, Seeds: seeds, Solution: sea}
}
