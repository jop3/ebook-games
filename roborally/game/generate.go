package game

import (
	"image"
	"math/rand"
)

// GenerateCourse builds a valid course for the given difficulty and seed. It
// generates, verifies (reachable + solvable by a competent reference planner),
// and regenerates on failure, falling back to a guaranteed-solvable plain course
// if it cannot find one in a bounded number of attempts.
func GenerateCourse(diff CourseDiff, seed int64) *Board {
	bd := budgetFor(diff)
	rng := rand.New(rand.NewSource(seed))
	for attempt := 0; attempt < 60; attempt++ {
		b := buildCandidate(bd, rng)
		if b != nil && verifyCourse(b, bd) {
			return b
		}
	}
	return plainCourse(bd)
}

func isSpecial(t *Tile) bool {
	return t.Checkpoint != 0 || t.Kind != FloorPlain || t.Antenna ||
		t.StartDock != 0 || t.Belt != DirNone || t.Gear != GearNone || t.Laser != DirNone
}

func randomPlain(b *Board, rng *rand.Rand) (image.Point, bool) {
	for tries := 0; tries < 200; tries++ {
		p := image.Pt(rng.Intn(b.W), rng.Intn(b.H))
		if !isSpecial(b.At(p)) {
			return p, true
		}
	}
	return image.Point{}, false
}

func buildCandidate(bd budget, rng *rand.Rand) *Board {
	sz := bd.size
	b := newBoard(sz, sz)

	// Antenna near the centre (kept a plain tile).
	b.Antenna = image.Pt(sz/2, sz/2)
	b.At(b.Antenna).Antenna = true

	// Belt lanes: a few directed random walks so belts read as designed lanes.
	beltTarget := int(float64(sz*sz) * bd.beltFrac)
	laid := 0
	for lane := 0; lane < 3 && laid < beltTarget; lane++ {
		p, ok := randomPlain(b, rng)
		if !ok {
			break
		}
		dir := Dir(rng.Intn(4))
		express := bd.express && lane%2 == 0
		length := sz/2 + rng.Intn(sz/2+1)
		for step := 0; step < length && laid < beltTarget; step++ {
			if !b.In(p) || isSpecial(b.At(p)) {
				break
			}
			t := b.At(p)
			t.Belt = dir
			t.BeltExpress = express
			laid++
			// Occasionally curve the lane.
			if rng.Intn(4) == 0 {
				if rng.Intn(2) == 0 {
					dir = dir.Right()
				} else {
					dir = dir.Left()
				}
			}
			p = p.Add(dir.Step())
		}
	}

	// Gears on a few plain tiles.
	if bd.gears {
		for i := 0; i < sz/3; i++ {
			if p, ok := randomPlain(b, rng); ok {
				if rng.Intn(2) == 0 {
					b.At(p).Gear = GearCW
				} else {
					b.At(p).Gear = GearCCW
				}
			}
		}
	}

	// Pits on plain tiles (kept off belts so no inescapable belt-dump).
	pitTarget := int(float64(sz*sz) * bd.pitFrac)
	for i := 0; i < pitTarget; i++ {
		if p, ok := randomPlain(b, rng); ok {
			b.At(p).Kind = FloorPit
		}
	}

	// Wall lasers firing inward from a few tiles.
	for i := 0; i < bd.lasers; i++ {
		p, ok := randomPlain(b, rng)
		if !ok {
			break
		}
		dir := Dir(rng.Intn(4))
		t := b.At(p)
		t.Laser = dir
		t.LaserCount = 1
		if bd.doubleBarrel && rng.Intn(2) == 0 {
			t.LaserCount = 2
		}
	}

	// Checkpoints, spaced apart.
	var checks []image.Point
	for ord := 1; ord <= bd.checkpoints; ord++ {
		placed := false
		for tries := 0; tries < 300; tries++ {
			p, ok := randomPlain(b, rng)
			if !ok {
				break
			}
			okSpace := true
			for _, c := range checks {
				if manhattan(p, c) < bd.minCheckSpace {
					okSpace = false
					break
				}
			}
			if !okSpace {
				continue
			}
			b.At(p).Checkpoint = uint8(ord)
			checks = append(checks, p)
			placed = true
			break
		}
		if !placed {
			return nil
		}
	}
	b.NCheck = uint8(bd.checkpoints)

	// A few chokepoint walls near checkpoints.
	for _, c := range checks {
		if rng.Intn(2) == 0 {
			d := Dir(rng.Intn(4))
			if b.In(c.Add(d.Step())) {
				b.At(c).Walls |= d.wallBit()
			}
		}
	}

	// Start docks: a row of four adjacent squares along the bottom edge, facing
	// up into the board. The starting line is cleared of hazards so every robot
	// gets a fair launch; which robot lands in which dock is randomised at
	// NewGame time.
	if !placeBottomDocks(b, rng) {
		return nil
	}
	return b
}

