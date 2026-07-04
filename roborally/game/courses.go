package game

import "image"

// --- Curated (hand-authored) courses ---------------------------------------
//
// These are designed by hand via the rune map below, each with one "signature"
// mechanic, then run through the SAME verifier as generated courses (see
// TestCuratedCoursesValid). The generator lives on as the endless "Slumpbana"
// mode; the curated set is the primary, tuned experience.

type curatedCourse struct {
	tier    CourseDiff
	rows    []string
	overlay func(*Board) // walls / lasers / express belts (edge/attribute detail)
}

// Bana 1 (Lätt): a gentle climb with a helpful belt lane up the middle and a
// couple of pits off to the sides — signature: the belt is a fast lane north but
// checkpoint 1 sits just off it, so you branch off, tag it, then ride the belt.
var bana1 = curatedCourse{
	tier: DiffEasy,
	rows: []string{
		"....2...",
		"....^...",
		"..O.^.O.",
		"....A...",
		"...1^...",
		"....^...",
		"........",
		"a.b.c.d.",
	},
}

// Bana 2 (Mellan): signature "laser gauntlet" — a beam sweeps the middle row you
// must cross between checkpoints 1 and 2 (time it or eat the hit), plus an
// express belt shortcut and a gear near checkpoint 2.
var bana2 = curatedCourse{
	tier: DiffMedium,
	rows: []string{
		"....3.....",
		"..........",
		"..O....O..",
		"...>>2....",
		".....R..A.",
		"..........",
		"..O....O..",
		"...1......",
		"..........",
		".a.b.c.d..",
	},
	overlay: func(b *Board) {
		b.addLaser(0, 5, E, 1) // gauntlet across the middle row
		b.addLaser(9, 5, W, 1)
		b.setExpress(3, 3) // belt carries 2 — from x3 it lands you on cp2 (x5)
		b.setExpress(4, 3)
	},
}

// Bana 3 (Svår): signature "gear chokepoint + crossfire" — double-barrel lasers
// rake the middle, gears spin you at the tight approaches, and pits flank every
// checkpoint so a sloppy line dies.
var bana3 = curatedCourse{
	tier: DiffHard,
	rows: []string{
		"...3......",
		"..O..O....",
		"....R.....",
		"..>>2<<...",
		"....A.....",
		".O.....O..",
		"...RR.....",
		"..O.1.O...",
		"..........",
		".a.b.c.d..",
	},
	overlay: func(b *Board) {
		b.addLaser(0, 5, E, 2) // crossfire, double-barrel
		b.addLaser(9, 5, W, 2)
	},
}

var curatedCourses = []curatedCourse{bana1, bana2, bana3}

// NumCurated returns how many hand-authored courses exist.
func NumCurated() int { return len(curatedCourses) }

// CuratedTier returns the difficulty tier of curated course i (for the round
// budget used when verifying/solving it).
func CuratedTier(i int) CourseDiff { return curatedCourses[i].tier }

// CuratedCourse builds hand-authored course i.
func CuratedCourse(i int) *Board {
	c := curatedCourses[i]
	b := DecodeBoard(c.rows)
	if c.overlay != nil {
		c.overlay(b)
	}
	return b
}

// --- overlay helpers (edge/attribute detail the rune grid can't carry) ------

func (b *Board) setExpress(x, y int) { b.At(image.Pt(x, y)).BeltExpress = true }
func (b *Board) addWall(x, y int, d Dir) {
	b.At(image.Pt(x, y)).Walls |= d.wallBit()
}
func (b *Board) addLaser(x, y int, d Dir, count uint8) {
	t := b.At(image.Pt(x, y))
	t.Laser = d
	t.LaserCount = count
}

// --- Rune-map decoder -------------------------------------------------------

// DecodeBoard builds a board from a rune map, for curated courses, tests and
// fixtures:
//
//	'.' ' '  plain floor      'O' pit        '+' repair
//	'^>v<'   single belts      'R' gear CW    'L' gear CCW
//	'A' antenna                '1'..'9' checkpoint ordinal
//	'a'..'h' start dock (facing North)
//
// Express belts, walls and lasers are applied by an overlay after decoding.
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
			case ch == 'R':
				t.Gear = GearCW
			case ch == 'L':
				t.Gear = GearCCW
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
