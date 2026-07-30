// Package sievecache shows how to plug a third-party cache into
// devicedetector via WithResultCacheBackend, using the SIEVE eviction policy
// (https://github.com/guerinoni/sieve).
//
// SIEVE promotes entries lazily (a visited bit instead of a list move), which
// makes it more scan-resistant than LRU: a flood of one-hit-wonder user agents
// (randomising bots) does not evict the hot entries that serve most traffic.
// That is exactly the churn profile of high-RPS ad/RTB traffic, where this
// backend is a better fit than the built-in LRU.
package sievecache

import (
	"github.com/guerinoni/sieve"

	dd "github.com/skalibog/device-detector-go"
)

// Cache adapts guerinoni/sieve to devicedetector's ResultCache interface.
// The detector clones results at the cache boundary, so the adapter stores the
// pointers it is given as-is.
type Cache struct {
	s *sieve.Cache[string, *dd.Info]
}

// New returns a SIEVE-backed result cache holding up to size entries.
func New(size int32) *Cache {
	return &Cache{s: sieve.New[string, *dd.Info](size)}
}

// Get implements dd.ResultCache.
func (c *Cache) Get(key string) (*dd.Info, bool) { return c.s.Get(key) }

// Put implements dd.ResultCache.
func (c *Cache) Put(key string, info *dd.Info) { c.s.Set(key, info) }

var _ dd.ResultCache = (*Cache)(nil)
