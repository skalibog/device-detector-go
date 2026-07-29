package parser

import (
	"net/http"
	"regexp"
	"strings"
)

// BrandVersion is one entry of a Sec-CH-UA brand/version list.
type BrandVersion struct {
	Brand   string
	Version string
}

// ClientHints holds the parsed HTTP Client Hints for a request, mirroring
// DeviceDetector\ClientHints. The zero value carries no hints.
type ClientHints struct {
	Architecture    string
	Bitness         string
	Mobile          bool
	Model           string
	Platform        string
	PlatformVersion string
	UAFullVersion   string
	FullVersionList []BrandVersion
	App             string
	FormFactors     []string
}

// IsMobile reports the Sec-CH-UA-Mobile hint.
func (c *ClientHints) IsMobile() bool { return c != nil && c.Mobile }

// BrandList returns the brand/version pairs, de-duplicated by brand keeping the
// first position and the last version (mirroring PHP array_combine).
func (c *ClientHints) BrandList() []BrandVersion {
	if c == nil {
		return nil
	}

	pos := make(map[string]int, len(c.FullVersionList))
	out := make([]BrandVersion, 0, len(c.FullVersionList))

	for _, bv := range c.FullVersionList {
		if i, ok := pos[bv.Brand]; ok {
			out[i].Version = bv.Version
			continue
		}

		pos[bv.Brand] = len(out)
		out = append(out, bv)
	}

	return out
}

// BrandVersion returns the Sec-CH-UA-Full-Version hint (getBrandVersion).
func (c *ClientHints) BrandVersion() string {
	if c == nil {
		return ""
	}

	return c.UAFullVersion
}

// NewClientHintsFromHeaders builds ClientHints from HTTP request headers.
func NewClientHintsFromHeaders(h http.Header) *ClientHints {
	m := make(map[string]any, len(h))
	for name, vals := range h {
		m[name] = strings.Join(vals, ", ")
	}

	return NewClientHintsFromMap(m)
}

// NewClientHintsFromMap builds ClientHints from a header/value map. Values may
// be strings (HTTP headers) or structured values from navigator.userAgentData
// (bool mobile, []string form factors, a brand/version list). It mirrors
// ClientHints::factory, including its strict per-field type handling.
func NewClientHintsFromMap(headers map[string]any) *ClientHints {
	c := &ClientHints{}

	for name, value := range headers {
		if isEmptyHint(value) {
			continue
		}

		// Any HTML tag is unexpected in these headers; discard such values.
		if s, ok := value.(string); ok && htmlTagRe.MatchString(s) {
			continue
		}

		switch strings.ReplaceAll(strings.ToLower(name), "_", "-") {
		case "http-sec-ch-ua-arch", "sec-ch-ua-arch", "arch", "architecture":
			if s, ok := value.(string); ok {
				c.Architecture = strings.Trim(s, `"`)
			}
		case "http-sec-ch-ua-bitness", "sec-ch-ua-bitness", "bitness":
			if s, ok := value.(string); ok {
				c.Bitness = strings.Trim(s, `"`)
			}
		case "http-sec-ch-ua-mobile", "sec-ch-ua-mobile", "mobile":
			c.Mobile = value == true || value == "1" || value == "?1"
		case "http-sec-ch-ua-model", "sec-ch-ua-model", "model":
			if s, ok := value.(string); ok {
				c.Model = strings.Trim(s, `"`)
			}
		case "http-sec-ch-ua-full-version", "sec-ch-ua-full-version", "uafullversion":
			if s, ok := value.(string); ok {
				c.UAFullVersion = strings.Trim(s, `"`)
			}
		case "http-sec-ch-ua-platform", "sec-ch-ua-platform", "platform":
			if s, ok := value.(string); ok {
				c.Platform = strings.Trim(s, `"`)
			}
		case "http-sec-ch-ua-platform-version", "sec-ch-ua-platform-version", "platformversion":
			if s, ok := value.(string); ok {
				c.PlatformVersion = strings.Trim(s, `"`)
			}
		case "brands":
			if len(c.FullVersionList) == 0 {
				if list, ok := toBrandList(value); ok {
					c.FullVersionList = list
				}
			}
		case "fullversionlist":
			if list, ok := toBrandList(value); ok {
				c.FullVersionList = list
			}
		case "http-sec-ch-ua", "sec-ch-ua":
			if len(c.FullVersionList) == 0 {
				if s, ok := value.(string); ok {
					if list := parseBrandListHeader(s); len(list) > 0 {
						c.FullVersionList = list
					}
				}
			}
		case "http-sec-ch-ua-full-version-list", "sec-ch-ua-full-version-list":
			if s, ok := value.(string); ok {
				if list := parseBrandListHeader(s); len(list) > 0 {
					c.FullVersionList = list
				}
			}
		case "http-x-requested-with", "x-requested-with":
			if s, ok := value.(string); ok && strings.ToLower(s) != "xmlhttprequest" {
				c.App = s
			}
		case "formfactors", "http-sec-ch-ua-form-factors", "sec-ch-ua-form-factors":
			c.FormFactors = parseFormFactors(value)
		}
	}

	return c
}

