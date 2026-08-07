package client

import (
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"github.com/skalibog/device-detector-go/internal/parser"
)

// browserEngine mirrors the "engine" mapping of a browsers.yml entry.
type browserEngine struct {
	Default  string                    `yaml:"default"`
	Versions parser.OrderedMap[string] `yaml:"versions"`
}

// browserEntry is one record of browsers.yml.
type browserEntry struct {
	Regex   string         `yaml:"regex"`
	Name    string         `yaml:"name"`
	Version string         `yaml:"version"`
	Engine  *browserEngine `yaml:"engine"`
}

// Lookup tables derived from the transcribed maps in browser_maps.go.
var (
	shortByNameLower = map[string]string{} // lowercase name -> short code (first wins)
	shortByName      = map[string]string{} // exact name -> short code (first wins)
	familyByCode     = map[string]string{} // short code -> family (first family wins)
	mobileOnlyCodes  = map[string]struct{}{}
)

func init() {
	for _, b := range availableBrowsers {
		lower := strings.ToLower(b.Name)
		if _, ok := shortByNameLower[lower]; !ok {
			shortByNameLower[lower] = b.Short
		}

		if _, ok := shortByName[b.Name]; !ok {
			shortByName[b.Name] = b.Short
		}
	}

	for _, f := range browserFamilies {
		for _, code := range f.Codes {
			if _, ok := familyByCode[code]; !ok {
				familyByCode[code] = f.Family
			}
		}
	}

	for _, code := range mobileOnlyBrowsersList {
		mobileOnlyCodes[code] = struct{}{}
	}
}

// cypressPhantomRegex matches automation user agents that must not be reported
// as a browser. Ported verbatim from Browser::parse(); case-sensitive.
var cypressPhantomRegex = regexp.MustCompile(`Cypress|PhantomJS`)

// Browser ports Parser/Client/Browser.php: it detects browsers together with
// their rendering engine and engine version.
type Browser struct {
	entries    []browserEntry
	gateSet    *parser.GateSet
	engine     *Engine
	appHints   map[string]string // client/hints/browsers.yml: app id -> browser name
	truncation int
}

// NewBrowser loads the browser database (and the engine database it depends on)
// from fsys.
func NewBrowser(fsys fs.FS) (*Browser, error) {
	var entries []browserEntry
	if err := parser.Load(fsys, "client/browsers.yml", &entries); err != nil {
		return nil, err
	}

	engine, err := NewEngine(fsys)
	if err != nil {
		return nil, err
	}

	var appHints map[string]string
	if err := parser.Load(fsys, "client/hints/browsers.yml", &appHints); err != nil {
		return nil, err
	}

	patterns := make([]string, len(entries))
	for i := range entries {
		patterns[i] = entries[i].Regex
	}

	return &Browser{
		entries:    entries,
		gateSet:    parser.CompileGateSet(patterns),
		engine:     engine,
		appHints:   appHints,
		truncation: parser.VersionTruncationMinor,
	}, nil
}

// Name returns the parser name.
func (b *Browser) Name() string { return "browser" }

// SetVersionTruncation sets the version truncation level.
func (b *Browser) SetVersionTruncation(t int) { b.truncation = t }

// Warm compiles every browser entry regex and the engine regexes.
func (b *Browser) Warm() error {
	for i := range b.entries {
		if _, err := parser.Compile(b.entries[i].Regex); err != nil {
			return fmt.Errorf("client browser: compiling %q: %w", b.entries[i].Regex, err)
		}
	}

	return b.engine.warm()
}

