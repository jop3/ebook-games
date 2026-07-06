package game

import "image"

// AI heuristic: Patchwork is perfect information (both boards and the whole
// patch queue are visible to everyone), so no hidden-information modelling
// or deep search is needed for a reasonable opponent. Each turn the AI
// scores every legal (patch, orientation, position) among the 3 buyable
// candidates by income-per-button-spent (with a time-cost and size
// adjustment) minus a fragmentation penalty for placements that wall off
// small unfillable pockets of empty board, and compares that against the
// baseline value of simply advancing for buttons — then takes whichever
// scores highest. No lookahead beyond this single decision is required
// because the branching factor (3 candidate patches x up to 8 orientations
// x board positions) is already fully enumerated for the immediate choice.

// aiChoice is one candidate action for the AI to take.
type aiChoice struct {
	isAdvance bool
	offset    int // which of NextThree() (only if !isAdvance)
	orientIdx int
	anchor    image.Point
	score     float64
}

// fragmentationPenalty estimates how much placing newCells on board b would
// wall off small, hard-to-fill pockets of empty board: it counts (with
// weight) empty cells that would end up with 3 or 4 of their orthogonal
// neighbors filled, since those are the squares most likely to end up
// stranded (too small for any remaining patch to reach) by the time the
// game ends — and every stranded empty square costs 2 points at final
// scoring.
func fragmentationPenalty(b *Board, newCells []image.Point) float64 {
	filled := b.Filled // array value-copy
	for _, c := range newCells {
		filled[c.Y][c.X] = true
	}
	penalty := 0.0
	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			if filled[y][x] {
				continue
			}
			n := 0
			if x > 0 && filled[y][x-1] {
				n++
			}
			if x < BoardSize-1 && filled[y][x+1] {
				n++
			}
			if y > 0 && filled[y-1][x] {
				n++
			}
			if y < BoardSize-1 && filled[y+1][x] {
				n++
			}
			switch n {
			case 4:
				penalty += 3
			case 3:
				penalty += 1
			}
		}
	}
	return penalty
}

// aiChoose picks player's best action for the current turn: buy one of the
// 3 candidate patches (in whichever legal orientation/position scores
// highest) or advance for buttons, whichever scores higher.
func aiChoose(s *GameState, player int) aiChoice {
	three := s.NextThree()
	other := 1 - player
	delta := clampTrack(s.Marker[other]+1) - s.Marker[player]
	if delta < 0 {
		delta = 0
	}
	// Advancing gains `delta` buttons outright — that's its baseline value.
	best := aiChoice{isAdvance: true, score: float64(delta)}

	board := &s.Boards[player]
	for offset, patch := range three {
		if patch.Cost > s.Buttons[player] {
			continue
		}
		for oi, o := range Orientations(patch.Cells) {
			for _, pl := range legalPlacementsForOriented(board, o) {
				frag := fragmentationPenalty(board, pl.Cells)
				// Buying a patch costs `Cost` buttons but fills `Size`
				// board cells that would otherwise likely sit empty at
				// -2 each in FinalScore, so size is weighted like that
				// avoided penalty; income is weighted as a rough proxy
				// for the buttons it'll pay out over the rest of the
				// game (crossing several income squares); time-cost is
				// a mild tempo penalty (it hands the opponent more
				// turns before you can act again).
				value := float64(patch.Income)*3.0 -
					float64(patch.Cost) -
					float64(patch.TimeCost)*0.5 +
					float64(patch.Size())*2.0 -
					frag*2.0
				if value > best.score {
					best = aiChoice{
						isAdvance: false,
						offset:    offset,
						orientIdx: oi,
						anchor:    pl.Anchor,
						score:     value,
					}
				}
			}
		}
	}
	return best
}

// findEmptyCell returns any empty cell on b (preferring the one with the
// most already-filled orthogonal neighbors, to consolidate rather than
// scatter free patches), and whether the board has room at all.
func findEmptyCell(b *Board) (image.Point, bool) {
	best := image.Point{}
	bestN := -1
	found := false
	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			if b.Filled[y][x] {
				continue
			}
			n := 0
			if x > 0 && b.Filled[y][x-1] {
				n++
			}
			if x < BoardSize-1 && b.Filled[y][x+1] {
				n++
			}
			if y > 0 && b.Filled[y-1][x] {
				n++
			}
			if y < BoardSize-1 && b.Filled[y+1][x] {
				n++
			}
			if n > bestN {
				bestN = n
				best = image.Pt(x, y)
				found = true
			}
		}
	}
	return best, found
}

// StepAI performs exactly one atomic AI action (resolving a pending
// free-patch placement takes priority over a normal turn action) for
// player 1, the AI by convention. Returns whether it changed state, so the
// caller's Draw-then-StepAI loop (mirroring this repo's other AI games)
// knows whether to keep stepping. A false return with AITurn() still true
// would indicate a logic bug (no legal action) rather than a normal stop.
func (s *GameState) StepAI() bool {
	const p = 1
	if s.Phase != PhasePlaying {
		return false
	}
	if s.Pending[p] > 0 {
		board := &s.Boards[p]
		pt, ok := findEmptyCell(board)
		if !ok {
			// Board is completely full — nowhere to put the free patch;
			// discard the obligation rather than soft-locking the game.
			s.Pending[p] = 0
			s.maybeFinish()
			return true
		}
		return s.PlaceFreePatch(p, pt)
	}
	if s.ActingPlayer() != p {
		return false
	}
	choice := aiChoose(s, p)
	if choice.isAdvance {
		return s.Advance(p)
	}
	return s.BuyPatch(p, choice.offset, choice.orientIdx, choice.anchor)
}
