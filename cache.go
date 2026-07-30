package devicedetector

import (
	"container/list"
	"strconv"
	"strings"
	"sync"
)

// ResultCache is a pluggable backend for the parse-result cache, letting
// callers bring their own eviction policy (SIEVE, TTL, ristretto, ...) via
// [WithResultCacheBackend]. Implementations must be safe for concurrent use.
//
// The detector owns copy semantics: it always Puts an isolated copy and clones
// what Get returns before handing it to the caller, so a backend simply stores
// the pointers it is given — it must not mutate them.
type ResultCache interface {
	// Get returns the cached Info for key, if present.
	Get(key string) (*Info, bool)
	// Put stores info under key, evicting per the backend's policy.
	Put(key string, info *Info)
}

// WithResultCache adds the built-in in-memory sharded LRU cache of parse
// results, keyed by the (truncated) user agent plus client hints. Real traffic
// repeats user agents heavily, so a modest cache (tens of thousands of
// entries) removes nearly all parse cost on hot paths; size is the maximum
// number of cached entries and n <= 0 leaves caching disabled (the default —
// the detector stays stateless).
//
// Only successful parses are cached; errors (e.g. a match timeout on
// adversarial input) are always recomputed. Returned Info values are
// independent copies — mutating one never affects later lookups.
//
// To use a different eviction policy, supply your own backend with
// [WithResultCacheBackend] instead.
func WithResultCache(size int) Option {
	return func(d *DeviceDetector) {
		if size > 0 {
			d.cache = newShardedLRU(size)
		}
	}
}

// WithResultCacheBackend is WithResultCache with a caller-supplied cache
// backend instead of the built-in LRU. Key format is opaque and may change
// between minor versions — treat keys as arbitrary strings. A nil backend is
// ignored (caching stays disabled).
func WithResultCacheBackend(c ResultCache) Option {
	return func(d *DeviceDetector) {
		if c != nil {
			d.cache = c
		}
	}
}

// resultCacheShards spreads lock contention across goroutines. Fixed power of
// two so the shard pick compiles to a mask.
const resultCacheShards = 16

// shardedLRU is the built-in ResultCache: a sharded LRU where each shard is an
// independent mutex-guarded map + recency list, so concurrent lookups for
// different user agents rarely contend on the same lock.
type shardedLRU struct {
	shards [resultCacheShards]cacheShard
}

type cacheShard struct {
	mu  sync.Mutex
	cap int
	ll  *list.List // front = most recently used
	m   map[string]*list.Element
}

type cacheEntry struct {
	key  string
	info *Info
}

func newShardedLRU(size int) *shardedLRU {
	perShard := (size + resultCacheShards - 1) / resultCacheShards

	c := &shardedLRU{}
	for i := range c.shards {
		c.shards[i] = cacheShard{cap: perShard, ll: list.New(), m: make(map[string]*list.Element)}
	}

	return c
}

// shardFor picks a shard by FNV-1a of the key.
func (c *shardedLRU) shardFor(key string) *cacheShard {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)

	h := uint64(offset64)
	for i := 0; i < len(key); i++ {
		h ^= uint64(key[i])
		h *= prime64
	}

	return &c.shards[h&(resultCacheShards-1)]
}

func (c *shardedLRU) Get(key string) (*Info, bool) {
	s := c.shardFor(key)

	s.mu.Lock()
	defer s.mu.Unlock()

	el, ok := s.m[key]
	if !ok {
		return nil, false
	}

	s.ll.MoveToFront(el)

	// No clone here: the detector clones at the cache boundary (see parse), the
	// same contract every ResultCache backend gets.
	return el.Value.(*cacheEntry).info, true
}

func (c *shardedLRU) Put(key string, info *Info) {
	s := c.shardFor(key)

	s.mu.Lock()
	defer s.mu.Unlock()

	if el, ok := s.m[key]; ok { // lost a race with another goroutine — refresh
		el.Value.(*cacheEntry).info = info
		s.ll.MoveToFront(el)

		return
	}

	s.m[key] = s.ll.PushFront(&cacheEntry{key: key, info: info})

	if s.ll.Len() > s.cap {
		oldest := s.ll.Back()
		s.ll.Remove(oldest)
		delete(s.m, oldest.Value.(*cacheEntry).key)
	}
}

// clone returns an independent copy of the Info. Bot, OS and Client are flat
// value structs, so copying the pointed-to values is a full deep copy.
func (i *Info) clone() *Info {
	c := *i

	if i.bot != nil {
		b := *i.bot
		c.bot = &b
	}

	if i.os != nil {
		o := *i.os
		c.os = &o
	}

	if i.client != nil {
		cl := *i.client
		c.client = &cl
	}

	return &c
}

// cacheKey builds a deterministic key from the truncated user agent and the
// client hints. A nil hints pointer is distinguished from an empty struct
// because parse() treats them differently for letterless user agents.
func cacheKey(ua string, hints *ClientHints) string {
	if hints == nil {
		return ua
	}

	var b strings.Builder

	b.Grow(len(ua) + 64)
	b.WriteString(ua)
	b.WriteByte(0)
	b.WriteString(hints.Architecture)
	b.WriteByte(0)
	b.WriteString(hints.Bitness)
	b.WriteByte(0)
	b.WriteString(strconv.FormatBool(hints.Mobile))
	b.WriteByte(0)
	b.WriteString(hints.Model)
	b.WriteByte(0)
	b.WriteString(hints.Platform)
	b.WriteByte(0)
	b.WriteString(hints.PlatformVersion)
	b.WriteByte(0)
	b.WriteString(hints.UAFullVersion)
	b.WriteByte(0)
	b.WriteString(hints.App)

	for _, bv := range hints.FullVersionList {
		b.WriteByte(1)
		b.WriteString(bv.Brand)
		b.WriteByte(2)
		b.WriteString(bv.Version)
	}

	for _, ff := range hints.FormFactors {
		b.WriteByte(3)
		b.WriteString(ff)
	}

	return b.String()
}
