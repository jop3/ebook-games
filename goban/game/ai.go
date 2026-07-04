package game

import (
	"image"
	"math/rand"
)

// BestMove returns a weak ("svag") AI move for color on board b. It is
// offered only for 9x9 in the menu — a real search-based Go AI is out of
// scope for this device's ARM chip, so this is a deliberately simple, fast
// heuristic: prefer moves that capture, avoid leaving your own new stone in
// self-atari (unless it just captured), rescue a friendly group that was in
// atari, put an enemy group in atari, and otherwise mildly prefer contested
// ground near existing stones over playing in an empty corner. It evaluates
// every legal candidate move once (no search), so it stays comfortably fast
// even on 19x19 despite only ever being offered for 9x9.
func BestMove(b Board, color Color, koPrev *Board) (image.Point, bool) {
	opp := color.Opponent()
	size := b.Size()

	type candidate struct {
		p     image.Point
		score int
	}
	var cands []candidate

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			p := image.Pt(x, y)
			if b.At(p) != Empty {
				continue
			}
			nb, captured, ok := Place(b, p, color)
			if !ok {
				continue
			}
			if koPrev != nil && Equal(nb, *koPrev) {
				continue // ko-forbidden
			}

			score := len(captured) * 30

			ownGroup := Group(nb, p)
			ownLibs := len(Liberties(nb, ownGroup))
			if ownLibs == 1 && len(captured) == 0 {
				score -= 25 // avoid self-atari unless the move just captured
			}
			score += ownLibs

			touching := 0
			for _, n := range b.Neighbors(p) {
				switch b.At(n) {
				case opp:
					touching++
					og := Group(b, n)
					if len(Liberties(b, og)) == 1 {
						score += 12 // this neighbor enemy group was already in atari
					}
				case color:
					touching++
					mg := Group(b, n)
					if len(Liberties(b, mg)) == 1 {
						score += 15 // rescue a friendly group that was in atari
					}
				}
			}
			if touching > 0 {
				score += 2 // mildly prefer contested ground over an empty corner
			}
			score += rand.Intn(3) // small jitter to break ties/avoid staleness

			cands = append(cands, candidate{p, score})
		}
	}

	if len(cands) == 0 {
		return image.Point{}, false
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if c.score > best.score {
			best = c
		}
	}
	return best.p, true
}
