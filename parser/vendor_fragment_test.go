package parser

import (
	"os"
	"testing"
)

// TestVendorFragmentParse checks vendor fragment tokens map to the right brand.
// The fragments come from a device UA's browser-mode marker; the parser applies
// a trailing "[^a-z0-9]+" guard, so each token is followed by a delimiter.
func TestVendorFragmentParse(t *testing.T) {
	v, err := NewVendorFragment(os.DirFS("../data/regexes"))
	if err != nil {
		t.Fatalf("NewVendorFragment: %v", err)
	}

	tests := []struct {
		name      string
		ua        string
		wantBrand string
		wantRegex string
	}{
		{"dell mddr", "Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; Trident/5.0; MDDRJS)", "Dell", "MDDR(JS)?"},
		{"dell mdds", "Mozilla/4.0 (compatible; MSIE 8.0; Windows NT 6.1; MDDSJS; .NET CLR 2.0)", "Dell", "MDDS(JS)?"},
		{"acer maar", "Mozilla/5.0 (compatible; MSIE 10.0; Windows NT 6.2; MAAR; rv:11.0)", "Acer", "MAAR(JS)?"},
		{"samsung masm", "Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; MASMJS; Trident/5.0)", "Samsung", "MASM(JS)?"},
		{"asus maau", "Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; MAAU; Trident/5.0)", "Asus", "MAAU"},
		{"asus np0 range", "Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; NP06; Trident/5.0)", "Asus", "NP0[26789]"},
		{"hp fragment", "Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; HPNTDFJS; Trident/5.0)", "HP", "HPNTDF(JS)?"},
		{"no fragment", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0 Safari/537.36", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brand, matched, err := v.Parse(tt.ua)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			if brand != tt.wantBrand {
				t.Errorf("Parse(%q) brand = %q, want %q", tt.ua, brand, tt.wantBrand)
			}

			if matched != tt.wantRegex {
				t.Errorf("Parse(%q) matched regex = %q, want %q", tt.ua, matched, tt.wantRegex)
			}
		})
	}
}
