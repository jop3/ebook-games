package game

import "testing"

func TestEntryFlow(t *testing.T) {
	s := NewGameSeeded(Presets[1], 42) // 4 digits
	if !s.AppendDigit(1) || !s.AppendDigit(2) {
		t.Fatal("appends should succeed")
	}
	if s.AppendDigit(1) {
		t.Error("duplicate digit should be rejected")
	}
	if s.EntryComplete() {
		t.Error("entry not complete yet")
	}
	s.AppendDigit(3)
	s.AppendDigit(4)
	if !s.EntryComplete() {
		t.Error("entry should be complete at length 4")
	}
	if s.AppendDigit(5) {
		t.Error("no room for a 5th digit")
	}
}

func TestBackspace(t *testing.T) {
	s := NewGameSeeded(Presets[0], 1)
	s.AppendDigit(7)
	if !s.Backspace() {
		t.Fatal("backspace should remove the digit")
	}
	if len(s.Entry) != 0 {
		t.Error("entry should be empty")
	}
	if s.Backspace() {
		t.Error("backspace on empty entry should be a no-op")
	}
}

func TestSubmitAndWin(t *testing.T) {
	s := NewGameSeeded(Presets[1], 7)
	// Guess the actual secret to force a win.
	for _, d := range s.Secret {
		s.AppendDigit(d)
	}
	if !s.Submit() {
		t.Fatal("submit of complete entry should succeed")
	}
	if !s.Solved {
		t.Error("guessing the secret should win")
	}
	if s.Lost {
		t.Error("a win should not also be a loss")
	}
	if len(s.Guesses) != 1 {
		t.Errorf("expected 1 recorded guess, got %d", len(s.Guesses))
	}
	// No more guesses after solving.
	if s.AppendDigit(0) {
		t.Error("should not accept input after solving")
	}
}

func TestSubmitIncompleteRejected(t *testing.T) {
	s := NewGameSeeded(Presets[1], 3)
	s.AppendDigit(1)
	if s.Submit() {
		t.Error("incomplete entry should not submit")
	}
}

func TestLossOnLastGuess(t *testing.T) {
	p := Preset{Name: "test", Length: 3, MaxTurn: 3}
	s := NewGameSeeded(p, 9)
	// A guess guaranteed to differ from the secret: pick digits, and if they
	// happen to equal the secret, perturb. Simplest: build a wrong guess.
	wrong := wrongGuess(s.Secret)
	for turn := 0; turn < p.MaxTurn; turn++ {
		s.Entry = s.Entry[:0]
		for _, d := range wrong {
			s.AppendDigit(d)
		}
		if !s.Submit() {
			t.Fatalf("submit %d should succeed", turn)
		}
	}
	if !s.Lost {
		t.Error("running out of guesses should be a loss")
	}
	if s.Solved {
		t.Error("a loss should not be solved")
	}
	if s.TurnsLeft() != 0 {
		t.Errorf("no turns should remain, got %d", s.TurnsLeft())
	}
	if s.AppendDigit(0) {
		t.Error("should not accept input after loss")
	}
}

func TestTurnsLeft(t *testing.T) {
	p := Preset{Name: "test", Length: 3, MaxTurn: 5}
	s := NewGameSeeded(p, 1)
	if s.TurnsLeft() != 5 {
		t.Errorf("expected 5 turns, got %d", s.TurnsLeft())
	}
	wrong := wrongGuess(s.Secret)
	for _, d := range wrong {
		s.AppendDigit(d)
	}
	s.Submit()
	if s.TurnsLeft() != 4 {
		t.Errorf("expected 4 turns after one guess, got %d", s.TurnsLeft())
	}
}

// wrongGuess returns a distinct-digit code the same length as secret that is
// guaranteed not to equal secret (rotates digits).
func wrongGuess(secret Code) Code {
	out := make(Code, len(secret))
	// Use digits 0..len-1 shifted; if equal to secret, shift again.
	base := 0
	for {
		used := map[int]bool{}
		ok := true
		for i := range out {
			d := (base + i) % 10
			if used[d] {
				ok = false
				break
			}
			used[d] = true
			out[i] = d
		}
		if ok && !Equal(out, secret) {
			return out
		}
		base++
		if base > 20 {
			return out
		}
	}
}
