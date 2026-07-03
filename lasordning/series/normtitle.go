package series

import "strings"

// normTitle normalizes a title for fuzzy matching between the device's title
// text and an online source's canonical title: lowercase, drop a leading
// article, strip punctuation and any trailing "(series ...)" / "#N" noise, and
// collapse whitespace.
func normTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// cut a trailing parenthetical or "#n" / ": book n" suffix
	if i := strings.IndexAny(s, "("); i > 0 {
		s = s[:i]
	}
	if i := strings.Index(s, " #"); i > 0 {
		s = s[:i]
	}
	// drop leading article
	for _, art := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(s, art) {
			s = s[len(art):]
			break
		}
	}
	// keep only alnum + spaces
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
