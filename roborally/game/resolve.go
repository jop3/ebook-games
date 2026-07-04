package game

import (
	"image"
	"sort"
)

// robotAt returns the index of a living robot on tile p, or -1. Off-board or
// finished robots never count as occupants.
func (s *GameState) robotAt(p image.Point) int {
	if !s.Board.In(p) {
		return -1
	}
	for i := range s.Robots {
		r := &s.Robots[i]
		if r.Alive && r.NextCheck <= s.Board.NCheck && r.Pos == p {
			return i
		}
	}
	return -1
}

// active reports whether robot i takes part in resolution this register.
func (s *GameState) active(i int) bool {
	r := &s.Robots[i]
	return r.Alive && !r.deadThisRound && r.NextCheck <= s.Board.NCheck
}

func manhattan(a, b image.Point) int {
	dx := a.X - b.X
	if dx < 0 {
		dx = -dx
	}
	dy := a.Y - b.Y
	if dy < 0 {
		dy = -dy
	}
	return dx + dy
}

// priorityOrder returns robot indices for register reg in activation order:
// closest to the antenna first, ties broken by the register card's priority,
// then by ID.
func (s *GameState) priorityOrder(reg int) []int {
	var order []int
	for i := range s.Robots {
		if s.active(i) {
			order = append(order, i)
		}
	}
	sort.SliceStable(order, func(a, b int) bool {
		ia, ib := order[a], order[b]
		da := manhattan(s.Robots[ia].Pos, s.Board.Antenna)
		db := manhattan(s.Robots[ib].Pos, s.Board.Antenna)
		if da != db {
			return da < db
		}
		pa := cardPriority(s.Registers[ia][reg])
		pb := cardPriority(s.Registers[ib][reg])
		if pa != pb {
			return pa > pb
		}
		return ia < ib
	})
	return order
}

// StepRegister resolves the current register (movement in priority order, then
// board elements, lasers, and checkpoint touches), advances CurReg, and ends
// the round (or the game) after register 5. It is a no-op outside PhaseResolve.
func (s *GameState) StepRegister() {
	if s.Phase != PhaseResolve {
		return
	}
	reg := s.CurReg

	// 1. Reveal & move, in priority order.
	for _, i := range s.priorityOrder(reg) {
		if !s.active(i) {
			continue
		}
		s.executeCard(i, s.Registers[i][reg])
	}
	// 2. Board elements.
	s.stepBelts(true)  // express belts move their extra step
	s.stepBelts(false) // then all belts move one
	s.spinGears()
	s.fireWallLasers()
	s.fireRobotLasers()
	// 3. Checkpoint / repair touches.
	s.touchCheckpoints()

	s.CurReg++
	if s.Winner >= 0 {
		s.Phase = PhaseDone
		return
	}
	if s.CurReg >= 5 {
		s.endRound()
	}
}

// executeCard applies one register card: a rotation, or a push-aware move.
func (s *GameState) executeCard(i int, c Card) {
	r := &s.Robots[i]
	switch c {
	case RotR:
		r.Facing = r.Facing.Right()
	case RotL:
		r.Facing = r.Facing.Left()
	case UTurn:
		r.Facing = r.Facing.Opposite()
	case Move1:
		s.moveRobot(i, r.Facing, 1)
	case Move2:
		s.moveRobot(i, r.Facing, 2)
	case Move3:
		s.moveRobot(i, r.Facing, 3)
	case BackUp:
		s.moveRobot(i, r.Facing.Opposite(), 1)
	}
}

// moveRobot steps robot i n tiles in dir, one tile at a time, pushing others.
func (s *GameState) moveRobot(i int, dir Dir, n int) {
	for k := 0; k < n; k++ {
		if !s.active(i) {
			return
		}
		if !s.stepPush(i, dir) {
			return // blocked by a wall
		}
	}
}

// stepPush attempts to move robot i one tile in dir, recursively pushing any
// robot in the way. Returns false if a wall blocks the move.
func (s *GameState) stepPush(i int, dir Dir) bool {
	from := s.Robots[i].Pos
	if s.Board.wallBetween(from, dir) {
		return false
	}
	to := from.Add(dir.Step())
	if occ := s.robotAt(to); occ >= 0 {
		if !s.stepPush(occ, dir) {
			return false
		}
	}
	s.Robots[i].Pos = to
	s.checkLethal(i)
	return true
}

func (s *GameState) checkLethal(i int) {
	if s.Robots[i].Alive && s.Board.lethal(s.Robots[i].Pos) {
		s.killRobot(i)
	}
}

func (s *GameState) killRobot(i int) {
	s.Robots[i].Alive = false
	s.Robots[i].deadThisRound = true
	s.Robots[i].tookDamage = true
	s.logf("Robot " + itoa(i+1) + " förstördes")
}

