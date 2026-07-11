package main

// Knuth's five-guess Mastermind solver, in the "device guesses" mode: the
// player holds a secret code in their head and gives black/white feedback to
// each of the device's guesses. This file is PURE logic (no ink import) so it
// unit-tests without the cgo SDK. It reuses Evaluate from game.go.
//
// Algorithm (Knuth 1977):
//  1. Keep the set of all still-possible codes (initially every combination
//     allowed by the Config).
//  2. First guess: classic opening 1122 for 4/6 (indices 0,0,1,1); otherwise a
//     reasonable "two pairs" style start.
//  3. Player gives feedback (black, white).
//  4. Filter possibles: keep code c iff Evaluate(c, guess) == feedback.
//  5. Next guess by minimax: over every candidate guess g (drawn from ALL
//     codes), for each possible feedback bucket count how many possibles would
//     remain; take the worst (max). Choose g minimising that worst case. Ties
//     are broken preferring a g that is itself still possible.
//  6. Repeat until feedback is all-black (solved) or the possible set becomes
//     empty (player gave inconsistent feedback -> impossible).

// KnuthSolver drives the device's guessing for one code the player is holding.
type KnuthSolver struct {
	cfg        Config
	all        []Guess // every code allowed by cfg (candidate guess pool)
	possible   []Guess // codes still consistent with all feedback so far
	guess      Guess   // the device's current guess awaiting feedback
	turn       int     // number of guesses made so far (1-based once started)
	solved     bool
	impossible bool
}

// feedbackKey packs a Feedback into a small int for bucketing.
func feedbackKey(fb Feedback) int { return fb.Black*100 + fb.White }

// allCodes enumerates every code permitted by cfg (respecting AllowRepeat).
func allCodes(cfg Config) []Guess {
	var out []Guess
	code := make(Guess, cfg.Pegs)
	var rec func(pos int, used []bool)
	rec = func(pos int, used []bool) {
		if pos == cfg.Pegs {
			out = append(out, append(Guess(nil), code...))
			return
		}
		for c := 0; c < cfg.Colors; c++ {
			if !cfg.AllowRepeat && used[c] {
				continue
			}
			code[pos] = Color(c)
			if !cfg.AllowRepeat {
				used[c] = true
			}
			rec(pos+1, used)
			if !cfg.AllowRepeat {
				used[c] = false
			}
		}
	}
	rec(0, make([]bool, cfg.Colors))
	return out
}

// openingGuess returns Knuth's first guess for cfg. For the classic 4-peg,
// 6-color game this is 1122 (color indices 0,0,1,1). Generally we build a
// "two of each of the first colors" pattern, which is a strong generic opener;
// when repeats are disallowed we fall back to the first distinct colors.
func openingGuess(cfg Config) Guess {
	g := make(Guess, cfg.Pegs)
	if !cfg.AllowRepeat {
		for i := 0; i < cfg.Pegs; i++ {
			g[i] = Color(i % cfg.Colors)
		}
		return g
	}
	// Pairs: 0,0,1,1,2,2,... capped at the available colors.
	for i := 0; i < cfg.Pegs; i++ {
		g[i] = Color((i / 2) % cfg.Colors)
	}
	return g
}

// NewKnuthSolver builds a solver for cfg and computes the opening guess.
func NewKnuthSolver(cfg Config) *KnuthSolver {
	all := allCodes(cfg)
	// possible starts as a full copy so filtering never mutates all.
	possible := make([]Guess, len(all))
	copy(possible, all)
	s := &KnuthSolver{
		cfg:      cfg,
		all:      all,
		possible: possible,
		guess:    openingGuess(cfg),
		turn:     1,
	}
	return s
}

// Guess returns the device's current guess awaiting feedback.
func (s *KnuthSolver) CurrentGuess() Guess { return s.guess }

// Turn returns how many guesses the device has made (1-based).
func (s *KnuthSolver) Turn() int { return s.turn }

// Solved reports whether the last feedback was all-black.
func (s *KnuthSolver) Solved() bool { return s.solved }

// Impossible reports whether the player's feedback became self-contradictory
// (no remaining possible code).
func (s *KnuthSolver) Impossible() bool { return s.impossible }

// Remaining reports how many codes are still consistent with feedback so far.
func (s *KnuthSolver) Remaining() int { return len(s.possible) }

// Feedback records the player's feedback for the current guess and advances the
// solver: it filters the possible set and, if not solved/impossible, computes
// the next guess by minimax. Returns true if the game is now over (solved or
// impossible).
func (s *KnuthSolver) Feedback(fb Feedback) bool {
	if s.solved || s.impossible {
		return true
	}
	if fb.Black == s.cfg.Pegs {
		s.solved = true
		return true
	}
	// Filter possibles: keep codes that would have produced this feedback.
	kept := s.possible[:0]
	for _, c := range s.possible {
		if Evaluate(Secret(c), s.guess) == fb {
			kept = append(kept, c)
		}
	}
	s.possible = kept

	if len(s.possible) == 0 {
		s.impossible = true
		return true
	}
	if len(s.possible) == 1 {
		// Only one code left: guess it next; it must be the answer.
		s.guess = s.possible[0]
		s.turn++
		return false
	}
	s.guess = s.nextGuess()
	s.turn++
	return false
}

// nextGuess picks the minimax guess over all candidate codes. For each
// candidate g it partitions the current possible set by the feedback g would
// receive, takes the largest partition (worst case), and chooses the g with the
// smallest worst case. Ties prefer a g that is itself still possible.
func (s *KnuthSolver) nextGuess() Guess {
	possibleSet := make(map[string]bool, len(s.possible))
	for _, c := range s.possible {
		possibleSet[codeKey(c)] = true
	}

	bestWorst := 1 << 30
	var best Guess
	bestIsPossible := false

	buckets := make(map[int]int)
	for _, g := range s.all {
		for k := range buckets {
			delete(buckets, k)
		}
		worst := 0
		for _, c := range s.possible {
			k := feedbackKey(Evaluate(Secret(c), g))
			buckets[k]++
			if buckets[k] > worst {
				worst = buckets[k]
			}
		}
		gPossible := possibleSet[codeKey(g)]
		// Prefer strictly smaller worst case; on ties prefer a possible guess.
		if worst < bestWorst || (worst == bestWorst && gPossible && !bestIsPossible) {
			bestWorst = worst
			best = g
			bestIsPossible = gPossible
		}
	}
	return best
}

// codeKey builds a stable string key for a code (colors are 0..Colors-1, and
// Colors <= a small number, so one byte per peg is enough for the classic
// configs; use a wider offset to stay safe for up to 255 colors).
func codeKey(c Guess) string {
	b := make([]byte, len(c))
	for i, v := range c {
		b[i] = byte(v)
	}
	return string(b)
}

// KnuthAllowed reports whether the Knuth device-guesses mode is offered for
// cfg. The minimax is O(n^2) in the number of codes, so it is only enabled for
// small code spaces (classic 4/6 = 1296, easy 4/4 = 256). Larger configs would
// be too slow per move on the device's ARM CPU.
func KnuthAllowed(cfg Config) bool {
	return len(allCodesCount(cfg)) <= 1300
}

// allCodesCount returns a slice sized to the number of codes without building
// the full code list twice at call sites that only need the count. It simply
// reuses allCodes; kept separate for readability of KnuthAllowed.
func allCodesCount(cfg Config) []Guess { return allCodes(cfg) }
