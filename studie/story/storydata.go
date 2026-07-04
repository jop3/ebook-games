package story

// Hand-authored world data for "En Studie i Grått" — an ORIGINAL locked-room
// mystery inspired by the Sherlock Holmes debut (a London-fog death), written
// fresh (no transcribed text). Unlike the cave (whose data is generated from
// Open Adventure's YAML), a small themed story is authored directly as Go
// literals. The engine (model.go / engine.go / save.go) is byte-for-byte the
// SAME source as grottan — this file is the "new data" half of the reuse thesis;
// notebook.go is the "one extension" half.
//
// NOTE (authoring): narration is in English for this scaffold to match the
// established repo pattern; a Swedish narration pass is a future authoring
// choice. UI chrome is Swedish throughout.

// Location ids. LOC_START (the consulting-rooms hub) is required by the shared
// engine's New(); the rest are this story's.
const (
	LOC_NOWHERE LocID = 0
	LOC_START   LocID = 1 // your consulting-rooms (the hub you return to)
	LOC_STREET  LocID = 2
	LOC_HALL    LocID = 3
	LOC_STUDY   LocID = 4
	LOC_BEDROOM LocID = 5
	LOC_YARD    LocID = 6
)

// Object ids.
const (
	OBJ_NONE      ObjID = 0
	OBJ_BODY      ObjID = 1
	OBJ_RING      ObjID = 2
	OBJ_CLOCK     ObjID = 3
	OBJ_LETTER    ObjID = 4
	OBJ_BOOTPRINT ObjID = 5
	OBJ_LEDGER    ObjID = 6
)

// Motions (exit directions) and their Swedish button labels.
const (
	MotHome Motion = iota + 1
	MotStreet
	MotIn
	MotOut
	MotStudy
	MotHall
	MotBedroom
	MotYard
)

var MotionSwedish = map[Motion]string{
	MotHome:    "Hem",
	MotStreet:  "Gatan",
	MotIn:      "In",
	MotOut:     "Ut",
	MotStudy:   "Arbetsrummet",
	MotHall:    "Hallen",
	MotBedroom: "Sovrummet",
	MotYard:    "Bakgården",
}

// --- inert cave sentinels ----------------------------------------------------
//
// The shared engine.go references a handful of cave-only symbols (the lamp, the
// grate, the bird puzzle, the XYZZY magic word). This story has none of them, so
// they are defined here as harmless placeholders: the negative object ids never
// match a real noun, every room here is Lit so the darkness/lamp path is never
// taken, and no magic word is ever spoken. (A future refactor could lift these
// hooks out of the engine into per-story data; for now the engine compiles and
// runs unchanged, which is the point.)
const (
	OBJ_LAMP  ObjID = -2
	OBJ_GRATE ObjID = -3
	OBJ_KEYS  ObjID = -4
	OBJ_BIRD  ObjID = -5
	OBJ_CAGE  ObjID = -6
	OBJ_ROD   ObjID = -7

	LAMP_DARK    = 0
	LAMP_BRIGHT  = 1
	GRATE_CLOSED = 0
	GRATE_OPEN   = 1
	BIRD_UNCAGED = 0
	BIRD_CAGED   = 1

	LOC_DEBRIS   LocID  = -1
	LOC_BUILDING LocID  = -8 // referenced by the engine's XYZZY teleport (never fires here)
	MotXYZZY     Motion = -1
	MotPLUGH     Motion = -2
)

var (
	MagicWords    []Motion
	MagicWordText = map[Motion]string{}
)

// --- the world ---------------------------------------------------------------

