package game

import "image"

// Jump is a single jump: a stone moves from From, over an enemy stone at
// Over, landing on the empty cell at To (exactly two cells beyond From in
// one of the 4 orthogonal directions). Over is always removed.
type Jump struct {
	From, Over, To image.Point
}

// LegalJumpsFrom returns every single legal jump available to side's stone
// currently sitting at pos: for each of the 4 orthogonal directions, the
// adjacent cell must hold an enemy stone and the cell immediately beyond it
// must be empty and in bounds. It does not consider chaining.
func (b *Board) LegalJumpsFrom(pos image.Point, side Cell) []Jump {
	var out []Jump
	if b.At(pos.X, pos.Y) != side {
		return out
	}
	enemy := side.Opponent()
	for _, d := range dirs4 {
		overX, overY := pos.X+d.X, pos.Y+d.Y
		toX, toY := pos.X+2*d.X, pos.Y+2*d.Y
		if !inBounds(toX, toY) {
			continue
		}
		if b.At(overX, overY) != enemy {
			continue
		}
		if b.At(toX, toY) != Empty {
			continue
		}
		out = append(out, Jump{From: pos, Over: image.Pt(overX, overY), To: image.Pt(toX, toY)})
	}
	return out
}

// LegalJumpsAnywhere returns every single first-jump legally available to
// side, from every one of side's stones on the board.
func (b *Board) LegalJumpsAnywhere(side Cell) []Jump {
	var out []Jump
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) != side {
				continue
			}
			out = append(out, b.LegalJumpsFrom(image.Pt(x, y), side)...)
		}
	}
	return out
}

// HasAnyJump reports whether side has at least one legal jump anywhere on
// the board — the sole win/loss test in Konane (a side with none, on their
// turn, loses immediately).
func (b *Board) HasAnyJump(side Cell) bool {
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) != side {
				continue
			}
			if len(b.LegalJumpsFrom(image.Pt(x, y), side)) > 0 {
				return true
			}
		}
	}
	return false
}

// Apply plays jump j for side (assumed legal — callers should check via
// LegalJumpsFrom first) and returns the resulting board: From is vacated,
// Over (the jumped enemy stone) is removed, To now holds side's stone.
func (b Board) Apply(j Jump, side Cell) Board {
	nb := b
	nb.set(j.From.X, j.From.Y, Empty)
	nb.set(j.Over.X, j.Over.Y, Empty)
	nb.set(j.To.X, j.To.Y, side)
	return nb
}

// ApplyChain plays an entire chain of jumps for side in sequence, returning
// the final resulting board.
func ApplyChain(b Board, side Cell, chain []Jump) Board {
	nb := b
	for _, j := range chain {
		nb = nb.Apply(j, side)
	}
	return nb
}

// capturedCells returns every cell captured (jumped over) by a chain, in
// order — used to flash the whole turn's captures on the UI.
func capturedCells(chain []Jump) []image.Point {
	out := make([]image.Point, len(chain))
	for i, j := range chain {
		out[i] = j.Over
	}
	return out
}

// findJump returns the jump in js that lands on to, if any.
func findJump(js []Jump, to image.Point) (Jump, bool) {
	for _, j := range js {
		if j.To == to {
			return j, true
		}
	}
	return Jump{}, false
}

// GenerateChains returns every legal move available to side on board b: a
// complete chain of 1 or more jumps by a single stone. Because a player may
// choose to stop chaining at any point (they are never forced to take the
// longest available chain), every prefix of a possible multi-jump sequence
// is itself a distinct, returned candidate move — a chain of length 1 (stop
// after the first jump), length 2, and so on, up to however far that stone
// can keep jumping.
func GenerateChains(b Board, side Cell) [][]Jump {
	var out [][]Jump
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) != side {
				continue
			}
			start := image.Pt(x, y)
			var walk func(cur image.Point, board Board, path []Jump)
			walk = func(cur image.Point, board Board, path []Jump) {
				if len(path) > 0 {
					cp := make([]Jump, len(path))
					copy(cp, path)
					out = append(out, cp)
				}
				for _, j := range board.LegalJumpsFrom(cur, side) {
					nb := board.Apply(j, side)
					walk(j.To, nb, append(path, j))
				}
			}
			walk(start, b, nil)
		}
	}
	return out
}
