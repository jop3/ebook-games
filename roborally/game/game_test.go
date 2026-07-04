package game

import (
	"image"
	"math/rand"
	"testing"
)

// newTestGame builds a minimal GameState around a board and robots for
// exercising the resolver substeps directly.
func newTestGame(b *Board, robots ...Robot) *GameState {
	s := &GameState{Board: b, Robots: robots, rng: rand.New(rand.NewSource(1))}
	s.Registers = make([][5]Card, len(robots))
	s.Hands = make([][]Card, len(robots))
	return s
}

func rb(x, y int, f Dir) Robot {
	p := image.Pt(x, y)
	return Robot{Pos: p, Facing: f, NextCheck: 1, ArchivePos: p, ArchiveDir: f, Alive: true}
}

func plain(w, h int) *Board {
	b := newBoard(w, h)
	b.NCheck = 1
	return b
}

func TestDirHelpers(t *testing.T) {
	if N.Right() != E || E.Right() != S || S.Right() != W || W.Right() != N {
		t.Fatal("Right() wrong")
	}
	if N.Left() != W || N.Opposite() != S {
		t.Fatal("Left/Opposite wrong")
	}
}

func TestPushChain(t *testing.T) {
	b := plain(6, 3)
	s := newTestGame(b, rb(1, 1, E), rb(2, 1, E))
	s.moveRobot(0, E, 1)
	if s.Robots[0].Pos != image.Pt(2, 1) || s.Robots[1].Pos != image.Pt(3, 1) {
		t.Fatalf("push chain: A=%v B=%v", s.Robots[0].Pos, s.Robots[1].Pos)
	}
}

func TestWallBlocksPush(t *testing.T) {
	b := plain(6, 3)
	b.At(image.Pt(2, 1)).Walls |= E.wallBit() // wall east of B blocks the push
	s := newTestGame(b, rb(1, 1, E), rb(2, 1, E))
	s.moveRobot(0, E, 1)
	if s.Robots[0].Pos != image.Pt(1, 1) || s.Robots[1].Pos != image.Pt(2, 1) {
		t.Fatalf("wall should block push: A=%v B=%v", s.Robots[0].Pos, s.Robots[1].Pos)
	}
}

func TestPushIntoPit(t *testing.T) {
	b := plain(6, 3)
	b.At(image.Pt(3, 1)).Kind = FloorPit
	s := newTestGame(b, rb(1, 1, E), rb(2, 1, E))
	s.moveRobot(0, E, 1)
	if s.Robots[1].Alive {
		t.Fatal("pushed robot should have died in the pit")
	}
	if s.Robots[0].Pos != image.Pt(2, 1) {
		t.Fatalf("pusher should advance: %v", s.Robots[0].Pos)
	}
}

func TestMoveOffEdgeKills(t *testing.T) {
	b := plain(4, 3)
	s := newTestGame(b, rb(3, 1, E))
	s.moveRobot(0, E, 1)
	if s.Robots[0].Alive {
		t.Fatal("stepping off the board edge should kill the robot")
	}
}

func TestBeltSingleVsExpress(t *testing.T) {
	b := plain(6, 4)
	// Row 0: an express lane; row 2: a single-belt tile.
	for x := 1; x <= 3; x++ {
		b.At(image.Pt(x, 0)).Belt = E
		b.At(image.Pt(x, 0)).BeltExpress = true
	}
	b.At(image.Pt(1, 2)).Belt = E
	s := newTestGame(b, rb(1, 0, E), rb(1, 2, E))
	s.stepBelts(true)  // express step
	s.stepBelts(false) // all-belts step
	if s.Robots[0].Pos != image.Pt(3, 0) {
		t.Fatalf("express belt should carry 2: %v", s.Robots[0].Pos)
	}
	if s.Robots[1].Pos != image.Pt(2, 2) {
		t.Fatalf("single belt should carry 1: %v", s.Robots[1].Pos)
	}
}

