package game

// ai.go: a perfect-play AI via exhaustive memoized minimax over the
// staircase representation. Chomp's state space (monotone partitions that
// fit inside the starting rows x cols rectangle) is tiny for any grid up to
// the largest offered here (6x7) — at most a few thousand distinct
// staircases — so full minimax is instant. Chomp ships an UNBEATABLE AI, not
// merely a strong one, mirroring nim's Sprague-Grundy solver; say so on the
// menu/rules screen.
//
// The search only ever considers the poisoned cell (0,0) as an absolute last
// resort: taking it always loses immediately and is never a winning try, so
// pruning it out of the search keeps the recursion small and the reasoning
// simple (see solve below).

import "strconv"

// key encodes a State as a comma-joined string for memoization. Grids here
// are tiny (<= a few dozen cells) so a simple string key is plenty fast.
func key(s State) string {
	b := make([]byte, 0, len(s)*3)
	for i, n := range s {
		if i > 0 {
			b = append(b, ',')
		}
		b = strconv.AppendInt(b, int64(n), 10)
	}
	return string(b)
}

// WinningForMover reports whether the player about to move in s can force a
// win with perfect play from both sides. A state holding only the poisoned
// cell (Total()==1) is always a loss for the mover, since their one and only
// legal move IS the poison.
func WinningForMover(s State) bool {
	memo := make(map[string]bool)
	return solve(s, memo)
}

// solve is the memoized recursive minimax: s is a WIN for the player to move
// iff some non-suicidal move leaves the opponent in a state that is a LOSS
// for them. If no such move exists (every non-poison move hands the opponent
// a winning state, or none exist because only the poison itself remains),
// the mover cannot avoid defeat.
func solve(s State, memo map[string]bool) bool {
	k := key(s)
	if w, ok := memo[k]; ok {
		return w
	}
	// Guard against infinite recursion on a corrupt/cyclic call before the
	// result is memoized (Apply always shrinks the board, so this never
	// actually recurses on s itself, but memoizing before the loop keeps the
	// function total even if that invariant were ever violated).
	memo[k] = false
	result := false
	for _, m := range s.LegalMoves() {
		if m.IsPoison() {
			continue // never a winning try: taking the poison always loses.
		}
		if !solve(s.Apply(m), memo) {
			result = true
			break
		}
	}
	memo[k] = result
	return result
}

// BestMove returns a perfect-play move for the player to move in s. ok is
// false only when s is already completely empty (no move possible — should
// not happen during normal play, since the game ends the instant someone
// takes the poison). When only the poisoned cell remains, BestMove returns
// it — it is the only legal move, a forced loss. Otherwise, among winning
// states it returns a move that leaves the opponent in a loss; among losing
// states (the mover cannot avoid eventual defeat against a perfect
// opponent) it returns some non-suicidal legal move rather than resigning
// outright — the same "keep playing, don't hand over the win early" honesty
// as nim's fallback move.
func BestMove(s State) (Move, bool) {
	moves := s.LegalMoves()
	if len(moves) == 0 {
		return Move{}, false
	}
	var nonPoison []Move
	for _, m := range moves {
		if !m.IsPoison() {
			nonPoison = append(nonPoison, m)
		}
	}
	if len(nonPoison) == 0 {
		return moves[0], true // only the poisoned cell is left; forced.
	}
	memo := make(map[string]bool)
	for _, m := range nonPoison {
		if !solve(s.Apply(m), memo) {
			return m, true // leaves the opponent in a losing state.
		}
	}
	return nonPoison[0], true // losing anyway; make some legal, non-suicidal move.
}
