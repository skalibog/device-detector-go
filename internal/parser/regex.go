package parser

import (
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dlclark/regexp2"
)

// uaAnchor mirrors AbstractParser::matchUserAgent(): a pattern only matches
// at the start of the user agent or when not preceded by a letter/digit,
// with the historical sprd-/MZ- exceptions.
const uaAnchor = `(?:^|[^A-Z0-9_-]|[^A-Z0-9-]_|sprd-|MZ-)`

// DefaultMatchTimeout is the per-match backstop stamped onto every compiled
// pattern. regexp2 is a backtracking engine, and the database mirrors the
// upstream design of one large alternation per file; without a bound, a
// crafted user agent can pin a core for tens of seconds. A single legitimate
// match never approaches this, so the value only ever fires on pathological
// input. Tune with SetMatchTimeout before constructing detectors.
const DefaultMatchTimeout = time.Second

// matchTimeoutNanos is the process-wide per-match timeout in nanoseconds,
// read concurrently while lazily-compiled patterns are built. 0 disables it.
var matchTimeoutNanos atomic.Int64

func init() { matchTimeoutNanos.Store(int64(DefaultMatchTimeout)) }

// SetMatchTimeout sets the per-match timeout applied to patterns compiled
// afterwards; d <= 0 disables it. The regex cache is process-wide and a
// pattern keeps the timeout in effect when it was first compiled, so call
// this once at startup, before constructing detectors, for a uniform bound.
func SetMatchTimeout(d time.Duration) {
	if d < 0 {
		d = 0
	}

	matchTimeoutNanos.Store(int64(d))
}

// MatchTimeout returns the current per-match timeout (0 if disabled).
func MatchTimeout() time.Duration { return time.Duration(matchTimeoutNanos.Load()) }

// Compiled pairs the authoritative regexp2 pattern with its optional RE2
// prefilter gate (nil when the pattern cannot be gated — see prefilter.go).
type Compiled struct {
	full *regexp2.Regexp
	gate *regexp.Regexp
}

// Full exposes the underlying regexp2 for callers that manage matching
// themselves (timeout stamping, direct FindStringMatch).
func (c *Compiled) Full() *regexp2.Regexp { return c.full }

var regexCache sync.Map // string -> *Compiled

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
// process-wide and carry the current match timeout.
//
// An empty pattern deliberately matches every user agent: the database uses
// empty model regexes as catch-alls (e.g. Roku's "Digital Video Player").
// The empty-list guard for preMatchOverall lives in its callers, not here —
// see PreMatchEmpty.
func Compile(pattern string) (*Compiled, error) {
	if cached, ok := regexCache.Load(pattern); ok {
		return cached.(*Compiled), nil
	}

	re, err := regexp2.Compile(uaAnchor+`(?:`+normalizePattern(pattern)+`)`, regexp2.IgnoreCase)
	if err != nil {
		return nil, err
	}

	StampTimeout(re)

	c := &Compiled{full: re, gate: compileGate(pattern)}

	regexCache.Store(pattern, c)

	return c, nil
}

// StampTimeout applies the current match timeout to re. Call before publishing
// a regexp for concurrent matching; the field is then read-only and race-free.
func StampTimeout(re *regexp2.Regexp) {
	if t := MatchTimeout(); t > 0 {
		re.MatchTimeout = t
	}
}

// MatchUserAgent matches ua against a database pattern and returns the
// capture groups PHP-style: index 0 is the full match, unmatched groups are
// empty strings. Returns nil when there is no match.
//
// An empty pattern matches every user agent, mirroring PHP preg_match on an
// empty alternation; the database relies on this for catch-all model regexes.
// Callers that build a preMatchOverall regex must reject the empty-list case
// themselves via PreMatchEmpty before matching.
func MatchUserAgent(ua, pattern string) ([]string, error) {
	re, err := Compile(pattern)
	if err != nil {
		return nil, err
	}

	return matchWith(re, ua)
}

func matchWith(c *Compiled, ua string) ([]string, error) {
	// The RE2 gate is a strict superset of the regexp2 pattern's match
	// semantics: a gate miss proves the full pattern cannot match, skipping
	// the backtracking engine entirely. Linear in len(ua) regardless of
	// pattern size.
	if c.gate != nil && !c.gate.MatchString(ua) {
		return nil, nil
	}

	m, err := c.full.FindStringMatch(ua)
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

// PreMatchEmpty reports whether a combined preMatchOverall regex is empty and
// must therefore be treated as no-match. matomo/device-detector 6.5.1
// (PR #8271) added this guard so an empty regex list no longer degrades into a
// bare anchor that matches every user agent. Individual empty patterns keep
// their catch-all semantics; only the combined-list case is guarded.
func PreMatchEmpty(combined string) bool { return combined == "" }