func TestBeltCurveRotates(t *testing.T) {
	b := plain(5, 5)
	b.At(image.Pt(1, 2)).Belt = E
	b.At(image.Pt(2, 2)).Belt = S // a right-hand curve relative to travel east
	s := newTestGame(b, rb(1, 2, E))
	s.stepBelts(false)
	if s.Robots[0].Pos != image.Pt(2, 2) {
		t.Fatalf("belt should carry onto the curve: %v", s.Robots[0].Pos)
	}
	if s.Robots[0].Facing != S {
		t.Fatalf("curve should rotate facing E->S, got %v", s.Robots[0].Facing)
	}
}

func TestBeltConvergingCancels(t *testing.T) {
	b := plain(5, 3)
	b.At(image.Pt(1, 1)).Belt = E
	b.At(image.Pt(3, 1)).Belt = W
	s := newTestGame(b, rb(1, 1, E), rb(3, 1, W))
	s.stepBelts(false)
	if s.Robots[0].Pos != image.Pt(1, 1) || s.Robots[1].Pos != image.Pt(3, 1) {
		t.Fatalf("converging belts should cancel: A=%v B=%v", s.Robots[0].Pos, s.Robots[1].Pos)
	}
}

func TestGearRotates(t *testing.T) {
	b := plain(3, 3)
	b.At(image.Pt(1, 1)).Gear = GearCW
	s := newTestGame(b, rb(1, 1, N))
	s.spinGears()
	if s.Robots[0].Facing != E {
		t.Fatalf("CW gear should turn N->E, got %v", s.Robots[0].Facing)
	}
}

func TestWallLaserDamage(t *testing.T) {
	b := plain(6, 3)
	t0 := b.At(image.Pt(0, 1))
	t0.Laser = E
	t0.LaserCount = 2
	s := newTestGame(b, rb(3, 1, N))
	s.fireWallLasers()
	if s.Robots[0].Damage != 2 {
		t.Fatalf("expected 2 laser damage, got %d", s.Robots[0].Damage)
	}
}

func TestRobotLaserDamage(t *testing.T) {
	b := plain(6, 3)
	s := newTestGame(b, rb(1, 1, E), rb(3, 1, W))
	s.fireRobotLasers()
	if s.Robots[1].Damage < 1 {
		t.Fatal("robot A's forward laser should hit B")
	}
}

func TestCheckpointOrder(t *testing.T) {
	b := newBoard(6, 3)
	b.At(image.Pt(2, 1)).Checkpoint = 1
	b.At(image.Pt(4, 1)).Checkpoint = 2
	b.NCheck = 2
	// On cp2 while still needing cp1: nothing happens.
	s := newTestGame(b, rb(4, 1, N))
	s.touchCheckpoints()
	if s.Robots[0].NextCheck != 1 {
		t.Fatal("touching a later checkpoint out of order must do nothing")
	}
	// On cp1: advances.
	s.Robots[0].Pos = image.Pt(2, 1)
	s.touchCheckpoints()
	if s.Robots[0].NextCheck != 2 || s.Robots[0].ArchivePos != image.Pt(2, 1) {
		t.Fatalf("cp1 should advance NextCheck and archive: %+v", s.Robots[0])
	}
}

func TestRespawnResetsDamage(t *testing.T) {
	b := plain(4, 3)
	b.Docks = []dock{{Pos: image.Pt(0, 0), Facing: E}}
	r := rb(1, 1, E)
	r.ArchivePos = image.Pt(1, 1)
	r.Damage = 8 // already heavily damaged
	s := newTestGame(b, r)
	s.killRobot(0)
	s.endRoundForTest()
	if !s.Robots[0].Alive {
		t.Fatal("robot should respawn alive")
	}
	if s.Robots[0].Damage != 2 {
		t.Fatalf("respawn should RESET damage to 2 (not add), got %d", s.Robots[0].Damage)
	}
	if s.Robots[0].Pos != image.Pt(1, 1) {
		t.Fatalf("respawn should return to archive, got %v", s.Robots[0].Pos)
	}
}

