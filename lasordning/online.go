package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"lasordning/series"
)

// online.go implements the network fallback: when the DB and title heuristics
// can't place a book in a series, we ask Wikidata (structured) and then the
// Wikipedia infobox (best coverage) — the chain verified during design.
//
// All calls have short timeouts and fail soft: if the device is offline or a
// lookup misses, we simply return an empty Series and the app carries on.

const userAgent = "Lasordning/1.0 (PocketBook reading-order app; contact: local user)"

var httpClient = &http.Client{Timeout: 6 * time.Second}

// Lookup resolves series info for a title+author from the network. It returns
// an empty Series (Name=="") if nothing was found or the network is down.
func Lookup(ctx context.Context, title, author string) series.Series {
	// Strip any trailing "#1"/"(Series #1)" noise from the query title.
	q := cleanQueryTitle(title)

	if s := lookupWikidata(ctx, q); s.Name != "" {
		return s
	}
	if s := lookupWikipedia(ctx, q); s.Name != "" {
		return s
	}
	return series.Series{}
}

// --- Wikidata ---------------------------------------------------------------

func lookupWikidata(ctx context.Context, title string) series.Series {
	qid := wikidataSearch(ctx, title)
	if qid == "" {
		return series.Series{}
	}
	return wikidataSeries(ctx, qid)
}

// wikidataSearch finds the best-matching entity QID for a book title.
func wikidataSearch(ctx context.Context, title string) string {
	u := "https://www.wikidata.org/w/api.php?" + url.Values{
		"action":   {"wbsearchentities"},
		"search":   {title},
		"language": {"en"},
		"type":     {"item"},
		"limit":    {"1"},
		"format":   {"json"},
	}.Encode()

	var resp struct {
		Search []struct {
			ID string `json:"id"`
		} `json:"search"`
	}
	if !getJSON(ctx, u, &resp) || len(resp.Search) == 0 {
		return ""
	}
	return resp.Search[0].ID
}

// wikidataSeries reads P179 (series) + P1545 (ordinal) via the entity data API.
func wikidataSeries(ctx context.Context, qid string) series.Series {
	u := "https://www.wikidata.org/wiki/Special:EntityData/" + qid + ".json"
	var doc struct {
		Entities map[string]struct {
			Claims map[string][]struct {
				Mainsnak struct {
					Datavalue struct {
						Value struct {
							ID string `json:"id"`
						} `json:"value"`
					} `json:"datavalue"`
				} `json:"mainsnak"`
				Qualifiers map[string][]struct {
					Datavalue struct {
						Value string `json:"value"`
					} `json:"datavalue"`
				} `json:"qualifiers"`
			} `json:"claims"`
		} `json:"entities"`
	}
	if !getJSON(ctx, u, &doc) {
		return series.Series{}
	}
	ent, ok := doc.Entities[qid]
	if !ok {
		return series.Series{}
	}
	p179, ok := ent.Claims["P179"]
	if !ok || len(p179) == 0 {
		return series.Series{}
	}
	seriesQID := p179[0].Mainsnak.Datavalue.Value.ID
	name := wikidataLabel(ctx, seriesQID)
	if name == "" {
		return series.Series{}
	}
	var num float64
	if q, ok := p179[0].Qualifiers["P1545"]; ok && len(q) > 0 {
		fmt.Sscanf(q[0].Datavalue.Value, "%g", &num)
	}
	return series.Series{Name: name, Number: num, Source: series.SourceWikidata}
}

func wikidataLabel(ctx context.Context, qid string) string {
	u := "https://www.wikidata.org/wiki/Special:EntityData/" + qid + ".json"
	var doc struct {
		Entities map[string]struct {
			Labels map[string]struct {
				Value string `json:"value"`
			} `json:"labels"`
		} `json:"entities"`
	}
	if !getJSON(ctx, u, &doc) {
		return ""
	}
	if ent, ok := doc.Entities[qid]; ok {
		if l, ok := ent.Labels["en"]; ok {
			return l.Value
		}
		for _, l := range ent.Labels { // any language as fallback
			return l.Value
		}
	}
	return ""
}

// --- Full series book list --------------------------------------------------

// LookupReport records which sources were consulted and what each returned, so
// the UI can tell the user exactly what was checked when nothing was found.
type LookupReport struct {
	Lines []string
}

func (r *LookupReport) add(line string) {
	if r != nil {
		r.Lines = append(r.Lines, line)
	}
}

