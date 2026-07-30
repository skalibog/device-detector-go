# devicedetector

[![CI](https://github.com/skalibog/device-detector-go/actions/workflows/ci.yml/badge.svg)](https://github.com/skalibog/device-detector-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/skalibog/device-detector-go.svg)](https://pkg.go.dev/github.com/skalibog/device-detector-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/skalibog/device-detector-go)](https://goreportcard.com/report/github.com/skalibog/device-detector-go)
[![Upstream fixtures](https://img.shields.io/badge/upstream_fixtures-37%2C640_%2F_100%25-brightgreen)](#validation)
[![codecov](https://codecov.io/gh/skalibog/device-detector-go/graph/badge.svg)](https://codecov.io/gh/skalibog/device-detector-go)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/skalibog/device-detector-go/badge)](https://scorecard.dev/viewer/?uri=github.com/skalibog/device-detector-go)
[![Go version](https://img.shields.io/github/go-mod/go-version/skalibog/device-detector-go)](go.mod)
[![License: LGPL-3.0-or-later](https://img.shields.io/badge/license-LGPL--3.0--or--later-blue)](LICENSE)

A native Go port of [matomo/device-detector](https://github.com/matomo-org/device-detector) — the Universal Device Detection library. Parses any User Agent string and detects the browser, operating system, device type (desktop, tablet, mobile, tv, cars, console, etc.), brand and model.

This is an **unofficial, AI-assisted port** translated directly from the PHP sources and validated against the complete upstream test corpus. It is not affiliated with or endorsed by Matomo.

## Highlights

- **Zero-config** — the regex database ships inside the binary via `go:embed`; `devicedetector.New()` just works. Loading from an external directory is also supported for out-of-band database updates.
- **Byte-faithful to upstream** — all 37,640 fixture entries (with client hints) from matomo/device-detector reproduce identically, enforced in CI with a zero-mismatch gate.
- **Thread-safe by design** — one `DeviceDetector` instance is shared across goroutines; parsers are immutable after construction. Verified with the race detector and a concurrent-determinism test.
- **Complete detection surface** — 803 bots, 716 browsers (with engine and engine version), 203 operating systems, 2,106 device brands across 14 device types.

## Install

```bash
go get github.com/skalibog/device-detector-go
```

## Quick start

```go
package main

import (
	"fmt"

	dd "github.com/skalibog/device-detector-go"
)

func main() {
	detector, err := dd.New() // embedded regex database
	if err != nil {
		panic(err)
	}

	info, err := detector.Parse("Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1")
	if err != nil {
		panic(err)
	}

	if info.IsBot() {
		fmt.Println("bot:", info.Bot().Name)
		return
	}

	fmt.Println(info.Client().Name, info.Client().Version) // Mobile Safari 17.4
	fmt.Println(info.OS().Name, info.OS().Version)         // iOS 17.4
	fmt.Println(info.DeviceName(), info.Brand())           // smartphone Apple
	fmt.Println(info.IsMobile(), info.IsDesktop())         // true false
}
```

Options:

```go
dd.New(dd.WithVersionTruncation(dd.VersionTruncationNone)) // full versions (default: minor)
dd.New(dd.WithSkipBotDetection())                          // skip the bot stage
dd.New(dd.WithMaxUARawLength(512))                         // tighter length cap (see SECURITY.md)
dd.New(dd.WithLazyCompile())                               // defer compilation; faster New, no fail-fast
dd.New(dd.WithResultCache(65536))                          // LRU result cache; big win on repeat-heavy traffic
dd.NewFromDir("path/to/regexes")                           // external regex database
```

## Client Hints

Refine detection with HTTP Client Hints (`Sec-CH-UA-*`, `X-Requested-With`):

```go
hints := dd.NewClientHintsFromHeaders(request.Header)      // from an *http.Request
info, err := detector.ParseWithHints(request.UserAgent(), hints)
```

`NewClientHintsFromMap` accepts the structured `navigator.userAgentData` values (brand list, form factors, mobile flag). `Parse(ua)` is `ParseWithHints(ua, nil)`.

## Result surface

| Accessor | Returns |
|---|---|
| `info.IsBot()` / `info.Bot()` | bot name, category, URL, producer |
| `info.Client()` | type (browser / mobile app / mediaplayer / feed reader / library / pim), name, version; engine + engine version for browsers |
| `info.OS()` | name, short name, version, platform, family |
| `info.DeviceType()` / `info.DeviceName()` | device type — typed `DeviceType` (matomo ids) / its name (smartphone, tablet, tv, …) |
| `info.Brand()` / `info.Model()` | device brand and model |
| `info.IsMobile()` / `info.IsDesktop()` / `info.IsTouchEnabled()` | convenience checks mirroring upstream |

## Validation

The test suite replays the upstream fixture corpus — **37,640 user agents (user-agent and client-hints), 100.00% identical output** — and fails on a single mismatch. Statement coverage across all packages is ~83%, dominated by the corpus replay.

## Performance

The port keeps upstream's design: a big regex alternation walked with a backtracking engine ([dlclark/regexp2](https://github.com/dlclark/regexp2)), because the database uses PCRE features Go's RE2 `regexp` cannot express.

Indicatively (Ryzen 9950X): ~1 ms/parse across goroutines, from ~1–2 ms for early-exit UAs (bots, desktops) up to tens of ms for long-tail mobile UAs; warm heap ~50 MB. Reproduce and see methodology in [docs/BENCHMARKS.md](docs/BENCHMARKS.md).

Because the engine backtracks, oversized crafted user agents can be expensive. Two guards are on by default (see [SECURITY.md](SECURITY.md)): a 2048-byte length cap (`WithMaxUARawLength`) and a 1 s per-match timeout (`WithMatchTimeout`), which together bound a ~24 KB junk input from ~60 s to ~1 s without affecting genuine traffic.

Recommendations for high-volume callers:

- **Enable the result cache** — `WithResultCache(n)` adds a built-in sharded LRU keyed by UA + client hints; real traffic repeats UAs heavily, and a cache hit costs ~200 ns instead of ~1 ms+. Cached results are isolated copies, and only successful parses are stored.
- **For untrusted input**, tighten the guards (e.g. `WithMaxUARawLength(512)`, `WithMatchTimeout(100*time.Millisecond)`).
- A performance pass (RE2 prefilter fast-path for the common alternations) is on the roadmap.

## Comparison

Where this port sits among Go User-Agent libraries. It trades a bigger embedded
database and a backtracking engine for breadth and byte-for-byte parity with
matomo; the lightweight parsers trade coverage for a tiny footprint and raw
speed. Functional summary from each project's public documentation:

| | device-detector-go | [mssola/user_agent](https://github.com/mssola/user_agent) | [mileusna/useragent](https://github.com/mileusna/useragent) | [ua-parser/uap-go](https://github.com/ua-parser/uap-go) |
|---|---|---|---|---|
| Data source | matomo database (embedded) | built-in heuristics | built-in heuristics | uap-core regexes |
| Device brand + model | ✅ 2,000+ brands | — | — | partial (uap-core) |
| Device type | ✅ 14 types | — | basic | ✅ |
| Bots | ✅ 800+ | basic | basic | limited |
| Browser engine + version | ✅ | engine only | — | — |
| OS family | ✅ | — | — | ✅ |
| Client Hints | ✅ | — | — | — |
| Upstream parity gate | ✅ full corpus | n/a | n/a | n/a |
| Dependencies | regexp2 | none | none | yaml |
| Engine | PCRE (backtracking) | native | native | RE2 |

Migrating from any of these (or from the PHP original) is covered in
[docs/MIGRATION.md](docs/MIGRATION.md).

## Data provenance and updates

The regex database (`data/regexes/`) and test fixtures (`testdata/fixtures/`) are taken verbatim from [matomo/device-detector](https://github.com/matomo-org/device-detector) release [**6.5.1**](https://github.com/matomo-org/device-detector/releases/tag/6.5.1). See [data/NOTICE.md](data/NOTICE.md).

A scheduled workflow re-syncs the database from the latest upstream release tag fortnightly and opens a PR; the fixture gate then proves the Go code still reproduces upstream output on the new corpus. Manual sync: `make sync-upstream` or `scripts/sync-upstream.sh <ref>`.

## Development

```bash
make test            # unit tests + short corpus
make test-fixtures   # full 37k fixture corpus (~1 min)
make vet             # go vet
make sync-upstream   # pull regex DB + fixtures from upstream
```

## Versioning

[SemVer](https://semver.org) via git tags; see [CHANGELOG.md](CHANGELOG.md). From v1.0.0 the API is stable within the 1.x line — minor releases add features compatibly, patch releases fix bugs or refresh the database, and an incompatible change would require a new major (`/vN`). This is enforced in CI by a `gorelease` gate. Each release notes the pinned upstream database commit. Contributions welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).

## Roadmap

- [x] Client Hints support — `ParseWithHints`
- [x] 1.0 — frozen API (`apidiff` hard gate), OpenSSF Scorecard, migration guide
- [x] Opt-in result cache — `WithResultCache` (v1.1)
- [ ] Performance pass — RE2 prefilter fast-path (v1.2)

## License

LGPL-3.0-or-later, same as the original library — this port is a derivative work of matomo/device-detector. What that means for a statically-linked Go binary (short answer: it's fine for server-side use and for shipping a binary while publishing the pinned library version) is covered in the [FAQ](docs/FAQ.md).

- Original library and regex database: Copyright (C) [Matomo Team](https://matomo.org)
- Go port: Copyright (C) 2026 skalibog

Modifications relative to the original: complete translation from PHP to Go; API redesigned for Go idioms (immutable parsers, `fs.FS`-based data loading, embedded database). See git history for details.
