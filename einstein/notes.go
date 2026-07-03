package main

// notes.go — anteckningsrutnätets tillstånd (§6.9), FRI från inkview.
// Cellen (catA,valA) x (catB,valB) kan vara Unknown, Possible eller Impossible.
// Spelarens inlämnade svar deriveras separat.

type Mark int

const (
	Unknown Mark = iota
	Possible
	Impossible
)

// NoteGrid lagrar markeringar för alla par av (kategori,värde) mellan
// olika kategorier. Symmetriskt: mark(a,b) == mark(b,a).
type NoteGrid struct {
	N          int
	Categories int
	// marks[catA][valA][catB][valB]
	marks map[[4]int]Mark
}

func NewNoteGrid(n, categories int) *NoteGrid {
	return &NoteGrid{N: n, Categories: categories, marks: map[[4]int]Mark{}}
}

func key(ca, va, cb, vb int) [4]int { return [4]int{ca, va, cb, vb} }

// Get returnerar markeringen för paret (symmetriskt).
func (g *NoteGrid) Get(ca, va, cb, vb int) Mark {
	if m, ok := g.marks[key(ca, va, cb, vb)]; ok {
		return m
	}
	return Unknown
}

// Set sätter markeringen (och den symmetriska motsvarigheten).
func (g *NoteGrid) Set(ca, va, cb, vb int, m Mark) {
	if m == Unknown {
		delete(g.marks, key(ca, va, cb, vb))
		delete(g.marks, key(cb, vb, ca, va))
		return
	}
	g.marks[key(ca, va, cb, vb)] = m
	g.marks[key(cb, vb, ca, va)] = m
}

// Cycle roterar Unknown -> Possible -> Impossible -> Unknown.
func (g *NoteGrid) Cycle(ca, va, cb, vb int) {
	switch g.Get(ca, va, cb, vb) {
	case Unknown:
		g.Set(ca, va, cb, vb, Possible)
	case Possible:
		g.Set(ca, va, cb, vb, Impossible)
	default:
		g.Set(ca, va, cb, vb, Unknown)
	}
}

// PlayerSolution försöker härleda spelarens svar från "Possible"-markeringar.
// Vi tolkar det via en referenskategori (kategori 0 = positioner via värde-index):
// för varje kategori och värde, hitta vilket värde i kategori 0 det är "Possible"
// med. Detta är en förenkling; för validering använder vi istället AssignByAnchor.

// DerivePlacement mappar värden till positioner utifrån "Possible"-markeringar
// relativt en ankarkategori (anchor). Returnerar Assignment[cat][pos] och ok.
// ok=false om spelaren inte fyllt i tillräckligt entydigt.
func (g *NoteGrid) DerivePlacement(anchor int) ([][]ValueID, bool) {
	n, cats := g.N, g.Categories
	// Ankarkategorins värde v antas ligga på position v (kanonisk ordning).
	assign := make([][]ValueID, cats)
	for c := 0; c < cats; c++ {
		assign[c] = make([]ValueID, n)
		for p := 0; p < n; p++ {
			assign[c][p] = -1
		}
	}
	for pos := 0; pos < n; pos++ {
		assign[anchor][pos] = ValueID(pos)
	}
	for c := 0; c < cats; c++ {
		if c == anchor {
			continue
		}
		used := make([]bool, n)
		for pos := 0; pos < n; pos++ {
			anchorVal := pos // kanonisk
			placed := -1
			for v := 0; v < n; v++ {
				if used[v] {
					continue
				}
				if g.Get(anchor, anchorVal, c, v) == Possible {
					if placed != -1 {
						return nil, false // tvetydigt
					}
					placed = v
				}
			}
			if placed == -1 {
				return nil, false // ofullständigt
			}
			assign[c][pos] = ValueID(placed)
			used[placed] = true
		}
	}
	return assign, true
}

// CheckAgainst jämför spelarens härledda placering med facit-lösningen.
// Eftersom ankaret använder kanonisk ordning måste vi jämföra "hör ihop"-
// relationer snarare än absoluta positioner. Vi jämför parvisa samhörigheter.
func (g *NoteGrid) CheckAgainst(sol Solution, anchor int) (correct bool, complete bool) {
	assign, ok := g.DerivePlacement(anchor)
	if !ok {
		return false, false
	}
	// Bygg för spelaren: value->pos per kategori.
	// Jämför samhörighet: för varje kategori-par och position i facit,
	// säkerställ att spelarens gruppering matchar.
	n, cats := g.N, g.Categories
	// Facit: pos i facit -> map cat->val.
	// Spelare: pos (kanonisk) -> map cat->val.
	// Vi matchar genom att för varje facit-position hitta motsvarande
	// spelarposition via ankarvärdet.
	solPosOfAnchorVal := make([]int, n)
	for p := 0; p < n; p++ {
		solPosOfAnchorVal[int(sol.Assignment[anchor][p])] = p
	}
	for playerPos := 0; playerPos < n; playerPos++ {
		anchorVal := int(assign[anchor][playerPos]) // == playerPos
		solPos := solPosOfAnchorVal[anchorVal]
		for c := 0; c < cats; c++ {
			if c == anchor {
				continue
			}
			if int(assign[c][playerPos]) != int(sol.Assignment[c][solPos]) {
				return false, true
			}
		}
	}
	return true, true
}
