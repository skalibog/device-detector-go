# Benchmarks

Numbers below are indicative, not a guarantee — they depend heavily on the CPU
and the user-agent mix. Reproduce them yourself:

```bash
go test -bench=BenchmarkParse -benchtime=2s -count=6 -run='^$' .
```

For statistically meaningful comparisons across a change, pipe `-count` runs
through [`benchstat`](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat).

## Reference environment

- AMD Ryzen 9 9950X, Go 1.24, `dlclark/regexp2` v1.12, database 6.5.1.

## Results

| Benchmark | Result | Notes |
|---|---|---|
| `New()` | ~200 ms | one-time; compiles the whole database (fail-fast). Use `WithLazyCompile` for ~55 ms |
| `BenchmarkParse` | ~14 ms/op | single-goroutine, over a fixed 6-UA mix |
| `BenchmarkParseParallel` | ~1.1 ms/op | 32 goroutines sharing one detector |

Per-parse cost varies widely with the input: bots and desktop UAs exit early
(~1–2 ms), while long-tail mobile UAs walk deeper alternations (tens of ms).
The engine backtracks (see [why regexp2](FAQ.md)); an RE2 linear-time prefilter
for the common patterns is on the roadmap and should collapse the tail.

## Memory

The detector holds the compiled database. Warm resident footprint is on the
order of ~50 MB, rising toward ~150 MB once every lazily-compiled model regex is
touched — bounded by the database size (a cache, not a leak). All detectors over
the same embedded database share one process-wide compile cache.

## Throughput advice

Real traffic repeats user agents heavily; a small LRU keyed by the raw UA in
front of `Parse` removes almost all cost. See the README performance section.
