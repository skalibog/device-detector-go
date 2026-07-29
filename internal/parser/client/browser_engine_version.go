package client

import (
	"sync"

	"github.com/dlclark/regexp2"
)

// geckoVersionRegex matches the "rv:" version reported by Gecko/Clecko engines.
// Ported verbatim from Engine\Version::parse(); unlike the client regexes it is
// applied to the raw user agent without the standard anchor.
var geckoVersionRegex = regexp2.MustCompile(
	`[ ](?:rv[: ]([0-9.]+)).*(?:g|cl)ecko/[0-9]{8,10}`,
	regexp2.IgnoreCase,
)

// engineVersionRegexes caches the per-engine-token version regexes.
var engineVersionRegexes sync.Map // token string -> *regexp2.Regexp

// buildEngineVersion ports Engine\Version::parse(): it extracts the engine
// version for the given engine from the user agent, returning "" when unknown.
func buildEngineVersion(engine, ua string) (string, error) {
	if engine == "" {
		return "", nil
	}

	if engine == "Gecko" || engine == "Clecko" {
		m, err := geckoVersionRegex.FindStringMatch(ua)
		if err != nil {
			return "", err
		}

		if m != nil {
			return lastGroup(m), nil
		}
	}

	token := engine

	switch engine {
	case "Blink":
		token = `Chr[o0]me|Chromium|Cronet`
	case "Arachne":
		token = `Arachne\/5\.`
	case "LibWeb":
		token = `LibWeb\+LibJs`
	}

	re, err := engineVersionRegex(token)
	if err != nil {
		return "", err
	}

	m, err := re.FindStringMatch(ua)
	if err != nil {
		return "", err
	}

	if m == nil {
		return "", nil
	}

	return lastGroup(m), nil
}

func engineVersionRegex(token string) (*regexp2.Regexp, error) {
	if cached, ok := engineVersionRegexes.Load(token); ok {
		return cached.(*regexp2.Regexp), nil
	}

	pattern := `(?:` + token + `)\s*[/_]?\s*((?(?=\d+\.\d)\d+[.\d]*|\d{1,7}(?=(?:\D|$))))`

	re, err := regexp2.Compile(pattern, regexp2.IgnoreCase)
	if err != nil {
		return nil, err
	}

	engineVersionRegexes.Store(token, re)

	return re, nil
}

// lastGroup returns the last capture group of a match, mirroring PHP's
// array_pop($matches) on the preg_match result.
func lastGroup(m *regexp2.Match) string {
	groups := m.Groups()

	return groups[len(groups)-1].String()
}
