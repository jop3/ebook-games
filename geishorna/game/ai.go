package game

// The opponent's hand (and Secret card) are hidden information, so — like
// sushi/expeditionen — the AI runs no minimax. It scores every candidate
// action with a greedy board-evaluation heuristic computed only from public
// information (both players' face-up fields, the persistent favor markers) plus
// its own hand and Secret, and picks the highest-scoring one. For the two
// interactive actions (Gift, Competition) it assumes the opponent will respond
// greedily in the opponent's own favor, and scores the action by what is left
// after that response.
//
// Board evaluation (evalFor) rewards being the projected end-holder of each
// geisha, weighted by charm, with a margin gradient so the AI prefers safer
// leads and pushes contested high-charm geishas. Crucially it honors the
// tie-keeps-the-marker rule: a geisha the AI already holds is worth defending
// to a tie, and one the opponent holds must be won outright.

const negInf = -1 << 30

// evalFor scores the position from player p's perspective given both fields and
// the current favor markers. Higher is better for p.
func evalFor(p int, field [2][NumGeishas]int, favor [NumGeishas]int) float64 {
	me, opp := p, 1-p
	total := 0.0
	for i := 0; i < NumGeishas; i++ {
		a, b := field[me][i], field[opp][i]
		end := favor[i] // a tie leaves the marker where it is
		if a > b {
			end = me
		} else if b > a {
			end = opp
		}
		w := 0.0
		if end == me {
			w = 1
		} else if end == opp {
			w = -1
		}
		c := float64(Geishas[i].Charm)
		total += c * w                  // charm value of the projected outcome (11-charm path)
		total += 0.3 * w                // flat per-geisha weight (4-geisha path)
		total += 0.2 * c * float64(a-b) // margin gradient: prefer safer leads / push contests
	}
	return total
}

// aiFieldSnapshot is the AI's public+own-secret view of both fields: every
// face-up count, plus the AI's own hidden Secret (which it knows). The
// opponent's Secret is unknown and deliberately omitted.
func (s *State) aiFieldSnapshot() [2][NumGeishas]int {
	var f [2][NumGeishas]int
	f[HumanIdx] = s.Players[HumanIdx].Field
	f[AIIdx] = s.Players[AIIdx].Field
	if s.Players[AIIdx].HasSecret {
		f[AIIdx][s.Players[AIIdx].Secret.Geisha]++
	}
	return f
}

// addOne returns f with one card added to player p's count for geisha g
// (arrays copy by value, so f is untouched).
func addOne(f [2][NumGeishas]int, p, g int) [2][NumGeishas]int {
	f[p][g]++
	return f
}

// StepAI advances the AI by one micro-step: resolve an open offer the AI must
// choose from, or take the AI's action turn. Returns false if the game is not
// currently waiting on the AI. Callers loop while AITurn() to run the AI
// through any chain (e.g. resolving the human's Gift, then taking its own turn).
func (s *State) StepAI() bool {
	if s.Phase != PhaseAction {
		return false
	}
	if s.Pending != nil {
		if s.Pending.Chooser != AIIdx {
			return false
		}
		switch s.Pending.Action {
		case Gift:
			_ = s.ResolveGift(AIIdx, s.aiChooseGift())
		case Competition:
			_ = s.ResolveCompetition(AIIdx, s.aiChooseCompetition())
		}
		return true
	}
	if s.Turn != AIIdx {
		return false
	}
	s.aiAct()
	return true
}

// aiChooseGift picks, as the offer's chooser, the single offered card that
// most improves the AI's own position.
func (s *State) aiChooseGift() int {
	base := s.aiFieldSnapshot()
	actor := s.Pending.Actor
	best, bestVal := 0, float64(negInf)
	for t := range s.Pending.Cards {
		f := base
		for u, c := range s.Pending.Cards {
			if u == t {
				f[AIIdx][c.Geisha]++
			} else {
				f[actor][c.Geisha]++
			}
		}
		if v := evalFor(AIIdx, f, s.Favor); v > bestVal {
			bestVal, best = v, t
		}
	}
	return best
}

