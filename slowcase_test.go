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
// the database fails to match). With the RE2 prefilter this parses in ~60 ms
// on fast hardware; the bound below is the regression tripwire for the
// prefilter itself.
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

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("aggregate walk took %v for %d-byte junk (bound 5s)", elapsed, len(ua))
	}

	if info == nil {
		t.Fatal("Parse returned nil Info")
	}
}

// TestNonASCIISlowCases replays the nightly-fuzz findings from issues #26/#27:
// invalid-UTF-8 filler behind text that pattern literals anchor on (an
// "Android" fragment in one case, digit runs in the other). The anchor admits
// patterns through the literal probes, and regexp2 then backtracks over every
// replacement-rune position — with IgnoreCase paying a full unicode.ToLower
// per rune, which never hits the ASCII fast path. The prefilter took these
// from 6-7 s (slow runner) to under 100 ms on dev hardware; kept as committed
// unit regressions rather than fuzz seeds so mutations around them cannot
// flake the PR fuzz smoke.
func TestNonASCIISlowCases(t *testing.T) {
	if raceEnabled {
		t.Skip("race instrumentation slows backtracking 10-20x; wall-time bound is meaningless")
	}

	d, err := New()
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range []string{
		"testdata/slowcases/nonascii-android-ea.bin",
		"testdata/slowcases/nonascii-digits-b1.bin",
	} {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}

		start := time.Now()

		info, perr := d.Parse(string(raw))
		_ = perr // adversarial input: an error is acceptable, a hang is not

		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Fatalf("%s took %v (bound 5s)", f, elapsed)
		}

		if info == nil {
			t.Fatalf("%s: Parse returned nil Info", f)
		}
	}
}
