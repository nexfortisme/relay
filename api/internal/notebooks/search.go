package notebooks

import (
	"sort"
	"strings"
	"unicode"
)

func buildFTSMatchQuery(query string) string {
	tokens := tokenSet(query)
	if len(tokens) == 0 {
		return ""
	}

	terms := make([]string, 0, len(tokens))
	for token := range tokens {
		terms = append(terms, `"`+strings.ReplaceAll(token, `"`, `""`)+`"`)
	}
	sort.Strings(terms)
	if len(terms) > 12 {
		terms = terms[:12]
	}
	return strings.Join(terms, " OR ")
}

// tokenSet builds a set of lowercase tokens (3+ chars) from s for FTS queries.
func tokenSet(s string) map[string]struct{} {
	out := make(map[string]struct{})
	var tok strings.Builder
	flush := func() {
		if tok.Len() >= 3 {
			out[strings.ToLower(tok.String())] = struct{}{}
		}
		tok.Reset()
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			tok.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}
