package game

import "testing"

// TestDetourReciprocity: firing from the exit point of a detour should trace
// the reverse path and emerge at the original entry point. This is a strong
// invariant of correct Black Box physics.
func TestDetourReciprocity(t *testing.T) {
	g := NewGrid(8, 8)
	g.SetAtom(2, 4, true) // single deflection
	eps := g.EdgePoints()

	entry, _ := g.edgePointAt(3, 0, dirDown)
	forward := g.Fire(entry)
	if forward.Outcome != OutcomeDetour {
		t.Fatalf("expected detour, got %v", forward.Outcome)
	}
	// Fire back from the exit point.
	back := g.Fire(eps[forward.ExitIndex])
	if back.Outcome != OutcomeDetour {
		t.Fatalf("reverse ray expected detour, got %v", back.Outcome)
	}
	if back.ExitIndex != entry.Index {
		t.Fatalf("reciprocity broken: forward %d->%d, back %d->%d",
			entry.Index, forward.ExitIndex, forward.ExitIndex, back.ExitIndex)
	}
}

// TestScoringPerfect: correctly identifying all atoms yields score == rays.
func TestScoringPerfect(t *testing.T) {
	s := NewGameSeeded(Presets[1], 42) // 8x8, 4 atoms
	// Fire a couple of rays.
	s.FireAt(0)
	s.FireAt(5)
	s.EnterGuessing()
	for _, a := range s.Grid.Atoms() {
		s.ToggleGuess(a.X, a.Y)
	}
	s.Submit()
	if !s.Solved() {
		t.Fatalf("expected solved; correct=%d wrong=%d missed=%d",
			s.CorrectAtoms, s.WrongAtoms, s.MissedAtoms)
	}
	if s.Score != s.RaysFired() {
		t.Fatalf("perfect score should equal rays fired (%d), got %d",
			s.RaysFired(), s.Score)
	}
}

// TestScoringWrongPenalty: wrong guesses add the configured penalty.
func TestScoringWrongPenalty(t *testing.T) {
	s := NewGameSeeded(Presets[0], 7) // 6x6, 3 atoms
	s.FireAt(0)
	s.EnterGuessing()
	// Guess an empty cell deliberately. Find one.
	var wrong Cell
	found := false
	for y := 0; y < s.Grid.H && !found; y++ {
		for x := 0; x < s.Grid.W && !found; x++ {
			if !s.Grid.HasAtom(x, y) {
				wrong = Cell{x, y}
				found = true
			}
		}
	}
	s.ToggleGuess(wrong.X, wrong.Y)
	s.Submit()
	if s.WrongAtoms != 1 {
		t.Fatalf("expected 1 wrong atom, got %d", s.WrongAtoms)
	}
	want := s.RaysFired() + s.Cfg.WrongAtomPenalty
	if s.Score != want {
		t.Fatalf("expected score %d (rays %d + penalty %d), got %d",
			want, s.RaysFired(), s.Cfg.WrongAtomPenalty, s.Score)
	}
}

// TestReFireIsIdempotent: firing the same edge point twice does not add a new
// ray or change the marker.
func TestReFireIsIdempotent(t *testing.T) {
	s := NewGameSeeded(Presets[1], 99)
	fr1, new1 := s.FireAt(0)
	if !new1 {
		t.Fatalf("first fire should be new")
	}
	fr2, new2 := s.FireAt(0)
	if new2 {
		t.Fatalf("re-fire should not be new")
	}
	if fr1.Marker != fr2.Marker {
		t.Fatalf("re-fire marker changed: %d vs %d", fr1.Marker, fr2.Marker)
	}
	if s.RaysFired() != 1 {
		t.Fatalf("expected 1 ray after re-fire, got %d", s.RaysFired())
	}
}

// TestDetourSharesMarkerOnBothEnds: tapping the paired exit point of a detour
// shows the same fired ray record (same marker) rather than firing anew.
func TestDetourSharesMarkerOnBothEnds(t *testing.T) {
	g := NewGrid(8, 8)
	g.SetAtom(2, 4, true)
	s := &GameState{Cfg: Presets[1], Grid: g, Phase: PhaseProbing,
		firedIndex: map[int]int{}, Guesses: map[Cell]bool{}, nextMark: 1}

	entry, _ := g.edgePointAt(3, 0, dirDown)
	fr, isNew := s.FireAt(entry.Index)
	if !isNew || fr.Result.Outcome != OutcomeDetour {
		t.Fatalf("expected new detour, got new=%v outcome=%v", isNew, fr.Result.Outcome)
	}
	// Tapping the exit point should return the SAME record, not a new ray.
	fr2, isNew2 := s.FireAt(fr.Result.ExitIndex)
	if isNew2 {
		t.Fatalf("tapping paired exit should not fire a new ray")
	}
	if fr2.Marker != fr.Marker {
		t.Fatalf("paired endpoints should share marker: %d vs %d", fr.Marker, fr2.Marker)
	}
	if s.RaysFired() != 1 {
		t.Fatalf("expected 1 ray, got %d", s.RaysFired())
	}
}
