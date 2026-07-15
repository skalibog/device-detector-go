package parser

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/dlclark/regexp2"
)

// OSResult is the operating system detected from a user agent.
type OSResult struct {
	Name      string
	ShortName string
	Version   string
	Platform  string
	Family    string
}

// osVersionRule is a nested "versions" entry that refines the name and/or
// version once the parent OS regex has matched. Name and Version are pointers
// so an absent YAML key (keep previous value) is distinguished from an explicit
// empty string (override with empty), mirroring PHP's array_key_exists checks.
type osVersionRule struct {
	Regex   string  `yaml:"regex"`
	Name    *string `yaml:"name"`
	Version *string `yaml:"version"`

	compiled *regexp2.Regexp
}

// osRule is a top-level entry from oss.yml.
type osRule struct {
	Regex    string          `yaml:"regex"`
	Name     string          `yaml:"name"`
	Version  string          `yaml:"version"`
	Versions []osVersionRule `yaml:"versions"`

	compiled *regexp2.Regexp
}

// OS parses a user agent for operating system information, mirroring
// DeviceDetector's Parser\OperatingSystem. Apart from SetVersionTruncation
// (intended to be called during setup, before any Parse), it is immutable and
// safe for concurrent use.
type OS struct {
	rules      []osRule
	truncation int
}

// NewOS loads oss.yml from fsys and precompiles every regex. Version truncation
// defaults to VersionTruncationMinor, matching the PHP default.
func NewOS(fsys fs.FS) (*OS, error) {
	var rules []osRule
	if err := Load(fsys, "oss.yml", &rules); err != nil {
		return nil, err
	}

	for i := range rules {
		re, err := Compile(rules[i].Regex)
		if err != nil {
			return nil, fmt.Errorf("devicedetector: compiling os regex %q: %w", rules[i].Regex, err)
		}

		rules[i].compiled = re

		for j := range rules[i].Versions {
			vre, err := Compile(rules[i].Versions[j].Regex)
			if err != nil {
				return nil, fmt.Errorf("devicedetector: compiling os version regex %q: %w", rules[i].Versions[j].Regex, err)
			}

			rules[i].Versions[j].compiled = vre
		}
	}

	return &OS{rules: rules, truncation: VersionTruncationMinor}, nil
}

// SetVersionTruncation sets how deep version numbers are reported. It accepts
// any of the VersionTruncation* constants and ignores anything else, mirroring
// AbstractParser::setVersionTruncation. Call it during setup, not concurrently
// with Parse.
func (o *OS) SetVersionTruncation(t int) {
	switch t {
	case VersionTruncationMajor, VersionTruncationMinor, VersionTruncationPatch,
		VersionTruncationBuild, VersionTruncationNone:
		o.truncation = t
	}
}

// Parse detects the operating system from ua. It returns (nil, nil) when no OS
// can be determined.
//
// v0.1 is user-agent-only: the client-hints merge paths of the PHP are skipped.
func (o *OS) Parse(ua string) (*OSResult, error) {
	// TODO(client-hints): v0.2 — restoreUserAgentFromClientHints and
	// parseOsFromClientHints are skipped; only the UA path is implemented.
	name, short, version, err := o.parseFromUserAgent(ua)
	if err != nil {
		return nil, err
	}

	if name == "" {
		return nil, nil
	}

	platform, err := o.parsePlatform(ua)
	if err != nil {
		return nil, err
	}

	family, _ := OSFamily(short)

	// TODO(client-hints): v0.2 — the androidApps / lineageos.jelly /
	// firefox.tv app remaps live here in the PHP.

	result := &OSResult{
		Name:      name,
		ShortName: short,
		Version:   version,
		Platform:  platform,
		Family:    family,
	}

	if s, ok := OSShortName(name); ok {
		result.ShortName = s
	}

	return result, nil
}