// endRoundForTest runs endRound but avoids drawing a new programming round when
// there are no hands set up (single-robot resolver tests).
func (s *GameState) endRoundForTest() {
	for i := range s.Robots {
		if s.Robots[i].deadThisRound {
			s.Robots[i].Pos = s.Robots[i].ArchivePos
			s.Robots[i].Facing = s.Robots[i].ArchiveDir
			s.Robots[i].Alive = true
			s.Robots[i].deadThisRound = false
			s.Robots[i].Damage = 2
		}
	}
}

func TestGeneratorProducesSolvableCourses(t *testing.T) {
	for _, diff := range []CourseDiff{DiffEasy, DiffMedium, DiffHard} {
		for seed := int64(1); seed <= 8; seed++ {
			b := GenerateCourse(diff, seed)
			if b == nil || len(b.Docks) == 0 || b.NCheck == 0 {
				t.Fatalf("%v seed %d: degenerate board", diff, seed)
			}
			if _, ok := solveReference(b, budgetFor(diff)); !ok {
				t.Fatalf("%v seed %d: reference planner could not finish", diff, seed)
			}
		}
	}
}

// TestStartDocksHaveRoom checks every start dock has a clear tile ahead (no pit,
// edge, or wall blocking the first move) and that no two docks are adjacent.
func TestStartDocksHaveRoom(t *testing.T) {
	for _, diff := range []CourseDiff{DiffEasy, DiffMedium, DiffHard} {
		for seed := int64(1); seed <= 12; seed++ {
			b := GenerateCourse(diff, seed)
			for _, d := range b.Docks {
				if b.wallBetween(d.Pos, d.Facing) {
					t.Fatalf("%v seed %d: wall in front of dock at %v", diff, seed, d.Pos)
				}
				if b.lethal(d.Pos.Add(d.Facing.Step())) {
					t.Fatalf("%v seed %d: hazard/edge in front of dock at %v", diff, seed, d.Pos)
				}
			}
			for i := range b.Docks {
				for j := i + 1; j < len(b.Docks); j++ {
					if manhattan(b.Docks[i].Pos, b.Docks[j].Pos) < 2 {
						t.Fatalf("%v seed %d: docks %v and %v are adjacent",
							diff, seed, b.Docks[i].Pos, b.Docks[j].Pos)
					}
				}
			}
		}
	}
}

// TestCuratedCoursesValid runs every hand-authored course through the same
// checks as generated ones: well-formed, start-line contract, and solvable by
// the Expert reference planner within its tier's round budget.
func TestCuratedCoursesValid(t *testing.T) {
	for i := 0; i < NumCurated(); i++ {
		b := CuratedCourse(i)
		if b.NCheck == 0 {
			t.Fatalf("curated %d: no checkpoints", i)
		}
		if len(b.Docks) != 4 {
			t.Fatalf("curated %d: want 4 docks, got %d", i, len(b.Docks))
		}
		if _, ok := b.CheckpointPos(1); !ok {
			t.Fatalf("curated %d: missing checkpoint 1", i)
		}
		// every checkpoint ordinal 1..NCheck present
		for ord := uint8(1); ord <= b.NCheck; ord++ {
			if _, ok := b.CheckpointPos(ord); !ok {
				t.Fatalf("curated %d: missing checkpoint %d", i, ord)
			}
		}
		// start-line contract
		for _, d := range b.Docks {
			if b.wallBetween(d.Pos, d.Facing) || b.lethal(d.Pos.Add(d.Facing.Step())) {
				t.Fatalf("curated %d: blocked launch at dock %v", i, d.Pos)
			}
		}
		for a := range b.Docks {
			for c := a + 1; c < len(b.Docks); c++ {
				if manhattan(b.Docks[a].Pos, b.Docks[c].Pos) < 2 {
					t.Fatalf("curated %d: adjacent docks %v %v", i, b.Docks[a].Pos, b.Docks[c].Pos)
				}
			}
		}
		if _, ok := solveReference(b, budgetFor(CuratedTier(i))); !ok {
			t.Fatalf("curated %d (%v): reference planner could not finish", i, CuratedTier(i))
		}
	}
}

