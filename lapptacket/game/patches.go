package game

// Patch is one buyable tile: a shape (polyomino, in its base orientation),
// its button cost, its time-track cost (how far the buyer's own marker
// advances), and its permanent button-income value (added to the owner's
// income total once placed, paid out whenever a marker crosses an income
// square).
type Patch struct {
	ID       int
	Name     string
	Cells    []Offset
	Cost     int
	TimeCost int
	Income   int
}

// Size is the number of squares the patch covers.
func (p Patch) Size() int { return len(p.Cells) }

// --- Shape library -----------------------------------------------------
//
// This is an ORIGINAL, invented roster of 33 patches loosely in the spirit
// of Patchwork (Uwe Rosenberg / Lookout Games): varied polyomino shapes,
// each with a button-cost, a time-cost, and a button-income value. It does
// not reproduce the real game's exact tile list, costs, or artwork — the
// spec for this port explicitly calls for inventing reasonable values
// rather than guessing the official numbers from memory. Several shapes
// are deliberately reused under different stats (the real game does this
// too: the same footprint can appear more than once with different
// cost/time/income combinations).

var (
	shMono   = []Offset{{0, 0}}
	shDomino = []Offset{{0, 0}, {1, 0}}
	shI3     = []Offset{{0, 0}, {1, 0}, {2, 0}}
	shL3     = []Offset{{0, 0}, {1, 0}, {0, 1}}
	shO4     = []Offset{{0, 0}, {1, 0}, {0, 1}, {1, 1}}
	shI4     = []Offset{{0, 0}, {1, 0}, {2, 0}, {3, 0}}
	shT4     = []Offset{{0, 0}, {1, 0}, {2, 0}, {1, 1}}
	shS4     = []Offset{{1, 0}, {2, 0}, {0, 1}, {1, 1}}
	shL4     = []Offset{{0, 0}, {0, 1}, {0, 2}, {1, 2}}
	shP5     = []Offset{{0, 0}, {1, 0}, {0, 1}, {1, 1}, {0, 2}}
	shPlus5  = []Offset{{1, 0}, {0, 1}, {1, 1}, {2, 1}, {1, 2}}
	shU5     = []Offset{{0, 0}, {0, 1}, {1, 1}, {2, 1}, {2, 0}}
	shZ5     = []Offset{{0, 0}, {1, 0}, {1, 1}, {1, 2}, {2, 2}}
	shL5     = []Offset{{0, 0}, {0, 1}, {0, 2}, {0, 3}, {1, 3}}
	shRect6  = []Offset{{0, 0}, {1, 0}, {2, 0}, {0, 1}, {1, 1}, {2, 1}}
	shL6     = []Offset{{0, 0}, {0, 1}, {0, 2}, {0, 3}, {1, 3}, {2, 3}}
	shZig6   = []Offset{{0, 0}, {1, 0}, {1, 1}, {2, 1}, {2, 2}, {3, 2}}
	shBlob7  = []Offset{{0, 0}, {1, 0}, {2, 0}, {0, 1}, {1, 1}, {2, 1}, {1, 2}}
	shBig8   = []Offset{{0, 0}, {1, 0}, {2, 0}, {0, 1}, {1, 1}, {2, 1}, {0, 2}, {1, 2}} // 3x3 minus one corner
)

