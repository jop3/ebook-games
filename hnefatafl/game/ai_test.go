package game

import (
	"image"
	"testing"
)

func TestBestMoveLegalFromStartingPositionBothSides(t *testing.T) {
	b := NewBoard()
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		if m, ok := BestMove(b, SideAttacker, depth); !ok || !b.IsLegalMove(SideAttacker, m) {
			t.Fatalf("depth %d: attacker BestMove = %v,%v not legal", depth, m, ok)
		}
		if m, ok := BestMove(b, SideDefender, depth); !ok || !b.IsLegalMove(SideDefender, m) {
			t.Fatalf("depth %d: defender BestMove = %v,%v not legal", depth, m, ok)
		}
	}
}

func TestBestMoveNoMovesReturnsFalse(t *testing.T) {
	b := emptyBoard()
	// A lone attacker boxed in on all sides by defenders: no legal move.
	b.set(3, 3, Attacker)
	b.set(2, 3, Defender)
	b.set(4, 3, Defender)
	b.set(3, 2, Defender)
	b.set(3, 4, Defender)
	if _, ok := BestMove(b, SideAttacker, DepthEasy); ok {
		t.Fatal("BestMove should report no move when the side is completely boxed in")
	}
}

// --- Defender AI takes an immediate king-escape win -------------------------

func TestDefenderAITakesImmediateEscape(t *testing.T) {
	b := emptyBoard()
	b.set(0, 1, King) // one step from the corner (0,0)
	// Block the OTHER direction (straight down the same file also reaches a
	// corner, (0,6), with nothing else on the board in the way) so there is
	// only one winning escape and the assertion below is unambiguous.
	b.set(0, 3, Attacker)
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		m, ok := BestMove(b, SideDefender, depth)
		if !ok {
			t.Fatalf("depth %d: no move found", depth)
		}
		if m.From != (image.Point{X: 0, Y: 1}) || m.To != (image.Point{X: 0, Y: 0}) {
			t.Fatalf("depth %d: BestMove = %v, want the immediate escaping move to the corner", depth, m)
		}
	}
}

// --- Attacker AI takes an immediate king-capture win ------------------------

func TestAttackerAITakesImmediateKingCapture(t *testing.T) {
	b := emptyBoard()
	b.set(1, 1, King)
	b.set(1, 0, Attacker)
	b.set(1, 2, Attacker)
	b.set(0, 1, Attacker)
	b.set(2, 4, Attacker) // can slide up to (2,1), completing the surround
	for _, depth := range []int{DepthEasy, DepthMedium, DepthHard} {
		m, ok := BestMove(b, SideAttacker, depth)
		if !ok {
			t.Fatalf("depth %d: no move found", depth)
		}
		nb, res := b.Apply(m)
		if !res.KingCaptured {
			t.Fatalf("depth %d: BestMove %v did not take the immediate king-capturing move", depth, m)
		}
		if _, alive := nb.KingPos(); alive {
			t.Fatalf("depth %d: king should be gone from the board after the capture", depth)
		}
	}
}

// --- Evaluation function sanity: each favors its own side's progress -------

func TestEvalForAttackerPrefersKingFartherFromCorner(t *testing.T) {
	near := emptyBoard()
	near.set(1, 0, King) // close to corner (0,0)
	far := emptyBoard()
	far.set(3, 3, King) // the throne, farthest from every corner
	if evalForAttacker(&far) <= evalForAttacker(&near) {
		t.Fatalf("evalForAttacker should score the king farther from a corner higher: far=%d near=%d",
			evalForAttacker(&far), evalForAttacker(&near))
	}
}

func TestEvalForDefenderPrefersKingCloserToCorner(t *testing.T) {
	near := emptyBoard()
	near.set(1, 0, King)
	far := emptyBoard()
	far.set(3, 3, King)
	if evalForDefender(&near) <= evalForDefender(&far) {
		t.Fatalf("evalForDefender should score the king closer to a corner higher: near=%d far=%d",
			evalForDefender(&near), evalForDefender(&far))
	}
}

func TestEvalForAttackerRewardsContainment(t *testing.T) {
	loose := emptyBoard()
	loose.set(3, 1, King)
	boxed := emptyBoard()
	boxed.set(3, 1, King)
	boxed.set(2, 1, Attacker)
	boxed.set(4, 1, Attacker)
	boxed.set(3, 0, Attacker)
	if evalForAttacker(&boxed) <= evalForAttacker(&loose) {
		t.Fatalf("evalForAttacker should reward containment of the king: boxed=%d loose=%d",
			evalForAttacker(&boxed), evalForAttacker(&loose))
	}
}

func TestEvalForDefenderRewardsOpenEscapeLines(t *testing.T) {
	open := emptyBoard()
	open.set(3, 1, King)
	blocked := emptyBoard()
	blocked.set(3, 1, King)
	blocked.set(2, 1, Attacker)
	blocked.set(4, 1, Attacker)
	blocked.set(3, 0, Attacker)
	blocked.set(3, 2, Attacker)
	if evalForDefender(&open) <= evalForDefender(&blocked) {
		t.Fatalf("evalForDefender should reward the king having open escape lines: open=%d blocked=%d",
			evalForDefender(&open), evalForDefender(&blocked))
	}
}
