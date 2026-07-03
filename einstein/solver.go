package main

// solver.go — Domän-propagering (bitmask per [cat][pos]) + backtracking
// som räknar "0, 1 eller >1 lösningar" med tidigt avbrott vid 2.
// Neighbor/DirectionalNeighbor hanteras som KONTROLL under backtracking.
// Denna fil är FRI från inkview-import så den kan enhetstestas utan cgo.

// --- Datamodell (§6.3) ---

type CategoryID int
type ValueID int // 0..N-1 inom sin kategori

// Solution: för varje kategori, vilken position varje värde har.
// Assignment[cat][pos] = värdeindex för den kategorin vid den positionen.
type Solution struct {
	N          int
	Categories int
	Assignment [][]ValueID
}

// ClueType identifierar vilken sorts påstående en ledtråd är.
type ClueType int

const (
	Direct              ClueType = iota // värde X är på position P
	SamePosition                        // värde A och värde B (olika kategorier) på samma position
	NotSamePosition                     // värde A och värde B INTE på samma position
	Neighbor                            // |posA - posB| == 1
	DirectionalNeighbor                 // posA == posB + 1 (A direkt höger om B)
)

type Clue struct {
	Type       ClueType
	CatA, ValA int
	CatB, ValB int // ej satt för Direct
	Position   int // bara satt för Direct
}

// --- Domän-representation ---
//
// Domain[cat][pos] = bitmask över vilka värden som fortfarande är möjliga.
// Bit v satt => värde v kan vara i kategori cat vid position pos.

type Domains struct {
	N          int
	Categories int
	// mask[cat*N + pos] = bitmask
	mask []uint32
}

func fullMask(n int) uint32 {
	return (uint32(1) << uint(n)) - 1
}

func newDomains(n, categories int) *Domains {
	full := fullMask(n)
	d := &Domains{N: n, Categories: categories, mask: make([]uint32, n*categories)}
	for i := range d.mask {
		d.mask[i] = full
	}
	return d
}

func (d *Domains) idx(cat, pos int) int { return cat*d.N + pos }

func (d *Domains) get(cat, pos int) uint32    { return d.mask[d.idx(cat, pos)] }
func (d *Domains) set(cat, pos int, m uint32) { d.mask[d.idx(cat, pos)] = m }
func (d *Domains) copy() *Domains {
	c := &Domains{N: d.N, Categories: d.Categories, mask: make([]uint32, len(d.mask))}
	copy(c.mask, d.mask)
	return c
}

func popcount(m uint32) int {
	c := 0
	for m != 0 {
		m &= m - 1
		c++
	}
	return c
}

// lowestBit returns the index of the single set bit; only valid for singletons.
func bitIndex(m uint32) int {
	i := 0
	for m > 1 {
		m >>= 1
		i++
	}
	return i
}

