package parser

import (
	"strconv"
	"strings"
)

// Version truncation levels, mirroring AbstractParser::VERSION_TRUNCATION_*.
const (
	VersionTruncationMajor = 0
	VersionTruncationMinor = 1
	VersionTruncationPatch = 2
	VersionTruncationBuild = 3
	VersionTruncationNone  = -1
)

// BuildByMatch substitutes $1..$N placeholders in item with capture groups,
// replicating AbstractParser::buildByMatch() exactly — including its
// sequential-replacement semantics ($1 is substituted before $10).
func BuildByMatch(item string, matches []string) string {
	for nb := 1; nb < len(matches); nb++ {
		item = strings.ReplaceAll(item, "$"+strconv.Itoa(nb), matches[nb])
	}

	// PHP iterates $nb up to count($matches) inclusive; the last placeholder
	// has no corresponding group and resolves to an empty string.
	item = strings.ReplaceAll(item, "$"+strconv.Itoa(len(matches)), "")

	return strings.TrimSpace(item)
}

// BuildVersion expands placeholders in versionString, normalizes underscores
// to dots and truncates to the requested precision, mirroring
// AbstractParser::buildVersion().
func BuildVersion(versionString string, matches []string, truncation int) string {
	versionString = BuildByMatch(versionString, matches)
	versionString = strings.ReplaceAll(versionString, "_", ".")

	if truncation != VersionTruncationNone && strings.Count(versionString, ".") > truncation {
		parts := strings.Split(versionString, ".")
		versionString = strings.Join(parts[:1+truncation], ".")
	}

	return strings.Trim(versionString, " .")
}

// FuzzyCompare reports whether two strings are equal ignoring case and spaces,
// mirroring AbstractParser::fuzzyCompare().
func FuzzyCompare(a, b string) bool {
	return strings.ReplaceAll(strings.ToLower(a), " ", "") ==
		strings.ReplaceAll(strings.ToLower(b), " ", "")
}
