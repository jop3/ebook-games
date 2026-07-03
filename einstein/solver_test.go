package main

import "testing"

// §6.8 — Validering med det klassiska zebra-/Einstein-pusslet.
//
// 5 hus i rad (positioner 0..4). Fem kategorier:
//   Cat 0 Nationalitet: Brit, Swede, Dane, Norwegian, German
//   Cat 1 Färg:         Red, Green, White, Yellow, Blue
//   Cat 2 Dryck:        Tea, Coffee, Milk, Beer, Water
//   Cat 3 Rök:          PallMall, Dunhill, Blends, BlueMasters, Prince
//   Cat 4 Husdjur:      Dogs, Birds, Cats, Horses, Fish

const (
	// Nationalitet
	Brit = iota
	Swede
	Dane
	Norwegian
	German
)
const (
	// Färg
	Red = iota
	Green
	White
	Yellow
	Blue
)
const (
	// Dryck
	Tea = iota
	Coffee
	Milk
	Beer
	Water
)
const (
	// Rök
	PallMall = iota
	Dunhill
	Blends
	BlueMasters
	Prince
)
const (
	// Husdjur
	Dogs = iota
	Birds
	Cats
	Horses
	Fish
)

const (
	catNat = iota
	catColor
	catDrink
	catSmoke
	catPet
)

func zebraClues() []Clue {
	return []Clue{
		// 1. Britten bor i det röda huset.
		{Type: SamePosition, CatA: catNat, ValA: Brit, CatB: catColor, ValB: Red},
		// 2. Svensken har hundar.
		{Type: SamePosition, CatA: catNat, ValA: Swede, CatB: catPet, ValB: Dogs},
		// 3. Dansken dricker te.
		{Type: SamePosition, CatA: catNat, ValA: Dane, CatB: catDrink, ValB: Tea},
		// 4. Det gröna huset ligger direkt till vänster om det vita.
		//    => grön direkt till vänster => vit == grön + 1 => White direkt höger om Green.
		{Type: DirectionalNeighbor, CatA: catColor, ValA: White, CatB: catColor, ValB: Green},
		// 5. Ägaren av det gröna huset dricker kaffe.
		{Type: SamePosition, CatA: catColor, ValA: Green, CatB: catDrink, ValB: Coffee},
		// 6. Personen som röker Pall Mall har fåglar.
		{Type: SamePosition, CatA: catSmoke, ValA: PallMall, CatB: catPet, ValB: Birds},
		// 7. Ägaren av det gula huset röker Dunhill.
		{Type: SamePosition, CatA: catColor, ValA: Yellow, CatB: catSmoke, ValB: Dunhill},
		// 8. Mannen i mittenhuset dricker mjölk.
		{Type: Direct, CatA: catDrink, ValA: Milk, Position: 2},
		// 9. Norrmannen bor i första huset.
		{Type: Direct, CatA: catNat, ValA: Norwegian, Position: 0},
		// 10. Mannen som röker Blends bor bredvid den som har katter.
		{Type: Neighbor, CatA: catSmoke, ValA: Blends, CatB: catPet, ValB: Cats},
		// 11. Mannen som har hästar bor bredvid mannen som röker Dunhill.
		{Type: Neighbor, CatA: catPet, ValA: Horses, CatB: catSmoke, ValB: Dunhill},
		// 12. Mannen som röker Blue Master dricker öl.
		{Type: SamePosition, CatA: catSmoke, ValA: BlueMasters, CatB: catDrink, ValB: Beer},
		// 13. Tysken röker Prince.
		{Type: SamePosition, CatA: catNat, ValA: German, CatB: catSmoke, ValB: Prince},
		// 14. Norrmannen bor bredvid det blå huset.
		{Type: Neighbor, CatA: catNat, ValA: Norwegian, CatB: catColor, ValB: Blue},
		// 15. Mannen som röker Blends har en granne som dricker vatten.
		{Type: Neighbor, CatA: catSmoke, ValA: Blends, CatB: catDrink, ValB: Water},
	}
}

func TestZebraUnique(t *testing.T) {
	clues := zebraClues()
	count := CountSolutions(5, 5, clues)
	if count != 1 {
		t.Fatalf("zebra-pusslet: förväntade unik lösning (1), fick %d", count)
	}
}

func TestZebraGermanOwnsFish(t *testing.T) {
	clues := zebraClues()
	sol, unique := Solve(5, 5, clues)
	if !unique {
		t.Fatalf("zebra-pusslet: ingen unik lösning")
	}
	// Hitta tyskens position.
	germanPos := -1
	for p := 0; p < 5; p++ {
		if sol.Assignment[catNat][p] == German {
			germanPos = p
		}
	}
	if germanPos < 0 {
		t.Fatalf("hittade inte tysken i lösningen")
	}
	if sol.Assignment[catPet][germanPos] != Fish {
		t.Fatalf("tysken (pos %d) äger %d, förväntade Fish(%d)",
			germanPos, sol.Assignment[catPet][germanPos], Fish)
	}
	// Fisken ska ägas av tysken.
	fishPos := -1
	for p := 0; p < 5; p++ {
		if sol.Assignment[catPet][p] == Fish {
			fishPos = p
		}
	}
	if sol.Assignment[catNat][fishPos] != German {
		t.Fatalf("fisken (pos %d) ägs av nationalitet %d, förväntade German(%d)",
			fishPos, sol.Assignment[catNat][fishPos], German)
	}
}
