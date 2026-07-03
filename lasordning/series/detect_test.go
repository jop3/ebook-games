package series

import "testing"

func TestDetectFromTitle(t *testing.T) {
	cases := []struct {
		title    string
		wantName string
		wantNum  float64
	}{
		// real examples from the target library
		{"The Captain’s Daughter: Book Two of the Arkship Trilogy", "Arkship Trilogy", 2},
		{"Raybearer #1", "Raybearer", 1},
		{"Raybearer (Raybearer Series #1)", "Raybearer Series", 1},
		{"Dark Forge", "", 0},
		{"Pride and Prejudice", "", 0},
		{"Mistborn: Book One", "Mistborn", 1},
		{"The Way of Kings (The Stormlight Archive #1)", "The Stormlight Archive", 1},
		{"Some Saga, Book Three", "Some Saga", 3},
		{"Foundation (Foundation Book 2)", "Foundation", 2},
		// roman numerals in parens
		{"Dune Messiah (Dune Chronicles #II)", "Dune Chronicles", 2},
	}
	for _, c := range cases {
		got := DetectFromTitle(c.title)
		if got.Name != c.wantName || got.Number != c.wantNum {
			t.Errorf("DetectFromTitle(%q) = {%q, %v}, want {%q, %v}",
				c.title, got.Name, got.Number, c.wantName, c.wantNum)
		}
	}
}

func TestGroupOrdersAndBuckets(t *testing.T) {
	books := []Book{
		{ID: 1, Title: "B", Author: "X", FirstAuthor: "X", Series: Series{Name: "S", Number: 2, Source: SourceTitle}},
		{ID: 2, Title: "A", Author: "X", FirstAuthor: "X", Series: Series{Name: "S", Number: 1, Source: SourceMetadata}},
		{ID: 3, Title: "Loose", Author: "Y", FirstAuthor: "Y"},
	}
	g := Group(books)
	if len(g.Series) != 1 {
		t.Fatalf("want 1 series, got %d", len(g.Series))
	}
	s := g.Series[0]
	if s.Books[0].Series.Number != 1 || s.Books[1].Series.Number != 2 {
		t.Errorf("books not ordered by number: %+v", s.Books)
	}
	if s.Source != SourceTitle { // strongest source wins
		t.Errorf("want strongest source Title, got %v", s.Source)
	}
	if len(g.Standalone) != 1 || g.Standalone[0].ID != 3 {
		t.Errorf("standalone bucket wrong: %+v", g.Standalone)
	}
}

func TestMissingBefore(t *testing.T) {
	g := SeriesGroup{Books: []Book{
		{Series: Series{Number: 1}},
		{Series: Series{Number: 3}},
	}}
	gaps := g.MissingBefore()
	if len(gaps) != 1 || gaps[0] != 2 {
		t.Errorf("want gap [2], got %v", gaps)
	}
}

func TestNormSeriesKey(t *testing.T) {
	if normSeriesKey("The Arkship Trilogy") != normSeriesKey("arkship trilogy") {
		t.Error("article/case normalization failed")
	}
}
