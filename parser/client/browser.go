package client

import (
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"github.com/skalibog/device-detector-go/parser"
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
	engine     *Engine
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

	return &Browser{
		entries:    entries,
		engine:     engine,
		truncation: parser.VersionTruncationMinor,
	}, nil
}

// Name returns the parser name.
func (b *Browser) Name() string { return "browser" }

// SetVersionTruncation sets the version truncation level.
func (b *Browser) SetVersionTruncation(t int) { b.truncation = t }

// Parse mirrors Browser::parse() on the pure user-agent path.
func (b *Browser) Parse(ua string) (*Result, error) {
	ua0, err := b.parseFromUserAgent(ua)
	if err != nil {
		return nil, err
	}

	name := ua0.name
	version := ua0.version
	short := ua0.short
	engine := ua0.engine
	engineVersion := ua0.engineVersion

	family, _ := BrowserFamily(short)

	// TODO(client-hints): v0.2 — BrowserHints.parse() may override the name
	// and re-derive engine/family; skipped on the UA-only path.

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

	if name == "Wolvic" && engine == "Blink" {
		family = "Chrome"
	}

	if name == "Wolvic" && engine == "Gecko" {
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

	for i := range b.entries {
		m, err := parser.MatchUserAgent(ua, b.entries[i].Regex)
		if err != nil {
			return uaBrowser{}, err
		}

		if m != nil {
			matches = m
			entry = &b.entries[i]

			break
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
