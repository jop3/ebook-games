package game

import (
	"image"
	"sort"
)

// Placement is one candidate (or chosen) location for a side's L-piece: an
// orientation index into LOrientations, the anchor (top-left of that
// orientation's bounding box), and the 4 absolute board cells it covers
// (sorted into the same canonical order normalizeShape uses, so two
// Placements covering the same cells compare equal via Cells).
type Placement struct {
	Orient int
	Anchor image.Point
	Cells  [4]image.Point
}

// sortCells sorts p in place into canonical (y, then x) order.
func sortCells(p *[4]image.Point) {
	sort.Slice(p[:], func(i, j int) bool {
		if p[i].Y != p[j].Y {
			return p[i].Y < p[j].Y
		}
		return p[i].X < p[j].X
	})
}

// currentLCells returns the 4 cells currently occupied by side's L-piece, in
// canonical order. Since side's L is the only piece of that color on the
// board, this is simply every cell holding that color — no separate
// placement bookkeeping is needed.
func currentLCells(b *Board, side Side) [4]image.Point {
	var out [4]image.Point
	cells := b.occupiedCells(side)
	for i := 0; i < 4 && i < len(cells); i++ {
		out[i] = cells[i]
	}
	sortCells(&out)
	return out
}

// LegalLPlacements returns every legal new placement of side's L-piece: all
// 8 orientations at every anchor position that (a) stays on the board, (b)
// doesn't overlap the opponent's L or either neutral piece (side's own
// current L cells don't block, since it is lifted before being placed), and
// (c) differs from side's current position (comparing the actual set of 4
// cells covered, not the orientation index — a placement that happens to
// cover the same 4 cells the piece already occupies is not "different" even
// if reached via a different orientation label).
func LegalLPlacements(b Board, side Side) []Placement {
	blocked := func(x, y int) bool {
		c := b.At(x, y)
		return c != Empty && c != side
	}
	cur := currentLCells(&b, side)

	var out []Placement
	for orient, shape := range LOrientations {
		w, h := shapeDims(shape)
		for ay := 0; ay <= Size-h; ay++ {
			for ax := 0; ax <= Size-w; ax++ {
				anchor := image.Pt(ax, ay)
				var cells [4]image.Point
				ok := true
				for i, off := range shape {
					p := image.Pt(ax+off.X, ay+off.Y)
					if blocked(p.X, p.Y) {
						ok = false
						break
					}
					cells[i] = p
				}
				if !ok {
					continue
				}
				sortCells(&cells)
				if cells == cur {
					continue
				}
				out = append(out, Placement{Orient: orient, Anchor: anchor, Cells: cells})
			}
		}
	}
	return out
}

// IsLegalLPlacement reports whether pl is currently a legal placement of
// side's L-piece (used to validate a UI-chosen placement without having to
// scan the whole legal list by hand).
func IsLegalLPlacement(b Board, side Side, pl Placement) bool {
	for _, cand := range LegalLPlacements(b, side) {
		if cand.Cells == pl.Cells {
			return true
		}
	}
	return false
}

// ApplyLPlacement returns a copy of b with side's L-piece lifted from its
// current cells and placed on pl.Cells. It does not check legality — callers
// should have already validated pl via LegalLPlacements/IsLegalLPlacement.
func ApplyLPlacement(b Board, side Side, pl Placement) Board {
	nb := b
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if nb[y][x] == side {
				nb[y][x] = Empty
			}
		}
	}
	for _, c := range pl.Cells {
		nb.set(c.X, c.Y, side)
	}
	return nb
}

// NeutralMove is a relocation of one of the two neutral pieces.
type NeutralMove struct {
	From, To image.Point
}

// LegalNeutralMoves returns every legal neutral-piece move: each neutral
// piece's current cell paired with every empty cell on the board. Moving a
// neutral piece is always optional — a full turn may skip this step
// entirely (see GameState.SkipNeutral).
func LegalNeutralMoves(b Board) []NeutralMove {
	var out []NeutralMove
	neutrals := b.occupiedCells(Neutral)
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) != Empty {
				continue
			}
			for _, from := range neutrals {
				out = append(out, NeutralMove{From: from, To: image.Pt(x, y)})
			}
		}
	}
	return out
}

// IsLegalNeutralMove reports whether m is currently legal: From must hold a
// neutral piece and To must be empty (and, trivially, different from From
// since To is required to be empty).
func IsLegalNeutralMove(b Board, m NeutralMove) bool {
	return b.At(m.From.X, m.From.Y) == Neutral && b.At(m.To.X, m.To.Y) == Empty
}

// ApplyNeutralMove returns a copy of b with the neutral piece at m.From
// relocated to m.To. It does not check legality.
func ApplyNeutralMove(b Board, m NeutralMove) Board {
	nb := b
	nb.set(m.From.X, m.From.Y, Empty)
	nb.set(m.To.X, m.To.Y, Neutral)
	return nb
}
