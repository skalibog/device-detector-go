package devicedetector

import (
	"net/http"

	"github.com/skalibog/device-detector-go/internal/parser"
)

// ClientHints holds parsed HTTP Client Hints for a request. Build one with
// NewClientHintsFromHeaders or NewClientHintsFromMap and pass it to
// (*DeviceDetector).ParseWithHints.
type ClientHints = parser.ClientHints

// BrandVersion is one entry of a Sec-CH-UA brand/version list.
type BrandVersion = parser.BrandVersion

// NewClientHintsFromHeaders builds ClientHints from HTTP request headers
// (typically request.Header).
func NewClientHintsFromHeaders(h http.Header) *ClientHints {
	return parser.NewClientHintsFromHeaders(h)
}

// NewClientHintsFromMap builds ClientHints from a header/value map. Values may
// be plain strings (HTTP headers) or the structured values reported by
// navigator.userAgentData (a bool "mobile", a []string "formFactors", and a
// brand/version list under "brands" or "fullVersionList").
func NewClientHintsFromMap(headers map[string]any) *ClientHints {
	return parser.NewClientHintsFromMap(headers)
}
