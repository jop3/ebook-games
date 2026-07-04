// Package game implements 2048's board logic with no dependency on the inkview
// SDK, so it unit-tests cgo-free. A 4x4 grid of tile values (0 = empty); each
// move slides every tile as far as it can in one direction, merging equal
// tiles that collide (each tile merges at most once per move), then a new
// tile spawns on a random empty cell.
package game

import "math/rand"

// Size is the edge length of the board.
const Size = 4

// Board holds the grid in row-major order (index = y*Size + x); 0 = empty.
type Board [Size * Size]int

// Dir is a swipe direction.
type Dir int

const (
	Left Dir = iota
	Right
	Up
	Down
)

func (b *Board) at(x, y int) int { return b[y*Size+x] }
func (b *Board) set(x, y, v int) { b[y*Size+x] = v }

// slideRowLeft compresses+merges-once+compresses a single row of Size values,
// leftward, returning the new row and the score gained from merges.
func slideRowLeft(row [Size]int) (out [Size]int, gained int, moved bool) {
	// Compress: gather non-zero values.
	var vals []int
	for _, v := range row {
		if v != 0 {
			vals = append(vals, v)
		}
	}
	// Merge once per tile, left to right (leading edge first).
	var merged []int
	for i := 0; i < len(vals); i++ {
		if i+1 < len(vals) && vals[i] == vals[i+1] {
			sum := vals[i] * 2
			merged = append(merged, sum)
			gained += sum
			i++ // skip the tile just merged into this one
		} else {
			merged = append(merged, vals[i])
		}
	}
	for i := 0; i < Size; i++ {
		if i < len(merged) {
			out[i] = merged[i]
		} else {
			out[i] = 0
		}
	}
	moved = out != row
	return out, gained, moved
}

// rotate returns a copy of b rotated 90 degrees clockwise.
func rotateCW(b Board) Board {
	var out Board
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			// (x,y) -> (Size-1-y, x)
			out.set(Size-1-y, x, b.at(x, y))
		}
	}
	return out
}

// Slide applies dir to b, returning the resulting board, the score gained,
// and whether the board actually changed. All four directions are derived
// from slideRowLeft by rotating the board so the merge logic lives in one
// place: Left needs no rotation; the others rotate so the target direction
// becomes "left", slide, then rotate back.
func Slide(b Board, dir Dir) (out Board, gained int, moved bool) {
	// Number of clockwise quarter-turns that make `dir` face left, and to undo.
	// rotateCW maps (x,y) -> (Size-1-y, x); working through the composition,
	// one CW turn turns "left" into what was "down" in the original frame, two
	// turns into "right", three into "up" — verified by TestSlideDirections.
	var turns int
	switch dir {
	case Left:
		turns = 0
	case Down:
		turns = 1
	case Right:
		turns = 2
	case Up:
		turns = 3
	}
	work := b
	for i := 0; i < turns; i++ {
		work = rotateCW(work)
	}
	for y := 0; y < Size; y++ {
		var row [Size]int
		for x := 0; x < Size; x++ {
			row[x] = work.at(x, y)
		}
		newRow, g, m := slideRowLeft(row)
		for x := 0; x < Size; x++ {
			work.set(x, y, newRow[x])
		}
		gained += g
		moved = moved || m
	}
	// Undo the rotation: 4-turns total brings us back to original orientation.
	for i := 0; i < (4-turns)%4; i++ {
		work = rotateCW(work)
	}
	return work, gained, moved
}

// Spawn places a new tile (2 with ~90% probability, else 4) on a uniformly
// random empty cell. If the board is full, it is returned unchanged.
func Spawn(b Board, rng *rand.Rand) Board {
	var empties []int
	for i, v := range b {
		if v == 0 {
			empties = append(empties, i)
		}
	}
	if len(empties) == 0 {
		return b
	}
	idx := empties[rng.Intn(len(empties))]
	v := 2
	if rng.Intn(10) == 0 {
		v = 4
	}
	b[idx] = v
	return b
}

// CanMove reports whether any move (in any direction) would change the
// board: true if there's an empty cell, or two equal orthogonal neighbours.
func CanMove(b Board) bool {
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			v := b.at(x, y)
			if v == 0 {
				return true
			}
			if x+1 < Size && b.at(x+1, y) == v {
				return true
			}
			if y+1 < Size && b.at(x, y+1) == v {
				return true
			}
		}
	}
	return false
}

// Won reports whether the board contains a tile with at least `target` value
// (default target is 2048; the menu may offer other targets).
func Won(b Board, target int) bool {
	for _, v := range b {
		if v >= target {
			return true
		}
	}
	return false
}

// At returns the tile value at (x,y).
func (b *Board) At(x, y int) int { return b.at(x, y) }
