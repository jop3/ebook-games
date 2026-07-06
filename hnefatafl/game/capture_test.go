package game

import (
	"image"
	"testing"
)

// --- Basic custodial capture (ordinary pieces, no throne involved) ----------

func TestCaptureSingleAttackerCapturesDefender(t *testing.T) {
	b := emptyBoard()
	b.set(0, 2, Attacker) // mover, slides right to (2,2)
	b.set(3, 2, Defender) // sandwiched
	b.set(4, 2, Attacker) // the other bracket, already in place
	nb, res := b.Apply(Move{From: image.Pt(0, 2), To: image.Pt(2, 2)})
	if !ptsEqual(res.Captured, []image.Point{{X: 3, Y: 2}}) {
		t.Fatalf("captured = %v, want [(3,2)]", res.Captured)
	}
	if nb.At(3, 2) != Empty {
		t.Fatalf("(3,2) should be empty after capture, got %v", nb.At(3, 2))
	}
	if res.KingCaptured {
		t.Fatal("no king involved; KingCaptured must be false")
	}
}

func TestCaptureDefenderCapturesAttacker(t *testing.T) {
	b := emptyBoard()
	b.set(0, 2, Defender)
	b.set(3, 2, Attacker) // sandwiched
	b.set(4, 2, Defender)
	nb, res := b.Apply(Move{From: image.Pt(0, 2), To: image.Pt(2, 2)})
	if !ptsEqual(res.Captured, []image.Point{{X: 3, Y: 2}}) {
		t.Fatalf("captured = %v, want [(3,2)]", res.Captured)
	}
	if nb.At(3, 2) != Empty {
		t.Fatal("the sandwiched Attacker should be captured")
	}
}

func TestCaptureRunOfMultiplePieces(t *testing.T) {
	b := emptyBoard()
	b.set(1, 4, Defender)
	b.set(2, 4, Defender)
	b.set(3, 4, Attacker) // far bracket, already in place
	b.set(0, 0, Attacker) // mover: will land at (0,4), the near bracket
	nb, res := b.Apply(Move{From: image.Pt(0, 0), To: image.Pt(0, 4)})
	want := []image.Point{{X: 1, Y: 4}, {X: 2, Y: 4}}
	if !ptsEqual(res.Captured, want) {
		t.Fatalf("captured = %v, want %v (whole 2-piece run)", res.Captured, want)
	}
	_ = nb
}

// --- GOTCHA: safe entry — moving into the gap between two enemies ----------

func TestSafeEntryIsNotSelfCapture(t *testing.T) {
	b := emptyBoard()
	b.set(2, 4, Defender)
	b.set(4, 4, Defender)
	b.set(3, 0, Attacker) // mover, slides down column 3 to (3,4)
	nb, res := b.Apply(Move{From: image.Pt(3, 0), To: image.Pt(3, 4)})
	if len(res.Captured) != 0 {
		t.Fatalf("moving into the gap between two enemies must NOT self-capture, got %v", res.Captured)
	}
	if nb.At(3, 4) != Attacker {
		t.Fatal("the mover's own piece must remain on the board")
	}
}

// --- THE gotcha: empty throne is hostile to BOTH sides ----------------------

func TestEmptyThroneHostileToAttackerCapturingDefender(t *testing.T) {
	// The king has left the throne, leaving it empty. An attacker uses the
	// empty throne as one bracket to capture a defender adjacent to it.
	b := emptyBoard()
	// Throne at (3,3) is empty (king already elsewhere, not placed here).
	b.set(3, 3, Empty)
	b.set(3, 2, Defender) // sandwiched directly north of the throne
	b.set(3, 0, Attacker) // mover: slides down to (3,1), bracketing with the throne at (3,3)...
	// Correction: the defender at (3,2) must be bracketed between the mover's
	// landing square and the throne. Mover should land at (3,1) so the run
	// (3,2) sits directly between (3,1) [mover] and (3,3) [throne].
	nb, res := b.Apply(Move{From: image.Pt(3, 0), To: image.Pt(3, 1)})
	if !ptsEqual(res.Captured, []image.Point{{X: 3, Y: 2}}) {
		t.Fatalf("captured = %v, want [(3,2)] — the empty throne must act as a hostile bracket for the ATTACKER", res.Captured)
	}
	if nb.At(3, 2) != Empty {
		t.Fatal("the defender bracketed against the empty throne should be captured")
	}
}

