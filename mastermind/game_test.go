package main

import (
	"math/rand"
	"testing"
)

func fb(b, w int) Feedback { return Feedback{Black: b, White: w} }

func TestEvaluate(t *testing.T) {
	cases := []struct {
		name   string
		secret Secret
		guess  Guess
		want   Feedback
	}{
		{
			// Design-doc case: secret=[1,1,2,3] guess=[1,2,1,1].
			// Pos0: 1==1 black. Remaining secret {1,2,3}, guess {2,1,1}.
			// Color 1: secret has 1, guess has 2 -> min = 1 white.
			// Color 2: secret has 1, guess has 1 -> min = 1 white.
			// Total: black=1, white=2. The extra guessed 1 must NOT double-count
			// off the already-black position.
			name:   "repeated colors doc case",
			secret: Secret{1, 1, 2, 3},
			guess:  Guess{1, 2, 1, 1},
			want:   fb(1, 2),
		},
		{
			name:   "all correct",
			secret: Secret{0, 1, 2, 3},
			guess:  Guess{0, 1, 2, 3},
			want:   fb(4, 0),
		},
		{
			name:   "zero correct",
			secret: Secret{0, 0, 0, 0},
			guess:  Guess{1, 1, 1, 1},
			want:   fb(0, 0),
		},
		{
			name:   "all same color secret and guess",
			secret: Secret{2, 2, 2, 2},
			guess:  Guess{2, 2, 2, 2},
			want:   fb(4, 0),
		},
		{
			// secret all same, guess has one match -> 1 black, extras cannot
			// be white because secret has no non-black 2s left.
			name:   "guess repeats against single-color secret",
			secret: Secret{2, 2, 2, 2},
			guess:  Guess{2, 3, 3, 3},
			want:   fb(1, 0),
		},
		{
			// One color fully misplaced.
			name:   "full swap two colors",
			secret: Secret{0, 1, 0, 1},
			guess:  Guess{1, 0, 1, 0},
			want:   fb(0, 4),
		},
		{
			// guess has more of a color than secret does.
			name:   "guess over-supplies a color",
			secret: Secret{1, 2, 3, 4},
			guess:  Guess{1, 1, 1, 1},
			want:   fb(1, 0),
		},
		{
			name:   "mixed blacks and whites",
			secret: Secret{3, 1, 4, 1},
			guess:  Guess{1, 1, 1, 4},
			// Pos1: 1==1 black. Remaining secret {3,4,1}, guess {1,1,4}.
			// Color 1: secret 1, guess 2 -> 1 white. Color 4: secret 1, guess 1 -> 1 white.
			want: fb(1, 2),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Evaluate(c.secret, c.guess)
			if got != c.want {
				t.Errorf("Evaluate(%v, %v) = %+v, want %+v", c.secret, c.guess, got, c.want)
			}
		})
	}
}

func TestEvaluateSymmetryInvariant(t *testing.T) {
	// Black+White can never exceed the number of pegs.
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 10000; i++ {
		pegs := 4 + rng.Intn(2)
		colors := 4 + rng.Intn(5)
		s := make(Secret, pegs)
		g := make(Guess, pegs)
		for j := 0; j < pegs; j++ {
			s[j] = Color(rng.Intn(colors))
			g[j] = Color(rng.Intn(colors))
		}
		f := Evaluate(s, g)
		if f.Black+f.White > pegs {
			t.Fatalf("black+white=%d > pegs=%d for secret=%v guess=%v", f.Black+f.White, pegs, s, g)
		}
		if f.Black < 0 || f.White < 0 {
			t.Fatalf("negative feedback %+v", f)
		}
	}
}

func TestNewSecretNoRepeat(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	cfg := Presets[3] // Expert, AllowRepeat=false
	for i := 0; i < 1000; i++ {
		s := NewSecret(cfg, rng)
		if len(s) != cfg.Pegs {
			t.Fatalf("wrong length %d", len(s))
		}
		seen := map[Color]bool{}
		for _, c := range s {
			if c < 0 || int(c) >= cfg.Colors {
				t.Fatalf("color %d out of range", c)
			}
			if seen[c] {
				t.Fatalf("repeat color in no-repeat secret %v", s)
			}
			seen[c] = true
		}
	}
}

func TestNewSecretWithRepeat(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	cfg := Presets[0] // Klassisk
	for i := 0; i < 1000; i++ {
		s := NewSecret(cfg, rng)
		if len(s) != cfg.Pegs {
			t.Fatalf("wrong length %d", len(s))
		}
		for _, c := range s {
			if c < 0 || int(c) >= cfg.Colors {
				t.Fatalf("color %d out of range", c)
			}
		}
	}
}

func TestSubmitWinLose(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	cfg := Config{Name: "T", Pegs: 4, Colors: 6, MaxGuesses: 3, AllowRepeat: true}
	g := NewGame(cfg, rng)

	// Winning guess.
	fbk := g.Submit(Guess(append(Secret(nil), g.Secret...)))
	if fbk.Black != cfg.Pegs {
		t.Fatalf("expected all black, got %+v", fbk)
	}
	if g.Status != Won {
		t.Fatalf("expected Won, got %v", g.Status)
	}
	// Further submits ignored.
	before := len(g.History)
	g.Submit(Guess{0, 0, 0, 0})
	if len(g.History) != before {
		t.Fatalf("submit after game over should be ignored")
	}
}

func TestSubmitLoss(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	cfg := Config{Name: "T", Pegs: 4, Colors: 6, MaxGuesses: 2, AllowRepeat: true}
	g := NewGame(cfg, rng)
	// Craft a guaranteed-wrong guess by offsetting each secret color.
	wrong := make(Guess, cfg.Pegs)
	for i := range wrong {
		wrong[i] = Color((int(g.Secret[i]) + 1) % cfg.Colors)
	}
	g.Submit(append(Guess(nil), wrong...))
	if g.Status != Playing {
		t.Fatalf("should still be playing after 1 of 2, got %v", g.Status)
	}
	g.Submit(append(Guess(nil), wrong...))
	if g.Status != Lost {
		t.Fatalf("expected Lost after exhausting guesses, got %v", g.Status)
	}
}
