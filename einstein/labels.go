package main

// labels.go — människoläsbara etiketter för kategorier/värden och
// rendering av ledtrådstext. FRI från inkview-import (testbar).

import "fmt"

// LabelSet ger namn åt kategorier och deras värden.
type LabelSet struct {
	Categories []string   // len == Categories
	Values     [][]string // Values[cat][val]
}

// DefaultLabels bygger ett tematiskt etikettset som räcker för N upp till 5.
// För mindre N används prefix av listorna.
func DefaultLabels(n, categories int) LabelSet {
	catNames := []string{"Nationalitet", "Färg", "Dryck", "Rök", "Husdjur"}
	values := [][]string{
		{"Britt", "Svensk", "Dansk", "Norsk", "Tysk"},
		{"Röd", "Grön", "Vit", "Gul", "Blå"},
		{"Te", "Kaffe", "Mjölk", "Öl", "Vatten"},
		{"Pall Mall", "Dunhill", "Blend", "Blue Master", "Prince"},
		{"Hund", "Fågel", "Katt", "Häst", "Fisk"},
	}
	ls := LabelSet{}
	for c := 0; c < categories && c < len(catNames); c++ {
		ls.Categories = append(ls.Categories, catNames[c])
		row := make([]string, 0, n)
		for v := 0; v < n && v < len(values[c]); v++ {
			row = append(row, values[c][v])
		}
		ls.Values = append(ls.Values, row)
	}
	return ls
}

func (ls LabelSet) cat(c int) string {
	if c >= 0 && c < len(ls.Categories) {
		return ls.Categories[c]
	}
	return fmt.Sprintf("K%d", c)
}

func (ls LabelSet) val(c, v int) string {
	if c >= 0 && c < len(ls.Values) && v >= 0 && v < len(ls.Values[c]) {
		return ls.Values[c][v]
	}
	return fmt.Sprintf("V%d.%d", c, v)
}

// ClueText renderar en ledtråd som svensk mening.
func ClueText(ls LabelSet, c Clue) string {
	switch c.Type {
	case Direct:
		return fmt.Sprintf("%s finns på plats %d.", ls.val(c.CatA, c.ValA), c.Position+1)
	case SamePosition:
		return fmt.Sprintf("%s hör ihop med %s.", ls.val(c.CatA, c.ValA), ls.val(c.CatB, c.ValB))
	case NotSamePosition:
		return fmt.Sprintf("%s hör INTE ihop med %s.", ls.val(c.CatA, c.ValA), ls.val(c.CatB, c.ValB))
	case Neighbor:
		return fmt.Sprintf("%s är granne med %s.", ls.val(c.CatA, c.ValA), ls.val(c.CatB, c.ValB))
	case DirectionalNeighbor:
		return fmt.Sprintf("%s är direkt till höger om %s.", ls.val(c.CatA, c.ValA), ls.val(c.CatB, c.ValB))
	}
	return "?"
}