// --- Propagering (§6.5) ---
//
// Returnerar false om en motsägelse upptäcks (tom domän).
func propagate(d *Domains, clues []Clue) bool {
	n := d.N
	changed := true
	for changed {
		changed = false

		// 2. Direct: sätt Domain[cat][pos] = {val}.
		// 3. SamePosition (länkad).
		// 4. NotSamePosition.
		for _, c := range clues {
			switch c.Type {
			case Direct:
				only := uint32(1) << uint(c.ValA)
				if d.get(c.CatA, c.Position) != only {
					if d.get(c.CatA, c.Position)&only == 0 {
						return false // val ej möjligt där
					}
					d.set(c.CatA, c.Position, only)
					changed = true
				}
				// ta bort valA från övriga positioner i kategorin
				for p := 0; p < n; p++ {
					if p == c.Position {
						continue
					}
					m := d.get(c.CatA, p)
					if m&only != 0 {
						d.set(c.CatA, p, m&^only)
						changed = true
					}
				}

			case SamePosition:
				aBit := uint32(1) << uint(c.ValA)
				bBit := uint32(1) << uint(c.ValB)
				for p := 0; p < n; p++ {
					aPossible := d.get(c.CatA, p)&aBit != 0
					bPossible := d.get(c.CatB, p)&bBit != 0
					// Om A inte kan vara här, kan inte B heller (och omvänt).
					if !aPossible && bPossible {
						d.set(c.CatB, p, d.get(c.CatB, p)&^bBit)
						changed = true
					}
					if !bPossible && aPossible {
						d.set(c.CatA, p, d.get(c.CatA, p)&^aBit)
						changed = true
					}
					// Om A är fixerad till valA här, fixera B till valB här.
					if d.get(c.CatA, p) == aBit && d.get(c.CatB, p) != bBit {
						if d.get(c.CatB, p)&bBit == 0 {
							return false
						}
						d.set(c.CatB, p, bBit)
						changed = true
					}
					if d.get(c.CatB, p) == bBit && d.get(c.CatA, p) != aBit {
						if d.get(c.CatA, p)&aBit == 0 {
							return false
						}
						d.set(c.CatA, p, aBit)
						changed = true
					}
				}

			case NotSamePosition:
				aBit := uint32(1) << uint(c.ValA)
				bBit := uint32(1) << uint(c.ValB)
				for p := 0; p < n; p++ {
					// Om A fixerad till valA på p, ta bort valB från B[p].
					if d.get(c.CatA, p) == aBit {
						m := d.get(c.CatB, p)
						if m&bBit != 0 {
							d.set(c.CatB, p, m&^bBit)
							changed = true
						}
					}
					if d.get(c.CatB, p) == bBit {
						m := d.get(c.CatA, p)
						if m&aBit != 0 {
							d.set(c.CatA, p, m&^aBit)
							changed = true
						}
					}
				}
			case Neighbor:
				// §6.5.5: relationsvillkor kontrolleras vid tilldelning. Vi lägger
				// dessutom till SUND forward-checking (positions-domänbeskärning):
				// värde A kan bara stå på position p om värde B kan stå på p-1 eller
				// p+1, och omvänt. Detta ändrar aldrig lösningsmängden (endast
				// eliminerar omöjliga positioner) men beskär sökträdet drastiskt.
				aBit := uint32(1) << uint(c.ValA)
				bBit := uint32(1) << uint(c.ValB)
				if narrowNeighbor(d, c.CatA, aBit, c.CatB, bBit, false) {
					changed = true
				}

			case DirectionalNeighbor:
				// posA == posB + 1: A kan bara stå på p om B kan stå på p-1;
				// B kan bara stå på p om A kan stå på p+1.
				aBit := uint32(1) << uint(c.ValA)
				bBit := uint32(1) << uint(c.ValB)
				if narrowNeighbor(d, c.CatA, aBit, c.CatB, bBit, true) {
					changed = true
				}
			}
		}

		// 1. All-different per kategori (naked/hidden singles).
		for cat := 0; cat < d.Categories; cat++ {
			// naked single: fixerad position -> ta bort dess värde från övriga
			for p := 0; p < n; p++ {
				m := d.get(cat, p)
				if m != 0 && popcount(m) == 1 {
					for q := 0; q < n; q++ {
						if q == p {
							continue
						}
						om := d.get(cat, q)
						if om&m != 0 {
							d.set(cat, q, om&^m)
							changed = true
						}
					}
				}
			}
			// hidden single: värde v som bara kan vara på en position -> fixera
			for v := 0; v < n; v++ {
				vBit := uint32(1) << uint(v)
				lastPos := -1
				cnt := 0
				for p := 0; p < n; p++ {
					if d.get(cat, p)&vBit != 0 {
						cnt++
						lastPos = p
					}
				}
				if cnt == 0 {
					return false // värdet får plats ingenstans
				}
				if cnt == 1 && d.get(cat, lastPos) != vBit {
					d.set(cat, lastPos, vBit)
					changed = true
				}
			}
		}

		// motsägelsekontroll: tom domän
		for i := 0; i < len(d.mask); i++ {
			if d.mask[i] == 0 {
				return false
			}
		}
	}
	return true
}

