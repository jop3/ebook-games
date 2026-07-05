package game

import "image"

// precomputedLines lists every straight line on the board (rows, columns,
// and diagonals of both families) that is at least 5 cells long — the only
// lines a 5-in-a-row can occur on. Computed once; the board layout of lines
// never changes shape (only which glyph sits where), so this needs no
// per-game recomputation.
var precomputedLines = allLines()

func diagLine(start, dir image.Point) []image.Point {
	var out []image.Point
	x, y := start.X, start.Y
	for inBounds(x, y) {
		out = append(out, image.Pt(x, y))
		x += dir.X
		y += dir.Y
	}
	return out
}

func allLines() [][]image.Point {
	var lines [][]image.Point
	for y := 0; y < Size; y++ {
		row := make([]image.Point, Size)
		for x := 0; x < Size; x++ {
			row[x] = image.Pt(x, y)
		}
		lines = append(lines, row)
	}
	for x := 0; x < Size; x++ {
		col := make([]image.Point, Size)
		for y := 0; y < Size; y++ {
			col[y] = image.Pt(x, y)
		}
		lines = append(lines, col)
	}
	// "╲" (D2) diagonals: starting cells along the top row, then down the
	// left column (top-left cell already covered by the top-row loop).
	for x := 0; x < Size; x++ {
		if line := diagLine(image.Pt(x, 0), image.Pt(1, 1)); len(line) >= 5 {
			lines = append(lines, line)
		}
	}
	for y := 1; y < Size; y++ {
		if line := diagLine(image.Pt(0, y), image.Pt(1, 1)); len(line) >= 5 {
			lines = append(lines, line)
		}
	}
	// "╱" (D1) diagonals: starting cells along the top row, then down the
	// right column.
	for x := 0; x < Size; x++ {
		if line := diagLine(image.Pt(x, 0), image.Pt(-1, 1)); len(line) >= 5 {
			lines = append(lines, line)
		}
	}
	for y := 1; y < Size; y++ {
		if line := diagLine(image.Pt(Size-1, y), image.Pt(-1, 1)); len(line) >= 5 {
			lines = append(lines, line)
		}
	}
	return lines
}

// Five reports whether player has an unbroken run of 5+ rings anywhere on
// the board, along any row, column, or diagonal.
func Five(b *Board, player Cell) bool {
	for _, line := range precomputedLines {
		run := 0
		for _, p := range line {
			if b.At(p.X, p.Y) == player {
				run++
				if run >= 5 {
					return true
				}
			} else {
				run = 0
			}
		}
	}
	return false
}

var orthoDirs = [4]image.Point{{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}}

// LargestGroup returns the size of player's largest orthogonally-connected
// (rook-adjacent, NOT diagonal) group of rings — the board-full tiebreak.
func LargestGroup(b *Board, player Cell) int {
	var visited [Size][Size]bool
	best := 0
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.At(x, y) != player || visited[y][x] {
				continue
			}
			if n := floodFill(b, player, x, y, &visited); n > best {
				best = n
			}
		}
	}
	return best
}

func floodFill(b *Board, player Cell, sx, sy int, visited *[Size][Size]bool) int {
	stack := []image.Point{{X: sx, Y: sy}}
	visited[sy][sx] = true
	count := 0
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		for _, d := range orthoDirs {
			nx, ny := p.X+d.X, p.Y+d.Y
			if inBounds(nx, ny) && !visited[ny][nx] && b.At(nx, ny) == player {
				visited[ny][nx] = true
				stack = append(stack, image.Pt(nx, ny))
			}
		}
	}
	return count
}

func boardFull(b *Board) bool {
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if b.Ring[y][x] == Empty {
				return false
			}
		}
	}
	return true
}

// tiebreakWinner decides a full board with no five-in-a-row: the player
// with the larger orthogonally-connected group wins. Equal group sizes are
// a draw — the smallest reasonable choice absent a stated tiebreak in the
// source material (see rulesParagraphs, which states this explicitly to the
// player).
func tiebreakWinner(b *Board) Cell {
	bg, wg := LargestGroup(b, Black), LargestGroup(b, White)
	switch {
	case bg > wg:
		return Black
	case wg > bg:
		return White
	default:
		return Empty
	}
}

// Winner returns the winning color for a FINISHED board: outright if either
// player already has 5 in a row, otherwise via the largest-group tiebreak
// (meaningful once the board is full; Empty otherwise/for a draw).
func Winner(b Board) Cell {
	if Five(&b, Black) {
		return Black
	}
	if Five(&b, White) {
		return White
	}
	return tiebreakWinner(&b)
}
