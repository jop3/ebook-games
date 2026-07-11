package game

// ai.go implements Juvelerna's AI as a greedy-plus-shallow-lookahead
// heuristic, in the same spirit as sushi/game/ai.go: rather than a deep
// alpha-beta search over the wide take-3/take-2/reserve/buy action space
// (which branches quickly), each legal action is valued by a per-action
// heuristic (immediate prestige gained, progress toward affording every
// visible card, progress toward every visible noble's requirement, a small
// token-overflow penalty) and, at Medel/Svår, a further one-ply "does this
// leave the opponent an obvious great reply" denial term — this is
// legitimate here (unlike a hidden-hand game) precisely because Splendor is
// perfect information: nothing about the opponent's simulated reply is
// guesswork.

const denialWeight = 1.0

// ActionKind identifies which of the 4 turn actions an Action represents.
type ActionKind int

const (
	ActionTake3 ActionKind = iota
	ActionTake2
	ActionReserveTableau
	ActionReserveBlind
	ActionBuyTableau
	ActionBuyReserved
	ActionPass // last resort: no other action exists
)

// Action is a candidate turn action, as enumerated by LegalActions and
// applied by Apply. Only the fields relevant to Kind are meaningful.
type Action struct {
	Kind        ActionKind
	Colors      [3]Color // Take3: the NumTake colors used; Take2: Colors[0]
	NumTake     int      // Take3: how many of Colors are used (0 means 3)
	Tier, Slot  int      // Reserve/Buy tableau
	ReservedIdx int      // BuyReserved
}

// takeColors returns the Take3 action's color list, honouring the take-fewer
// fallback encoding (NumTake 0 = classic 3).
func (a Action) takeColors() []Color {
	n := a.NumTake
	if n == 0 {
		n = 3
	}
	return a.Colors[:n]
}

// LegalActions enumerates every legal Action for the player currently to
// move (gs.Turn), appending the last-resort Pass only when nothing else is
// legal — so a pathological empty-bank position can never deadlock a turn.
func (gs *GameState) LegalActions() []Action {
	out := gs.legalActionsNoPass()
	if len(out) == 0 && gs.Phase == PhasePlaying {
		out = append(out, Action{Kind: ActionPass})
	}
	return out
}

func (gs *GameState) legalActionsNoPass() []Action {
	var out []Action
	p := &gs.Players[gs.Turn]

	// Take different colors: 3 normally; when fewer piles remain, the single
	// maximal take of every remaining color (official take-fewer fallback).
	switch n := gs.takeableColors(); n {
	case 3:
		for i := Color(0); i < NumColors; i++ {
			for j := i + 1; j < NumColors; j++ {
				for k := j + 1; k < NumColors; k++ {
					colors := [3]Color{i, j, k}
					if gs.CanTake3(colors) {
						out = append(out, Action{Kind: ActionTake3, Colors: colors, NumTake: 3})
					}
				}
			}
		}
	case 1, 2:
		var a Action
		a.Kind = ActionTake3
		for c := Color(0); c < NumColors; c++ {
			if gs.Bank[c] > 0 {
				a.Colors[a.NumTake] = c
				a.NumTake++
			}
		}
		out = append(out, a)
	}
	for c := Color(0); c < NumColors; c++ {
		if gs.CanTake2(c) {
			out = append(out, Action{Kind: ActionTake2, Colors: [3]Color{c}})
		}
	}
	if len(p.Reserved) < MaxReserved {
		for t := 0; t < NumTiers; t++ {
			for s := 0; s < TableauSlots; s++ {
				if gs.CanReserveTableau(t, s) {
					out = append(out, Action{Kind: ActionReserveTableau, Tier: t, Slot: s})
				}
			}
			if gs.CanReserveBlind(t) {
				out = append(out, Action{Kind: ActionReserveBlind, Tier: t})
			}
		}
	}
	for t := 0; t < NumTiers; t++ {
		for s := 0; s < TableauSlots; s++ {
			if gs.CanBuyTableau(t, s) {
				out = append(out, Action{Kind: ActionBuyTableau, Tier: t, Slot: s})
			}
		}
	}
	for idx := range p.Reserved {
		if gs.CanBuyReserved(idx) {
			out = append(out, Action{Kind: ActionBuyReserved, ReservedIdx: idx})
		}
	}
	return out
}

