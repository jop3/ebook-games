package game

import (
	"image"
	"math/rand"
	"sort"
)

// AILevel is the opponent-competence tier chosen on the menu. It is independent
// of course difficulty.
type AILevel int

const (
	AIEasy   AILevel = iota // Nybörjare: clumsy, high fumble
	AIMedium                // Van: the fun default
	AIExpert                // Expert: tough, still fumbles rarely
)

func (l AILevel) String() string {
	switch l {
	case AIEasy:
		return "Nybörjare"
	case AIMedium:
		return "Van"
	case AIExpert:
		return "Expert"
	}
	return "?"
}

// personality flavours an AI's scoring so a table of bots feels varied.
type personality uint8

const (
	pBalanced personality = iota
	pRusher
	pBully
	pCautious
)

// AIProfile bundles the competence and human-error parameters for one AI robot.
type AIProfile struct {
	level      AILevel
	pers       personality
	baseFumble float64 // base probability of a programming mistake
	stressGain float64 // how strongly pressure raises the fumble chance
	lookLasers bool    // whether scoring accounts for wall-laser damage
	beam       int     // beam width for the planner
}

func aiProfiles(level AILevel, nAI int, rng *rand.Rand) []AIProfile {
	base := AIProfile{level: level}
	switch level {
	case AIEasy:
		base.baseFumble, base.stressGain, base.lookLasers, base.beam = 0.35, 0.20, false, 4
	case AIMedium:
		base.baseFumble, base.stressGain, base.lookLasers, base.beam = 0.15, 0.12, true, 8
	default:
		base.baseFumble, base.stressGain, base.lookLasers, base.beam = 0.04, 0.06, true, 12
	}
	persCycle := []personality{pBalanced, pRusher, pBully, pCautious}
	out := make([]AIProfile, nAI)
	for i := 0; i < nAI; i++ {
		p := base
		p.pers = persCycle[i%len(persCycle)]
		out[i] = p
	}
	return out
}

// --- Blind planner ----------------------------------------------------------
//
// A hard fairness rule (spec §6, Layer 0): the planner sees ONLY public state —
// the board, every robot's *current* visible position/facing, and its OWN hand.
// It never reads any robot's committed Registers. To make that structural, the
// planner takes an explicit publicView built from current positions only; the
// Registers slice is simply not part of it, so it cannot be consulted.

type publicView struct {
	board    *Board
	self     simRobot
	blockers map[image.Point]bool // other robots' current tiles (immovable obstacles)
	hand     []Card
	profile  AIProfile
}

type simRobot struct {
	pos       image.Point
	facing    Dir
	damage    int
	nextCheck uint8
	alive     bool
}

// planAI plans robot i's five registers, blind to every other robot's program.
func (s *GameState) planAI(i int) [5]Card {
	pv := publicView{
		board:   s.Board,
		self:    simRobot{s.Robots[i].Pos, s.Robots[i].Facing, s.Robots[i].Damage, s.Robots[i].NextCheck, true},
		hand:    s.Hands[i],
		profile: s.Robots[i].Profile,
	}
	pv.blockers = map[image.Point]bool{}
	for j := range s.Robots {
		if j == i || !s.Robots[j].Alive || s.Robots[j].NextCheck > s.Board.NCheck {
			continue
		}
		if s.Board.In(s.Robots[j].Pos) {
			pv.blockers[s.Robots[j].Pos] = true
		}
	}
	prog := planProgram(pv)
	return applyFumble(prog, &s.Robots[i], pv, s.rng)
}

// planProgram runs the beam search and returns the best full program.
func planProgram(pv publicView) [5]Card {
	counts := map[Card]int{}
	total := 0
	for _, c := range pv.hand {
		counts[c]++
		total++
	}
	regs := 5
	if total < regs {
		regs = total
	}
	type partial struct {
		prog   [5]Card
		counts map[Card]int
		sim    simRobot
		score  int
	}
	cloneCounts := func(m map[Card]int) map[Card]int {
		n := map[Card]int{}
		for k, v := range m {
			n[k] = v
		}
		return n
	}
	beamW := pv.profile.beam
	if beamW < 1 {
		beamW = 4
	}
	start := partial{counts: counts, sim: pv.self}
	for r := 0; r < 5; r++ {
		start.prog[r] = CardNone
	}
	beam := []partial{start}
	types := []Card{Move1, Move2, Move3, BackUp, RotR, RotL, UTurn}
	for r := 0; r < regs; r++ {
		var next []partial
		for _, p := range beam {
			for _, c := range types {
				if p.counts[c] == 0 {
					continue
				}
				np := partial{prog: p.prog, counts: cloneCounts(p.counts), sim: p.sim}
				np.prog[r] = c
				np.counts[c]--
				np.sim = simRegister(pv, np.sim, c)
				np.score = scoreSim(pv, np.sim)
				next = append(next, np)
			}
		}
		if len(next) == 0 {
			break
		}
		sort.Slice(next, func(a, b int) bool { return next[a].score > next[b].score })
		if len(next) > beamW {
			next = next[:beamW]
		}
		beam = next
	}
	best := beam[0]
	for _, p := range beam[1:] {
		if p.score > best.score {
			best = p
		}
	}
	return best.prog
}