// parseFromUserAgent mirrors parseOsFromUserAgent: it finds the first matching
// oss.yml rule, then applies the first matching nested "versions" refinement.
func (o *OS) parseFromUserAgent(ua string) (name, short, version string, err error) {
	var matched *osRule
	var matches []string

	for i := range o.rules {
		m, mErr := matchWith(o.rules[i].compiled, ua)
		if mErr != nil {
			return "", "", "", mErr
		}

		if m != nil {
			matched = &o.rules[i]
			matches = m

			break
		}
	}

	if matched == nil {
		return "", "", "", nil
	}

	name = BuildByMatch(matched.Name, matches)
	name, short = getShortOsData(name)
	version = BuildVersion(matched.Version, matches, o.truncation)

	for j := range matched.Versions {
		rule := &matched.Versions[j]

		m, mErr := matchWith(rule.compiled, ua)
		if mErr != nil {
			return "", "", "", mErr
		}

		if m == nil {
			continue
		}

		if rule.Name != nil {
			name = BuildByMatch(*rule.Name, m)
			name, short = getShortOsData(name)
		}

		if rule.Version != nil {
			version = BuildVersion(*rule.Version, m, o.truncation)
		}

		break
	}

	return name, short, version, nil
}

// parsePlatform mirrors parsePlatform for the user-agent-only path.
func (o *OS) parsePlatform(ua string) (string, error) {
	// TODO(client-hints): v0.2 — architecture/bitness from client hints is
	// checked before the UA patterns in the PHP.
	for _, c := range platformChecks {
		m, err := MatchUserAgent(ua, c.pattern)
		if err != nil {
			return "", err
		}

		if m != nil {
			return c.platform, nil
		}
	}

	return "", nil
}

type platformCheck struct {
	pattern  string
	platform string
}

// platformChecks preserves the exact order of the PHP platform detection.
var platformChecks = []platformCheck{
	{`arm[ _;)ev]|.*arm$|.*arm64|aarch64|Apple ?TV|Watch ?OS|Watch1,[12]`, "ARM"},
	{`loongarch64`, "LoongArch64"},
	{`mips`, "MIPS"},
	{`sh4`, "SuperH"},
	{`sparc64`, "SPARC64"},
	{`64-?bit|WOW64|(?:Intel)?x64|WINDOWS_64|win64|.*amd64|.*x86_?64`, "x64"},
	{`.*32bit|.*win32|(?:i[0-9]|x)86|i86pc`, "x86"},
}

// getShortOsData mirrors OperatingSystem::getShortOsData. If name matches a
// known OS (case-insensitively) it returns the canonical name and short code;
// otherwise it returns name unchanged with short "UNK".
func getShortOsData(name string) (canonName, short string) {
	for _, pair := range operatingSystemList {
		if strings.EqualFold(name, pair.Name) {
			return pair.Name, pair.Short
		}
	}

	return name, "UNK"
}

// OSShortName returns the short code for an OS full name, mirroring
// array_search over the operatingSystems map (first match, case-sensitive).
func OSShortName(name string) (string, bool) {
	for _, pair := range operatingSystemList {
		if pair.Name == name {
			return pair.Short, true
		}
	}

	return "", false
}

// OSFamily returns the OS family for the given label, which may be either a
// short code or a full OS name, mirroring OperatingSystem::getOsFamily. The
// boolean is false when the OS has no known family ("Unknown" in the PHP).
func OSFamily(osLabel string) (string, bool) {
	if short, ok := OSShortName(osLabel); ok {
		osLabel = short
	}

	for _, group := range osFamilyList {
		for _, code := range group.Codes {
			if code == osLabel {
				return group.Family, true
			}
		}
	}

	return "", false
}

// IsDesktopOS reports whether the OS (given by short code or name) belongs to a
// desktop-only family, mirroring OperatingSystem::isDesktopOs.
func IsDesktopOS(osName string) bool {
	family, ok := OSFamily(osName)
	if !ok {
		return false
	}

	_, isDesktop := desktopOsArray[family]

	return isDesktop
}

// OSNameFromID returns the full OS name for a short code with an optional
// version appended, mirroring OperatingSystem::getNameFromId. The boolean is
// false when the short code is unknown.
func OSNameFromID(short, version string) (string, bool) {
	name, ok := osNameByShort[short]
	if !ok {
		return "", false
	}

	return strings.TrimSpace(name + " " + version), true
}
