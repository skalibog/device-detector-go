package devicedetector

import (
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"
)

// TestAggregateWalkSeed replays the full-length worst-case input the nightly
// fuzzer found (near-cap junk with an unclosed parenthesis: every pattern in
// the database fails to match and backtracks). FuzzParse caps its inputs at
// 512 bytes to stay under the go fuzzing engine's ~10 s worker watchdog, so
// this test carries the full-length regression: the aggregate walk must stay
// bounded (~450 ms on a fast machine) until the RE2 prefilter (v1.2) collapses
// it, after which the limit here should tighten.
func TestAggregateWalkSeed(t *testing.T) {
	if raceEnabled {
		t.Skip("race instrumentation slows backtracking 10-20x; wall-time bound is meaningless")
	}

	raw, err := os.ReadFile("testdata/fuzz/FuzzParse/aggregate-walk-truncated-junk")
	if err != nil {
		t.Fatal(err)
	}

	// Corpus file format: `go test fuzz v1\nstring("...")` with a Go-quoted string.
	m := regexp.MustCompile(`(?s)string\((".*")\)`).FindSubmatch(raw)
	if m == nil {
		t.Fatal("seed file does not contain a quoted string")
	}

	ua, err := strconv.Unquote(string(m[1]))
	if err != nil {
		t.Fatalf("unquoting seed: %v", err)
	}

	d, err := New()
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()

	info, perr := d.Parse(ua)
	_ = perr // adversarial input: an error is acceptable, a hang is not

	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Fatalf("aggregate walk took %v for %d-byte junk (bound 15s)", elapsed, len(ua))
	}

	if info == nil {
		t.Fatal("Parse returned nil Info")
	}
}