// Apply dispatches Action a to the matching legality-checked action
// function. Returns false (no effect) if a is no longer legal.
func (gs *GameState) Apply(a Action) bool {
	switch a.Kind {
	case ActionTake3:
		return gs.TakeColors(a.takeColors())
	case ActionTake2:
		return gs.Take2(a.Colors[0])
	case ActionPass:
		return gs.Pass()
	case ActionReserveTableau:
		return gs.ReserveTableau(a.Tier, a.Slot)
	case ActionReserveBlind:
		return gs.ReserveBlind(a.Tier)
	case ActionBuyTableau:
		return gs.BuyTableau(a.Tier, a.Slot)
	case ActionBuyReserved:
		return gs.BuyReserved(a.ReservedIdx)
	}
	return false
}

// BestAction returns the AI's chosen action for the player to move
// (DepthEasy/DepthMedium/DepthHard). ok is false only if there are no legal
// actions (should not happen in a normal game — some action is always
// available since take-3-of-different-colors degrades gracefully, but the
// AI must still handle a pathological empty bank+empty tableau gracefully).
func BestAction(gs *GameState, diff int) (Action, bool) {
	candidates := gs.LegalActions()
	if len(candidates) == 0 {
		return Action{}, false
	}

	var value func(Action) float64
	switch {
	case diff <= DepthEasy:
		value = func(a Action) float64 { return baseValue(gs, a) }
	case diff == DepthMedium:
		value = func(a Action) float64 { return lookaheadValue(gs, a, false) }
	default:
		value = func(a Action) float64 { return lookaheadValue(gs, a, true) }
	}

	best := candidates[0]
	bestV := value(best)
	for _, a := range candidates[1:] {
		if v := value(a); v > bestV {
			best, bestV = a, v
		}
	}
	return best, true
}

// baseValue estimates the immediate merit of action a for the player
// currently to move: simulate it on a clone, auto-resolve any resulting
// noble-choice/discard sub-phase for that same player (so the value
// reflects the position AFTER the action fully settles), then score the
// resulting player state.
func baseValue(gs *GameState, a Action) float64 {
	side := gs.Turn
	ns := gs.Clone()
	if !ns.Apply(a) {
		return -1e9 // illegal (shouldn't happen for enumerated candidates)
	}
	autoResolvePending(ns, side)
	p := ns.Players[side]
	v := float64(p.Prestige) * 20
	v += float64(totalBonuses(&p)) * 6 // owning ANY card is permanent engine-building progress
	v += cardProgress(ns, side)
	v += nobleProgress(ns, side)
	if over := p.TokensTotal() - TokenCap; over > 0 {
		v -= float64(over) * 1.5
	}
	return v
}

// totalBonuses sums a player's permanent per-color card discounts.
func totalBonuses(p *PlayerState) int {
	n := 0
	for c := Color(0); c < NumColors; c++ {
		n += p.Bonuses[c]
	}
	return n
}

// lookaheadValue extends baseValue with a one-ply "denial" term: after our
// move (and its own resolution), if it is now the opponent's turn, subtract
// a weighted estimate of the opponent's own best baseValue reply. When
// hard is true, that reply is estimated using lookaheadValue itself (one
// ply deeper, non-hard) — i.e. Svår assumes the opponent replies at Medel
// strength, without an explicit deeper recursive tree.
func lookaheadValue(gs *GameState, a Action, hard bool) float64 {
	side := gs.Turn
	v := baseValue(gs, a)
	ns := gs.Clone()
	if !ns.Apply(a) {
		return v
	}
	autoResolvePending(ns, side)
	if ns.Phase != PhasePlaying || ns.Turn == side {
		return v
	}
	oppCands := topCandidates(ns, searchWidth)
	best := 0.0
	for _, om := range oppCands {
		var bv float64
		if hard {
			bv = lookaheadValue(ns, om, false)
		} else {
			bv = baseValue(ns, om)
		}
		if bv > best {
			best = bv
		}
	}
	return v - denialWeight*best
}

const searchWidth = 12

// topCandidates returns up to k of ns's legal actions for the player to
// move, favoring buys and reserves (the actions whose value varies the most)
// over pure token-taking so the capped-width opponent-reply search still
// looks at the moves most likely to matter.
func topCandidates(ns *GameState, k int) []Action {
	all := ns.LegalActions()
	if len(all) <= k {
		return all
	}
	rank := func(a Action) int {
		switch a.Kind {
		case ActionBuyTableau, ActionBuyReserved:
			return 0
		case ActionReserveTableau, ActionReserveBlind:
			return 1
		default:
			return 2
		}
	}
	out := make([]Action, len(all))
	copy(out, all)
	// simple stable partition by rank (buys first, then reserves, then takes)
	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && rank(out[j-1]) > rank(out[j]) {
			out[j-1], out[j] = out[j], out[j-1]
			j--
		}
	}
	return out[:k]
}

