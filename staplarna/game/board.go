package game

// Side identifies a player color.
type Side int

const (
	None Side = iota
	Black
	White
)

// Opponent returns the other side (None maps to None).
func (s Side) Opponent() Side {
	switch s {
	case Black:
		return White
	case White:
		return Black
	default:
		return None
	}
}

// PieceType identifies one of TZAAR's three piece types.
type PieceType int

const (
	Tzaar  PieceType = iota // scarcest: 6 per side — losing your last one loses the game
	Tzarra                  // 9 per side
	Tott                     // most numerous: 15 per side
)

// AllTypes lists the 3 piece types in a fixed (scarcest-first) order, used
// for stable iteration and UI ordering.
var AllTypes = [3]PieceType{Tzaar, Tzarra, Tott}

// StartCount returns how many of typ each side places during setup.
func StartCount(typ PieceType) int {
	switch typ {
	case Tzaar:
		return 6
	case Tzarra:
		return 9
	default:
		return 15
	}
}

// TotalPerSide is the total pieces (6+9+15) each side places during setup.
const TotalPerSide = 6 + 9 + 15

// Stack is one occupied board cell: one or more pieces belonging to the same
// side, stacked together by merges.
type Stack struct {
	Owner  Side
	Type   PieceType // the TOP (visible) piece's type — what render/movement-legality display shows, and (via Height) what a future capture must beat
	Height int       // total physical pieces in the stack; Height == Comp[Tzaar]+Comp[Tzarra]+Comp[Tott]

	// Comp tracks how many of EACH type are physically inside this stack —
	// not just the visible top one. This is what makes TZAAR's core
	// defensive tactic ("bury your last scarce piece under others for
	// protection") mean something: from the outside only Type/Height are
	// visible, but if this whole stack is later captured, EVERY piece
	// counted in Comp is eliminated, including a buried Tzaar — exactly as
	// in the physical game, where a captured tower's full contents come off
	// the board, not just its top piece. A merge must therefore ADD Comp
	// element-wise, not just bump Height, or the type-elimination win check
	// (winner.go) would miss pieces that are still really there, buried.
	Comp [3]int
}

// Board holds the mutable position: at most one stack per cell.
type Board struct {
	Stacks map[Point]Stack
}

// NewBoard returns an empty board.
func NewBoard() *Board { return &Board{Stacks: map[Point]Stack{}} }

// Clone returns a deep copy, used by the AI search so it can try moves
// without disturbing the real game state.
func (b *Board) Clone() *Board {
	nb := &Board{Stacks: make(map[Point]Stack, len(b.Stacks))}
	for p, s := range b.Stacks {
		nb.Stacks[p] = s
	}
	return nb
}

// At returns the stack at p, if any.
func (b *Board) At(p Point) (Stack, bool) {
	s, ok := b.Stacks[p]
	return s, ok
}

// TypeCount sums the physical pieces of type typ belonging to side currently
// on the board — counting every piece actually present (via each stack's
// Comp), not just stacks whose visible top happens to be that type. This is
// the basis of the type-elimination win condition (see winner.go): a side
// with zero remaining pieces of ANY ONE type — even if the last one is
// buried inside a taller stack topped by a different type — has lost.
func (b *Board) TypeCount(side Side, typ PieceType) int {
	n := 0
	for _, s := range b.Stacks {
		if s.Owner == side {
			n += s.Comp[typ]
		}
	}
	return n
}

// StackCount returns how many stacks (occupied cells) side owns.
func (b *Board) StackCount(side Side) int {
	n := 0
	for _, s := range b.Stacks {
		if s.Owner == side {
			n++
		}
	}
	return n
}

// PlaceNew adds a brand-new height-1 stack of (side, typ) at p, used during
// the setup phase. Returns false (no-op) if p is off-board or already
// occupied.
func (b *Board) PlaceNew(side Side, typ PieceType, p Point) bool {
	if !Valid(p) {
		return false
	}
	if _, occupied := b.Stacks[p]; occupied {
		return false
	}
	s := Stack{Owner: side, Type: typ, Height: 1}
	s.Comp[typ] = 1
	b.Stacks[p] = s
	return true
}

// Move is a single stack move from From to To.
type Move struct {
	From, To Point
}

// destinationKind classifies what a stack of (side, height) landing on `to`
// would mean: whether the landing is legal at all, and whether it captures.
func (b *Board) destinationKind(side Side, height int, to Point) (legal, capture bool) {
	if tgt, ok := b.Stacks[to]; ok {
		if tgt.Owner == side {
			return true, false // merge with one's own stack — always legal
		}
		return tgt.Height <= height, true // capture iff the enemy stack is not taller
	}
	return true, false // empty cell: a plain move
}

