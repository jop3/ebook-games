package game

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestEvaluateAllFermi(t *testing.T) {
	s := Evaluate(Code{1, 2, 3, 4}, Code{1, 2, 3, 4})
	if s.Fermi != 4 || s.Pico != 0 {
		t.Fatalf("expected 4 fermi 0 pico, got %+v", s)
	}
	if !s.Solved(4) {
		t.Error("should be solved")
	}
}

func TestEvaluateAllPico(t *testing.T) {
	// Complete permutation with no digit in place.
	s := Evaluate(Code{1, 2, 3, 4}, Code{2, 1, 4, 3})
	if s.Fermi != 0 || s.Pico != 4 {
		t.Fatalf("expected 0 fermi 4 pico, got %+v", s)
	}
}

func TestEvaluateMixed(t *testing.T) {
	// secret 1234, guess 1325: fermi=1 (the 1). 3->present wrong pos,
	// 2->present wrong pos, 5->absent. pico=2.
	s := Evaluate(Code{1, 2, 3, 4}, Code{1, 3, 2, 5})
	if s.Fermi != 1 {
		t.Errorf("expected 1 fermi, got %d", s.Fermi)
	}
	if s.Pico != 2 {
		t.Errorf("expected 2 pico, got %d", s.Pico)
	}
}

func TestEvaluateBagels(t *testing.T) {
	s := Evaluate(Code{1, 2, 3}, Code{4, 5, 6})
	if s.Fermi != 0 || s.Pico != 0 {
		t.Fatalf("expected 0/0, got %+v", s)
	}
	if fb := s.Feedback(); !reflect.DeepEqual(fb, []string{"Bagels"}) {
		t.Errorf("no-match should be Bagels, got %v", fb)
	}
}

func TestEvaluateLengthMismatch(t *testing.T) {
	s := Evaluate(Code{1, 2, 3}, Code{1, 2})
	if s.Fermi != 0 || s.Pico != 0 {
		t.Error("length mismatch should yield zero score")
	}
}

func TestFeedbackSortedAndCounted(t *testing.T) {
	// 1 fermi + 2 pico -> sorted words "Fermi Pico Pico" (Fermi before Pico,
	// hiding which position matched).
	s := Score{Fermi: 1, Pico: 2}
	got := s.Feedback()
	want := []string{"Fermi", "Pico", "Pico"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
	// The words must be non-decreasing (sorted) so order leaks no position.
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Errorf("feedback not sorted: %v", got)
		}
	}
}

func TestFeedbackAllFermi(t *testing.T) {
	s := Score{Fermi: 3, Pico: 0}
	got := s.Feedback()
	want := []string{"Fermi", "Fermi", "Fermi"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
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
