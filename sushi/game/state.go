package game

import "sort"

// Phase is where the game currently stands.
type Phase int

const (
	PhasePlaying  Phase = iota // hands are out, waiting on this turn's picks
	PhaseRoundEnd              // round scored; waiting to advance (or end game)
	PhaseGameEnd               // 3 rounds + Pudding scored; game over
)

const NumRounds = 3

// HandSize returns the starting hand size for a given player count, per the
// spec's table (deliberately NOT the official Sushi Go! sizes — this repo's
// spec calls out 5p/4p->7, 3p->8, 2p->10 explicitly, so that table is used
// as given rather than the standard 7/8/9/10 progression).
func HandSize(numPlayers int) int {
	switch numPlayers {
	case 2:
		return 10
	case 3:
		return 8
	case 4, 5:
		return 7
	default:
		return 7
	}
}

// Player holds one player's hand, this round's played tableau, and the
// persistent (cross-round) Pudding count and running Score.
type Player struct {
	Hand    []Card
	Tableau []Card
	Pudding int // persists across rounds; never reset
	Score   int // sum of each round's Total(), plus Pudding at game end
}

// State is the full game state for one Sushi match: 1 human (index 0) plus
// (NumPlayers-1) AI seats.
type State struct {
	NumPlayers int
	Round      int // 1..3
	Direction  int // this round's pass direction: +1 or -1
	Phase      Phase
	Players    []Player

	Shuffle Shuffler // injected deck shuffler; nil = deterministic (no shuffle)

	LastRoundScores []RoundScore // set on entering PhaseRoundEnd/PhaseGameEnd
	LastPudding     []int        // set on entering PhaseGameEnd (ScorePudding output)
}

// NewGame starts a fresh match for numPlayers (2..5), dealing round 1.
func NewGame(numPlayers int, shuffle Shuffler) *State {
	if numPlayers < 2 {
		numPlayers = 2
	}
	if numPlayers > 5 {
		numPlayers = 5
	}
	s := &State{
		NumPlayers: numPlayers,
		Round:      0,
		Phase:      PhasePlaying,
		Players:    make([]Player, numPlayers),
		Shuffle:    shuffle,
	}
	s.dealRound(1)
	return s
}

// dealRound resets every player's Tableau (Pudding and Score are untouched —
// they persist across rounds) and deals a fresh hand from a freshly shuffled
// deck, exactly as the physical game reshuffles all 108 cards each round.
func (s *State) dealRound(round int) {
	s.Round = round
	if round%2 == 1 {
		s.Direction = 1
	} else {
		s.Direction = -1
	}
	hs := HandSize(s.NumPlayers)
	deck := NewDeck()
	shuffleDeck(deck, s.Shuffle)
	pos := 0
	for i := range s.Players {
		s.Players[i].Tableau = nil
		s.Players[i].Hand = append([]Card(nil), deck[pos:pos+hs]...)
		pos += hs
	}
	s.Phase = PhasePlaying
}

// HandLen returns the current hand size (same for every player, since a turn
// always removes exactly one net card from everyone's hand — see ApplyTurn).
func (s *State) HandLen() int {
	if len(s.Players) == 0 {
		return 0
	}
	return len(s.Players[0].Hand)
}

// Pick is one player's choice for the current turn: 1 index (a normal
// take), or 2 indices (a Chopsticks swap-back — only legal if that player
// currently has an unplayed Chopsticks card sitting in their Tableau).
// Indices refer to that player's Hand as it stands BEFORE this turn's
// resolution and must be given in ascending order with no duplicates.
type Pick struct {
	Idx []int
}

func (p Pick) valid(handLen int, hasChopsticks bool) bool {
	switch len(p.Idx) {
	case 1:
		return p.Idx[0] >= 0 && p.Idx[0] < handLen
	case 2:
		if !hasChopsticks {
			return false
		}
		if p.Idx[0] == p.Idx[1] {
			return false
		}
		if p.Idx[0] < 0 || p.Idx[1] < 0 || p.Idx[0] >= handLen || p.Idx[1] >= handLen {
			return false
		}
		return p.Idx[0] < p.Idx[1]
	default:
		return false
	}
}

