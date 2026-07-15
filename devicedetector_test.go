package devicedetector

import (
	"fmt"
	"sync"
	"testing"

	"github.com/skalibog/devicedetector/parser"
	"github.com/skalibog/devicedetector/parser/device"
)

func newDetector(t *testing.T, opts ...Option) *DeviceDetector {
	t.Helper()

	d, err := New(opts...)
	if err != nil {
		t.Fatal(err)
	}

	return d
}

func TestParseEmptyAndGarbage(t *testing.T) {
	d := newDetector(t)

	for _, ua := range []string{"", "1234567890", "-.;()"} {
		info, err := d.Parse(ua)
		if err != nil {
			t.Fatal(err)
		}

		if info.IsBot() || info.Client() != nil || info.OS() != nil ||
			info.Device() != device.TypeUnknown || info.DeviceName() != "" {
			t.Errorf("UA %q: expected fully-unknown result", ua)
		}
	}
}

func TestParseBot(t *testing.T) {
	d := newDetector(t)

	info, err := d.Parse("Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)")
	if err != nil {
		t.Fatal(err)
	}

	if !info.IsBot() || info.Bot().Name != "Googlebot" {
		t.Fatalf("expected Googlebot, got %+v", info.Bot())
	}

	if info.Client() != nil || info.OS() != nil {
		t.Error("bot detection must short-circuit client/os parsing")
	}
}

func TestSkipBotDetection(t *testing.T) {
	d := newDetector(t, WithSkipBotDetection())

	info, err := d.Parse("Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)")
	if err != nil {
		t.Fatal(err)
	}

	if info.IsBot() {
		t.Error("bot detection should be skipped")
	}
}

// TestDeviceHeuristics exercises the DeviceDetector::parseDevice()
// post-detection chain branch by branch.
func TestDeviceHeuristics(t *testing.T) {
	d := newDetector(t)

	cases := []struct {
		name     string
		ua       string
		wantType int
	}{
		{
			"chrome android with Mobile keyword -> smartphone",
			"Mozilla/5.0 (Linux; Android 10) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
			device.TypeSmartphone,
		},
		{
			"chrome android without Mobile keyword -> tablet",
			"Mozilla/5.0 (Linux; Android 10) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			device.TypeTablet,
		},
		{
			"android mobile fragment -> smartphone",
			"Mozilla/5.0 (Android; Mobile; rv:40.0) Gecko/40.0 Firefox/40.0",
			device.TypeSmartphone,
		},
		{
			"windows RT touch -> tablet",
			"Mozilla/5.0 (Windows NT 6.2; ARM; Trident/6.0; Touch; rv:11.0) like Gecko",
			device.TypeTablet,
		},
		{
			"KaiOS -> feature phone",
			"Mozilla/5.0 (Mobile; LYF/F41t/LYF-F41t-000-02-24-130318;Android;rv:48.0) Gecko/48.0 Firefox/48.0 KAIOS/2.5",
			device.TypeFeaturePhone,
		},
		{
			"desktop os fallback -> desktop",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
			device.TypeDesktop,
		},
		{
			// Regression: DeviceDetector::matchUserAgent() allows a digit
			// before the match (' TV$' after 'Safari/537.36'), unlike the
			// stricter parser anchor.
			"trailing TV token after digit -> tv",
			"Mozilla/5.0 (Linux; Android 13; AKAI_TA43BU500 Build/TP1A.220905.004.A2; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/101.0.4951.61 YaBrowser/23.1.0.39 (lite) Safari/537.36 TV",
			device.TypeTV,
		},
		{
			"Opera TV Store -> tv",
			"Mozilla/5.0 (Linux; U) AppleWebKit/538.1 (KHTML, like Gecko) Opera TV Store Safari/538.1",
			device.TypeTV,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info, err := d.Parse(c.ua)
			if err != nil {
				t.Fatal(err)
			}

			if info.Device() != c.wantType {
				t.Errorf("device type = %q, want %q", info.DeviceName(), device.TypeName(c.wantType))
			}
		})
	}
}

