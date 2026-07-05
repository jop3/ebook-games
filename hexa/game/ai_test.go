package game

import (
	"testing"
	"time"
)

// buildDenseMovingPosition returns a GameState with a full, connected
// 42-tile board (21 Black, 21 White) in PhaseMoving and no shape already
// won — the actual worst case for BestMove's branching, since the movement
// phase always starts from a completely full board. Built directly (a
// breadth-first blob grown from the center, colors alternating in visit
// order) rather than via self-play AIPlacement, because self-play tends to
// cluster tiles and often completes a shape well before all 42 land,
// which would skip the very densest position this test needs to measure.
func buildDenseMovingPosition(t *testing.T) *GameState {
	t.Helper()
	tiles := map[Hex]Side{}
	added := []Hex{{0, 0, 0}}
	tiles[Hex{0, 0, 0}] = Black // placeholder color, reassigned below
	for len(added) < 2*TilesPerSide {
		frontier := PlaceMoves(tiles)
		if len(frontier) == 0 {
			t.Fatal("ran out of board space building the dense test position")
		}
		for _, p := range frontier {
			if len(added) >= 2*TilesPerSide {
				break
			}
			tiles[p] = Black // placeholder; real colors assigned below
			added = append(added, p)
		}
	}

	// Assign alternating colors in visit order; if that happens to complete
	// a shape (rare, but the whole point of the shape-check is that some
	// configurations DO win), swap adjacent-in-order tiles' colors until it
	// doesn't — a full board with no winner is what a real "into the
	// movement phase" game looks like.
	final := map[Hex]Side{}
	assign := func(swapAt int) {
		for i, p := range added {
			side := Black
			if i%2 == 1 {
				side = White
			}
			if i == swapAt || i == swapAt+1 {
				if side == Black {
					side = White
				} else {
					side = Black
				}
			}
			final[p] = side
		}
	}
	swapAt := -2
	for {
		assign(swapAt)
		bk, _ := HasShape(final, Black)
		wk, _ := HasShape(final, White)
		if bk == ShapeNone && wk == ShapeNone {
			break
		}
		swapAt += 2
		if swapAt >= len(added) {
			t.Fatal("could not find a shape-free color assignment for the dense test position")
		}
	}

	s := NewGame(OpponentAI, DepthEasy, false)
	s.Board.Tiles = final
	s.Remaining[Black], s.Remaining[White] = 0, 0
	s.Phase = PhaseMoving
	s.Turn = Black
	return s
}

func TestAIPlacementAlwaysLegal(t *testing.T) {
	s := NewGame(OpponentAI, DepthEasy, false)
	for i := 0; i < 2*TilesPerSide; i++ {
		p, ok := AIPlacement(s)
		if !ok {
			t.Fatalf("step %d: AIPlacement found no legal move on a non-full board", i)
		}
		before := len(s.Board.Tiles)
		if !s.PlaceTile(p) {
			t.Fatalf("step %d: AIPlacement proposed illegal placement %v", i, p)
		}
		if len(s.Board.Tiles) != before+1 {
			t.Fatalf("step %d: placement should add exactly one tile", i)
		}
		if s.Phase == PhaseDone {
			return
		}
	}
}

func TestBestMoveAlwaysLegal(t *testing.T) {
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		s := buildDenseMovingPosition(t)
		if s.Phase != PhaseMoving {
			continue // a shape completed while building the position; nothing to test here
		}
		s.AIDepth = depth
		for i := 0; i < 5 && s.Phase == PhaseMoving; i++ {
			m, ok := BestMove(s)
			if !ok {
				t.Fatalf("depth %d: BestMove found no legal move on a full board", depth)
			}
			if !s.MoveTile(m.From, m.To) {
				t.Fatalf("depth %d: BestMove proposed illegal move %v->%v", depth, m.From, m.To)
			}
		}
	}
}

// TestBestMoveTakesAnImmediateWin confirms the search actually recognizes
// and grabs a one-move win when one is available, rather than just playing
// "a legal move".
func TestBestMoveTakesAnImmediateWin(t *testing.T) {
	s := NewGame(OpponentAI, DepthEasy, false)
	s.Phase = PhaseMoving
	s.Remaining[Black], s.Remaining[White] = 0, 0

	center := Hex{0, 0, 0}
	ring := Neighbors(center)
	parked := ring[0].Add(Directions[0])
	tiles := map[Hex]Side{}
	for _, p := range ring[:5] {
		tiles[p] = White
	}
	tiles[parked] = White
	s.Board.Tiles = tiles
	s.Turn = White

	m, ok := BestMove(s)
	if !ok {
		t.Fatal("BestMove should find the winning move")
	}
	if m.From != parked || m.To != ring[5] {
		t.Fatalf("BestMove should slide the parked tile into the final ring cell, got %v->%v", m.From, m.To)
	}
}

// TestAIWallClockTiming measures BestMove's actual search time at each menu
// difficulty on a dense, realistic 42-tile board (the worst case for
// branching) — the same discipline ringar/game/ai_test.go used to pick its
// own AIDepth: measure on real hardware-adjacent conditions and only ship a
// depth that stays comfortably fast, rather than assuming.
func TestAIWallClockTiming(t *testing.T) {
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		s := buildDenseMovingPosition(t)
		if s.Phase != PhaseMoving {
			t.Skip("position resolved into a win while building; re-run to measure timing")
		}
		s.AIDepth = depth
		start := time.Now()
		_, ok := BestMove(s)
		elapsed := time.Since(start)
		if !ok {
			t.Fatalf("depth %d: BestMove found no legal move", depth)
		}
		t.Logf("depth %d: BestMove took %v on a full 42-tile board", depth, elapsed)
	}
}
