// Package game implements the pure rules of Chomp with no dependency on the
// inkview SDK, so it can be unit-tested cgo-free.
//
// Chomp is played on a rectangular grid of cells — a chocolate bar — whose
// top-left cell (row 0, column 0) is poisoned. On a turn, a player picks any
// remaining cell (r,c); that cell AND every remaining cell with row >= r AND
// column >= c is removed in one go (as if the bar were snapped along the
// jagged line running down-and-right from the chosen cell). Whoever is
// forced to eat the poisoned cell loses.
//
// Because every move only ever removes a "lower-right" rectangle from each
// row, the set of remaining cells is always a "staircase": one integer per
// row (how many cells remain in that row, columns 0..n-1), monotonically
// non-increasing from row 0 downward. That staircase is the entire state —
// State below is just []int — which keeps the whole engine (and its AI, see
// ai.go) tiny and exhaustively searchable.
package game

// State is the staircase representation of a Chomp board: State[r] is the
// number of remaining cells in row r (columns 0..State[r]-1 are present,
// State[r]..end are already eaten). State is always monotonically
// non-increasing (State[r] >= State[r+1]) — every Apply call preserves this
// invariant, and every constructor establishes it.
type State []int

// NewState returns a full rows x cols rectangle (the starting position).
func NewState(rows, cols int) State {
	s := make(State, rows)
	for i := range s {
		s[i] = cols
	}
	return s
}

// Clone returns an independent copy of s.
func (s State) Clone() State {
	c := make(State, len(s))
	copy(c, s)
	return c
}

// NumRows returns the number of rows in the original rectangle (a row whose
// count has reached 0 is still "in" the state — it just has no cells left).
func (s State) NumRows() int { return len(s) }

// RowLen returns the number of remaining cells in row r (columns 0..n-1), or
// 0 if r is out of range.
func (s State) RowLen(r int) int {
	if r < 0 || r >= len(s) {
		return 0
	}
	return s[r]
}

// Has reports whether cell (r,c) is still present on the board.
func (s State) Has(r, c int) bool {
	return r >= 0 && r < len(s) && c >= 0 && c < s[r]
}

// Empty reports whether no cells remain at all.
func (s State) Empty() bool {
	for _, n := range s {
		if n > 0 {
			return false
		}
	}
	return true
}

// Total returns the number of remaining cells across every row.
func (s State) Total() int {
	n := 0
	for _, v := range s {
		n += v
	}
	return n
}

// Move picks cell (Row, Col) to eat. Applying it removes that cell and every
// remaining cell with row >= Row AND col >= Col.
type Move struct {
	Row, Col int
}

// IsPoison reports whether m targets the poisoned top-left cell (0,0) —
// taking it ends the game immediately in a loss for whoever takes it.
func (m Move) IsPoison() bool { return m.Row == 0 && m.Col == 0 }

// IsLegal reports whether m targets a cell still present on s.
func (s State) IsLegal(m Move) bool { return s.Has(m.Row, m.Col) }

// LegalMoves returns every remaining cell as a possible move, row-major.
func (s State) LegalMoves() []Move {
	var out []Move
	for r, n := range s {
		for c := 0; c < n; c++ {
			out = append(out, Move{Row: r, Col: c})
		}
	}
	return out
}

// Apply returns the state after eating m: m itself, and every remaining cell
// with row >= m.Row AND col >= m.Col, is removed. Rows above m.Row are
// untouched; from m.Row downward, each row's remaining length is clamped to
// at most m.Col (columns 0..m.Col-1 survive, m.Col.. is gone) — which is
// exactly what keeps the result a valid non-increasing staircase, since a
// non-increasing sequence clamped everywhere from some point on to the same
// ceiling stays non-increasing. Illegal moves are a no-op (callers should
// check IsLegal first); this never panics.
func (s State) Apply(m Move) State {
	if !s.IsLegal(m) {
		return s
	}
	out := s.Clone()
	for r := m.Row; r < len(out); r++ {
		if out[r] > m.Col {
			out[r] = m.Col
		}
	}
	return out
}
