// Package client ports the matomo/device-detector client parsers
// (Parser/Client/*) to Go: browsers, feed readers, libraries, media players,
// mobile apps and PIMs. It implements the pure user-agent detection path;
// client-hints handling is out of scope for v0.1.
package client

import (
	"io/fs"

	"github.com/skalibog/device-detector-go/parser"
)

// Result holds a detected client. Engine, EngineVersion, Family and ShortName
// are only populated for the browser parser.
type Result struct {
	Type          string // "browser", "feed reader", "library", "mediaplayer", "mobile app", "pim"
	Name          string
	Version       string
	Engine        string // browser only
	EngineVersion string // browser only
	Family        string // browser only
	ShortName     string // browser only (internal short code)
}

// Parser is the common interface implemented by every client parser.
type Parser interface {
	// Parse returns the detected client, or nil,nil when the user agent does
	// not match this parser.
	Parse(ua string) (*Result, error)
	// Name returns the parser name as used in PHP ($parserName).
	Name() string
	// SetVersionTruncation controls how deep versions are reported. It must be
	// called before the parser is used concurrently.
	SetVersionTruncation(t int)
}

// clientEntry is one record of a generic client regex file.
type clientEntry struct {
	Regex   string `yaml:"regex"`
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// genericParser implements AbstractClientParser::parse(): a preMatchOverall
// bail-out followed by a per-entry regex match. It backs every client parser
// except Browser.
type genericParser struct {
	name       string
	entries    []clientEntry
	overall    string
	truncation int
}

func newGeneric(fsys fs.FS, file, name string) (*genericParser, error) {
	var entries []clientEntry
	if err := parser.Load(fsys, file, &entries); err != nil {
		return nil, err
	}

	patterns := make([]string, len(entries))
	for i := range entries {
		patterns[i] = entries[i].Regex
	}

	return &genericParser{
		name:       name,
		entries:    entries,
		overall:    parser.CombineRegexes(patterns),
		truncation: parser.VersionTruncationMinor,
	}, nil
}

// Name returns the parser name.
func (p *genericParser) Name() string { return p.name }

// SetVersionTruncation sets the version truncation level.
func (p *genericParser) SetVersionTruncation(t int) { p.truncation = t }

// Parse mirrors AbstractClientParser::parse().
func (p *genericParser) Parse(ua string) (*Result, error) {
	if parser.PreMatchEmpty(p.overall) {
		return nil, nil
	}

	overall, err := parser.MatchUserAgent(ua, p.overall)
	if err != nil {
		return nil, err
	}

	if overall == nil {
		return nil, nil
	}

	for i := range p.entries {
		matches, err := parser.MatchUserAgent(ua, p.entries[i].Regex)
		if err != nil {
			return nil, err
		}

		if matches == nil {
			continue
		}

		return &Result{
			Type:    p.name,
			Name:    parser.BuildByMatch(p.entries[i].Name, matches),
			Version: parser.BuildVersion(p.entries[i].Version, matches, p.truncation),
		}, nil
	}

	return nil, nil
}

// FeedReader detects feed readers (Parser/Client/FeedReader.php).
type FeedReader struct{ *genericParser }

// NewFeedReader loads the feed reader database from fsys.
func NewFeedReader(fsys fs.FS) (*FeedReader, error) {
	g, err := newGeneric(fsys, "client/feed_readers.yml", "feed reader")
	if err != nil {
		return nil, err
	}

	return &FeedReader{g}, nil
}

// Library detects tools and software libraries (Parser/Client/Library.php).
type Library struct{ *genericParser }

// NewLibrary loads the library database from fsys.
func NewLibrary(fsys fs.FS) (*Library, error) {
	g, err := newGeneric(fsys, "client/libraries.yml", "library")
	if err != nil {
		return nil, err
	}

	return &Library{g}, nil
}

// MediaPlayer detects media players (Parser/Client/MediaPlayer.php).
type MediaPlayer struct{ *genericParser }

// NewMediaPlayer loads the media player database from fsys.
func NewMediaPlayer(fsys fs.FS) (*MediaPlayer, error) {
	g, err := newGeneric(fsys, "client/mediaplayers.yml", "mediaplayer")
	if err != nil {
		return nil, err
	}

	return &MediaPlayer{g}, nil
}

// MobileApp detects mobile applications (Parser/Client/MobileApp.php).
//
// TODO(client-hints): v0.2 — the PHP parser augments the UA result with app
// hints (AppHints.parse); only the pure UA path is implemented here.
type MobileApp struct{ *genericParser }

// NewMobileApp loads the mobile app database from fsys.
func NewMobileApp(fsys fs.FS) (*MobileApp, error) {
	g, err := newGeneric(fsys, "client/mobile_apps.yml", "mobile app")
	if err != nil {
		return nil, err
	}

	return &MobileApp{g}, nil
}

// PIM detects personal information managers (Parser/Client/PIM.php).
type PIM struct{ *genericParser }

// NewPIM loads the PIM database from fsys.
func NewPIM(fsys fs.FS) (*PIM, error) {
	g, err := newGeneric(fsys, "client/pim.yml", "pim")
	if err != nil {
		return nil, err
	}

	return &PIM{g}, nil
}

// All returns the client parsers in the exact order DeviceDetector.php tries
// them (DeviceDetector::$clientParsers): FeedReader, MobileApp, MediaPlayer,
// PIM, Browser, Library.
func All(fsys fs.FS) ([]Parser, error) {
	feedReader, err := NewFeedReader(fsys)
	if err != nil {
		return nil, err
	}

	mobileApp, err := NewMobileApp(fsys)
	if err != nil {
		return nil, err
	}

	mediaPlayer, err := NewMediaPlayer(fsys)
	if err != nil {
		return nil, err
	}

	pim, err := NewPIM(fsys)
	if err != nil {
		return nil, err
	}

	browser, err := NewBrowser(fsys)
	if err != nil {
		return nil, err
	}

	library, err := NewLibrary(fsys)
	if err != nil {
		return nil, err
	}

	return []Parser{feedReader, mobileApp, mediaPlayer, pim, browser, library}, nil
}
