package parser

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"
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

	compiled *Compiled
}

// osRule is a top-level entry from oss.yml.
type osRule struct {
	Regex    string          `yaml:"regex"`
	Name     string          `yaml:"name"`
	Version  string          `yaml:"version"`
	Versions []osVersionRule `yaml:"versions"`

	compiled *Compiled
}

// OS parses a user agent for operating system information, mirroring
// DeviceDetector's Parser\OperatingSystem. Apart from SetVersionTruncation
// (intended to be called during setup, before any Parse), it is immutable and
// safe for concurrent use.
type OS struct {
	rules      []osRule
	gateSet    *GateSet
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

	patterns := make([]string, len(rules))
	for i := range rules {
		patterns[i] = rules[i].Regex
	}

	return &OS{rules: rules, gateSet: CompileGateSet(patterns), truncation: VersionTruncationMinor}, nil
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

var androidClientHintApps = []string{
	"com.hisense.odinbrowser", "com.seraphic.openinet.pre",
	"com.appssppa.idesktoppcbrowser", "every.browser.inc",
}

// Parse detects the operating system from ua, optionally refined by client
// hints. It returns (nil, nil) when no OS can be determined.
func (o *OS) Parse(ua string, hints *ClientHints) (*OSResult, error) {
	if hints != nil {
		ua = hints.RestoreUserAgent(ua)
	}

	chName, chShort, chVersion := o.parseFromClientHints(hints)

	uaName, uaShort, uaVersion, err := o.parseFromUserAgent(ua)
	if err != nil {
		return nil, err
	}

	var name, short, version string

	switch {
	case chName != "":
		name, version, short = chName, chVersion, chShort

		// Use the UA version when client hints gave none and the OS families match.
		if version == "" && osFamilyOf(name) == osFamilyOf(uaName) {
			version = uaVersion
		}

		// On Windows, 0.0.0 can be 7, 8 or 8.1 — prefer the UA version.
		if name == "Windows" && version == "0.0.0" {
			if uaVersion == "10" {
				version = ""
			} else {
				version = uaVersion
			}
		}

		// When the CH name is the UA name's family, the UA name is more detailed.
		if uaName != name && osFamilyOf(uaName) == name {
			name = uaName

			switch name {
			case "LeafOS", "HarmonyOS":
				version = ""
			case "PICO OS":
				version = uaVersion
			case "Fire OS":
				if chVersion != "" {
					version = fireOSVersion(version)
				}
			}
		}

		// Chrome OS is sometimes reported as Linux in client hints (version must match).
		if name == "GNU/Linux" && uaName == "Chrome OS" && chVersion == uaVersion {
			name, short = uaName, uaShort
		}

		// Chrome OS is sometimes reported as Android in client hints.
		if name == "Android" && uaName == "Chrome OS" {
			name, version, short = uaName, "", uaShort
		}

		// Meta Horizon is reported as Linux in client hints.
		if name == "GNU/Linux" && uaName == "Meta Horizon" {
			name, short = uaName, uaShort
		}
	case uaName != "":
		name, version, short = uaName, uaVersion, uaShort
	default:
		return nil, nil
	}

	platform, err := o.parsePlatform(ua, hints)
	if err != nil {
		return nil, err
	}

	family, _ := OSFamily(short)

	if hints != nil {
		switch app := hints.App; {
		case name != "Android" && containsStr(androidClientHintApps, app):
			name, family, short, version = "Android", "Android", "ADR", ""
		case name != "Lineage OS" && app == "org.lineageos.jelly":
			name, family, short, version = "Lineage OS", "Android", "LEN", lineageOSVersion(version)
		case name != "Fire OS" && app == "org.mozilla.tv.firefox":
			name, family, short, version = "Fire OS", "Android", "FIR", fireOSVersion(version)
		}
	}

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

// parseFromClientHints mirrors parseOsFromClientHints.
func (o *OS) parseFromClientHints(hints *ClientHints) (name, short, version string) {
	if hints == nil || hints.Platform == "" {
		return "", "", ""
	}

	hintName := osApplyClientHintMapping(hints.Platform)

	for _, pair := range operatingSystemList {
		if FuzzyCompare(hintName, pair.Name) {
			name, short = pair.Name, pair.Short
			break
		}
	}

	version = hints.PlatformVersion

	if name == "Windows" {
		major := phpAtoi(version)
		minor := phpAtoi(afterFirstDot(version))

		switch {
		case major == 0:
			switch minor {
			case 1:
				version = "7"
			case 2:
				version = "8"
			case 3:
				version = "8.1"
			}
		case major > 0 && major < 11:
			version = "10"
		case major > 10:
			version = "11"
		}
	}

	// On Windows 0.0.0 is meaningful; elsewhere a zero version is dropped.
	if name != "Windows" && version != "0.0.0" && phpAtoi(version) == 0 {
		version = ""
	}

	return name, short, BuildVersion(version, nil, o.truncation)
}

// osApplyClientHintMapping maps a client-hints platform name to the name this
// library uses (OperatingSystem::$clientHintMapping).
func osApplyClientHintMapping(name string) string {
	switch strings.ToLower(name) {
	case "linux":
		return "GNU/Linux"
	case "macos":
		return "Mac"
	default:
		return name
	}
}

func osFamilyOf(label string) string {
	f, _ := OSFamily(label)

	return f
}

// fireOSVersion / lineageOSVersion mirror the PHP `map[version] ?? map[major] ?? ”`.
func fireOSVersion(version string) string {
	if v, ok := fireOsVersionMapping[version]; ok {
		return v
	}

	return fireOsVersionMapping[strconv.Itoa(phpAtoi(version))]
}

func lineageOSVersion(version string) string {
	if v, ok := lineageOsVersionMapping[version]; ok {
		return v
	}

	return lineageOsVersionMapping[strconv.Itoa(phpAtoi(version))]
}

// parseFromUserAgent mirrors parseOsFromUserAgent: it finds the first matching
// oss.yml rule, then applies the first matching nested "versions" refinement.
func (o *OS) parseFromUserAgent(ua string) (name, short, version string, err error) {
	var matched *osRule
	var matches []string

	try := func(i int) (bool, error) {
		m, mErr := matchWith(o.rules[i].compiled, ua)
		if mErr != nil || m == nil {
			return false, mErr
		}

		matched = &o.rules[i]
		matches = m

		return true, nil
	}

	// One linear RE2 pass over the union of all translatable rules: on a
	// miss only the untranslatable few need walking.
	if only, ok := o.gateSet.SkipGated(ua); ok {
		for _, i := range only {
			hit, tErr := try(i)
			if tErr != nil {
				return "", "", "", tErr
			}

			if hit {
				break
			}
		}
	} else {
		for i := range o.rules {
			hit, tErr := try(i)
			if tErr != nil {
				return "", "", "", tErr
			}

			if hit {
				break
			}
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
func (o *OS) parsePlatform(ua string, hints *ClientHints) (string, error) {
	// Architecture from client hints takes precedence over the UA patterns.
	if hints != nil && hints.Architecture != "" {
		arch := strings.ToLower(hints.Architecture)

		switch {
		case strings.Contains(arch, "arm"):
			return "ARM", nil
		case strings.Contains(arch, "loongarch64"):
			return "LoongArch64", nil
		case strings.Contains(arch, "mips"):
			return "MIPS", nil
		case strings.Contains(arch, "sh4"):
			return "SuperH", nil
		case strings.Contains(arch, "sparc64"):
			return "SPARC64", nil
		case strings.Contains(arch, "x64") || (strings.Contains(arch, "x86") && hints.Bitness == "64"):
			return "x64", nil
		case strings.Contains(arch, "x86"):
			return "x86", nil
		}
	}

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
