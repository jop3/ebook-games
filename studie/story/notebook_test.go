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
	if len(s.Clues) != 1 {
		t.Fatalf("expected 1 clue, got %d", len(s.Clues))
	}
	// An object with no clue yields nothing.
	if _, ok := AddClueFor(s, OBJ_NONE); ok {
		t.Fatal("OBJ_NONE should yield no clue")
	}
}

func TestCombineValidAndInvalidPairs(t *testing.T) {
	s := New()
	AddClueFor(s, OBJ_BOOTPRINT) // "boot"
	AddClueFor(s, OBJ_RING)      // "ring"
	AddClueFor(s, OBJ_CLOCK)     // "clock"

	// boot + ring is a valid deduction (entry).
	d, ok := Combine(s, 0, 1)
	if !ok || d.ID != "entry" {
		t.Fatalf("boot+ring should deduce 'entry', got %+v ok=%v", d, ok)
	}
	if !s.Deductions["entry"] {
		t.Fatal("the deduction should be recorded on State")
	}
	// order independence: ring + boot is the same deduction.
	if d2, ok := Combine(s, 1, 0); !ok || d2.ID != "entry" {
		t.Fatalf("combine should be order-independent, got %+v ok=%v", d2, ok)
	}
	// ring + clock is not a valid pair.
	if _, ok := Combine(s, 1, 2); ok {
		t.Fatal("ring+clock should not deduce anything")
	}
	// out-of-range / self pairs are rejected.
	if _, ok := Combine(s, 0, 0); ok {
		t.Fatal("a clue combined with itself must be rejected")
	}
	if _, ok := Combine(s, 0, 99); ok {
		t.Fatal("out-of-range index must be rejected")
	}
}

func TestCaseHasThreeDeductions(t *testing.T) {
	if TotalDeductions() != 3 {
		t.Fatalf("expected 3 deductions in the case, got %d", TotalDeductions())
	}
}

// TestSharedEngineDrivesMysteryData sanity-checks that the byte-identical cave
// engine runs correctly against this story's data: a walk gathers clues.
func TestSharedEngineDrivesMysteryData(t *testing.T) {
	s := New()
	if s.Loc != LOC_START {
		t.Fatalf("start loc = %d, want consulting-rooms", s.Loc)
	}
	// Rooms are all lit; darkness path never engages.
	if IsDark(s) {
		t.Fatal("mystery rooms should never be dark")
	}
	// Walk to the study via street→hall→study using the shared Move().
	Move(s, MotStreet)
	Move(s, MotIn)
	Move(s, MotStudy)
	if s.Loc != LOC_STUDY {
		t.Fatalf("expected to reach the study, at %d", s.Loc)
	}
	// The body is present and examinable here.
	if !Present(s, OBJ_BODY) {
		t.Fatal("the body should be present in the study")
	}
	if !containsObj(VisibleObjects(s), OBJ_BODY) {
		t.Fatalf("body should be a visible noun; got %v", VisibleObjects(s))
	}
}

func containsObj(list []ObjID, id ObjID) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}
