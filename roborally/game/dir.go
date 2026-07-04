package game

import "image"

// Dir is one of the four cardinal facings. N points toward the top of the
// board (decreasing y), E toward increasing x, and so on clockwise.
type Dir uint8

const (
	N Dir = iota
	E
	S
	W
	DirNone Dir = 255 // sentinel: "no direction" (e.g. a tile with no belt)
)

// Right turns 90° clockwise.
func (d Dir) Right() Dir { return (d + 1) & 3 }

// Left turns 90° counter-clockwise.
func (d Dir) Left() Dir { return (d + 3) & 3 }

// Opposite is a 180° turn.
func (d Dir) Opposite() Dir { return (d + 2) & 3 }

// Step returns the unit delta for a single step in this direction.
func (d Dir) Step() image.Point {
	switch d {
	case N:
		return image.Pt(0, -1)
	case E:
		return image.Pt(1, 0)
	case S:
		return image.Pt(0, 1)
	case W:
		return image.Pt(-1, 0)
	}
	return image.Pt(0, 0)
}

// String gives a short label used in logs.
func (d Dir) String() string {
	switch d {
	case N:
		return "N"
	case E:
		return "Ö"
	case S:
		return "S"
	case W:
		return "V"
	}
	return "-"
}

// wallBit is the edge bitmask value for a direction (used by Tile.Walls).
func (d Dir) wallBit() uint8 {
	if d > W {
		return 0
	}
	return 1 << d
}
