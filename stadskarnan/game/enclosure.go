package game

import "image"

// CapturedInfo describes one opponent piece captured by an enclosure: its
// owner, which of that owner's 13 pieces it was, and the cells it occupied
// (now cleared back to Empty on the board).
type CapturedInfo struct {
	Owner   Cell
	PieceID int
	Cells   []image.Point
}

// isMember reports whether the cell at (x,y) counts as "inside" a region
// being tested for enclosure by wallColor: empty cells and the opponent's
// pieces are members (an opponent's building does not block the flood-fill,
// exactly how a stray enemy piece ends up trapped inside a fully-walled
// area); wallColor's own pieces and the Cathedral are never members — they
// are the walls.
func isMember(b *Board, x, y int, wallColor Cell) bool {
	c := b.Owner[y][x]
	return c == Empty || c == wallColor.Opponent()
}

// floodMember explores the connected component of "member" cells (per
// isMember) reachable from (x0,y0), marking them in visited. It reports the
// full region and whether any cell in it touches the board edge — a region
// touching the edge is always open (never enclosed), which is the classic
// Cathedral bug this flood-fill is written to avoid: the check is baked into
// the traversal itself, not bolted on afterwards.
func floodMember(b *Board, x0, y0 int, wallColor Cell, visited *[Size][Size]bool) (region []image.Point, touchesEdge bool) {
	stack := []image.Point{{X: x0, Y: y0}}
	visited[y0][x0] = true
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		region = append(region, p)
		if p.X == 0 || p.X == Size-1 || p.Y == 0 || p.Y == Size-1 {
			touchesEdge = true
		}
		for _, d := range dirs4 {
			nx, ny := p.X+d.X, p.Y+d.Y
			if !inBounds(nx, ny) || visited[ny][nx] {
				continue
			}
			if !isMember(b, nx, ny, wallColor) {
				continue
			}
			visited[ny][nx] = true
			stack = append(stack, image.Pt(nx, ny))
		}
	}
	return region, touchesEdge
}

// Enclosure scans the whole board for newly-enclosed regions (tested
// independently for both Black-walled and White-walled interpretations) and
// applies their effects in place: any opponent pieces trapped inside are
// removed from the board (returned to hand by the caller, using the returned
// CapturedInfo) and every cell of the enclosed region — captured or not — is
// permanently marked Sealed, so nothing can ever be placed there again. It
// returns the list of newly-sealed cells (for a UI to briefly outline) and
// the list of captures (for the caller to credit back to the owning side's
// hand).
//
// A region is only considered once it fails to reach the board edge (see
// floodMember) — this is the one rule that is easy to get backwards, so it
// is enforced entirely by the traversal's own membership test rather than as
// a separate post-hoc check.
//
// Because a region under wallColor's test only ever includes Empty cells and
// wallColor's OPPONENT's cells (never wallColor's own pieces, which act as
// walls), a single pass over the pre-mutation board is sufficient: removing
// captured pieces afterwards cannot create any new connectivity that wasn't
// already present during the flood (a captured cell was already a traversable
// "member" before its piece was removed).
func Enclosure(b *Board) (sealed []image.Point, captured []CapturedInfo) {
	for _, wallColor := range [2]Cell{Black, White} {
		opp := wallColor.Opponent()
		var visited [Size][Size]bool
		for y := 0; y < Size; y++ {
			for x := 0; x < Size; x++ {
				if visited[y][x] || !isMember(b, x, y, wallColor) {
					continue
				}
				region, touchesEdge := floodMember(b, x, y, wallColor, &visited)
				if touchesEdge {
					continue
				}
				sealed = append(sealed, region...)
				byPiece := map[int][]image.Point{}
				for _, p := range region {
					if b.Owner[p.Y][p.X] == opp {
						pid := int(b.PieceID[p.Y][p.X])
						byPiece[pid] = append(byPiece[pid], p)
					}
				}
				for pid, cells := range byPiece {
					captured = append(captured, CapturedInfo{Owner: opp, PieceID: pid, Cells: cells})
				}
			}
		}
	}

	for _, p := range sealed {
		b.Sealed[p.Y][p.X] = true
	}
	for _, cp := range captured {
		for _, p := range cp.Cells {
			b.Owner[p.Y][p.X] = Empty
			b.PieceID[p.Y][p.X] = -1
		}
	}
	return sealed, captured
}
