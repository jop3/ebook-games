package game

// solver.go: a pure-deduction constraint solver used to certify that a
// generated puzzle is uniquely solvable by logic alone (no guessing). It
// applies forced-bulb / forced-blank rules to a fixpoint:
//
//   - A numbered wall with Number == 0 forbids a bulb in every adjacent cell.
//   - A numbered wall with Number == (count of non-wall neighbours) forces a
//     bulb in every one of those neighbours (in particular Number == 4 always
//     forces all four).
//   - A numbered wall whose remaining undecided neighbours exactly equals
//     (Number - bulbs already placed) forces bulbs in all of them; if bulbs
//     already placed == Number, forces the rest to non-bulb.
//   - A white cell that is the only remaining cell able to light some other
//     white cell (or itself) that isn't yet lit must contain a bulb (basic
//     "single reachable light source" deduction), applied by exhaustive
//     recheck each pass.
//
// This mirrors nonogram's LineSolvable pattern: iterate local deductions to a
// fixpoint, then report Unique / Stuck / Contradiction. It is intentionally
// conservative — puzzles it can't fully resolve are rejected by the
// generator, guaranteeing every shipped puzzle is fair (no guessing needed).

// cellTri is a cell's solve state for the white cells.
type cellTri int8

const (
	sUnknown cellTri = iota
	sBulb
	sBlank // definitely no bulb here (but may or may not end up lit)
)

// SolveResult reports the outcome of the deduction solver.
type SolveResult int

const (
	SolveUnique SolveResult = iota
	SolveStuck
	SolveContradiction
)

// Solve runs the deduction solver on the board and reports whether it fully
// and uniquely determines every white cell's bulb/no-bulb state.
func Solve(b *Board) SolveResult {
	_, r := solve(b)
	return r
}

// SolveBulbs runs the deduction solver and, when the puzzle is uniquely
// solvable, returns the bulb placement it deduced (true = bulb). The second
// return is false for a stuck or contradictory board (bulbs is then nil). Used
// to reveal/verify the intended solution.
func SolveBulbs(b *Board) ([][]bool, bool) {
	bulbs, r := solve(b)
	return bulbs, r == SolveUnique
}

// solve is the shared deduction engine. On SolveUnique it also returns the
// deduced bulb grid; otherwise the grid is nil.
func solve(b *Board) ([][]bool, SolveResult) {
	state := make([][]cellTri, b.H)
	for y := range state {
		state[y] = make([]cellTri, b.W)
	}

	neighbours := func(x, y int) [][2]int {
		var out [][2]int
		for _, d := range dirs {
			cx, cy := x+d[0], y+d[1]
			if b.inBounds(cx, cy) && b.at(cx, cy).Kind == White {
				out = append(out, [2]int{cx, cy})
			}
		}
		return out
	}

	setBulb := func(x, y int) bool {
		switch state[y][x] {
		case sBlank:
			return false // contradiction: forced both ways
		case sUnknown:
			state[y][x] = sBulb
		}
		return true
	}
	setBlank := func(x, y int) bool {
		switch state[y][x] {
		case sBulb:
			return false
		case sUnknown:
			state[y][x] = sBlank
		}
		return true
	}

	bulbsGrid := func() [][]bool {
		g := make([][]bool, b.H)
		for y := range g {
			g[y] = make([]bool, b.W)
			for x := range g[y] {
				g[y][x] = state[y][x] == sBulb
			}
		}
		return g
	}

	for {
		changed := false

		// Wall-number deductions.
		for y := 0; y < b.H; y++ {
			for x := 0; x < b.W; x++ {
				c := b.at(x, y)
				if c.Kind != Wall || c.Number < 0 {
					continue
				}
				ns := neighbours(x, y)
				bulbCount, unknownCells := 0, [][2]int{}
				for _, n := range ns {
					switch state[n[1]][n[0]] {
					case sBulb:
						bulbCount++
					case sUnknown:
						unknownCells = append(unknownCells, n)
					}
				}
				remaining := c.Number - bulbCount
				if remaining < 0 || remaining > len(unknownCells) {
					return nil, SolveContradiction
				}
				if remaining == 0 {
					for _, n := range unknownCells {
						if !setBlank(n[0], n[1]) {
							return nil, SolveContradiction
						}
						changed = true
					}
				} else if remaining == len(unknownCells) && len(unknownCells) > 0 {
					for _, n := range unknownCells {
						if !setBulb(n[0], n[1]) {
							return nil, SolveContradiction
						}
						changed = true
					}
				}
			}
		}

		// Bulb-sees-bulb deduction: any bulb forbids bulbs along its rays.
		for y := 0; y < b.H; y++ {
			for x := 0; x < b.W; x++ {
				if state[y][x] != sBulb {
					continue
				}
				for _, d := range dirs {
					cx, cy := x+d[0], y+d[1]
					for b.inBounds(cx, cy) && b.at(cx, cy).Kind != Wall {
						if state[cy][cx] == sUnknown {
							state[cy][cx] = sBlank
							changed = true
						} else if state[cy][cx] == sBulb {
							return nil, SolveContradiction
						}
						cx += d[0]
						cy += d[1]
					}
				}
			}
		}

		// Forced-bulb: a white cell that can only ever be lit by itself (every
		// cell that could light it is known blank / a wall) must be a bulb.
		bulbs := bulbsGrid()
		lit := Lit(b, bulbs)
		for y := 0; y < b.H; y++ {
			for x := 0; x < b.W; x++ {
				if b.at(x, y).Kind != White || lit[y][x] || state[y][x] != sUnknown {
					continue
				}
				// Candidates that could still light this cell: itself, or any
				// unknown/bulb white cell visible along its 4 rays.
				canBeLit := false
				if state[y][x] != sBlank {
					canBeLit = true // itself could become a bulb
				}
				for _, d := range dirs {
					cx, cy := x+d[0], y+d[1]
					for b.inBounds(cx, cy) && b.at(cx, cy).Kind != Wall {
						if state[cy][cx] != sBlank {
							canBeLit = true
						}
						cx += d[0]
						cy += d[1]
					}
				}
				if !canBeLit {
					return nil, SolveContradiction
				}
				// If itself is the ONLY candidate, force a bulb here.
				onlySelf := true
				for _, d := range dirs {
					cx, cy := x+d[0], y+d[1]
					for b.inBounds(cx, cy) && b.at(cx, cy).Kind != Wall {
						if state[cy][cx] != sBlank {
							onlySelf = false
						}
						cx += d[0]
						cy += d[1]
					}
				}
				if onlySelf {
					if !setBulb(x, y) {
						return nil, SolveContradiction
					}
					changed = true
				}
			}
		}

		if !changed {
			break
		}
	}

	// Fully resolved?
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			if b.at(x, y).Kind == White && state[y][x] == sUnknown {
				return nil, SolveStuck
			}
		}
	}

	// Final sanity check: the deduced solution must actually be valid.
	bulbs := bulbsGrid()
	if !Solved(b, bulbs) {
		return nil, SolveContradiction
	}
	return bulbs, SolveUnique
}
