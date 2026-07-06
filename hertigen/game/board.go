// Package game implements the rules of Hertigen ("The Duke"), a double-sided
// tile duel with no dependency on the inkview SDK, so it can be unit-tested
// cgo-free. See tiles.go for the tile roster and README-style commentary at
// the top of that file for the design's relationship to the real The Duke.
//
// The board is 6x6. Each side starts with a Duke plus 2 troop tiles on the
// board (Fotknekt and Kämpe — an original starting layout, since the spec
// this game is built from explicitly flags the real starting roster as
// unverified from memory); the other 4 troop types start in reserve,
// recruitable next to the Duke. Every tile is double-sided: acting (moving,
// jumping, or striking) flips it to its other face, changing what it can do
// next time — this is the entire mechanic, and it applies uniformly to every
// tile including the Duke. Win by capturing the opposing Duke.
package game

import "image"

// Side names a player color.
type Side uint8

const (
	Black Side = iota
	White
)

// Opponent returns the other player color.
func (s Side) Opponent() Side {
	if s == Black {
		return White
	}
	return Black
}

// Tile is a single piece, on the board or (logically) in reserve.
type Tile struct {
	Type TileType
	Side Side
	Face Face
}

// Size is the edge length of the board.
const Size = 6

// Board holds the grid, indexed [y][x] (row-major, y=0 is White's home row,
// matching hasami/shong's top-White/bottom-Black convention). A nil entry is
// an empty square.
type Board [Size][Size]*Tile

func inBounds(x, y int) bool { return x >= 0 && x < Size && y >= 0 && y < Size }

// At returns the tile at (x,y), or nil if the square is empty or
// out-of-bounds.
func (b *Board) At(x, y int) *Tile {
	if !inBounds(x, y) {
		return nil
	}
	return b[y][x]
}

func (b *Board) set(x, y int, t *Tile) { b[y][x] = t }

// homeRow returns side's own back row.
func homeRow(side Side) int {
	if side == Black {
		return Size - 1
	}
	return 0
}

// startX are the board columns used for the Duke and its two starting
// troops, chosen to be simple and central on a 6-wide board: Fotknekt at
// x=1, Duke at x=2, Kämpe at x=3 — our own reasonable starting layout (see
// the package doc comment on why the official layout isn't used here).
const (
	startDukeX    = 2
	startFootmanX = 1
	startChampX   = 3
)

// NewBoard returns a board in Hertigen's starting position: both sides'
// Duke plus a Fotknekt and a Kämpe on their home row, all on FaceA. The
// remaining 4 troop types (Riddare, Ryttare, Diagonalvakt, Katapult) start
// off-board, in reserve — see NewReserve.
func NewBoard() Board {
	var b Board
	for _, side := range [2]Side{Black, White} {
		y := homeRow(side)
		b.set(startDukeX, y, &Tile{Type: Duke, Side: side, Face: FaceA})
		b.set(startFootmanX, y, &Tile{Type: Footman, Side: side, Face: FaceA})
		b.set(startChampX, y, &Tile{Type: Champion, Side: side, Face: FaceA})
	}
	return b
}

// ReserveMask is a bitset of TileType values (bit index = TileType) that a
// side still has in reserve, off the board, awaiting recruitment.
type ReserveMask uint16

func reserveBit(t TileType) ReserveMask { return 1 << uint(t) }

// Has reports whether t is still in reserve.
func (m ReserveMask) Has(t TileType) bool { return m&reserveBit(t) != 0 }

// Remove returns m with t no longer in reserve.
func (m ReserveMask) Remove(t TileType) ReserveMask { return m &^ reserveBit(t) }

// Types returns every TileType currently in reserve, in TroopTypes order.
func (m ReserveMask) Types() []TileType {
	var out []TileType
	for _, t := range TroopTypes {
		if m.Has(t) {
			out = append(out, t)
		}
	}
	return out
}

// NewReserve returns the starting reserve for one side: every troop type
// except the two that start on the board (Footman, Champion).
func NewReserve() ReserveMask {
	var m ReserveMask
	for _, t := range TroopTypes {
		if t == Footman || t == Champion {
			continue
		}
		m |= reserveBit(t)
	}
	return m
}

