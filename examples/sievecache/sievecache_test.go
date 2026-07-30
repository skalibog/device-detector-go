package sievecache_test

import (
	"testing"

	dd "github.com/skalibog/device-detector-go"
	"github.com/skalibog/device-detector-go/examples/sievecache"
)

const ua = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1"

func TestSieveBackend(t *testing.T) {
	d, err := dd.New(dd.WithResultCacheBackend(sievecache.New(1024)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	first, err := d.Parse(ua)
	if err != nil {
		t.Fatal(err)
	}

	if got := first.Client().Name; got != "Mobile Safari" {
		t.Fatalf("Client.Name = %q, want %q", got, "Mobile Safari")
	}

	// Poison attempt: the detector's boundary clones must isolate the backend.
	first.Client().Name = "poisoned"

	second, err := d.Parse(ua) // cache hit
	if err != nil {
		t.Fatal(err)
	}

	if got := second.Client().Name; got != "Mobile Safari" {
		t.Fatalf("cache hit Client.Name = %q, want %q", got, "Mobile Safari")
	}
}
