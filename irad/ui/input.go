package ui

import (
	"image"

	"irad/game"
)

// TouchResult describes what a board touch resolved to.
type TouchResult struct {
	Move          game.Move // valid when HasMove
	HasMove       bool
	NewlySelected int  // >=0 when a stone was selected (moving phase), else -1
	Changed       bool // selection changed (needs redraw) even without a move
}

// ResolveTouch interprets a touch at point p on the board for the current
// game state, following the §8 input table. It does not mutate gs; it returns
// the intended action for the caller to apply.
func ResolveTouch(l *Layout, gs *game.GameState, p image.Point) TouchResult {
	res := TouchResult{NewlySelected: -1}
	b := &gs.Board

	x, y, ok := l.ScreenToCell(p)
	if !ok {
		return res
	}

	switch gs.Phase {
	case game.PhasePlacing:
		if b.DropMode {
			if idx, ok := b.DropTarget(x); ok {
				res.Move = game.Move{From: -1, To: idx}
				res.HasMove = true
			}
			return res
		}
		i := b.Idx(x, y)
		if b.Cells[i] == game.PlayerNone && !b.Blocked[i] {
			res.Move = game.Move{From: -1, To: i}
			res.HasMove = true
		}
		return res

	case game.PhaseMoving:
		i := b.Idx(x, y)
		if gs.Selected < 0 {
			// Nothing selected: select an own stone.
			if b.Cells[i] == gs.Turn {
				res.NewlySelected = i
				res.Changed = true
			}
			return res
		}
		// Something selected.
		switch {
		case b.Cells[i] == gs.Turn:
			// Tapped another own stone: switch selection.
			res.NewlySelected = i
			res.Changed = true
		case b.Cells[i] == game.PlayerNone && !b.Blocked[i] && adjacent(b, gs.Selected, i):
			res.Move = game.Move{From: gs.Selected, To: i}
			res.HasMove = true
		default:
			// Ignore (non-adjacent empty, blocked, or opponent stone).
		}
		return res
	}
	return res
}

// adjacent reports whether two indices are king-move neighbours.
func adjacent(b *game.Board, a, c int) bool {
	ax, ay := b.XY(a)
	cx, cy := b.XY(c)
	dx := ax - cx
	if dx < 0 {
		dx = -dx
	}
	dy := ay - cy
	if dy < 0 {
		dy = -dy
	}
	return (dx <= 1 && dy <= 1) && !(dx == 0 && dy == 0)
}
