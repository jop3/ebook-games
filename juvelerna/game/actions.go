package game

// actions.go implements the 4 turn actions (take-3, take-2, reserve, buy) as
// independent, legality-checked functions, plus the shared end-of-turn
// resolution pipeline (noble auto-claim / active-player choice, forced
// token discard, the 15-prestige end trigger with "finish the round" turn
// parity, and the tiebroken FinalWinner).

// --- Take 3 tokens of 3 different colors ------------------------------------

// CanTake3 reports whether taking one token each of 3 distinct colors is
// currently legal: the colors must be pairwise distinct, and each color's
// bank pile must have at least 1 token.
func (gs *GameState) CanTake3(colors [3]Color) bool {
	return gs.CanTakeColors(colors[:])
}

// Take3 applies the take-3 action. Returns false (no effect) if illegal.
func (gs *GameState) Take3(colors [3]Color) bool {
	return gs.TakeColors(colors[:])
}

// takeableColors returns how many DIFFERENT colors a take action must cover:
// 3 normally, fewer when the bank has run dry — the official-rules fallback
// that a player takes as many different tokens as the bank can supply.
func (gs *GameState) takeableColors() int {
	n := 0
	for c := Color(0); c < NumColors; c++ {
		if gs.Bank[c] > 0 {
			n++
		}
	}
	if n > 3 {
		n = 3
	}
	return n
}

// CanTakeColors reports whether taking one token each of the given distinct
// colors is currently legal. Normally that is exactly 3 different colors;
// when fewer than 3 bank piles are non-empty the official fallback applies:
// the player takes one of EVERY remaining color (2, or 1) instead — taking
// fewer than the bank can supply is never legal.
func (gs *GameState) CanTakeColors(colors []Color) bool {
	if gs.Phase != PhasePlaying {
		return false
	}
	if len(colors) < 1 || len(colors) > 3 || len(colors) != gs.takeableColors() {
		return false
	}
	for i, c := range colors {
		if c < 0 || c >= NumColors || gs.Bank[c] < 1 {
			return false
		}
		for j := 0; j < i; j++ {
			if colors[j] == c {
				return false
			}
		}
	}
	return true
}

// TakeColors applies a take of one token per given color (see CanTakeColors).
// Returns false (no effect) if illegal.
func (gs *GameState) TakeColors(colors []Color) bool {
	if !gs.CanTakeColors(colors) {
		return false
	}
	p := &gs.Players[gs.Turn]
	for _, c := range colors {
		gs.Bank[c]--
		p.Tokens[c]++
	}
	gs.afterAction()
	return true
}

// --- Pass (last resort) -------------------------------------------------------

// CanPass reports whether passing the turn is legal: only when the player has
// NO other action at all (empty bank, nothing reservable, nothing affordable)
// — the official-rules escape hatch that keeps a pathological position from
// deadlocking the game.
func (gs *GameState) CanPass() bool {
	return gs.Phase == PhasePlaying && len(gs.legalActionsNoPass()) == 0
}

// Pass ends the turn without acting. Returns false (no effect) if any real
// action is available.
func (gs *GameState) Pass() bool {
	if !gs.CanPass() {
		return false
	}
	gs.afterAction()
	return true
}

// --- Take 2 tokens of the same color ----------------------------------------

// CanTake2 reports whether taking 2 tokens of color c is currently legal:
// the GOTCHA rule is that the bank pile must have had at least 4 tokens
// BEFORE taking (not merely >=2) — this is checked against the current
// (pre-take) bank count.
func (gs *GameState) CanTake2(c Color) bool {
	if gs.Phase != PhasePlaying {
		return false
	}
	if c < 0 || c >= NumColors {
		return false
	}
	return gs.Bank[c] >= 4
}

// Take2 applies the take-2 action. Returns false (no effect) if illegal.
func (gs *GameState) Take2(c Color) bool {
	if !gs.CanTake2(c) {
		return false
	}
	gs.Bank[c] -= 2
	gs.Players[gs.Turn].Tokens[c] += 2
	gs.afterAction()
	return true
}

// --- Reserve a card (face-up from the tableau, or blind from a deck) -------