// posSet returnerar en bitmask över positioner där värdet (bit) fortfarande är
// möjligt i kategorin cat (bit p satt => värdet kan stå på position p).
func posSet(d *Domains, cat int, bit uint32) uint32 {
	var s uint32
	for p := 0; p < d.N; p++ {
		if d.get(cat, p)&bit != 0 {
			s |= 1 << uint(p)
		}
	}
	return s
}

// removeValAtPos tar bort värdet (bit) från kategorins position p.
func removeValAtPos(d *Domains, cat, p int, bit uint32) bool {
	m := d.get(cat, p)
	if m&bit != 0 {
		d.set(cat, p, m&^bit)
		return true
	}
	return false
}

// narrowNeighbor beskär positionsdomäner för ett Neighbor/DirectionalNeighbor-
// villkor. directional=false => |posA-posB|==1; directional=true => posA==posB+1.
// Returnerar true om något ändrades. Sund: eliminerar endast positioner som inte
// kan uppfylla villkoret oavsett hur resten löses.
func narrowNeighbor(d *Domains, catA int, aBit uint32, catB int, bBit uint32, directional bool) bool {
	n := d.N
	changed := false
	for {
		pa := posSet(d, catA, aBit)
		pb := posSet(d, catB, bBit)
		// Tillåtna A-positioner utifrån B-positioner.
		var allowedA, allowedB uint32
		if directional {
			// A på p tillåtet om B kan stå på p-1.
			allowedA = (pb << 1) & fullMask(n)
			// B på p tillåtet om A kan stå på p+1.
			allowedB = (pa >> 1)
		} else {
			// A på p tillåtet om B kan stå på p-1 eller p+1.
			allowedA = ((pb << 1) | (pb >> 1)) & fullMask(n)
			allowedB = ((pa << 1) | (pa >> 1)) & fullMask(n)
		}
		local := false
		for p := 0; p < n; p++ {
			pbit := uint32(1) << uint(p)
			if pa&pbit != 0 && allowedA&pbit == 0 {
				if removeValAtPos(d, catA, p, aBit) {
					local, changed = true, true
				}
			}
			if pb&pbit != 0 && allowedB&pbit == 0 {
				if removeValAtPos(d, catB, p, bBit) {
					local, changed = true, true
				}
			}
		}
		if !local {
			break
		}
	}
	return changed
}

// relationalHold verifierar Neighbor/DirectionalNeighbor på singleton-domäner.
// Antar att alla domäner är singletons.
func relationalHold(d *Domains, clues []Clue) bool {
	posOf := func(cat, val int) int {
		vBit := uint32(1) << uint(val)
		for p := 0; p < d.N; p++ {
			if d.get(cat, p) == vBit {
				return p
			}
		}
		return -1
	}
	for _, c := range clues {
		if c.Type != Neighbor && c.Type != DirectionalNeighbor {
			continue
		}
		pa := posOf(c.CatA, c.ValA)
		pb := posOf(c.CatB, c.ValB)
		if pa < 0 || pb < 0 {
			return false
		}
		switch c.Type {
		case Neighbor:
			diff := pa - pb
			if diff != 1 && diff != -1 {
				return false
			}
		case DirectionalNeighbor:
			if pa != pb+1 {
				return false
			}
		}
	}
	return true
}

// checkPartialRelational: tidig backtrack. När BÅDA värdena i en relationsklyva
// har fått en position tilldelad (singleton), verifiera villkoret direkt.
func checkPartialRelational(d *Domains, clues []Clue) bool {
	posOfIfFixed := func(cat, val int) int {
		vBit := uint32(1) << uint(val)
		for p := 0; p < d.N; p++ {
			if d.get(cat, p) == vBit {
				return p
			}
		}
		return -1
	}
	for _, c := range clues {
		if c.Type != Neighbor && c.Type != DirectionalNeighbor {
			continue
		}
		pa := posOfIfFixed(c.CatA, c.ValA)
		pb := posOfIfFixed(c.CatB, c.ValB)
		if pa < 0 || pb < 0 {
			continue // inte båda fixerade än
		}
		switch c.Type {
		case Neighbor:
			diff := pa - pb
			if diff != 1 && diff != -1 {
				return false
			}
		case DirectionalNeighbor:
			if pa != pb+1 {
				return false
			}
		}
	}
	return true
}

