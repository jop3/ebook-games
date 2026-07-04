package game

// Board is a single player's board: the 5 pattern lines (row i holds up to
// i+1 tiles, all one color), the 5x5 wall (fixed color layout per
// wallColOf), the floor line (penalty tiles + possibly the start marker),
// and the running score.
type Board struct {
	Lines [WallSize][]Color // Lines[i] holds 0..i+1 tiles, all the same color
	Wall  [WallSize][WallSize]bool

	Floor     []Color // overflow/discard tiles sitting on the floor line this round
	HasMarker bool    // the start-player marker is on this floor line this round
	Score     int
}

// FloorSlots is the number of penalty slots on the floor line.
const FloorSlots = 7

// floorTrack is the (cumulative, per-slot) penalty for the 7 floor slots:
// -1, -1, -2, -2, -2, -3, -3.
var floorTrack = [FloorSlots]int{1, 1, 2, 2, 2, 3, 3}

// floorPenalty returns the total penalty (a positive number of points to
// subtract) for n items sitting on the floor line (tiles + the start marker,
// if present). Only the first FloorSlots items are penalized: real Azul
// simply discards anything beyond the 7th slot with no further penalty, and
// this implementation never lets more than FloorSlots items accumulate on a
// board's Floor+marker in the first place (see Board.floorRoom).
func floorPenalty(n int) int {
	if n > FloorSlots {
		n = FloorSlots
	}
	total := 0
	for i := 0; i < n; i++ {
		total += floorTrack[i]
	}
	return total
}

// floorCount is how many items (tiles + marker) currently sit on the floor
// line.
func (b *Board) floorCount() int {
	n := len(b.Floor)
	if b.HasMarker {
		n++
	}
	return n
}

// floorRoom is how many more tiles can land on the floor line before it is
// full (the marker, if present, already occupies one slot).
func (b *Board) floorRoom() int {
	room := FloorSlots - b.floorCount()
	if room < 0 {
		room = 0
	}
	return room
}

// lineAccepts reports whether pattern line i can currently accept at least
// one tile of color c — the three-part legality rule: (a) the line isn't
// already full, (b) it is empty or already holds only color c, and (c) the
// wall cell that color+row would tile into isn't already filled.
func (b *Board) lineAccepts(i int, c Color) bool {
	if i < 0 || i >= WallSize {
		return false
	}
	if len(b.Lines[i]) >= i+1 {
		return false // full
	}
	if len(b.Lines[i]) > 0 && b.Lines[i][0] != c {
		return false // holds a different color already
	}
	col := wallColOf(i, c)
	if b.Wall[i][col] {
		return false // this color's wall cell in this row is already filled
	}
	return true
}

// lineColor returns the color currently committed to line i and true, or
// (0, false) if the line is empty.
func (b *Board) lineColor(i int) (Color, bool) {
	if len(b.Lines[i]) == 0 {
		return 0, false
	}
	return b.Lines[i][0], true
}

// Clone returns a deep copy of b (independent slices), so callers such as
// the AI search can mutate a copy without affecting the original.
func (b Board) Clone() Board {
	nb := b
	for i := range nb.Lines {
		nb.Lines[i] = append([]Color(nil), b.Lines[i]...)
	}
	nb.Floor = append([]Color(nil), b.Floor...)
	return nb
}

// fullRow reports whether wall row r is completely filled.
func fullRow(wall *[WallSize][WallSize]bool, r int) bool {
	for c := 0; c < WallSize; c++ {
		if !wall[r][c] {
			return false
		}
	}
	return true
}

// fullCol reports whether wall column c is completely filled.
func fullCol(wall *[WallSize][WallSize]bool, c int) bool {
	for r := 0; r < WallSize; r++ {
		if !wall[r][c] {
			return false
		}
	}
	return true
}

// colorComplete reports whether every wall cell belonging to color c (one
// per row, per wallColOf) is filled.
func colorComplete(wall *[WallSize][WallSize]bool, c Color) bool {
	for r := 0; r < WallSize; r++ {
		if !wall[r][wallColOf(r, c)] {
			return false
		}
	}
	return true
}

