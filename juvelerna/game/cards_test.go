package game

import "testing"

func TestDeckSizesPerTier(t *testing.T) {
	decks := buildAllCards()
	wantSizes := [NumTiers]int{40, 30, 20}
	for i, want := range wantSizes {
		if len(decks[i]) != want {
			t.Fatalf("tier %d: got %d cards, want %d", i+1, len(decks[i]), want)
		}
	}
}

func TestTotalCardCountIs90(t *testing.T) {
	decks := buildAllCards()
	total := 0
	for _, d := range decks {
		total += len(d)
	}
	if total != 90 {
		t.Fatalf("total cards = %d, want 90", total)
	}
}

func TestCardIDsAreUnique(t *testing.T) {
	decks := buildAllCards()
	seen := map[int]bool{}
	for _, d := range decks {
		for _, c := range d {
			if seen[c.ID] {
				t.Fatalf("duplicate card ID %d", c.ID)
			}
			seen[c.ID] = true
		}
	}
	if len(seen) != 90 {
		t.Fatalf("expected 90 unique IDs, got %d", len(seen))
	}
}

// A card never costs any of its own bonus color (matches the official
// game's design and keeps "buy a card with its own future discount" a
// non-issue).
func TestCardNeverCostsOwnColor(t *testing.T) {
	decks := buildAllCards()
	for _, d := range decks {
		for _, c := range d {
			if c.Cost[c.Color] != 0 {
				t.Fatalf("card %d (tier %d, color %v) costs %d of its own bonus color, want 0",
					c.ID, c.Tier, c.Color, c.Cost[c.Color])
			}
		}
	}
}

// Sanity-check the cost/points curve rises across tiers: tier 3 should be
// costlier and worth more points, on average, than tier 1.
func TestCostAndPointsCurveRisesAcrossTiers(t *testing.T) {
	decks := buildAllCards()
	avg := func(d []Card) (avgCost, avgPoints float64) {
		var totalCost, totalPoints int
		for _, c := range d {
			totalPoints += c.Points
			for _, v := range c.Cost {
				totalCost += v
			}
		}
		n := float64(len(d))
		return float64(totalCost) / n, float64(totalPoints) / n
	}
	c1, p1 := avg(decks[0])
	c2, p2 := avg(decks[1])
	c3, p3 := avg(decks[2])
	if !(c1 < c2 && c2 < c3) {
		t.Fatalf("average cost should rise tier1<tier2<tier3, got %.2f %.2f %.2f", c1, c2, c3)
	}
	if !(p1 < p2 && p2 < p3) {
		t.Fatalf("average points should rise tier1<tier2<tier3, got %.2f %.2f %.2f", p1, p2, p3)
	}
}

// Every color should get an equal share of cards per tier (40/5, 30/5,
// 20/5) so the tableau isn't lopsided.
func TestEachColorEquallyRepresentedPerTier(t *testing.T) {
	decks := buildAllCards()
	wantPerColor := [NumTiers]int{8, 6, 4}
	for tier, d := range decks {
		counts := map[Color]int{}
		for _, c := range d {
			counts[c.Color]++
		}
		for col := Color(0); col < NumColors; col++ {
			if counts[col] != wantPerColor[tier] {
				t.Fatalf("tier %d color %v: got %d cards, want %d", tier+1, col, counts[col], wantPerColor[tier])
			}
		}
	}
}

func TestNoblesFiveTemplatesEachThreeColorsAtThree(t *testing.T) {
	nobles := buildAllNobles()
	if len(nobles) != 5 {
		t.Fatalf("expected 5 noble templates, got %d", len(nobles))
	}
	for _, n := range nobles {
		nonzero := 0
		for _, v := range n.Requirement {
			if v != 0 {
				if v != 3 {
					t.Fatalf("noble %d: expected requirement values of 3, got %d", n.ID, v)
				}
				nonzero++
			}
		}
		if nonzero != 3 {
			t.Fatalf("noble %d: expected exactly 3 required colors, got %d", n.ID, nonzero)
		}
	}
}
