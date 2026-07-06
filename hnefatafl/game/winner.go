package game

// Reason records WHY a game ended, purely for status/UI text — it plays no
// part in the rules themselves.
type Reason int

const (
	ReasonNone Reason = iota
	ReasonKingEscaped
	ReasonKingCaptured
	ReasonNoMoves
)

// Winner performs the two board-only win checks that can be read straight
// off the board, with no notion of whose turn it is:
//
//   - the king has been captured (removed from the board) -> attackers win;
//   - the king sits on any corner square -> defenders win (escaped).
//
// It deliberately does NOT check "no legal move" — that check is inherently
// turn-dependent (it asks "does the side about to move have anything to
// play?") and lives in GameState.advance, mirroring how hasami separates its
// analogous stalemate check from its own board-only Winner(). This keeps the
// two sides' win conditions honestly asymmetric rather than forcing them
// through one shared "reduce to N pieces" check, which would not fit this
// game at all (Hasami-style piece-count reduction is not how Hnefatafl ends).
func Winner(b *Board) (Side, Reason, bool) {
	kp, alive := b.KingPos()
	if !alive {
		return SideAttacker, ReasonKingCaptured, true
	}
	if IsCorner(kp.X, kp.Y) {
		return SideDefender, ReasonKingEscaped, true
	}
	return Side(0), ReasonNone, false
}