// SeriesBooks fetches every book in a named series, ordered for reading. It
// queries the Wikidata Query Service for items whose "part of the series"
// (P179) is the series with the given English label, taking each book's series
// ordinal (P1545) when present and its earliest publication year (P577) as a
// fallback order. If Wikidata is empty it walks the Wikipedia infobox
// preceded_by/followed_by chain from each owned title. The optional report
// records what was tried. Returns an empty FullSeries on failure/offline.
func SeriesBooks(ctx context.Context, name string, report *LookupReport, seedTitles ...string) series.FullSeries {
	fs := series.FullSeries{Name: name}
	if name == "" {
		return fs
	}
	// SPARQL: books in the series + optional ordinal + earliest pub year.
	// %q-escaping the label into the query; the label match is exact.
	query := `SELECT ?bookLabel (SAMPLE(?ord) AS ?ordinal) (MIN(YEAR(?pub)) AS ?year) WHERE {
  ?series rdfs:label ` + sparqlString(name) + `@en .
  ?book wdt:P179 ?series .
  OPTIONAL { ?book p:P179 ?st. ?st ps:P179 ?series. ?st pq:P1545 ?ord. }
  OPTIONAL { ?book wdt:P577 ?pub. }
  SERVICE wikibase:label { bd:serviceParam wikibase:language "en". }
} GROUP BY ?bookLabel ORDER BY ?ordinal ?year`

	u := "https://query.wikidata.org/sparql?format=json&query=" + url.QueryEscape(query)
	var resp struct {
		Results struct {
			Bindings []struct {
				BookLabel struct {
					Value string `json:"value"`
				} `json:"bookLabel"`
				Ordinal struct {
					Value string `json:"value"`
				} `json:"ordinal"`
				Year struct {
					Value string `json:"value"`
				} `json:"year"`
			} `json:"bindings"`
		} `json:"results"`
	}
	if !getJSON(ctx, u, &resp) {
		report.add("Wikidata: ingen kontakt (offline?)")
		return fs
	}
	for _, b := range resp.Results.Bindings {
		if b.BookLabel.Value == "" {
			continue
		}
		var num float64
		if b.Ordinal.Value != "" {
			fmt.Sscanf(b.Ordinal.Value, "%g", &num)
		}
		var yr int
		if b.Year.Value != "" {
			fmt.Sscanf(b.Year.Value, "%d", &yr)
		}
		fs.Books = append(fs.Books, series.SeriesEntry{Title: b.BookLabel.Value, Number: num, Year: yr})
	}
	if len(fs.Books) > 0 {
		report.add("Wikidata: hittade " + itoa(len(fs.Books)) + " böcker")
		return fs
	}
	report.add("Wikidata: serien \"" + name + "\" saknas")

	// Fallback: if Wikidata knew nothing, try the Wikipedia followed_by chain
	// from each owned book title until one yields a series.
	if len(fs.Books) == 0 {
		for _, seed := range seedTitles {
			// Resolve the (possibly ambiguous) title to a real article first.
			resolved := wikipediaResolve(ctx, cleanQueryTitle(seed)+" novel")
			if resolved == "" {
				resolved = cleanQueryTitle(seed)
			}
			if entries := seriesByChain(ctx, resolved); len(entries) >= 2 {
				report.add("Wikipedia: byggde serie från \"" + resolved + "\"")
				fs.Books = entries
				break
			}
		}
		if len(fs.Books) == 0 {
			report.add("Wikipedia: ingen serie-kedja i infoboxen")
		}
	}
	return fs
}

// wikipediaResolve resolves a free-text title (optionally with author) to the
// actual Wikipedia article title, so a bare/ambiguous title like "Imperium"
// becomes "Imperium (Harris novel)". Returns "" on failure.
func wikipediaResolve(ctx context.Context, query string) string {
	u := "https://en.wikipedia.org/w/api.php?" + url.Values{
		"action": {"query"}, "list": {"search"}, "srsearch": {query},
		"srlimit": {"1"}, "format": {"json"},
	}.Encode()
	var resp struct {
		Query struct {
			Search []struct {
				Title string `json:"title"`
			} `json:"search"`
		} `json:"query"`
	}
	if !getJSON(ctx, u, &resp) || len(resp.Query.Search) == 0 {
		return ""
	}
	return resp.Query.Search[0].Title
}

