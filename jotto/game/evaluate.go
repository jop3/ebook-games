package game

// Status is the per-position feedback for one letter of a guess.
type Status int

const (
	// Absent: the letter does not appear in the secret (or all of its
	// occurrences were already consumed by better-placed/earlier tiles).
	Absent Status = iota
	// Present: the letter appears in the secret but at a different position.
	Present
	// Correct: the letter is at the right position.
	Correct
)

// Evaluate compares a guess against the secret and returns a per-position
// status slice, using Wordle-style duplicate handling: each occurrence of a
// letter in the secret can be "consumed" by at most one guess tile. Correct
// (right place) tiles are matched first, then remaining tiles claim leftover
// occurrences left-to-right as Present, else Absent.
//
// Both guess and secret are treated as rune slices so Swedish å/ä/ö count as
// single letters. If their rune lengths differ, a nil slice is returned.
func Evaluate(guess, secret string) []Status {
	g := []rune(guess)
	s := []rune(secret)
	if len(g) != len(s) {
		return nil
	}
	res := make([]Status, len(g))

	// Count remaining letters available in the secret after removing exact
	// (Correct) matches.
	remaining := make(map[rune]int, len(s))
	for i := range s {
		if g[i] == s[i] {
			res[i] = Correct
		} else {
			remaining[s[i]]++
		}
	}

	// Second pass: non-correct tiles claim a remaining occurrence if any.
	for i := range g {
		if res[i] == Correct {
			continue
		}
		if remaining[g[i]] > 0 {
			remaining[g[i]]--
			res[i] = Present
		} else {
			res[i] = Absent
		}
	}
	return res
}

// AllCorrect reports whether every status in the slice is Correct (i.e. the
// guess exactly matched the secret).
func AllCorrect(st []Status) bool {
	if len(st) == 0 {
		return false
	}
	for _, s := range st {
		if s != Correct {
			return false
		}
	}
	return true
}
