package story

import (
	"reflect"
	"testing"
)

// exitTo finds the reachable exit whose destination is dst and returns its
// motion, failing the test if there is no such exit from the current room.
func exitTo(t *testing.T, s *State, dst LocID) Motion {
	t.Helper()
	for _, e := range Exits(s) {
		if e.Dest == dst {
			return e.Motion
		}
	}
	t.Fatalf("no exit from %d to %d; exits=%v", s.Loc, dst, Exits(s))
	return 0
}

// walkTo moves the player one step to an adjacent room dst.
func walkTo(t *testing.T, s *State, dst LocID) {
	t.Helper()
	Move(s, exitTo(t, s, dst))
	if s.Loc != dst {
		t.Fatalf("walk to %d landed at %d", dst, s.Loc)
	}
}

func TestNewInitialState(t *testing.T) {
	s := New()
	if s.Loc != LOC_START {
		t.Fatalf("start loc = %d, want %d", s.Loc, LOC_START)
	}
	if s.ObjState[OBJ_LAMP] != LAMP_DARK {
		t.Fatal("lamp should start dark")
	}
	if s.ObjState[OBJ_GRATE] != GRATE_CLOSED {
		t.Fatal("grate should start locked")
	}
	if s.ObjAt[OBJ_KEYS] != LOC_BUILDING || s.ObjAt[OBJ_LAMP] != LOC_BUILDING {
		t.Fatal("keys and lamp should start in the well house")
	}
	// The road is lit; you can see it.
	if IsDark(s) {
		t.Fatal("LOC_START is lit, should not be dark")
	}
}

func TestScriptedWalk(t *testing.T) {
	s := New()

	// Into the well house; pick up the lamp and keys.
	walkTo(t, s, LOC_BUILDING)
	vis := VisibleObjects(s)
	if !containsObj(vis, OBJ_LAMP) || !containsObj(vis, OBJ_KEYS) {
		t.Fatalf("well house should show lamp and keys, got %v", vis)
	}
	Act(s, VerbTake, OBJ_LAMP)
	Act(s, VerbTake, OBJ_KEYS)
	if !s.Carried[OBJ_LAMP] || !s.Carried[OBJ_KEYS] {
		t.Fatal("failed to pick up lamp/keys")
	}

	// Out to the road and over to the grate depression.
	walkTo(t, s, LOC_START)
	walkTo(t, s, LOC_GRATE)

	// The grate is locked: there must be no downward exit yet.
	for _, e := range Exits(s) {
		if e.Dest == LOC_BELOWGRATE {
			t.Fatal("grate is locked but a descent exit is offered")
		}
	}

	// Unlock it with the keys, then descend.
	Act(s, VerbUnlock, OBJ_GRATE)
	if s.ObjState[OBJ_GRATE] != GRATE_OPEN {
		t.Fatal("grate did not unlock")
	}
	walkTo(t, s, LOC_BELOWGRATE)

	// Deeper into the dark debris room.
	walkTo(t, s, LOC_DEBRIS)
	if !IsDark(s) {
		t.Fatal("debris room should be pitch dark without a lit lamp")
	}
	if got := Describe(s); got[0] != msgPitchDark {
		t.Fatalf("dark room description = %q, want pitch-dark warning", got[0])
	}

	// Light the lamp — the room becomes visible.
	Act(s, VerbLight, OBJ_LAMP)
	if IsDark(s) {
		t.Fatal("lamp is lit but room still reads as dark")
	}
	if s.ObjState[OBJ_LAMP] != LAMP_BRIGHT {
		t.Fatal("lamp state not bright after LIGHT")
	}

	// Discovering the debris room reveals the XYZZY scrawl.
	if !s.Known[MagicWordText[MotXYZZY]] {
		t.Fatal("visiting the debris room should discover XYZZY")
	}
}

