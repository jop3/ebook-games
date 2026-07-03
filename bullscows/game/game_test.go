package game

import (
	"math/rand"
	"testing"
)

func TestEvaluateAllBulls(t *testing.T) {
	s := Evaluate(Code{1, 2, 3, 4}, Code{1, 2, 3, 4})
	if s.Bulls != 4 || s.Cows != 0 {
		t.Fatalf("expected 4 bulls 0 cows, got %+v", s)
	}
	if !s.Solved(4) {
		t.Error("should be solved")
	}
}

func TestEvaluateAllCows(t *testing.T) {
	// Complete permutation with no digit in place.
	s := Evaluate(Code{1, 2, 3, 4}, Code{2, 1, 4, 3})
	if s.Bulls != 0 || s.Cows != 4 {
		t.Fatalf("expected 0 bulls 4 cows, got %+v", s)
	}
}

func TestEvaluateMixed(t *testing.T) {
	// secret 1234, guess 1325: bull=1 (the 1). 3->present wrong pos, 2->present
	// wrong pos, 5->absent. cows=2.
	s := Evaluate(Code{1, 2, 3, 4}, Code{1, 3, 2, 5})
	if s.Bulls != 1 {
		t.Errorf("expected 1 bull, got %d", s.Bulls)
	}
	if s.Cows != 2 {
		t.Errorf("expected 2 cows, got %d", s.Cows)
	}
}

func TestEvaluateNoneMatch(t *testing.T) {
	s := Evaluate(Code{1, 2, 3}, Code{4, 5, 6})
	if s.Bulls != 0 || s.Cows != 0 {
		t.Fatalf("expected 0/0, got %+v", s)
	}
}

func TestEvaluateLengthMismatch(t *testing.T) {
	s := Evaluate(Code{1, 2, 3}, Code{1, 2})
	if s.Bulls != 0 || s.Cows != 0 {
		t.Error("length mismatch should yield zero score")
	}
}

func TestRandomCodeDistinct(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		c := RandomCode(4, rng)
		if len(c) != 4 {
			t.Fatalf("wrong length %d", len(c))
		}
		seen := map[int]bool{}
		for _, d := range c {
			if d < 0 || d > 9 {
				t.Fatalf("digit out of range: %d", d)
			}
			if seen[d] {
				t.Fatalf("repeated digit in %v", c)
			}
			seen[d] = true
		}
	}
}
