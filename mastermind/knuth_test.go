package main

import "testing"

// simulate plays the solver against a known secret, feeding it real feedback
// via Evaluate each turn, and returns the number of guesses used (or -1 if the
// solver failed / exceeded the cap).
func simulate(cfg Config, secret Secret, cap int) int {
	s := NewKnuthSolver(cfg)
	for turn := 1; turn <= cap; turn++ {
		g := s.CurrentGuess()
		fb := Evaluate(secret, g)
		over := s.Feedback(fb)
		if s.Solved() {
			return turn
		}
		if s.Impossible() {
			return -1
		}
		if over {
			return -1
		}
	}
	return -1
}

// TestKnuthSolvesAllClassic is the headline guarantee: the solver cracks every
// one of the 1296 codes in the classic 4-peg / 6-color game within 5 guesses.
func TestKnuthSolvesAllClassic(t *testing.T) {
	cfg := Config{Name: "Klassisk", Pegs: 4, Colors: 6, MaxGuesses: 10, AllowRepeat: true}
	codes := allCodes(cfg)
	if len(codes) != 1296 {
		t.Fatalf("expected 1296 codes, got %d", len(codes))
	}
	worst := 0
	total := 0
	for _, c := range codes {
		n := simulate(cfg, Secret(c), 6)
		if n < 0 {
			t.Fatalf("solver failed to crack %v", c)
		}
		if n > 5 {
			t.Fatalf("solver used %d guesses for %v, exceeds Knuth's 5", n, c)
		}
		if n > worst {
			worst = n
		}
		total += n
	}
	avg := float64(total) / float64(len(codes))
	t.Logf("classic 4/6: solved all %d codes, worst=%d guesses, avg=%.4f", len(codes), worst, avg)
}

// TestKnuthSolvesAllEasy44 checks the 4-peg / 4-color easy config too.
func TestKnuthSolvesAllEasy44(t *testing.T) {
	cfg := Config{Name: "Lätt", Pegs: 4, Colors: 4, MaxGuesses: 12, AllowRepeat: true}
	codes := allCodes(cfg)
	if len(codes) != 256 {
		t.Fatalf("expected 256 codes, got %d", len(codes))
	}
	worst := 0
	for _, c := range codes {
		n := simulate(cfg, Secret(c), 8)
		if n < 0 {
			t.Fatalf("solver failed to crack %v", c)
		}
		if n > worst {
			worst = n
		}
	}
	t.Logf("easy 4/4: solved all %d codes, worst=%d guesses", len(codes), worst)
	if worst > 5 {
		t.Fatalf("easy 4/4 worst case %d exceeds 5", worst)
	}
}

// TestAllCodes verifies enumeration counts for repeat and no-repeat configs.
func TestAllCodes(t *testing.T) {
	cases := []struct {
		cfg  Config
		want int
	}{
		{Config{Pegs: 4, Colors: 6, AllowRepeat: true}, 1296},
		{Config{Pegs: 4, Colors: 4, AllowRepeat: true}, 256},
		{Config{Pegs: 2, Colors: 3, AllowRepeat: true}, 9},
		{Config{Pegs: 3, Colors: 4, AllowRepeat: false}, 24}, // 4*3*2
		{Config{Pegs: 4, Colors: 4, AllowRepeat: false}, 24}, // 4!
	}
	for _, c := range cases {
		got := len(allCodes(c.cfg))
		if got != c.want {
			t.Errorf("allCodes(%dp/%dc repeat=%v) = %d, want %d",
				c.cfg.Pegs, c.cfg.Colors, c.cfg.AllowRepeat, got, c.want)
		}
	}
}

