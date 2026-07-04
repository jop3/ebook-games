package game

// CourseDiff selects how hazard-heavy a generated (or fixed) course is. It is
// independent of AI difficulty (see ai.go) — any pairing is valid.
type CourseDiff int

const (
	DiffEasy   CourseDiff = iota // "Träning": walls, single belts, a few off-path pits
	DiffMedium                   // + express belts, gears, single-barrel lasers, chokepoints
	DiffHard                     // + double-barrel lasers, belt→pit routes, crossfire
)

func (d CourseDiff) String() string {
	switch d {
	case DiffEasy:
		return "Lätt"
	case DiffMedium:
		return "Mellan"
	case DiffHard:
		return "Svår"
	}
	return "?"
}

// budget is the per-tier element budget the generator reads. Densities are
// fractions of the board's floor tiles. Difficulty is placement + which
// elements are enabled, not raw board size (see spec §5a).
type budget struct {
	size          int     // default board side
	checkpoints   int     // number of checkpoints to place
	pitFrac       float64 // fraction of tiles that become pits
	beltFrac      float64 // fraction of tiles carved into belt lanes
	express       bool    // express belts enabled
	gears         bool    // gears enabled
	lasers        int     // number of wall lasers to place
	doubleBarrel  bool    // lasers may be 2-barrel
	roundBudget   int     // rounds the reference planner is allowed to solve within
	minCheckSpace int     // minimum Manhattan spacing between consecutive checkpoints
}

func budgetFor(d CourseDiff) budget {
	switch d {
	case DiffEasy:
		return budget{size: 8, checkpoints: 2, pitFrac: 0.04, beltFrac: 0.10,
			express: false, gears: false, lasers: 0, doubleBarrel: false,
			roundBudget: 14, minCheckSpace: 5}
	case DiffMedium:
		return budget{size: 10, checkpoints: 3, pitFrac: 0.08, beltFrac: 0.14,
			express: true, gears: true, lasers: 2, doubleBarrel: false,
			roundBudget: 18, minCheckSpace: 6}
	default: // DiffHard
		return budget{size: 12, checkpoints: 4, pitFrac: 0.11, beltFrac: 0.16,
			express: true, gears: true, lasers: 4, doubleBarrel: true,
			roundBudget: 22, minCheckSpace: 6}
	}
}