// seriesByChain reconstructs a series by walking Wikipedia infobox
// "preceded_by"/"followed_by" links starting from a seed article title. It is
// the fallback for series Wikidata doesn't model (many do not). Returns entries
// in reading order with sequential numbers. seedTitle should already be a
// resolved article title (see wikipediaResolve).
func seriesByChain(ctx context.Context, seedTitle string) []series.SeriesEntry {
	// Walk backwards to the first book, then forwards collecting titles.
	seen := map[string]bool{}
	// find the earliest predecessor
	start := seedTitle
	for i := 0; i < 12; i++ {
		prev := infoboxLink(ctx, start, rePrecededBy)
		if prev == "" || seen[prev] {
			break
		}
		seen[prev] = true
		start = prev
	}
	// walk forward
	var titles []string
	seen = map[string]bool{}
	cur := start
	for i := 0; i < 20 && cur != "" && !seen[cur]; i++ {
		seen[cur] = true
		titles = append(titles, cleanChainTitle(cur))
		cur = infoboxLink(ctx, cur, reFollowedBy)
	}
	if len(titles) < 2 {
		return nil // not a real chain
	}
	out := make([]series.SeriesEntry, len(titles))
	for i, t := range titles {
		out[i] = series.SeriesEntry{Title: t, Number: float64(i + 1)}
	}
	return out
}

var (
	rePrecededBy = regexp.MustCompile(`(?i)\|\s*preceded_by\s*=\s*\[\[([^\]|]+)`)
	reFollowedBy = regexp.MustCompile(`(?i)\|\s*followed_by\s*=\s*\[\[([^\]|]+)`)
)

// infoboxLink returns the linked article title for a given infobox field on a
// Wikipedia page (the part before any "|" in the wikilink), or "".
func infoboxLink(ctx context.Context, page string, re *regexp.Regexp) string {
	u := "https://en.wikipedia.org/w/api.php?" + url.Values{
		"action": {"parse"}, "page": {page}, "prop": {"wikitext"},
		"section": {"0"}, "format": {"json"},
	}.Encode()
	var resp struct {
		Parse struct {
			Wikitext struct {
				Text string `json:"*"`
			} `json:"wikitext"`
		} `json:"parse"`
	}
	if !getJSON(ctx, u, &resp) {
		return ""
	}
	m := re.FindStringSubmatch(resp.Parse.Wikitext.Text)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// cleanChainTitle strips a disambiguation suffix like " (novel)" for display.
func cleanChainTitle(s string) string {
	if i := strings.Index(s, " ("); i > 0 {
		return s[:i]
	}
	return s
}

// sparqlString quotes a value as a SPARQL string literal, escaping quotes and
// backslashes so a title with punctuation can't break the query.
func sparqlString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// --- Wikipedia infobox ------------------------------------------------------

var (
	reWpSeries = regexp.MustCompile(`(?i)\|\s*series\s*=\s*(.+)`)
	reWpNumber = regexp.MustCompile(`#\s*([0-9]+)`)
	reWpMarkup = regexp.MustCompile(`'{2,}|\[\[|\]\]|<[^>]+>`)
)

// lookupWikipedia fetches the lead section wikitext and reads the infobox's
// "series =" (and any "#N") — the field that placed Raybearer during design.
func lookupWikipedia(ctx context.Context, title string) series.Series {
	u := "https://en.wikipedia.org/w/api.php?" + url.Values{
		"action":  {"parse"},
		"page":    {title},
		"prop":    {"wikitext"},
		"section": {"0"},
		"format":  {"json"},
	}.Encode()

	var resp struct {
		Parse struct {
			Wikitext struct {
				Text string `json:"*"`
			} `json:"wikitext"`
		} `json:"parse"`
	}
	if !getJSON(ctx, u, &resp) {
		return series.Series{}
	}
	wt := resp.Parse.Wikitext.Text
	m := reWpSeries.FindStringSubmatch(wt)
	if m == nil {
		return series.Series{}
	}
	raw := m[1]
	var num float64
	if nm := reWpNumber.FindStringSubmatch(raw); nm != nil {
		fmt.Sscanf(nm[1], "%g", &num)
	}
	// Strip wiki markup and the trailing "#N" to get a clean series name.
	name := reWpMarkup.ReplaceAllString(raw, "")
	name = reWpNumber.ReplaceAllString(name, "")
	name = strings.Trim(strings.TrimSpace(name), " |")
	if name == "" {
		return series.Series{}
	}
	return series.Series{Name: name, Number: num, Source: series.SourceWikipedia}
}

// --- helpers ----------------------------------------------------------------

func getJSON(ctx context.Context, u string, v any) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	return json.NewDecoder(resp.Body).Decode(v) == nil
}

var reQueryStrip = regexp.MustCompile(`(?i)\s*[(\[].*$|\s*#\s*[0-9]+.*$|[:,]\s*book\s+\w+.*$`)

func cleanQueryTitle(title string) string {
	t := reQueryStrip.ReplaceAllString(title, "")
	return strings.TrimSpace(t)
}
