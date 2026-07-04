package story

// The pure engine: presence/darkness queries, room description, movement, and
// the verb table. Everything here is deterministic and SDK-free, so the play
// harness and unit tests exercise the whole rule set without a device.

// --- stock narration (English, matching Colossal Cave's own voice) -----------
//
// Phase 1 ships ENGLISH narration with SWEDISH UI chrome (spec §4), so these
// in-transcript responses stay in the cave's original voice; only the buttons,
// menus, headers, and rules screen are Swedish.
const (
	msgPitchDark = "It is now pitch dark. If you proceed you will likely fall into a pit."
	msgNoWay     = "You can't go that way."
	msgNothing   = "Nothing happens."
	msgTaken     = "Taken."
	msgDropped   = "Dropped."
	msgHaveIt    = "You already have it."
	msgNotHere   = "I see no such thing here."
	msgNotCarry  = "You aren't carrying it."
	msgCantCarry = "You can't carry that."
	msgNothingSp = "You see nothing special."
	msgUnknown   = "Nothing happens."
)

// --- queries -----------------------------------------------------------------

// objAt reports the location an object currently occupies (movable objects).
func objAt(s *State, id ObjID) LocID {
	if l, ok := s.ObjAt[id]; ok {
		return l
	}
	return LOC_NOWHERE
}

// Present reports whether object id is reachable from the player's position:
// carried, lying in the current room, or an immovable fixture of the room.
func Present(s *State, id ObjID) bool {
	if s.Carried[id] {
		return true
	}
	o := Objects[id]
	if o.Immovable {
		return o.Start == s.Loc || o.Start2 == s.Loc
	}
	return objAt(s, id) == s.Loc
}

// inRoom reports whether object id is visible in the current room (present but
// not carried) with a non-empty description for its current state.
func inRoom(s *State, id ObjID) bool {
	if s.Carried[id] {
		return false
	}
	o := Objects[id]
	if !(o.Immovable && (o.Start == s.Loc || o.Start2 == s.Loc)) && objAt(s, id) != s.Loc {
		return false
	}
	return objDesc(s, id) != ""
}

// objDesc returns the room description for an object's current state.
func objDesc(s *State, id ObjID) string {
	o := Objects[id]
	st := s.ObjState[id]
	if st >= 0 && st < len(o.Descriptions) {
		return o.Descriptions[st]
	}
	if len(o.Descriptions) > 0 {
		return o.Descriptions[0]
	}
	return ""
}

// lampLit reports whether a lit lamp is illuminating the player's position.
func lampLit(s *State, id ObjID) bool {
	return Present(s, id) && s.ObjState[OBJ_LAMP] == LAMP_BRIGHT
}

// IsDark reports whether the player cannot see: the room is not naturally lit
// and no lit lamp is present.
func IsDark(s *State) bool {
	if Locations[s.Loc].Lit {
		return false
	}
	return !lampLit(s, OBJ_LAMP)
}

// RoomName is the short header label for the current room (Swedish "Mörker" when
// the player is in the dark).
func RoomName(s *State) string {
	if IsDark(s) {
		return "Mörker"
	}
	return Locations[s.Loc].Short
}

// VisibleObjects lists the objects lying in the current room (not carried) that
// the player can see — these become the "HÄR" noun buttons. Empty in the dark.
func VisibleObjects(s *State) []ObjID {
	if IsDark(s) {
		return nil
	}
	var out []ObjID
	for id := ObjID(1); int(id) < len(Objects); id++ {
		if inRoom(s, id) {
			out = append(out, id)
		}
	}
	return out
}

// CarriedObjects lists the objects the player is carrying (the "Ryggsäck").
func CarriedObjects(s *State) []ObjID {
	var out []ObjID
	for id := ObjID(1); int(id) < len(Objects); id++ {
		if s.Carried[id] {
			out = append(out, id)
		}
	}
	return out
}

// Describe returns the room narration: the long text on first sight (short
// thereafter), then a line per visible object, or the pitch-dark warning.
func Describe(s *State) []string {
	return describe(s, false)
}

func describe(s *State, forceLong bool) []string {
	if IsDark(s) {
		return []string{msgPitchDark}
	}
	loc := Locations[s.Loc]
	var out []string
	if forceLong || !s.Visited[s.Loc] || loc.Short == "" {
		out = append(out, loc.Long)
	} else {
		out = append(out, loc.Short)
	}
	for _, id := range VisibleObjects(s) {
		out = append(out, objDesc(s, id))
	}
	return out
}

// --- exits -------------------------------------------------------------------

