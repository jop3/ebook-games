package game

// EliminatedSide returns the side that has lost every piece of some ONE type
// (Tzaar, Tzarra, or Tott) — checked in TYPE units (TypeCount), NOT total
// piece count. This is TZAAR's actual win condition: a side that still holds
// plenty of total pieces, but zero of (say) Tzaar, has already lost — the
// classic misimplementation is to check total pieces == 0 instead, which
// this function deliberately does not do (see winner_test.go).
//
// This is a pure board scan and does not need to know whose turn it is. It
// is only meaningful once the setup phase has ended — during setup, most
// types legitimately read 0 for a side that simply hasn't placed one yet, so
// GameState only calls this after a play-phase move, never after a
// placement.
func EliminatedSide(b *Board) Side {
	for _, side := range [2]Side{Black, White} {
		total := 0
		for _, t := range AllTypes {
			total += b.TypeCount(side, t)
		}
		if total == 0 {
			// Hasn't placed anything yet (e.g. a fresh/empty board before or
			// during setup) — that's not elimination, just "hasn't started."
			continue
		}
		for _, t := range AllTypes {
			if b.TypeCount(side, t) == 0 {
				return side
			}
		}
	}
	return None
}