// ApplyTurn resolves one drafting turn for every player AT ONCE: every pick
// is validated and the drafted cards extracted against each player's
// PRE-TURN hand (a single snapshot), and only once every pick has been
// resolved do hands get passed on. No player's pick can see or react to
// another player's same-turn pick — that would require the game to leak a
// hidden hand, which never happens here (each player's Pick is checked only
// against their own Hand slice, and passing/rotation happens strictly after
// all of them are resolved).
func (s *State) ApplyTurn(picks []Pick) error {
	n := s.NumPlayers
	if len(picks) != n {
		return errPickCount
	}
	leftover := make([][]Card, n)
	drafted := make([][]Card, n)
	for i := 0; i < n; i++ {
		p := &s.Players[i]
		hasChop := countKind(p.Tableau, KindChopsticks) > 0
		if !picks[i].valid(len(p.Hand), hasChop) {
			return errBadPick
		}
		kept, taken := removeIndices(p.Hand, picks[i].Idx)
		if len(picks[i].Idx) == 2 {
			// Chopsticks swap-back: pull ONE Chopsticks card out of the
			// tableau and back into the hand that gets passed on. (It can be
			// played again later for another swap-back.)
			ci := firstIndexOfKind(p.Tableau, KindChopsticks)
			returned := p.Tableau[ci]
			newTableau := make([]Card, 0, len(p.Tableau)-1)
			newTableau = append(newTableau, p.Tableau[:ci]...)
			newTableau = append(newTableau, p.Tableau[ci+1:]...)
			p.Tableau = newTableau
			kept = append(kept, returned)
		}
		leftover[i] = kept
		drafted[i] = taken
	}
	for i := 0; i < n; i++ {
		s.Players[i].Tableau = append(s.Players[i].Tableau, drafted[i]...)
	}
	for i := 0; i < n; i++ {
		dst := ((i+s.Direction)%n + n) % n
		s.Players[dst].Hand = leftover[i]
	}
	if s.HandLen() == 0 {
		s.finishRound()
	}
	return nil
}

func (s *State) finishRound() {
	tableaus := make([][]Card, s.NumPlayers)
	for i := range s.Players {
		tableaus[i] = s.Players[i].Tableau
		s.Players[i].Pudding += countKind(s.Players[i].Tableau, KindPudding)
	}
	scores := ScoreRound(tableaus)
	for i := range s.Players {
		s.Players[i].Score += scores[i].Total()
	}
	s.LastRoundScores = scores
	if s.Round >= NumRounds {
		puddingCounts := make([]int, s.NumPlayers)
		for i := range s.Players {
			puddingCounts[i] = s.Players[i].Pudding
		}
		adj := ScorePudding(puddingCounts)
		for i := range s.Players {
			s.Players[i].Score += adj[i]
		}
		s.LastPudding = adj
		s.Phase = PhaseGameEnd
	} else {
		s.Phase = PhaseRoundEnd
	}
}

// PlayTurn resolves the current turn given the human's (player 0) pick: it
// computes every AI seat's pick from the SAME pre-turn snapshot (none of the
// AI picks can see the human's pick, or each other's — see AIPick) and then
// applies all of them at once via ApplyTurn.
func (s *State) PlayTurn(human Pick) error {
	picks := make([]Pick, s.NumPlayers)
	picks[0] = human
	for i := 1; i < s.NumPlayers; i++ {
		picks[i] = s.AIPick(i)
	}
	return s.ApplyTurn(picks)
}

// AdvanceRound deals the next round after a PhaseRoundEnd screen has been
// shown. It is a no-op once the game has reached PhaseGameEnd.
func (s *State) AdvanceRound() {
	if s.Phase != PhaseRoundEnd {
		return
	}
	s.dealRound(s.Round + 1)
}

// Winner returns the index (or indices, on an exact tie) of the player(s)
// with the highest final Score. Only meaningful once Phase==PhaseGameEnd.
func (s *State) Winner() []int {
	best := s.Players[0].Score
	for _, p := range s.Players {
		if p.Score > best {
			best = p.Score
		}
	}
	var out []int
	for i, p := range s.Players {
		if p.Score == best {
			out = append(out, i)
		}
	}
	return out
}

// --- small helpers -----------------------------------------------------

type gameError string

func (e gameError) Error() string { return string(e) }

const (
	errPickCount = gameError("sushi: wrong number of picks for ApplyTurn")
	errBadPick   = gameError("sushi: invalid pick (bad index, duplicate, or chopsticks unavailable)")
)

// removeIndices splits hand into the cards NOT named by idx ("kept", in
// original order) and the cards named by idx ("taken", also in original
// hand order — i.e. ascending idx order, not the order idx was given in).
func removeIndices(hand []Card, idx []int) (kept, taken []Card) {
	remove := make(map[int]bool, len(idx))
	for _, i := range idx {
		remove[i] = true
	}
	kept = make([]Card, 0, len(hand)-len(idx))
	taken = make([]Card, 0, len(idx))
	for i, c := range hand {
		if remove[i] {
			taken = append(taken, c)
		} else {
			kept = append(kept, c)
		}
	}
	return kept, taken
}

func firstIndexOfKind(cards []Card, k CardKind) int {
	for i, c := range cards {
		if c.Kind == k {
			return i
		}
	}
	return -1
}

// sortInts is a tiny helper kept local so the AI/UI can build ascending Pick
// indices without importing sort at every call site.
func sortInts(idx []int) { sort.Ints(idx) }
