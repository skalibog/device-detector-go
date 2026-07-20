package devicedetector

import (
	"strings"
	"testing"
	"time"
)

// TestAdversarialInputBounded is the ReDoS regression guard: crafted user
// agents that pinned a core for tens of seconds before v0.1.2 must now return
// quickly under the default guards (length cap + per-match timeout).
func TestAdversarialInputBounded(t *testing.T) {
	if testing.Short() {
		// Wall-clock assertions are meaningless under the race detector's
		// slowdown (the -short CI job runs with -race); the full fixtures job
		// exercises this at real speed.
		t.Skip("timing assertion skipped in -short mode")
	}

	d := testDetector(t)

	cases := map[string]string{
		"repeated-a0-6kb":  strings.Repeat("a0 ", 2000),
		"repeated-a0-24kb": strings.Repeat("a0 ", 8000),
		"alnum-64kb":       strings.Repeat("a", 64*1024),
		"paren-flood-8kb":  strings.Repeat("(", 8192),
	}

	// Generous bound: real cost is ~1.3s on a fast machine, but CI runners are
	// slower. The point is to catch a regression back to tens of seconds.
	const limit = 5 * time.Second

	for name, ua := range cases {
		start := time.Now()

		if _, err := d.Parse(ua); err != nil {
			t.Logf("%s returned an error (acceptable — treat as unknown): %v", name, err)
		}

		if elapsed := time.Since(start); elapsed > limit {
			t.Errorf("%s took %v (> %v): DoS guard ineffective", name, elapsed, limit)
		}
	}
}

func TestMaxUARawLengthTruncates(t *testing.T) {
	long := strings.Repeat("x", 5000)

	d, err := New(WithMaxUARawLength(1000))
	if err != nil {
		t.Fatal(err)
	}

	info, err := d.Parse(long)
	if err != nil {
		t.Fatal(err)
	}

	if len(info.UserAgent) != 1000 {
		t.Errorf("truncated UserAgent length = %d, want 1000", len(info.UserAgent))
	}

	// A disabled cap keeps the full user agent (exact upstream parity).
	dOff, err := New(WithMaxUARawLength(0))
	if err != nil {
		t.Fatal(err)
	}

	info, err = dOff.Parse(long)
	if err != nil {
		t.Fatal(err)
	}

	if len(info.UserAgent) != 5000 {
		t.Errorf("disabled cap UserAgent length = %d, want 5000", len(info.UserAgent))
	}
}

func TestDefaultCapPreservesRealUA(t *testing.T) {
	// A normal user agent is well under the default cap and must be untouched.
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

	d := testDetector(t)

	info, err := d.Parse(ua)
	if err != nil {
		t.Fatal(err)
	}

	if info.UserAgent != ua {
		t.Error("default cap must not touch a normal-length user agent")
	}

	if info.Client() == nil || info.Client().Name != "Chrome" {
		t.Errorf("detection regressed under default guards: %+v", info.Client())
	}
}