// Patches is the fixed roster of 33 buyable patches, in track/queue order
// (index order is arbitrary but fixed — it's the order they queue up
// around the neutral token).
var Patches = []Patch{
	{ID: 0, Name: "Fläck", Cells: shMono, Cost: 1, TimeCost: 1, Income: 0},
	{ID: 1, Name: "Knapp", Cells: shMono, Cost: 2, TimeCost: 1, Income: 1},
	{ID: 2, Name: "Remsa", Cells: shDomino, Cost: 2, TimeCost: 2, Income: 0},
	{ID: 3, Name: "Band", Cells: shDomino, Cost: 3, TimeCost: 1, Income: 1},
	{ID: 4, Name: "Trerad", Cells: shI3, Cost: 3, TimeCost: 3, Income: 1},
	{ID: 5, Name: "Vinkel", Cells: shL3, Cost: 2, TimeCost: 2, Income: 0},
	{ID: 6, Name: "Krok", Cells: shL3, Cost: 6, TimeCost: 1, Income: 2},
	{ID: 7, Name: "Ruta", Cells: shO4, Cost: 6, TimeCost: 3, Income: 1},
	{ID: 8, Name: "Stång", Cells: shI4, Cost: 3, TimeCost: 3, Income: 0},
	{ID: 9, Name: "T-lapp", Cells: shT4, Cost: 5, TimeCost: 2, Income: 1},
	{ID: 10, Name: "S-lapp", Cells: shS4, Cost: 4, TimeCost: 2, Income: 0},
	{ID: 11, Name: "Vinkelhus", Cells: shL4, Cost: 7, TimeCost: 1, Income: 2},
	{ID: 12, Name: "Bred ruta", Cells: shO4, Cost: 1, TimeCost: 4, Income: 0},
	{ID: 13, Name: "P-lapp", Cells: shP5, Cost: 8, TimeCost: 2, Income: 2},
	{ID: 14, Name: "Kors", Cells: shPlus5, Cost: 7, TimeCost: 3, Income: 2},
	{ID: 15, Name: "U-lapp", Cells: shU5, Cost: 5, TimeCost: 4, Income: 1},
	{ID: 16, Name: "Zick-zack", Cells: shZ5, Cost: 4, TimeCost: 3, Income: 1},
	{ID: 17, Name: "Långvinkel", Cells: shL5, Cost: 10, TimeCost: 1, Income: 3},
	{ID: 18, Name: "Lugnt kors", Cells: shPlus5, Cost: 2, TimeCost: 5, Income: 0},
	{ID: 19, Name: "Rektangel", Cells: shRect6, Cost: 10, TimeCost: 3, Income: 2},
	{ID: 20, Name: "Storvinkel", Cells: shL6, Cost: 7, TimeCost: 4, Income: 1},
	{ID: 21, Name: "Trappa", Cells: shZig6, Cost: 5, TimeCost: 5, Income: 1},
	{ID: 22, Name: "Låg rektangel", Cells: shRect6, Cost: 3, TimeCost: 6, Income: 0},
	{ID: 23, Name: "Klump", Cells: shBlob7, Cost: 8, TimeCost: 5, Income: 2},
	{ID: 24, Name: "Dyr klump", Cells: shBlob7, Cost: 10, TimeCost: 2, Income: 2},
	{ID: 25, Name: "Storblock", Cells: shBig8, Cost: 10, TimeCost: 4, Income: 3},
	{ID: 26, Name: "Segt block", Cells: shBig8, Cost: 5, TimeCost: 6, Income: 1},
	{ID: 27, Name: "Kort rad", Cells: shI3, Cost: 1, TimeCost: 3, Income: 0},
	{ID: 28, Name: "Andra T", Cells: shT4, Cost: 2, TimeCost: 4, Income: 0},
	{ID: 29, Name: "Billig remsa", Cells: shDomino, Cost: 1, TimeCost: 2, Income: 0},
	{ID: 30, Name: "Snabb U", Cells: shU5, Cost: 9, TimeCost: 1, Income: 2},
	{ID: 31, Name: "Andra storvinkel", Cells: shL6, Cost: 6, TimeCost: 5, Income: 1},
	{ID: 32, Name: "Dyr fläck", Cells: shMono, Cost: 3, TimeCost: 1, Income: 1},
}

// FreePatch is the shape used for the ~8 special 1x1 scoring tiles claimed
// for free when a marker reaches/passes a fixed track position — a single
// square worth +1 permanent button-income, placed anywhere on the owner's
// board.
var FreePatch = Patch{ID: -1, Name: "Bonusruta", Cells: shMono, Cost: 0, TimeCost: 0, Income: 1}

// NewQueue returns a fresh copy of the 33-patch roster (callers mutate their
// own copy as patches are bought, so the package-level slice stays pristine
// for tests/new games).
func NewQueue() []Patch {
	q := make([]Patch, len(Patches))
	copy(q, Patches)
	return q
}
