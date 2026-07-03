package series

import "testing"

func TestMergeOwnedAndMissing(t *testing.T) {
	owned := []Book{
		{ID: 10, Title: "A Deadly Education", Series: Series{Name: "The Scholomance", Number: 1}},
	}
	full := FullSeries{Name: "The Scholomance", Books: []SeriesEntry{
		{Title: "A Deadly Education", Year: 2020},
		{Title: "The Last Graduate", Year: 2021},
		{Title: "The Golden Enclaves", Year: 2022},
	}}
	rows := Merge(owned, full)
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	if !rows[0].Owned || rows[0].BookID != 10 {
		t.Errorf("row1 should be owned book 10, got %+v", rows[0])
	}
	if rows[1].Owned || rows[2].Owned {
		t.Errorf("rows 2,3 should be missing: %+v %+v", rows[1], rows[2])
	}
	if next := NextToDownload(rows); next != "The Last Graduate" {
		t.Errorf("next-to-download = %q, want The Last Graduate", next)
	}
}

func TestMergeFallbackNoFull(t *testing.T) {
	owned := []Book{{ID: 1, Title: "X", Series: Series{Name: "S", Number: 2}}}
	rows := Merge(owned, FullSeries{})
	if len(rows) != 1 || !rows[0].Owned || rows[0].Position != 2 {
		t.Errorf("fallback wrong: %+v", rows)
	}
}

func TestNormTitleMatches(t *testing.T) {
	if normTitle("The Last Graduate") != normTitle("Last Graduate (The Scholomance #2)") {
		t.Errorf("normTitle mismatch: %q vs %q",
			normTitle("The Last Graduate"), normTitle("Last Graduate (The Scholomance #2)"))
	}
}
