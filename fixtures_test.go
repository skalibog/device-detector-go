package devicedetector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/skalibog/device-detector-go/parser"
)

// flexString decodes any YAML scalar as its literal source text, so that
// `version: 7.0` stays "7.0" instead of collapsing to a float.
type flexString string

func (s *flexString) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected scalar, got kind %d", node.Kind)
	}

	if node.Tag == "!!null" {
		*s = ""
		return nil
	}

	*s = flexString(node.Value)

	return nil
}

type fxProducer struct {
	Name flexString `yaml:"name"`
	URL  flexString `yaml:"url"`
}

type fxBot struct {
	Name     flexString `yaml:"name"`
	Category flexString `yaml:"category"`
	URL      flexString `yaml:"url"`
	Producer fxProducer `yaml:"producer"`
}

type fxOS struct {
	Name     flexString `yaml:"name"`
	Version  flexString `yaml:"version"`
	Platform flexString `yaml:"platform"`
}

type fxClient struct {
	Type          flexString `yaml:"type"`
	Name          flexString `yaml:"name"`
	Version       flexString `yaml:"version"`
	Engine        flexString `yaml:"engine"`
	EngineVersion flexString `yaml:"engine_version"`
}

type fxDevice struct {
	Type  flexString `yaml:"type"`
	Brand flexString `yaml:"brand"`
	Model flexString `yaml:"model"`
}

type fixtureEntry struct {
	UserAgent string    `yaml:"user_agent"`
	Bot       yaml.Node `yaml:"bot"`
	OS        yaml.Node `yaml:"os"`
	Client    yaml.Node `yaml:"client"`
	Device    yaml.Node `yaml:"device"`
	Headers   yaml.Node `yaml:"headers"`
}

func decodeMapping[T any](node yaml.Node) *T {
	if node.Kind != yaml.MappingNode {
		return nil
	}

	out := new(T)
	if err := node.Decode(out); err != nil {
		return nil
	}

	return out
}

type mismatch struct {
	File      string `json:"file"`
	UserAgent string `json:"user_agent"`
	Dimension string `json:"dimension"`
	Expected  string `json:"expected"`
	Got       string `json:"got"`
}

type fixtureStats struct {
	mu         sync.Mutex
	total      int
	passed     int
	byDim      map[string]int
	mismatches []mismatch
}

func (s *fixtureStats) record(file, ua string, fails []mismatch) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.total++

	if len(fails) == 0 {
		s.passed++
		return
	}

	for _, f := range fails {
		s.byDim[f.Dimension]++
	}

	s.mismatches = append(s.mismatches, fails...)
}

func check(file, ua, dim, expected, got string, fails []mismatch) []mismatch {
	if expected == got {
		return fails
	}

	return append(fails, mismatch{File: file, UserAgent: ua, Dimension: dim, Expected: expected, Got: got})
}

