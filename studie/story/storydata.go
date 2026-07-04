package story

// Hand-authored world data for "En Studie i Grått" — an ORIGINAL London-fog
// locked-room mystery inspired by the Sherlock Holmes debut, written fresh (no
// transcribed text). It is a COMPLETE, winnable case: a crime scene, three
// interviews, a gated lead, a four-step deduction chain, and an accusation.
//
// The engine (model.go / engine.go / save.go) is byte-for-byte the SAME source
// as grottan. This file is the authored world; notebook.go is the Notebook /
// deduction / accusation extension. Interview characters are modelled as rooms
// and their topics as examinable "objects" — no new engine mechanic needed.
//
// NOTE (authoring): narration is English for now (matching the repo pattern); a
// Swedish narration pass is a future choice. UI chrome is Swedish throughout.

// Location ids. LOC_START (the consulting-rooms hub) is required by the shared
// engine's New().
const (
	LOC_NOWHERE LocID = 0
	LOC_START   LocID = 1 // your consulting-rooms (hub; where you make the accusation)
	LOC_STREET  LocID = 2 // the foggy street (spoke to the interviews)
	LOC_HALL    LocID = 3 // lodging-house hall — the landlady, the burned letter
	LOC_STUDY   LocID = 4 // the victim's study — body, glass-ring, clock
	LOC_BEDROOM LocID = 5 // the forced window latch
	LOC_YARD    LocID = 6 // the coal yard — the boot-print
	LOC_CHEMIST LocID = 7 // the chemist's shop — ledger + tincture (gated by 'poison')
	LOC_CABMAN  LocID = 8 // the hansom cabman, Tom Feaney
)

// Object ids. Physical clues + interview "topics" (immovable objects whose
// examination yields a testimony clue).
const (
	OBJ_NONE     ObjID = 0
	OBJ_BODY     ObjID = 1
	OBJ_RING     ObjID = 2
	OBJ_CLOCK    ObjID = 3
	OBJ_LETTER   ObjID = 4
	OBJ_BOOT     ObjID = 5
	OBJ_LATCH    ObjID = 6
	OBJ_VISITOR  ObjID = 7  // landlady's testimony
	OBJ_CABTIME  ObjID = 8  // cabman's testimony
	OBJ_LEDGER   ObjID = 9  // chemist's ledger
	OBJ_TINCTURE ObjID = 10 // chemist's poison bottle
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
	MotCab
	MotChemist
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
	MotCab:     "Kusken",
	MotChemist: "Apoteket",
}

// --- inert cave sentinels ----------------------------------------------------
//
// The shared engine.go references cave-only symbols (lamp, grate, bird, XYZZY).
// This story has none, so they are harmless placeholders: the negative object
// ids never match a real noun, every room is Lit so the darkness/lamp path is
// never taken, and no magic word is ever spoken.
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
	LOC_BUILDING LocID  = -8
	MotXYZZY     Motion = -1
	MotPLUGH     Motion = -2
)

var (
	MagicWords    []Motion
	MagicWordText = map[Motion]string{}
)

// GatedExits maps a destination room to the deduction that must be made before
// its entrance is offered. (Enforced in the UI, which filters Exits(); the
// engine stays unchanged.) Tracing the poison to the chemist unlocks his shop.
var GatedExits = map[LocID]string{
	LOC_CHEMIST: "poison",
}

// --- the world ---------------------------------------------------------------

