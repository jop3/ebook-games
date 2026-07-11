package game

// tiles.go: the tile roster and its printed move/strike patterns. This file
// is pure DATA — a (tile type, face) -> list of (offset, kind) table — plus
// the small generators used to build a few repetitive entries (sliding rays).
// The move generator that reads this table lives in board.go and never
// special-cases a tile type; every tile, including the Duke, is driven
// entirely by what's printed here.
//
// # Design note: this roster is an ORIGINAL invention for this game, not a
// transcription of the real The Duke's tile set. The spec this game is built
// from explicitly flags that the real starting roster/layout and the
// same-turn-recruit rule are UNVERIFIED from memory, and instructs us not to
// guess the official specifics. So instead we designed our own reasonable,
// internally-consistent set of 6 troop archetypes (plus the Duke), each
// double-sided with two distinct patterns, covering all four move kinds:
//
//   - Fotknekt (Footman)   — short-range slider (adjacent squares only).
//   - Riddare  (Knight)    — the jumper (leaps, ignores blockers).
//   - Ryttare  (Rider)     — long-range slider (full-board rook/bishop rays).
//   - Diagonalvakt (Diagonal Guard) — diagonal-only piece (never touches an
//     orthogonal square on either face).
//   - Katapult (Catapult)  — strike-only piece (never relocates, ever).
//   - Kämpe    (Champion)  — flexible all-rounder (mixes a slide AND a jump
//     on the very same face).
//
// Credit: "Baserat på The Duke (Catalyst Game Labs)" — we borrow only the
// central mechanic (double-sided tiles that flip to a different move pattern
// after acting, plus a Duke-capture win condition and adjacent-to-Duke
// recruiting), not any specific tile names, patterns, or starting layout.

// TileType names a kind of tile, including the Duke.
type TileType uint8

const (
	Duke TileType = iota
	Footman
	Knight
	Rider
	DiagGuard
	Catapult
	Champion
	numTileTypes
)

// TroopTypes lists every non-Duke tile type, in a stable order used for
// reserve iteration and UI listings.
var TroopTypes = []TileType{Footman, Knight, Rider, DiagGuard, Catapult, Champion}

// Name returns the Swedish display name of a tile type.
func (t TileType) Name() string {
	switch t {
	case Duke:
		return "Hertig"
	case Footman:
		return "Fotknekt"
	case Knight:
		return "Riddare"
	case Rider:
		return "Ryttare"
	case DiagGuard:
		return "Diagonalvakt"
	case Catapult:
		return "Katapult"
	case Champion:
		return "Kämpe"
	default:
		return "?"
	}
}

// Face selects which of a tile's two printed sides is currently up. Acting
// (moving, jumping or striking) always flips a tile to its OTHER face —
// since there are only two faces, "flip" is simply toggling this value;
// there is no separate "which face is this" table beyond the bit itself.
// (An integer type, not bool, so a Face can be used directly as a composite
// literal array index below — see patternTable.)
type Face uint8

const (
	FaceA Face = 0
	FaceB Face = 1
)

// Flipped returns the other face.
func (f Face) Flipped() Face {
	if f == FaceA {
		return FaceB
	}
	return FaceA
}

// MoveKind names the action a single printed offset icon grants.
type MoveKind uint8

const (
	// MoveKind slides to the offset: every square strictly between the
	// origin and the offset (along that offset's own straight-line
	// direction) must be empty, and the destination itself must be EMPTY
	// (never a capture). The tile relocates there.
	MoveOnly MoveKind = iota
	// Jump relocates directly to the offset, IGNORING any intervening
	// squares/pieces entirely (this is what makes it a "jump" rather than a
	// slide) — used for offsets that aren't even a straight line (e.g. a
	// knight-shaped leap), where "intervening square" isn't well defined
	// anyway. The destination must be empty OR hold an enemy tile; landing
	// on an enemy tile captures it by displacement (the jumping tile lands
	// there, the enemy tile is removed) — same displacement-capture idea as
	// a chess knight landing on an enemy square.
	Jump
	// Strike captures an enemy tile sitting exactly at the offset WITHOUT
	// relocating the striking tile — it stays on its own square. Like
	// MoveOnly, a Strike offset is a straight line and every square strictly
	// between origin and target must be empty (a clear line of sight); the
	// target square itself must hold an ENEMY tile (never empty, never own).
	Strike
	// MoveOrStrike slides to the offset like MoveOnly (same clear-path
	// requirement) but the destination may be either empty (plain relocate)
	// or an enemy tile (capture-by-landing, chess-capture style: the enemy
	// tile is removed and the mover relocates onto its square) — unlike
	// Strike, which captures WITHOUT moving.
	MoveOrStrike
)

// PatternEntry is one printed offset icon on a tile face: a relative
// (Dx,Dy) from the tile's own square, plus the action it grants there.
type PatternEntry struct {
	Dx, Dy int
	Kind   MoveKind
}

// orthoDirs4 / diagDirs4 are the 4 orthogonal and 4 diagonal unit
// directions, used only to build repetitive pattern-table entries below —
// this is a data-construction convenience, not special-cased movement
// logic; the resulting []PatternEntry is consumed by the one generic
// generator in board.go exactly like every hand-written entry.
var orthoDirs4 = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
var diagDirs4 = [4][2]int{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}}

// rayOffsets returns, for each direction in dirs, one PatternEntry per
// distance 1..maxDist (all sharing kind) — a sliding ray capped at maxDist.
// Bounds beyond the board edge are simply never reachable (the generator in
// board.go rejects out-of-bounds offsets), so capping at 5 (the largest
// possible distance on a 6x6 board) is equivalent to "as far as the board
// allows."
func rayOffsets(dirs [4][2]int, maxDist int, kind MoveKind) []PatternEntry {
	var out []PatternEntry
	for _, d := range dirs {
		for n := 1; n <= maxDist; n++ {
			out = append(out, PatternEntry{Dx: d[0] * n, Dy: d[1] * n, Kind: kind})
		}
	}
	return out
}

