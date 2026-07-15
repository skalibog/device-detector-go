package client

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/skalibog/device-detector-go/parser"
)

// availableEngines mirrors Engine::$availableEngines: known browser engines in
// their canonical spelling.
var availableEngines = []string{
	"WebKit",
	"Blink",
	"Trident",
	"Text-based",
	"Dillo",
	"iCab",
	"Elektra",
	"Presto",
	"Clecko",
	"Gecko",
	"KHTML",
	"NetFront",
	"Edge",
	"NetSurf",
	"Servo",
	"Goanna",
	"EkiohFlow",
	"Arachne",
	"LibWeb",
	"Maple",
}

type engineEntry struct {
	Regex string `yaml:"regex"`
	Name  string `yaml:"name"`
}

// Engine ports Parser/Client/Browser/Engine.php: it detects a browser engine
// from the user agent using the browser_engine.yml regexes.
type Engine struct {
	entries []engineEntry
}

// NewEngine loads the browser engine database from fsys.
func NewEngine(fsys fs.FS) (*Engine, error) {
	var entries []engineEntry
	if err := parser.Load(fsys, "client/browser_engine.yml", &entries); err != nil {
		return nil, err
	}

	return &Engine{entries: entries}, nil
}

// detect mirrors Engine::parse(): it returns the canonical engine name, or an
// empty string when no engine regex matches.
func (e *Engine) detect(ua string) (string, error) {
	var (
		matches []string
		entry   *engineEntry
	)

	for i := range e.entries {
		m, err := parser.MatchUserAgent(ua, e.entries[i].Regex)
		if err != nil {
			return "", err
		}

		if m != nil {
			matches = m
			entry = &e.entries[i]

			break
		}
	}

	if matches == nil || entry == nil {
		return "", nil
	}

	name := parser.BuildByMatch(entry.Name, matches)

	for _, engineName := range availableEngines {
		if strings.EqualFold(name, engineName) {
			return engineName, nil
		}
	}

	// Mirrors the PHP exception: a matched engine name must exist in
	// availableEngines. Reaching here means the database and code diverged.
	return "", fmt.Errorf("client: detected browser engine %q not found in availableEngines (ua %q)", name, ua)
}
