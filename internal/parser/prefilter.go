package parser

import (
	"regexp"
	"regexp/syntax"
	"sort"
	"strings"
)

// This file implements the RE2 prefilter gate. Every database pattern gets a
// companion stdlib-regexp (RE2) version that is a SUPERSET of the regexp2
// pattern's "does it match?" semantics: if the RE2 gate says no, the regexp2
// engine cannot match either, so the expensive backtracking run is skipped.
// If the gate says yes (or the pattern cannot be gated), regexp2 runs and
// remains the single source of truth for match results and capture groups —
// parity is preserved by construction, not by auditing thousands of patterns.
//
// RE2 matches in time linear in the input regardless of pattern size, which
// collapses the aggregate walk cost on junk input (the ReDoS long tail) and
// removes most negative-match cost on real traffic.
//
// Superset construction rules (.NET regexp2 semantics -> RE2):
//   - compiled WITHOUT the uaAnchor and with (?m): anchoring only narrows
//     matches, and (?m) makes ^/$ per-line, a superset of .NET's "$ also
//     matches before a trailing newline".
//   - \d and \w are ASCII-only in RE2 but Unicode in .NET -> widened to the
//     .NET-equivalent Unicode classes. \s likewise. Their negations (\D \W
//     \S) are already wider in RE2 and stay as-is.
//   - patterns using constructs whose RE2 semantics could be NARROWER and
//     cannot be widened mechanically (word boundaries \b/\B, which depend on
//     the \w alphabet) are not gated.
//   - anything RE2 fails to compile (lookaround, backreferences, atomic
//     groups, possessive quantifiers, .NET named-group syntax not translated
//     below) is not gated: those patterns always run the full regexp2 path.

// re2Translate rewrites a database pattern (already normalizePattern'd) into
// its RE2 superset form, or reports ok=false when the pattern must not be
// gated.
func re2Translate(p string) (string, bool) {
	// Word boundaries depend on the \w alphabet, which is narrower in RE2;
	// a mechanical widening does not exist, so skip the gate entirely.
	if strings.Contains(p, `\b`) || strings.Contains(p, `\B`) {
		return "", false
	}

	var b strings.Builder

	b.Grow(len(p) + 16)

	// The widened classes are emitted bracketed outside a character class and
	// as bare member lists inside one ([\d.] must become [0-9\p{Nd}.], not a
	// nested-bracket mangle).
	const (
		dClass = `0-9\p{Nd}`                         // .NET \d = Unicode decimal digits
		wClass = `0-9A-Za-z_\p{L}\p{Mn}\p{Nd}\p{Pc}` // .NET \w = letters/marks/digits/connector
		sClass = `\t\n\v\f\r \p{Z}\x{0085}`          // .NET \s adds Unicode separators
	)

	inClass := false

	for i := 0; i < len(p); i++ {
		c := p[i]

		if c == '\\' && i+1 < len(p) {
			var widened string

			switch p[i+1] {
			case 'd':
				widened = dClass
			case 'w':
				widened = wClass
			case 's':
				widened = sClass
			}

			if widened != "" {
				if inClass {
					b.WriteString(widened)
				} else {
					b.WriteString(`[` + widened + `]`)
				}
			} else {
				b.WriteByte(c)
				b.WriteByte(p[i+1])
			}

			i++

			continue
		}

		if c == '[' && !inClass {
			inClass = true
		} else if c == ']' && inClass {
			inClass = false
		}

		// Translate .NET named groups "(?<name>" / "(?'name'" to RE2 "(?P<name>".
		// Lookbehind "(?<=" / "(?<!" must stay untouched so RE2 rejects it.
		if c == '(' && i+2 < len(p) && p[i+1] == '?' {
			next := p[i+2]
			if (next == '<' && i+3 < len(p) && p[i+3] != '=' && p[i+3] != '!') || next == '\'' {
				b.WriteString("(?P<")

				closer := byte('>')
				if next == '\'' {
					closer = '\''
				}

				j := i + 3
				for j < len(p) && p[j] != closer {
					b.WriteByte(p[j])
					j++
				}

				b.WriteByte('>')

				i = j // consume up to and including the closer

				continue
			}
		}

		b.WriteByte(c)
	}

	return b.String(), true
}

// GateSet narrows an entry walk (device brands, OS rules, browsers — parsers
// with no upstream preMatchOverall) using required literals. For each entry a
// set of case-folded literals is extracted from its regex AST such that a
// match implies at least one literal is present in the user agent. At parse
// time one lowercased copy of the UA and a substring probe per distinct
// literal shrink the walk to the entries whose literal actually occurs (plus
// the few entries no literal set could be proven for). This is what collapses
// the aggregate walk on junk input — and most of the walk on real traffic —
// while remaining a strict superset of the regexp2 match semantics.
//
// (A single combined RE2 union was tried first and was slower than the
// per-pattern walk it replaced: Go's regexp simulates an NFA, so one pass
// over a ~20k-state union costs more than thousands of memchr-accelerated
// substring probes.)
type GateSet struct {
	// literals maps each distinct lowercased literal to the entries it can
	// admit. Probed with strings.Contains against the lowercased UA.
	literals map[string][]int
	// always holds entries with no provable literal set — walked every time.
	always []int
	n      int
}

