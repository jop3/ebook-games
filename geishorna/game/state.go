package game

// Action is one of the four action markers each player spends exactly once
// per round. The order is the player's own choice.
type Action int

const (
	// Secret places 1 card face-down; it is revealed at round end and scored
	// for the acting player.
	Secret Action = iota
	// TradeOff places 2 cards face-down out of the round; they score for no
	// one.
	TradeOff
	// Gift reveals 3 cards; the OPPONENT keeps 1, the acting player keeps the
	// other 2.
	Gift
	// Competition reveals 4 cards split into two pairs; the OPPONENT keeps one
	// pair, the acting player keeps the other.
	Competition

	NumActions = 4
)

// AllActions lists every action in a stable order.
var AllActions = [NumActions]Action{Secret, TradeOff, Gift, Competition}

// Cards is how many hand cards this action consumes.
func (a Action) Cards() int {
	switch a {
	case Secret:
		return 1
	case TradeOff:
		return 2
	case Gift:
		return 3
	case Competition:
		return 4
	}
	return 0
}

// Name is the Swedish label shown on the action buttons.
func (a Action) Name() string {
	switch a {
	case Secret:
		return "Hemlig"
	case TradeOff:
		return "Kasta"
	case Gift:
		return "Gåva"
	case Competition:
		return "Tävling"
	}
	return "?"
}

// Player seats. This is a 1 human vs 1 AI game throughout: the round's hidden
// hands (and the face-down Secret card) rule out a meaningful hot-seat mode.
const (
	HumanIdx = 0
	AIIdx    = 1
)

// HandStart is the hand size each player is dealt at the start of a round;
// each of their four turns then draws 1 more before acting.
const HandStart = 6

// Win thresholds, checked at the end of every round on a player's accumulated
// victory markers.
const (
	GeishasToWin = 4  // favor of 4+ geishas wins the match
	CharmToWin   = 11 // favor of geishas totaling 11+ charm wins the match
)

// Board is one player's per-round state plus the (per-round) face-up field
// counts that decide each geisha's favor.
type Board struct {
	Hand      []Card           // hidden from the opponent
	Field     [NumGeishas]int  // face-up item counts placed toward each geisha (public)
	Secret    Card             // the 1 face-down Secret card (valid iff HasSecret)
	HasSecret bool             // whether a Secret card is stashed
	Used      [NumActions]bool // which of the 4 action markers are spent
}

func (b *Board) usedCount() int {
	n := 0
	for _, u := range b.Used {
		if u {
			n++
		}
	}
	return n
}

// Pending is an offer that is waiting on the opponent's choice: a Gift (pick 1
// of 3 cards) or a Competition (pick 1 of 2 pairs). While it is set, the game
// is waiting on Chooser, not on whoever's action turn it nominally is.
type Pending struct {
	Action  Action    // Gift or Competition
	Actor   int       // who made the offer
	Chooser int       // opponent who must pick (== 1-Actor)
	Cards   []Card    // Gift: the 3 offered cards
	Groups  [2][]Card // Competition: the two pairs
}

// GeishaResult is one geisha's outcome for a finished round.
type GeishaResult struct {
	HumanCount int
	AICount    int
	Winner     int // HumanIdx, AIIdx, or -1 on a tie (favor marker did not move)
}

// RoundResult captures a finished round for the summary screen.
type RoundResult struct {
	Geisha         [NumGeishas]GeishaResult
	HumanSecret    Card
	AISecret       Card
	HasHumanSecret bool
	HasAISecret    bool
}

// Phase is where the match currently stands.
type Phase int

const (
	PhaseAction   Phase = iota // a round is in progress (someone to act or choose)
	PhaseRoundEnd              // a round just scored; showing the summary
	PhaseMatchEnd              // the match is decided
)

// State is the full match state: an ongoing best-of series of Hanamikoji
// rounds between 1 human (HumanIdx) and 1 AI (AIIdx). Victory markers (Favor)
// persist across rounds; everything else in Board is per-round.
type State struct {
	Deck    []Card
	Players [2]Board

	// Favor is each geisha's victory-marker holder: HumanIdx, AIIdx, or -1 for
	// the neutral center. Persists between rounds (markers do NOT reset).
	Favor [NumGeishas]int

	Turn    int      // whose action turn it is
	Pending *Pending // a pending opponent-choice, or nil
	Phase   Phase
	Round   int // 1-based round counter

	Result      *RoundResult // the most recently finished round (for PhaseRoundEnd)
	MatchWinner int          // HumanIdx / AIIdx once decided, else -1

	nextStarter int // who leads the next round (players alternate)
	Shuffle     Shuffler
}

