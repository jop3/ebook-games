// Package game implements the rules engine for the "X in a row" family of
// games (tic-tac-toe, Connect Four, Gomoku, three men's morris, ...).
// It is deliberately free of any UI or SDK dependency so it can be unit
// tested with the standard Go toolchain.
package game

// Player identifies whose stone occupies a cell. Up to four players are
// supported (hot-seat); PlayerNone marks an empty cell.
type Player int

const (
	PlayerNone Player = iota
	Player1
	Player2
	Player3
	Player4
)

// MaxPlayers is the highest supported player count.
const MaxPlayers = 4

// Other returns the opposing player in a two-player game. It is a convenience
// for the AI and two-player flows only; multi-player turn rotation is handled
// by GameState.NumPlayers and nextTurn. Other(PlayerNone) is PlayerNone.
func (p Player) Other() Player {
	switch p {
	case Player1:
		return Player2
	case Player2:
		return Player1
	default:
		return PlayerNone
	}
}

// Phase is the global state of a match.
type Phase int

const (
	PhasePlacing Phase = iota
	PhaseMoving
	PhaseGameOver
)

// Board holds the grid and the rule parameters that drive it. It is a value
// type: Apply returns a fresh copy so the AI can explore moves without
// mutating live state.
type Board struct {
	Width, Height int
	Cells         []Player // length Width*Height, row-major: Cells[y*Width+x]
	Blocked       []bool   // same length, true = unplayable cell
	DropMode      bool
	WinLength     int
	PieceLimit    int                 // 0 = unlimited
	Placed        [MaxPlayers + 1]int // Placed[p] = stones placed by player p (1..MaxPlayers)
}

// Move is a single action. From == -1 for a placement/drop; From >= 0 for a
// relocation during the moving phase.
type Move struct {
	From, To int
}

// directions for line scanning: right, down, down-right, up-right. Scanning
// both ways along each covers all four orientations.
var directions = [4][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}}

// NewBoard builds an empty board from a preset.
func NewBoard(p Preset) Board {
	n := p.Width * p.Height
	b := Board{
		Width:      p.Width,
		Height:     p.Height,
		Cells:      make([]Player, n),
		Blocked:    make([]bool, n),
		DropMode:   p.DropMode,
		WinLength:  p.WinLength,
		PieceLimit: p.PieceLimit,
	}
	if p.Blocked != nil {
		blocked := p.Blocked(p.Width, p.Height)
		// Defensive: only copy if the preset returned a correctly sized slice.
		if len(blocked) == n {
			copy(b.Blocked, blocked)
		}
	}
	return b
}

// Idx maps (x, y) to a flat index. Caller must ensure bounds.
func (b *Board) Idx(x, y int) int { return y*b.Width + x }

// XY maps a flat index back to coordinates.
func (b *Board) XY(idx int) (x, y int) { return idx % b.Width, idx / b.Width }

// InBounds reports whether (x, y) lies on the board.
func (b *Board) InBounds(x, y int) bool {
	return x >= 0 && x < b.Width && y >= 0 && y < b.Height
}

// Clone returns a deep copy of the board.
func (b *Board) Clone() Board {
	nb := *b
	nb.Cells = make([]Player, len(b.Cells))
	copy(nb.Cells, b.Cells)
	nb.Blocked = make([]bool, len(b.Blocked))
	copy(nb.Blocked, b.Blocked)
	return nb
}

// dropRow returns the lowest empty, unblocked row in a column, or -1 if the
// column is full/unreachable. In drop mode a stone falls until it rests on
// the floor, another stone, or a blocked cell.
func (b *Board) dropRow(col int) int {
	if col < 0 || col >= b.Width {
		return -1
	}
	for y := b.Height - 1; y >= 0; y-- {
		i := b.Idx(col, y)
		if b.Blocked[i] {
			// A blocked cell acts as floor: nothing can rest below it that
			// we haven't already rejected, and nothing can pass through it.
			return -1
		}
		if b.Cells[i] == PlayerNone {
			return y
		}
	}
	return -1
}

