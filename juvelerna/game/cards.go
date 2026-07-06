// Package game implements Juvelerna ("The Gems"), a 2-player engine-building
// game based on Splendor (Marc André, Space Cowboys), reimplemented here with
// original card/noble values and a neutral working title. Pure Go, no SDK
// dependency, fully unit-tested.
//
// NOTE on card data: the exact cost/point values on the 90 development cards
// and the 5 noble tiles below are ORIGINAL values designed for this
// implementation (a simple, internally-consistent cost/points curve across
// the 3 tiers), not transcribed from the official Splendor card list — per
// this repo's naming-courtesy convention of building original data alongside
// a neutral title, this file documents that explicitly.
package game

// Color is one of the 5 gem-token colors. Rendered on e-ink as 5 distinct
// greyscale patterns (solid fill, ring, cross-hatch, diagonal stripe, dot
// grid), mirroring mosaik's pattern palette. Swedish gem names (for UI
// labels) are attached in ui.go: Diamant/Safir/Smaragd/Rubin/Onyx.
type Color int

const (
	ColorSolid  Color = iota // "Diamant"
	ColorRing                // "Safir"
	ColorCross               // "Smaragd"
	ColorStripe              // "Rubin"
	ColorDot                 // "Onyx"
	NumColors
)

// Tunable constants for the 2-player configuration this app targets.
const (
	TokensPerColor2P = 4  // each of the 5 gem colors starts with this many tokens
	GoldTokens       = 5  // wildcard tokens, always 5 regardless of player count
	MaxReserved      = 3  // a player may hold at most this many reserved cards
	TokenCap         = 10 // total tokens (gems+gold) a player may hold at end of turn
	PrestigeToWin    = 15 // reaching this at the end of a turn triggers game-end
	NoblePoints      = 3  // prestige awarded for claiming a noble
	NumNobles2P      = 3  // nobles drawn face-up for a 2-player game
	NumTiers         = 3
	TableauSlots     = 4 // face-up slots per tier
)

// Card is one development card. Tier is 1..3; zero-value Card{} (Tier == 0)
// is used throughout as the "empty slot / no card" sentinel, since every
// real card has Tier >= 1 — this avoids a separate pointer or bool array for
// tableau slots.
type Card struct {
	ID     int
	Tier   int
	Color  Color // the permanent 1-gem discount this card grants once bought
	Points int
	Cost   [NumColors]int // gem cost per color; Cost[Color] is always 0 (a
	// card never costs its own bonus color, matching the official game)
}

// Noble is a noble tile: 3 prestige, auto-claimed once a player's card-color
// bonuses meet Requirement in every listed color.
type Noble struct {
	ID          int
	Requirement [NumColors]int
}

// costTemplate is a cost shape shared by every color's card at a given
// index within a tier — see buildCost for how it's rotated per color/index
// to avoid 5 identical-shaped cards per template.
type costTemplate struct {
	cost   [4]int // costs for the 4 non-bonus colors, before rotation
	points int
}

// tier1Templates: 8 templates x 5 colors = 40 tier-1 cards. Cheap (2-4 gems
// total), 0 points except the single most expensive shape (1 point).
var tier1Templates = []costTemplate{
	{cost: [4]int{1, 1, 1, 0}, points: 0},
	{cost: [4]int{2, 1, 0, 0}, points: 0},
	{cost: [4]int{2, 0, 1, 0}, points: 0},
	{cost: [4]int{0, 2, 1, 0}, points: 0},
	{cost: [4]int{3, 0, 0, 0}, points: 0},
	{cost: [4]int{0, 0, 2, 1}, points: 0},
	{cost: [4]int{1, 0, 0, 2}, points: 0},
	{cost: [4]int{4, 0, 0, 0}, points: 1},
}

// tier2Templates: 6 templates x 5 colors = 30 tier-2 cards. Mid-range
// (6-8 gems total), 1-3 points.
var tier2Templates = []costTemplate{
	{cost: [4]int{3, 2, 2, 0}, points: 1},
	{cost: [4]int{0, 3, 2, 3}, points: 2},
	{cost: [4]int{5, 3, 0, 0}, points: 2},
	{cost: [4]int{2, 0, 0, 6}, points: 3},
	{cost: [4]int{0, 0, 5, 3}, points: 2},
	{cost: [4]int{6, 0, 0, 0}, points: 3},
}

// tier3Templates: 4 templates x 5 colors = 20 tier-3 cards. Expensive
// (10-14 gems total), 3-5 points.
var tier3Templates = []costTemplate{
	{cost: [4]int{3, 3, 5, 3}, points: 3},
	{cost: [4]int{0, 3, 3, 6}, points: 4},
	{cost: [4]int{7, 3, 0, 0}, points: 4},
	{cost: [4]int{7, 0, 0, 3}, points: 5},
}

// nobleTemplates: 5 nobles, each requiring 3 cards of each of 3 cyclically
// adjacent colors (an original, simple, symmetric design — every noble
// costs the same "shape", just rotated across the 5 colors).
var nobleTemplates = buildNobleTemplates()

func buildNobleTemplates() [5][NumColors]int {
	var out [5][NumColors]int
	for i := 0; i < 5; i++ {
		var req [NumColors]int
		req[i] = 3
		req[(i+1)%5] = 3
		req[(i+2)%5] = 3
		out[i] = req
	}
	return out
}

// otherColors returns the 4 colors other than c, in ascending numeric order.
func otherColors(c Color) [4]Color {
	var out [4]Color
	i := 0
	for k := Color(0); k < NumColors; k++ {
		if k == c {
			continue
		}
		out[i] = k
		i++
	}
	return out
}

// buildCost expands a costTemplate into a full per-color cost array for
// bonus color c, rotating which "other color" receives which template slot
// by `rotate` (the card's index within its tier/color group) so the 5
// same-template cards (one per color) don't all look like trivial relabeled
// copies of each other.
func buildCost(c Color, tmpl [4]int, rotate int) [NumColors]int {
	others := otherColors(c)
	var cost [NumColors]int
	for j := 0; j < 4; j++ {
		idx := (j + rotate) % 4
		cost[others[idx]] += tmpl[j]
	}
	return cost
}

func templatesForTier(tier int) []costTemplate {
	switch tier {
	case 1:
		return tier1Templates
	case 2:
		return tier2Templates
	case 3:
		return tier3Templates
	}
	return nil
}

// buildAllCards constructs the full, deterministic 90-card set (in a fixed
// order — tier, then color, then template index). Callers shuffle a copy
// before dealing; the order here itself is not randomized so tests can
// address specific cards by their deterministic ID.
func buildAllCards() [NumTiers][]Card {
	var decks [NumTiers][]Card
	id := 0
	for tier := 1; tier <= NumTiers; tier++ {
		templates := templatesForTier(tier)
		for c := Color(0); c < NumColors; c++ {
			for ti, tmpl := range templates {
				card := Card{
					ID:     id,
					Tier:   tier,
					Color:  c,
					Points: tmpl.points,
					Cost:   buildCost(c, tmpl.cost, ti),
				}
				decks[tier-1] = append(decks[tier-1], card)
				id++
			}
		}
	}
	return decks
}

// buildAllNobles constructs the fixed 5-noble pool; NewGame draws
// NumNobles2P of them at random.
func buildAllNobles() []Noble {
	out := make([]Noble, len(nobleTemplates))
	for i, req := range nobleTemplates {
		out[i] = Noble{ID: i, Requirement: req}
	}
	return out
}
