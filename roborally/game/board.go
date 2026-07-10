package game

import "image"

// Floor kinds. A tile is exactly one floor kind; belts/gears live on Floor
// tiles only (the generator never puts a belt on a pit/repair/checkpoint).
const (
	FloorPlain  uint8 = iota
	FloorPit          // falling in kills the robot
	FloorRepair       // heals 1 damage and updates the archive
)

// Tile is a single board square. The zero value is a plain floor with no walls
// or elements. Belt == DirNone means "no belt"; Laser == DirNone means "no
// laser emitter"; Gear == GearNone; Checkpoint/StartDock == 0 means none.
type Tile struct {
	Kind        uint8
	Belt        Dir  // conveyor direction, or DirNone
	BeltExpress bool // express belts carry an extra step per register
	Gear        Gear
	Walls       uint8 // bitmask of Dir.wallBit() edges that have a wall
	Laser       Dir   // wall-laser fire direction from this tile, or DirNone
	LaserCount  uint8 // barrels: damage dealt per hit
	Checkpoint  uint8 // 0 = none, else the ordinal (1..N)
	StartDock   uint8 // 0 = none, else the dock index
	Antenna     bool  // the single priority antenna
}

// Gear is a rotating floor element.
type Gear uint8

const (
	GearNone Gear = iota
	GearCW
	GearCCW
)

// HasWall reports whether this tile has a wall on the given edge.
func (t Tile) HasWall(d Dir) bool { return t.Walls&d.wallBit() != 0 }

// Board is a rectangular factory floor.
type Board struct {
	W, H    int
	Tiles   []Tile
	NCheck  uint8
	Antenna image.Point
	Docks   []dock // start positions, index 0 = dock 1
}

type dock struct {
	Pos    image.Point
	Facing Dir
}

func newBoard(w, h int) *Board {
	b := &Board{W: w, H: h, Tiles: make([]Tile, w*h)}
	for i := range b.Tiles {
		b.Tiles[i].Belt = DirNone
		b.Tiles[i].Laser = DirNone
	}
	return b
}

// In reports whether (x,y) is on the board.
func (b *Board) In(p image.Point) bool {
	return p.X >= 0 && p.X < b.W && p.Y >= 0 && p.Y < b.H
}

// At returns a pointer to the tile at (x,y); callers must ensure it is In.
func (b *Board) At(p image.Point) *Tile { return &b.Tiles[p.Y*b.W+p.X] }

// wallBetween reports whether movement from a to the adjacent tile b (in
// direction d) is blocked by a wall on either side of the shared edge.
func (bd *Board) wallBetween(a image.Point, d Dir) bool {
	if bd.In(a) && bd.At(a).HasWall(d) {
		return true
	}
	nb := a.Add(d.Step())
	if bd.In(nb) && bd.At(nb).HasWall(d.Opposite()) {
		return true
	}
	return false
}

// CheckpointPos returns the tile bearing checkpoint ordinal ord, and ok=false
// if there is none.
func (b *Board) CheckpointPos(ord uint8) (image.Point, bool) {
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			if b.Tiles[y*b.W+x].Checkpoint == ord {
				return image.Pt(x, y), true
			}
		}
	}
	return image.Point{}, false
}

// lethal reports whether landing on p means death (off-board or a pit).
func (b *Board) lethal(p image.Point) bool {
	if !b.In(p) {
		return true
	}
	return b.At(p).Kind == FloorPit
}
