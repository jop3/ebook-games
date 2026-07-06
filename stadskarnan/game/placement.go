package game

import "image"

// Placement is one concrete way to lay a piece on the board: the absolute
// cells it would cover, and the Anchor cell (its top-left-most cell) that the
// UI highlights and the player taps to commit it.
type Placement struct {
	Cells  []image.Point
	Anchor image.Point
}

// legalOriginsForOriented returns every legal Placement of the given already-
// normalized oriented shape (a single element of Orientations(piece.Cells)).
func legalPlacementsForOriented(b *Board, oriented []Offset) []Placement {
	maxX, maxY := BoundingBox(oriented)
	anchorRel := Anchor(oriented)
	var out []Placement
	for oy := 0; oy+maxY < Size; oy++ {
		for ox := 0; ox+maxX < Size; ox++ {
			ok := true
			abs := make([]image.Point, len(oriented))
			for i, c := range oriented {
				x, y := ox+c[0], oy+c[1]
				if b.Owner[y][x] != Empty || b.Sealed[y][x] {
					ok = false
					break
				}
				abs[i] = image.Pt(x, y)
			}
			if !ok {
				continue
			}
			out = append(out, Placement{
				Cells:  abs,
				Anchor: image.Pt(ox+anchorRel[0], oy+anchorRel[1]),
			})
		}
	}
	return out
}

// hasLegalPlacementForOriented reports whether the given oriented shape has
// at least one legal placement, without allocating the full list — used by
// the pass/end-of-game check, which only needs a yes/no answer.
func hasLegalPlacementForOriented(b *Board, oriented []Offset) bool {
	maxX, maxY := BoundingBox(oriented)
	for oy := 0; oy+maxY < Size; oy++ {
		for ox := 0; ox+maxX < Size; ox++ {
			ok := true
			for _, c := range oriented {
				x, y := ox+c[0], oy+c[1]
				if b.Owner[y][x] != Empty || b.Sealed[y][x] {
					ok = false
					break
				}
			}
			if ok {
				return true
			}
		}
	}
	return false
}

// LegalPlacementsForPiece returns every legal Placement of pieceDef, over all
// of its distinct orientations, on board b.
func LegalPlacementsForPiece(b *Board, pieceDef PieceDef) []Placement {
	var out []Placement
	for _, o := range Orientations(pieceDef.Cells) {
		out = append(out, legalPlacementsForOriented(b, o)...)
	}
	return out
}

// LegalPlacementsForOrientation returns every legal Placement of piece id
// pieceID's orientation number orientIdx (as indexed into
// Orientations(Pieces[pieceID].Cells)).
func LegalPlacementsForOrientation(b *Board, pieceID, orientIdx int) []Placement {
	orients := Orientations(Pieces[pieceID].Cells)
	if orientIdx < 0 || orientIdx >= len(orients) {
		return nil
	}
	return legalPlacementsForOriented(b, orients[orientIdx])
}

// HasAnyLegalPlacement reports whether side has at least one legal placement
// for any piece still available in hand.
func (b *Board) HasAnyLegalPlacement(hand *Hand) bool {
	for id := 0; id < NumPieces; id++ {
		if !hand[id] {
			continue
		}
		for _, o := range Orientations(Pieces[id].Cells) {
			if hasLegalPlacementForOriented(b, o) {
				return true
			}
		}
	}
	return false
}

// LegalCathedralPlacements returns every legal Placement of the neutral
// Cathedral shape (only meaningful before it has been placed, on an
// otherwise-arbitrary board — in practice called only while the board is
// still empty).
func LegalCathedralPlacements(b *Board) []Placement {
	return LegalPlacementsForPiece(b, CathedralShape)
}
