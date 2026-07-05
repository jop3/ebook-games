package game

import "sort"

// ai.go: a compact alpha-beta search for the movement phase (BestMove), plus
// a simple deterministic 1-ply heuristic for placement (AIPlacement) — the
// same split ringar/game/ai.go makes (a cheap heuristic for placement, real
// search only for the tactically interesting phase), since placement mostly
// just builds the shared mosaic and has comparatively little to calculate
// ahead for, while every movement-phase move can immediately complete or
// block a shape.
//
// Six's branching factor in the movement phase is wide — each of a side's
// ~21 on-board tiles typically has several legal frontier destinations, and
// even checking ONE candidate destination costs a full connectivity
// flood-fill — so per the spec this search prunes to "frontier cells near
// existing tiles" explicitly, in two stages: aiCandidateMoves caps which of
// the mover's own tiles are even considered (aiFromWidth) AND how many
// destinations each considers (aiToWidth), using a free structural score
// (same-side neighbour count) to rank candidates BEFORE paying for the
// expensive Connected() check — not merely capping the final move list
// after generating it in full. GameState/UI/tests still use the exhaustive,
// always-correct MoveMoves (board.go); only the AI's own search uses this
// bounded approximation, so a "casual" AI is genuinely faster without ever
// being allowed to attempt an illegal move (every move it proposes is
// re-validated by GameState.MoveTile exactly like a human's tap). See
// ai_test.go for measured wall-clock timing at each depth — the AI is
// shipped as an explicitly CASUAL "Mot dator" opponent; hot-seat two-player
// is the primary experience, exactly as with every other game in this
// batch that says so.

const (
	negInf   = -1 << 30
	posInf   = 1 << 30
	winScore = 1 << 20
)

// aiFromWidth/aiToWidth cap the search's move generation (see the package
// doc comment above).
//
// Measured on this dev machine (see ai_test.go's TestAIWallClockTiming) on a
// full, connected 42-tile board — the worst case for branching, since the
// movement phase always starts from a completely full board: with these
// widths, Lätt (depth 1) takes ~26ms, Medel (depth 2) ~38ms, and Svår
// (depth 3) ~162ms. An earlier, wider attempt (aiFromWidth=aiToWidth=8) let
// depth 3 spike to ~5.6s — clearly not "comfortably fast" once you account
// for the PocketBook's ARM CPU being considerably slower than this dev
// machine (the same conclusion ringar/game/ai.go's AIDepth comment reached
// about ITS rejected depth 3) — so the widths were tightened here instead of
// dropping Svår to depth 2, keeping all 3 menu difficulties meaningfully
// different while staying fast.
const (
	aiFromWidth = 3
	aiToWidth   = 4
)

// progressWeight scores how many of a shape instance's 6 cells one side has
// already claimed (the rest empty) — steeply increasing near completion, so
// a 5-of-6 threat dominates the evaluation the way it should.
func progressWeight(n int) int {
	switch n {
	case 5:
		return 500
	case 4:
		return 80
	case 3:
		return 20
	case 2:
		return 5
	case 1:
		return 1
	default:
		return 0
	}
}

// progress returns side's best shape progress: the largest progressWeight
// across every shape instance side could still complete (i.e. every cell is
// either side's own or still empty — none occupied by the opponent).
func progress(tiles map[Hex]Side, side Side) int {
	opp := side.Opponent()
	best := 0
	for _, inst := range allShapeInstances {
		count := 0
		blocked := false
		for _, c := range inst.Cells {
			switch tiles[c] {
			case side:
				count++
			case opp:
				blocked = true
			}
			if blocked {
				break
			}
		}
		if blocked {
			continue
		}
		if w := progressWeight(count); w > best {
			best = w
		}
	}
	return best
}

