package devicedetector

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
)

const cacheTestUA = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1"

// renderInfo flattens an Info to a comparable string, dereferencing the
// pointer fields (a raw %+v would compare pointer addresses).
func renderInfo(i *Info) string {
	var bot, osv, cl string

	if i.Bot() != nil {
		bot = fmt.Sprintf("%+v", *i.Bot())
	}

	if i.OS() != nil {
		osv = fmt.Sprintf("%+v", *i.OS())
	}

	if i.Client() != nil {
		cl = fmt.Sprintf("%+v", *i.Client())
	}

	return fmt.Sprintf("ua=%q bot=%q os=%q client=%q type=%d brand=%q model=%q mobile=%v",
		i.UserAgent, bot, osv, cl, i.DeviceType(), i.Brand(), i.Model(), i.IsMobile())
}

func newCachedDetector(t testing.TB, size int) *DeviceDetector {
	t.Helper()

	d, err := New(WithResultCache(size))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return d
}

func TestResultCacheHitMatchesUncached(t *testing.T) {
	cached := newCachedDetector(t, 128)

	plain, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	uas := []string{
		cacheTestUA,
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"", // letterless short-circuit path
	}

	for _, ua := range uas {
		want, err := plain.Parse(ua)
		if err != nil {
			t.Fatalf("plain Parse(%q): %v", ua, err)
		}

		// Twice: first populates, second must hit the cache.
		for pass := 1; pass <= 2; pass++ {
			got, err := cached.Parse(ua)
			if err != nil {
				t.Fatalf("cached Parse(%q) pass %d: %v", ua, pass, err)
			}

			if renderInfo(got) != renderInfo(want) {
				t.Errorf("Parse(%q) pass %d:\n got %s\nwant %s", ua, pass, renderInfo(got), renderInfo(want))
			}
		}
	}
}

func TestResultCacheHintsKeyedSeparately(t *testing.T) {
	d := newCachedDetector(t, 128)

	noHints, err := d.Parse(cacheTestUA)
	if err != nil {
		t.Fatal(err)
	}

	h := http.Header{}
	h.Set("Sec-CH-UA-Mobile", "?1")
	h.Set("Sec-CH-UA-Platform", `"Android"`)
	hints := NewClientHintsFromHeaders(h)

	withHints, err := d.ParseWithHints(cacheTestUA, hints)
	if err != nil {
		t.Fatal(err)
	}

	// Same UA, different hints — must not collide in the cache.
	if noHints.OS().Name == withHints.OS().Name {
		t.Errorf("hints variant not keyed separately: both OS %q", noHints.OS().Name)
	}

	again, err := d.Parse(cacheTestUA)
	if err != nil {
		t.Fatal(err)
	}

	if again.OS().Name != noHints.OS().Name {
		t.Errorf("hintless entry clobbered: got OS %q, want %q", again.OS().Name, noHints.OS().Name)
	}
}

func TestResultCacheReturnsIsolatedCopies(t *testing.T) {
	d := newCachedDetector(t, 128)

	first, err := d.Parse(cacheTestUA)
	if err != nil {
		t.Fatal(err)
	}

	// Mutate everything reachable through accessors.
	first.UserAgent = "poisoned"
	first.Client().Name = "poisoned"
	first.OS().Name = "poisoned"

	second, err := d.Parse(cacheTestUA)
	if err != nil {
		t.Fatal(err)
	}

	if second.UserAgent != cacheTestUA || second.Client().Name == "poisoned" || second.OS().Name == "poisoned" {
		t.Errorf("cache poisoned by caller mutation: %+v", second)
	}

	// And mutating the second result must not affect a third.
	second.OS().Name = "poisoned-again"

	third, err := d.Parse(cacheTestUA)
	if err != nil {
		t.Fatal(err)
	}

	if third.OS().Name == "poisoned-again" {
		t.Error("cached hit shares state between callers")
	}
}

func TestResultCacheEviction(t *testing.T) {
	// Tiny cache: per-shard capacity is 1, so inserting many keys must evict
	// old entries instead of growing without bound.
	c := newResultCache(1)

	for i := 0; i < 1000; i++ {
		c.put(fmt.Sprintf("key-%d", i), &Info{UserAgent: "x"})
	}

	total := 0
	for i := range c.shards {
		c.shards[i].mu.Lock()
		if got, want := c.shards[i].ll.Len(), c.shards[i].cap; got > want {
			t.Errorf("shard %d holds %d entries, cap %d", i, got, want)
		}

		if got, want := len(c.shards[i].m), c.shards[i].ll.Len(); got != want {
			t.Errorf("shard %d map/list out of sync: %d vs %d", i, got, want)
		}
		total += c.shards[i].ll.Len()
		c.shards[i].mu.Unlock()
	}

	if total > resultCacheShards {
		t.Errorf("cache holds %d entries, want <= %d", total, resultCacheShards)
	}
}

func TestResultCacheLRUOrder(t *testing.T) {
	c := newResultCache(resultCacheShards) // capacity 1 per shard

	// Two keys landing in the same shard: after touching the first, inserting
	// a third same-shard key must evict the untouched one. Find same-shard keys.
	shard := c.shardFor("a")

	keys := []string{"a"}
	for i := 0; len(keys) < 3; i++ {
		k := fmt.Sprintf("k%d", i)
		if c.shardFor(k) == shard {
			keys = append(keys, k)
		}
	}

	c.put(keys[0], &Info{UserAgent: keys[0]})
	c.put(keys[1], &Info{UserAgent: keys[1]}) // capacity 1: evicts keys[0]

	if _, ok := c.get(keys[0]); ok {
		t.Fatal("expected keys[0] evicted at capacity 1")
	}

	if got, ok := c.get(keys[1]); !ok || got.UserAgent != keys[1] {
		t.Fatalf("expected keys[1] cached, got %v ok=%v", got, ok)
	}
}

func TestResultCacheConcurrency(t *testing.T) {
	d := newCachedDetector(t, 64)

	uas := make([]string, 32)
	for i := range uas {
		uas[i] = fmt.Sprintf("Mozilla/5.0 (Linux; Android 1%d) Chrome/12%d.0.0.0 Mobile Safari/537.36", i, i)
	}

	var wg sync.WaitGroup

	for g := 0; g < 8; g++ {
		wg.Add(1)

		go func(seed int) {
			defer wg.Done()

			for i := 0; i < 50; i++ {
				ua := uas[(seed+i)%len(uas)]

				info, err := d.Parse(ua)
				if err != nil {
					t.Errorf("Parse(%q): %v", ua, err)

					return
				}

				if info.UserAgent != ua {
					t.Errorf("got UserAgent %q, want %q", info.UserAgent, ua)

					return
				}
			}
		}(g)
	}

	wg.Wait()
}

func BenchmarkParseCached(b *testing.B) {
	d := newCachedDetector(b, 1024)

	if _, err := d.Parse(cacheTestUA); err != nil { // warm
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := d.Parse(cacheTestUA); err != nil {
			b.Fatal(err)
		}
	}
}
