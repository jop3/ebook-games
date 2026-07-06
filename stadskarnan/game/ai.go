package game

import (
	"image"
	"sort"
)

// ai.go: a heuristic practice-strength AI, not a strong search-based
// opponent. Stadskärnan's placement branching (up to 13 pieces x up to 8
// orientations x up to ~100 board positions) is far too large for full-width
// alpha-beta, so instead of searching deeply, BestPlacement:
//
//  1. Enumerates every legal placement for the AI's remaining pieces and
//     scores each by the resulting hand-size gap (RemainingSquares(opp) -
//     RemainingSquares(self)) after simulating the placement and its
//     enclosure effects — this is exactly the game's own win condition, so
//     a move that captures opponent pieces (returning them to the
//     opponent's hand) is automatically rewarded, and a move that gets the
//     AI's own piece captured back out is automatically penalized.
//  2. Keeps only the top-K candidates by that depth-1 score (bounding the
//     more expensive step below to a handful of moves, per the "restrict to
//     top-K candidates" guidance — this is what keeps the AI cheap enough
//     for the device).
//  3. For each of those, simulates the single best reply available to the
//     opponent (same depth-1 scoring, from the opponent's perspective) and
//     scores the AI's candidate by the WORST case that reply leaves the AI
//     in — a shallow (2-ply) minimax restricted to a narrow candidate set,
//     rather than a full-width search.
//
// This is honestly a fairly simple heuristic and is labelled a "practice"
// opponent on the menu/rules screen, not a strong one — getting the core
// rules (placement legality, enclosure, win condition) right matters far
// more than AI sophistication for this game.
const topK = 10

type candidate struct {
	pieceID   int
	orientIdx int
	placement Placement
	score1    int // RemainingSquares(opp) - RemainingSquares(self) right after this move
}

// simulatePlacement returns the board and hands that would result from side
// placing pieceID at cells (assumed already legal), including any enclosure
// captures. It does not mutate its inputs.
func simulatePlacement(b Board, hands [2]Hand, side Cell, pieceID int, cells []image.Point) (Board, [2]Hand) {
	nb := b // Board is plain fixed-size arrays: this is a full value copy.
	for _, c := range cells {
		nb.Owner[c.Y][c.X] = side
		nb.PieceID[c.Y][c.X] = int8(pieceID)
	}
	nh := hands
	nh[handIndex(side)][pieceID] = false
	_, captured := Enclosure(&nb)
	for _, cp := range captured {
		nh[handIndex(cp.Owner)][cp.PieceID] = true
	}
	return nb, nh
}

// gap scores hands from side's perspective: higher is better for side.
func gap(hands *[2]Hand, side Cell) int {
	opp := side.Opponent()
	return hands[handIndex(opp)].RemainingSquares() - hands[handIndex(side)].RemainingSquares()
}

// BestPlacement returns the AI's chosen placement for side. ok is false only
// if side has no legal placement at all (should not happen when called via
// StepAI, since GameState.advance only ever hands a side the turn when it
// has at least one legal move).
func BestPlacement(gs *GameState, side Cell) (pieceID, orientIdx int, anchor image.Point, ok bool) {
	hand := gs.Hand(side)

	var cands []candidate
	for id := 0; id < NumPieces; id++ {
		if !hand[id] {
			continue
		}
		for oi, o := range Orientations(Pieces[id].Cells) {
			for _, p := range legalPlacementsForOriented(&gs.Board, o) {
				_, nh := simulatePlacement(gs.Board, gs.Hands, side, id, p.Cells)
				cands = append(cands, candidate{
					pieceID:   id,
					orientIdx: oi,
					placement: p,
					score1:    gap(&nh, side),
				})
			}
		}
	}
	if len(cands) == 0 {
		return 0, 0, image.Point{}, false
	}

	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].score1 != cands[j].score1 {
			return cands[i].score1 > cands[j].score1
		}
		// Tie-break toward placing bigger pieces first (uses up the harder-
		// to-place pieces while room remains) — a small, cheap heuristic.
		return cands[i].placement.size() > cands[j].placement.size()
	})
	top := cands
	if len(top) > topK {
		top = top[:topK]
	}

	best := top[0]
	bestFinal := worstCaseAfter(gs.Board, gs.Hands, side, best)
	for _, c := range top[1:] {
		final := worstCaseAfter(gs.Board, gs.Hands, side, c)
		if final > bestFinal {
			bestFinal, best = final, c
		}
	}
	return best.pieceID, best.orientIdx, best.placement.Anchor, true
}

// size returns the number of cells this placement covers.
func (p Placement) size() int { return len(p.Cells) }

// worstCaseAfter simulates side playing candidate c, then finds the single
// best reply available to the opponent (by the same depth-1 hand-gap score,
// but from the opponent's own perspective, i.e. the reply the opponent would
// actually prefer) and returns the resulting gap from side's perspective —
// this is what a shallow 2-ply minimax looks like restricted to side's own
// top-K first move and an unrestricted (but still cheap: same depth-1
// scoring, no further recursion) opponent reply.
func worstCaseAfter(b Board, hands [2]Hand, side Cell, c candidate) int {
	opp := side.Opponent()
	nb, nh := simulatePlacement(b, hands, side, c.pieceID, c.placement.Cells)

	baseline := gap(&nh, side) // if the opponent has no reply at all (passes)
	oppHand := nh[handIndex(opp)]
	worst := baseline
	any := false
	for id := 0; id < NumPieces; id++ {
		if !oppHand[id] {
			continue
		}
		for _, o := range Orientations(Pieces[id].Cells) {
			for _, p := range legalPlacementsForOriented(&nb, o) {
				_, nh2 := simulatePlacement(nb, nh, opp, id, p.Cells)
				v := gap(&nh2, side)
				if !any || v < worst {
					worst, any = v, true
				}
			}
		}
	}
	return worst
}
