// Package story is the ink-free heart of Grottan, a tap-driven port of Colossal
// Cave Adventure for the PocketBook Verse Pro. It holds the data model, the
// mutable game State, and the pure functions that drive play (New, Describe,
// Move, Act, save/restore). It imports nothing from the SDK, so the whole engine
// unit-tests cgo-free — mirroring how the puzzle games keep their logic pure.
//
// The world data itself is generated from Open Adventure's adventure.yaml into
// storydata_gen.go (see scratchpad/advgen and SPEC_TEXT_ADVENTURE.md §6).
package story

// Identifier types kept small and value-typed so the generated data stays terse
// and the engine reads clearly.
type (
	LocID  int // index into Locations
	ObjID  int // index into Objects
	Motion int // a movement / magic word (index into Open Adventure's motions)
)

// CondKind classifies a travel rule's guard.
type CondKind int

const (
	CondNone      CondKind = iota // unconditional
	CondPct                       // Pct% random chance (Phase 3; unused in Phase 1)
	CondCarry                     // player carries Obj
	CondWith                      // Obj is here (carried or in the room)
	CondNotState                  // Obj is NOT in State
	CondNoDwarves                 // Phase 3; treated as always-true here
)

// Cond is a travel rule's guard.
type Cond struct {
	Kind  CondKind
	Obj   ObjID
	State int
	Pct   int
}

// ActKind classifies what a travel rule does when it fires.
type ActKind int

const (
	ActGoto    ActKind = iota // move to Dest
	ActSpecial                // hand-coded routine #Sp (Phase 3)
	ActMessage                // print Messages[Msg], don't move
)

// Travel is one movement rule of a location.
type Travel struct {
	Verbs []Motion // motions that trigger this rule
	Dir   Motion   // representative direction for a goto (the exit-button label); 0 otherwise
	Cond  Cond
	Act   ActKind
	Dest  LocID // for ActGoto
	Msg   int   // index into Messages, for ActMessage
	Sp    int   // special routine number (Phase 3)
}

// Location is one room.
type Location struct {
	Long, Short string
	Lit         bool // LIT flag: lit even without a lamp
	Forest      bool
	Deep        bool
	Travel      []Travel
}

// Object is one thing in the world.
type Object struct {
	Words        []string // vocabulary synonyms
	Inventory    string   // label when carried / listed
	States       []string // e.g. ["LAMP_DARK","LAMP_BRIGHT"]; empty = stateless
	Descriptions []string // per-state room description ("" = invisible in that state)
	Start        LocID    // initial location (LOC_NOWHERE if none)
	Start2       LocID    // some objects occupy two places (e.g. the grate)
	Immovable    bool
	Treasure     bool
}

// Verb is a tap-UI action verb.
type Verb int

const (
	VerbLook Verb = iota
	VerbInventory
	VerbExamine
	VerbTake
	VerbDrop
	VerbOpen
	VerbClose
	VerbLock
	VerbUnlock
	VerbLight
	VerbExtinguish
)

// Clue is a Notebook entry for the later mystery theme (spec §10b). Unused in
// Phase 1 but designed into State now so themed stories drop in as data.
type Clue struct {
	ID   string
	Text string
}

// State is the entire mutable game — and the save payload (§5). Every field is
// exported so encoding/gob can round-trip it.
type State struct {
	Loc, OldLoc LocID
	Visited     map[LocID]bool
	Carried     map[ObjID]bool
	ObjAt       map[ObjID]LocID // current location of each movable object
	ObjState    map[ObjID]int   // current state index per object
	Known       map[string]bool // discovered magic words / story flags
	Turns       int
	LampLife    int // Phase 2: battery countdown (seeded large in Phase 1)
	Score       int
	Dead        bool
	Won         bool

	// Reserved for later themed stories (spec §10b/§10c) — present now so adding
	// them is data, not a migration. Untouched by Phase 1 play.
	Clues      []Clue
	Deductions map[string]bool
	Dread      int
}

// initialLampLife is a generous battery so Phase 1 never times out (the limit
// becomes a real mechanic in Phase 2).
const initialLampLife = 100000

// New returns a fresh game: objects at their start locations, the lamp dark, the
// grate locked, and the player at the road before the well house.
func New() *State {
	s := &State{
		Loc:        LOC_START,
		OldLoc:     LOC_START,
		Visited:    map[LocID]bool{},
		Carried:    map[ObjID]bool{},
		ObjAt:      map[ObjID]LocID{},
		ObjState:   map[ObjID]int{},
		Known:      map[string]bool{},
		Deductions: map[string]bool{},
		LampLife:   initialLampLife,
	}
	for id := ObjID(1); int(id) < len(Objects); id++ {
		o := Objects[id]
		if o.Start != LOC_NOWHERE {
			s.ObjAt[id] = o.Start
		} else {
			s.ObjAt[id] = LOC_NOWHERE
		}
		s.ObjState[id] = 0 // default state (lamp dark, grate closed, bird uncaged)
	}
	return s
}