// simRegister applies one register card plus board elements to a lone robot,
// with other robots held stationary as immovable blockers (the blind model).
func simRegister(pv publicView, r simRobot, c Card) simRobot {
	if !r.alive {
		return r
	}
	switch c {
	case RotR:
		r.facing = r.facing.Right()
	case RotL:
		r.facing = r.facing.Left()
	case UTurn:
		r.facing = r.facing.Opposite()
	case Move1:
		r = simMove(pv, r, r.facing, 1)
	case Move2:
		r = simMove(pv, r, r.facing, 2)
	case Move3:
		r = simMove(pv, r, r.facing, 3)
	case BackUp:
		r = simMove(pv, r, r.facing.Opposite(), 1)
	}
	// Board elements on the acting robot only.
	r = simBelt(pv, r, true)
	r = simBelt(pv, r, false)
	if r.alive && pv.board.In(r.pos) {
		switch pv.board.At(r.pos).Gear {
		case GearCW:
			r.facing = r.facing.Right()
		case GearCCW:
			r.facing = r.facing.Left()
		}
	}
	if pv.profile.lookLasers && r.alive && pv.board.In(r.pos) {
		r.damage += simLaserHits(pv, r.pos)
	}
	if r.alive && pv.board.In(r.pos) && pv.board.At(r.pos).Checkpoint == r.nextCheck {
		r.nextCheck++
	}
	return r
}

func simMove(pv publicView, r simRobot, dir Dir, n int) simRobot {
	for k := 0; k < n; k++ {
		if !r.alive {
			return r
		}
		if pv.board.wallBetween(r.pos, dir) {
			return r
		}
		to := r.pos.Add(dir.Step())
		if pv.blockers[to] {
			return r // can't push in the blind sim
		}
		r.pos = to
		if pv.board.lethal(to) {
			r.alive = false
			return r
		}
	}
	return r
}

func simBelt(pv publicView, r simRobot, expressOnly bool) simRobot {
	if !r.alive || !pv.board.In(r.pos) {
		return r
	}
	t := pv.board.At(r.pos)
	if t.Belt == DirNone || (expressOnly && !t.BeltExpress) {
		return r
	}
	if pv.board.wallBetween(r.pos, t.Belt) {
		return r
	}
	to := r.pos.Add(t.Belt.Step())
	if pv.blockers[to] {
		return r
	}
	oldDir := t.Belt
	r.pos = to
	if pv.board.lethal(to) {
		r.alive = false
		return r
	}
	nd := pv.board.At(to).Belt
	if nd != DirNone && nd != oldDir {
		if nd == oldDir.Right() {
			r.facing = r.facing.Right()
		} else if nd == oldDir.Left() {
			r.facing = r.facing.Left()
		}
	}
	return r
}

// simLaserHits returns the wall-laser damage a robot on p would take.
func simLaserHits(pv publicView, p image.Point) int {
	dmg := 0
	for y := 0; y < pv.board.H; y++ {
		for x := 0; x < pv.board.W; x++ {
			t := pv.board.At(image.Pt(x, y))
			if t.Laser == DirNone {
				continue
			}
			cur := image.Pt(x, y)
			for pv.board.In(cur) {
				if cur == p {
					dmg += int(t.LaserCount)
					break
				}
				if pv.board.blockedBeam(cur, t.Laser, pv.blockers) {
					break
				}
				cur = cur.Add(t.Laser.Step())
			}
		}
	}
	return dmg
}

// blockedBeam reports whether a beam from cur in dir is stopped before the next
// tile (a wall, or a blocker robot standing in the way).
func (b *Board) blockedBeam(cur image.Point, dir Dir, blockers map[image.Point]bool) bool {
	if b.wallBetween(cur, dir) {
		return true
	}
	nb := cur.Add(dir.Step())
	return blockers[nb]
}

