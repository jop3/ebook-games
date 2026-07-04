package story

// The Notebook / deduction / accusation sub-system (spec §10b, §10d) — the
// engine EXTENSION this themed story adds on top of the shared cave engine. It
// reuses the State fields designed in from the start (Clues, Deductions) and
// adds pure tables: which object yields which clue, which pair of clues combines
// into a deduction, and how an accusation is judged. No engine change.

// ClueFor maps an examinable object (physical evidence or an interview topic) to
// the clue that examining it records.
var ClueFor = map[ObjID]Clue{
	// Physical evidence.
	OBJ_BODY:   {ID: "body", Text: "You examine the body. The face is fixed in a rictus of terror, yet there is no wound and no blood — only a faint bitterness, like almonds, about the lips."},
	OBJ_RING:   {ID: "ring", Text: "The scratch is fresh, the width of a tumbler's base: a second glass stood here recently, and was carried away."},
	OBJ_CLOCK:  {ID: "clock", Text: "The mantel clock is stopped, and set a careless two hours fast. Someone wished the hour of death to be misread."},
	OBJ_LETTER: {ID: "letter", Text: "Only a charred triangle escaped the flame. One word survives in a clerk's careful hand: '…ledger'."},
	OBJ_BOOT:   {ID: "boot", Text: "A single clear print in the coal dust — a heavy workman's boot, far too large for the slight dead man. Someone came in from the yard."},
	OBJ_LATCH:  {ID: "latch", Text: "The bedroom window latch has been forced from OUTSIDE: the scratches bite inward. This, not the bolted study door, was the way in."},
	// Testimony (interview topics).
	OBJ_VISITOR:  {ID: "visitor", Text: "Mrs. Hudd: 'The study door was bolted on the inside, sir, I'd swear my life on it. But I did hear a heavy tread on the back stair, gone midnight, and the scrape of the yard door.'"},
	OBJ_CABTIME:  {ID: "cabtime", Text: "Feaney: 'Set a stout gent down at the yard corner — not the front door, mind — near on midnight. Rum thing: the house clock was striking ten as I drove off.'"},
	OBJ_LEDGER:   {ID: "ledger", Text: "The chemist's ledger shows Whitlock deep in debt to this very shop — and, three days past, a sale of a bitter tincture, deadly in quantity, entered in Crole's own hand."},
	OBJ_TINCTURE: {ID: "tincture", Text: "The dark bottle reeks of bitter almond: the same scent that clung to the dead man's lips. Crole's thumb has smudged the fresh-poured ring on its base."},
}

// Deduction is a conclusion drawn from combining two clues in the Notebook.
type Deduction struct {
	ID   string
	Text string
}

// combines is the deduction table, keyed by an unordered pair of clue ids.
// Four deductions form the chain the accusation requires. The 'poison'
// deduction unlocks the chemist (see GatedExits) — its text says so.
var combines = map[[2]string]Deduction{
	pairKey("boot", "latch"):    {ID: "entry", Text: "Boot-print in the yard, and a bedroom latch forced from outside: the killer never touched the bolted study door. He came and went by the yard — the locked room is a fiction."},
	pairKey("body", "ring"):     {ID: "poison", Text: "No wound, the reek of bitter almond, and a second glass carried away: the victim was POISONED in a drink he shared willingly. Trace the poison to its source — the chemist's shop opposite is worth a call."},
	pairKey("clock", "cabtime"): {ID: "timing", Text: "The clock set two hours fast, and Feaney's fare set down at midnight while the house heard ten: the whole household will swear to an hour that never held the crime."},
	pairKey("letter", "ledger"): {ID: "motive", Text: "The burned letter's 'ledger' and the chemist's ledger are one: a crushing debt, and a poison bought to erase both it and the man who could tell of it. There is the motive — and the hand that signed."},
}

// requiredDeductions are the conclusions a sound accusation must rest on.
var requiredDeductions = []string{"entry", "poison", "timing", "motive"}

// deductionLabelSv gives a Swedish label for naming an unsupported deduction.
var deductionLabelSv = map[string]string{
	"entry":  "hur mördaren tog sig in",
	"poison": "att det var gift",
	"timing": "den verkliga tiden för dådet",
	"motive": "motivet",
}

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

// AddClueFor records the clue for examining obj, if any, once.
func AddClueFor(s *State, obj ObjID) (Clue, bool) {
	c, ok := ClueFor[obj]
	if !ok || HasClue(s, c.ID) {
		return Clue{}, false
	}
	s.Clues = append(s.Clues, c)
	return c, true
}

// Combine connects the clues at indices i and j; on a valid pair it records the
// deduction (idempotently) and returns it.
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

func DeductionCount(s *State) int { return len(s.Deductions) }

func TotalDeductions() int { return len(requiredDeductions) }

// --- accusation endgame (spec §10d) -----------------------------------------

// Choice is one selectable option on the accusation screen.
type Choice struct {
	ID    string
	Label string
}

// The three questions of the accusation, and their options. Exactly one of each
// is correct; the wrong ones are plausible and drawn from the case's red herrings.
var (
	Culprits = []Choice{
		{"crole", "Elias Crole, apotekaren"},
		{"hudd", "Fru Hudd, hyresvärdinnan"},
		{"feaney", "Tom Feaney, kusken"},
	}
	Methods = []Choice{
		{"poison", "Gift i en delad drink"},
		{"blow", "Ett slag mot huvudet"},
		{"strangle", "Strypning"},
	}
	Motives = []Choice{
		{"debt", "En skuld antecknad i liggaren"},
		{"jealousy", "Svartsjuka"},
		{"robbery", "Rån"},
	}
)

const (
	correctCulprit = "crole"
	correctMethod  = "poison"
	correctMotive  = "debt"
)

// resolutionText is the closing scene printed on a correct, supported accusation.
const resolutionText = "Crole does not deny it long. Confronted with his own ledger, the forced latch, and the bottle that still smells of almonds, the big man's composure cracks like ice. He came by the yard while the house slept, poured the tincture into a friendly glass, set the clock to muddy the hour, and let himself out the way he came — trusting the bolted door to swear no one had entered. The debt died with its keeper; but the account is settled now. Case closed."

// Verdict is the outcome of an accusation.
type Verdict struct {
	Win  bool
	Text string
}

// Accuse judges an accusation. A correct, fully-supported charge wins and sets
// State.Won. Otherwise it names what is wrong — an unsupported deduction, or the
// pillar the evidence does not bear out — and the player may keep investigating.
func Accuse(s *State, culprit, method, motive string) Verdict {
	// A wrong pillar is refused outright, whatever the deductions.
	switch {
	case culprit != correctCulprit:
		return Verdict{false, "The evidence does not point to that person as the murderer. Look again at who had the means and the motive."}
	case method != correctMethod:
		return Verdict{false, "That is not how this man died. The body itself tells you otherwise."}
	case motive != correctMotive:
		return Verdict{false, "That motive is not borne out by what you have found. What was worth a man's life here?"}
	}
	// Right charge — but it must rest on the deductions, not a lucky guess.
	for _, d := range requiredDeductions {
		if !s.Deductions[d] {
			return Verdict{false, "You may be right — but you have not yet established " + deductionLabelSv[d] + ". A charge you cannot prove will not hold. Keep investigating."}
		}
	}
	s.Won = true
	return Verdict{true, resolutionText}
}
