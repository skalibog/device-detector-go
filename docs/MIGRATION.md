# Migration guide

How to move to `device-detector-go` from the PHP original or from a lightweight
Go User-Agent parser. This port reproduces upstream output byte-for-byte on the
full fixture corpus, so detection results should not surprise you — the changes
are in the *shape* of the API, not in what it detects.

## From PHP `matomo/device-detector`

The Go API maps to the PHP one accessor-for-accessor; the main structural change
is that a detector is built once and reused, not constructed per request.

### Lifecycle

```php
// PHP: construct per user agent, then parse().
$dd = new DeviceDetector($userAgent);
$dd->parse();
```

```go
// Go: build ONE detector at startup, share it across goroutines, Parse per UA.
detector, err := dd.New()          // compiles the regex database once
info, err := detector.Parse(userAgent)
```

`DeviceDetector` is immutable after `New()` and safe for concurrent use — do not
allocate one per request. `Parse` returns a fresh, self-contained `Info` value.

### Accessor mapping

> **Nil check.** `info.Client()`, `info.OS()` and `info.Bot()` return `nil` when
> that facet was not detected (PHP returns an empty array instead). Guard before
> dereferencing — `if c := info.Client(); c != nil { … c.Name … }` — or use the
> boolean helpers (`info.IsBot()`, `info.IsMobile()`) which are nil-safe.

| PHP | Go |
|---|---|
| `$dd->isBot()` / `$dd->getBot()` | `info.IsBot()` / `info.Bot()` |
| `$dd->getClient('name')` / `'version'` / `'type'` | `info.Client().Name` / `.Version` / `.Type` |
| `$dd->getClient('engine')` / `'engine_version'` | `info.Client().Engine` / `.EngineVersion` |
| `$dd->getOs('name')` / `'version'` / `'platform'` | `info.OS().Name` / `.Version` / `.Platform` |
| `$dd->getDeviceName()` | `info.DeviceName()` |
| `$dd->getBrandName()` | `info.Brand()` |
| `$dd->getModel()` | `info.Model()` |
| `$dd->isMobile()` / `isDesktop()` / `isTouchEnabled()` | `info.IsMobile()` / `IsDesktop()` / `IsTouchEnabled()` |

### Options

| PHP | Go |
|---|---|
| `VERSION_TRUNCATION_MINOR` (default) | `WithVersionTruncation(dd.VersionTruncationMinor)` (default) |
| `VERSION_TRUNCATION_NONE` | `WithVersionTruncation(dd.VersionTruncationNone)` |
| `skipBotDetection()` | `WithSkipBotDetection()` |
| external regex directory | `NewFromDir(path)` / `NewFromFS(fsys)` |

### Client Hints

```php
$ch = ClientHints::factory($_SERVER);
$dd->setClientHints($ch);
$dd->parse();
```

```go
ch := dd.NewClientHintsFromHeaders(request.Header) // from an *http.Request
info, err := detector.ParseWithHints(request.UserAgent(), ch)
```

`NewClientHintsFromMap` accepts the structured `navigator.userAgentData` values
(brand list, form factors, mobile flag) when you collect hints client-side.

### Behavioural notes

- **No per-result cache by default.** PHP can wrap parsed regexes in a PSR cache;
  here the database is compiled once at `New()`. There is no built-in per-UA
  result cache — real traffic repeats UAs heavily, so put a small LRU keyed by UA
  hash in front if throughput matters. (An opt-in `WithResultCache` is on the
  roadmap.)
- **Errors instead of empty results.** `Parse` returns an `error` for guard
  conditions (e.g. an over-length UA hitting `WithMaxUARawLength`); check it.
- **Guards are on by default.** A 2048-byte length cap and a 1 s per-match
  timeout bound worst-case backtracking cost. See [SECURITY.md](../SECURITY.md).

## From a lightweight Go UA parser

Libraries such as `mssola/user_agent`, `mileusna/useragent`, or `ua-parser/uap-go`
favour a tiny dependency and raw speed; this port favours breadth and staying
byte-identical to matomo's continuously-updated database (2,000+ device brands,
800+ bots, engine + engine version, OS family, client-hints reconciliation). See
the [comparison table in the README](../README.md#comparison).

Two habits to change when you switch:

- **Reuse the detector.** Those libraries often expose a package-level `Parse`
  function; here you build a `*DeviceDetector` once (it holds the compiled
  database) and call `Parse` on it.
- **Read from `Info`, not loose fields.** Instead of a flat struct of strings you
  get an `Info` with typed accessors (`Client()`, `OS()`, `DeviceType()` as a
  typed `DeviceType`, `Brand()`, `Model()`), plus bot detection in the same call.

Detection differences are expected: this port will classify more devices, bots,
and browsers because it carries the full upstream database rather than a curated
subset. Validate against your own traffic sample before switching thresholds.