// rayTarget walks EXACTLY `steps` cells from `from` along direction d. Every
// intervening cell (steps 1..steps-1) must be empty and on the board — a
// move can never pass through, or jump over, any occupied cell, friend or
// foe. Returns the landing cell and true if the walk stayed clear and
// on-board that far; the landing cell's own occupancy (empty / own stack /
// capturable enemy) is deliberately NOT checked here — see destinationKind.
func rayTarget(b *Board, from, d Point, steps int) (Point, bool) {
	p := from
	for i := 1; i <= steps; i++ {
		p = p.Add(d)
		if !Valid(p) {
			return Point{}, false
		}
		if i < steps {
			if _, occupied := b.Stacks[p]; occupied {
				return Point{}, false // blocked partway: cannot pass through
			}
		}
	}
	return p, true
}

// LegalMoves returns every legal move for side during the play phase: from
// each of side's stacks of height N, in each of the 6 hex directions, walked
// EXACTLY N steps (never "up to N", unlike a sliding piece — a lone piece,
// height 1, can only ever move exactly one cell), landing on an empty cell,
// one of side's own stacks (a merge), or an enemy stack whose height is <= N
// (a capture, removing it whole). A cell short of the destination that is
// occupied by ANYTHING blocks that whole ray.
//
// DESIGN DECISION — captures are NOT mandatory: the task's spec flagged
// "capture forced unless it's the only legal move type" as unverified
// folklore, and explicitly asked for the safer documented assumption: a
// plain non-capturing move (to an empty cell, or a merge with one's own
// stack) remains legal even when a capturing move exists elsewhere on the
// board. This function does not special-case or filter out non-captures in
// any way when a capture is available — every legal destination (of all 3
// kinds) is always included.
func LegalMoves(b *Board, side Side) []Move {
	var moves []Move
	for from, s := range b.Stacks {
		if s.Owner != side {
			continue
		}
		for _, d := range Directions {
			to, ok := rayTarget(b, from, d, s.Height)
			if !ok {
				continue
			}
			if legal, _ := b.destinationKind(side, s.Height, to); legal {
				moves = append(moves, Move{From: from, To: to})
			}
		}
	}
	return moves
}

// DestinationsFrom returns the legal landing cells for the stack at `from`
// (from must currently hold one of side's stacks) — used by the UI to
// highlight exact-distance destinations once a stack is selected.
func DestinationsFrom(b *Board, from Point) []Point {
	s, ok := b.Stacks[from]
	if !ok {
		return nil
	}
	var out []Point
	for _, d := range Directions {
		to, ok := rayTarget(b, from, d, s.Height)
		if !ok {
			continue
		}
		if legal, _ := b.destinationKind(s.Owner, s.Height, to); legal {
			out = append(out, to)
		}
	}
	return out
}

// IsLegalMove reports whether side may legally play m right now.
func IsLegalMove(b *Board, side Side, m Move) bool {
	s, ok := b.Stacks[m.From]
	if !ok || s.Owner != side {
		return false
	}
	if m.From == m.To {
		return false
	}
	for _, d := range Directions {
		to, ok := rayTarget(b, m.From, d, s.Height)
		if !ok || to != m.To {
			continue
		}
		legal, _ := b.destinationKind(side, s.Height, m.To)
		return legal
	}
	return false
}

// Apply executes move m on b IN PLACE — b must be a board the caller owns
// exclusively (a Clone() taken for AI search, or the live game's own board;
// callers should check IsLegalMove first). Returns whether it captured an
// enemy stack.
func (b *Board) Apply(m Move) bool {
	mover := b.Stacks[m.From]
	delete(b.Stacks, m.From)

	tgt, occupied := b.Stacks[m.To]
	if !occupied {
		b.Stacks[m.To] = mover
		return false
	}
	if tgt.Owner == mover.Owner {
		// Merge: the mover's stack lands ON TOP of the destination stack, so
		// its Type stays the visible one; Comp/Height simply add (see the
		// Stack doc comment — no pieces are lost or hidden from the
		// type-count bookkeeping by a merge, only visually).
		merged := Stack{Owner: mover.Owner, Type: mover.Type, Height: mover.Height + tgt.Height}
		for i := range merged.Comp {
			merged.Comp[i] = mover.Comp[i] + tgt.Comp[i]
		}
		b.Stacks[m.To] = merged
		return false
	}
	// Enemy stack, assumed height <= mover's (legal): captured whole —
	// EVERY piece in it (Comp) leaves the board along with it.
	b.Stacks[m.To] = mover
	return true
}
