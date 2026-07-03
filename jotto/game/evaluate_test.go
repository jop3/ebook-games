package game

import "testing"

// statuses builds an expected status slice from a compact string:
// 'c' Correct, 'p' Present, 'a' Absent.
func statuses(s string) []Status {
	out := make([]Status, 0, len(s))
	for _, r := range s {
		switch r {
		case 'c':
			out = append(out, Correct)
		case 'p':
			out = append(out, Present)
		case 'a':
			out = append(out, Absent)
		}
	}
	return out
}

func eq(a, b []Status) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestEvaluateBasic(t *testing.T) {
	cases := []struct {
		guess, secret, want string
	}{
		{"abcde", "abcde", "ccccc"}, // exact match
		{"abcde", "fghij", "aaaaa"}, // nothing in common
		// atlas vs slate:
		// a->present(a@2) t->present(t@3) l->present(l@1) a->absent(a used) s->present(s@0)
		{"atlas", "slate", "pppap"},
	}
	for _, c := range cases {
		got := Evaluate(c.guess, c.secret)
		if !eq(got, statuses(c.want)) {
			t.Errorf("Evaluate(%q,%q) = %v, want %v", c.guess, c.secret, got, statuses(c.want))
		}
	}
}

// TestEvaluateDuplicates is the classic Wordle duplicate-letter test: a guessed
// letter consumes at most one occurrence in the secret.
func TestEvaluateDuplicates(t *testing.T) {
	cases := []struct {
		name, guess, secret, want string
	}{
		{
			// The classic case: guess has more of a letter than the secret.
			// guess "sssaa" secret "aessn" -> letters guess: s,s,s,a,a
			//   secret a,e,s,s,n
			// Correct: idx2 s==s. Remaining secret: a,e,s,n (one s left, one a).
			//   idx0 s present (consumes remaining s), idx1 s absent,
			//   idx3 a present (consumes a), idx4 a absent.
			name:  "guess-has-more-of-a-letter",
			guess: "sssaa", secret: "aessn", want: "pacpa",
		},
		{
			// guess "eelet" secret "level": two E's guessed, two E's in secret
			// but positions differ.
			//   guess  e e l e t
			//   secret l e v e l
			// Correct: idx1 e==e, idx3 e==e. Remaining secret: l,v,l (no e left).
			//   idx0 e absent (no e remaining), idx2 l present (consumes l),
			//   idx4 t absent.
			name:  "equal-count-different-positions",
			guess: "eelet", secret: "level", want: "acpca",
		},
		{
			// Single secret occurrence, three guessed, none in place.
			//   guess  s a a a a
			//   secret x s x x x   -> guess "saaaa" secret "xsxxx"
			// Correct: none. Remaining: x,s,x,x,x (one s).
			//   idx0 s present (consumes s), idx1..4 a absent.
			name:  "one-in-secret-many-guessed",
			guess: "saaaa", secret: "xsxxx", want: "paaaa",
		},
	}
	for _, c := range cases {
		got := Evaluate(c.guess, c.secret)
		if !eq(got, statuses(c.want)) {
			t.Errorf("%s: Evaluate(%q,%q) = %v, want %v (%s)",
				c.name, c.guess, c.secret, got, statuses(c.want), c.want)
		}
	}
}

func TestEvaluateSwedishLetters(t *testing.T) {
	// å/ä/ö are single letters. björn vs björk -> first four correct, n absent.
	got := Evaluate("björn", "björk")
	if !eq(got, statuses("cccca")) {
		t.Errorf("swedish: got %v want %v", got, statuses("cccca"))
	}
	// mismatched rune length returns nil.
	if Evaluate("abc", "abcde") != nil {
		t.Errorf("expected nil for length mismatch")
	}
}

func TestAllCorrect(t *testing.T) {
	if !AllCorrect(statuses("ccccc")) {
		t.Error("expected AllCorrect true")
	}
	if AllCorrect(statuses("ccccp")) {
		t.Error("expected AllCorrect false")
	}
	if AllCorrect(nil) {
		t.Error("expected AllCorrect(nil) false")
	}
}