// Count returns the number of side's tiles currently on the board.
func (b *Board) Count(side Side) int {
	n := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if t := b[y][x]; t != nil && t.Side == side {
				n++
			}
		}
	}
	return n
}

// DukePos returns the position of side's Duke, and false if it has been
// captured (i.e. the game is over).
func (b *Board) DukePos(side Side) (image.Point, bool) {
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if t := b[y][x]; t != nil && t.Type == Duke && t.Side == side {
				return image.Pt(x, y), true
			}
		}
	}
	return image.Point{}, false
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	default:
		return 0
	}
}

func maxAbs(a, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	if a > b {
		return a
	}
	return b
}

// pathClear reports whether every square strictly between (x0,y0) and
// (x0+dx,y0+dy) is empty. It assumes (dx,dy) is a straight line — pure
// orthogonal (one of dx,dy is 0) or pure diagonal (|dx|==|dy|) — which is
// the only shape MoveOnly/Strike/MoveOrStrike offsets ever use in this
// roster's pattern table (see tiles.go); a non-straight offset (e.g. a
// knight leap) is only ever tagged Jump, which ignores this check entirely.
// Distance-1 offsets trivially pass (the loop below never runs).
func pathClear(b *Board, x0, y0, dx, dy int) bool {
	n := maxAbs(dx, dy)
	stepX, stepY := sign(dx), sign(dy)
	x, y := x0, y0
	for i := 1; i < n; i++ {
		x += stepX
		y += stepY
		if !inBounds(x, y) || b.At(x, y) != nil {
			return false
		}
	}
	return true
}

// ActionKind names what a legal Action does.
type ActionKind uint8

const (
	// ActRelocate covers MoveOnly, Jump, and the relocating half of
	// MoveOrStrike: the acting tile ends up at To. If To held an enemy tile
	// (only possible for Jump and MoveOrStrike), that enemy tile is
	// captured by displacement.
	ActRelocate ActionKind = iota
	// ActStrike captures the enemy tile at To WITHOUT relocating the acting
	// tile, which stays at From.
	ActStrike
	// ActRecruit places a reserve tile of type Recruit on the empty square
	// To, which must be orthogonally adjacent to the acting side's Duke's
	// CURRENT square. From is unused.
	ActRecruit
)

// Action is one legal thing a side may do on its turn: exactly one of
// relocating/jumping a tile, striking with a tile, or recruiting a reserve
// tile next to the Duke.
type Action struct {
	Kind    ActionKind
	From    image.Point // origin square (ActRelocate/ActStrike)
	To      image.Point // destination (ActRelocate) or target (ActStrike) or placement (ActRecruit)
	Recruit TileType     // meaningful only when Kind == ActRecruit
}

// ActionsFrom returns every legal ActRelocate/ActStrike action for the tile
// sitting at p (nil if the square is empty). This is the WHOLE generic move
// generator: it reads patternTable[tile.Type][tile.Face] — always the
// tile's CURRENT face, since flip-on-act means yesterday's face is never
// consulted — and evaluates each printed offset against the live board.
// There is no per-tile-type special case anywhere in this function.
func (b *Board) ActionsFrom(p image.Point) []Action {
	tile := b.At(p.X, p.Y)
	if tile == nil {
		return nil
	}
	var out []Action
	for _, e := range Patterns(tile.Type, tile.Face) {
		tx, ty := p.X+e.Dx, p.Y+e.Dy
		if !inBounds(tx, ty) {
			continue
		}
		target := b.At(tx, ty)
		to := image.Pt(tx, ty)
		switch e.Kind {
		case MoveOnly:
			if target != nil {
				continue // MoveOnly never captures; destination must be empty
			}
			if !pathClear(b, p.X, p.Y, e.Dx, e.Dy) {
				continue
			}
			out = append(out, Action{Kind: ActRelocate, From: p, To: to})
		case Jump:
			if target != nil && target.Side == tile.Side {
				continue // never lands on your own tile
			}
			// Ignores intervening squares/pieces entirely — that's the
			// point of a jump — so no pathClear call here.
			out = append(out, Action{Kind: ActRelocate, From: p, To: to})
		case Strike:
			if target == nil || target.Side == tile.Side {
				continue // Strike only ever targets an enemy tile
			}
			if !pathClear(b, p.X, p.Y, e.Dx, e.Dy) {
				continue
			}
			out = append(out, Action{Kind: ActStrike, From: p, To: to})
		case MoveOrStrike:
			if target != nil && target.Side == tile.Side {
				continue // never lands on your own tile
			}
			if !pathClear(b, p.X, p.Y, e.Dx, e.Dy) {
				continue
			}
			out = append(out, Action{Kind: ActRelocate, From: p, To: to})
		}
	}
	return out
}

