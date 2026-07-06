// Package game implements the rules of "Murar" (Quoridor) with no dependency
// on the inkview SDK, so it can be unit-tested cgo-free.
//
// The board is 9x9. Each side has a single pawn, starting centered on its own
// edge; the goal is to reach any cell of the opposite edge. Each turn a player
// either steps their pawn one cell (with a jump/diagonal exception when the
// opponent's pawn is in the way), or places one of their 10 walls in the
// "groove" grid between cells — as long as the wall does not fully block
// either player's path to their own goal edge.
package game

import "image"

// Size is the edge length of the board (9x9 cells).
const Size = 9

// WallGrid is the edge length of the wall-groove grid: walls sit at the
// intersections of 4 cells, and there are Size-1 such intersections per axis.
const WallGrid = Size - 1

// Side names a player.
type Side int

const (
	// P1 ("Svart") starts on the bottom row (y=Size-1) and aims for the top
	// row (y=0).
	P1 Side = iota
	// P2 ("Vit") starts on the top row (y=0) and aims for the bottom row
	// (y=Size-1).
	P2
)

// Opponent returns the other side.
func (s Side) Opponent() Side {
	if s == P1 {
		return P2
	}
	return P1
}

// GoalRow returns the row index a side must reach to win.
func GoalRow(s Side) int {
	if s == P1 {
		return 0
	}
	return Size - 1
}

// startRow returns the row index a side's pawn starts on.
func startRow(s Side) int {
	if s == P1 {
		return Size - 1
	}
	return 0
}

// Orientation is the axis a wall spans.
type Orientation int

const (
	Horizontal Orientation = iota
	Vertical
)

// Wall is a 2-cell-long wall segment anchored at groove intersection (X,Y),
// X,Y in [0,WallGrid). A Horizontal wall blocks the two vertical-movement
// edges between rows Y/Y+1 at columns X and X+1. A Vertical wall blocks the
// two horizontal-movement edges between columns X/X+1 at rows Y and Y+1.
type Wall struct {
	X, Y   int
	Orient Orientation
}

// Board holds both pawns' positions, each side's remaining wall count, and
// the wall state as two WallGrid x WallGrid bool grids.
type Board struct {
	Pawns     [2]image.Point
	WallsLeft [2]int
	WallH     [WallGrid][WallGrid]bool
	WallV     [WallGrid][WallGrid]bool
}

// StartingWalls is the number of walls each side holds at the start.
const StartingWalls = 10

// NewBoard returns a board in the standard starting position: P1 centered on
// the bottom row, P2 centered on the top row, each holding StartingWalls
// walls and no walls yet placed.
func NewBoard() Board {
	var b Board
	b.Pawns[P1] = image.Pt(Size/2, startRow(P1))
	b.Pawns[P2] = image.Pt(Size/2, startRow(P2))
	b.WallsLeft[P1] = StartingWalls
	b.WallsLeft[P2] = StartingWalls
	return b
}

func inBounds(p image.Point) bool {
	return p.X >= 0 && p.X < Size && p.Y >= 0 && p.Y < Size
}

// place sets the given wall's bits. It does not validate legality — callers
// must check CanPlaceWall first.
func (b *Board) place(w Wall) {
	switch w.Orient {
	case Horizontal:
		b.WallH[w.Y][w.X] = true
	case Vertical:
		b.WallV[w.Y][w.X] = true
	}
}

// blockedVerticalEdge reports whether the edge between cell (x,y) and cell
// (x+1,y) is blocked by a vertical wall. x must be in [0,WallGrid).
func (b *Board) blockedVerticalEdge(x, y int) bool {
	if x < 0 || x >= WallGrid {
		return false
	}
	if y >= 0 && y < WallGrid && b.WallV[y][x] {
		return true
	}
	if y-1 >= 0 && y-1 < WallGrid && b.WallV[y-1][x] {
		return true
	}
	return false
}

// blockedHorizontalEdge reports whether the edge between cell (x,y) and cell
// (x,y+1) is blocked by a horizontal wall. y must be in [0,WallGrid).
func (b *Board) blockedHorizontalEdge(x, y int) bool {
	if y < 0 || y >= WallGrid {
		return false
	}
	if x >= 0 && x < WallGrid && b.WallH[y][x] {
		return true
	}
	if x-1 >= 0 && x-1 < WallGrid && b.WallH[y][x-1] {
		return true
	}
	return false
}

// wallBetween reports whether a wall blocks direct movement between two
// orthogonally-adjacent cells a and c. Non-adjacent points are treated as
// blocked (defensive: callers should only pass adjacent cells).
func (b *Board) wallBetween(a, c image.Point) bool {
	switch {
	case c.X == a.X+1 && c.Y == a.Y:
		return b.blockedVerticalEdge(a.X, a.Y)
	case c.X == a.X-1 && c.Y == a.Y:
		return b.blockedVerticalEdge(c.X, c.Y)
	case c.Y == a.Y+1 && c.X == a.X:
		return b.blockedHorizontalEdge(a.X, a.Y)
	case c.Y == a.Y-1 && c.X == a.X:
		return b.blockedHorizontalEdge(c.X, c.Y)
	default:
		return true
	}
}

// dirs4 are the four orthogonal step directions.
var dirs4 = [4]image.Point{{X: 0, Y: -1}, {X: 0, Y: 1}, {X: -1, Y: 0}, {X: 1, Y: 0}}

// perpendiculars returns the two directions perpendicular to d, used for the
// diagonal jump exception.
func perpendiculars(d image.Point) [2]image.Point {
	if d.X == 0 {
		return [2]image.Point{{X: -1, Y: 0}, {X: 1, Y: 0}}
	}
	return [2]image.Point{{X: 0, Y: -1}, {X: 0, Y: 1}}
}
