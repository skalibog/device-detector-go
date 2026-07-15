package device

import (
	"os"
	"testing"
)

func TestParse(t *testing.T) {
	fsys := os.DirFS("../../data/regexes")

	mobile, err := NewMobile(fsys)
	if err != nil {
		t.Fatalf("NewMobile: %v", err)
	}

	console, err := NewConsole(fsys)
	if err != nil {
		t.Fatalf("NewConsole: %v", err)
	}

	hbbTv, err := NewHbbTv(fsys)
	if err != nil {
		t.Fatalf("NewHbbTv: %v", err)
	}

	parsers := map[string]Parser{
		"mobile":  mobile,
		"console": console,
		"tv":      hbbTv,
	}

	// Expected values are taken from testdata/fixtures/{smartphone-1,console,tv}.yml.
	tests := []struct {
		name      string
		parser    string
		ua        string
		wantType  string
		wantBrand string
		wantModel string
	}{
		{
			name:      "advan 5061",
			parser:    "mobile",
			ua:        "Mozilla/5.0 (Linux; Android 7.0; 5061 Build/NRD90M; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/59.0.3071.125 Mobile Safari/537.36",
			wantType:  "smartphone",
			wantBrand: "Advan",
			wantModel: "5061",
		},
		{
			name:      "advan i lite i7u",
			parser:    "mobile",
			ua:        "Mozilla/5.0 (Linux; Android 7.0; i7U Build/NRD90M; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/63.0.3239.111 Mobile Safari/537.36",
			wantType:  "smartphone",
			wantBrand: "Advan",
			wantModel: "I Lite i7U",
		},
		{
			name:      "advan i4u",
			parser:    "mobile",
			ua:        "Mozilla/5.0 (Linux; Android 7.0; i4U Build/NRD90M; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/63.0.3239.111 Mobile Safari/537.36",
			wantType:  "smartphone",
			wantBrand: "Advan",
			wantModel: "I4U",
		},
		{
			name:      "advan i55d",
			parser:    "mobile",
			ua:        "Mozilla/5.0 (Linux; Android 7.0; i55D Build/NRD90M; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/63.0.3239.111 Mobile Safari/537.36",
			wantType:  "smartphone",
			wantBrand: "Advan",
			wantModel: "I55D",
		},
		{
			name:      "archos gamepad",
			parser:    "console",
			ua:        "Mozilla/5.0 (Linux; Android 4.1.1; ARCHOS GAMEPAD Build/JRO03H) AppleWebKit/535.19 (KHTML, like Gecko) Chrome/18.0.1025.166 Safari/535.19",
			wantType:  "console",
			wantBrand: "Archos",
			wantModel: "Gamepad",
		},
		{
			name:      "archos gamepad 2",
			parser:    "console",
			ua:        "Mozilla/5.0 (Linux; U; Android 4.2.2; fr-fr; ARCHOS GAMEPAD2 Build/JDQ39) AppleWebKit/534.30 (KHTML, like Gecko) Version/4.0 Safari/534.30",
			wantType:  "console",
			wantBrand: "Archos",
			wantModel: "Gamepad 2",
		},
		{
			name:      "xbox 360",
			parser:    "console",
			ua:        "Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; Trident/5.0; Xbox)",
			wantType:  "console",
			wantBrand: "Microsoft",
			wantModel: "Xbox 360",
		},
		{
			name:      "xbox one",
			parser:    "console",
			ua:        "Mozilla/5.0 (compatible; MSIE 10.0; Windows NT 6.2; Trident/6.0; Xbox; Xbox One)",
			wantType:  "console",
			wantBrand: "Microsoft",
			wantModel: "Xbox One",
		},
		{
			name:      "nintendo switch",
			parser:    "console",
			ua:        "Mozilla/5.0 (Nintendo Switch; WifiWebAuthApplet) AppleWebKit/601.6 (KHTML, like Gecko) NF/4.0.0.7.9 NintendoBrowser/5.1.0.15785",
			wantType:  "console",
			wantBrand: "Nintendo",
			wantModel: "Switch",
		},
		{
			name:      "nintendo wii",
			parser:    "console",
			ua:        "Opera/9.30 (Nintendo Wii; U; ; 3642; en)",
			wantType:  "console",
			wantBrand: "Nintendo",
			wantModel: "Wii",
		},
		{
			name:      "hbbtv generic no brand",
			parser:    "tv",
			ua:        "HbbTV/1.1.1 (;;;;) Mozilla/5.0 (compatible; ANTGalio/3.0.2.1.22.43.08; Linux2.6.18-7.1/7405d0-smp)",
			wantType:  "tv",
			wantBrand: "",
			wantModel: "",
		},
		{
			name:      "hbbtv airties air7210",
			parser:    "tv",
			ua:        "Opera/9.80 (Linux mips; U; HbbTV/1.1.1 (+PVR+RTSP;Airties;Air7210;16999168;;); xx) Presto/2.10.287 Version/12.00",
			wantType:  "tv",
			wantBrand: "Airties",
			wantModel: "Air7210",
		},
		{
			name:      "hbbtv altech uec pvr9600",
			parser:    "tv",
			ua:        "Opera/9.80 (Linux mips; Opera TV Store/6162; HbbTV/1.2.1 (; Altech UEC; PVR9600; ; ; )) Presto/2.12.407 Version/12.51 Model/AltechMultimedia-TestingDevice",
			wantType:  "tv",
			wantBrand: "Altech UEC",
			wantModel: "PVR9600",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsers[tt.parser].Parse(tt.ua)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			if got == nil {
				t.Fatalf("Parse returned no match")
			}

			if name := TypeName(got.Type); name != tt.wantType {
				t.Errorf("type = %q, want %q", name, tt.wantType)
			}

			if got.Brand != tt.wantBrand {
				t.Errorf("brand = %q, want %q", got.Brand, tt.wantBrand)
			}

			if got.Model != tt.wantModel {
				t.Errorf("model = %q, want %q", got.Model, tt.wantModel)
			}
		})
	}
}