var Locations = []Location{
	{}, // LOC_NOWHERE
	{ // LOC_START
		Long:  "Your own consulting-rooms: a low fire, two worn armchairs, and the litter of a dozen unfinished experiments. It is a place to think — and to lay out, in the notebook, what the fog has yielded.",
		Short: "Your consulting-rooms.",
		Lit:   true,
		Travel: []Travel{
			{Verbs: []Motion{MotStreet}, Dir: MotStreet, Act: ActGoto, Dest: LOC_STREET},
		},
	},
	{ // LOC_STREET
		Long:  "A mean street off the Euston Road, drowned in yellow fog. A hansom waits at the kerb, its lamps two smears of light. The lodging-house door stands ajar; a chemist's shop glows dimly opposite.",
		Short: "The foggy street.",
		Lit:   true,
		Travel: []Travel{
			{Verbs: []Motion{MotIn}, Dir: MotIn, Act: ActGoto, Dest: LOC_HALL},
			{Verbs: []Motion{MotHome}, Dir: MotHome, Act: ActGoto, Dest: LOC_START},
		},
	},
	{ // LOC_HALL
		Long:  "The lodging-house hall, narrow and gaslit. A cold grate holds the ashes of a hasty fire. A door gives onto the study; a back stair descends toward the yard.",
		Short: "The lodging-house hall.",
		Lit:   true,
		Travel: []Travel{
			{Verbs: []Motion{MotStudy}, Dir: MotStudy, Act: ActGoto, Dest: LOC_STUDY},
			{Verbs: []Motion{MotYard}, Dir: MotYard, Act: ActGoto, Dest: LOC_YARD},
			{Verbs: []Motion{MotOut}, Dir: MotOut, Act: ActGoto, Dest: LOC_STREET},
		},
	},
	{ // LOC_STUDY
		Long:  "The dead man's study. The papers sit squared on the desk; the air is stale and faintly sweet. Everything is in perfect order, which is itself the most disquieting thing in the room.",
		Short: "The victim's study.",
		Lit:   true,
		Travel: []Travel{
			{Verbs: []Motion{MotHall}, Dir: MotHall, Act: ActGoto, Dest: LOC_HALL},
			{Verbs: []Motion{MotBedroom}, Dir: MotBedroom, Act: ActGoto, Dest: LOC_BEDROOM},
		},
	},
	{ // LOC_BEDROOM
		Long:  "A cramped bedroom. The bed is made and unslept-in: whatever the victim feared, he did not lie down to wait for it. A window gives onto the yard, its latch newly forced.",
		Short: "The bedroom.",
		Lit:   true,
		Travel: []Travel{
			{Verbs: []Motion{MotStudy}, Dir: MotStudy, Act: ActGoto, Dest: LOC_STUDY},
		},
	},
	{ // LOC_YARD
		Long:  "The back yard: a coal-store, a water-butt, and the reeking fog pooled between high walls. It is the one way in that need not pass the landlady's parlour.",
		Short: "The back yard.",
		Lit:   true,
		Travel: []Travel{
			{Verbs: []Motion{MotHall}, Dir: MotHall, Act: ActGoto, Dest: LOC_HALL},
		},
	},
}

var Objects = []Object{
	{}, // OBJ_NONE
	{ // OBJ_BODY
		Words:        []string{"body", "man"},
		Inventory:    "the body",
		Descriptions: []string{"The dead man lies slumped across the desk."},
		Start:        LOC_STUDY,
		Immovable:    true,
	},
	{ // OBJ_RING
		Words:        []string{"ring", "mark"},
		Inventory:    "the scratched ring",
		Descriptions: []string{"A faint ring is scratched into the desk's varnish."},
		Start:        LOC_STUDY,
		Immovable:    true,
	},
	{ // OBJ_CLOCK
		Words:        []string{"clock"},
		Inventory:    "the mantel clock",
		Descriptions: []string{"A mantel clock stands stopped on the shelf."},
		Start:        LOC_STUDY,
		Immovable:    true,
	},
	{ // OBJ_LETTER
		Words:        []string{"letter", "scrap", "paper"},
		Inventory:    "the charred scrap",
		Descriptions: []string{"A charred scrap of notepaper survives in the cold grate."},
		Start:        LOC_HALL,
	},
	{ // OBJ_BOOTPRINT
		Words:        []string{"print", "boot"},
		Inventory:    "the boot-print",
		Descriptions: []string{"A boot-print is pressed into the coal dust."},
		Start:        LOC_YARD,
		Immovable:    true,
	},
	{ // OBJ_LEDGER
		Words:        []string{"ledger", "book"},
		Inventory:    "the chemist's ledger",
		Descriptions: []string{"Through the chemist's window, a day-ledger lies open on the counter."},
		Start:        LOC_STREET,
		Immovable:    true,
	},
}

// Messages is unused by this story (no blocked-travel or speak actions yet) but
// the shared engine expects the symbol to exist.
var Messages = []string{}