func TestEmptyThroneHostileToDefenderCapturingAttacker(t *testing.T) {
	// Symmetric case: the empty throne also brackets an ATTACKER for the
	// defending side — the detail fan implementations most often get wrong
	// (treating the throne as hostile to attackers only).
	b := emptyBoard()
	b.set(3, 3, Empty)
	b.set(3, 2, Attacker) // sandwiched directly north of the throne
	b.set(3, 0, Defender) // mover: slides down to (3,1)
	nb, res := b.Apply(Move{From: image.Pt(3, 0), To: image.Pt(3, 1)})
	if !ptsEqual(res.Captured, []image.Point{{X: 3, Y: 2}}) {
		t.Fatalf("captured = %v, want [(3,2)] — the empty throne must ALSO act as a hostile bracket for the DEFENDER", res.Captured)
	}
	if nb.At(3, 2) != Empty {
		t.Fatal("the attacker bracketed against the empty throne should be captured")
	}
}

func TestOccupiedThroneIsNotHostileToAttacker(t *testing.T) {
	// When the king sits ON the throne, the throne is not "empty" and must
	// not act as a hostile bracket for the ATTACKER — only the king's own
	// separate surround rule can threaten a king-occupied throne, never the
	// ordinary custodial-capture bracket check (isBracket only special-cases
	// the EMPTY throne; a king-occupied one falls through to the ordinary
	// ownership check, which favors the defending side, not the attacker).
	b := emptyBoard()
	b.set(3, 3, King)
	b.set(3, 2, Defender) // would be "sandwiched" against the throne if an occupied throne were attacker-hostile
	b.set(3, 0, Attacker) // mover: slides down to (3,1)
	_, res := b.Apply(Move{From: image.Pt(3, 0), To: image.Pt(3, 1)})
	if len(res.Captured) != 0 {
		t.Fatalf("a king-OCCUPIED throne must not act as a hostile bracket for the attacker, got %v", res.Captured)
	}
}

// --- King's own capture rule is separate from ordinary custodial capture ---

func TestKingIsNotCapturedByOrdinaryTwoSidedSandwich(t *testing.T) {
	// King on an open square, flanked north and south by attackers, but NOT
	// flanked east/west: this must NOT capture the king via the ordinary
	// 2-piece custodial rule — the king needs the special surround check.
	b := emptyBoard()
	b.set(1, 1, King) // an open square, nowhere near the throne
	b.set(1, 0, Attacker)
	b.set(1, 3, Attacker) // mover, slides up to (1,2), completing north+south flank
	nb, res := b.Apply(Move{From: image.Pt(1, 3), To: image.Pt(1, 2)})
	if res.KingCaptured {
		t.Fatal("a mere 2-sided vertical sandwich must not capture the king")
	}
	if nb.At(1, 1) != King {
		t.Fatal("the king must remain on the board")
	}
}

// --- King capture: 4 attackers required on an open square -------------------

func TestKingCaptureOpenSquareNeedsFourAttackers(t *testing.T) {
	b := emptyBoard()
	b.set(1, 1, King) // open square, not throne-adjacent
	b.set(1, 0, Attacker)
	b.set(1, 2, Attacker)
	b.set(0, 1, Attacker)
	// Only 3 of 4 sides covered — not yet captured.
	b.set(4, 4, Defender) // irrelevant filler, keeps the board non-trivial
	mover := b
	mover.set(2, 5, Attacker) // will slide to (2,1), completing the 4th side
	_, res := mover.Apply(Move{From: image.Pt(2, 5), To: image.Pt(2, 1)})
	if !res.KingCaptured {
		t.Fatal("4 attackers surrounding the king on an open square must capture it")
	}
}

func TestKingCaptureOpenSquareThreeAttackersInsufficient(t *testing.T) {
	b := emptyBoard()
	b.set(1, 1, King)
	b.set(1, 0, Attacker)
	b.set(1, 2, Attacker)
	b.set(2, 4, Attacker) // mover: slides to (2,1), only 3 of 4 sides covered (west still open)
	_, res := b.Apply(Move{From: image.Pt(2, 4), To: image.Pt(2, 1)})
	if res.KingCaptured {
		t.Fatal("only 3 of 4 sides covered on an open square must NOT capture the king")
	}
}