// gameOver reports whether wall has completed at least one full horizontal
// row — the trigger for ending the game (checked once wall-tiling for a
// round has finished).
func gameOver(wall *[WallSize][WallSize]bool) bool {
	for r := 0; r < WallSize; r++ {
		if fullRow(wall, r) {
			return true
		}
	}
	return false
}

// completeRows counts wall's complete rows — used both for the +2/row end
// bonus and as the tiebreak criterion when two final scores are equal.
func completeRows(wall *[WallSize][WallSize]bool) int {
	n := 0
	for r := 0; r < WallSize; r++ {
		if fullRow(wall, r) {
			n++
		}
	}
	return n
}

// scorePlacement scores the tile that now sits at wall[r][c] (the caller
// must have already set wall[r][c] = true). An isolated tile (no orthogonal
// neighbor in either axis) scores 1. Otherwise it scores the length of its
// connected horizontal run plus the length of its connected vertical run,
// each run including the new tile itself; an axis with no neighbor (run
// length 1) contributes nothing to the sum in that case.
func scorePlacement(wall *[WallSize][WallSize]bool, r, c int) int {
	hLen := 1
	for x := c - 1; x >= 0 && wall[r][x]; x-- {
		hLen++
	}
	for x := c + 1; x < WallSize && wall[r][x]; x++ {
		hLen++
	}
	vLen := 1
	for y := r - 1; y >= 0 && wall[y][c]; y-- {
		vLen++
	}
	for y := r + 1; y < WallSize && wall[y][c]; y++ {
		vLen++
	}
	if hLen == 1 && vLen == 1 {
		return 1
	}
	score := 0
	if hLen > 1 {
		score += hLen
	}
	if vLen > 1 {
		score += vLen
	}
	return score
}

// endBonusesDetailed returns the end-game bonus breakdown for wall: +2 per
// complete row, +7 per complete column, +10 per color with all 5 tiles on
// the wall, plus their sum.
func endBonusesDetailed(wall *[WallSize][WallSize]bool) (rows, cols, colors, total int) {
	for r := 0; r < WallSize; r++ {
		if fullRow(wall, r) {
			rows++
		}
	}
	for c := 0; c < WallSize; c++ {
		if fullCol(wall, c) {
			cols++
		}
	}
	for c := Color(0); c < NumColors; c++ {
		if colorComplete(wall, c) {
			colors++
		}
	}
	total = rows*2 + cols*7 + colors*10
	return rows, cols, colors, total
}

// endBonuses returns just the total from endBonusesDetailed.
func endBonuses(wall *[WallSize][WallSize]bool) int {
	_, _, _, total := endBonusesDetailed(wall)
	return total
}

// Placement records one pattern-line-to-wall move made during end-of-round
// tiling, for the UI's "per-tile points" display.
type Placement struct {
	Row, Col int
	Color    Color
	Points   int
}

// TileBoard performs end-of-round wall-tiling for a single board: every
// complete pattern line moves one tile onto its fixed wall cell (scored via
// scorePlacement) and discards the rest of that line's tiles; incomplete
// lines are left untouched for next round. Afterwards the floor-line penalty
// is applied (score clamped at 0) and the floor is cleared. Returns the
// placements made (in row order), the floor penalty charged, and every tile
// that should return to the lid (discarded pattern-line overflow + floor
// tiles) so the caller can feed them back for a future bag reshuffle.
func TileBoard(b *Board) (placements []Placement, floorPenaltyCharged int, discarded []Color) {
	for i := 0; i < WallSize; i++ {
		if len(b.Lines[i]) != i+1 {
			continue
		}
		c := b.Lines[i][0]
		col := wallColOf(i, c)
		b.Wall[i][col] = true
		pts := scorePlacement(&b.Wall, i, col)
		b.Score += pts
		placements = append(placements, Placement{Row: i, Col: col, Color: c, Points: pts})
		discarded = append(discarded, b.Lines[i][1:]...)
		b.Lines[i] = nil
	}
	pen := floorPenalty(b.floorCount())
	b.Score -= pen
	if b.Score < 0 {
		b.Score = 0
	}
	discarded = append(discarded, b.Floor...)
	b.Floor = nil
	b.HasMarker = false
	return placements, pen, discarded
}
