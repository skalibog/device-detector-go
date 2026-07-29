package parser

import (
	"reflect"
	"testing"
)

func TestClientHintsHeaders(t *testing.T) {
	ch := NewClientHintsFromMap(map[string]any{
		"sec-ch-ua":                  `"Opera";v="83", " Not;A Brand";v="99", "Chromium";v="98"`,
		"sec-ch-ua-mobile":           "?0",
		"sec-ch-ua-platform":         "Windows",
		"sec-ch-ua-platform-version": "14.0.0",
	})

	if ch.IsMobile() {
		t.Error("mobile should be false")
	}
	if ch.Platform != "Windows" || ch.PlatformVersion != "14.0.0" {
		t.Errorf("os = %q %q", ch.Platform, ch.PlatformVersion)
	}

	want := []BrandVersion{{"Opera", "83"}, {" Not;A Brand", "99"}, {"Chromium", "98"}}
	if got := ch.BrandList(); !reflect.DeepEqual(got, want) {
		t.Errorf("BrandList = %v, want %v", got, want)
	}
}

func TestClientHintsHeadersHTTP(t *testing.T) {
	ch := NewClientHintsFromMap(map[string]any{
		"HTTP_SEC_CH_UA_FULL_VERSION_LIST": `" Not A;Brand";v="99.0.0.0", "Chromium";v="98.0.4758.82", "Opera";v="98.0.4758.82"`,
		"HTTP_SEC_CH_UA":                   `" Not A;Brand";v="99", "Chromium";v="98", "Opera";v="84"`,
		"HTTP_SEC_CH_UA_MOBILE":            "?1",
		"HTTP_SEC_CH_UA_MODEL":             "DN2103",
		"HTTP_SEC_CH_UA_PLATFORM":          "Ubuntu",
		"HTTP_SEC_CH_UA_PLATFORM_VERSION":  "3.7",
		"HTTP_SEC_CH_UA_FULL_VERSION":      "98.0.14335.105",
		"HTTP_SEC_CH_UA_FORM_FACTORS":      `"Desktop"`,
	})

	if !ch.IsMobile() {
		t.Error("mobile should be true")
	}
	if ch.Platform != "Ubuntu" || ch.PlatformVersion != "3.7" || ch.Model != "DN2103" {
		t.Errorf("os/model = %q %q %q", ch.Platform, ch.PlatformVersion, ch.Model)
	}

	// The full-version-list wins over sec-ch-ua regardless of header order.
	want := []BrandVersion{{" Not A;Brand", "99.0.0.0"}, {"Chromium", "98.0.4758.82"}, {"Opera", "98.0.4758.82"}}
	if got := ch.BrandList(); !reflect.DeepEqual(got, want) {
		t.Errorf("BrandList = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(ch.FormFactors, []string{"desktop"}) {
		t.Errorf("form factors = %v", ch.FormFactors)
	}
}

func TestClientHintsJavascript(t *testing.T) {
	ch := NewClientHintsFromMap(map[string]any{
		"fullVersionList": []map[string]string{
			{"brand": " Not A;Brand", "version": "99.0.0.0"},
			{"brand": "Chromium", "version": "99.0.4844.51"},
			{"brand": "Google Chrome", "version": "99.0.4844.51"},
		},
		"formFactors":     []string{"Desktop"},
		"mobile":          false,
		"model":           "",
		"platform":        "Windows",
		"platformVersion": "10.0.0",
	})

	if ch.IsMobile() || ch.Platform != "Windows" || ch.PlatformVersion != "10.0.0" || ch.Model != "" {
		t.Errorf("unexpected: mobile=%v os=%q %q model=%q", ch.IsMobile(), ch.Platform, ch.PlatformVersion, ch.Model)
	}

	want := []BrandVersion{{" Not A;Brand", "99.0.0.0"}, {"Chromium", "99.0.4844.51"}, {"Google Chrome", "99.0.4844.51"}}
	if got := ch.BrandList(); !reflect.DeepEqual(got, want) {
		t.Errorf("BrandList = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(ch.FormFactors, []string{"desktop"}) {
		t.Errorf("form factors = %v", ch.FormFactors)
	}
}

func TestClientHintsIncorrectVersionListDiscarded(t *testing.T) {
	ch := NewClientHintsFromMap(map[string]any{
		"fullVersionList": []map[string]string{
			{"brand": " Not A;Brand", "version": "99.0.0.0"},
			{"brand": "Chromium", "version": "99.0.4844.51"},
			{"version": "99.0.4844.51"}, // missing brand
		},
	})

	if got := ch.BrandList(); len(got) != 0 {
		t.Errorf("BrandList should be empty for a mismatched list, got %v", got)
	}
}

func TestClientHintsMalformedScalarsIgnored(t *testing.T) {
	// Scalar hints given as arrays must be ignored (strict type handling).
	ch := NewClientHintsFromMap(map[string]any{
		"architecture":                []string{"x86"},
		"bitness":                     []string{"64"},
		"model":                       []string{"DN2103"},
		"platform":                    []string{"Windows"},
		"platformVersion":             []string{"14.0.0"},
		"uaFullVersion":               []string{"98.0.14335.105"},
		"sec-ch-ua-full-version-list": []string{`"Chromium";v="98.0.4758.82"`},
		"sec-ch-ua-form-factors":      []any{[]any{"Desktop"}},
		"x-requested-with":            []string{"com.example.app"},
	})

	if ch.Architecture != "" || ch.Bitness != "" || ch.Model != "" || ch.Platform != "" ||
		ch.PlatformVersion != "" || ch.BrandVersion() != "" || ch.App != "" ||
		len(ch.BrandList()) != 0 || len(ch.FormFactors) != 0 {
		t.Errorf("malformed array-typed scalars must be ignored: %+v", ch)
	}
}

func TestRestoreUserAgent(t *testing.T) {
	ch := &ClientHints{Model: "Pixel 8", PlatformVersion: "14.0.0"}
	ua := "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Mobile Safari/537.36"

	got := ch.RestoreUserAgent(ua)
	want := "Mozilla/5.0 (Linux; Android 14.0.0; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Mobile Safari/537.36"
	if got != want {
		t.Errorf("restore:\n got %q\nwant %q", got, want)
	}

	if HasUserAgentClientHintsFragment("Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 Telegram-Android/11.0") {
		t.Error("Telegram-Android must be excluded from the CH fragment check")
	}

	if (&ClientHints{}).RestoreUserAgent(ua) != ua {
		t.Error("restore without a model must be a no-op")
	}
}

func TestClientHintsHTMLTagDiscarded(t *testing.T) {
	ch := NewClientHintsFromMap(map[string]any{
		"sec-ch-ua-platform": `Windows<script>alert(1)</script>`,
	})
	if ch.Platform != "" {
		t.Errorf("values containing HTML tags must be discarded, got %q", ch.Platform)
	}
}
