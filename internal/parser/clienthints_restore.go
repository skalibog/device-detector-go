package parser

import (
	"regexp"
	"strings"
)

var (
	// uaCHFragmentRe matches a frozen "Android 10; K"-style user agent that
	// client hints can restore. These patterns use no lookaround, so the
	// linear-time engine is fine (they run on the raw UA, like PHP preg_match).
	uaCHFragmentRe   = regexp.MustCompile(`(?i)Android (?:1[0-6][.\d]*; K(?: Build/|[;)])|1[0-6]\)) AppleWebKit`)
	androidRestoreRe = regexp.MustCompile(`Android (?:10[.\d]*; K|1[1-5])`)
	desktopRestoreRe = regexp.MustCompile(`X11; Linux x86_64`)
)

// HasUserAgentClientHintsFragment reports whether ua carries a frozen
// Android client-hints fragment, mirroring
// AbstractParser::hasUserAgentClientHintsFragment. Telegram-Android reuses the
// same shape and is excluded.
func HasUserAgentClientHintsFragment(ua string) bool {
	if !uaCHFragmentRe.MatchString(ua) {
		return false
	}

	return !strings.Contains(strings.ToLower(ua), "telegram-android/")
}

// phpAtoi mirrors PHP's (int) cast of a string: an optional sign then the
// leading run of digits, yielding 0 when there is no leading number.
func phpAtoi(s string) int {
	s = strings.TrimLeft(s, " \t\n\r\v\f")

	i, neg := 0, false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}

	v, got := 0, false
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		v = v*10 + int(s[i]-'0')
		got = true
	}

	if !got {
		return 0
	}

	if neg {
		return -v
	}

	return v
}

// afterFirstDot returns the substring after the first '.', or "".
func afterFirstDot(s string) string {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return s[i+1:]
	}

	return ""
}

func containsStr(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}

	return false
}

// RestoreUserAgent rebuilds a frozen user agent from the device model reported
// via client hints, mirroring AbstractParser::restoreUserAgentFromClientHints.
// It returns ua unchanged when there is nothing to restore.
func (c *ClientHints) RestoreUserAgent(ua string) string {
	if c == nil || c.Model == "" {
		return ua
	}

	if HasUserAgentClientHintsFragment(ua) {
		osVersion := c.PlatformVersion
		if osVersion == "" {
			osVersion = "10"
		}

		ua = androidRestoreRe.ReplaceAllString(ua, "Android "+osVersion+"; "+c.Model)
	}

	if HasDesktopFragment(ua) {
		ua = desktopRestoreRe.ReplaceAllString(ua, "X11; Linux x86_64; "+c.Model)
	}

	return ua
}
