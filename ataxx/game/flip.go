package game

import "image"

// Apply plays move m (assumed legal — callers should check IsLegalMove
// first) and returns the resulting board plus the list of enemy cells
// flipped to the mover's color by it.
//
// A clone (Chebyshev distance 1) leaves From occupied by the mover and
// places a new man at To; a jump (distance 2) vacates From before placing
// the man at To. Either way, every one of the 8 neighbors of To — not just
// the 4 orthogonal ones, the classic bug — that holds an enemy man flips to
// the mover's color.
func (b Board) Apply(m Move) (Board, []image.Point) {
	mover := b.At(m.From.X, m.From.Y)
	nb := b
	if m.IsJump() {
		nb.set(m.From.X, m.From.Y, Empty)
	}
	nb.set(m.To.X, m.To.Y, mover)

	enemy := mover.Opponent()
	var flipped []image.Point
	for _, d := range neighbor8 {
		x, y := m.To.X+d.X, m.To.Y+d.Y
		if nb.At(x, y) == enemy {
			nb.set(x, y, mover)
			flipped = append(flipped, image.Pt(x, y))
		}
	}
	return nb, flipped
}
