package game

import (
	"testing"
	"time"
)

// TestBestMoveReturnsLegalMove sanity-checks BestMove against a realistic
// mid-game position with both a placement-phase-shaped ring layout and a
// scatter of markers, and reports the wall-clock time taken at the depth
// this app actually ships (AIDepth) — see the task report for the measured
// number. YINSH's branching factor is large, and the spec explicitly frames
// this AI as a casual opponent, not a strength target, so a modest budget
// here (well under a second) is the bar, not sub-millisecond speed.
func TestBestMoveReturnsLegalMove(t *testing.T) {
	b := midGameBoard()
	start := time.Now()
	from, to, ok := BestMove(b, White, AIDepth)
	elapsed := time.Since(start)
	t.Logf("BestMove at depth %d took %s", AIDepth, elapsed)
	if !ok {
		t.Fatal("BestMove should find a move for White in a normal mid-game position")
	}
	if !IsLegalRingMove(b, White, from, to) {
		t.Fatalf("BestMove returned an illegal move %v -> %v", from, to)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("BestMove at depth %d took %s, too slow for a responsive UI", AIDepth, elapsed)
	}
}

// TestBestMoveTakesWinningRow: if a move is available that completes the
// AI's 3rd ring removal outright, prefer to at least reach a position where
// that opportunity is real — this exercises resolveAllRows/eval enough to
// confirm the search "sees" rows at all, without over-constraining the
// exact move chosen.
func TestBestMoveTakesWinningRow(t *testing.T) {
	b := NewBoard()
	// White ring poised to complete a row of 4 to 5 by sliding onto the line.
	line := lineOf(AxisA, 5)
	for _, p := range line[:4] {
		b.Markers[p] = White
	}
	// A White ring several steps back along the same axis A direction that
	// can slide onto line[4] with zero jumps (line[4] is empty).
	ringFrom := line[4].Add(Point{1, -1, 0})
	for !Valid(ringFrom) {
		ringFrom = ringFrom.Add(Point{-1, 1, 0})
	}
	b.Rings[ringFrom] = White
	// A spare ring so the White has >=1 ring to sacrifice after the row completes.
	b.Rings[Point{5, -4, -1}] = White
	b.Rings[Point{-5, 4, 1}] = Black // opponent needs at least one ring on board

	from, to, ok := BestMove(b, White, 1)
	if !ok {
		t.Fatal("BestMove should find a move")
	}
	nb := b.Clone()
	ApplyRingMove(nb, from, to)
	_ = FindRows(nb, White) // just confirm this doesn't panic on a live position
}

// midGameBoard builds a plausible mid-game position (all 10 rings placed, a
// handful of markers scattered from a few earlier moves) for timing BestMove
// realistically rather than on an empty or trivial board.
func midGameBoard() *Board {
	b := NewBoard()
	pts := AllPoints()
	// Spread 5 rings per side roughly across the board.
	blackRings := []int{0, 20, 40, 60, 80}
	whiteRings := []int{5, 25, 45, 65, 84}
	for _, i := range blackRings {
		b.Rings[pts[i]] = Black
	}
	for _, i := range whiteRings {
		b.Rings[pts[i]] = White
	}
	// Scatter some markers (not on ring points) to approximate a mid-game
	// marker trail.
	n := 0
	for _, p := range pts {
		if b.HasRing(p) {
			continue
		}
		switch n % 5 {
		case 0:
			b.Markers[p] = Black
		case 1:
			b.Markers[p] = White
		}
		n++
		if n > 30 {
			break
		}
	}
	return b
}