func TestNoMatch(t *testing.T) {
	fsys := os.DirFS("../../data/regexes")

	console, err := NewConsole(fsys)
	if err != nil {
		t.Fatalf("NewConsole: %v", err)
	}

	// A plain desktop UA must not match the console parser.
	got, err := console.Parse("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got != nil {
		t.Errorf("expected no match, got %+v", got)
	}
}

func TestTypeNameRoundTrip(t *testing.T) {
	tests := []struct {
		id   int
		name string
	}{
		{TypeDesktop, "desktop"},
		{TypeSmartphone, "smartphone"},
		{TypeTablet, "tablet"},
		{TypeFeaturePhone, "feature phone"},
		{TypeConsole, "console"},
		{TypeTV, "tv"},
		{TypeCarBrowser, "car browser"},
		{TypeSmartDisplay, "smart display"},
		{TypeCamera, "camera"},
		{TypePortableMediaPlayer, "portable media player"},
		{TypePhablet, "phablet"},
		{TypeSmartSpeaker, "smart speaker"},
		{TypeWearable, "wearable"},
		{TypePeripheral, "peripheral"},
	}

	for _, tt := range tests {
		if got := TypeName(tt.id); got != tt.name {
			t.Errorf("TypeName(%d) = %q, want %q", tt.id, got, tt.name)
		}

		if got := TypeFromName(tt.name); got != tt.id {
			t.Errorf("TypeFromName(%q) = %d, want %d", tt.name, got, tt.id)
		}
	}

	if got := TypeName(TypeUnknown); got != "" {
		t.Errorf("TypeName(TypeUnknown) = %q, want %q", got, "")
	}

	if got := TypeFromName("nonsense"); got != TypeUnknown {
		t.Errorf("TypeFromName(nonsense) = %d, want %d", got, TypeUnknown)
	}
}

func TestAllOrder(t *testing.T) {
	fsys := os.DirFS("../../data/regexes")

	all, err := All(fsys)
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	want := []string{
		"tv",                  // HbbTv
		"shelltv",             // ShellTv
		"notebook",            // Notebook
		"console",             // Console
		"car browser",         // CarBrowser
		"camera",              // Camera
		"portablemediaplayer", // PortableMediaPlayer
		"mobile",              // Mobile
	}

	if len(all) != len(want) {
		t.Fatalf("All returned %d parsers, want %d", len(all), len(want))
	}

	for i, name := range want {
		if got := all[i].Name(); got != name {
			t.Errorf("All[%d].Name() = %q, want %q", i, got, name)
		}
	}
}