// allSingletons rapporterar om alla domäner är fixerade.
func allSingletons(d *Domains) bool {
	for i := 0; i < len(d.mask); i++ {
		if popcount(d.mask[i]) != 1 {
			return false
		}
	}
	return true
}

// pickCell väljer cell (cat,pos) med minst domän >1 (MRV).
func pickCell(d *Domains) (cat, pos int, ok bool) {
	best := 1 << 30
	for c := 0; c < d.Categories; c++ {
		for p := 0; p < d.N; p++ {
			sz := popcount(d.get(c, p))
			if sz > 1 && sz < best {
				best = sz
				cat, pos, ok = c, p, true
			}
		}
	}
	return
}

// solveCount räknar lösningar (0, 1 eller 2=">=2") med tidigt avbrott.
func solveCount(d *Domains, clues []Clue) int {
	if !propagate(d, clues) {
		return 0
	}
	if !checkPartialRelational(d, clues) {
		return 0
	}
	if allSingletons(d) {
		if relationalHold(d, clues) {
			return 1
		}
		return 0
	}
	cat, pos, ok := pickCell(d)
	if !ok {
		return 0
	}
	m := d.get(cat, pos)
	count := 0
	for v := 0; v < d.N; v++ {
		vBit := uint32(1) << uint(v)
		if m&vBit == 0 {
			continue
		}
		branch := d.copy()
		branch.set(cat, pos, vBit)
		count += solveCount(branch, clues)
		if count >= 2 {
			return 2
		}
	}
	return count
}

// CountSolutions är publika ingången: bygg initiala domäner och räkna.
// Returnerar 0, 1, eller 2 (=">=2").
func CountSolutions(n, categories int, clues []Clue) int {
	d := newDomains(n, categories)
	return solveCount(d, clues)
}

// Solve returnerar (Solution, unique) där unique==true endast om exakt en lösning.
// Om ingen eller flera lösningar finns returneras unique==false.
func Solve(n, categories int, clues []Clue) (Solution, bool) {
	d := newDomains(n, categories)
	var found *Domains
	count := 0
	var rec func(dd *Domains)
	rec = func(dd *Domains) {
		if count >= 2 {
			return
		}
		if !propagate(dd, clues) {
			return
		}
		if !checkPartialRelational(dd, clues) {
			return
		}
		if allSingletons(dd) {
			if relationalHold(dd, clues) {
				count++
				if count == 1 {
					found = dd.copy()
				}
			}
			return
		}
		cat, pos, ok := pickCell(dd)
		if !ok {
			return
		}
		m := dd.get(cat, pos)
		for v := 0; v < dd.N; v++ {
			vBit := uint32(1) << uint(v)
			if m&vBit == 0 {
				continue
			}
			branch := dd.copy()
			branch.set(cat, pos, vBit)
			rec(branch)
			if count >= 2 {
				return
			}
		}
	}
	rec(d)
	if count != 1 {
		return Solution{}, false
	}
	return domainsToSolution(found), true
}

func domainsToSolution(d *Domains) Solution {
	sol := Solution{N: d.N, Categories: d.Categories}
	sol.Assignment = make([][]ValueID, d.Categories)
	for cat := 0; cat < d.Categories; cat++ {
		sol.Assignment[cat] = make([]ValueID, d.N)
		for pos := 0; pos < d.N; pos++ {
			sol.Assignment[cat][pos] = ValueID(bitIndex(d.get(cat, pos)))
		}
	}
	return sol
}