// blockThreats counts side's shape instances that are exactly one tile from
// completion (5 of 6 claimed, the 6th still empty) — an explicit "block
// this now" term layered on top of progress's already-steep 5-of-6
// weighting, so that TWO simultaneous 5-of-6 threats (an unblockable fork)
// are penalized much more than a single one, rather than just once via the
// plain max-based progress subtraction.
func blockThreats(tiles map[Hex]Side, side Side) int {
	opp := side.Opponent()
	n := 0
	for _, inst := range allShapeInstances {
		count, blocked := 0, false
		for _, c := range inst.Cells {
			switch tiles[c] {
			case side:
				count++
			case opp:
				blocked = true
			}
		}
		if !blocked && count == 5 {
			n++
		}
	}
	return n
}

// eval scores tiles from side's perspective: side's own best shape progress
// minus the opponent's, minus an extra penalty per unblocked opponent
// 5-of-6 threat.
func eval(tiles map[Hex]Side, side Side) int {
	opp := side.Opponent()
	return progress(tiles, side) - progress(tiles, opp) - 400*blockThreats(tiles, opp)
}

// quickScore cheaply favors positions that already touch more of side's own
// tiles (building compactly toward a shape) over sparse ones — deliberately
// no shape-instance scan here, just neighbour counting, so ordering (and
// capping the width of) candidates stays effectively free.
func quickScore(tiles map[Hex]Side, side Side, p Hex) int {
	n := 0
	for _, d := range Directions {
		if tiles[p.Add(d)] == side {
			n++
		}
	}
	return n
}

func cloneTiles(tiles map[Hex]Side) map[Hex]Side {
	out := make(map[Hex]Side, len(tiles)+1)
	for k, v := range tiles {
		out[k] = v
	}
	return out
}