// scoreSim rates a simulated end position (higher is better).
func scoreSim(pv publicView, r simRobot) int {
	if !r.alive {
		return -100000
	}
	score := 0
	if r.nextCheck > pv.board.NCheck {
		return 1000000 - r.damage*10 // finished
	}
	if cp, ok := pv.board.CheckpointPos(r.nextCheck); ok {
		d := bfsDist(pv.board, r.pos, cp)
		score -= d * 100
	}
	// Progress toward later checkpoints is worth a lot.
	score += int(r.nextCheck) * 500
	score -= r.damage * dmgWeight(pv.profile)
	// Alignment nudge: facing toward the goal is slightly preferred.
	if cp, ok := pv.board.CheckpointPos(r.nextCheck); ok {
		if facingToward(r.pos, r.facing, cp) {
			score += 15
		}
	}
	return score
}

func dmgWeight(p AIProfile) int {
	switch p.pers {
	case pCautious:
		return 40
	case pRusher:
		return 6
	}
	return 12
}

func facingToward(pos image.Point, f Dir, target image.Point) bool {
	switch f {
	case N:
		return target.Y < pos.Y
	case S:
		return target.Y > pos.Y
	case E:
		return target.X > pos.X
	case W:
		return target.X < pos.X
	}
	return false
}

// bfsDist is the shortest step count from a to b over floor tiles, respecting
// walls and avoiding pits. Returns a large number if unreachable.
func bfsDist(b *Board, a, target image.Point) int {
	if a == target {
		return 0
	}
	const inf = 1 << 20
	dist := make([]int, b.W*b.H)
	for i := range dist {
		dist[i] = inf
	}
	if !b.In(a) {
		return inf
	}
	dist[a.Y*b.W+a.X] = 0
	queue := []image.Point{a}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		d := dist[cur.Y*b.W+cur.X]
		for _, dir := range []Dir{N, E, S, W} {
			if b.wallBetween(cur, dir) {
				continue
			}
			nb := cur.Add(dir.Step())
			if !b.In(nb) || b.At(nb).Kind == FloorPit {
				continue
			}
			idx := nb.Y*b.W + nb.X
			if dist[idx] > d+1 {
				dist[idx] = d + 1
				if nb == target {
					return d + 1
				}
				queue = append(queue, nb)
			}
		}
	}
	return inf
}

// --- Human-error / stress model ---------------------------------------------

// applyFumble perturbs a planned program the way a rushed human would, with a
// probability that rises with pressure (near a pit/edge, damaged, behind, or
// one leg from a checkpoint). Uses the game RNG so it is reproducible.
func applyFumble(prog [5]Card, self *Robot, pv publicView, rng *rand.Rand) [5]Card {
	p := pv.profile.baseFumble + pv.profile.stressGain*stressLevel(self, pv)
	if p > 0.9 {
		p = 0.9
	}
	if rng.Float64() >= p {
		return prog
	}
	// Choose a fumble kind.
	switch rng.Intn(4) {
	case 0: // swap two adjacent registers (the classic slip)
		r := rng.Intn(4)
		prog[r], prog[r+1] = prog[r+1], prog[r]
	case 1: // substitute a plausible-but-worse card from the hand
		r := rng.Intn(5)
		if alt, ok := altCard(prog, pv.hand); ok {
			prog[r] = alt
		}
	case 2: // panic: shuffle the last two registers
		prog[3], prog[4] = prog[4], prog[3]
	default: // over-commit a rotation where a move was planned
		r := rng.Intn(5)
		if prog[r].IsMove() {
			prog[r] = RotR
		}
	}
	return prog
}

// stressLevel returns a 0..~1 pressure estimate for the fumble model.
func stressLevel(self *Robot, pv publicView) float64 {
	s := 0.0
	// Adjacent to a pit or the board edge.
	for _, d := range []Dir{N, E, S, W} {
		nb := self.Pos.Add(d.Step())
		if pv.board.lethal(nb) {
			s += 0.4
			break
		}
	}
	if self.damagedLastRound {
		s += 0.3
	}
	// One leg from the next checkpoint (choke pressure).
	if cp, ok := pv.board.CheckpointPos(self.NextCheck); ok {
		if bfsDist(pv.board, self.Pos, cp) <= 2 {
			s += 0.3
		}
	}
	if s > 1 {
		s = 1
	}
	return s
}

// altCard picks a card from the hand that the program hasn't already used up
// (best-effort), so a fumble substitution can never plant a card beyond its
// multiplicity in the hand — the invariant TestAIProducesLegalPrograms exists
// to protect.
func altCard(prog [5]Card, hand []Card) (Card, bool) {
	used := map[Card]int{}
	for _, c := range prog {
		used[c]++
	}
	for _, c := range hand {
		if used[c] > 0 {
			used[c]--
			continue
		}
		return c, true
	}
	return CardNone, false
}
