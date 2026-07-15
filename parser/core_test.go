package parser

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNormalizePattern(t *testing.T) {
	cases := []struct{ in, want string }{
		{`plain`, `plain`},
		{`SYMPHONY[ \_]`, `SYMPHONY[ _]`},
		{`a\_b\_c`, `a_b_c`},
		{`JY-G4[\\_]G5`, `JY-G4[\\_]G5`}, // escaped backslash + literal _, must stay
		{`\\\_`, `\\_`},                  // escaped backslash, then \_ -> _
		{`end\`, `end\`},
	}

	for _, c := range cases {
		if got := normalizePattern(c.in); got != c.want {
			t.Errorf("normalizePattern(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMatchUserAgentAnchor(t *testing.T) {
	cases := []struct {
		ua, pattern string
		want        bool
	}{
		{"Opera/9.80", "Opera", true},                   // start of string
		{"Mozilla (Opera)", "Opera", true},              // preceded by non-alphanumeric
		{"XOpera/1.0", "Opera", false},                  // preceded by a letter
		{"9Opera/1.0", "Opera", false},                  // preceded by a digit (parser anchor)
		{"sprd-Galaxy", "Galaxy", true},                 // sprd- exception
		{"MZ-Meizu M6", "Meizu", true},                  // MZ- exception
		{"foo_Browser", "Browser", false},               // letter+underscore prefix blocked
		{"foo;_Browser", "Browser", true},               // non-alnum + underscore allowed
		{"Tablet PC 2.0", `Tablet(?! PC)`, false},       // lookahead must work (regexp2)
		{"Android Tablet; rv:1", `Tablet(?! PC)`, true}, // lookahead pass-through
	}

	for _, c := range cases {
		m, err := MatchUserAgent(c.ua, c.pattern)
		if err != nil {
			t.Fatalf("MatchUserAgent(%q, %q): %v", c.ua, c.pattern, err)
		}

		if (m != nil) != c.want {
			t.Errorf("MatchUserAgent(%q, %q) matched=%v, want %v", c.ua, c.pattern, m != nil, c.want)
		}
	}
}

func TestMatchUserAgentGroups(t *testing.T) {
	m, err := MatchUserAgent("Chrome/119.0.6045.194 Mobile", `Chrome/(\d+)\.(\d+)(?:\.(\d+))?(?:\.(\d+))?`)
	if err != nil || m == nil {
		t.Fatalf("expected match, got m=%v err=%v", m, err)
	}

	want := []string{"119", "0", "6045", "194"}
	for i, w := range want {
		if m[i+1] != w {
			t.Errorf("group %d = %q, want %q", i+1, m[i+1], w)
		}
	}
}

func TestCompileError(t *testing.T) {
	if _, err := Compile(`(unbalanced`); err == nil {
		t.Error("expected compile error for unbalanced pattern")
	}
}

func TestCombineRegexes(t *testing.T) {
	got := CombineRegexes([]string{"a", "b", "c"})
	if got != "c|b|a" {
		t.Errorf("CombineRegexes = %q, want c|b|a", got)
	}
}

func TestBuildByMatch(t *testing.T) {
	cases := []struct {
		item    string
		matches []string
		want    string
	}{
		{"Model $1", []string{"full", "X10"}, "Model X10"},
		{"$1 $2", []string{"full", "A", ""}, "A"}, // empty group + trim
		{"$1$2", []string{"full", "A"}, "A"},      // trailing placeholder -> ""
		{"static", []string{"full"}, "static"},
	}

	for _, c := range cases {
		if got := BuildByMatch(c.item, c.matches); got != c.want {
			t.Errorf("BuildByMatch(%q, %v) = %q, want %q", c.item, c.matches, got, c.want)
		}
	}
}

func TestBuildVersion(t *testing.T) {
	matches := []string{"full", "1_2_3_4"}

	cases := []struct {
		trunc int
		want  string
	}{
		{VersionTruncationNone, "1.2.3.4"},
		{VersionTruncationMajor, "1"},
		{VersionTruncationMinor, "1.2"},
		{VersionTruncationPatch, "1.2.3"},
		{VersionTruncationBuild, "1.2.3.4"},
	}

	for _, c := range cases {
		if got := BuildVersion("$1", matches, c.trunc); got != c.want {
			t.Errorf("BuildVersion trunc=%d = %q, want %q", c.trunc, got, c.want)
		}
	}

	if got := BuildVersion("v$1.", []string{"full", "7_0"}, VersionTruncationNone); got != "v7.0" {
		t.Errorf("dot/space trim: got %q, want v7.0", got)
	}
}

func TestFuzzyCompare(t *testing.T) {
	if !FuzzyCompare("Mobile Safari", "mobilesafari") {
		t.Error("FuzzyCompare should ignore case and spaces")
	}

	if FuzzyCompare("Safari", "Mobile Safari") {
		t.Error("FuzzyCompare should not match different names")
	}
}

func TestOrderedMapPreservesOrder(t *testing.T) {
	doc := "Zeta:\n  v: 1\nAlpha:\n  v: 2\nMid:\n  v: 3\n"

	var m OrderedMap[map[string]int]
	if err := yaml.Unmarshal([]byte(doc), &m); err != nil {
		t.Fatal(err)
	}

	var keys []string
	for _, e := range m.Entries {
		keys = append(keys, e.Key)
	}

	if strings.Join(keys, ",") != "Zeta,Alpha,Mid" {
		t.Errorf("order not preserved: %v", keys)
	}
}

func TestOrderedMapRejectsSequence(t *testing.T) {
	var m OrderedMap[int]
	if err := yaml.Unmarshal([]byte("- 1\n- 2\n"), &m); err == nil {
		t.Error("expected error for sequence node")
	}
}

func TestHasDesktopFragment(t *testing.T) {
	cases := []struct {
		ua   string
		want bool
	}{
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Gecko/20100101 Firefox/121.0", true},
		{"Mozilla/5.0 (X11; Linux x86_64) Gecko/20100101 Firefox/121.0", true},
		{"Mozilla/5.0 (Windows NT 10.0) Trident/7.0; Touch", false},     // Trident excluded
		{"Mozilla/5.0 (Linux; Android 13; Pixel) Mobile Safari", false}, // no desktop fragment
	}

	for _, c := range cases {
		if got := HasDesktopFragment(c.ua); got != c.want {
			t.Errorf("HasDesktopFragment(%q) = %v, want %v", c.ua, got, c.want)
		}
	}
}