// TestFixtures runs the upstream matomo fixture corpus against the port.
func TestFixtures(t *testing.T) {
	files, err := filepath.Glob("testdata/fixtures/*.yml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures found: %v", err)
	}

	detector, err := New(WithVersionTruncation(parser.VersionTruncationNone))
	if err != nil {
		t.Fatalf("constructing detector: %v", err)
	}

	shortSet := map[string]bool{
		"bots.yml": true, "smartphone-1.yml": true, "desktop.yml": true,
		"tablet.yml": true, "tv.yml": true, "unknown.yml": true,
	}

	stats := &fixtureStats{byDim: map[string]int{}}
	perFile := map[string]string{}

	var perFileMu sync.Mutex

	for _, file := range files {
		base := filepath.Base(file)

		if strings.HasPrefix(base, "clienthints") {
			continue // client hints support lands in v0.2
		}

		if testing.Short() && !shortSet[base] {
			continue
		}

		t.Run(base, func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}

			var entries []fixtureEntry
			if err := yaml.Unmarshal(raw, &entries); err != nil {
				t.Fatalf("parsing %s: %v", base, err)
			}

			filePassed, fileTotal, fileSkipped := 0, 0, 0

			for _, entry := range entries {
				// Entries with request headers exercise client hints,
				// which land in v0.2.
				if entry.Headers.Kind != 0 {
					fileSkipped++
					continue
				}

				info, err := detector.Parse(entry.UserAgent)
				if err != nil {
					t.Fatalf("parse error for %q: %v", entry.UserAgent, err)
				}

				var fails []mismatch

				if bot := decodeMapping[fxBot](entry.Bot); bot != nil {
					gotName := ""
					if info.Bot() != nil {
						gotName = info.Bot().Name
					}

					fails = check(base, entry.UserAgent, "bot.name", string(bot.Name), gotName, fails)
				} else {
					fails = compareRegular(base, entry, info, fails)
				}

				stats.record(base, entry.UserAgent, fails)

				fileTotal++
				if len(fails) == 0 {
					filePassed++
				}
			}

			skippedNote := ""
			if fileSkipped > 0 {
				skippedNote = fmt.Sprintf(" [%d client-hints entries skipped]", fileSkipped)
			}

			perFileMu.Lock()
			perFile[base] = fmt.Sprintf("%d/%d (%.1f%%)%s", filePassed, fileTotal, 100*float64(filePassed)/float64(max(fileTotal, 1)), skippedNote)
			perFileMu.Unlock()
		})
	}

	t.Cleanup(func() {
		names := make([]string, 0, len(perFile))
		for name := range perFile {
			names = append(names, name)
		}

		sort.Strings(names)

		for _, name := range names {
			t.Logf("%-28s %s", name, perFile[name])
		}

		stats.mu.Lock()
		defer stats.mu.Unlock()

		t.Logf("TOTAL: %d/%d (%.2f%%)", stats.passed, stats.total, 100*float64(stats.passed)/float64(max(stats.total, 1)))

		if stats.passed != stats.total {
			t.Errorf("fixture corpus regressed: %d mismatched entries", stats.total-stats.passed)
		}

		dims := make([]string, 0, len(stats.byDim))
		for dim := range stats.byDim {
			dims = append(dims, dim)
		}

		sort.Slice(dims, func(a, b int) bool { return stats.byDim[dims[a]] > stats.byDim[dims[b]] })

		for _, dim := range dims {
			t.Logf("mismatches %-24s %d", dim, stats.byDim[dim])
		}

		if report := os.Getenv("DD_FIXTURE_REPORT"); report != "" {
			data, _ := json.MarshalIndent(stats.mismatches, "", " ")
			if err := os.WriteFile(report, data, 0o644); err != nil {
				t.Logf("writing mismatch report: %v", err)
			} else {
				t.Logf("mismatch report: %s", report)
			}
		}
	})
}

func compareRegular(base string, entry fixtureEntry, info *Info, fails []mismatch) []mismatch {
	ua := entry.UserAgent

	if expOS := decodeMapping[fxOS](entry.OS); expOS != nil {
		gotName, gotVersion, gotPlatform := "", "", ""
		if info.OS() != nil {
			gotName, gotVersion, gotPlatform = info.OS().Name, info.OS().Version, info.OS().Platform
		}

		fails = check(base, ua, "os.name", string(expOS.Name), gotName, fails)
		fails = check(base, ua, "os.version", string(expOS.Version), gotVersion, fails)
		fails = check(base, ua, "os.platform", string(expOS.Platform), gotPlatform, fails)
	}

	if expClient := decodeMapping[fxClient](entry.Client); expClient != nil {
		gotType, gotName, gotVersion, gotEngine, gotEngineVersion := "", "", "", "", ""
		if info.Client() != nil {
			c := info.Client()
			gotType, gotName, gotVersion, gotEngine, gotEngineVersion = c.Type, c.Name, c.Version, c.Engine, c.EngineVersion
		}

		fails = check(base, ua, "client.type", string(expClient.Type), gotType, fails)
		fails = check(base, ua, "client.name", string(expClient.Name), gotName, fails)
		fails = check(base, ua, "client.version", string(expClient.Version), gotVersion, fails)

		if expClient.Type == "browser" {
			fails = check(base, ua, "client.engine", string(expClient.Engine), gotEngine, fails)
			fails = check(base, ua, "client.engine_version", string(expClient.EngineVersion), gotEngineVersion, fails)
		}
	} else if info.Client() != nil {
		fails = append(fails, mismatch{File: base, UserAgent: ua, Dimension: "client.spurious", Expected: "", Got: info.Client().Name})
	}

	if expDevice := decodeMapping[fxDevice](entry.Device); expDevice != nil {
		fails = check(base, ua, "device.type", string(expDevice.Type), info.DeviceName(), fails)
		fails = check(base, ua, "device.brand", string(expDevice.Brand), info.Brand(), fails)
		fails = check(base, ua, "device.model", string(expDevice.Model), info.Model(), fails)
	}

	return fails
}
