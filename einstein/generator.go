package main

// generator.go — §6.4 kandidatgenerering + §6.6 generate-and-reduce.
// FRI från inkview-import (enhetstestbar utan cgo).
// Använder en injicerad *rand.Rand så tester är reproducerbara (seedas explicit).

import "math/rand"

// Difficulty enligt §6.7.
type Difficulty int

const (
	Easy   Difficulty = iota // 3x3: Direct, SamePosition, delvis beskärning
	Medium                   // 4x4: + Neighbor, DirectionalNeighbor, fullt minimerad
	Hard                     // 5x5: alla typer, viktat bort från Direct, fullt minimerad
)

// DifficultyConfig beskriver en svårighetsgrad.
type DifficultyConfig struct {
	N          int // kategorier × positioner (kvadratiskt)
	Categories int
	Types      []ClueType
	FullReduce bool
}

func ConfigFor(d Difficulty) DifficultyConfig {
	switch d {
	case Easy:
		return DifficultyConfig{N: 3, Categories: 3,
			Types: []ClueType{Direct, SamePosition}, FullReduce: false}
	case Medium:
		return DifficultyConfig{N: 4, Categories: 4,
			Types: []ClueType{Direct, SamePosition, Neighbor, DirectionalNeighbor}, FullReduce: true}
	default: // Hard
		return DifficultyConfig{N: 5, Categories: 5,
			Types: []ClueType{Direct, SamePosition, NotSamePosition, Neighbor, DirectionalNeighbor}, FullReduce: true}
	}
}

// randomSolution: slumpmässig permutation per kategori (§6.6.1).
func randomSolution(n, categories int, rng *rand.Rand) Solution {
	sol := Solution{N: n, Categories: categories}
	sol.Assignment = make([][]ValueID, categories)
	for cat := 0; cat < categories; cat++ {
		perm := rng.Perm(n) // perm[pos] = värde
		sol.Assignment[cat] = make([]ValueID, n)
		for pos := 0; pos < n; pos++ {
			sol.Assignment[cat][pos] = ValueID(perm[pos])
		}
	}
	return sol
}

// posOfValue: var ligger värdet val i kategori cat i lösningen?
func posOfValue(sol Solution, cat, val int) int {
	for p := 0; p < sol.N; p++ {
		if int(sol.Assignment[cat][p]) == val {
			return p
		}
	}
	return -1
}

// valueAt: vilket värde har kategori cat på position pos?
func valueAt(sol Solution, cat, pos int) int {
	return int(sol.Assignment[cat][pos])
}

// candidateClues: generera ALLA sanna ledtrådar av de tillåtna typerna (§6.4).
func candidateClues(sol Solution, types []ClueType) []Clue {
	n := sol.N
	cats := sol.Categories
	allowed := map[ClueType]bool{}
	for _, t := range types {
		allowed[t] = true
	}
	var out []Clue

	// Direct: för varje (kategori, position).
	if allowed[Direct] {
		for cat := 0; cat < cats; cat++ {
			for pos := 0; pos < n; pos++ {
				out = append(out, Clue{Type: Direct, CatA: cat, ValA: valueAt(sol, cat, pos), Position: pos})
			}
		}
	}

	// Parvisa kategorier.
	for a := 0; a < cats; a++ {
		for b := 0; b < cats; b++ {
			if a == b {
				continue
			}
			// SamePosition: den faktiska värdeparningen på varje position.
			// (a<b räcker för symmetriska, men vi tar a!=b och begränsar nedan)
			if allowed[SamePosition] && a < b {
				for pos := 0; pos < n; pos++ {
					out = append(out, Clue{Type: SamePosition,
						CatA: a, ValA: valueAt(sol, a, pos),
						CatB: b, ValB: valueAt(sol, b, pos)})
				}
			}
			// NotSamePosition: alla värdepar som INTE delar position.
			if allowed[NotSamePosition] && a < b {
				for va := 0; va < n; va++ {
					pa := posOfValue(sol, a, va)
					for vb := 0; vb < n; vb++ {
						pb := posOfValue(sol, b, vb)
						if pa != pb {
							out = append(out, Clue{Type: NotSamePosition,
								CatA: a, ValA: va, CatB: b, ValB: vb})
						}
					}
				}
			}
			// Neighbor: par av angränsande positioner (a<b för att undvika dubletter).
			if allowed[Neighbor] && a < b {
				for pos := 0; pos < n-1; pos++ {
					// värde i a på pos, värde i b på pos+1 (|diff|==1)
					out = append(out, Clue{Type: Neighbor,
						CatA: a, ValA: valueAt(sol, a, pos),
						CatB: b, ValB: valueAt(sol, b, pos+1)})
					out = append(out, Clue{Type: Neighbor,
						CatA: a, ValA: valueAt(sol, a, pos+1),
						CatB: b, ValB: valueAt(sol, b, pos)})
				}
			}
			// DirectionalNeighbor: A direkt höger om B (posA == posB+1). a!=b, båda riktningar via a,b-loop.
			if allowed[DirectionalNeighbor] {
				for pos := 0; pos < n-1; pos++ {
					// A på pos+1, B på pos => posA = posB+1
					out = append(out, Clue{Type: DirectionalNeighbor,
						CatA: a, ValA: valueAt(sol, a, pos+1),
						CatB: b, ValB: valueAt(sol, b, pos)})
				}
			}
		}
	}
	return out
}