// NewGame starts a fresh match: all favor markers neutral, human leads round 1.
func NewGame(shuffle Shuffler) *State {
	s := &State{Shuffle: shuffle, MatchWinner: -1, nextStarter: HumanIdx}
	for i := range s.Favor {
		s.Favor[i] = -1
	}
	s.beginRound()
	return s
}

// beginRound deals a new round: reshuffle all 21 cards, remove 1 face-down and
// unseen, deal 6 to each player, and set the round's leader (who then draws).
func (s *State) beginRound() {
	deck := NewDeck()
	shuffleDeck(deck, s.Shuffle)

	s.Players[HumanIdx] = Board{}
	s.Players[AIIdx] = Board{}

	rest := deck[1:] // deck[0] is removed from the round, unseen
	s.Players[HumanIdx].Hand = append([]Card(nil), rest[0:HandStart]...)
	s.Players[AIIdx].Hand = append([]Card(nil), rest[HandStart:2*HandStart]...)
	s.Deck = append([]Card(nil), rest[2*HandStart:]...) // 8 cards, one per turn

	s.Pending = nil
	s.Result = nil
	s.Phase = PhaseAction
	s.Round++
	s.Turn = s.nextStarter
	s.nextStarter = 1 - s.nextStarter
	s.startTurn()
}

// startTurn draws the mandatory one card for the player about to act.
func (s *State) startTurn() {
	if len(s.Deck) == 0 {
		return
	}
	last := len(s.Deck) - 1
	c := s.Deck[last]
	s.Deck = s.Deck[:last]
	s.Players[s.Turn].Hand = append(s.Players[s.Turn].Hand, c)
}

// --- queries ---------------------------------------------------------------

// ToAct returns the player the game is currently waiting on: the Pending
// chooser if an offer is open, otherwise the player whose action turn it is.
func (s *State) ToAct() int {
	if s.Pending != nil {
		return s.Pending.Chooser
	}
	return s.Turn
}

// AwaitingChoice reports whether an opponent must pick from an open offer.
func (s *State) AwaitingChoice() bool { return s.Pending != nil }

// HumanTurn reports whether the game is waiting on the human (to act or pick).
func (s *State) HumanTurn() bool { return s.Phase == PhaseAction && s.ToAct() == HumanIdx }

// AITurn reports whether the game is waiting on the AI (to act or pick).
func (s *State) AITurn() bool { return s.Phase == PhaseAction && s.ToAct() == AIIdx }

// Available reports whether player p may still spend action a this turn — it
// must be unused and there must be enough cards in hand to perform it.
func (s *State) Available(p int, a Action) bool {
	return !s.Players[p].Used[a] && len(s.Players[p].Hand) >= a.Cards()
}

// Standing returns player p's accumulated favor: how many geishas they hold
// and the total charm of those geishas.
func (s *State) Standing(p int) (geishas, charm int) {
	for i := 0; i < NumGeishas; i++ {
		if s.Favor[i] == p {
			geishas++
			charm += Geishas[i].Charm
		}
	}
	return geishas, charm
}

// --- errors ----------------------------------------------------------------

type gameError string

func (e gameError) Error() string { return string(e) }

const (
	errPhase      = gameError("geishorna: not in the action phase")
	errPending    = gameError("geishorna: an offer is awaiting a choice")
	errNoPending  = gameError("geishorna: no matching offer is open")
	errNotTurn    = gameError("geishorna: not this player's turn")
	errNotChooser = gameError("geishorna: not this player's choice to make")
	errActionUsed = gameError("geishorna: that action marker is already spent")
	errBadIndex   = gameError("geishorna: card selection out of range or malformed")
	errNotEnough  = gameError("geishorna: not enough cards for that action")
)