// aiChooseCompetition picks, as the offer's chooser, the pair that most
// improves the AI's own position.
func (s *State) aiChooseCompetition() int {
	base := s.aiFieldSnapshot()
	actor := s.Pending.Actor
	best, bestVal := 0, float64(negInf)
	for gi := 0; gi < 2; gi++ {
		f := base
		for k := 0; k < 2; k++ {
			dst := actor
			if k == gi {
				dst = AIIdx
			}
			for _, c := range s.Pending.Groups[k] {
				f[dst][c.Geisha]++
			}
		}
		if v := evalFor(AIIdx, f, s.Favor); v > bestVal {
			bestVal, best = v, gi
		}
	}
	return best
}

// aiAct chooses and performs the AI's best available action for this turn.
func (s *State) aiAct() {
	hand := s.Players[AIIdx].Hand
	base := s.aiFieldSnapshot()

	bestVal := float64(negInf)
	var commit func()

	if s.Available(AIIdx, Secret) {
		if idx, v := s.aiBestSecret(hand, base); v > bestVal {
			bestVal = v
			i := idx
			commit = func() { _ = s.DoSecret(AIIdx, i) }
		}
	}
	if s.Available(AIIdx, TradeOff) {
		if idxs, v := s.aiBestTradeOff(hand, base); v > bestVal {
			bestVal = v
			pick := idxs
			commit = func() { _ = s.DoTradeOff(AIIdx, pick) }
		}
	}
	if s.Available(AIIdx, Gift) {
		if idxs, v := s.aiBestGift(hand, base); v > bestVal {
			bestVal = v
			pick := idxs
			commit = func() { _ = s.DoGift(AIIdx, pick) }
		}
	}
	if s.Available(AIIdx, Competition) {
		if ga, gb, v := s.aiBestCompetition(hand, base); v > bestVal {
			bestVal = v
			a, b := ga, gb
			commit = func() { _ = s.DoCompetition(AIIdx, a, b) }
		}
	}

	if commit == nil {
		// Defensive: no scored candidate (should not happen). Fall back to the
		// first affordable unused action on the first legal cards.
		commit = s.aiFallback(hand)
	}
	commit()
}

// aiBestSecret returns the hand index whose card, kept secret, best improves
// the AI's position, and that position's value.
func (s *State) aiBestSecret(hand []Card, base [2][NumGeishas]int) (int, float64) {
	bestIdx, bestVal := 0, float64(negInf)
	for i, c := range hand {
		v := evalFor(AIIdx, addOne(base, AIIdx, c.Geisha), s.Favor)
		if v > bestVal {
			bestVal, bestIdx = v, i
		}
	}
	return bestIdx, bestVal
}

// aiBestTradeOff returns the two hand indices least worth keeping (own use low,
// opponent use high) to discard, and the resulting board value (unchanged,
// since the cards leave play).
func (s *State) aiBestTradeOff(hand []Card, base [2][NumGeishas]int) ([]int, float64) {
	selfBase := evalFor(AIIdx, base, s.Favor)
	oppBase := evalFor(HumanIdx, base, s.Favor)
	type ck struct {
		idx   int
		score float64 // lower = better to discard
	}
	cks := make([]ck, len(hand))
	for i, c := range hand {
		selfUse := evalFor(AIIdx, addOne(base, AIIdx, c.Geisha), s.Favor) - selfBase
		oppUse := evalFor(HumanIdx, addOne(base, HumanIdx, c.Geisha), s.Favor) - oppBase
		cks[i] = ck{idx: i, score: selfUse - 0.5*oppUse}
	}
	// selection sort the two lowest (hand is tiny)
	for a := 0; a < 2; a++ {
		min := a
		for b := a + 1; b < len(cks); b++ {
			if cks[b].score < cks[min].score {
				min = b
			}
		}
		cks[a], cks[min] = cks[min], cks[a]
	}
	return []int{cks[0].idx, cks[1].idx}, selfBase
}

