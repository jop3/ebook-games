package game

// Phase is where the game currently stands.
type Phase int

const (
	PhasePlaying Phase = iota // the round is in progress
	PhaseDone                 // the deck ran out; final scores are settled
)

// HandSize is the starting (and steady-state) hand size for each player:
// every turn removes exactly one card (play or discard) and then draws
// exactly one back, so the hand size never changes mid-round.
const HandSize = 8

// HumanIdx and AIIdx name the two (fixed) player seats — this is a 1 human
// vs 1 AI game throughout, since hidden simultaneous... actually sequential
// hidden hands rule out a meaningful hot-seat mode (either player could just
// look at the screen while it's not their turn).
const (
	HumanIdx = 0
	AIIdx    = 1
)

// Player holds one player's hand (hidden from the opponent) and their 5
// expedition rows (always public — exactly like the real cards on a table).
type Player struct {
	Hand []Card
	Rows [NumSuits][]Card
}

// Score returns this player's current total score, computed live from their
// rows — valid at any time, not just at PhaseDone (an expedition's score is
// a pure function of the cards played in it so far).
func (p *Player) Score() int { return TotalScore(p.Rows) }

// State is the full game state for one Expeditionen round: 1 human (index
// HumanIdx) versus 1 AI (index AIIdx).
type State struct {
	Deck     []Card
	Discards [NumSuits][]Card
	Players  [2]Player

	Turn  int // HumanIdx or AIIdx: whose turn it currently is
	Phase Phase

	// awaitingDraw is true once the current player has played or discarded
	// this turn but has not yet drawn — the turn does not pass to the other
	// player until they do.
	awaitingDraw bool

	// LastDrawnFromDiscard/LastPlayed etc. are intentionally omitted: the UI
	// can always read Players[i].Hand/Rows and Discards directly, so there
	// is no separate "what just happened" channel to keep in sync.

	Shuffle Shuffler // injected deck shuffler; nil = deterministic (no shuffle)
}

// NewGame deals a fresh round: a shuffled 60-card deck, 8 cards to each
// player, human to move first.
func NewGame(shuffle Shuffler) *State {
	s := &State{Shuffle: shuffle}
	deck := NewDeck()
	shuffleDeck(deck, shuffle)
	s.Players[HumanIdx].Hand = append([]Card(nil), deck[:HandSize]...)
	s.Players[AIIdx].Hand = append([]Card(nil), deck[HandSize:2*HandSize]...)
	s.Deck = deck[2*HandSize:]
	s.Turn = HumanIdx
	s.Phase = PhasePlaying
	return s
}

// AwaitingDraw reports whether the player whose turn it is has already
// played/discarded this turn and now owes a draw before the turn passes.
func (s *State) AwaitingDraw() bool { return s.awaitingDraw }

// CurrentPlayer returns whose turn it is (HumanIdx or AIIdx).
func (s *State) CurrentPlayer() int { return s.Turn }

// AITurn reports whether it is currently the AI's turn to act (used by the
// UI to defer the AI's move to the frame after the human's move is drawn —
// same aiPend pattern as hasami/sushi).
func (s *State) AITurn() bool { return s.Phase == PhasePlaying && s.Turn == AIIdx }

// HumanTurn reports whether it is currently the human's turn to act.
func (s *State) HumanTurn() bool { return s.Phase == PhasePlaying && s.Turn == HumanIdx }

// --- errors -----------------------------------------------------------

type gameError string

func (e gameError) Error() string { return string(e) }

const (
	errGameOver      = gameError("expeditionen: round is already over")
	errNotYourTurn   = gameError("expeditionen: not this player's turn")
	errMustDrawFirst = gameError("expeditionen: must draw before acting again")
	errMustPlayFirst = gameError("expeditionen: must play or discard before drawing")
	errBadHandIndex  = gameError("expeditionen: hand index out of range")
	errIllegalPlay   = gameError("expeditionen: illegal play for that expedition")
	errEmptyDeck     = gameError("expeditionen: the draw pile is empty")
	errEmptyDiscard  = gameError("expeditionen: that discard pile is empty")
)