// Exit is one tappable way out of a room.
type Exit struct {
	Motion Motion
	Label  string
	Dest   LocID
}

// isMagic reports whether a motion is a hidden magic word (never auto-shown as
// an exit; surfaced only through the Säg… verb once discovered).
func isMagic(m Motion) bool {
	for _, mw := range MagicWords {
		if m == mw {
			return true
		}
	}
	return m == MotXYZZY || m == MotPLUGH
}

// Exits evaluates the current room's travel rules and returns one button per
// distinct direction whose goto is currently reachable. Magic-word motions are
// excluded — they are puzzle knowledge, not menu options.
func Exits(s *State) []Exit {
	// Exits are surfaced even in the dark: this is a tap-only device, so the
	// player must still be able to feel their way out of an unlit room.
	var out []Exit
	seen := map[Motion]bool{}
	for _, t := range Locations[s.Loc].Travel {
		if t.Act != ActGoto || t.Dir == 0 || isMagic(t.Dir) {
			continue
		}
		if !condPasses(s, t.Cond) {
			continue
		}
		if seen[t.Dir] {
			continue
		}
		seen[t.Dir] = true
		out = append(out, Exit{Motion: t.Dir, Label: MotionSwedish[t.Dir], Dest: t.Dest})
	}
	return out
}

// KnownMagicWords returns the discovered magic words to offer in the Säg… menu.
func KnownMagicWords(s *State) []Motion {
	var out []Motion
	for _, m := range MagicWords {
		if s.Known[MagicWordText[m]] {
			out = append(out, m)
		}
	}
	return out
}

// --- movement ----------------------------------------------------------------

func condPasses(s *State, c Cond) bool {
	switch c.Kind {
	case CondNone:
		return true
	case CondNotState:
		return s.ObjState[c.Obj] != c.State
	case CondCarry:
		return s.Carried[c.Obj]
	case CondWith:
		return Present(s, c.Obj)
	case CondPct:
		// Deterministic pseudo-random from the turn counter (Phase 3 territory;
		// no pct conds exist in the Phase 1 subset). Kept replayable per §3.4.
		return int(hash(uint64(s.Turns))%100) < c.Pct
	case CondNoDwarves:
		return true
	}
	return true
}

func hash(x uint64) uint64 {
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	return x
}

// enter moves the player to dst, marking the old room visited and discovering
// the XYZZY scrawl on first arrival in the debris room.
func (s *State) enter(dst LocID) {
	s.Visited[s.Loc] = true
	s.OldLoc = s.Loc
	s.Loc = dst
	if dst == LOC_DEBRIS {
		s.Known[MagicWordText[MotXYZZY]] = true // "MAGIC WORD XYZZY" is scrawled here
	}
}

// Move attempts motion m from the current room and returns the resulting
// narration. Magic words teleport; ordinary motions consult the travel table.
func Move(s *State, m Motion) []string {
	s.Turns++
	if s.LampLife > 0 {
		s.LampLife--
	}

	// XYZZY is the one working magic word in Phase 1: it shuttles between the
	// well house and the debris room, exactly as in the classic cave.
	if m == MotXYZZY {
		if !s.Known[MagicWordText[MotXYZZY]] {
			return []string{msgNothing}
		}
		switch s.Loc {
		case LOC_BUILDING:
			s.enter(LOC_DEBRIS)
			return Describe(s)
		case LOC_DEBRIS:
			s.enter(LOC_BUILDING)
			return Describe(s)
		default:
			return []string{msgNothing}
		}
	}

	for _, t := range Locations[s.Loc].Travel {
		if !motionIn(t.Verbs, m) || !condPasses(s, t.Cond) {
			continue
		}
		switch t.Act {
		case ActGoto:
			s.enter(t.Dest)
			return Describe(s)
		case ActMessage:
			return []string{Messages[t.Msg]}
		case ActSpecial:
			return []string{msgNothing} // Phase 3 routines are not ported yet
		}
	}
	return []string{msgNoWay}
}

func motionIn(list []Motion, m Motion) bool {
	for _, x := range list {
		if x == m {
			return true
		}
	}
	return false
}

// --- verbs -------------------------------------------------------------------

// NeedsNoun reports whether a verb executes immediately (LOOK, INVENTORY) or
// waits for a noun (everything else).
func NeedsNoun(v Verb) bool {
	return v != VerbLook && v != VerbInventory
}