// stepBelts moves robots on belts one tile, resolving conflicts simultaneously.
// expressOnly restricts to express belts (called first for their extra step).
func (s *GameState) stepBelts(expressOnly bool) {
	dest := map[int]image.Point{}
	for i := range s.Robots {
		if !s.active(i) || !s.Board.In(s.Robots[i].Pos) {
			continue
		}
		t := s.Board.At(s.Robots[i].Pos)
		if t.Belt == DirNone || (expressOnly && !t.BeltExpress) {
			continue
		}
		if s.Board.wallBetween(s.Robots[i].Pos, t.Belt) {
			continue
		}
		dest[i] = s.Robots[i].Pos.Add(t.Belt.Step())
	}
	if len(dest) == 0 {
		return
	}
	// Cancel movers whose destination collides with another mover's.
	for {
		counts := map[image.Point]int{}
		for _, to := range dest {
			counts[to]++
		}
		changed := false
		for i, to := range dest {
			if counts[to] > 1 {
				delete(dest, i)
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	// Cancel movers whose destination is occupied by a robot that stays put.
	for {
		changed := false
		for i, to := range dest {
			if occ := s.robotAt(to); occ >= 0 && occ != i {
				if _, moving := dest[occ]; !moving {
					delete(dest, i)
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	// Apply simultaneously.
	moved := make([]int, 0, len(dest))
	from := map[int]image.Point{}
	for i := range dest {
		from[i] = s.Robots[i].Pos
		moved = append(moved, i)
	}
	sort.Ints(moved)
	for _, i := range moved {
		oldDir := s.Board.At(from[i]).Belt
		s.Robots[i].Pos = dest[i]
		// Belt curve: rotate the robot if it lands on a belt that turns.
		if s.Board.In(dest[i]) {
			nd := s.Board.At(dest[i]).Belt
			if nd != DirNone && nd != oldDir {
				if nd == oldDir.Right() {
					s.Robots[i].Facing = s.Robots[i].Facing.Right()
				} else if nd == oldDir.Left() {
					s.Robots[i].Facing = s.Robots[i].Facing.Left()
				}
			}
		}
		s.checkLethal(i)
	}
}

func (s *GameState) spinGears() {
	for i := range s.Robots {
		if !s.active(i) || !s.Board.In(s.Robots[i].Pos) {
			continue
		}
		switch s.Board.At(s.Robots[i].Pos).Gear {
		case GearCW:
			s.Robots[i].Facing = s.Robots[i].Facing.Right()
		case GearCCW:
			s.Robots[i].Facing = s.Robots[i].Facing.Left()
		}
	}
}

// fireWallLasers fires every wall-mounted emitter; the first robot in each beam
// takes LaserCount damage.
func (s *GameState) fireWallLasers() {
	for y := 0; y < s.Board.H; y++ {
		for x := 0; x < s.Board.W; x++ {
			t := s.Board.At(image.Pt(x, y))
			if t.Laser == DirNone {
				continue
			}
			s.beam(image.Pt(x, y), t.Laser, int(t.LaserCount), true)
		}
	}
}

// fireRobotLasers fires each living robot's forward laser (1 damage).
func (s *GameState) fireRobotLasers() {
	for i := range s.Robots {
		if !s.active(i) {
			continue
		}
		s.beam(s.Robots[i].Pos, s.Robots[i].Facing, 1, false)
	}
}

// beam walks a laser from p in dir until it hits a wall, the board edge, or a
// robot. includeStart controls whether a robot on the origin tile can be hit
// (true for wall lasers whose tile is the beam's source; false for robot lasers
// so a robot never shoots itself).
func (s *GameState) beam(p image.Point, dir Dir, dmg int, includeStart bool) {
	cur := p
	if !includeStart {
		if s.Board.wallBetween(cur, dir) {
			return
		}
		cur = cur.Add(dir.Step())
	}
	for s.Board.In(cur) {
		if hit := s.robotAt(cur); hit >= 0 {
			s.damageRobot(hit, dmg)
			return
		}
		if s.Board.wallBetween(cur, dir) {
			return
		}
		cur = cur.Add(dir.Step())
	}
}

func (s *GameState) damageRobot(i, n int) {
	if n <= 0 {
		return
	}
	s.Robots[i].Damage += n
	if s.Robots[i].Damage > 9 {
		s.Robots[i].Damage = 9
	}
	s.Robots[i].tookDamage = true
}

func (s *GameState) heal(i, n int) {
	s.Robots[i].Damage -= n
	if s.Robots[i].Damage < 0 {
		s.Robots[i].Damage = 0
	}
}

// touchCheckpoints advances a robot that ends the register on its next
// checkpoint (in order), and heals/re-archives on checkpoints and repair sites.
func (s *GameState) touchCheckpoints() {
	for i := range s.Robots {
		if !s.active(i) || !s.Board.In(s.Robots[i].Pos) {
			continue
		}
		t := s.Board.At(s.Robots[i].Pos)
		if t.Checkpoint == s.Robots[i].NextCheck {
			s.Robots[i].NextCheck++
			s.Robots[i].ArchivePos = s.Robots[i].Pos
			s.Robots[i].ArchiveDir = s.Robots[i].Facing
			s.heal(i, 1)
			if s.Robots[i].NextCheck > s.Board.NCheck && s.Winner < 0 {
				s.Winner = i
			}
		} else if t.Kind == FloorRepair {
			s.heal(i, 1)
			s.Robots[i].ArchivePos = s.Robots[i].Pos
			s.Robots[i].ArchiveDir = s.Robots[i].Facing
		}
	}
}

// endRound respawns the dead, records stress, draws fresh hands, and returns to
// the programming phase (or ends the game if someone finished).
func (s *GameState) endRound() {
	for i := range s.Robots {
		if s.Robots[i].deadThisRound {
			s.Robots[i].Pos = s.Robots[i].ArchivePos
			s.Robots[i].Facing = s.Robots[i].ArchiveDir
			s.Robots[i].Alive = true
			s.Robots[i].deadThisRound = false
			// Reboot with a fresh 2 damage (RR-faithful) — a reset, NOT an
			// increment, so dying clears an accumulated damage spiral instead of
			// pushing the robot toward a permanent locked hand.
			s.Robots[i].Damage = 2
		}
		s.Robots[i].damagedLastRound = s.Robots[i].tookDamage
		s.Robots[i].tookDamage = false
	}
	if s.Winner >= 0 {
		s.Phase = PhaseDone
		return
	}
	s.beginRound()
}

// logf appends a short line to the resolution log.
func (s *GameState) logf(msg string) { s.Log = append(s.Log, msg) }
