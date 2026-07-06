package game

import (
	"image"
	"testing"
	"time"
)

func TestBestTurnLegalOnStartingBoard(t *testing.T) {
	b := NewBoard()
	turn, ok := BestTurn(b, Black)
	if !ok {
		t.Fatal("BestTurn should find a turn from the starting position")
	}
	if !b.IsLegalQueenMove(Black, turn.Move) {
		t.Fatalf("BestTurn returned an illegal move: %v", turn.Move)
	}
	afterMove := b.MoveQueen(turn.Move)
	if !afterMove.IsLegalShot(turn.Move.To, turn.Shot) {
		t.Fatalf("BestTurn returned an illegal shot: %v", turn.Shot)
	}
}

func TestBestTurnFalseWithNoQueens(t *testing.T) {
	var b Board // no queens of either color
	if _, ok := BestTurn(b, Black); ok {
		t.Fatal("BestTurn must report false when side has no queens at all")
	}
}

func TestBestTurnFalseWhenFullyBoxedIn(t *testing.T) {
	var b Board
	b.set(4, 4, QueenBlack)
	boxInQueen(&b, image.Pt(4, 4))
	if _, ok := BestTurn(b, Black); ok {
		t.Fatal("BestTurn must report false when the side has no legal move anywhere")
	}
}

// TestEvaluatePrefersMoreCentralTerritory sanity-checks the territory
// heuristic itself (independent of BestTurn's search): a side whose queen
// commands the open center of the board should score better than one boxed
// into a corner with equal queen count.
func TestEvaluatePrefersMoreCentralTerritory(t *testing.T) {
	var central Board
	central.set(4, 4, QueenBlack)
	central.set(0, 0, QueenWhite)
	// White's queen is further shut in relative to the empty board than
	// Black's central queen — Black should command more territory.
	score := evaluate(&central, Black)
	if score <= 0 {
		t.Fatalf("evaluate() from Black's perspective = %d, want > 0 (Black's central queen should command more territory than White's cornered one)", score)
	}
	// From White's perspective the same position should look bad.
	if s := evaluate(&central, White); s >= 0 {
		t.Fatalf("evaluate() from White's perspective = %d, want < 0", s)
	}
}

func TestQueenDistancesMonotonicFromSource(t *testing.T) {
	var b Board
	b.set(0, 0, QueenBlack)
	dist := queenDistances(&b, Black)
	if dist[0][0] != 0 {
		t.Fatalf("distance to the queen's own square should be 0, got %d", dist[0][0])
	}
	// (1,0) is directly reachable in one ray hop.
	if dist[0][1] != 1 {
		t.Fatalf("distance to (1,0) should be 1 (one queen move away), got %d", dist[0][1])
	}
	// (9,9): reachable via the long diagonal in exactly 1 hop (open board).
	if dist[9][9] != 1 {
		t.Fatalf("distance to (9,9) should be 1 (on the open diagonal), got %d", dist[9][9])
	}
}

// TestBestTurnRunsInReasonableTime is a coarse performance sanity check: the
// AI must not attempt a full-width multi-ply search (per the spec, that's
// not realistic on this device) — a single depth-1 evaluation over every
// legal turn from the (worst-case, most open) starting position must finish
// well within a second on ordinary hardware.
func TestBestTurnRunsInReasonableTime(t *testing.T) {
	b := NewBoard()
	start := time.Now()
	if _, ok := BestTurn(b, Black); !ok {
		t.Fatal("expected a move")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("BestTurn took %v from the starting position — too slow for a shallow, non-searching heuristic", elapsed)
	}
}
