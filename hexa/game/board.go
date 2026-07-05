package game

import "sort"

// Side is a player color. The zero value (None) means "empty", so a plain
// map lookup with no "ok" needed reads as "nothing there".
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

// TilesPerSide is the number of hex tiles each player has in total (in hand
// plus on the board) — 21 per side, 42 total, per the "Six" rules.
const TilesPerSide = 21

// Board holds the mutable tile layout: which cells carry a tile of which
// color. A tile map entry only ever exists for occupied cells.
type Board struct {
	Tiles map[Hex]Side
}

// NewBoard returns an empty board (no tiles placed yet).
func NewBoard() *Board {
	return &Board{Tiles: map[Hex]Side{}}
}

// Clone returns a deep copy, used by the AI search so it can try moves
// without disturbing the real game state.
func (b *Board) Clone() *Board {
	nb := &Board{Tiles: make(map[Hex]Side, len(b.Tiles))}
	for p, s := range b.Tiles {
		nb.Tiles[p] = s
	}
	return nb
}

// At returns the side occupying p (None if empty or off-board).
func (b *Board) At(p Hex) Side { return b.Tiles[p] }

// HasTile reports whether p is occupied.
func (b *Board) HasTile(p Hex) bool { return b.Tiles[p] != None }

// Count returns the number of side's tiles currently on the board.
func (b *Board) Count(side Side) int {
	n := 0
	for _, s := range b.Tiles {
		if s == side {
			n++
		}
	}
	return n
}

// Connected reports whether every tile in the map is reachable from every
// other tile by a chain of edge-adjacent tiles (an empty map, or a map with
// a single tile, both count as trivially connected).
func Connected(tiles map[Hex]Side) bool {
	if len(tiles) <= 1 {
		return true
	}
	var start Hex
	for p := range tiles {
		start = p
		break
	}
	seen := map[Hex]bool{start: true}
	queue := []Hex{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, d := range Directions {
			n := cur.Add(d)
			if _, ok := tiles[n]; ok && !seen[n] {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	return len(seen) == len(tiles)
}

// Components returns every connected component of tiles, each as a slice of
// hexes in deterministic (LessHex) order; the components themselves are
// ordered by their smallest member, so the result is fully deterministic —
// used by the advanced-rule disconnect/strand logic (state.go) and its
// tests.
func Components(tiles map[Hex]Side) [][]Hex {
	visited := map[Hex]bool{}
	var keys []Hex
	for p := range tiles {
		keys = append(keys, p)
	}
	sort.Slice(keys, func(i, j int) bool { return LessHex(keys[i], keys[j]) })

	var comps [][]Hex
	for _, p := range keys {
		if visited[p] {
			continue
		}
		var comp []Hex
		queue := []Hex{p}
		visited[p] = true
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			comp = append(comp, cur)
			for _, d := range Directions {
				n := cur.Add(d)
				if _, ok := tiles[n]; ok && !visited[n] {
					visited[n] = true
					queue = append(queue, n)
				}
			}
		}
		sort.Slice(comp, func(i, j int) bool { return LessHex(comp[i], comp[j]) })
		comps = append(comps, comp)
	}
	return comps
}

// PlaceMoves returns the legal empty in-board cells for the NEXT tile
// placement: cells edge-adjacent to the current cluster, or (when the board
// is still empty) every cell on the board, since the very first tile just
// starts the cluster wherever it lands. Returned in deterministic order.
func PlaceMoves(tiles map[Hex]Side) []Hex {
	if len(tiles) == 0 {
		return AllPoints()
	}
	seen := map[Hex]bool{}
	var out []Hex
	for p := range tiles {
		for _, d := range Directions {
			n := p.Add(d)
			if !InBoard(n) {
				continue
			}
			if _, occ := tiles[n]; occ {
				continue
			}
			if seen[n] {
				continue
			}
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return LessHex(out[i], out[j]) })
	return out
}

// Move is a candidate tile relocation during the movement phase.
type Move struct{ From, To Hex }

// MoveMoves lists side's legal tile moves. For each of side's own tiles, the
// candidate destinations are the frontier of the cluster with that tile
// removed (mirroring PlaceMoves' "adjacent to the cluster" rule, since a
// moved tile must re-attach to the remaining structure exactly like a fresh
// placement would). A candidate is legal in standard play only if applying
// it leaves the whole board connected; with advanced=true, a disconnecting
// move is legal too (state.go's ApplyMove then strands and removes whichever
// resulting component does not contain the moved tile).
func MoveMoves(tiles map[Hex]Side, side Side, advanced bool) []Move {
	var froms []Hex
	for p, s := range tiles {
		if s == side {
			froms = append(froms, p)
		}
	}
	sort.Slice(froms, func(i, j int) bool { return LessHex(froms[i], froms[j]) })

	var moves []Move
	for _, from := range froms {
		rest := withoutTile(tiles, from)
		for _, to := range PlaceMoves(rest) {
			if to == from {
				continue // not a real move
			}
			candidate := withoutTile(tiles, from)
			candidate[to] = side
			if Connected(candidate) {
				moves = append(moves, Move{from, to})
			} else if advanced {
				moves = append(moves, Move{from, to})
			}
		}
	}
	return moves
}

func withoutTile(tiles map[Hex]Side, p Hex) map[Hex]Side {
	out := make(map[Hex]Side, len(tiles))
	for k, v := range tiles {
		if k != p {
			out[k] = v
		}
	}
	return out
}