// adjacentOffsets returns one PatternEntry per direction in dirs at
// distance 1, all sharing kind.
func adjacentOffsets(dirs [4][2]int, kind MoveKind) []PatternEntry {
	return rayOffsets(dirs, 1, kind)
}

// distance2Offsets returns one PatternEntry per direction in dirs at
// exactly distance 2, all sharing kind.
func distance2Offsets(dirs [4][2]int, kind MoveKind) []PatternEntry {
	var out []PatternEntry
	for _, d := range dirs {
		out = append(out, PatternEntry{Dx: d[0] * 2, Dy: d[1] * 2, Kind: kind})
	}
	return out
}

// knightOffsets are the 8 classic knight-shaped leaps (±2,±1)/(±1,±2).
var knightOffsets = []PatternEntry{
	{Dx: 2, Dy: 1, Kind: Jump}, {Dx: 2, Dy: -1, Kind: Jump},
	{Dx: -2, Dy: 1, Kind: Jump}, {Dx: -2, Dy: -1, Kind: Jump},
	{Dx: 1, Dy: 2, Kind: Jump}, {Dx: 1, Dy: -2, Kind: Jump},
	{Dx: -1, Dy: 2, Kind: Jump}, {Dx: -1, Dy: -2, Kind: Jump},
}

// maxRay is the longest sliding distance worth listing on a 6x6 board.
const maxRay = 5

// patternTable maps [TileType][Face] -> the printed offsets on that face.
// This is the WHOLE engine's content: board.go's move generator does nothing
// but look up patternTable[tile.Type][tile.Face] and evaluate each entry
// against the current board — never a per-type switch on movement rules.
var patternTable = map[TileType][2][]PatternEntry{
	// Duke: a short-range all-rounder that alternates orthogonal and
	// diagonal adjacency each time it acts — genuinely dangerous both ways,
	// but never further than one square, so guarding it is about controlling
	// its 8 neighbouring squares, not long lines.
	Duke: {
		FaceA: adjacentOffsets(orthoDirs4, MoveOrStrike), // "Hertig·I" — orthogonal
		FaceB: adjacentOffsets(diagDirs4, MoveOrStrike),  // "Hertig·II" — diagonal
	},

	// Fotknekt (Footman): the basic short-range slider — same shape family
	// as the Duke (adjacent MoveOrStrike, alternating axis), but a
	// disposable troop rather than the piece you must protect.
	Footman: {
		FaceA: adjacentOffsets(orthoDirs4, MoveOrStrike), // "Fotknekt·I"
		FaceB: adjacentOffsets(diagDirs4, MoveOrStrike),  // "Fotknekt·II"
	},

	// Riddare (Knight): the jumper. Face I is the classic knight leap
	// (ignores blockers, can capture by landing); face II hops exactly 2
	// squares orthogonally, also ignoring whatever sits in between.
	Knight: {
		FaceA: knightOffsets,                      // "Riddare·I" — knight leap
		FaceB: distance2Offsets(orthoDirs4, Jump), // "Riddare·II" — orthogonal hop
	},

	// Ryttare (Rider): the long-range slider — a full rook ray on one face,
	// a full bishop ray on the other, each face's ray blocked by the first
	// occupied square in its path (own or enemy).
	Rider: {
		FaceA: rayOffsets(orthoDirs4, maxRay, MoveOrStrike), // "Ryttare·I" — rook-like
		FaceB: rayOffsets(diagDirs4, maxRay, MoveOrStrike),  // "Ryttare·II" — bishop-like
	},

	// Diagonalvakt (Diagonal Guard): diagonal-only on BOTH faces — it never
	// has an orthogonal offset at all. Face I can strike adjacent
	// diagonally; face II reaches 2 squares diagonally but MoveOnly (no
	// capture) — flipping between "can hit" and "can reposition further."
	DiagGuard: {
		FaceA: adjacentOffsets(diagDirs4, MoveOrStrike), // "Diagonalvakt·I"
		FaceB: distance2Offsets(diagDirs4, MoveOnly),    // "Diagonalvakt·II" — no capture
	},

	// Katapult (Catapult): strike-only, on both faces — it NEVER relocates.
	// Face I strikes exactly 2 squares orthogonally (needs a clear square in
	// between); face II strikes exactly 2 squares diagonally. A Katapult
	// that has no enemy sitting at one of its 4 printed offsets simply has
	// no legal action that turn.
	Catapult: {
		FaceA: distance2Offsets(orthoDirs4, Strike), // "Katapult·I"
		FaceB: distance2Offsets(diagDirs4, Strike),  // "Katapult·II"
	},

	// Kämpe (Champion): the flexible all-rounder — the only tile that mixes
	// TWO different MoveKinds on the SAME face: an adjacent MoveOrStrike
	// plus a 2-square diagonal (or orthogonal, on the flip side) Jump.
	Champion: {
		FaceA: append(append([]PatternEntry{}, adjacentOffsets(orthoDirs4, MoveOrStrike)...),
			distance2Offsets(diagDirs4, Jump)...), // "Kämpe·I"
		FaceB: append(append([]PatternEntry{}, adjacentOffsets(diagDirs4, MoveOrStrike)...),
			distance2Offsets(orthoDirs4, Jump)...), // "Kämpe·II"
	},
}

// Patterns returns the printed offsets for t's given face.
func Patterns(t TileType, f Face) []PatternEntry {
	return patternTable[t][f]
}
