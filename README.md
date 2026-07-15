# devicedetector

A native Go port of [matomo/device-detector](https://github.com/matomo-org/device-detector) — the Universal Device Detection library. Parses any User Agent string and detects the browser, operating system, device type (desktop, tablet, mobile, tv, cars, console, etc.), brand and model.

This is an **unofficial, AI-assisted port** of the original PHP library, translated directly from the PHP sources and validated against the upstream test fixtures. It is not affiliated with or endorsed by Matomo.

## Why another port?

- **Native Go, zero runtime config** — regex database is embedded via `go:embed`; `devicedetector.New()` just works. Loading from an external directory is also supported for out-of-band regex updates.
- **Thread-safe by design** — one `DeviceDetector` instance can be shared across goroutines; parsers carry no per-parse mutable state.
- **Validated against upstream fixtures** — the test suite runs the original matomo fixture corpus: **36,333 user agents, 100.00% identical output** (client-hints entries excluded until v0.2).

## Usage

```go
import dd "github.com/skalibog/devicedetector"

detector, err := dd.New() // embedded regex database
// detector, err := dd.NewFromDir("path/to/regexes") // external database

info := detector.Parse(userAgent)
if info.IsBot() {
    fmt.Println(info.Bot().Name)
} else {
    fmt.Println(info.Client().Name, info.Client().Version) // e.g. "Mobile Safari 17.4"
    fmt.Println(info.OS().Name, info.OS().Version)         // e.g. "iOS 17.4"
    fmt.Println(info.Device().Type, info.Device().Brand, info.Device().Model)
}
```

## Status / roadmap

- [x] Bot detection
- [x] Client parsers (browser + engine + version, feed readers, libraries, media players, mobile apps, PIM)
- [x] Device parsers (mobiles, TVs, consoles, cameras, car browsers, notebooks, portable media players)
- [x] OS parser, vendor fragments
- [x] Fixture-based test suite against the upstream corpus
- [ ] Client Hints support (planned for v0.2)
- [ ] Automated regex database sync from upstream
- [ ] Performance pass — ~17ms/parse single-threaded today (regexp2 backtracking over the full alternation, same design as upstream PHP); an RE2 prefilter fast-path is the likely first win. Callers doing high volume should cache results by UA hash.

## Data provenance

The regex database (`data/regexes/`) and test fixtures (`testdata/fixtures/`) are taken verbatim from [matomo/device-detector](https://github.com/matomo-org/device-detector) at commit `6f07f615199851548db47a900815d2ea2cdcde08` (post-6.4.6 master). Run `scripts/sync-upstream.sh` to re-sync. See `data/NOTICE.md`.

## License

LGPL-3.0-or-later, same as the original library — this port is a derivative work of matomo/device-detector.

- Original library: Copyright (C) Matomo Team ([matomo.org](https://matomo.org))
- Go port: Copyright (C) 2026 skalibog

Modifications relative to the original: complete translation from PHP to Go; API redesigned for Go idioms (immutable parsers, `fs.FS`-based data loading, embedded database). See git history for details.
