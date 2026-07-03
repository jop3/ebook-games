package game

import "testing"

func emptyEdges(W, H int) (hEdge, vEdge [][]EdgeState) {
	hEdge = make([][]EdgeState, H+1)
	for y := range hEdge {
		hEdge[y] = make([]EdgeState, W)
	}
	vEdge = make([][]EdgeState, H)
	for y := range vEdge {
		vEdge[y] = make([]EdgeState, W+1)
	}
	return
}

// singleBoxLoop returns the edges tracing the border of a 1x1 box at (x0,y0).
func setBox(hEdge, vEdge [][]EdgeState, x0, y0 int) {
	hEdge[y0][x0] = EdgeOn
	hEdge[y0+1][x0] = EdgeOn
	vEdge[y0][x0] = EdgeOn
	vEdge[y0][x0+1] = EdgeOn
}

func TestIsSingleLoop_SingleBox(t *testing.T) {
	W, H := 5, 5
	hEdge, vEdge := emptyEdges(W, H)
	setBox(hEdge, vEdge, 1, 1)
	if !IsSingleLoop(hEdge, vEdge, W, H) {
		t.Fatal("expected a single 1x1 box to be recognized as a single loop")
	}
}

func TestIsSingleLoop_TwoSeparateLoops(t *testing.T) {
	W, H := 5, 5
	hEdge, vEdge := emptyEdges(W, H)
	setBox(hEdge, vEdge, 0, 0)
	setBox(hEdge, vEdge, 3, 3)
	if IsSingleLoop(hEdge, vEdge, W, H) {
		t.Fatal("two disjoint loops must NOT be reported as a single loop")
	}
}

func TestIsSingleLoop_NoEdges(t *testing.T) {
	W, H := 5, 5
	hEdge, vEdge := emptyEdges(W, H)
	if IsSingleLoop(hEdge, vEdge, W, H) {
		t.Fatal("empty board must not be a loop")
	}
}

func TestIsSingleLoop_DanglingBranch(t *testing.T) {
	// A box with an extra "tail" edge sticking out — a dot with degree 3 or 1.
	W, H := 5, 5
	hEdge, vEdge := emptyEdges(W, H)
	setBox(hEdge, vEdge, 1, 1)
	// add a stray edge from a corner of the box outward, creating a degree-3 dot
	hEdge[1][0] = EdgeOn // extends left from dot (1,1)
	if IsSingleLoop(hEdge, vEdge, W, H) {
		t.Fatal("a branch off the loop must not be reported as a single loop")
	}
}

func TestIsSingleLoop_Rectangle2x2(t *testing.T) {
	W, H := 5, 5
	hEdge, vEdge := emptyEdges(W, H)
	// 2x2 block loop from (0,0) to (2,2)
	hEdge[0][0] = EdgeOn
	hEdge[0][1] = EdgeOn
	hEdge[2][0] = EdgeOn
	hEdge[2][1] = EdgeOn
	vEdge[0][0] = EdgeOn
	vEdge[1][0] = EdgeOn
	vEdge[0][2] = EdgeOn
	vEdge[1][2] = EdgeOn
	if !IsSingleLoop(hEdge, vEdge, W, H) {
		t.Fatal("2x2 rectangle should be a single loop")
	}
}

func TestBoard_ToggleCycles(t *testing.T) {
	p := &Puzzle{W: 3, H: 3, Clue: make([][]int, 3)}
	for i := range p.Clue {
		p.Clue[i] = []int{-1, -1, -1}
	}
	b := NewBoard(p)
	if b.HEdge[0][0] != EdgeUnknown {
		t.Fatal("expected initial state Unknown")
	}
	b.ToggleH(0, 0)
	if b.HEdge[0][0] != EdgeOn {
		t.Fatal("expected On after first toggle")
	}
	b.ToggleH(0, 0)
	if b.HEdge[0][0] != EdgeOff {
		t.Fatal("expected Off after second toggle")
	}
	b.ToggleH(0, 0)
	if b.HEdge[0][0] != EdgeUnknown {
		t.Fatal("expected Unknown after third toggle (full cycle)")
	}
}

func TestBoard_SolvedRequiresLoopAndClues(t *testing.T) {
	p := &Puzzle{W: 3, H: 3, Clue: make([][]int, 3)}
	for y := range p.Clue {
		p.Clue[y] = []int{-1, -1, -1}
	}
	p.Clue[1][1] = 4 // center cell must have all 4 edges on
	b := NewBoard(p)
	if b.Solved() {
		t.Fatal("empty board must not be solved")
	}
	// Set the box around (1,1).
	b.HEdge[1][1] = EdgeOn
	b.HEdge[2][1] = EdgeOn
	b.VEdge[1][1] = EdgeOn
	b.VEdge[1][2] = EdgeOn
	if !b.Solved() {
		t.Fatal("expected solved: clue satisfied and forms a single loop")
	}
}
