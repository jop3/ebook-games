package story

// The Notebook / deduction sub-system (spec §10b) — the single engine EXTENSION
// this themed story adds on top of the shared cave engine. It reuses the State
// fields that were designed in from the start (Clues []Clue, Deductions
// map[string]bool) and adds two small pure tables: which object yields which
// clue when examined, and which pair of clues combines into a deduction. A
// horror story would instead lean on State.Dread; the point is that a themed
// story is "shared engine + one small extension + authored data", not a rewrite.

// ClueFor maps an examinable object to the clue that examining it records.
var ClueFor = map[ObjID]Clue{
	OBJ_BODY:      {ID: "body", Text: "You examine the body. The face is fixed in a rictus of terror, yet there is no wound and no blood — only a faint bitterness, like almonds, about the lips."},
	OBJ_RING:      {ID: "ring", Text: "The scratch is fresh, the width of a tumbler's base: a second glass stood here recently, and was carried away."},
	OBJ_CLOCK:     {ID: "clock", Text: "The clock is stopped, and set a careless two hours fast. Someone wished the hour of death to be misread."},
	OBJ_LETTER:    {ID: "letter", Text: "Only a charred triangle escaped the flame. One word survives in a clerk's careful hand: '…ledger'."},
	OBJ_BOOTPRINT: {ID: "boot", Text: "A single clear print in the coal dust — a heavy workman's boot, far too large for the slight dead man. Someone came in from the yard."},
	OBJ_LEDGER:    {ID: "ledger", Text: "The chemist's day-ledger records a recent sale of a rare tincture — bitter, and deadly in any quantity — paid in cash, signed with a single initial."},
}

// Deduction is a conclusion drawn from combining two clues in the Notebook.
type Deduction struct {
	ID   string
	Text string
}

// combines is the deduction table, keyed by an unordered pair of clue ids.
var combines = map[[2]string]Deduction{
	pairKey("boot", "ring"):     {ID: "entry", Text: "Boot-print and glass-ring together: the victim admitted a heavy stranger by the yard door and shared a drink with him. This was no burglary — it was a meeting he agreed to."},
	pairKey("letter", "ledger"): {ID: "motive", Text: "The burned letter's 'ledger' and the chemist's ledger are one and the same — a debt, and a purchase of poison to wipe it clean. There is your motive."},
	pairKey("clock", "boot"):    {ID: "timing", Text: "A clock set two hours fast, and a caller who came unseen from the yard: the whole house will swear to an hour that never held the crime."},
}

// pairKey orders two clue ids so (a,b) and (b,a) hit the same entry.
func pairKey(a, b string) [2]string {
	if a <= b {
		return [2]string{a, b}
	}
	return [2]string{b, a}
}

// HasClue reports whether a clue with the given id is already recorded.
func HasClue(s *State, id string) bool {
	for _, c := range s.Clues {
		if c.ID == id {
			return true
		}
	}
	return false
}

// AddClueFor records the clue for examining obj, if any, once. It returns the
// clue and whether it was newly added (so the UI can announce "noted").
func AddClueFor(s *State, obj ObjID) (Clue, bool) {
	c, ok := ClueFor[obj]
	if !ok || HasClue(s, c.ID) {
		return Clue{}, false
	}
	s.Clues = append(s.Clues, c)
	return c, true
}

// Combine attempts to connect the clues at indices i and j in s.Clues. On a
// valid pair it records the deduction (idempotently) and returns it; otherwise
// ok is false.
func Combine(s *State, i, j int) (Deduction, bool) {
	if i < 0 || j < 0 || i == j || i >= len(s.Clues) || j >= len(s.Clues) {
		return Deduction{}, false
	}
	d, ok := combines[pairKey(s.Clues[i].ID, s.Clues[j].ID)]
	if !ok {
		return Deduction{}, false
	}
	s.Deductions[d.ID] = true
	return d, true
}

// DeductionCount returns how many distinct deductions the player has made.
func DeductionCount(s *State) int { return len(s.Deductions) }

// TotalDeductions is how many deductions the case contains (for progress).
func TotalDeductions() int {
	seen := map[string]bool{}
	for _, d := range combines {
		seen[d.ID] = true
	}
	return len(seen)
}
