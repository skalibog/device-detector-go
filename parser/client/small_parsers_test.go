package client

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/skalibog/devicedetector/parser"
	"gopkg.in/yaml.v3"
)

// clientFixture mirrors the relevant slice of a device-detector test fixture.
type clientFixture struct {
	UserAgent string `yaml:"user_agent"`
	Client    struct {
		Type          string `yaml:"type"`
		Name          string `yaml:"name"`
		Version       string `yaml:"version"`
		Engine        string `yaml:"engine"`
		EngineVersion string `yaml:"engine_version"`
	} `yaml:"client"`
	BrowserFamily string `yaml:"browser_family"`
}

func regexFS() fs.FS { return os.DirFS("../../data/regexes") }

func loadClientFixtures(t *testing.T, name string) []clientFixture {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}

	var fixtures []clientFixture
	if err := yaml.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}

	return fixtures
}

func TestSmallParsers(t *testing.T) {
	fsys := regexFS()

	cases := []struct {
		name    string
		fixture string
		newFn   func(fs.FS) (Parser, error)
	}{
		{
			name:    "feed reader",
			fixture: "feed_reader.yml",
			newFn:   func(f fs.FS) (Parser, error) { return NewFeedReader(f) },
		},
		{
			name:    "mediaplayer",
			fixture: "mediaplayer.yml",
			newFn:   func(f fs.FS) (Parser, error) { return NewMediaPlayer(f) },
		},
		{
			name:    "mobile app",
			fixture: "mobile_apps.yml",
			newFn:   func(f fs.FS) (Parser, error) { return NewMobileApp(f) },
		},
	}

	const perFile = 3

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := tc.newFn(fsys)
			if err != nil {
				t.Fatalf("new parser: %v", err)
			}
			p.SetVersionTruncation(parser.VersionTruncationNone)

			fixtures := loadClientFixtures(t, tc.fixture)
			if len(fixtures) < perFile {
				t.Fatalf("fixture %s has only %d entries", tc.fixture, len(fixtures))
			}

			for _, fx := range fixtures[:perFile] {
				t.Run(fx.UserAgent, func(t *testing.T) {
					got, err := p.Parse(fx.UserAgent)
					if err != nil {
						t.Fatalf("parse: %v", err)
					}
					if got == nil {
						t.Fatalf("no client detected, want %q", fx.Client.Name)
					}

					if got.Type != fx.Client.Type {
						t.Errorf("type = %q, want %q", got.Type, fx.Client.Type)
					}
					if got.Name != fx.Client.Name {
						t.Errorf("name = %q, want %q", got.Name, fx.Client.Name)
					}
					if got.Version != fx.Client.Version {
						t.Errorf("version = %q, want %q", got.Version, fx.Client.Version)
					}
				})
			}
		})
	}
}

func TestAllOrder(t *testing.T) {
	parsers, err := All(regexFS())
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	want := []string{"feed reader", "mobile app", "mediaplayer", "pim", "browser", "library"}
	if len(parsers) != len(want) {
		t.Fatalf("All returned %d parsers, want %d", len(parsers), len(want))
	}

	for i, p := range parsers {
		if p.Name() != want[i] {
			t.Errorf("parser[%d] = %q, want %q", i, p.Name(), want[i])
		}
	}
}