// CanReserveTableau reports whether reserving the face-up card at
// Tableau[tier][slot] is currently legal: the slot must hold a card, and the
// active player must hold fewer than MaxReserved reserved cards already.
func (gs *GameState) CanReserveTableau(tier, slot int) bool {
	if gs.Phase != PhasePlaying {
		return false
	}
	if tier < 0 || tier >= NumTiers || slot < 0 || slot >= TableauSlots {
		return false
	}
	if gs.Tableau[tier][slot].Tier == 0 {
		return false
	}
	return len(gs.Players[gs.Turn].Reserved) < MaxReserved
}

// ReserveTableau reserves the face-up card at Tableau[tier][slot]: it moves
// to the active player's reserved list, the tableau slot is refilled from
// that tier's deck, and the player gains 1 gold if the bank has any left.
func (gs *GameState) ReserveTableau(tier, slot int) bool {
	if !gs.CanReserveTableau(tier, slot) {
		return false
	}
	p := &gs.Players[gs.Turn]
	card := gs.Tableau[tier][slot]
	p.Reserved = append(p.Reserved, card)
	gs.Tableau[tier][slot] = gs.drawCard(tier)
	gs.grantReserveGold(p)
	gs.afterAction()
	return true
}

// CanReserveBlind reports whether reserving the top (unseen) card of tier
// t's deck is currently legal: the deck must be non-empty, and the active
// player must hold fewer than MaxReserved reserved cards already.
func (gs *GameState) CanReserveBlind(t int) bool {
	if gs.Phase != PhasePlaying {
		return false
	}
	if t < 0 || t >= NumTiers {
		return false
	}
	if len(gs.Decks[t]) == 0 {
		return false
	}
	return len(gs.Players[gs.Turn].Reserved) < MaxReserved
}

// ReserveBlind reserves the top card of tier t's deck directly (no tableau
// slot is vacated) and grants 1 gold if any remains in the bank.
//
// Design note: in the physical game, a blind reservation stays hidden from
// the OTHER player until played or revealed — the one piece of hidden
// information in Splendor. This app deliberately shows all reserved cards to
// both players (see the package doc and the UI's "Visa motståndare" full
// reveal): the spec frames this game as perfect-information throughout (no
// hidden hands, everything visible) to keep the AI a straightforward
// heuristic, and hot-seat play shares one physical screen anyway, where true
// hidden information isn't practical.
func (gs *GameState) ReserveBlind(t int) bool {
	if !gs.CanReserveBlind(t) {
		return false
	}
	p := &gs.Players[gs.Turn]
	card := gs.drawCard(t)
	p.Reserved = append(p.Reserved, card)
	gs.grantReserveGold(p)
	gs.afterAction()
	return true
}

func (gs *GameState) grantReserveGold(p *PlayerState) {
	if gs.BankGold > 0 {
		gs.BankGold--
		p.Gold++
	}
}

// --- Buy a card (from the tableau, or from your own reserve) ---------------

// canAfford reports whether p can pay cost using their tokens and permanent
// card-bonus discounts, with gold substituting for any shortfall.
func canAfford(p *PlayerState, cost [NumColors]int) bool {
	goldNeeded := 0
	for c := Color(0); c < NumColors; c++ {
		need := cost[c] - p.Bonuses[c]
		if need < 0 {
			need = 0
		}
		if need > p.Tokens[c] {
			goldNeeded += need - p.Tokens[c]
		}
	}
	return goldNeeded <= p.Gold
}

// pay deducts cost from p's tokens (using gold to cover any per-color
// shortfall after the card-bonus discount) and returns the spent tokens to
// the bank. Assumes canAfford(p, cost) was already checked true.
func pay(gs *GameState, p *PlayerState, cost [NumColors]int) {
	for c := Color(0); c < NumColors; c++ {
		need := cost[c] - p.Bonuses[c]
		if need < 0 {
			need = 0
		}
		useTokens := need
		if useTokens > p.Tokens[c] {
			useTokens = p.Tokens[c]
		}
		p.Tokens[c] -= useTokens
		gs.Bank[c] += useTokens
		shortfall := need - useTokens
		if shortfall > 0 {
			p.Gold -= shortfall
			gs.BankGold += shortfall
		}
	}
}

func (gs *GameState) acquireCard(p *PlayerState, card Card) {
	p.Cards = append(p.Cards, card)
	p.Bonuses[card.Color]++
	p.Prestige += card.Points
}

