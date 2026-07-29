package parser

import (
	"fmt"
	"io/fs"

	"github.com/dlclark/regexp2"
)

// vendorRegex pairs a raw vendorfragments.yml pattern with its compiled form.
// The raw pattern is retained so callers can inspect the matched regex, as
// DeviceDetector's getMatchedRegex() exposes.
type vendorRegex struct {
	raw      string
	compiled *regexp2.Regexp
}

// vendorEntry is a brand together with its ordered list of fragment regexes.
type vendorEntry struct {
	brand   string
	regexes []vendorRegex
}

// VendorFragment maps device vendor fragments in a user agent to a brand,
// mirroring DeviceDetector's Parser\VendorFragment. It is immutable after
// construction and safe for concurrent use.
type VendorFragment struct {
	entries []vendorEntry
}

// NewVendorFragment loads vendorfragments.yml from fsys, preserving brand order,
// and precompiles every fragment regex with the trailing "[^a-z0-9]+" guard the
// PHP applies at match time.
func NewVendorFragment(fsys fs.FS) (*VendorFragment, error) {
	var raw OrderedMap[[]string]
	if err := Load(fsys, "vendorfragments.yml", &raw); err != nil {
		return nil, err
	}

	entries := make([]vendorEntry, 0, len(raw.Entries))

	for _, brandEntry := range raw.Entries {
		regexes := make([]vendorRegex, 0, len(brandEntry.Value))

		for _, pattern := range brandEntry.Value {
			re, err := Compile(pattern + `[^a-z0-9]+`)
			if err != nil {
				return nil, fmt.Errorf("devicedetector: compiling vendor fragment %q: %w", pattern, err)
			}

			regexes = append(regexes, vendorRegex{raw: pattern, compiled: re})
		}

		entries = append(entries, vendorEntry{brand: brandEntry.Key, regexes: regexes})
	}

	return &VendorFragment{entries: entries}, nil
}

// Parse returns the brand whose fragment matches ua, plus the raw regex that
// matched (mirroring getMatchedRegex). Both are empty when nothing matches.
func (v *VendorFragment) Parse(ua string) (brand, matchedRegex string, err error) {
	for _, entry := range v.entries {
		for _, r := range entry.regexes {
			m, matchErr := matchWith(r.compiled, ua)
			if matchErr != nil {
				return "", "", matchErr
			}

			if m != nil {
				return entry.brand, r.raw, nil
			}
		}
	}

	return "", "", nil
}