// Parse mirrors Browser::parse() on the pure user-agent path.
func (b *Browser) Parse(ua string, hints *parser.ClientHints) (*Result, error) {
	chName, chShort, chVersion := b.parseFromClientHints(hints)

	ua0, err := b.parseFromUserAgent(ua)
	if err != nil {
		return nil, err
	}

	var name, version, short, engine, engineVersion string

	// Use client hints in favour of user-agent data when both name and version
	// are present.
	if chName != "" && chVersion != "" {
		name, version, short = chName, chVersion, chShort

		// A client-hints version of the form 2020..2024 identifies Iridium.
		if iridiumRe.MatchString(version) {
			name, short = "Iridium", "I1"
		}

		if strings.HasPrefix(version, "15") && strings.HasPrefix(ua0.version, "114") {
			name, short = "360 Secure Browser", "3B"
			engine, engineVersion = ua0.engine, ua0.engineVersion
		}

		// These browsers report a coarse CH version; keep the UA version.
		if ua0.version != "" && slicesContains([]string{"A0", "AL", "HP", "JR", "MU", "OM", "OP", "VR"}, short) {
			version = ua0.version
		}

		if name == "Vewd Browser" {
			engine, engineVersion = ua0.engine, ua0.engineVersion
		}

		// Client hints report Chromium, but the UA detected a specific
		// Chromium-based browser — prefer that.
		if (name == "Chromium" || name == "Chrome Webview") && ua0.name != "" &&
			!slicesContains([]string{"CR", "CV", "AN", "CM"}, ua0.short) {
			name, short, version = ua0.name, ua0.short, ua0.version
		}

		// Fix mobile browser names, e.g. Chrome => Chrome Mobile.
		if name+" Mobile" == ua0.name {
			name, short = ua0.name, ua0.short
		}

		// Different browser but same family: take the engine from the UA.
		if name != ua0.name && browserFamilyOf(name) == browserFamilyOf(ua0.name) {
			engine, engineVersion = ua0.engine, ua0.engineVersion
		}

		if name == ua0.name {
			engine, engineVersion = ua0.engine, ua0.engineVersion
		}

		// Prefer the UA version when it is a more detailed extension of the CH one.
		if ua0.version != "" && strings.HasPrefix(ua0.version, version) &&
			phpVersionCompare(version, ua0.version) < 0 {
			version = ua0.version
		}

		if name == "DuckDuckGo Privacy Browser" {
			version = ""
		}

		// Prefer a more detailed engine version reported via client hints.
		if engine == "Blink" && name != "Iridium" && phpVersionCompare(engineVersion, chVersion) < 0 {
			engineVersion = chVersion
		}
	} else {
		name, version, short = ua0.name, ua0.version, ua0.short
		engine, engineVersion = ua0.engine, ua0.engineVersion
	}

	family, _ := BrowserFamily(short)

	// BrowserHints: an Android app id can identify a browser the UA cannot.
	if appName := b.browserHintName(hints); appName != "" && appName != name {
		name = appName
		version = ""

		s, ok := browserShortName(name)
		if !ok {
			return nil, fmt.Errorf("client browser: detected name %q not found in availableBrowsers (ua %q)", name, ua)
		}

		short = s

		if chromeSafariRe.MatchString(ua) {
			engine = "Blink"
			if family, _ = BrowserFamily(short); family == "" {
				family = "Chrome"
			}

			engineVersion, err = buildEngineVersion(engine, ua)
			if err != nil {
				return nil, err
			}
		}
	}

	if name == "" || cypressPhantomRegex.MatchString(ua) {
		return nil, nil
	}

	// exclude Blink engine version for browsers
	if engine == "Blink" && name == "Flow Browser" {
		engineVersion = ""
	}

	// the browser simulates a ua for Android OS
	if name == "Every Browser" {
		family = "Chrome"
		engine = "Blink"
		engineVersion = ""
	}

	// this browser simulates the user agent of Firefox
	if name == "TV-Browser Internet" && engine == "Gecko" {
		family = "Chrome"
		engine = "Blink"
		engineVersion = ""
	}

	if (name == "Yaani Browser" || name == "Wolvic") && engine == "Blink" {
		family = "Chrome"
	}

	if (name == "Yaani Browser" || name == "Wolvic") && engine == "Gecko" {
		family = "Firefox"
	}

	return &Result{
		Type:          "browser",
		Name:          name,
		ShortName:     short,
		Version:       version,
		Engine:        engine,
		EngineVersion: engineVersion,
		Family:        family,
	}, nil
}

var (
	iridiumRe      = regexp.MustCompile(`^202[0-4]`)
	chromeSafariRe = regexp.MustCompile(`(?i)Chrome/.+ Safari/537\.36`)
)

// browserClientHintMapping maps a brand reported in client hints to the name
// this library uses (Browser::$clientHintMapping).
var browserClientHintMapping = map[string][]string{
	"Chrome":                     {"Google Chrome"},
	"Chrome Webview":             {"Android WebView"},
	"DuckDuckGo Privacy Browser": {"DuckDuckGo"},
	"Edge WebView":               {"Microsoft Edge WebView2"},
	"Mi Browser":                 {"Miui Browser", "XiaoMiBrowser"},
	"Microsoft Edge":             {"Edge"},
	"Norton Private Browser":     {"Norton Secure Browser"},
	"Opera GX":                   {"Opera GX Android"},
	"Opera Mini":                 {"Opera Mini Android"},
	"Vewd Browser":               {"Vewd Core"},
	"Yandex Browser":             {"YaSearchBrowser"},
}

func browserApplyClientHintMapping(name string) string {
	lower := strings.ToLower(name)
	for mapped, hints := range browserClientHintMapping {
		for _, h := range hints {
			if lower == strings.ToLower(h) {
				return mapped
			}
		}
	}

	return name
}

func browserFamilyOf(label string) string {
	f, _ := BrowserFamily(label)

	return f
}