// TestFilterCorrect verifies that after feedback, every code left in the
// possible set is consistent with the feedback, and no consistent code was
// wrongly discarded.
func TestFilterCorrect(t *testing.T) {
	cfg := Config{Pegs: 4, Colors: 6, AllowRepeat: true}
	s := NewKnuthSolver(cfg)
	g := s.CurrentGuess()
	// Pick an arbitrary secret and get real feedback.
	secret := Secret{2, 4, 1, 5}
	fb := Evaluate(secret, g)
	s.Feedback(fb)

	// Every remaining possible must reproduce fb against the guess.
	for _, c := range s.possible {
		if got := Evaluate(Secret(c), g); got != fb {
			t.Fatalf("kept inconsistent code %v: Evaluate=%v want %v", c, got, fb)
		}
	}
	// The true secret must still be in the set.
	found := false
	for _, c := range s.possible {
		if codeKey(c) == codeKey(Guess(secret)) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("true secret %v was filtered out", secret)
	}

	// Cross-check: count all full-space codes consistent with fb; must equal
	// the possible-set size.
	want := 0
	for _, c := range allCodes(cfg) {
		if Evaluate(Secret(c), g) == fb {
			want++
		}
	}
	if len(s.possible) != want {
		t.Fatalf("filter kept %d, brute force says %d consistent", len(s.possible), want)
	}
}

// TestImpossibleFeedback verifies contradiction detection: feeding inconsistent
// feedback drives the possible set to empty and flags Impossible.
func TestImpossibleFeedback(t *testing.T) {
	cfg := Config{Pegs: 4, Colors: 6, AllowRepeat: true}
	s := NewKnuthSolver(cfg)

	// First guess is 1122 (0,0,1,1). Claim 4 black — meaning the secret IS
	// 0,0,1,1. Next guess should be that code (only one possible).
	over := s.Feedback(Feedback{Black: 4, White: 0})
	if !over || !s.Solved() {
		t.Fatalf("4 black on opener should solve immediately")
	}

	// Fresh solver: give contradictory feedback across two turns.
	s2 := NewKnuthSolver(cfg)
	// Turn 1: opener 0,0,1,1. Say 0 black, 0 white -> secret uses none of
	// colors 0 or 1 at all.
	s2.Feedback(Feedback{Black: 0, White: 0})
	if s2.Impossible() {
		t.Fatalf("0/0 on opener is a legal outcome, should not be impossible")
	}
	g2 := s2.CurrentGuess()
	// Now claim the new guess is fully correct AND later contradict — instead
	// directly force an impossible state: assert a feedback no remaining code
	// can produce. After 0/0, all remaining codes use only colors 2..5.
	// Evaluate any such code vs g2; pick a feedback value that never occurs.
	// Simplest robust contradiction: claim more black than pegs is invalid at
	// UI level, so instead drive to empty by exhausting.
	_ = g2
	// Feed a feedback that cannot match: we compute the set of feedbacks that
	// CAN occur for g2, then pick one that cannot.
	occurring := map[int]bool{}
	for _, c := range s2.possible {
		occurring[feedbackKey(Evaluate(Secret(c), g2))] = true
	}
	var bad Feedback
	found := false
	for b := 0; b <= cfg.Pegs && !found; b++ {
		for w := 0; b+w <= cfg.Pegs; w++ {
			if b == cfg.Pegs {
				continue // that would be "solved"
			}
			if !occurring[feedbackKey(Feedback{Black: b, White: w})] {
				bad = Feedback{Black: b, White: w}
				found = true
				break
			}
		}
	}
	if !found {
		t.Skip("no impossible feedback available for this state")
	}
	s2.Feedback(bad)
	if !s2.Impossible() {
		t.Fatalf("feeding impossible feedback %v should flag Impossible", bad)
	}
}

// TestKnuthAllowed checks the gating for which configs offer the mode.
func TestKnuthAllowed(t *testing.T) {
	if !KnuthAllowed(Config{Pegs: 4, Colors: 6, AllowRepeat: true}) {
		t.Error("classic 4/6 (1296) should be allowed")
	}
	if !KnuthAllowed(Config{Pegs: 4, Colors: 4, AllowRepeat: true}) {
		t.Error("easy 4/4 (256) should be allowed")
	}
	if KnuthAllowed(Config{Pegs: 5, Colors: 8, AllowRepeat: true}) {
		t.Error("hard 5/8 (32768) should NOT be allowed (too slow)")
	}
}