// checkCanAct validates the common preconditions for PlayCard/DiscardCard.
func (s *State) checkCanAct(pi, handIdx int) error {
	if s.Phase != PhasePlaying {
		return errGameOver
	}
	if s.Turn != pi {
		return errNotYourTurn
	}
	if s.awaitingDraw {
		return errMustDrawFirst
	}
	if handIdx < 0 || handIdx >= len(s.Players[pi].Hand) {
		return errBadHandIndex
	}
	return nil
}

// PlayCard plays hand card handIdx onto its own suit's expedition row, if
// legal (see LegalPlay). The turn then owes a draw.
func (s *State) PlayCard(pi, handIdx int) error {
	if err := s.checkCanAct(pi, handIdx); err != nil {
		return err
	}
	p := &s.Players[pi]
	card := p.Hand[handIdx]
	row := p.Rows[card.Suit]
	if !LegalPlay(card, row) {
		return errIllegalPlay
	}
	p.Rows[card.Suit] = append(row, card)
	p.Hand = removeAt(p.Hand, handIdx)
	s.awaitingDraw = true
	return nil
}

// DiscardCard discards hand card handIdx face-up onto its own suit's discard
// pile. Discarding is always legal (no ordering restriction). The turn then
// owes a draw.
func (s *State) DiscardCard(pi, handIdx int) error {
	if err := s.checkCanAct(pi, handIdx); err != nil {
		return err
	}
	p := &s.Players[pi]
	card := p.Hand[handIdx]
	s.Discards[card.Suit] = append(s.Discards[card.Suit], card)
	p.Hand = removeAt(p.Hand, handIdx)
	s.awaitingDraw = true
	return nil
}

// checkCanDraw validates the common preconditions for DrawFromDeck/
// DrawFromDiscard.
func (s *State) checkCanDraw(pi int) error {
	if s.Phase != PhasePlaying {
		return errGameOver
	}
	if s.Turn != pi {
		return errNotYourTurn
	}
	if !s.awaitingDraw {
		return errMustPlayFirst
	}
	return nil
}

// DrawFromDeck draws the top card of the face-down draw pile into pi's hand,
// ending their turn. If this empties the deck, the round ends immediately
// (per the spec: the round runs until the deck is exhausted).
func (s *State) DrawFromDeck(pi int) error {
	if err := s.checkCanDraw(pi); err != nil {
		return err
	}
	if len(s.Deck) == 0 {
		return errEmptyDeck
	}
	last := len(s.Deck) - 1
	card := s.Deck[last]
	s.Deck = s.Deck[:last]
	s.Players[pi].Hand = append(s.Players[pi].Hand, card)
	s.awaitingDraw = false
	if len(s.Deck) == 0 {
		s.Phase = PhaseDone
		return nil
	}
	s.endTurn()
	return nil
}

// DrawFromDiscard draws the top (most recently discarded) card of the given
// suit's discard pile into pi's hand, ending their turn.
func (s *State) DrawFromDiscard(pi int, suit Suit) error {
	if err := s.checkCanDraw(pi); err != nil {
		return err
	}
	pile := s.Discards[suit]
	if len(pile) == 0 {
		return errEmptyDiscard
	}
	last := len(pile) - 1
	card := pile[last]
	s.Discards[suit] = pile[:last]
	s.Players[pi].Hand = append(s.Players[pi].Hand, card)
	s.awaitingDraw = false
	s.endTurn()
	return nil
}

func (s *State) endTurn() {
	s.Turn = 1 - s.Turn
}

// Winner returns the index (or indices, on an exact tie) of the player(s)
// with the highest total score. Meaningful at any time, but only final once
// Phase == PhaseDone.
func (s *State) Winner() []int {
	scores := [2]int{s.Players[0].Score(), s.Players[1].Score()}
	best := scores[0]
	if scores[1] > best {
		best = scores[1]
	}
	var out []int
	for i, sc := range scores {
		if sc == best {
			out = append(out, i)
		}
	}
	return out
}

// removeAt returns hand with the card at index i removed, preserving order.
func removeAt(hand []Card, i int) []Card {
	out := make([]Card, 0, len(hand)-1)
	out = append(out, hand[:i]...)
	out = append(out, hand[i+1:]...)
	return out
}
