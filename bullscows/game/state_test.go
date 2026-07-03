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
