package main

import (
	"math/rand"
	"testing"
)

// M1-test (§6.10): generera 100 pussel och verifiera att ALLA har exakt
// en lösning enligt samma lösare. Reproducerbart via fast seed.
func TestGenerate100Unique(t *testing.T) {
	for _, d := range []Difficulty{Easy, Medium, Hard} {
		rng := rand.New(rand.NewSource(int64(d) + 12345))
		for i := 0; i < 100; i++ {
			p := Generate(d, rng)
			cnt := CountSolutions(p.N, p.Categories, p.Clues)
			if cnt != 1 {
				t.Fatalf("difficulty=%d pussel %d: förväntade 1 lösning, fick %d (clues=%d)",
					d, i, cnt, len(p.Clues))
			}
			// Verifiera att den funna lösningen matchar den genererade lösningen.
			sol, unique := Solve(p.N, p.Categories, p.Clues)
			if !unique {
				t.Fatalf("difficulty=%d pussel %d: Solve gav ej unik", d, i)
			}
			if !sameSolution(sol, p.Solution) {
				t.Fatalf("difficulty=%d pussel %d: löst lösning != genererad lösning", d, i)
			}
			// Alla ledtrådar måste vara sanna i lösningen.
			for _, c := range p.Clues {
				if !clueValid(p.Solution, c) {
					t.Fatalf("difficulty=%d pussel %d: ogiltig ledtråd %+v", d, i, c)
				}
			}
		}
	}
}

func sameSolution(a, b Solution) bool {
	if a.N != b.N || a.Categories != b.Categories {
		return false
	}
	for cat := 0; cat < a.Categories; cat++ {
		for pos := 0; pos < a.N; pos++ {
			if a.Assignment[cat][pos] != b.Assignment[cat][pos] {
				return false
			}
		}
	}
	return true
}

// Minimering för Medium/Hard bör ge få ledtrådar (sanity, ej hårt krav).
func TestGenerateReducesClues(t *testing.T) {
	rng := rand.New(rand.NewSource(999))
	p := Generate(Hard, rng)
	if len(p.Clues) == 0 {
		t.Fatalf("Hard-pussel har inga ledtrådar")
	}
	// Ett minimalt 5x5-pussel har typiskt < full pool.
	if len(p.Clues) > 40 {
		t.Fatalf("Hard-pussel verkar ominimerat: %d ledtrådar", len(p.Clues))
	}
}