// Act runs verb v on object n (n is ignored for LOOK/INVENTORY) and returns the
// narration. Unrecognised combinations yield a neutral "Nothing happens."
func Act(s *State, v Verb, n ObjID) []string {
	s.Turns++
	switch v {
	case VerbLook:
		return describe(s, true)
	case VerbInventory:
		return inventory(s)
	}

	if n == OBJ_NONE {
		return []string{msgUnknown}
	}
	if !Present(s, n) {
		return []string{msgNotHere}
	}

	switch v {
	case VerbExamine:
		return examine(s, n)
	case VerbTake:
		return take(s, n)
	case VerbDrop:
		return drop(s, n)
	case VerbOpen, VerbUnlock:
		return unlockGrate(s, n)
	case VerbClose, VerbLock:
		return lockGrate(s, n)
	case VerbLight:
		return light(s, n)
	case VerbExtinguish:
		return extinguish(s, n)
	}
	return []string{msgUnknown}
}

func inventory(s *State) []string {
	c := CarriedObjects(s)
	if len(c) == 0 {
		return []string{"You're not carrying anything."}
	}
	out := []string{"You are currently holding the following:"}
	for _, id := range c {
		out = append(out, "  "+Objects[id].Inventory)
	}
	return out
}

func examine(s *State, n ObjID) []string {
	if d := objDesc(s, n); d != "" {
		return []string{d}
	}
	return []string{msgNothingSp}
}

func take(s *State, n ObjID) []string {
	if s.Carried[n] {
		return []string{msgHaveIt}
	}
	if Objects[n].Immovable {
		return []string{msgCantCarry}
	}
	if n == OBJ_BIRD {
		return takeBird(s)
	}
	s.Carried[n] = true
	s.ObjAt[n] = LOC_NOWHERE
	return []string{msgTaken}
}

// takeBird reproduces the classic cage/rod puzzle: you need the wicker cage to
// hold the bird, and the black rod frightens it off.
func takeBird(s *State) []string {
	if !s.Carried[OBJ_CAGE] {
		return []string{"You can catch the bird, but you cannot hold it."}
	}
	if s.Carried[OBJ_ROD] {
		return []string{"The bird was frightened by the rod and refuses to be caught."}
	}
	s.Carried[OBJ_BIRD] = true
	s.ObjAt[OBJ_BIRD] = LOC_NOWHERE
	s.ObjState[OBJ_BIRD] = BIRD_CAGED
	return []string{"You catch the bird in the wicker cage."}
}

func drop(s *State, n ObjID) []string {
	if !s.Carried[n] {
		return []string{msgNotCarry}
	}
	s.Carried[n] = false
	s.ObjAt[n] = s.Loc
	if n == OBJ_BIRD {
		s.ObjState[OBJ_BIRD] = BIRD_UNCAGED // it hops free of the cage
	}
	return []string{msgDropped}
}

// unlockGrate handles OPEN/UNLOCK. The only lockable thing in the Phase 1 world
// is the steel grate, which needs the keys.
func unlockGrate(s *State, n ObjID) []string {
	if n != OBJ_GRATE {
		return []string{"You can't open that."}
	}
	if s.ObjState[OBJ_GRATE] == GRATE_OPEN {
		return []string{"The grate is already unlocked."}
	}
	if !s.Carried[OBJ_KEYS] {
		return []string{"You have no keys!"}
	}
	s.ObjState[OBJ_GRATE] = GRATE_OPEN
	return []string{"The grate is now unlocked."}
}

func lockGrate(s *State, n ObjID) []string {
	if n != OBJ_GRATE {
		return []string{"You can't close that."}
	}
	if s.ObjState[OBJ_GRATE] == GRATE_CLOSED {
		return []string{"The grate is already locked."}
	}
	if !s.Carried[OBJ_KEYS] {
		return []string{"You have no keys!"}
	}
	s.ObjState[OBJ_GRATE] = GRATE_CLOSED
	return []string{"The grate is now locked."}
}

func light(s *State, n ObjID) []string {
	if n != OBJ_LAMP {
		return []string{"You can't light that."}
	}
	if s.ObjState[OBJ_LAMP] == LAMP_BRIGHT {
		return []string{"Your lamp is already on."}
	}
	s.ObjState[OBJ_LAMP] = LAMP_BRIGHT
	return []string{"Your lamp is now on."}
}

func extinguish(s *State, n ObjID) []string {
	if n != OBJ_LAMP {
		return []string{"You can't do that."}
	}
	if s.ObjState[OBJ_LAMP] == LAMP_DARK {
		return []string{"Your lamp is already off."}
	}
	s.ObjState[OBJ_LAMP] = LAMP_DARK
	return []string{"Your lamp is now off."}
}