// autoResolvePending auto-settles any PhaseNobleChoice/PhaseDiscard left
// pending for `side` after simulating an action, using the same simple
// policies StepAI uses for a real AI turn — needed so the lookahead scores
// the position AFTER the turn fully resolves, not mid-resolution.
func autoResolvePending(ns *GameState, side int) {
	for ns.Turn == side {
		switch ns.Phase {
		case PhaseNobleChoice:
			ns.ChooseNoble(0)
		case PhaseDiscard:
			aiDiscardOnce(ns)
		default:
			return
		}
	}
}

// cardProgress scores how close `side` is to affording a good next
// purchase, looking at every tableau slot plus side's own reserved cards.
//
// Only the SINGLE best already-affordable card counts toward the "can buy
// right now" bonus (via bestAfford) rather than summing over every
// affordable card: a player can only actually spend their tokens once, so
// crediting N simultaneously-affordable cards as if all N were about to be
// bought systematically overvalues hoarding tokens over actually buying
// anything (an early version of this heuristic had exactly that bug — the
// AI took tokens forever and never purchased a single card). Cards still out
// of reach contribute a much smaller, summed proximity term (how close, not
// whether affordable), so pursuing several plausible near-term targets at
// once is still rewarded, just far more mildly than an actual purchase.
func cardProgress(ns *GameState, side int) float64 {
	p := &ns.Players[side]
	bestAfford := 0.0
	proximity := 0.0
	consider := func(card Card) {
		if card.Tier == 0 {
			return
		}
		missing := 0
		for c := Color(0); c < NumColors; c++ {
			need := card.Cost[c] - p.Bonuses[c]
			if need < 0 {
				need = 0
			}
			have := p.Tokens[c]
			if have > need {
				have = need
			}
			missing += need - have
		}
		if missing <= p.Gold {
			if v := 5 + float64(card.Points)*3; v > bestAfford {
				bestAfford = v
			}
		} else {
			proximity += 2.0/float64(missing+1) + float64(card.Points)*0.3
		}
	}
	for t := 0; t < NumTiers; t++ {
		for s := 0; s < TableauSlots; s++ {
			consider(ns.Tableau[t][s])
		}
	}
	for _, c := range p.Reserved {
		consider(c)
	}
	return bestAfford + proximity*0.3
}

// nobleProgress scores how close `side` is to qualifying for every
// still-available noble.
func nobleProgress(ns *GameState, side int) float64 {
	p := &ns.Players[side]
	total := 0.0
	for _, n := range ns.Nobles {
		missing := 0
		for c := Color(0); c < NumColors; c++ {
			if need := n.Requirement[c] - p.Bonuses[c]; need > 0 {
				missing += need
			}
		}
		if missing == 0 {
			total += 15
		} else {
			total += 3.0 / float64(missing+1)
		}
	}
	return total
}

// aiDiscardOnce discards a single token from whichever color the active
// player holds the most of (gold last, since it's the most flexible token to
// keep) — a simple greedy policy used both by StepAI (real AI turns) and by
// autoResolvePending (lookahead simulation).
func aiDiscardOnce(gs *GameState) bool {
	p := &gs.Players[gs.Turn]
	best := -1
	bestN := 0
	for c := Color(0); c < NumColors; c++ {
		if p.Tokens[c] > bestN {
			bestN = p.Tokens[c]
			best = int(c)
		}
	}
	if best >= 0 {
		return gs.DiscardColor(Color(best))
	}
	return gs.DiscardGold()
}

// StepAI plays one step of the AI's turn (ModeAI, player 1): the main
// action if Phase == PhasePlaying, or an auto-resolution of a pending noble
// choice / forced discard otherwise. Returns true if something happened.
// Mirrors the other games' StepAI-called-once-per-Draw-frame convention: the
// caller re-arms it (via AITurn()) until the turn fully passes.
func (gs *GameState) StepAI() bool {
	if !gs.AITurn() {
		return false
	}
	switch gs.Phase {
	case PhasePlaying:
		act, ok := BestAction(gs, gs.AILevel)
		if !ok {
			return false
		}
		return gs.Apply(act)
	case PhaseNobleChoice:
		return gs.ChooseNoble(0)
	case PhaseDiscard:
		return aiDiscardOnce(gs)
	}
	return false
}