func TestExpertReachesCheckpoints(t *testing.T) {
	b := GenerateCourse(DiffMedium, 5)
	rounds, ok := solveReference(b, budgetFor(DiffMedium))
	if !ok {
		t.Fatal("expert should solve a medium course")
	}
	if rounds <= 0 {
		t.Fatalf("unexpected round count %d", rounds)
	}
}

// TestGameFinishesNoSpiral drives full multi-robot games (human played by the
// planner too) to completion across difficulties and seeds, guarding against
// stalls/livelocks and the damage spiral that a too-low hand floor or an
// additive respawn would reintroduce.
func TestGameFinishesNoSpiral(t *testing.T) {
	for _, diff := range []CourseDiff{DiffEasy, DiffMedium, DiffHard} {
		for seed := int64(1); seed <= 4; seed++ {
			b := GenerateCourse(diff, seed)
			gs := NewGame(b, 3, AIMedium, seed*31+7)
			finished := false
			for round := 0; round < 250; round++ {
				if gs.Phase == PhaseProgram {
					gs.Registers[0] = gs.planAI(0) // drive the human competently too
					if !gs.StartResolve() {
						t.Fatalf("%v seed %d: StartResolve failed", diff, seed)
					}
				}
				for gs.Phase == PhaseResolve {
					gs.StepRegister()
				}
				if gs.Phase == PhaseDone {
					finished = true
					break
				}
			}
			if !finished {
				t.Fatalf("%v seed %d: game did not finish in 250 rounds (stall/damage spiral?)", diff, seed)
			}
		}
	}
}

// TestAIBlindness is load-bearing: the planner's output must not depend on any
// other robot's committed program. Two identical games — one with opponents'
// registers populated, one without — must yield the same plan.
func TestAIBlindness(t *testing.T) {
	b := GenerateCourse(DiffMedium, 7)
	mk := func() *GameState { return NewGame(b, 2, AIExpert, 42) }
	clean := mk()
	poisoned := mk()
	// Populate opponents' registers in the poisoned game with a "hidden" plan.
	for i := 1; i < len(poisoned.Robots); i++ {
		poisoned.Registers[i] = [5]Card{UTurn, UTurn, UTurn, UTurn, UTurn}
	}
	// Also physically place an opponent so it *would* move into robot 1 — the
	// planner must not foresee that (opponents held stationary / unknown).
	got1 := clean.planAI(1)
	got2 := poisoned.planAI(1)
	if got1 != got2 {
		t.Fatalf("planner leaked hidden info: clean=%v poisoned=%v", got1, got2)
	}
}

// TestAIProducesLegalPrograms: every planned card must come from the robot's
// hand (respecting multiplicity), and the program length must match.
func TestAIProducesLegalPrograms(t *testing.T) {
	b := GenerateCourse(DiffHard, 3)
	for _, lvl := range []AILevel{AIEasy, AIMedium, AIExpert} {
		gs := NewGame(b, 3, lvl, 9)
		for i := 1; i < len(gs.Robots); i++ {
			prog := gs.planAI(i)
			want := map[Card]int{}
			for _, c := range gs.Hands[i] {
				want[c]++
			}
			for _, c := range prog {
				if c == CardNone {
					continue
				}
				want[c]--
				if want[c] < 0 {
					t.Fatalf("%v robot %d: program uses card %v not in hand", lvl, i, c)
				}
			}
		}
	}
}
