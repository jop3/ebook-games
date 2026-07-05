package game

import (
	"math/rand"
	"testing"
)

func TestNewGameHasTwoTiles(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	s := NewGame(2048, 0, rng)
	count := 0
	for _, v := range s.Board {
		if v != 0 {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected 2 starting tiles, got %d", count)
	}
	if s.Status != StatusPlaying {
		t.Fatalf("expected StatusPlaying, got %v", s.Status)
	}
}

func TestMoveNoChangeDoesNotSpawnOrScore(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	s := &GameState{Target: 2048, rng: rng}
	s.Board = mkBoard([Size][Size]int{
		{2, 4, 8, 16},
		{4, 8, 16, 2},
		{8, 16, 2, 4},
		{16, 2, 4, 8},
	}) // fully packed, no equal neighbours: every direction is a no-op
	before := s.Board
	if s.Move(Left) {
		t.Fatal("expected Move to report false for a no-op swipe")
	}
	if s.Board != before {
		t.Fatal("board changed on a no-op swipe (should not spawn)")
	}
	if s.Score != 0 {
		t.Fatal("score changed on a no-op swipe")
	}
}

func TestMoveScoresAndUpdatesBest(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	s := &GameState{Target: 2048, rng: rng}
	s.Board = mkBoard([Size][Size]int{
		{2, 2, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	})
	if !s.Move(Left) {
		t.Fatal("expected Move to apply")
	}
	if s.Score != 4 {
		t.Fatalf("score = %d, want 4", s.Score)
	}
	if s.Best != 4 {
		t.Fatalf("best = %d, want 4", s.Best)
	}
	// A spawned tile must exist in addition to the merged 4.
	nonzero := 0
	for _, v := range s.Board {
		if v != 0 {
			nonzero++
		}
	}
	if nonzero != 2 {
		t.Fatalf("expected merged tile + 1 spawn = 2 nonzero cells, got %d", nonzero)
	}
}

func TestMoveBestPersistsAcrossLowerScoringMoves(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	s := &GameState{Target: 99999, Best: 100, rng: rng}
	s.Board = mkBoard([Size][Size]int{
		{2, 2, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	})
	s.Move(Left) // scores 4, far below the existing best of 100
	if s.Best != 100 {
		t.Fatalf("best regressed to %d, want unchanged 100", s.Best)
	}
}

func TestWinDetectionAndContinue(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	s := &GameState{Target: 2048, rng: rng}
	s.Board = mkBoard([Size][Size]int{
		{1024, 1024, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	})
	s.Move(Left) // 1024+1024 -> 2048
	if s.Status != StatusWon {
		t.Fatalf("status = %v, want StatusWon", s.Status)
	}
	s.Continue()
	if s.Status != StatusPlaying {
		t.Fatalf("status after Continue = %v, want StatusPlaying", s.Status)
	}
	if !s.WonSeen {
		t.Fatal("WonSeen should be set after Continue")
	}
	// Winning again later (e.g. a second 2048) must not re-trigger the banner.
	s.Board = mkBoard([Size][Size]int{
		{2048, 2, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 0, 0},
	})
	s.Move(Right)
	if s.Status == StatusWon {
		t.Fatal("win banner re-triggered after Continue; should only show once")
	}
}

func TestGameOverDetection(t *testing.T) {
	// statusFor is what Move calls right after every spawn; test it directly
	// against a hand-built full, dead board so the result doesn't depend on
	// which value a real Spawn happens to roll.
	dead := mkBoard([Size][Size]int{
		{2, 4, 2, 4},
		{4, 2, 4, 2},
		{2, 4, 2, 4},
		{4, 2, 4, 2},
	})
	if got := statusFor(dead, 2048, false); got != StatusOver {
		t.Fatalf("statusFor(dead board) = %v, want StatusOver", got)
	}

	rng := rand.New(rand.NewSource(1))
	s := &GameState{Target: 2048, Status: StatusOver, rng: rng}
	s.Board = dead
	if s.Move(Up) {
		t.Fatal("no move should be accepted once the game is over")
	}
}

// --- Best-score persistence encode/decode -----------------------------------

func TestParseBestValid(t *testing.T) {
	if got := ParseBest([]byte("1234")); got != 1234 {
		t.Fatalf("ParseBest = %d, want 1234", got)
	}
	if got := ParseBest([]byte(" 42 \n")); got != 42 {
		t.Fatalf("ParseBest with whitespace = %d, want 42", got)
	}
}

func TestParseBestMissingOrCorruptDefaultsZero(t *testing.T) {
	cases := [][]byte{nil, []byte(""), []byte("not a number"), []byte("-5"), []byte("\x00\x01garbage")}
	for _, c := range cases {
		if got := ParseBest(c); got != 0 {
			t.Fatalf("ParseBest(%q) = %d, want 0", c, got)
		}
	}
}

func TestFormatBestRoundTrips(t *testing.T) {
	for _, n := range []int{0, 4, 123456} {
		if got := ParseBest(FormatBest(n)); got != n {
			t.Fatalf("round trip %d -> %q -> %d", n, FormatBest(n), got)
		}
	}
}
