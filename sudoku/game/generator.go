package game

import (
	"math/rand"
)

// Difficulty controls how many clues (given numbers) remain.
type Difficulty int

const (
	Easy Difficulty = iota
	Medium
	Hard
)

// String returns a Swedish label for the difficulty.
func (d Difficulty) String() string {
	switch d {
	case Easy:
		return "Latt"
	case Medium:
		return "Medel"
	case Hard:
		return "Svar"
	default:
		return "?"
	}
}

// targetClues is the number of givens we aim to leave on the board for
// each difficulty. Fewer clues => harder. A standard minimal Sudoku has
// 17 clues; we stay comfortably above that so puzzles are humane and
// generation is fast.
func targetClues(d Difficulty) int {
	switch d {
	case Easy:
		return 42
	case Medium:
		return 34
	case Hard:
		return 28
	default:
		return 34
	}
}

// Puzzle is a generated Sudoku: the starting grid (with holes) plus its
// unique solution and the set of cells that were given (fixed).
type Puzzle struct {
	Start    Board
	Solution Board
	Given    [9][9]bool
}

// Generate builds a puzzle of the given difficulty whose solution is
// GUARANTEED unique. rng lets callers seed deterministically for tests;
// pass nil to use the global source.
func Generate(d Difficulty, rng *rand.Rand) Puzzle {
	if rng == nil {
		rng = rand.New(rand.NewSource(rand.Int63()))
	}
	solution := fullSolution(rng)

	puzzle := solution
	// Randomised removal order over all 81 positions.
	order := rng.Perm(81)
	clues := 81
	target := targetClues(d)

	for _, idx := range order {
		if clues <= target {
			break
		}
		r, c := idx/9, idx%9
		if puzzle[r][c] == 0 {
			continue
		}
		saved := puzzle[r][c]
		puzzle[r][c] = 0
		// Keep the removal only if the puzzle still has exactly one
		// solution; otherwise put the digit back.
		if puzzle.CountSolutions(2) != 1 {
			puzzle[r][c] = saved
		} else {
			clues--
		}
	}

	var given [9][9]bool
	for r := 0; r < N; r++ {
		for c := 0; c < N; c++ {
			given[r][c] = puzzle[r][c] != 0
		}
	}
	return Puzzle{Start: puzzle, Solution: solution, Given: given}
}

// fullSolution returns a random, fully-filled valid Sudoku grid.
func fullSolution(rng *rand.Rand) Board {
	var b Board
	b.fill(rng)
	return b
}

// fill solves an empty board while trying candidates in random order,
// producing a random complete grid.
func (b *Board) fill(rng *rand.Rand) bool {
	row, col, found := b.firstEmpty()
	if !found {
		return true
	}
	cands := b.candidates(row, col)
	rng.Shuffle(len(cands), func(i, j int) {
		cands[i], cands[j] = cands[j], cands[i]
	})
	for _, v := range cands {
		b[row][col] = v
		if b.fill(rng) {
			return true
		}
	}
	b[row][col] = 0
	return false
}

func (b *Board) firstEmpty() (int, int, bool) {
	for r := 0; r < N; r++ {
		for c := 0; c < N; c++ {
			if b[r][c] == 0 {
				return r, c, true
			}
		}
	}
	return 0, 0, false
}