// aiBestGift returns the three hand indices to offer whose worst-case (the
// opponent takes their best of the three) leaves the AI strongest, and that
// value.
func (s *State) aiBestGift(hand []Card, base [2][NumGeishas]int) ([]int, float64) {
	var bestPick []int
	bestVal := float64(negInf)
	forEachCombo(len(hand), 3, func(combo []int) {
		// The opponent will take whichever of the 3 helps them most; the other
		// 2 stay with the AI.
		oppBestVal := float64(negInf)
		var afterOppPick [2][NumGeishas]int
		for t := 0; t < 3; t++ {
			f := base
			for u := 0; u < 3; u++ {
				g := hand[combo[u]].Geisha
				if u == t {
					f[HumanIdx][g]++
				} else {
					f[AIIdx][g]++
				}
			}
			if v := evalFor(HumanIdx, f, s.Favor); v > oppBestVal {
				oppBestVal, afterOppPick = v, f
			}
		}
		if v := evalFor(AIIdx, afterOppPick, s.Favor); v > bestVal {
			bestVal = v
			bestPick = []int{combo[0], combo[1], combo[2]}
		}
	})
	return bestPick, bestVal
}

// aiBestCompetition returns the two pairs (as hand-index slices) whose split
// leaves the AI strongest after the opponent takes the pair better for them,
// and that value.
func (s *State) aiBestCompetition(hand []Card, base [2][NumGeishas]int) ([]int, []int, float64) {
	var bestA, bestB []int
	bestVal := float64(negInf)
	// The three ways to split four cards {0,1,2,3} into two unordered pairs.
	splits := [3][2][2]int{
		{{0, 1}, {2, 3}},
		{{0, 2}, {1, 3}},
		{{0, 3}, {1, 2}},
	}
	forEachCombo(len(hand), 4, func(combo []int) {
		for _, sp := range splits {
			pairA := []int{combo[sp[0][0]], combo[sp[0][1]]}
			pairB := []int{combo[sp[1][0]], combo[sp[1][1]]}
			// Opponent takes whichever pair helps them most.
			takeA := placePair(base, hand, pairA, HumanIdx, pairB, AIIdx)
			takeB := placePair(base, hand, pairB, HumanIdx, pairA, AIIdx)
			after := takeA
			if evalFor(HumanIdx, takeB, s.Favor) > evalFor(HumanIdx, takeA, s.Favor) {
				after = takeB
			}
			if v := evalFor(AIIdx, after, s.Favor); v > bestVal {
				bestVal = v
				bestA = []int{pairA[0], pairA[1]}
				bestB = []int{pairB[0], pairB[1]}
			}
		}
	})
	return bestA, bestB, bestVal
}

// placePair returns base with oppPair added to player po's field and myPair to
// player pm's field.
func placePair(base [2][NumGeishas]int, hand []Card, oppPair []int, po int, myPair []int, pm int) [2][NumGeishas]int {
	f := base
	for _, i := range oppPair {
		f[po][hand[i].Geisha]++
	}
	for _, i := range myPair {
		f[pm][hand[i].Geisha]++
	}
	return f
}

// aiFallback returns a commit closure for the first affordable unused action,
// used only if scoring somehow produced no candidate.
func (s *State) aiFallback(hand []Card) func() {
	for _, a := range AllActions {
		if !s.Available(AIIdx, a) {
			continue
		}
		switch a {
		case Secret:
			return func() { _ = s.DoSecret(AIIdx, 0) }
		case TradeOff:
			return func() { _ = s.DoTradeOff(AIIdx, []int{0, 1}) }
		case Gift:
			return func() { _ = s.DoGift(AIIdx, []int{0, 1, 2}) }
		case Competition:
			return func() { _ = s.DoCompetition(AIIdx, []int{0, 1}, []int{2, 3}) }
		}
	}
	return func() {} // unreachable given a non-empty hand
}

// forEachCombo calls fn with each k-sized combination of indices [0,n).
func forEachCombo(n, k int, fn func(combo []int)) {
	if k > n || k <= 0 {
		return
	}
	combo := make([]int, k)
	var rec func(start, depth int)
	rec = func(start, depth int) {
		if depth == k {
			fn(combo)
			return
		}
		for i := start; i <= n-(k-depth); i++ {
			combo[depth] = i
			rec(i+1, depth+1)
		}
	}
	rec(0, 0)
}