// --- King capture: only 3 attackers needed adjacent to the empty throne ----

func TestKingCaptureAdjacentToThroneNeedsOnlyThreeAttackers(t *testing.T) {
	// King at (3,2), directly north of the empty throne (3,3): the throne
	// supplies the south side "for free", so only north/east/west need
	// attackers.
	b := emptyBoard()
	b.set(3, 2, King)
	b.set(3, 3, Empty) // throne, empty
	b.set(3, 1, Attacker)
	b.set(2, 2, Attacker)
	b.set(4, 5, Attacker) // mover: slides to (4,2), completing the 3rd side (east)
	_, res := b.Apply(Move{From: image.Pt(4, 5), To: image.Pt(4, 2)})
	if !res.KingCaptured {
		t.Fatal("3 attackers plus the adjacent empty throne must capture the king")
	}
}

func TestKingCaptureAdjacentToThroneTwoAttackersInsufficient(t *testing.T) {
	b := emptyBoard()
	b.set(3, 2, King)
	b.set(3, 3, Empty)
	b.set(3, 1, Attacker)
	b.set(4, 5, Attacker) // mover: slides to (4,2); only 2 of the 3 needed sides covered
	_, res := b.Apply(Move{From: image.Pt(4, 5), To: image.Pt(4, 2)})
	if res.KingCaptured {
		t.Fatal("only 2 attackers (throne + 1) adjacent to the throne must NOT capture the king")
	}
}

// --- King capture: 4 attackers still required when king is ON the throne ---

func TestKingCaptureOnThroneStillNeedsFourAttackers(t *testing.T) {
	b := emptyBoard()
	b.set(throneCell.X, throneCell.Y, King)
	b.set(throneCell.X-1, throneCell.Y, Attacker)
	b.set(throneCell.X+1, throneCell.Y, Attacker)
	b.set(throneCell.X, throneCell.Y-1, Attacker)
	b.set(throneCell.X, throneCell.Y+2, Attacker) // mover: slides to throneCell.Y+1, completing all 4 sides
	_, res := b.Apply(Move{From: image.Pt(throneCell.X, throneCell.Y+2), To: image.Pt(throneCell.X, throneCell.Y+1)})
	if !res.KingCaptured {
		t.Fatal("4 attackers surrounding the king ON the throne must capture it")
	}
}

func TestKingCaptureOnThroneThreeAttackersInsufficient(t *testing.T) {
	// On the throne, the throne itself never counts toward its own capture
	// (the king's own square isn't one of its neighbors) — so exactly 3
	// attackers must NOT be enough here, unlike the throne-adjacent case.
	b := emptyBoard()
	b.set(throneCell.X, throneCell.Y, King)
	b.set(throneCell.X-1, throneCell.Y, Attacker)
	b.set(throneCell.X, throneCell.Y-1, Attacker)
	b.set(throneCell.X, throneCell.Y+2, Attacker) // mover: slides to throneCell.Y+1; east side still open
	_, res := b.Apply(Move{From: image.Pt(throneCell.X, throneCell.Y+2), To: image.Pt(throneCell.X, throneCell.Y+1)})
	if res.KingCaptured {
		t.Fatal("only 3 of 4 sides covered while the king sits ON the throne must NOT capture it")
	}
}

// --- Only an attacker's move can trigger the king-capture check ------------

func TestDefenderMoveNeverTriggersKingCapture(t *testing.T) {
	// Construct a position where the king is already fully surrounded, then
	// have the DEFENDER side make an unrelated move; the king must not be
	// swept up by that move (no such thing as self-capture by inaction).
	b := emptyBoard()
	b.set(1, 1, King)
	b.set(1, 0, Attacker)
	b.set(1, 2, Attacker)
	b.set(0, 1, Attacker)
	b.set(2, 1, Attacker) // king already fully surrounded
	b.set(5, 5, Defender) // an unrelated defender piece to move
	nb, res := b.Apply(Move{From: image.Pt(5, 5), To: image.Pt(5, 4)})
	if res.KingCaptured {
		t.Fatal("a defender move must never trigger the king-capture check")
	}
	if nb.At(1, 1) != King {
		t.Fatal("the king must remain on the board after a defender-side move")
	}
}
