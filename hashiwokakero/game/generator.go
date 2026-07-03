package game

import "math/rand"

// Generate builds islands with a random growth process (extend from an
// existing island in a random open direction), derives Need from a fully
// connected bridge layout, then verifies the puzzle is uniquely solvable.
// Mirrors PocketPuzzles' bridges.c approach: grow the island graph directly
// rather than solution-first-then-strip. Retries up to a cap; falls back to
// the last attempt (even if not unique) so the app always starts.
func Generate(p Preset, rng *rand.Rand) *Puzzle {
	const outerAttempts = 60
	var last *Puzzle
	for attempt := 0; attempt < outerAttempts; attempt++ {
		puz := growLayout(p, rng)
		if puz == nil {
			continue
		}
		last = puz
		if Solve(puz) == SolveUnique {
			return puz
		}
	}
	if last != nil {
		return last
	}
	// Absolute fallback: minimal 2-island puzzle so the app never fails to start.
	return trivialPuzzle()
}

func trivialPuzzle() *Puzzle {
	islands := []Island{{X: 0, Y: 0, Need: 1}, {X: 2, Y: 0, Need: 1}}
	idx := map[[2]int]int{{0, 0}: 0, {2, 0}: 1}
	return &Puzzle{W: 3, H: 1, Islands: islands, indexAt: idx}
}

// growLayout places islands via randomized graph expansion: start with one
// island, repeatedly pick a random existing island and extend a bridge in a
// random open direction to a new (or existing) island a random distance away,
// building up a spanning-tree-plus-extra-edges connected bridge network. Then
// derives each island's Need from its final bridge degree.
func growLayout(p Preset, rng *rand.Rand) *Puzzle {
	occupied := map[[2]int]int{} // grid pos -> island index
	var islands []Island

	addIsland := func(x, y int) int {
		i := len(islands)
		islands = append(islands, Island{X: x, Y: y})
		occupied[[2]int{x, y}] = i
		return i
	}

	addIsland(p.W/2, p.H/2)

	bridgeCount := map[[2]int]int{}
	dirs := [4][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}

	tryExtend := func() bool {
		if len(islands) == 0 {
			return false
		}
		srcIdx := rng.Intn(len(islands))
		src := islands[srcIdx]
		order := rng.Perm(4)
		for _, di := range order {
			dv := dirs[di]
			// find how far we can go before hitting grid edge or existing island
			maxDist := 0
			x, y := src.X, src.Y
			var hit *int
			for {
				x += dv[0]
				y += dv[1]
				if x < 0 || x >= p.W || y < 0 || y >= p.H {
					break
				}
				if j, ok := occupied[[2]int{x, y}]; ok {
					h := j
					hit = &h
					break
				}
				maxDist++
			}
			if hit != nil {
				j := *hit
				k := pairKey(srcIdx, j)
				if bridgeCount[k] >= 2 {
					continue
				}
				// crossing check
				tmpPuz := &Puzzle{W: p.W, H: p.H, Islands: islands, indexAt: occupied}
				if bridgeCount[k] == 0 && !canPlaceEdge(tmpPuz, bridgeCount, srcIdx, j) {
					continue
				}
				bridgeCount[k]++
				return true
			}
			if maxDist >= 2 {
				dist := 2 + rng.Intn(maxDist-1)
				if dist > maxDist {
					dist = maxDist
				}
				nx, ny := src.X+dv[0]*dist, src.Y+dv[1]*dist
				if _, taken := occupied[[2]int{nx, ny}]; taken {
					continue
				}
				newIdx := len(islands)
				islands = append(islands, Island{X: nx, Y: ny})
				occupied[[2]int{nx, ny}] = newIdx
				if !canPlaceEdge(&Puzzle{W: p.W, H: p.H, Islands: islands, indexAt: occupied}, bridgeCount, srcIdx, newIdx) {
					// undo
					delete(occupied, [2]int{nx, ny})
					islands = islands[:len(islands)-1]
					continue
				}
				k := pairKey(srcIdx, newIdx)
				bridgeCount[k] = 1
				return true
			}
		}
		return false
	}

	target := p.NumIslands
	failStreak := 0
	for len(islands) < target && failStreak < 50 {
		if !tryExtend() {
			failStreak++
			continue
		}
		failStreak = 0
	}
	if len(islands) < 2 {
		return nil
	}

	// Occasionally add a few extra bridges (parallel or between existing
	// islands) to make the puzzle richer, respecting crossing + max 2.
	puz := &Puzzle{W: p.W, H: p.H, Islands: islands, indexAt: occupied}
	extra := rng.Intn(len(islands)/2 + 1)
	for e := 0; e < extra; e++ {
		i := rng.Intn(len(islands))
		nbrs := puz.NeighbourList(i)
		if len(nbrs) == 0 {
			continue
		}
		j := nbrs[rng.Intn(len(nbrs))]
		k := pairKey(i, j)
		if bridgeCount[k] >= 2 {
			continue
		}
		if bridgeCount[k] == 0 && !canPlaceEdge(puz, bridgeCount, i, j) {
			continue
		}
		bridgeCount[k]++
	}

	// Derive Need from final degrees; verify connectivity.
	degree := make([]int, len(islands))
	for k, c := range bridgeCount {
		degree[k[0]] += c
		degree[k[1]] += c
	}
	for i := range islands {
		if degree[i] == 0 {
			return nil // isolated island, invalid layout
		}
		islands[i].Need = degree[i]
	}
	puz.Islands = islands

	if !layoutConnected(len(islands), bridgeCount) {
		return nil
	}

	return puz
}

func canPlaceEdge(p *Puzzle, existing map[[2]int]int, i, j int) bool {
	for k, cnt := range existing {
		if cnt == 0 {
			continue
		}
		a, b := k[0], k[1]
		if (a == i && b == j) || (a == j && b == i) {
			continue
		}
		if crosses(p, i, j, a, b) {
			return false
		}
	}
	return true
}

func layoutConnected(n int, bridges map[[2]int]int) bool {
	if n == 0 {
		return true
	}
	adj := make([][]int, n)
	for k, c := range bridges {
		if c == 0 {
			continue
		}
		adj[k[0]] = append(adj[k[0]], k[1])
		adj[k[1]] = append(adj[k[1]], k[0])
	}
	seen := make([]bool, n)
	stack := []int{0}
	seen[0] = true
	count := 1
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, nb := range adj[cur] {
			if !seen[nb] {
				seen[nb] = true
				count++
				stack = append(stack, nb)
			}
		}
	}
	return count == n
}
