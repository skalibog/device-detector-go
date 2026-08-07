package devicedetector

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// placeholderRe detects an unsubstituted $1..$N capture-group placeholder that
// leaked into an output field — a BuildByMatch bug.
var placeholderRe = regexp.MustCompile(`\$\d`)

// fuzzSeeds returns a diverse but bounded set of real user agents from the
// fixture corpus — a few per file so every device class is represented without
// making the seed replay (which runs on every `go test`) slow.
func fuzzSeeds(tb testing.TB) []string {
	tb.Helper()

	files, err := filepath.Glob("testdata/fixtures/*.yml")
	if err != nil {
		tb.Fatal(err)
	}

	perFile := 6
	if testing.Short() {
		perFile = 1
	}

	line := regexp.MustCompile(`(?m)^\s*user_agent:\s*(.+?)\s*$`)

	var seeds []string

	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			tb.Fatalf("reading %s: %v", f, err)
		}

		n := 0
		for _, m := range line.FindAllStringSubmatch(string(raw), -1) {
			if ua := strings.Trim(m[1], `'"`); ua != "" {
				seeds = append(seeds, ua)

				if n++; n >= perFile {
					break
				}
			}
		}
	}

	return seeds
}

// FuzzParse drives Parse with arbitrary input and asserts its invariants: it
// never panics, always returns a non-nil Info (the partial-result contract),
// reports only known device types, never leaks a regex placeholder, and stays
// bounded in time (a regression guard for catastrophic backtracking, e.g. after
// a database sync).
func FuzzParse(f *testing.F) {
	for _, ua := range fuzzSeeds(f) {
		f.Add(ua)
	}

	det, err := New()
	if err != nil {
		f.Fatal(err)
	}

	f.Fuzz(func(t *testing.T, ua string) {
		start := time.Now()

		info, err := det.Parse(ua)
		_ = err // errors are acceptable on adversarial input; treat as unknown.

		// A single catastrophic pattern is already cut off by the 1 s match
		// timeout (it surfaces as an error, accepted above); wall time here
		// guards the aggregate walk, which the RE2 prefilter keeps under
		// ~60 ms even for near-cap worst-case junk on fast hardware — safely
		// inside 5 s (and the go fuzzing engine's ~10 s worker watchdog) on
		// slow shared runners too.
		limit := 5 * time.Second

		// Under the race detector regexp2 backtracking runs 10-20x slower, so a
		// tight wall-time bound would only measure the instrumentation; keep a
		// generous hang backstop there.
		if raceEnabled {
			limit = 5 * time.Minute
		}

		if elapsed := time.Since(start); elapsed > limit {
			t.Fatalf("Parse took %v (possible catastrophic backtracking) for %q", elapsed, ua)
		}

		if info == nil {
			t.Fatalf("Parse returned a nil Info (contract: never nil) for %q", ua)
		}

		if name := info.DeviceName(); name != "" && DeviceTypeFromName(name) == DeviceTypeUnknown {
			t.Fatalf("Parse reported an unknown device name %q for %q", name, ua)
		}

		// A UA containing a literal "$N" legitimately flows into results through
		// capture groups (e.g. "HUAWEI$0 Build" -> model "$0") — upstream's
		// buildByMatch is a plain str_replace, so this is parity, not a leak. A
		// "$N" token in a result is therefore a leak only when that exact token
		// never appeared in the input.
		leaked := func(field string) bool {
			for _, tok := range placeholderRe.FindAllString(field, -1) {
				if !strings.Contains(ua, tok) {
					return true
				}
			}

			return false
		}

		if info.Client() != nil {
			c := info.Client()
			for _, field := range []string{c.Name, c.Version, c.Engine, c.EngineVersion} {
				if leaked(field) {
					t.Fatalf("client field %q leaked a capture-group placeholder for %q", field, ua)
				}
			}
		}

		if leaked(info.Model()) || leaked(info.Brand()) {
			t.Fatalf("device brand/model leaked a placeholder (brand=%q model=%q) for %q", info.Brand(), info.Model(), ua)
		}
	})
}