// dukeAdjacent are the 4 orthogonal offsets checked for a legal recruit
// placement.
var dukeAdjacent = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// RecruitActions returns every legal ActRecruit action for side: one per
// (reserve tile type) x (empty square orthogonally adjacent to side's
// Duke's CURRENT square). Empty if side's Duke has been captured (game
// over) or side has nothing left in reserve.
func (b *Board) RecruitActions(side Side, reserve ReserveMask) []Action {
	if reserve == 0 {
		return nil
	}
	dukePos, ok := b.DukePos(side)
	if !ok {
		return nil
	}
	var squares []image.Point
	for _, d := range dukeAdjacent {
		x, y := dukePos.X+d[0], dukePos.Y+d[1]
		if inBounds(x, y) && b.At(x, y) == nil {
			squares = append(squares, image.Pt(x, y))
		}
	}
	if len(squares) == 0 {
		return nil
	}
	var out []Action
	for _, t := range reserve.Types() {
		for _, sq := range squares {
			out = append(out, Action{Kind: ActRecruit, To: sq, Recruit: t})
		}
	}
	return out
}

// LegalActions returns every legal action for side: every tile-move/strike
// action from every one of side's tiles on the board, plus every legal
// recruit action.
func (b *Board) LegalActions(side Side, reserve ReserveMask) []Action {
	var out []Action
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if t := b[y][x]; t != nil && t.Side == side {
				out = append(out, b.ActionsFrom(image.Pt(x, y))...)
			}
		}
	}
	out = append(out, b.RecruitActions(side, reserve)...)
	return out
}

// IsLegalAction reports whether a is among side's legal actions right now.
func (b *Board) IsLegalAction(side Side, reserve ReserveMask, a Action) bool {
	for _, la := range b.LegalActions(side, reserve) {
		if la == a {
			return true
		}
	}
	return false
}

// Apply plays action a (assumed legal — callers should check
// IsLegalAction first) and returns the resulting board plus the list of
// cells that lost a tile as a direct result (0 or 1 cells; kept as a slice
// so the UI can reuse the same "briefly mark the last capture" pattern used
// by the other games in this repo).
//
// ActRelocate: the acting tile moves from From to To, flipping to its other
// face; if To held an enemy tile, that tile is captured (removed).
// ActStrike: the enemy tile at To is captured (removed); the striking tile
// stays at From but still flips to its other face — it acted, even though
// it didn't move.
// ActRecruit is NOT handled here: it doesn't touch any existing tile's face
// and it also mutates the reserve, which Board doesn't know about — see
// GameState.Play in state.go.
func (b Board) Apply(a Action) (Board, []image.Point) {
	nb := b
	switch a.Kind {
	case ActRelocate:
		mover := nb.At(a.From.X, a.From.Y)
		var captured []image.Point
		if t := nb.At(a.To.X, a.To.Y); t != nil {
			captured = []image.Point{a.To}
		}
		nb.set(a.From.X, a.From.Y, nil)
		nb.set(a.To.X, a.To.Y, &Tile{Type: mover.Type, Side: mover.Side, Face: mover.Face.Flipped()})
		return nb, captured
	case ActStrike:
		striker := nb.At(a.From.X, a.From.Y)
		nb.set(a.To.X, a.To.Y, nil)
		nb.set(a.From.X, a.From.Y, &Tile{Type: striker.Type, Side: striker.Side, Face: striker.Face.Flipped()})
		return nb, []image.Point{a.To}
	default:
		return nb, nil
	}
}
