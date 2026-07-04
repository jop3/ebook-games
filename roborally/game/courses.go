package game

import "image"

// fixedSeeds are curated constant seeds so each named course plays the same
// every time. They still run through the verified generator, so they are always
// solvable.
var fixedSeeds = [3]int64{101, 202, 303}

// FixedCourse returns the stable named course for a difficulty (same layout each
// time). "Slumpbana" callers should use GenerateCourse with a varying seed.
func FixedCourse(diff CourseDiff) *Board {
	return GenerateCourse(diff, fixedSeeds[diff])
}

// DecodeBoard builds a board from a rune map, for tests and fixtures:
//
//	'.' ' '  plain floor      'O' pit        '+' repair
//	'^>v<'   single belts     'A' antenna    '1'..'9' checkpoint ordinal
//	'a'..'h' start dock (facing North by default; set Facing after if needed)
//
// Walls, lasers and express belts are set programmatically by the caller.
func DecodeBoard(rows []string) *Board {
	w := 0
	for _, r := range rows {
		if len(r) > w {
			w = len(r)
		}
	}
	h := len(rows)
	b := newBoard(w, h)
	var maxCheck uint8
	for y, row := range rows {
		for x, ch := range row {
			t := b.At(image.Pt(x, y))
			switch {
			case ch == 'O':
				t.Kind = FloorPit
			case ch == '+':
				t.Kind = FloorRepair
			case ch == '^':
				t.Belt = N
			case ch == '>':
				t.Belt = E
			case ch == 'v':
				t.Belt = S
			case ch == '<':
				t.Belt = W
			case ch == 'A':
				t.Antenna = true
				b.Antenna = image.Pt(x, y)
			case ch >= '1' && ch <= '9':
				t.Checkpoint = uint8(ch - '0')
				if t.Checkpoint > maxCheck {
					maxCheck = t.Checkpoint
				}
			case ch >= 'a' && ch <= 'h':
				t.StartDock = uint8(ch-'a') + 1
				b.Docks = append(b.Docks, dock{Pos: image.Pt(x, y), Facing: N})
			}
		}
	}
	b.NCheck = maxCheck
	return b
}
