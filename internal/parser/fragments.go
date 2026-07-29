package parser

import "strings"

var desktopFragmentExcludes = strings.Join([]string{
	`CE-HTML`,
	` Mozilla/|Andr[o0]id|Tablet|Mobile|iPhone|Windows Phone|ricoh|OculusBrowser`,
	`PicoBrowser|Lenovo|compatible; MSIE|Trident/|Tesla/|XBOX|FBMD/|ARM; ?([^)]+)`,
}, "|")

// HasDesktopFragment mirrors AbstractParser::hasDesktopFragment(): the UA
// carries a desktop OS fragment and none of the known mobile/TV markers.
func HasDesktopFragment(ua string) bool {
	desktop, err := MatchUserAgent(ua, `(?:Windows (?:NT|IoT)|X11; Linux x86_64)`)
	if err != nil || desktop == nil {
		return false
	}

	excluded, err := MatchUserAgent(ua, desktopFragmentExcludes)

	return err == nil && excluded == nil
}
