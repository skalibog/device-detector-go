package parser

import (
	"os"
	"testing"
)

func newTestOS(t *testing.T) *OS {
	t.Helper()

	p, err := NewOS(os.DirFS("../data/regexes"))
	if err != nil {
		t.Fatalf("NewOS: %v", err)
	}

	// Fixtures were generated with no version truncation.
	p.SetVersionTruncation(VersionTruncationNone)

	return p
}

// TestOSParse checks real UA -> OS pairs taken from testdata/fixtures/desktop.yml.
func TestOSParse(t *testing.T) {
	p := newTestOS(t)

	tests := []struct {
		name         string
		ua           string
		wantName     string
		wantVersion  string
		wantPlatform string
		wantFamily   string
	}{
		{
			name:       "amigaos no version",
			ua:         "Mozilla/6.0 (Macintosh; U; Amiga-AWeb) Safari 3.1",
			wantName:   "AmigaOS",
			wantFamily: "AmigaOS",
		},
		{
			name:        "amigaos with version",
			ua:          "IBrowse/2.4 (AmigaOS 3.9; 68K)",
			wantName:    "AmigaOS",
			wantVersion: "3.9",
			wantFamily:  "AmigaOS",
		},
		{
			name:         "centos maps nt-like version and x86",
			ua:           "Mozilla/5.0 (X11; U; Linux i686; en-US; rv:1.9.2.13) Gecko/20101209 CentOS/3.6-2.el5.centos Firefox/3.6.13",
			wantName:     "CentOS",
			wantVersion:  "5",
			wantPlatform: "x86",
			wantFamily:   "GNU/Linux",
		},
		{
			name:         "generic gnu linux x86",
			ua:           "NCSA_Mosaic/2.7b5 (X11;Linux 2.6.7 i686) libwww/2.12 modified",
			wantName:     "GNU/Linux",
			wantPlatform: "x86",
			wantFamily:   "GNU/Linux",
		},
		{
			name:         "windows 7 from nt 6.1 x64",
			ua:           "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML like Gecko) Chrome/33.0.1750.91 Safari/537.36 OPR/20.0.1387.37 (Edition Next-Campaign 21)",
			wantName:     "Windows",
			wantVersion:  "7",
			wantPlatform: "x64",
			wantFamily:   "Windows",
		},
		{
			name:         "chrome os keeps full version under no truncation",
			ua:           "Mozilla/5.0 (X11; CrOS x86_64 4731.101.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/31.0.1650.67 Safari/537.36",
			wantName:     "Chrome OS",
			wantVersion:  "4731.101.0",
			wantPlatform: "x64",
			wantFamily:   "Chrome OS",
		},
		{
			name:         "arch linux x64",
			ua:           "Mozilla/5.0 ArchLinux (X11; U; Linux x86_64; en-US) AppleWebKit/534.30 (KHTML, like Gecko) Chrome/12.0.742.100 Safari/534.30",
			wantName:     "Arch Linux",
			wantPlatform: "x64",
			wantFamily:   "GNU/Linux",
		},
		{
			name:       "beos",
			ua:         "Mozilla/3.0 (compatible; NetPositive/2.2.1; BeOS)",
			wantName:   "BeOS",
			wantFamily: "BeOS",
		},
		{
			name:       "mac from device fragment",
			ua:         "Linux UPnP/1.0 Sonos/81.1-58074 (MDCR_Mac15,12)",
			wantName:   "Mac",
			wantFamily: "Mac",
		},
		{
			name:         "sailfish wins over windows nt token",
			ua:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/47.0.2526.111 Safari/537.36 SailfishBrowser/Rulz ~LenovoG780",
			wantName:     "Sailfish OS",
			wantPlatform: "x64",
			wantFamily:   "GNU/Linux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.Parse(tt.ua)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			if got == nil {
				t.Fatalf("Parse(%q) = nil, want name %q", tt.ua, tt.wantName)
			}

			if got.Name != tt.wantName || got.Version != tt.wantVersion || got.Platform != tt.wantPlatform {
				t.Errorf("Parse(%q)\n got  name=%q version=%q platform=%q\n want name=%q version=%q platform=%q",
					tt.ua, got.Name, got.Version, got.Platform, tt.wantName, tt.wantVersion, tt.wantPlatform)
			}

			if got.Family != tt.wantFamily {
				t.Errorf("Parse(%q) family = %q, want %q", tt.ua, got.Family, tt.wantFamily)
			}
		})
	}
}

// TestOSParseNoMatch verifies that an OS-less string yields no result.
func TestOSParseNoMatch(t *testing.T) {
	p := newTestOS(t)

	got, err := p.Parse("qzxwvu-not-a-user-agent-9999")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got != nil {
		t.Errorf("Parse(non-UA) = %+v, want nil", *got)
	}
}

// TestOSVersionTruncationDefault checks that the default truncation is minor.
func TestOSVersionTruncationDefault(t *testing.T) {
	p, err := NewOS(os.DirFS("../data/regexes"))
	if err != nil {
		t.Fatalf("NewOS: %v", err)
	}

	got, err := p.Parse("Mozilla/5.0 (X11; CrOS x86_64 4731.101.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/31.0.1650.67 Safari/537.36")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got == nil || got.Version != "4731.101" {
		t.Fatalf("default minor truncation: got %+v, want version 4731.101", got)
	}
}

func TestOSFamily(t *testing.T) {
	tests := []struct {
		label      string
		wantFamily string
		wantOK     bool
	}{
		{"WIN", "Windows", true},
		{"Windows", "Windows", true},
		{"AND", "Android", true},
		{"Android", "Android", true},
		{"MAC", "Mac", true},
		{"IOS", "iOS", true},
		{"ZZZ", "", false},
	}

	for _, tt := range tests {
		gotFamily, gotOK := OSFamily(tt.label)
		if gotFamily != tt.wantFamily || gotOK != tt.wantOK {
			t.Errorf("OSFamily(%q) = (%q, %v), want (%q, %v)", tt.label, gotFamily, gotOK, tt.wantFamily, tt.wantOK)
		}
	}
}

func TestOSShortName(t *testing.T) {
	tests := []struct {
		name      string
		wantShort string
		wantOK    bool
	}{
		{"Android", "AND", true},
		{"Windows", "WIN", true},
		{"GNU/Linux", "LIN", true},
		{"Chrome OS", "COS", true},
		{"Nope OS", "", false},
	}

	for _, tt := range tests {
		gotShort, gotOK := OSShortName(tt.name)
		if gotShort != tt.wantShort || gotOK != tt.wantOK {
			t.Errorf("OSShortName(%q) = (%q, %v), want (%q, %v)", tt.name, gotShort, gotOK, tt.wantShort, tt.wantOK)
		}
	}
}

func TestIsDesktopOS(t *testing.T) {
	tests := []struct {
		label string
		want  bool
	}{
		{"WIN", true},
		{"MAC", true},
		{"LIN", true},
		{"AND", false},
		{"IOS", false},
	}

	for _, tt := range tests {
		if got := IsDesktopOS(tt.label); got != tt.want {
			t.Errorf("IsDesktopOS(%q) = %v, want %v", tt.label, got, tt.want)
		}
	}
}