// checkAct validates the common preconditions for a Do* action by player p.
func (s *State) checkAct(p int, a Action) error {
	if s.Phase != PhaseAction {
		return errPhase
	}
	if s.Pending != nil {
		return errPending
	}
	if s.Turn != p {
		return errNotTurn
	}
	if s.Players[p].Used[a] {
		return errActionUsed
	}
	if len(s.Players[p].Hand) < a.Cards() {
		return errNotEnough
	}
	return nil
}

// --- actions ---------------------------------------------------------------

// DoSecret spends the Secret marker on hand card idx, stashing it face-down.
func (s *State) DoSecret(p, idx int) error {
	if err := s.checkAct(p, Secret); err != nil {
		return err
	}
	b := &s.Players[p]
	if idx < 0 || idx >= len(b.Hand) {
		return errBadIndex
	}
	b.Secret = b.Hand[idx]
	b.HasSecret = true
	b.Hand = removeAt(b.Hand, idx)
	b.Used[Secret] = true
	s.afterAction()
	return nil
}

// DoTradeOff spends the Trade-off marker on the two given hand cards, removing
// them from the round entirely.
func (s *State) DoTradeOff(p int, idxs []int) error {
	if err := s.checkAct(p, TradeOff); err != nil {
		return err
	}
	if err := validSelection(s.Players[p].Hand, idxs, 2); err != nil {
		return err
	}
	takeCards(&s.Players[p], idxs)
	s.Players[p].Used[TradeOff] = true
	s.afterAction()
	return nil
}

// DoGift spends the Gift marker: the three given cards become an offer the
// opponent then picks 1 of (ResolveGift). The turn does not pass until the
// choice is resolved.
func (s *State) DoGift(p int, idxs []int) error {
	if err := s.checkAct(p, Gift); err != nil {
		return err
	}
	if err := validSelection(s.Players[p].Hand, idxs, 3); err != nil {
		return err
	}
	cards := takeCards(&s.Players[p], idxs)
	s.Players[p].Used[Gift] = true
	s.Pending = &Pending{Action: Gift, Actor: p, Chooser: 1 - p, Cards: cards}
	return nil
}

// DoCompetition spends the Competition marker: the four cards (two pairs,
// groupA and groupB, each two distinct hand indices) become an offer the
// opponent then picks one pair of (ResolveCompetition).
func (s *State) DoCompetition(p int, groupA, groupB []int) error {
	if err := s.checkAct(p, Competition); err != nil {
		return err
	}
	if len(groupA) != 2 || len(groupB) != 2 {
		return errBadIndex
	}
	all := append(append([]int{}, groupA...), groupB...)
	if err := validSelection(s.Players[p].Hand, all, 4); err != nil {
		return err
	}
	hand := s.Players[p].Hand
	ga := []Card{hand[groupA[0]], hand[groupA[1]]}
	gb := []Card{hand[groupB[0]], hand[groupB[1]]}
	takeCards(&s.Players[p], all)
	s.Players[p].Used[Competition] = true
	s.Pending = &Pending{Action: Competition, Actor: p, Chooser: 1 - p, Groups: [2][]Card{ga, gb}}
	return nil
}

// ResolveGift settles an open Gift: chooser keeps the pick'th offered card;
// the other two go to the actor. Both sets are placed face-up.
func (s *State) ResolveGift(chooser, pick int) error {
	if s.Pending == nil || s.Pending.Action != Gift {
		return errNoPending
	}
	if s.Pending.Chooser != chooser {
		return errNotChooser
	}
	if pick < 0 || pick >= len(s.Pending.Cards) {
		return errBadIndex
	}
	actor := s.Pending.Actor
	for i, c := range s.Pending.Cards {
		if i == pick {
			s.Players[chooser].Field[c.Geisha]++
		} else {
			s.Players[actor].Field[c.Geisha]++
		}
	}
	s.Pending = nil
	s.afterAction()
	return nil
}

// ResolveCompetition settles an open Competition: chooser keeps the pick'th
// pair; the other pair goes to the actor.
func (s *State) ResolveCompetition(chooser, pick int) error {
	if s.Pending == nil || s.Pending.Action != Competition {
		return errNoPending
	}
	if s.Pending.Chooser != chooser {
		return errNotChooser
	}
	if pick < 0 || pick > 1 {
		return errBadIndex
	}
	actor := s.Pending.Actor
	for gi := 0; gi < 2; gi++ {
		dst := actor
		if gi == pick {
			dst = chooser
		}
		for _, c := range s.Pending.Groups[gi] {
			s.Players[dst].Field[c.Geisha]++
		}
	}
	s.Pending = nil
	s.afterAction()
	return nil
}