// aiCandidateMoves is the AI's own bounded approximation of MoveMoves — see
// the package doc comment for why it caps both which tiles are considered
// as `from` and how many destinations each considers, before ever paying
// for a Connected() flood-fill. Every move it returns IS still fully legal
// (each is confirmed connected, or advanced-mode-legal, exactly like
// MoveMoves would); it just doesn't promise to enumerate every legal move.
func aiCandidateMoves(tiles map[Hex]Side, side Side, advanced bool) []Move {
	var froms []Hex
	for p, s := range tiles {
		if s == side {
			froms = append(froms, p)
		}
	}
	sort.Slice(froms, func(i, j int) bool { return LessHex(froms[i], froms[j]) })
	// Tiles with FEWER same-side neighbours are less structurally
	// load-bearing (less likely to be the sole link holding two clumps
	// together) and more likely to have a useful destination — prioritize
	// those.
	sort.SliceStable(froms, func(i, j int) bool {
		return quickScore(tiles, side, froms[i]) < quickScore(tiles, side, froms[j])
	})
	if len(froms) > aiFromWidth {
		froms = froms[:aiFromWidth]
	}

	var moves []Move
	for _, from := range froms {
		rest := withoutTile(tiles, from)
		dests := PlaceMoves(rest)
		sort.SliceStable(dests, func(i, j int) bool {
			return quickScore(rest, side, dests[i]) > quickScore(rest, side, dests[j])
		})
		if len(dests) > aiToWidth {
			dests = dests[:aiToWidth]
		}
		for _, to := range dests {
			if to == from {
				continue
			}
			candidate := cloneTiles(rest)
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

// applyMoveSim applies m for side on a COPY of tiles (never mutating the
// input), handling the advanced-rule disconnect/strand exactly like
// GameState.MoveTile, and reports whether the move ended the game (a shape
// win for side, or an advanced-rule <=5 elimination for either side) along
// with the winner if so.
func applyMoveSim(tiles map[Hex]Side, m Move, side Side, advanced bool, remaining map[Side]int) (next map[Hex]Side, done bool, winner Side) {
	candidate := withoutTile(tiles, m.From)
	candidate[m.To] = side
	if !Connected(candidate) {
		comps := Components(candidate)
		for _, comp := range comps {
			keep := false
			for _, c := range comp {
				if c == m.To {
					keep = true
					break
				}
			}
			if keep {
				continue
			}
			for _, c := range comp {
				delete(candidate, c)
			}
		}
		opp := side.Opponent()
		tilesLeft := func(s Side) int {
			n := remaining[s]
			for _, v := range candidate {
				if v == s {
					n++
				}
			}
			return n
		}
		if tilesLeft(opp) <= 5 {
			return candidate, true, side
		}
		if tilesLeft(side) <= 5 {
			return candidate, true, opp
		}
	}
	if kind, _ := HasShape(candidate, side); kind != ShapeNone {
		return candidate, true, side
	}
	return candidate, false, None
}

// negamax searches the movement phase to the given depth from toMove's
// perspective. remaining is only consulted for the advanced-rule <=5 check
// (it never changes during the movement phase itself).
func negamax(tiles map[Hex]Side, remaining map[Side]int, toMove Side, advanced bool, depth, alpha, beta int) int {
	moves := aiCandidateMoves(tiles, toMove, advanced)
	if len(moves) == 0 {
		return -winScore - depth // boxed in entirely: treated as a loss for toMove
	}
	if depth == 0 {
		return eval(tiles, toMove)
	}
	best := negInf
	for _, m := range moves {
		next, done, winner := applyMoveSim(tiles, m, toMove, advanced, remaining)
		var score int
		if done {
			if winner == toMove {
				score = winScore + depth
			} else {
				score = -winScore - depth
			}
		} else {
			score = -negamax(next, remaining, toMove.Opponent(), advanced, depth-1, -beta, -alpha)
		}
		if score > best {
			best = score
		}
		if best > alpha {
			alpha = best
		}
		if alpha >= beta {
			break
		}
	}
	return best
}

// BestMove returns the AI's chosen tile move for the state's side to move.
// ok is false only if that side has no legal move at all (checked against
// the exhaustive, always-correct MoveMoves — not the capped search
// generator — so this is never wrong about whether a move exists).
func BestMove(s *GameState) (Move, bool) {
	side := s.Turn
	if len(MoveMoves(s.Board.Tiles, side, s.Advanced)) == 0 {
		return Move{}, false
	}
	moves := aiCandidateMoves(s.Board.Tiles, side, s.Advanced)
	if len(moves) == 0 {
		// The capped generator missed every real legal move (only
		// possible if the cluster is unusually thin); fall back to the
		// exhaustive generator for this one decision.
		moves = MoveMoves(s.Board.Tiles, side, s.Advanced)
	}
	depth := s.AIDepth
	if depth < 1 {
		depth = 1
	}
	opp := side.Opponent()
	best := negInf
	bestMove := moves[0]
	alpha, beta := negInf, posInf
	for _, m := range moves {
		next, done, winner := applyMoveSim(s.Board.Tiles, m, side, s.Advanced, s.Remaining)
		var score int
		if done {
			if winner == side {
				score = winScore + depth
			} else {
				score = -winScore - depth
			}
		} else {
			score = -negamax(next, s.Remaining, opp, s.Advanced, depth-1, -beta, -alpha)
		}
		if score > best {
			best, bestMove = score, m
		}
		if best > alpha {
			alpha = best
		}
	}
	return bestMove, true
}

// AIPlacement chooses where the AI places its next tile during
// PhasePlacement: a 1-ply heuristic (maximize eval — this side's own best
// progress minus the opponent's, minus opponent threats — immediately after
// the placement), with ties broken by the most central cell for determinism
// and maximum future flexibility. Unlike BestMove, this is deliberately not
// a multi-ply search: placement mostly just grows the shared mosaic and has
// comparatively little to calculate ahead for (mirroring
// ringar/game/ai.go's AIPlacement, which is likewise a simple heuristic
// while its BestMove does the real search).
func AIPlacement(s *GameState) (Hex, bool) {
	side := s.Turn
	candidates := PlaceMoves(s.Board.Tiles)
	if len(candidates) == 0 {
		return Hex{}, false
	}
	best := candidates[0]
	bestScore := negInf
	for _, p := range candidates {
		tiles2 := cloneTiles(s.Board.Tiles)
		tiles2[p] = side
		if kind, _ := HasShape(tiles2, side); kind != ShapeNone {
			return p, true // an immediate win is always best
		}
		sc := eval(tiles2, side)
		if sc > bestScore || (sc == bestScore && CubeRadius(p) < CubeRadius(best)) {
			bestScore, best = sc, p
		}
	}
	return best, true
}
