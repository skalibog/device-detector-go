package devicedetector

import (
	"testing"

	"github.com/skalibog/device-detector-go/internal/parser"
)

func TestGateCoverage(t *testing.T) {
	if _, err := New(); err != nil { // eager compile fills the cache
		t.Fatal(err)
	}

	gated, total := parser.GateStats()
	t.Logf("RE2-gated patterns: %d / %d (%.2f%%)", gated, total, 100*float64(gated)/float64(total))

	if gated == 0 {
		t.Fatal("no patterns gated — prefilter inert")
	}
}