var Locations = []Location{
	{}, // LOC_NOWHERE
	{ // LOC_START — the hub
		Long:  "Your own consulting-rooms: a low fire, two worn armchairs, and the litter of a dozen experiments. When the notebook is full enough, it is here, by the fire, that a case is finally named. Open the Blocket to accuse.",
		Short: "Your consulting-rooms.",
		Lit:   true,
		Travel: []Travel{
			{Verbs: []Motion{MotStreet}, Dir: MotStreet, Act: ActGoto, Dest: LOC_STREET},
		},
	},
	{ // LOC_STREET — the spoke
		Long:  "A mean street off the Euston Road, drowned in yellow fog. A hansom waits at the kerb; its driver stamps against the cold. The lodging-house door stands ajar, and a chemist's shop glows dimly opposite.",
		Short: "The foggy street.",
		Lit:   true,
		Travel: []Travel{
			{Verbs: []Motion{MotIn}, Dir: MotIn, Act: ActGoto, Dest: LOC_HALL},
			{Verbs: []Motion{MotCab}, Dir: MotCab, Act: ActGoto, Dest: LOC_CABMAN},
			{Verbs: []Motion{MotChemist}, Dir: MotChemist, Act: ActGoto, Dest: LOC_CHEMIST},
			{Verbs: []Motion{MotHome}, Dir: MotHome, Act: ActGoto, Dest: LOC_START},
		},
	},
	{ // LOC_HALL — landlady + letter
		Long:  "The lodging-house hall, narrow and gaslit. A cold grate holds the ashes of a hasty fire. Mrs. Hudd, the landlady, hovers by the stair, wringing her apron. A door gives onto the study; a back stair descends toward the yard.",
		Short: "The lodging-house hall.",
		Lit:   true,
		Travel: []Travel{
			{Verbs: []Motion{MotStudy}, Dir: MotStudy, Act: ActGoto, Dest: LOC_STUDY},
			{Verbs: []Motion{MotYard}, Dir: MotYard, Act: ActGoto, Dest: LOC_YARD},
			{Verbs: []Motion{MotOut}, Dir: MotOut, Act: ActGoto, Dest: LOC_STREET},
		},
	},
	{ // LOC_STUDY
		Long:  "The dead man's study. The papers sit squared on the desk; the air is stale and faintly sweet. Everything is in perfect order — which is itself the most disquieting thing in the room.",
		Short: "The victim's study.",
		Lit:   true,
		Travel: []Travel{
			{Verbs: []Motion{MotHall}, Dir: MotHall, Act: ActGoto, Dest: LOC_HALL},
			{Verbs: []Motion{MotBedroom}, Dir: MotBedroom, Act: ActGoto, Dest: LOC_BEDROOM},
		},
	},
	{ // LOC_BEDROOM
		Long:  "A cramped bedroom. The bed is made and unslept-in: whatever the victim feared, he did not lie down to wait for it. A single window gives onto the yard.",
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
	{ // LOC_CHEMIST — gated by 'poison'
		Long:  "The chemist's shop, close with the smell of camphor and bitter almond. Mr. Elias Crole keeps the counter — a heavy, soft-spoken man whose eyes will not settle. Behind him, a wall of dark bottles; before him, his open day-ledger.",
		Short: "The chemist's shop.",
		Lit:   true,
		Travel: []Travel{
			{Verbs: []Motion{MotStreet}, Dir: MotStreet, Act: ActGoto, Dest: LOC_STREET},
		},
	},
	{ // LOC_CABMAN
		Long:  "Tom Feaney leans against his hansom, blowing on his hands. He has the loose tongue of a man who has waited all night in the cold and wants company for it.",
		Short: "The cabman's stand.",
		Lit:   true,
		Travel: []Travel{
			{Verbs: []Motion{MotStreet}, Dir: MotStreet, Act: ActGoto, Dest: LOC_STREET},
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
		Words:        []string{"letter", "scrap", "ashes"},
		Inventory:    "the charred scrap",
		Descriptions: []string{"A charred scrap of notepaper survives in the cold grate."},
		Start:        LOC_HALL,
		Immovable:    true,
	},
	{ // OBJ_BOOT
		Words:        []string{"print", "boot"},
		Inventory:    "the boot-print",
		Descriptions: []string{"A boot-print is pressed into the coal dust."},
		Start:        LOC_YARD,
		Immovable:    true,
	},
	{ // OBJ_LATCH
		Words:        []string{"latch", "window"},
		Inventory:    "the window latch",
		Descriptions: []string{"The window latch catches the light — it does not sit quite true."},
		Start:        LOC_BEDROOM,
		Immovable:    true,
	},
	{ // OBJ_VISITOR — landlady's testimony (a "topic")
		Words:        []string{"landlady", "hudd", "visitor"},
		Inventory:    "Mrs. Hudd's account",
		Descriptions: []string{"Mrs. Hudd looks ready to speak of the night's visitor."},
		Start:        LOC_HALL,
		Immovable:    true,
	},
	{ // OBJ_CABTIME — cabman's testimony
		Words:        []string{"cabman", "feaney", "fare"},
		Inventory:    "Feaney's account",
		Descriptions: []string{"Feaney is happy to talk about the fare he drove that night."},
		Start:        LOC_CABMAN,
		Immovable:    true,
	},
	{ // OBJ_LEDGER
		Words:        []string{"ledger", "book"},
		Inventory:    "the chemist's ledger",
		Descriptions: []string{"The chemist's day-ledger lies open on the counter."},
		Start:        LOC_CHEMIST,
		Immovable:    true,
	},
	{ // OBJ_TINCTURE
		Words:        []string{"tincture", "bottle", "poison"},
		Inventory:    "the dark bottle",
		Descriptions: []string{"One dark bottle on the shelf is turned label-out, as if lately handled."},
		Start:        LOC_CHEMIST,
		Immovable:    true,
	},
}

// Messages is unused by this story (no blocked-travel or speak actions).
var Messages = []string{}