// CompileGateSet builds a literal gate over patterns; entries whose regex
// yields no usable literal set are recorded as always-walk.
func CompileGateSet(patterns []string) *GateSet {
	g := &GateSet{literals: make(map[string][]int), n: len(patterns)}

	for i, p := range patterns {
		lits, ok := requiredLiterals(p)
		if !ok {
			g.always = append(g.always, i)

			continue
		}

		for _, lit := range lits {
			g.literals[lit] = append(g.literals[lit], i)
		}
	}

	return g
}

// minGateLiteral keeps probe literals selective; branches whose best literal
// is shorter go to the always-walk list instead of admitting most inputs.
const minGateLiteral = 3

// requiredLiterals extracts a literal cover for one database pattern: the
// pattern can only match a UA containing at least one returned literal
// (lowercased). ok=false when no such cover can be proven.
func requiredLiterals(pattern string) ([]string, bool) {
	t, ok := re2Translate(normalizePattern(pattern))
	if !ok {
		return nil, false
	}

	ast, err := syntax.Parse(`(?i)(?:`+t+`)`, syntax.Perl)
	if err != nil {
		return nil, false // lookaround etc: RE2 syntax cannot express it
	}

	lits, ok := literalCover(ast)
	if !ok {
		return nil, false
	}

	for i := range lits {
		if len(lits[i]) < minGateLiteral {
			return nil, false
		}

		lits[i] = strings.ToLower(lits[i])
	}

	return lits, true
}

// literalCover walks a parsed regex and returns literals such that any match
// must contain at least one of them.
func literalCover(re *syntax.Regexp) ([]string, bool) {
	switch re.Op {
	case syntax.OpLiteral:
		return []string{string(re.Rune)}, true

	case syntax.OpCapture:
		return literalCover(re.Sub[0])

	case syntax.OpPlus:
		// The body occurs at least once.
		return literalCover(re.Sub[0])

	case syntax.OpRepeat:
		if re.Min >= 1 {
			return literalCover(re.Sub[0])
		}

		return nil, false

	case syntax.OpConcat:
		// Any mandatory component covers the whole concat; pick the one with
		// the longest shortest-literal (most selective probe).
		var best []string

		bestScore := -1

		for _, sub := range re.Sub {
			lits, ok := literalCover(sub)
			if !ok {
				continue
			}

			score := len(lits[0])
			for _, l := range lits {
				if len(l) < score {
					score = len(l)
				}
			}

			if score > bestScore {
				best, bestScore = lits, score
			}
		}

		return best, best != nil

	case syntax.OpAlternate:
		// Every branch must contribute, else a literal-free branch could
		// match without any probe hitting.
		var all []string

		for _, sub := range re.Sub {
			lits, ok := literalCover(sub)
			if !ok {
				return nil, false
			}

			all = append(all, lits...)
		}

		return all, true

	default:
		// Char classes, anchors, stars, empty matches: no literal to prove.
		return nil, false
	}
}

// SkipGated reports whether the walk can be narrowed for ua. When it returns
// (only, true), entries outside `only` cannot match and the caller walks just
// those indexes (sorted, possibly empty). (nil, false) means walk everything.
func (g *GateSet) SkipGated(ua string) ([]int, bool) {
	if g == nil || g.literals == nil {
		return nil, false
	}

	lower := strings.ToLower(ua)

	// Collect survivors: always-walk entries plus entries admitted by a
	// literal present in the UA.
	seen := make(map[int]struct{}, len(g.always)+8)
	for _, i := range g.always {
		seen[i] = struct{}{}
	}

	for lit, idxs := range g.literals {
		if strings.Contains(lower, lit) {
			for _, i := range idxs {
				seen[i] = struct{}{}
			}
		}
	}

	if len(seen) == g.n {
		return nil, false
	}

	only := make([]int, 0, len(seen))
	for i := range seen {
		only = append(only, i)
	}

	sort.Ints(only)

	return only, true
}

// GateStats reports how many cached patterns carry an RE2 gate. Diagnostic
// only (used by tests and benchmarks to prove coverage).
func GateStats() (gated, total int) {
	regexCache.Range(func(_, v any) bool {
		total++

		if v.(*Compiled).gate != nil {
			gated++
		}

		return true
	})

	return gated, total
}

// compileGate builds the RE2 gate for a raw database pattern, or nil when the
// pattern cannot be gated (regexp2 then always runs). Case-insensitive and
// multiline per the superset rules; unanchored on purpose.
func compileGate(pattern string) *regexp.Regexp {
	translated, ok := re2Translate(normalizePattern(pattern))
	if !ok {
		return nil
	}

	gate, err := regexp.Compile(`(?im)(?:` + translated + `)`)
	if err != nil {
		return nil // lookaround, backrefs, PCRE-isms: full regexp2 path
	}

	return gate
}
