package client

import (
	"testing"

	"github.com/skalibog/device-detector-go/parser"
)

func TestBrowserParse(t *testing.T) {
	b, err := NewBrowser(regexFS())
	if err != nil {
		t.Fatalf("NewBrowser: %v", err)
	}
	b.SetVersionTruncation(parser.VersionTruncationNone)

	for _, fx := range loadClientFixtures(t, "desktop-1.yml") {
		t.Run(fx.UserAgent, func(t *testing.T) {
			got, err := b.Parse(fx.UserAgent)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got == nil {
				t.Fatalf("no browser detected, want %q", fx.Client.Name)
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
			if got.Engine != fx.Client.Engine {
				t.Errorf("engine = %q, want %q", got.Engine, fx.Client.Engine)
			}
			if got.EngineVersion != fx.Client.EngineVersion {
				t.Errorf("engine_version = %q, want %q", got.EngineVersion, fx.Client.EngineVersion)
			}
			if got.Family != fx.BrowserFamily {
				t.Errorf("family = %q, want %q", got.Family, fx.BrowserFamily)
			}
		})
	}
}

func TestBrowserNoMatch(t *testing.T) {
	b, err := NewBrowser(regexFS())
	if err != nil {
		t.Fatalf("NewBrowser: %v", err)
	}

	// A PhantomJS user agent must be suppressed even though it matches a
	// browser regex, mirroring Browser::parse().
	got, err := b.Parse("Mozilla/5.0 (Unknown; Linux x86_64) AppleWebKit/534.34 (KHTML, like Gecko) PhantomJS/2.1.1 Safari/534.34")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != nil {
		t.Errorf("expected no browser for PhantomJS ua, got %+v", got)
	}
}

func TestBrowserFamily(t *testing.T) {
	cases := []struct {
		label  string
		want   string
		wantOK bool
	}{
		{"CH", "Chrome", true},
		{"FF", "Firefox", true},
		{"SF", "Safari", true},
		{"Chrome", "Chrome", true},        // resolved by full name
		{"Mobile Safari", "Safari", true}, // full name -> MF -> Safari
		{"ZZ", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got, ok := BrowserFamily(tc.label)
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("BrowserFamily(%q) = (%q, %v), want (%q, %v)", tc.label, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestIsMobileOnlyBrowser(t *testing.T) {
	cases := []struct {
		browser string
		want    bool
	}{
		{"MF", true},            // Mobile Safari short code
		{"Mobile Safari", true}, // full name
		{"CH", false},           // Chrome is not mobile-only
		{"Chrome", false},
	}

	for _, tc := range cases {
		t.Run(tc.browser, func(t *testing.T) {
			if got := IsMobileOnlyBrowser(tc.browser); got != tc.want {
				t.Errorf("IsMobileOnlyBrowser(%q) = %v, want %v", tc.browser, got, tc.want)
			}
		})
	}
}
