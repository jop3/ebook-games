package game

import (
	"image"
	"math/rand"
)

// Phase is the high-level state of a round.
type Phase int

const (
	PhaseProgram Phase = iota // human is filling registers; AIs have planned
	PhaseResolve              // stepping registers on the board
	PhaseDone                 // someone won
)

// Robot is one racer. The human is always Robots[0].
type Robot struct {
	Pos        image.Point
	Facing     Dir
	Damage     int
	NextCheck  uint8 // checkpoint still needed (1..NCheck); > NCheck means finished
	ArchivePos image.Point
	ArchiveDir Dir
	Alive      bool
	IsHuman    bool
	Profile    AIProfile
	ID         int

	damagedLastRound bool // stress input for the AI fumble model
	deadThisRound    bool // died mid-round, awaiting end-of-round respawn
	tookDamage       bool // took damage/died this round (folds into damagedLastRound)
}

// GameState is a full playable game.
type GameState struct {
	Board     *Board
	Robots    []Robot
	Round     int
	Phase     Phase
	CurReg    int
	Registers [][5]Card
	Hands     [][]Card
	Log       []string
	Winner    int // -1 until someone finishes
	AIDiff    AILevel

	rng          *rand.Rand
	humanRegSlot [5]int // hand index backing each human register, or -1
}

// NewGame seeds a game: 1 human (Robots[0]) plus nAI opponents on a course.
func NewGame(board *Board, nAI int, aiDiff AILevel, seed int64) *GameState {
	total := nAI + 1
	if total > len(board.Docks) {
		total = len(board.Docks)
	}
	s := &GameState{
		Board:  board,
		Phase:  PhaseProgram,
		Winner: -1,
		AIDiff: aiDiff,
		rng:    rand.New(rand.NewSource(seed)),
	}
	profiles := aiProfiles(aiDiff, nAI, s.rng)
	for i := 0; i < total; i++ {
		d := board.Docks[i]
		r := Robot{
			Pos: d.Pos, Facing: d.Facing,
			NextCheck: 1, ArchivePos: d.Pos, ArchiveDir: d.Facing,
			Alive: true, ID: i,
		}
		if i == 0 {
			r.IsHuman = true
		} else {
			r.Profile = profiles[i-1]
		}
		s.Robots = append(s.Robots, r)
	}
	s.Registers = make([][5]Card, len(s.Robots))
	s.Hands = make([][]Card, len(s.Robots))
	s.beginRound()
	return s
}

// handSize is the number of cards a robot draws given its damage.
func handSize(damage int) int {
	n := 9 - damage
	if n < 1 {
		n = 1
	}
	return n
}

// beginRound draws fresh hands and clears registers for a new programming phase.
func (s *GameState) beginRound() {
	s.Round++
	s.Phase = PhaseProgram
	s.CurReg = 0
	s.Log = nil
	for i := range s.Robots {
		if s.Robots[i].NextCheck > s.Board.NCheck {
			s.Hands[i] = nil
			continue
		}
		s.Hands[i] = drawHand(s.rng, handSize(s.Robots[i].Damage))
		s.Registers[i] = [5]Card{CardNone, CardNone, CardNone, CardNone, CardNone}
	}
	for r := range s.humanRegSlot {
		s.humanRegSlot[r] = -1
	}
}

// --- Human programming ------------------------------------------------------

// PlaceFromHand slots the human's hand[idx] card into the next empty register.
// Returns false if the card is already used or all registers are full.
func (s *GameState) PlaceFromHand(idx int) bool {
	if s.Phase != PhaseProgram || idx < 0 || idx >= len(s.Hands[0]) {
		return false
	}
	if s.handCardUsed(idx) {
		return false
	}
	for r := 0; r < 5; r++ {
		if s.Registers[0][r] == CardNone {
			s.Registers[0][r] = s.Hands[0][idx]
			s.humanRegSlot[r] = idx
			return true
		}
	}
	return false
}

// ClearRegister empties the human's register r, returning its card to the hand.
func (s *GameState) ClearRegister(r int) bool {
	if s.Phase != PhaseProgram || r < 0 || r >= 5 || s.Registers[0][r] == CardNone {
		return false
	}
	s.Registers[0][r] = CardNone
	s.humanRegSlot[r] = -1
	return true
}

// handCardUsed reports whether hand slot idx is currently placed in a register.
func (s *GameState) handCardUsed(idx int) bool {
	for r := 0; r < 5; r++ {
		if s.humanRegSlot[r] == idx {
			return true
		}
	}
	return false
}

// HandCardUsed is the exported view for the UI (grey out placed cards).
func (s *GameState) HandCardUsed(idx int) bool { return s.handCardUsed(idx) }

// ProgramComplete reports whether the human has committed as many registers as
// their hand allows (all 5, or all cards when a damaged hand holds fewer than 5).
func (s *GameState) ProgramComplete() bool {
	if s.Robots[0].NextCheck > s.Board.NCheck {
		return true // already finished — nothing to program
	}
	need := 5
	if len(s.Hands[0]) < need {
		need = len(s.Hands[0])
	}
	placed := 0
	for r := 0; r < 5; r++ {
		if s.Registers[0][r] != CardNone {
			placed++
		}
	}
	return placed >= need
}

// StartResolve locks in the program: every AI plans its five registers (blind —
// see ai.go), and the phase switches to resolution at register 0.
func (s *GameState) StartResolve() bool {
	if s.Phase != PhaseProgram || !s.ProgramComplete() {
		return false
	}
	for i := range s.Robots {
		if s.Robots[i].IsHuman || s.Robots[i].NextCheck > s.Board.NCheck || !s.Robots[i].Alive {
			continue
		}
		s.Registers[i] = s.planAI(i)
	}
	s.Phase = PhaseResolve
	s.CurReg = 0
	s.Log = nil
	return true
}

// AICount returns the number of AI opponents.
func (s *GameState) AICount() int { return len(s.Robots) - 1 }