// CanBuyTableau reports whether the active player can afford the face-up
// card at Tableau[tier][slot].
func (gs *GameState) CanBuyTableau(tier, slot int) bool {
	if gs.Phase != PhasePlaying {
		return false
	}
	if tier < 0 || tier >= NumTiers || slot < 0 || slot >= TableauSlots {
		return false
	}
	card := gs.Tableau[tier][slot]
	if card.Tier == 0 {
		return false
	}
	return canAfford(&gs.Players[gs.Turn], card.Cost)
}

// BuyTableau buys the face-up card at Tableau[tier][slot]: pays its cost,
// adds it to the active player's owned cards (granting its bonus + points),
// and refills the tableau slot from that tier's deck.
func (gs *GameState) BuyTableau(tier, slot int) bool {
	if !gs.CanBuyTableau(tier, slot) {
		return false
	}
	p := &gs.Players[gs.Turn]
	card := gs.Tableau[tier][slot]
	pay(gs, p, card.Cost)
	gs.Tableau[tier][slot] = gs.drawCard(tier)
	gs.acquireCard(p, card)
	gs.afterAction()
	return true
}

// CanBuyReserved reports whether the active player can afford their own
// reserved card at index idx.
func (gs *GameState) CanBuyReserved(idx int) bool {
	if gs.Phase != PhasePlaying {
		return false
	}
	p := &gs.Players[gs.Turn]
	if idx < 0 || idx >= len(p.Reserved) {
		return false
	}
	return canAfford(p, p.Reserved[idx].Cost)
}

// BuyReserved buys the active player's own reserved card at index idx.
func (gs *GameState) BuyReserved(idx int) bool {
	if !gs.CanBuyReserved(idx) {
		return false
	}
	p := &gs.Players[gs.Turn]
	card := p.Reserved[idx]
	pay(gs, p, card.Cost)
	p.Reserved = append(p.Reserved[:idx], p.Reserved[idx+1:]...)
	gs.acquireCard(p, card)
	gs.afterAction()
	return true
}

// --- End-of-turn resolution pipeline: nobles -> discard -> turn/game end ---

// NobleQualifies reports whether a player with the given card-color bonuses
// meets a noble's requirement (every listed color's bonus count must be at
// least the requirement).
func NobleQualifies(bonuses [NumColors]int, req [NumColors]int) bool {
	for c := Color(0); c < NumColors; c++ {
		if bonuses[c] < req[c] {
			return false
		}
	}
	return true
}

// QualifyingNobles returns the indices (into nobles) of every noble whose
// requirement the given bonuses satisfy.
func QualifyingNobles(bonuses [NumColors]int, nobles []Noble) []int {
	var out []int
	for i, n := range nobles {
		if NobleQualifies(bonuses, n.Requirement) {
			out = append(out, i)
		}
	}
	return out
}

// afterAction runs the shared end-of-action pipeline: nobles, then the
// token-cap discard, then (once both are clear) finishTurn.
func (gs *GameState) afterAction() {
	gs.resolveNobles()
}

// resolveNobles checks the active player's noble eligibility right now
// (only card purchases change card-bonuses, so this is a no-op after a
// take/reserve). Zero qualifying nobles moves straight to the discard step;
// exactly one is auto-claimed; two or more require the ACTIVE player's
// choice (PhaseNobleChoice), per the spec's gotcha.
func (gs *GameState) resolveNobles() {
	p := &gs.Players[gs.Turn]
	cands := QualifyingNobles(p.Bonuses, gs.Nobles)
	switch len(cands) {
	case 0:
		gs.resolveDiscard()
	case 1:
		gs.claimNobleAt(cands[0])
		gs.resolveDiscard()
	default:
		gs.Phase = PhaseNobleChoice
		gs.PendingNobles = cands
	}
}

// claimNobleAt removes gs.Nobles[i], awards it to the active player, and adds
// NoblePoints prestige.
func (gs *GameState) claimNobleAt(i int) {
	p := &gs.Players[gs.Turn]
	noble := gs.Nobles[i]
	gs.Nobles = append(gs.Nobles[:i:i], gs.Nobles[i+1:]...)
	p.Nobles = append(p.Nobles, noble)
	p.Prestige += NoblePoints
}

