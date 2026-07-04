package story

import "testing"

func TestExamineRecordsCluesOnce(t *testing.T) {
	s := New()
	c, added := AddClueFor(s, OBJ_BODY)
	if !added || c.ID != "body" {
		t.Fatalf("examining the body should record the 'body' clue, got %+v added=%v", c, added)
	}
	if _, again := AddClueFor(s, OBJ_BODY); again {
		t.Fatal("the same clue must not be recorded twice")
	}
	if _, ok := AddClueFor(s, OBJ_NONE); ok {
		t.Fatal("OBJ_NONE should yield no clue")
	}
}

func TestCombineChain(t *testing.T) {
	s := New()
	for _, o := range []ObjID{OBJ_BODY, OBJ_RING, OBJ_BOOT, OBJ_LATCH, OBJ_CLOCK, OBJ_CABTIME, OBJ_LETTER, OBJ_LEDGER} {
		AddClueFor(s, o)
	}
	want := map[string][2]string{
		"poison": {"body", "ring"},
		"entry":  {"boot", "latch"},
		"timing": {"clock", "cabtime"},
		"motive": {"letter", "ledger"},
	}
	for id, pair := range want {
		i, j := clueIndex(t, s, pair[0]), clueIndex(t, s, pair[1])
		d, ok := Combine(s, i, j)
		if !ok || d.ID != id {
			t.Fatalf("%s+%s should deduce %q, got %+v ok=%v", pair[0], pair[1], id, d, ok)
		}
	}
	if DeductionCount(s) != 4 || TotalDeductions() != 4 {
		t.Fatalf("expected all 4 deductions, have %d/%d", DeductionCount(s), TotalDeductions())
	}
	// A non-matching pair yields nothing.
	if _, ok := Combine(s, clueIndex(t, s, "body"), clueIndex(t, s, "clock")); ok {
		t.Fatal("body+clock should not deduce anything")
	}
}

func TestAccusationRequiresRightChargeAndSupport(t *testing.T) {
	// A correct charge with NO deductions is refused (unsupported).
	s := New()
	if v := Accuse(s, "crole", "poison", "debt"); v.Win {
		t.Fatal("a correct charge with no deductions must not win")
	}

	// Make all deductions.
	s = fullyDeduced(t)

	// Wrong pillars are refused even when fully supported.
	if v := Accuse(s, "hudd", "poison", "debt"); v.Win {
		t.Fatal("wrong culprit must not win")
	}
	if v := Accuse(s, "crole", "blow", "debt"); v.Win {
		t.Fatal("wrong method must not win")
	}
	if v := Accuse(s, "crole", "poison", "robbery"); v.Win {
		t.Fatal("wrong motive must not win")
	}

	// The correct, fully-supported charge wins and sets Won.
	v := Accuse(s, "crole", "poison", "debt")
	if !v.Win || !s.Won {
		t.Fatalf("the correct supported charge should win; v=%+v won=%v", v, s.Won)
	}
	if v.Text == "" {
		t.Fatal("a winning verdict should carry the resolution text")
	}
}

func TestAccusationNamesMissingDeduction(t *testing.T) {
	s := fullyDeduced(t)
	delete(s.Deductions, "motive") // undo one
	v := Accuse(s, "crole", "poison", "debt")
	if v.Win {
		t.Fatal("must not win with a missing deduction")
	}
	if !containsSub(v.Text, deductionLabelSv["motive"]) {
		t.Fatalf("verdict should name the missing deduction (motive); got %q", v.Text)
	}
}

func TestSharedEngineDrivesMysteryData(t *testing.T) {
	s := New()
	if s.Loc != LOC_START || IsDark(s) {
		t.Fatal("should start lit, at the consulting-rooms")
	}
	Move(s, MotStreet)
	Move(s, MotIn)
	Move(s, MotStudy)
	if s.Loc != LOC_STUDY {
		t.Fatalf("expected the study, at %d", s.Loc)
	}
	if !containsObj(VisibleObjects(s), OBJ_BODY) {
		t.Fatalf("body should be a visible noun; got %v", VisibleObjects(s))
	}
}

// --- helpers ---

func fullyDeduced(t *testing.T) *State {
	t.Helper()
	s := New()
	for _, o := range []ObjID{OBJ_BODY, OBJ_RING, OBJ_BOOT, OBJ_LATCH, OBJ_CLOCK, OBJ_CABTIME, OBJ_LETTER, OBJ_LEDGER} {
		AddClueFor(s, o)
	}
	pairs := [][2]string{{"body", "ring"}, {"boot", "latch"}, {"clock", "cabtime"}, {"letter", "ledger"}}
	for _, p := range pairs {
		if _, ok := Combine(s, clueIndex(t, s, p[0]), clueIndex(t, s, p[1])); !ok {
			t.Fatalf("failed to deduce from %v", p)
		}
	}
	return s
}

func clueIndex(t *testing.T, s *State, id string) int {
	t.Helper()
	for i, c := range s.Clues {
		if c.ID == id {
			return i
		}
	}
	t.Fatalf("clue %q not present", id)
	return -1
}

func containsObj(list []ObjID, id ObjID) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