// clueValid: sant om ledtråden verkligen håller i lösningen (säkerhetskontroll).
func clueValid(sol Solution, c Clue) bool {
	switch c.Type {
	case Direct:
		return posOfValue(sol, c.CatA, c.ValA) == c.Position
	case SamePosition:
		return posOfValue(sol, c.CatA, c.ValA) == posOfValue(sol, c.CatB, c.ValB)
	case NotSamePosition:
		return posOfValue(sol, c.CatA, c.ValA) != posOfValue(sol, c.CatB, c.ValB)
	case Neighbor:
		d := posOfValue(sol, c.CatA, c.ValA) - posOfValue(sol, c.CatB, c.ValB)
		return d == 1 || d == -1
	case DirectionalNeighbor:
		return posOfValue(sol, c.CatA, c.ValA) == posOfValue(sol, c.CatB, c.ValB)+1
	}
	return false
}

// Puzzle: en genererad gåta.
type Puzzle struct {
	N          int
	Categories int
	Clues      []Clue
	Solution   Solution
	Difficulty Difficulty
}

// Generate producerar ett pussel med exakt en lösning (§6.6).
// rng injiceras för reproducerbarhet.
func Generate(d Difficulty, rng *rand.Rand) Puzzle {
	cfg := ConfigFor(d)
	n, cats := cfg.N, cfg.Categories

	for attempt := 0; attempt < 200; attempt++ {
		sol := randomSolution(n, cats, rng)
		pool := candidateClues(sol, cfg.Types)

		// 3. Blanda poolen.
		rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

		// 4. Lägg till ledtrådar tills exakt 1 lösning.
		var chosen []Clue
		unique := false
		for _, c := range pool {
			chosen = append(chosen, c)
			cnt := CountSolutions(n, cats, chosen)
			if cnt == 1 {
				unique = true
				break
			}
		}
		if !unique {
			continue // ovanligt; prova ny lösning
		}

		// 5. Beskärning: ta bort överflödiga ledtrådar (fixpunkt).
		reduce := cfg.FullReduce
		if reduce {
			chosen = reduceClues(n, cats, chosen, rng)
		} else {
			// Delvis beskärning: ett pass, men lämna några extra kvar (§6.7 Lätt).
			chosen = reducePartial(n, cats, chosen, rng)
		}

		// Verifiera slutresultatet.
		if CountSolutions(n, cats, chosen) != 1 {
			continue
		}
		return Puzzle{N: n, Categories: cats, Clues: chosen, Solution: sol, Difficulty: d}
	}
	// Fallback: borde aldrig nås för N<=5.
	sol := randomSolution(n, cats, rng)
	return Puzzle{N: n, Categories: cats, Clues: candidateClues(sol, cfg.Types), Solution: sol, Difficulty: d}
}

// reduceClues: fullständig minimering till fixpunkt (§6.6.5).
func reduceClues(n, cats int, clues []Clue, rng *rand.Rand) []Clue {
	order := rng.Perm(len(clues))
	// Vi arbetar med en "removed"-markering.
	removed := make([]bool, len(clues))
	changed := true
	for changed {
		changed = false
		for _, i := range order {
			if removed[i] {
				continue
			}
			// Testa att ta bort clue i.
			removed[i] = true
			trial := activeClues(clues, removed)
			if CountSolutions(n, cats, trial) == 1 {
				changed = true // förblev unik => håll borttagen
			} else {
				removed[i] = false // behövdes => återställ
			}
		}
	}
	return activeClues(clues, removed)
}

// reducePartial: ta bort några men lämna extra bekräftelser (Lätt-läge).
// Ett enda pass, och behåll ~30% av de "överflödiga".
func reducePartial(n, cats int, clues []Clue, rng *rand.Rand) []Clue {
	order := rng.Perm(len(clues))
	removed := make([]bool, len(clues))
	for _, i := range order {
		if removed[i] {
			continue
		}
		// Lämna vissa överflödiga kvar som gratis bekräftelse.
		if rng.Intn(100) < 30 {
			continue
		}
		removed[i] = true
		trial := activeClues(clues, removed)
		if CountSolutions(n, cats, trial) != 1 {
			removed[i] = false
		}
	}
	return activeClues(clues, removed)
}

func activeClues(clues []Clue, removed []bool) []Clue {
	out := make([]Clue, 0, len(clues))
	for i, c := range clues {
		if !removed[i] {
			out = append(out, c)
		}
	}
	return out
}
