package game

// This file scores one player's Tableau (the cards they've kept this round).
// Every category is one small pure function, independently unit-tested
// against a hand-computed expected score — the same discipline this repo
// applies to its puzzle validators.

// RoundScore is the per-category breakdown for one player's tableau at the
// end of a round (Pudding is scored separately, only at game end).
type RoundScore struct {
	Nigiri   int
	Tempura  int
	Sashimi  int
	Dumpling int
	Maki     int // filled in by ScoreRound via ScoreMaki; 0 if scored alone
}

// Total sums the round's categories (everything except Pudding, which is
// scored once at game end by ScorePudding).
func (r RoundScore) Total() int {
	return r.Nigiri + r.Tempura + r.Sashimi + r.Dumpling + r.Maki
}

// scoreNigiriCat scores Nigiri (and the Wasabi that triples it) by walking
// the tableau in play order: each Wasabi banks one "next nigiri x3" credit;
// each Nigiri consumes one banked credit if available (tripling its value),
// otherwise scores its plain value. A Wasabi played AFTER a Nigiri never
// applies to it — order matters, which is exactly why this walks the slice
// in append (chronological) order rather than counting kinds independently.
func scoreNigiriCat(tableau []Card) int {
	total := 0
	bankedWasabi := 0
	for _, c := range tableau {
		switch c.Kind {
		case KindWasabi:
			bankedWasabi++
		case KindNigiri:
			if bankedWasabi > 0 {
				total += c.N * 3
				bankedWasabi--
			} else {
				total += c.N
			}
		}
	}
	return total
}

// scoreTempuraCat: each pair of Tempura is worth 5; an unpaired leftover
// scores 0.
func scoreTempuraCat(tableau []Card) int {
	return (countKind(tableau, KindTempura) / 2) * 5
}

// scoreSashimiCat: each complete set of 3 Sashimi is worth 10; an incomplete
// set (0, 1, or 2 leftover) scores 0.
func scoreSashimiCat(tableau []Card) int {
	return (countKind(tableau, KindSashimi) / 3) * 10
}

// dumplingTable[n] is the score for exactly n Dumplings, n in [0,5]; 5+ all
// score the same as 5 (the ceiling in the rules).
var dumplingTable = [...]int{0, 1, 3, 6, 10, 15}

func dumplingScore(n int) int {
	if n > 5 {
		n = 5
	}
	if n < 0 {
		n = 0
	}
	return dumplingTable[n]
}

func scoreDumplingCat(tableau []Card) int {
	return dumplingScore(countKind(tableau, KindDumpling))
}

// ScoreMaki scores the maki-icon majority across all players' tableaus for
// one round: most icons = 6 (split, rounded down, on a tie), second-most = 3
// (same tie handling). A player with 0 maki icons never scores. Standard
// Sushi Go! ruling: if two or more players TIE for the most icons, they
// split the 6 and NO second place is awarded at all that round (there isn't
// a meaningful "second" when the top spot is shared).
//
// Judgment call (spec explicitly asks for the smallest reasonable choice,
// stated here): with exactly 2 players, the lone second-place bonus is ALSO
// dropped — mirroring the 2-player Pudding rule below, since with only two
// players "second place" is just "not first," which is a strange thing to
// reward. With 3+ players (and a clear, untied leader) second place is
// awarded normally, including when it is itself tied.
func ScoreMaki(tableaus [][]Card) []int {
	n := len(tableaus)
	out := make([]int, n)
	counts := make([]int, n)
	maxCount := 0
	for i, t := range tableaus {
		counts[i] = makiCount(t)
		if counts[i] > maxCount {
			maxCount = counts[i]
		}
	}
	if maxCount == 0 {
		return out // nobody played any maki
	}
	var firstPlace []int
	for i, c := range counts {
		if c == maxCount {
			firstPlace = append(firstPlace, i)
		}
	}
	share := 6 / len(firstPlace)
	for _, i := range firstPlace {
		out[i] += share
	}
	if len(firstPlace) > 1 {
		return out // tied for the lead: no second place this round
	}
	if n == 2 {
		return out // 2-player judgment call: no second-place award
	}
	secondCount := 0
	for _, c := range counts {
		if c < maxCount && c > secondCount {
			secondCount = c
		}
	}
	if secondCount == 0 {
		return out // no one else has any maki at all
	}
	var secondPlace []int
	for i, c := range counts {
		if c == secondCount {
			secondPlace = append(secondPlace, i)
		}
	}
	share2 := 3 / len(secondPlace)
	for _, i := range secondPlace {
		out[i] += share2
	}
	return out
}

// ScoreRound scores every non-Pudding category for each player's tableau,
// including the cross-player Maki majority.
func ScoreRound(tableaus [][]Card) []RoundScore {
	out := make([]RoundScore, len(tableaus))
	maki := ScoreMaki(tableaus)
	for i, t := range tableaus {
		out[i] = RoundScore{
			Nigiri:   scoreNigiriCat(t),
			Tempura:  scoreTempuraCat(t),
			Sashimi:  scoreSashimiCat(t),
			Dumpling: scoreDumplingCat(t),
			Maki:     maki[i],
		}
	}
	return out
}

// ScorePudding scores accumulated Pudding counts at game end: most = +6,
// fewest = -6, ties on either end split their award/penalty rounded down
// (toward zero, i.e. the smaller-magnitude split — the same "split down"
// rule as Maki).
//
// Judgment call: with exactly 2 players, the -6 penalty is dropped (the
// official 2-player variant), so the trailing player merely scores 0 extra;
// the +6 (or its split) is still awarded normally. If every player has the
// same Pudding count (including the whole-field tie in a 2-player game),
// nobody gains or loses anything — "most" and "fewest" would be the same
// people, which isn't a meaningful distinction to reward or punish.
func ScorePudding(counts []int) []int {
	n := len(counts)
	out := make([]int, n)
	if n == 0 {
		return out
	}
	maxC, minC := counts[0], counts[0]
	for _, c := range counts {
		if c > maxC {
			maxC = c
		}
		if c < minC {
			minC = c
		}
	}
	if maxC == minC {
		return out // everyone tied overall
	}
	var mostPlayers, fewestPlayers []int
	for i, c := range counts {
		if c == maxC {
			mostPlayers = append(mostPlayers, i)
		}
		if c == minC {
			fewestPlayers = append(fewestPlayers, i)
		}
	}
	plus := 6 / len(mostPlayers)
	for _, i := range mostPlayers {
		out[i] += plus
	}
	if n > 2 {
		minus := -6 / len(fewestPlayers)
		for _, i := range fewestPlayers {
			out[i] += minus
		}
	}
	return out
}