// DropTarget resolves the cell a stone would occupy if dropped in the column
// containing x. Returns (idx, true) or (-1, false).
func (b *Board) DropTarget(x int) (int, bool) {
	row := b.dropRow(x)
	if row < 0 {
		return -1, false
	}
	return b.Idx(x, row), true
}

// ValidMoves enumerates every legal move for turn in the given phase.
func (b *Board) ValidMoves(turn Player, phase Phase) []Move {
	switch phase {
	case PhasePlacing:
		return b.placementMoves()
	case PhaseMoving:
		return b.relocationMoves(turn)
	default:
		return nil
	}
}

func (b *Board) placementMoves() []Move {
	var moves []Move
	if b.DropMode {
		for x := 0; x < b.Width; x++ {
			if idx, ok := b.DropTarget(x); ok {
				moves = append(moves, Move{From: -1, To: idx})
			}
		}
		return moves
	}
	for i := range b.Cells {
		if b.Cells[i] == PlayerNone && !b.Blocked[i] {
			moves = append(moves, Move{From: -1, To: i})
		}
	}
	return moves
}

// neighbourOffsets are the 8 adjacent directions used in the moving phase.
var neighbourOffsets = [8][2]int{
	{-1, -1}, {0, -1}, {1, -1},
	{-1, 0}, {1, 0},
	{-1, 1}, {0, 1}, {1, 1},
}

func (b *Board) relocationMoves(turn Player) []Move {
	var moves []Move
	for i := range b.Cells {
		if b.Cells[i] != turn {
			continue
		}
		x, y := b.XY(i)
		for _, d := range neighbourOffsets {
			nx, ny := x+d[0], y+d[1]
			if !b.InBounds(nx, ny) {
				continue
			}
			j := b.Idx(nx, ny)
			if b.Cells[j] == PlayerNone && !b.Blocked[j] {
				moves = append(moves, Move{From: i, To: j})
			}
		}
	}
	return moves
}

// Apply returns a new board with the move performed by turn. The caller is
// responsible for passing a legal move (use ValidMoves); Apply does minimal
// validation and silently returns a clone on an obviously bad index.
func (b *Board) Apply(m Move, turn Player) Board {
	nb := b.Clone()
	if m.To < 0 || m.To >= len(nb.Cells) {
		return nb
	}
	if m.From >= 0 && m.From < len(nb.Cells) {
		nb.Cells[m.From] = PlayerNone
	} else {
		// Placement: count it against the player's piece budget.
		if turn >= Player1 && turn <= Player4 {
			nb.Placed[turn]++
		}
	}
	nb.Cells[m.To] = turn
	return nb
}

// CheckWin returns the winning player if the stone at lastIdx completes a
// line of WinLength, else PlayerNone. Only lines through lastIdx are scanned,
// which is all that can change after a single move.
func (b *Board) CheckWin(lastIdx int) Player {
	if lastIdx < 0 || lastIdx >= len(b.Cells) {
		return PlayerNone
	}
	who := b.Cells[lastIdx]
	if who == PlayerNone {
		return PlayerNone
	}
	x0, y0 := b.XY(lastIdx)
	for _, d := range directions {
		count := 1
		// walk +d
		for x, y := x0+d[0], y0+d[1]; b.InBounds(x, y) && b.Cells[b.Idx(x, y)] == who; x, y = x+d[0], y+d[1] {
			count++
		}
		// walk -d
		for x, y := x0-d[0], y0-d[1]; b.InBounds(x, y) && b.Cells[b.Idx(x, y)] == who; x, y = x-d[0], y-d[1] {
			count++
		}
		if count >= b.WinLength {
			return who
		}
	}
	return PlayerNone
}

// IsFull reports whether no empty, unblocked cell remains.
func (b *Board) IsFull() bool {
	for i := range b.Cells {
		if b.Cells[i] == PlayerNone && !b.Blocked[i] {
			return false
		}
	}
	return true
}