// parseFromClientHints mirrors Browser::parseBrowserFromClientHints.
func (b *Browser) parseFromClientHints(hints *parser.ClientHints) (name, short, version string) {
	if hints == nil {
		return "", "", ""
	}

	brands := hints.BrandList()
	if len(brands) == 0 {
		return "", "", ""
	}

	for _, bv := range brands {
		brand := browserApplyClientHintMapping(bv.Brand)

		for _, e := range availableBrowsers {
			if parser.FuzzyCompare(brand, e.Name) ||
				parser.FuzzyCompare(brand+" Browser", e.Name) ||
				parser.FuzzyCompare(brand, e.Name+" Browser") {
				name, short, version = e.Name, e.Short, bv.Version
				break
			}
		}

		// Keep looking past Chromium / Microsoft Edge for a more specific brand.
		if name != "" && name != "Chromium" && name != "Microsoft Edge" {
			break
		}
	}

	if bvers := hints.BrandVersion(); bvers != "" {
		version = bvers
	}

	return name, short, parser.BuildVersion(version, nil, b.truncation)
}

// browserHintName returns the browser name for the client-hints app id, or "".
func (b *Browser) browserHintName(hints *parser.ClientHints) string {
	if hints == nil {
		return ""
	}

	return b.appHints[hints.App]
}

// uaBrowser holds the browser fields detected from the user agent.
type uaBrowser struct {
	name          string
	short         string
	version       string
	engine        string
	engineVersion string
}

// parseFromUserAgent mirrors Browser::parseBrowserFromUserAgent().
func (b *Browser) parseFromUserAgent(ua string) (uaBrowser, error) {
	var (
		matches []string
		entry   *browserEntry
	)

	try := func(i int) (bool, error) {
		m, err := parser.MatchUserAgent(ua, b.entries[i].Regex)
		if err != nil || m == nil {
			return false, err
		}

		matches = m
		entry = &b.entries[i]

		return true, nil
	}

	// Required-literal probes over the lowercased UA: walk only the entries
	// whose literal occurs (plus the few with no provable literal cover).
	if only, ok := b.gateSet.SkipGated(ua); ok {
		for _, i := range only {
			hit, err := try(i)
			if err != nil {
				return uaBrowser{}, err
			}

			if hit {
				break
			}
		}
	} else {
		for i := range b.entries {
			hit, err := try(i)
			if err != nil {
				return uaBrowser{}, err
			}

			if hit {
				break
			}
		}
	}

	if matches == nil || entry == nil {
		return uaBrowser{}, nil
	}

	name := parser.BuildByMatch(entry.Name, matches)

	short, ok := browserShortName(name)
	if !ok {
		// Mirrors the PHP exception: a detected browser name must exist in
		// availableBrowsers.
		return uaBrowser{}, fmt.Errorf("client: detected browser name %q not found in availableBrowsers (ua %q)", name, ua)
	}

	version := parser.BuildVersion(entry.Version, matches, b.truncation)

	engine, err := b.buildEngine(entry.Engine, version, ua)
	if err != nil {
		return uaBrowser{}, err
	}

	engineVersion, err := buildEngineVersion(engine, ua)
	if err != nil {
		return uaBrowser{}, err
	}

	return uaBrowser{
		name:          name,
		short:         short,
		version:       version,
		engine:        engine,
		engineVersion: engineVersion,
	}, nil
}

// buildEngine mirrors Browser::buildEngine(): an explicit default, then any
// version-threshold override, then a fallback to engine regex detection.
func (b *Browser) buildEngine(ed *browserEngine, browserVersion, ua string) (string, error) {
	engine := ""

	if ed != nil {
		engine = ed.Default

		for _, kv := range ed.Versions.Entries {
			if phpVersionCompare(browserVersion, kv.Key) < 0 {
				continue
			}

			engine = kv.Value
		}
	}

	if engine == "" {
		detected, err := b.engine.detect(ua)
		if err != nil {
			return "", err
		}

		engine = detected
	}

	return engine, nil
}

// browserShortName mirrors Browser::getBrowserShortName(): case-insensitive
// name lookup returning the internal short code.
func browserShortName(name string) (string, bool) {
	short, ok := shortByNameLower[strings.ToLower(name)]

	return short, ok
}

// BrowserFamily mirrors Browser::getBrowserFamily(): it accepts either a short
// code or a full browser name and returns the family it belongs to.
func BrowserFamily(browserLabel string) (string, bool) {
	if short, ok := shortByName[browserLabel]; ok {
		browserLabel = short
	}

	family, ok := familyByCode[browserLabel]

	return family, ok
}

// IsMobileOnlyBrowser mirrors Browser::isMobileOnlyBrowser(): it accepts either
// a short code or a full browser name.
func IsMobileOnlyBrowser(browser string) bool {
	if _, ok := mobileOnlyCodes[browser]; ok {
		return true
	}

	if short, ok := shortByName[browser]; ok {
		if _, ok := mobileOnlyCodes[short]; ok {
			return true
		}
	}

	return false
}

func slicesContains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