var (
	htmlTagRe        = regexp.MustCompile(`<[^>]*>`)
	brandHeaderRe    = regexp.MustCompile(`^"([^"]+)"; ?v="([^"]+)"(?:, )?`)
	formFactorWordRe = regexp.MustCompile(`(?i)"([a-z]+)"`)
)

// isEmptyHint mirrors PHP empty(): "", false, 0, empty slice/map are skipped.
func isEmptyHint(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case bool:
		return !x
	case []string:
		return len(x) == 0
	case []any:
		return len(x) == 0
	case []BrandVersion:
		return len(x) == 0
	default:
		return false
	}
}

// parseBrandListHeader parses a Sec-CH-UA header value like
// `"Opera";v="83", "Chromium";v="98"` into ordered brand/version pairs.
func parseBrandListHeader(value string) []BrandVersion {
	var list []BrandVersion

	for {
		m := brandHeaderRe.FindStringSubmatch(value)
		if m == nil {
			break
		}

		list = append(list, BrandVersion{Brand: m[1], Version: m[2]})
		value = value[len(m[0]):]
	}

	return list
}

// toBrandList converts a structured brand list (navigator.userAgentData) into
// ordered pairs. It mirrors getBrandList's array_column + count check: if any
// entry is missing a brand or a version, the whole list is discarded.
func toBrandList(value any) ([]BrandVersion, bool) {
	entries := toEntrySlice(value)
	if entries == nil {
		return nil, false
	}

	var brands, versions []string

	for _, e := range entries {
		if b, ok := e["brand"]; ok {
			brands = append(brands, b)
		}

		if v, ok := e["version"]; ok {
			versions = append(versions, v)
		}
	}

	if len(brands) != len(versions) {
		return []BrandVersion{}, true
	}

	list := make([]BrandVersion, len(brands))
	for i := range brands {
		list[i] = BrandVersion{Brand: brands[i], Version: versions[i]}
	}

	return list, true
}

// toEntrySlice normalizes the various shapes a brand list may arrive in.
func toEntrySlice(value any) []map[string]string {
	switch xs := value.(type) {
	case []BrandVersion:
		out := make([]map[string]string, len(xs))
		for i, e := range xs {
			out[i] = map[string]string{"brand": e.Brand, "version": e.Version}
		}

		return out
	case []map[string]string:
		return xs
	case []map[string]any:
		out := make([]map[string]string, 0, len(xs))
		for _, e := range xs {
			out = append(out, entryToStringMap(e))
		}

		return out
	case []any:
		out := make([]map[string]string, 0, len(xs))
		for _, e := range xs {
			m, ok := e.(map[string]any)
			if !ok {
				return nil
			}

			out = append(out, entryToStringMap(m))
		}

		return out
	default:
		return nil
	}
}

func entryToStringMap(e map[string]any) map[string]string {
	out := make(map[string]string, len(e))

	for k, v := range e {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}

	return out
}

// parseFormFactors mirrors the Sec-CH-UA-Form-Factors handling: an array of
// strings is lowercased; a string is scanned for quoted words; anything else
// (e.g. an array containing non-strings) is ignored.
func parseFormFactors(value any) []string {
	switch xs := value.(type) {
	case []string:
		out := make([]string, len(xs))
		for i, s := range xs {
			out[i] = strings.ToLower(s)
		}

		return out
	case []any:
		out := make([]string, 0, len(xs))
		for _, e := range xs {
			s, ok := e.(string)
			if !ok {
				return nil
			}

			out = append(out, strings.ToLower(s))
		}

		return out
	case string:
		var out []string
		for _, m := range formFactorWordRe.FindAllStringSubmatch(strings.ToLower(xs), -1) {
			out = append(out, m[1])
		}

		return out
	default:
		return nil
	}
}
