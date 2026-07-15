package parser

import (
	"strings"
	"sync"

	"github.com/dlclark/regexp2"
)

// uaAnchor mirrors AbstractParser::matchUserAgent(): a pattern only matches
// at the start of the user agent or when not preceded by a letter/digit,
// with the historical sprd-/MZ- exceptions.
const uaAnchor = `(?:^|[^A-Z0-9_-]|[^A-Z0-9-]_|sprd-|MZ-)`

var regexCache sync.Map // string -> *regexp2.Regexp

// normalizePattern rewrites PCRE escapes that the regexp2 (.NET) dialect
// rejects: `\_` is a literal underscore in PCRE but a compile error in
// regexp2. Escape pairs are consumed left-to-right, so a literal backslash
// followed by an underscore (`\\_`) is left intact.
func normalizePattern(p string) string {
	if !strings.Contains(p, `\_`) {
		return p
	}

	var b strings.Builder

	b.Grow(len(p))

	for i := 0; i < len(p); i++ {
		if p[i] == '\\' && i+1 < len(p) {
			if p[i+1] == '_' {
				b.WriteByte('_')
			} else {
				b.WriteByte(p[i])
				b.WriteByte(p[i+1])
			}

			i++

			continue
		}

		b.WriteByte(p[i])
	}

	return b.String()
}

// Compile compiles a raw regex from the database wrapped with the standard
// user-agent anchor, case-insensitively. Compiled patterns are cached
// process-wide.
func Compile(pattern string) (*regexp2.Regexp, error) {
	if cached, ok := regexCache.Load(pattern); ok {
		return cached.(*regexp2.Regexp), nil
	}

	re, err := regexp2.Compile(uaAnchor+`(?:`+normalizePattern(pattern)+`)`, regexp2.IgnoreCase)
	if err != nil {
		return nil, err
	}

	regexCache.Store(pattern, re)

	return re, nil
}

// MatchUserAgent matches ua against a database pattern and returns the
// capture groups PHP-style: index 0 is the full match, unmatched groups are
// empty strings. Returns nil when there is no match.
func MatchUserAgent(ua, pattern string) ([]string, error) {
	re, err := Compile(pattern)
	if err != nil {
		return nil, err
	}

	return matchWith(re, ua)
}

func matchWith(re *regexp2.Regexp, ua string) ([]string, error) {
	m, err := re.FindStringMatch(ua)
	if err != nil || m == nil {
		return nil, err
	}

	groups := m.Groups()
	matches := make([]string, len(groups))

	for i := range groups {
		matches[i] = groups[i].String()
	}

	return matches, nil
}

// CombineRegexes builds the AbstractParser::preMatchOverall() alternation:
// all patterns reversed (generic entries last in the file match most UAs)
// and joined with '|'.
func CombineRegexes(patterns []string) string {
	reversed := make([]string, 0, len(patterns))
	for i := len(patterns) - 1; i >= 0; i-- {
		reversed = append(reversed, patterns[i])
	}

	return strings.Join(reversed, "|")
}
