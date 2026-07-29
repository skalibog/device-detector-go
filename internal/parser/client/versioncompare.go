package client

import "strings"

// phpVersionCompare mirrors PHP's version_compare($v1, $v2) (three-way, with
// operator omitted), returning -1, 0 or 1. Browser.buildEngine relies on it to
// pick an engine by version threshold.
//
// Both operands are canonicalized into digit/non-digit segments; segments are
// then compared numerically when both are numeric, otherwise by PHP's special
// version-form ordering (any-other < dev < alpha/a < beta/b < RC/rc < # < pl/p).
func phpVersionCompare(v1, v2 string) int {
	a := canonicalizeVersion(v1)
	b := canonicalizeVersion(v2)

	n := len(a)
	if len(b) < n {
		n = len(b)
	}

	for i := 0; i < n; i++ {
		if c := slotCompare(a[i], b[i]); c != 0 {
			return c
		}
	}

	switch {
	case len(a) > len(b):
		return tailSign(a[len(b)], 1)
	case len(b) > len(a):
		return tailSign(b[len(a)], -1)
	default:
		return 0
	}
}

// canonicalizeVersion splits a version into segments the way PHP's
// php_canonicalize_version does: separators (. - _ +) delimit segments and a
// boundary is inserted between digit and non-digit runs.
func canonicalizeVersion(v string) []string {
	var (
		segs []string
		cur  []byte
		last int // 0 = separator/start, 1 = digit, 2 = other
	)

	flush := func() {
		if len(cur) > 0 {
			segs = append(segs, string(cur))
			cur = cur[:0]
		}
	}

	for i := 0; i < len(v); i++ {
		c := v[i]

		switch {
		case c == '.' || c == '-' || c == '_' || c == '+':
			flush()
			last = 0
		case c >= '0' && c <= '9':
			if last == 2 {
				flush()
			}
			cur = append(cur, c)
			last = 1
		default:
			if last == 1 {
				flush()
			}
			cur = append(cur, c)
			last = 2
		}
	}

	flush()

	return segs
}

// tailSign resolves the comparison when one operand has extra trailing
// segments. A numeric leftover makes the longer operand greater; a non-numeric
// leftover is compared against the numeric marker "#". longerSign is +1 when
// the first operand is the longer one and -1 otherwise.
func tailSign(seg string, longerSign int) int {
	if isAllDigits(seg) {
		return longerSign
	}

	c := specialFormOrder(seg) - specialFormOrder("#")

	switch {
	case c < 0:
		return -longerSign
	case c > 0:
		return longerSign
	default:
		return 0
	}
}

// slotCompare compares two canonical segments.
func slotCompare(s1, s2 string) int {
	d1 := isAllDigits(s1)
	d2 := isAllDigits(s2)

	if d1 && d2 {
		return numericCompare(s1, s2)
	}

	f1, f2 := s1, s2
	if d1 {
		f1 = "#"
	}
	if d2 {
		f2 = "#"
	}

	switch o1, o2 := specialFormOrder(f1), specialFormOrder(f2); {
	case o1 < o2:
		return -1
	case o1 > o2:
		return 1
	default:
		return 0
	}
}

// numericCompare compares two all-digit segments by magnitude, ignoring the
// (bounded) length of the strings.
func numericCompare(s1, s2 string) int {
	s1 = strings.TrimLeft(s1, "0")
	s2 = strings.TrimLeft(s2, "0")

	switch {
	case len(s1) < len(s2):
		return -1
	case len(s1) > len(s2):
		return 1
	case s1 < s2:
		return -1
	case s1 > s2:
		return 1
	default:
		return 0
	}
}

// specialFormOrder returns PHP's ordering weight for a version special form.
// Unrecognized forms rank below every recognized one, as in PHP.
func specialFormOrder(form string) int {
	switch strings.ToLower(form) {
	case "dev":
		return 0
	case "alpha", "a":
		return 1
	case "beta", "b":
		return 2
	case "rc":
		return 3
	case "#":
		return 4
	case "pl", "p":
		return 5
	default:
		return -1
	}
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}

	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}
