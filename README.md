# devicedetector

[![CI](https://github.com/skalibog/device-detector-go/actions/workflows/ci.yml/badge.svg)](https://github.com/skalibog/device-detector-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/skalibog/device-detector-go.svg)](https://pkg.go.dev/github.com/skalibog/device-detector-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/skalibog/device-detector-go)](https://goreportcard.com/report/github.com/skalibog/device-detector-go)
[![Upstream fixtures](https://img.shields.io/badge/upstream_fixtures-36%2C333_%2F_100%25-brightgreen)](#validation)
[![Coverage](https://img.shields.io/badge/coverage-83%25-green)](#validation)
[![Go version](https://img.shields.io/github/go-mod/go-version/skalibog/device-detector-go)](go.mod)
[![License: LGPL-3.0-or-later](https://img.shields.io/badge/license-LGPL--3.0--or--later-blue)](LICENSE)

A native Go port of [matomo/device-detector](https://github.com/matomo-org/device-detector) — the Universal Device Detection library. Parses any User Agent string and detects the browser, operating system, device type (desktop, tablet, mobile, tv, cars, console, etc.), brand and model.

This is an **unofficial, AI-assisted port** translated directly from the PHP sources and validated against the complete upstream test corpus. It is not affiliated with or endorsed by Matomo.

## Highlights

- **Zero-config** — the regex database ships inside the binary via `go:embed`; `devicedetector.New()` just works. Loading from an external directory is also supported for out-of-band database updates.
- **Byte-faithful to upstream** — all 36,333 UA-string fixture entries from matomo/device-detector reproduce identically, enforced in CI with a zero-mismatch gate.
- **Thread-safe by design** — one `DeviceDetector` instance is shared across goroutines; parsers are immutable after construction. Verified with the race detector and a concurrent-determinism test.
- **Complete detection surface** — 1,083 bots, 679 browsers (with engine and engine version), 186 operating systems, 2,084 device brands across 14 device types.

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
dd.New(dd.WithVersionTruncation(parser.VersionTruncationNone)) // full versions (default: minor)
dd.New(dd.WithSkipBotDetection())                              // skip the bot stage
dd.NewFromDir("path/to/regexes")                               // external regex database
```

## Result surface

| Accessor | Returns |
|---|---|
| `info.IsBot()` / `info.Bot()` | bot name, category, URL, producer |
| `info.Client()` | type (browser / mobile app / mediaplayer / feed reader / library / pim), name, version; engine + engine version for browsers |
| `info.OS()` | name, short name, version, platform, family |
| `info.Device()` / `info.DeviceName()` | device type (smartphone, tablet, tv, console, wearable, …) |
| `info.Brand()` / `info.Model()` | device brand and model |
| `info.IsMobile()` / `info.IsDesktop()` / `info.IsTouchEnabled()` | convenience checks mirroring upstream |

## Validation

The test suite replays the upstream fixture corpus — **36,333 user agents, 100.00% identical output** — and fails on a single mismatch. Client-hints fixture entries are excluded until v0.2. Statement coverage across all packages is ~83%, dominated by the corpus replay.

## Performance

The port keeps upstream's design: a big regex alternation walked with a backtracking engine ([dlclark/regexp2](https://github.com/dlclark/regexp2)), because the database uses PCRE features Go's RE2 `regexp` cannot express.

Measured on the full diverse corpus (32 workers, warm caches): ~640 parses/sec sustained, from ~1.5 ms for early-exit UAs (bots, desktops) up to tens of ms for long-tail mobile UAs. Detector heap footprint is ~50 MB warm, ~110 MB with every model regex lazily compiled (bounded by database size — it is a cache, not a leak).

Recommendations for high-volume callers:

- **Cache results by UA hash** — real traffic repeats UAs heavily; a small LRU in front removes nearly all parse cost.
- A performance pass (RE2 prefilter fast-path for the common alternations) is on the roadmap.

## Data provenance and updates

The regex database (`data/regexes/`) and test fixtures (`testdata/fixtures/`) are taken verbatim from [matomo/device-detector](https://github.com/matomo-org/device-detector) at commit [`6f07f615`](https://github.com/matomo-org/device-detector/commit/6f07f615199851548db47a900815d2ea2cdcde08) (post-6.4.6 master). See [data/NOTICE.md](data/NOTICE.md).

A scheduled workflow re-syncs the database from upstream monthly and opens a PR; the fixture gate then proves the Go code still reproduces upstream output on the new corpus. Manual sync: `make sync-upstream` or `scripts/sync-upstream.sh <ref>`.

## Development

```bash
make test            # unit tests + short corpus
make test-fixtures   # full 36k fixture corpus (~1 min)
make vet             # go vet
make sync-upstream   # pull regex DB + fixtures from upstream
```

## Versioning

[SemVer](https://semver.org) via git tags; see [CHANGELOG.md](CHANGELOG.md). Until v1.0.0 minor releases may change the API; patch releases are safe (including database-only refreshes). Each release notes the pinned upstream database commit. Contributions welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).

## Roadmap

- [ ] Client Hints support (v0.2) — all skipped branches are marked `TODO(client-hints)` in the source
- [ ] Performance pass — RE2 prefilter fast-path
- [ ] Browser family / OS family surfacing parity review

## License

LGPL-3.0-or-later, same as the original library — this port is a derivative work of matomo/device-detector.

- Original library and regex database: Copyright (C) [Matomo Team](https://matomo.org)
- Go port: Copyright (C) 2026 skalibog

Modifications relative to the original: complete translation from PHP to Go; API redesigned for Go idioms (immutable parsers, `fs.FS`-based data loading, embedded database). See git history for details.