// placeBottomDocks lays four start docks on the bottom row with a free gap
// between each (so neighbours never touch) and clears the dock row plus the row
// directly ahead (so no robot has a pit or wall blocking its first move up).
// Returns false if no suitable window free of a checkpoint or the antenna exists.
func placeBottomDocks(b *Board, rng *rand.Rand) bool {
	const n = 4
	span := 2*n - 1 // dock, gap, dock, gap, dock, gap, dock  -> 7 columns
	step := 2
	if b.W < span { // tiny board: fall back to adjacent docks
		span, step = n, 1
	}
	y := b.H - 1
	clearAt := func(p image.Point) {
		if !b.In(p) {
			return
		}
		t := b.At(p)
		t.Kind = FloorPlain
		t.Belt = DirNone
		t.BeltExpress = false
		t.Gear = GearNone
		t.Laser = DirNone
		t.Walls = 0
	}
	// Windows whose bottom two rows hold no checkpoint/antenna (which we mustn't
	// clear away).
	var starts []int
	for x0 := 0; x0+span <= b.W; x0++ {
		ok := true
		for dx := 0; dx < span && ok; dx++ {
			for _, yy := range []int{y, y - 1} {
				if yy < 0 {
					continue
				}
				t := b.At(image.Pt(x0+dx, yy))
				if t.Checkpoint != 0 || t.Antenna {
					ok = false
					break
				}
			}
		}
		if ok {
			starts = append(starts, x0)
		}
	}
	if len(starts) == 0 {
		return false
	}
	// Prefer a centered starting block; shuffle first so ties break randomly.
	rng.Shuffle(len(starts), func(i, j int) { starts[i], starts[j] = starts[j], starts[i] })
	ideal := (b.W - span) / 2
	x0 := starts[0]
	bestd := absi(x0 - ideal)
	for _, s := range starts[1:] {
		if d := absi(s - ideal); d < bestd {
			bestd, x0 = d, s
		}
	}
	// Clear the whole staging area: dock row + one row ahead, across the span.
	for dx := 0; dx < span; dx++ {
		clearAt(image.Pt(x0+dx, y))
		clearAt(image.Pt(x0+dx, y-1))
	}
	for i := 0; i < n; i++ {
		p := image.Pt(x0+i*step, y)
		b.At(p).StartDock = uint8(i + 1)
		b.Docks = append(b.Docks, dock{Pos: p, Facing: N})
	}
	return true
}

// dirToward picks the cardinal direction from a that most reduces distance to b.
func dirToward(a, b image.Point) Dir {
	dx := b.X - a.X
	dy := b.Y - a.Y
	adx, ady := dx, dy
	if adx < 0 {
		adx = -adx
	}
	if ady < 0 {
		ady = -ady
	}
	if adx >= ady {
		if dx >= 0 {
			return E
		}
		return W
	}
	if dy >= 0 {
		return S
	}
	return N
}

// verifyCourse checks reachability between consecutive checkpoints and that a
// competent reference planner can finish within the round budget.
func verifyCourse(b *Board, bd budget) bool {
	// Reachability: dock1 → cp1 → cp2 → ... over floor tiles.
	prev := b.Docks[0].Pos
	for ord := uint8(1); ord <= b.NCheck; ord++ {
		cp, ok := b.CheckpointPos(ord)
		if !ok {
			return false
		}
		if bfsDist(b, prev, cp) >= (1 << 20) {
			return false
		}
		prev = cp
	}
	// Solvable in practice by the Expert reference planner.
	_, ok := solveReference(b, bd)
	return ok
}

// solveReference runs a lone Expert robot (no fumble) through the course and
// reports whether it finishes within the round budget.
func solveReference(b *Board, bd budget) (int, bool) {
	gs := NewGame(b, 0, AIExpert, 1)
	prof := aiProfiles(AIExpert, 1, rand.New(rand.NewSource(1)))[0]
	for round := 0; round < bd.roundBudget; round++ {
		if gs.Robots[0].NextCheck > b.NCheck {
			return round, true
		}
		pv := publicView{
			board:   b,
			self:    simRobot{gs.Robots[0].Pos, gs.Robots[0].Facing, gs.Robots[0].Damage, gs.Robots[0].NextCheck, true},
			hand:    gs.Hands[0],
			profile: prof,
		}
		gs.Registers[0] = planProgram(pv)
		gs.Phase = PhaseResolve
		gs.CurReg = 0
		for gs.Phase == PhaseResolve {
			gs.StepRegister()
		}
		if gs.Phase == PhaseDone {
			return round + 1, true
		}
	}
	return bd.roundBudget, gs.Robots[0].NextCheck > b.NCheck
}

// plainCourse is the guaranteed-solvable fallback: a bare floor with spaced
// checkpoints and docks, no hazards.
func plainCourse(bd budget) *Board {
	sz := bd.size
	b := newBoard(sz, sz)
	b.Antenna = image.Pt(sz/2, sz/2)
	b.At(b.Antenna).Antenna = true
	b.NCheck = uint8(bd.checkpoints)
	for ord := 1; ord <= bd.checkpoints; ord++ {
		x := (sz - 1) * (ord - 1) / max1(bd.checkpoints-1)
		y := sz / 2
		if ord%2 == 0 {
			y = sz/2 + 1
		}
		p := image.Pt(clampi(x, 0, sz-1), clampi(y, 0, sz-1))
		b.At(p).Checkpoint = uint8(ord)
	}
	y := sz - 1
	off := (sz - 7) / 2 // centre the 4-dock, gap-spaced block
	if off < 0 {
		off = 0
	}
	for i := 0; i < 4; i++ {
		x := off + i*2
		if x >= sz {
			break
		}
		p := image.Pt(x, y)
		if isSpecial(b.At(p)) {
			continue
		}
		b.At(p).StartDock = uint8(len(b.Docks) + 1)
		b.Docks = append(b.Docks, dock{Pos: p, Facing: N})
	}
	return b
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

func absi(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func clampi(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