// afterAction runs once an action fully resolves: end the round if both
// players have spent all four markers, else pass the turn (with its draw).
func (s *State) afterAction() {
	if s.roundComplete() {
		s.scoreRound()
		return
	}
	s.Turn = 1 - s.Turn
	s.startTurn()
}

func (s *State) roundComplete() bool {
	return s.Players[HumanIdx].usedCount() == NumActions &&
		s.Players[AIIdx].usedCount() == NumActions
}

// scoreRound reveals both Secret cards, resolves every geisha's favor, records
// the result, and checks for a match win.
func (s *State) scoreRound() {
	res := &RoundResult{
		HasHumanSecret: s.Players[HumanIdx].HasSecret,
		HumanSecret:    s.Players[HumanIdx].Secret,
		HasAISecret:    s.Players[AIIdx].HasSecret,
		AISecret:       s.Players[AIIdx].Secret,
	}
	for p := 0; p < 2; p++ {
		if s.Players[p].HasSecret {
			s.Players[p].Field[s.Players[p].Secret.Geisha]++
		}
	}
	for i := 0; i < NumGeishas; i++ {
		h := s.Players[HumanIdx].Field[i]
		a := s.Players[AIIdx].Field[i]
		gr := GeishaResult{HumanCount: h, AICount: a, Winner: -1}
		switch {
		case h > a:
			s.Favor[i] = HumanIdx
			gr.Winner = HumanIdx
		case a > h:
			s.Favor[i] = AIIdx
			gr.Winner = AIIdx
			// tie: marker does not move; favor stays wherever it already is
		}
		res.Geisha[i] = gr
	}
	s.Result = res
	s.MatchWinner = s.matchWinner()
	s.Phase = PhaseRoundEnd
}

// matchWinner returns the winning player if the current favor standings decide
// the match, else -1. A charm total of 11+ wins; failing that, favor of 4+
// geishas wins. Charm dominates a simultaneous trigger (and only one player
// can ever reach 11 charm, since the two totals sum to at most 21).
func (s *State) matchWinner() int {
	hg, hc := s.Standing(HumanIdx)
	ag, ac := s.Standing(AIIdx)
	switch {
	case hc >= CharmToWin:
		return HumanIdx
	case ac >= CharmToWin:
		return AIIdx
	case hg >= GeishasToWin:
		return HumanIdx
	case ag >= GeishasToWin:
		return AIIdx
	}
	return -1
}

// Continue advances past a round-end summary: to the match-end screen if the
// match is decided, otherwise into the next round.
func (s *State) Continue() {
	if s.Phase != PhaseRoundEnd {
		return
	}
	if s.MatchWinner >= 0 {
		s.Phase = PhaseMatchEnd
		return
	}
	s.beginRound()
}

// --- helpers ---------------------------------------------------------------

func removeAt(hand []Card, i int) []Card {
	out := make([]Card, 0, len(hand)-1)
	out = append(out, hand[:i]...)
	out = append(out, hand[i+1:]...)
	return out
}

// takeCards removes the cards at the given (validated) hand indices and
// returns them in hand order.
func takeCards(b *Board, idxs []int) []Card {
	drop := make(map[int]bool, len(idxs))
	for _, i := range idxs {
		drop[i] = true
	}
	var removed, keep []Card
	for i, c := range b.Hand {
		if drop[i] {
			removed = append(removed, c)
		} else {
			keep = append(keep, c)
		}
	}
	b.Hand = keep
	return removed
}

// validSelection checks that idxs holds exactly want distinct indices, all in
// range for hand.
func validSelection(hand []Card, idxs []int, want int) error {
	if len(idxs) != want {
		return errBadIndex
	}
	seen := make(map[int]bool, want)
	for _, i := range idxs {
		if i < 0 || i >= len(hand) || seen[i] {
			return errBadIndex
		}
		seen[i] = true
	}
	return nil
}
