package parser

import (
	"fmt"
	"io/fs"

	"github.com/dlclark/regexp2"
)

// BotProducer identifies the organisation operating a bot.
type BotProducer struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// BotResult is the outcome of a successful bot detection.
type BotResult struct {
	Name     string
	Category string
	URL      string
	Producer BotProducer
}

// botEntry is a single record from bots.yml, with its regex precompiled.
type botEntry struct {
	Regex    string      `yaml:"regex"`
	Name     string      `yaml:"name"`
	Category string      `yaml:"category"`
	URL      string      `yaml:"url"`
	Producer BotProducer `yaml:"producer"`

	compiled *regexp2.Regexp
}

// Bot parses a user agent for bot information, mirroring DeviceDetector's
// Parser\Bot. It is immutable after construction and safe for concurrent use.
type Bot struct {
	entries []botEntry
	overall *regexp2.Regexp
}

// NewBot loads bots.yml from fsys, precompiles every entry regex and builds the
// combined "overall" regex used to short-circuit non-bot user agents.
func NewBot(fsys fs.FS) (*Bot, error) {
	var entries []botEntry
	if err := Load(fsys, "bots.yml", &entries); err != nil {
		return nil, err
	}

	patterns := make([]string, len(entries))

	for i := range entries {
		re, err := Compile(entries[i].Regex)
		if err != nil {
			return nil, fmt.Errorf("devicedetector: compiling bot regex %q: %w", entries[i].Regex, err)
		}

		entries[i].compiled = re
		patterns[i] = entries[i].Regex
	}

	overall, err := Compile(CombineRegexes(patterns))
	if err != nil {
		return nil, fmt.Errorf("devicedetector: compiling combined bot regex: %w", err)
	}

	return &Bot{entries: entries, overall: overall}, nil
}

// Parse checks whether ua belongs to a bot and returns its details.
// It returns (nil, nil) when the user agent is not a known bot.
//
// The detection first tests the combined regex (a fast rejection for the common
// non-bot case) and only then walks the individual entries in file order.
func (b *Bot) Parse(ua string) (*BotResult, error) {
	pre, err := matchWith(b.overall, ua)
	if err != nil {
		return nil, err
	}

	if pre == nil {
		return nil, nil
	}

	for i := range b.entries {
		m, err := matchWith(b.entries[i].compiled, ua)
		if err != nil {
			return nil, err
		}

		if m != nil {
			e := b.entries[i]

			return &BotResult{
				Name:     e.Name,
				Category: e.Category,
				URL:      e.URL,
				Producer: e.Producer,
			}, nil
		}
	}

	return nil, nil
}

// IsBot reports whether ua belongs to a bot without collecting its details,
// mirroring Parser\Bot with discardDetails enabled: only the combined regex is
// evaluated.
func (b *Bot) IsBot(ua string) (bool, error) {
	pre, err := matchWith(b.overall, ua)
	if err != nil {
		return false, err
	}

	return pre != nil, nil
}
