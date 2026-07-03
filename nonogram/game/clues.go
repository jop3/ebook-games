// Package game implements Nonogram (picross) logic with no dependency on the
// inkview SDK, so it is unit-tested cgo-free.
//
// A puzzle is an W x H grid of cells that are either filled or blank. Each row
// and column carries the run-length clues of its filled runs. The player marks
// cells to reproduce the hidden picture. This file holds the clue math and the
// line solver used both to verify puzzles are uniquely solvable and (optionally)
// to help the player.
package game

// Clue is the run-length description of a single line (row or column): the
// lengths of consecutive filled blocks, in order. An empty line has no runs; we
// represent that as a single {0} for display convenience via Display().
type Clue []int

// LineClue computes the run-length clue for a line of booleans (true = filled).
func LineClue(line []bool) Clue {
	var c Clue
	run := 0
	for _, v := range line {
		if v {
			run++
		} else if run > 0 {
			c = append(c, run)
			run = 0
		}
	}
	if run > 0 {
		c = append(c, run)
	}
	return c
}

// Display returns the clue for rendering: an empty clue becomes {0}.
func (c Clue) Display() []int {
	if len(c) == 0 {
		return []int{0}
	}
	return c
}

// sum returns the total filled cells the clue requires.
func (c Clue) sum() int {
	t := 0
	for _, v := range c {
		t += v
	}
	return t
}

// minWidth is the least number of cells a clue can occupy (runs + one gap
// between each pair of runs).
func (c Clue) minWidth() int {
	if len(c) == 0 {
		return 0
	}
	return c.sum() + len(c) - 1
}

// lineFits reports whether the given full line of booleans matches this clue.
func (c Clue) matches(line []bool) bool {
	got := LineClue(line)
	if len(got) != len(c) {
		return false
	}
	for i := range c {
		if got[i] != c[i] {
			return false
		}
	}
	return true
}