// ChooseNoble resolves a pending PhaseNobleChoice: choiceIdx indexes into
// gs.PendingNobles (not directly into gs.Nobles). Returns false if there is
// no pending choice or the index is out of range.
func (gs *GameState) ChooseNoble(choiceIdx int) bool {
	if gs.Phase != PhaseNobleChoice {
		return false
	}
	if choiceIdx < 0 || choiceIdx >= len(gs.PendingNobles) {
		return false
	}
	nobleIdx := gs.PendingNobles[choiceIdx]
	gs.Phase = PhasePlaying
	gs.PendingNobles = nil
	gs.claimNobleAt(nobleIdx)
	gs.resolveDiscard()
	return true
}

// resolveDiscard moves to PhaseDiscard if the active player holds more than
// TokenCap tokens, otherwise finishes the turn directly.
func (gs *GameState) resolveDiscard() {
	p := &gs.Players[gs.Turn]
	if total := p.TokensTotal(); total > TokenCap {
		gs.Phase = PhaseDiscard
		gs.DiscardNeeded = total - TokenCap
	} else {
		gs.finishTurn()
	}
}

// DiscardColor discards one token of color c from the active player back to
// the bank, during PhaseDiscard.
func (gs *GameState) DiscardColor(c Color) bool {
	if gs.Phase != PhaseDiscard {
		return false
	}
	if c < 0 || c >= NumColors {
		return false
	}
	p := &gs.Players[gs.Turn]
	if p.Tokens[c] == 0 {
		return false
	}
	p.Tokens[c]--
	gs.Bank[c]++
	gs.DiscardNeeded--
	gs.afterDiscardStep()
	return true
}

// DiscardGold discards one gold token from the active player, during
// PhaseDiscard.
func (gs *GameState) DiscardGold() bool {
	if gs.Phase != PhaseDiscard {
		return false
	}
	p := &gs.Players[gs.Turn]
	if p.Gold == 0 {
		return false
	}
	p.Gold--
	gs.BankGold++
	gs.DiscardNeeded--
	gs.afterDiscardStep()
	return true
}

func (gs *GameState) afterDiscardStep() {
	if gs.DiscardNeeded <= 0 {
		gs.Phase = PhasePlaying
		gs.finishTurn()
	}
}

// finishTurn applies the 15-prestige end-game trigger and "finish the round"
// timing: reaching PrestigeToWin sets EndTriggered, but the game only ends
// once the LAST player in the fixed 2-player turn order (index 1) has also
// completed their own turn — so a trigger on player 0's turn still lets
// player 1 play once more before the game ends.
func (gs *GameState) finishTurn() {
	if gs.Players[gs.Turn].Prestige >= PrestigeToWin {
		gs.EndTriggered = true
	}
	if gs.EndTriggered && gs.Turn == lastTurnIndex {
		gs.Phase = PhaseDone
		gs.WinnerIdx = FinalWinner(summaryOf(gs.Players[0]), summaryOf(gs.Players[1]))
		return
	}
	gs.Turn = 1 - gs.Turn
	gs.Phase = PhasePlaying
}

// PlayerSummary is the minimal per-player data FinalWinner needs — kept
// separate from PlayerState so the tiebreak logic is testable in isolation.
type PlayerSummary struct {
	Prestige  int
	CardCount int
}

func summaryOf(p PlayerState) PlayerSummary {
	return PlayerSummary{Prestige: p.Prestige, CardCount: len(p.Cards)}
}

// EndTrigger reports whether reaching this much prestige triggers game-end
// (a standalone, testable version of the finishTurn threshold check).
func EndTrigger(prestige int) bool {
	return prestige >= PrestigeToWin
}

// FinalWinner returns 0 if player 0 (summary a) wins, 1 if player 1
// (summary b) wins, or -1 for a genuine tie: higher Prestige wins; if equal,
// FEWER development cards owned wins (the efficiency tiebreak); if still
// equal, it's a tie.
func FinalWinner(a, b PlayerSummary) int {
	if a.Prestige != b.Prestige {
		if a.Prestige > b.Prestige {
			return 0
		}
		return 1
	}
	if a.CardCount != b.CardCount {
		if a.CardCount < b.CardCount {
			return 0
		}
		return 1
	}
	return -1
}
