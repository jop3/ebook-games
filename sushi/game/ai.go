package game

// AI drafting heuristic: no deep search, just a per-card expected-value
// estimate given the player's own tableau, the OPPONENTS' visible tableaus
// (maki majority is public information — only hands are hidden), and how
// many cards remain in the hand being drafted from this turn (a crude 1-ply
// proxy for "will an unclaimed card of this kind wheel back around to me
// before the round ends" — combo pieces taken with few cards left in hand
// have no time left to pay off, so their value is discounted near the end of
// a round).

// aiCardValue estimates the value of drafting card c into a tableau, given:
//   - tableau: this player's own played cards so far this round.
//   - oppTableaus: every OTHER player's played cards so far (visible).
//   - handLenBefore: how many cards are in the hand being drafted from,
//     BEFORE this pick (i.e. how many turns including this one remain).
//   - ownPudding / maxOppPudding: persistent Pudding totals, used to chase
//     Pudding harder when behind.
func aiCardValue(c Card, tableau []Card, oppTableaus [][]Card, handLenBefore int, ownPudding, maxOppPudding int) float64 {
	switch c.Kind {
	case KindNigiri:
		if bankedWasabiCount(tableau) > 0 {
			return float64(c.N) * 3 // a banked Wasabi is waiting — cash it in
		}
		return float64(c.N) * 1.1 // still solid on its own
	case KindWasabi:
		if bankedWasabiCount(tableau) > 0 {
			return 1.0 // already holding one unused Wasabi; a 2nd is speculative
		}
		switch {
		case handLenBefore <= 1:
			return 0.3 // this is the round's last card — no nigiri can ever follow it
		case handLenBefore == 2:
			return 2.0 // one more turn only; moderate bet
		default:
			return 4.0 // plenty of turns left for a nigiri to show up
		}
	case KindTempura:
		if countKind(tableau, KindTempura)%2 == 1 {
			return 5.0 // completes a pair right now
		}
		return 2.2 // banks on a second one arriving later
	case KindSashimi:
		switch countKind(tableau, KindSashimi) % 3 {
		case 2:
			return 8.5 // completes a triple right now
		case 1:
			return 4.0 // one away, worth pursuing
		default:
			return 1.8 // starting a fresh set is speculative
		}
	case KindDumpling:
		n := countKind(tableau, KindDumpling)
		return float64(dumplingScore(n+1) - dumplingScore(n)) // marginal value
	case KindMaki:
		mine := makiCount(tableau) + c.N
		bestOpp := 0
		for _, t := range oppTableaus {
			if m := makiCount(t); m > bestOpp {
				bestOpp = m
			}
		}
		v := float64(c.N) * 0.8
		switch {
		case mine > bestOpp:
			v += 2.5 // takes or extends the lead
		case bestOpp-mine <= 2:
			v += 1.2 // still within striking distance — worth contesting
		}
		return v
	case KindChopsticks:
		if countKind(tableau, KindChopsticks) > 0 {
			return 1.0 // already have an unplayed one in the tableau
		}
		if handLenBefore <= 1 {
			return 0.5 // no future turn left to use it this round
		}
		return 3.2
	case KindPudding:
		v := 1.5
		if ownPudding < maxOppPudding {
			v += 1.5 // behind on Pudding — every one narrows the gap
		}
		return v
	}
	return 0
}

// bankedWasabiCount is the number of Wasabi cards in tableau not yet
// consumed by a later Nigiri (mirrors scoreNigiriCat's walk).
func bankedWasabiCount(tableau []Card) int {
	banked := 0
	for _, c := range tableau {
		switch c.Kind {
		case KindWasabi:
			banked++
		case KindNigiri:
			if banked > 0 {
				banked--
			}
		}
	}
	return banked
}

// AIPick computes player i's drafting choice for the CURRENT turn, using
// only that player's own Hand/Tableau plus every other player's visible
// Tableau — never another player's Hand, and never another player's pick
// for this same turn (there is no such thing yet: AIPick is pure over the
// pre-turn State, so calling it for every AI seat before any pick is
// applied is exactly what "simultaneous" requires).
func (s *State) AIPick(i int) Pick {
	p := &s.Players[i]
	hand := p.Hand
	oppTableaus := make([][]Card, 0, s.NumPlayers-1)
	maxOppPudding := 0
	for j := range s.Players {
		if j == i {
			continue
		}
		oppTableaus = append(oppTableaus, s.Players[j].Tableau)
		if s.Players[j].Pudding > maxOppPudding {
			maxOppPudding = s.Players[j].Pudding
		}
	}
	values := make([]float64, len(hand))
	for k, c := range hand {
		values[k] = aiCardValue(c, p.Tableau, oppTableaus, len(hand), p.Pudding, maxOppPudding)
	}
	best := argmaxIdx(values, -1)
	hasChop := countKind(p.Tableau, KindChopsticks) > 0
	if hasChop && len(hand) >= 2 {
		second := argmaxIdx(values, best)
		idx := []int{best, second}
		sortInts(idx)
		return Pick{Idx: idx}
	}
	return Pick{Idx: []int{best}}
}

// argmaxIdx returns the index of the largest value in vs, skipping the index
// `exclude` (pass -1 to exclude nothing). Ties resolve to the first (lowest)
// index, keeping the AI deterministic for a given hand.
func argmaxIdx(vs []float64, exclude int) int {
	best := -1
	for i, v := range vs {
		if i == exclude {
			continue
		}
		if best == -1 || v > vs[best] {
			best = i
		}
	}
	return best
}