func TestTakeDropMovesObject(t *testing.T) {
	s := New()
	// Reach the debris room with a lit lamp so the rod is visible.
	walkTo(t, s, LOC_BUILDING)
	Act(s, VerbTake, OBJ_LAMP)
	Act(s, VerbTake, OBJ_KEYS)
	Act(s, VerbLight, OBJ_LAMP)
	walkTo(t, s, LOC_START)
	walkTo(t, s, LOC_GRATE)
	Act(s, VerbUnlock, OBJ_GRATE)
	walkTo(t, s, LOC_BELOWGRATE)
	walkTo(t, s, LOC_DEBRIS)

	// The black rod is here; take it.
	if !containsObj(VisibleObjects(s), OBJ_ROD) {
		t.Fatalf("rod should be visible in lit debris room, got %v", VisibleObjects(s))
	}
	Act(s, VerbTake, OBJ_ROD)
	if !s.Carried[OBJ_ROD] {
		t.Fatal("rod not carried after TAKE")
	}
	if containsObj(VisibleObjects(s), OBJ_ROD) {
		t.Fatal("rod still on the ground after being taken")
	}

	// Drop it back into the room.
	Act(s, VerbDrop, OBJ_ROD)
	if s.Carried[OBJ_ROD] {
		t.Fatal("rod still carried after DROP")
	}
	if s.ObjAt[OBJ_ROD] != LOC_DEBRIS {
		t.Fatalf("rod dropped at %d, want debris room", s.ObjAt[OBJ_ROD])
	}
	if !containsObj(VisibleObjects(s), OBJ_ROD) {
		t.Fatal("dropped rod should be visible again")
	}
}

func TestXyzzyTeleport(t *testing.T) {
	s := New()
	// XYZZY is inert until discovered.
	if got := Move(s, MotXYZZY); got[0] != msgNothing {
		t.Fatalf("undiscovered XYZZY = %q, want nothing", got[0])
	}
	// Discover it by visiting the debris room, then teleport building<->debris.
	s.Known[MagicWordText[MotXYZZY]] = true
	s.Loc = LOC_BUILDING
	s.OldLoc = LOC_BUILDING
	Move(s, MotXYZZY)
	if s.Loc != LOC_DEBRIS {
		t.Fatalf("XYZZY from building went to %d, want debris", s.Loc)
	}
	Move(s, MotXYZZY)
	if s.Loc != LOC_BUILDING {
		t.Fatalf("XYZZY from debris went to %d, want building", s.Loc)
	}
}

func TestBirdPuzzle(t *testing.T) {
	s := New()
	s.Loc = LOC_BIRDCHAMBER
	s.ObjState[OBJ_LAMP] = LAMP_BRIGHT
	s.Carried[OBJ_LAMP] = true

	// Without the cage you cannot hold the bird.
	Act(s, VerbTake, OBJ_BIRD)
	if s.Carried[OBJ_BIRD] {
		t.Fatal("caught the bird with no cage")
	}
	// With the cage but carrying the rod, the bird bolts.
	s.Carried[OBJ_CAGE] = true
	s.Carried[OBJ_ROD] = true
	Act(s, VerbTake, OBJ_BIRD)
	if s.Carried[OBJ_BIRD] {
		t.Fatal("caught the bird while holding the rod")
	}
	// Drop the rod: now the bird can be caged.
	s.Carried[OBJ_ROD] = false
	Act(s, VerbTake, OBJ_BIRD)
	if !s.Carried[OBJ_BIRD] || s.ObjState[OBJ_BIRD] != BIRD_CAGED {
		t.Fatal("failed to cage the bird with cage and no rod")
	}
}

func TestSaveRestoreRoundTrip(t *testing.T) {
	s := New()
	// Play a handful of turns so the state is non-trivial.
	walkTo(t, s, LOC_BUILDING)
	Act(s, VerbTake, OBJ_LAMP)
	Act(s, VerbTake, OBJ_KEYS)
	Act(s, VerbLight, OBJ_LAMP)
	walkTo(t, s, LOC_START)
	walkTo(t, s, LOC_GRATE)
	Act(s, VerbUnlock, OBJ_GRATE)
	walkTo(t, s, LOC_BELOWGRATE)

	b, err := SaveBytes(s)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadBytes(b)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(s, got) {
		t.Fatalf("round-trip mismatch:\n saved = %+v\n loaded = %+v", s, got)
	}
}

func TestLoadRejectsBadSave(t *testing.T) {
	if _, err := LoadBytes([]byte{}); err != ErrBadSave {
		t.Fatalf("empty save err = %v, want ErrBadSave", err)
	}
	if _, err := LoadBytes([]byte{0xFF, 0x00, 0x01}); err != ErrBadSave {
		t.Fatalf("bad-version save err = %v, want ErrBadSave", err)
	}
	// A fresh, correctly-versioned state loads cleanly.
	b, _ := SaveBytes(New())
	if _, err := LoadBytes(b); err != nil {
		t.Fatalf("valid save rejected: %v", err)
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
