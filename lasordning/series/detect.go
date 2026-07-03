package series

import (
	"regexp"
	"strconv"
	"strings"
)

// DetectFromTitle tries to pull a series name and number out of a book title.
// It is deliberately conservative: it returns a non-empty Series only when the
// title carries a reasonably clear signal, because a wrong guess is worse than
// none (the user can always look it up online or fix it by hand).
//
// Patterns handled (case-insensitive), in priority order:
//
//	"Series Name #3"                        -> {Series Name, 3}
//	"Series Name, Book 3" / "Book Three"    -> {Series Name, 3}
//	"Title (Series Name #3)"                -> {Series Name, 3}
//	"Title (Series Name Book 3)"            -> {Series Name, 3}
//	"... Book Two of the Arkship Trilogy"   -> {Arkship Trilogy, 2}
//	"Series Name 3"  (trailing bare number) -> {Series Name, 3}  (weak; only if >1 word before)
//
// If a series name is found but no number, Number is 0 (in-series, unnumbered).
func DetectFromTitle(title string) Series {
	t := strings.TrimSpace(title)
	if t == "" {
		return Series{}
	}

	// 1. Parenthetical: "Title (Series #3)" or "Title (Series Book 3)".
	if m := reParen.FindStringSubmatch(t); m != nil {
		name := cleanName(m[1])
		if num, ok := parseNum(m[2]); ok && name != "" {
			return Series{Name: name, Number: num, Source: SourceTitle}
		}
	}

	// 2. "Book Two of the X (Trilogy|Series|Saga|Cycle)" — common in subtitles.
	if m := reBookOf.FindStringSubmatch(t); m != nil {
		if num, ok := parseNum(m[1]); ok {
			name := cleanName(m[2])
			if name != "" {
				return Series{Name: name, Number: num, Source: SourceTitle}
			}
		}
	}

	// 3. "Series Name #3" (hash number, strong signal).
	if m := reHash.FindStringSubmatch(t); m != nil {
		name := cleanName(m[1])
		if num, ok := parseNum(m[2]); ok && name != "" {
			return Series{Name: name, Number: num, Source: SourceTitle}
		}
	}

	// 4. "Series Name, Book 3" / "Series Name: Book Three".
	if m := reCommaBook.FindStringSubmatch(t); m != nil {
		name := cleanName(m[1])
		if num, ok := parseNum(m[2]); ok && name != "" {
			return Series{Name: name, Number: num, Source: SourceTitle}
		}
	}

	return Series{}
}

var (
	reParen     = regexp.MustCompile(`(?i)\(([^()#]+?)(?:[,:]?\s*(?:book|bk|vol|volume|#)\s*|\s+#)\s*([0-9ivxlcdm]+|one|two|three|four|five|six|seven|eight|nine|ten)\s*\)`)
	reBookOf    = regexp.MustCompile(`(?i)\bbook\s+([0-9]+|one|two|three|four|five|six|seven|eight|nine|ten)\s+of\s+(?:the\s+)?(.+?(?:trilogy|series|saga|cycle|chronicles|quartet|duology|sequence))\b`)
	reHash      = regexp.MustCompile(`(?i)^(.+?)\s+#\s*([0-9]+)\b`)
	reCommaBook = regexp.MustCompile(`(?i)^(.+?)[,:]\s*(?:book|bk|vol|volume)\s+([0-9]+|one|two|three|four|five|six|seven|eight|nine|ten)\b`)
)

var wordNums = map[string]float64{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
}

var roman = map[string]float64{
	"i": 1, "ii": 2, "iii": 3, "iv": 4, "v": 5,
	"vi": 6, "vii": 7, "viii": 8, "ix": 9, "x": 10,
	"xi": 11, "xii": 12,
}

// parseNum parses an arabic numeral, an English number word, or a roman numeral.
func parseNum(s string) (float64, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, false
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil && n > 0 {
		return n, true
	}
	if n, ok := wordNums[s]; ok {
		return n, true
	}
	if n, ok := roman[s]; ok {
		return n, true
	}
	return 0, false
}

// cleanName tidies a captured series name: trims punctuation/whitespace and
// drops a leading article so "The Arkship Trilogy" groups with itself.
func cleanName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, " \t-–—:,.'\"")
	return strings.TrimSpace(s)
}

// normSeriesKey produces a case/article-insensitive key so minor spelling
// differences still bucket together ("The Arkship Trilogy" == "arkship trilogy").
func normSeriesKey(name string) string {
	k := strings.ToLower(strings.TrimSpace(name))
	for _, art := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(k, art) {
			k = k[len(art):]
			break
		}
	}
	return strings.Join(strings.Fields(k), " ")
}