func TestAppleBrandAssumption(t *testing.T) {
	d := newDetector(t)

	info, err := d.Parse("Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1")
	if err != nil {
		t.Fatal(err)
	}

	if info.Brand() != "Apple" || info.Device() != device.TypeSmartphone {
		t.Errorf("expected Apple smartphone, got brand=%q type=%q", info.Brand(), info.DeviceName())
	}

	if !info.IsMobile() || info.IsDesktop() {
		t.Error("iPhone must be mobile, not desktop")
	}
}

func TestVersionTruncationOption(t *testing.T) {
	d := newDetector(t) // default: minor

	info, err := d.Parse("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.109 Safari/537.36")
	if err != nil {
		t.Fatal(err)
	}

	if got := info.Client().Version; got != "120.0" {
		t.Errorf("default truncation: version = %q, want 120.0", got)
	}

	full := newDetector(t, WithVersionTruncation(parser.VersionTruncationNone))

	info, err = full.Parse("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.109 Safari/537.36")
	if err != nil {
		t.Fatal(err)
	}

	if got := info.Client().Version; got != "120.0.6099.109" {
		t.Errorf("truncation none: version = %q, want 120.0.6099.109", got)
	}
}

func TestNewFromDirMissing(t *testing.T) {
	if _, err := NewFromDir("/nonexistent/regexes"); err == nil {
		t.Error("expected error for missing regex directory")
	}
}

func TestVersionCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2", "2.0", -1},
		{"2.0", "2", 1},
		{"2.0", "2.0", 0},
		{"8.1", "8", 1},
		{"4.4.2", "4.0", 1},
		{"1.9", "2.0", -1},
		{"3.0", "4.0", -1},
		{"", "8", -1},
		{"10", "9", 1},
	}

	for _, c := range cases {
		if got := versionCompare(c.a, c.b); got != c.want {
			t.Errorf("versionCompare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestMatchUAAnchorAllowsDigit(t *testing.T) {
	if !matchUA("Safari/537.36 TV", ` TV$`) {
		t.Error("DeviceDetector anchor must allow a digit before the match")
	}

	if m, _ := parser.MatchUserAgent("Safari/537.36 TV", ` TV$`); m != nil {
		t.Error("parser anchor must reject a digit before the match")
	}

	if matchUA("ua", `(broken`) {
		t.Error("invalid pattern must not match")
	}
}

// TestConcurrentParseDeterminism runs the same UA set from many goroutines
// against one shared detector and requires bit-identical results.
func TestConcurrentParseDeterminism(t *testing.T) {
	d := newDetector(t)

	uas := benchUAs

	serialize := func(info *Info) string {
		s := fmt.Sprintf("bot=%v type=%s brand=%s model=%s", info.IsBot(), info.DeviceName(), info.Brand(), info.Model())

		if info.Client() != nil {
			s += fmt.Sprintf(" client=%s/%s/%s", info.Client().Name, info.Client().Version, info.Client().Engine)
		}

		if info.OS() != nil {
			s += fmt.Sprintf(" os=%s/%s", info.OS().Name, info.OS().Version)
		}

		return s
	}

	want := make([]string, len(uas))

	for i, ua := range uas {
		info, err := d.Parse(ua)
		if err != nil {
			t.Fatal(err)
		}

		want[i] = serialize(info)
	}

	var wg sync.WaitGroup

	errs := make(chan string, 64)

	for g := 0; g < 32; g++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for iter := 0; iter < 50; iter++ {
				for i, ua := range uas {
					info, err := d.Parse(ua)
					if err != nil {
						errs <- err.Error()
						return
					}

					if got := serialize(info); got != want[i] {
						errs <- fmt.Sprintf("UA %d diverged:\n got %s\nwant %s", i, got, want[i])
						return
					}
				}
			}
		}()
	}

	wg.Wait()
	close(errs)

	for e := range errs {
		t.Error(e)
	}
}
