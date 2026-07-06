package game

// TrackEnd is the last square of the linear time track (rendered on-screen
// as a horizontal scrolling strip per the spec — a "circular" track is only
// used for the patch queue/neutral-token bookkeeping below, never drawn as
// an actual circle).
const TrackEnd = 53

// IncomeSquares are the (invented) time-track positions that pay out button
// income when a marker crosses or lands on them. Unevenly spaced on
// purpose, ending on the final space (53) so a marker finishing the track
// always collects its last payout. Original values, not the real game's.
var IncomeSquares = []int{4, 9, 15, 20, 26, 31, 37, 42, 48, 53}

// SpecialPatchPositions are the (invented) fixed track positions holding a
// free 1x1 scoring patch; first marker to reach/pass a position claims it.
// Disjoint from IncomeSquares by construction.
var SpecialPatchPositions = []int{2, 7, 13, 18, 24, 29, 35, 40}

// NextActor implements the "furthest behind acts next" turn-order rule: the
// player whose marker is at the smaller position moves; ties favor player 0
// (i.e. "player 1" in 1-based rules text).
func NextActor(marker0, marker1 int) int {
	if marker0 <= marker1 {
		return 0
	}
	return 1
}

// crossedCount returns how many of the sorted positions lie in the
// half-open-below/closed-above range (oldPos, newPos] — i.e. were passed
// over or landed on exactly by a marker moving from oldPos to newPos
// (newPos >= oldPos). This is the "crossing, not just landing on" rule: a
// single multi-square jump can pass several marked squares at once.
func crossedCount(oldPos, newPos int, positions []int) int {
	if newPos <= oldPos {
		return 0
	}
	n := 0
	for _, p := range positions {
		if p > oldPos && p <= newPos {
			n++
		}
	}
	return n
}

// crossedSpecialIndices returns the indices into SpecialPatchPositions that
// a marker moving oldPos->newPos crosses AND that are not yet claimed.
func crossedSpecialIndices(oldPos, newPos int, positions []int, claimed []bool) []int {
	if newPos <= oldPos {
		return nil
	}
	var out []int
	for i, p := range positions {
		if !claimed[i] && p > oldPos && p <= newPos {
			out = append(out, i)
		}
	}
	return out
}

func clampTrack(pos int) int {
	if pos > TrackEnd {
		return TrackEnd
	}
	if pos < 0 {
		return 0
	}
	return pos
}
