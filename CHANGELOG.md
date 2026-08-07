# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
From 1.0.0 the public API is stable: minor releases add features compatibly, patch
releases fix bugs, and any incompatible change would require a new major (`/vN`).

Each release also notes the pinned matomo/device-detector database commit it ships.

## [Unreleased]

### Added — RE2/literal prefilter (the v1.2 performance pass)

- Every database pattern now carries a companion stdlib-`regexp` (RE2) gate
  that is a strict **superset** of the regexp2 pattern's match semantics: a
  gate miss proves a non-match and skips the backtracking engine; on a hit
  regexp2 remains the single source of truth for results and captures, so
  parity is preserved by construction (and enforced by the 37,640-entry
  corpus, which passes unchanged). 99%+ of patterns are gated; lookaround,
  backreferences and `\b` fall back to the full regexp2 path. Translation
  widens `\d`/`\w`/`\s` to their .NET Unicode semantics (context-aware inside
  character classes) and rewrites .NET named-group syntax.
- The per-entry walks with no upstream `preMatchOverall` (device brands, OS
  rules, browsers) are additionally narrowed by a **required-literal index**:
  literals extracted from each regex AST such that a match implies one is
  present in the UA; a lowercased substring probe then shrinks the walk to
  plausible entries only. (A combined RE2 union was tried and rejected — Go's
  NFA simulation over a ~20k-state union is slower than memchr-accelerated
  probes.)
- Numbers (Ryzen 9950X): worst long-tail mobile UA 14.8 ms → **2.7 ms**;
  2 KB adversarial junk 450 ms → **~60 ms**; full corpus replay 67 s →
  **15 s**. The fuzz input cap (512 B) and the relaxed aggregate-walk bounds
  from the nightly-fuzz stabilisation are removed/tightened accordingly.

### Fixed

- Nightly fuzzing stabilised after three failure classes were diagnosed:
  fuzzed inputs are now capped at 512 bytes because the go fuzzing engine
  kills a worker spending ~10 s on one input — which near-cap aggregate-walk
  junk does on slow runners — before any test assertion can fire; the
  full-length worst case moved to an explicit `TestAggregateWalkSeed`
  regression test. The placeholder-leak assertion is now per-token: a `$N` in
  a result only counts as a leak when that exact token is absent from the
  input, because a UA carrying a literal `$N` flows it into results through
  capture groups (e.g. `HUAWEI$0 Build` → model `$0`) — upstream parity
  (`buildByMatch` is a literal `str_replace`), not a template leak.

## [1.1.0] - 2026-07-30

### Added

- `WithResultCache(size)` — opt-in sharded LRU cache of parse results, keyed by
  the (truncated) user agent plus client hints. A cache hit costs ~200 ns and
  3 allocations versus ~1 ms+ for a full parse, which removes nearly all parse
  cost on repeat-heavy traffic. Only successful parses are cached; returned
  `Info` values are independent copies, so mutating a result can never poison
  later lookups. Implemented on the standard library (no new dependencies);
  disabled by default — the detector stays stateless unless opted in.
- `ResultCache` interface + `WithResultCacheBackend(c)` — plug in your own
  eviction policy (SIEVE, TTL, ristretto, ...). The detector clones at the
  cache boundary for every backend, so isolation guarantees hold regardless of
  implementation. A compiled SIEVE adapter lives in `examples/sievecache`
  (separate Go module — its dependency never enters this library's graph);
  SIEVE's scan-resistance makes it the better policy under churn-heavy traffic
  such as randomising bot UAs.

### Fixed

- CI fuzzing surfaced a worst-case input class (near-cap unclosed junk, e.g.
  `MSIE …; T` + 2 KB of filler) where walking the full pattern set costs
  ~0.2 ms/byte — ~450 ms per parse on fast hardware, several seconds on slow
  shared runners. No single pattern backtracks catastrophically (the 1 s match
  timeout stands); the cost is the aggregate walk, which the planned RE2
  prefilter (v1.2) addresses. Until then the fuzz time bound is two-tier (5 s
  for ≤512-byte inputs, 15 s above) and the crasher is committed as a
  regression seed.

## [1.0.1] - 2026-07-30

### Changed — supply chain

- All workflows now declare a least-privilege top-level `permissions` block
  (`contents: read`), with write scopes narrowed to the specific jobs that need
  them (releases, sync PRs, fuzz-crash issues) — raises the OpenSSF Scorecard
  *Token-Permissions* check.
- Added a CodeQL (`security-and-quality`) SAST workflow feeding GitHub code
  scanning — raises the Scorecard *SAST* check.

## [1.0.0] - 2026-07-30

First stable release. The public API is frozen: within the 1.x line there will be
no incompatible changes, enforced in CI. No code behaviour changes from 0.4.1 —
this release is the compatibility promise plus the governance to keep it.

### Added

- `docs/MIGRATION.md` — moving from the PHP `matomo/device-detector` (accessor and
  option mapping, lifecycle and behavioural differences) and from lightweight Go
  UA parsers.
- README comparison table against common Go User-Agent libraries.
- OpenSSF Scorecard workflow and badge (supply-chain posture, published results).

### Changed

- `apidiff`/`gorelease` is now a **hard CI gate**: an incompatible change against
  the latest release fails the build instead of being informational. Additive
  ("compatible") changes still pass.

### Stability

- API frozen since 0.2.0 and unchanged since; `gorelease` reports zero
  incompatibilities against 0.4.1.

## [0.4.1] - 2026-07-30

### Changed — supply chain

- All GitHub Actions are pinned to full commit SHAs (with a `# vN` comment)
  instead of floating tags, and `golangci-lint` is pinned to `v2.11.3` — closes
  the mutable-third-party-action gap flagged by OpenSSF Scorecard.
- Coverage is reported to [Codecov](https://codecov.io/gh/skalibog/device-detector-go)
  tokenlessly via OIDC from the fixtures job (informational only — the fixture
  gate remains the arbiter). README carries a live coverage badge.
- The upstream-sync workflow now runs fortnightly (1st and 15th) and defaults to
  the latest upstream **release tag** rather than `master`, so the corpus tracks
  stable points instead of near-daily brand additions.

## [0.4.0] - 2026-07-30

### Added — hardening

- `FuzzParse` fuzz target seeded from the fixture corpus, asserting Parse never
  panics, always returns a non-nil `Info`, reports only known device types,
  never leaks a `$N` capture-group placeholder, and stays time-bounded (a ReDoS
  regression guard, e.g. after a database sync). CI runs a 60 s smoke on every
  PR and a 30-minute nightly job that opens an issue on any crasher.
- `docs/FAQ.md` — leads with the LGPL / Go static-linking question, plus regex
  engine, database update policy, and untrusted-input notes.
- `docs/BENCHMARKS.md` — reproducible benchmark methodology and current numbers.

## [0.3.0] - 2026-07-29

### Added — Client Hints (v0.3)

- Full HTTP Client Hints support. New `(*DeviceDetector).ParseWithHints(ua,
  hints)` combines the user agent with `Sec-CH-UA-*` / `X-Requested-With`
  headers. Build hints with `NewClientHintsFromHeaders(http.Header)` (from a
  request) or `NewClientHintsFromMap` (for `navigator.userAgentData`). Ported:
  OS reconciliation (platform/version, Windows remap, Chrome OS/Meta Horizon,
  Fire/Lineage app remaps), browser reconciliation + `BrowserHints` app lookup,
  `MobileApp` app-id detection, frozen-UA restoration from the reported model,
  device model/form-factor fallback, and architecture/bitness platform.
- `Parse` (user-agent only) is unchanged; hint support is purely additive.

### Validation

- The full upstream fixture corpus now runs with client hints enabled and
  compares `os_family`/`browser_family` as well: **37,640 / 37,640 = 100%**.

## [0.2.1] - 2026-07-29

### Changed

- Database resynced to matomo/device-detector release **6.5.1** (was post-6.4.6
  `6f07f615`) — ~11 months of upstream updates. The hand-mirrored Go lookup
  tables were regenerated to match the new database: `deviceBrands` (2,106),
  `availableBrowsers` (716), `browserFamilies`, `mobileOnlyBrowsers` (226),
  `operatingSystems` (203), `osFamilies`, `availableEngines` (21). The full UA
  fixture corpus is back to 100% (37,262 entries).

### Fixed

- Bot names with capture-group placeholders are now substituted (e.g. `$1` over
  `(360Spider(?:-Image|-Video)?)`), matching `Bot::parse`. 6.5.1 introduced such
  names; older data used literal names only.
- `Redline` client is classified as a TV device (upstream #8201).

### Notes

- `scripts/sync-upstream.sh` now keeps a full local `upstream/` clone
  (gitignored) as an offline reference for porting and for diffing PHP between
  refs.

## [0.2.0] - 2026-07-29

### Changed (BREAKING — API freeze ahead of v1.0)

The public API is being locked down before v1.0. The detection machinery moved
under `internal/`, and the root package now owns every type a caller needs, so
the API no longer leaks the regex engine, the YAML library, or the pipeline
shape. Migrate once:

| Removed / changed | Use instead |
|---|---|
| `parser`, `parser/client`, `parser/device` packages | now `internal/` — not importable |
| `info.Device() int` | `info.DeviceType() DeviceType` |
| `device.TypeSmartphone` (untyped int) | `devicedetector.DeviceTypeSmartphone` (typed) |
| `parser.VersionTruncationNone` | `devicedetector.VersionTruncationNone` |
| `WithVersionTruncation(int)` | `WithVersionTruncation(VersionTruncation)` |
| `*parser.BotResult` | `*devicedetector.Bot` |
| `*parser.OSResult` | `*devicedetector.OS` |
| `*client.Result` (`Type string`) | `*devicedetector.Client` (`Type ClientType`) |
| `devicedetector.Unknown` (`"UNK"`) | removed — unknown fields are `""` |

Also:

- `Parse` never returns `(nil, error)`: on a stage error (match timeout, or a
  broken pattern in an external database) it returns the partial `*Info` built
  so far alongside the error, so results are best-effort. `Info` is never nil.
- `New` now compiles the whole database up front (fail-fast, ~130 ms for the
  embedded DB) so a broken external database surfaces at construction. Opt out
  with `WithLazyCompile`.
- New `(*DeviceDetector).IsBot(ua) (bool, error)` — a cheap bot-only check.
- `DeviceType` values match matomo's `DEVICE_TYPE_*` ids; `DeviceType.String()`
  and `DeviceTypeFromName` round-trip the canonical names.

## [0.1.2] - 2026-07-18

### Security

- Bound denial-of-service on crafted user agents. The backtracking engine could
  be pinned for tens of seconds by an oversized junk UA (a ~24 KB input held one
  core for ~60 s). Two guards are now on by default: a 2048-byte length cap
  (`WithMaxUARawLength`) and a 1 s per-match timeout (`WithMatchTimeout`),
  together bounding that input to ~1 s. See SECURITY.md for hardening untrusted
  input. The durable RE2-prefilter fix is tracked in ROADMAP.md.

### Fixed

- Empty regex lists no longer match every user agent. A `preMatchOverall` regex
  built from an empty list degraded into a bare anchor that matched everything
  (upstream fix in matomo/device-detector 6.5.1, PR #8271). Individual empty
  patterns keep their catch-all semantics (e.g. Roku's "Digital Video Player").

### Added

- `WithMaxUARawLength`, `WithMatchTimeout` options; `parser.SetMatchTimeout`.
- `govulncheck` CI job (push/PR + weekly cron).
- `ROADMAP.md`: the plan to v1.0.

### Notes

- Database unchanged: matomo/device-detector `6f07f615`.
- Non-breaking: the new guards only affect oversized or pathological input.

## [0.1.1] - 2026-07-15

### Changed

- Module path renamed to `github.com/skalibog/device-detector-go` (repository rename).
- Added pkg.go.dev documentation: extended package docs, runnable examples.
- golangci-lint config with enforced godoc on exported identifiers.

### Removed

- **v0.1.0 is retracted**: it was tagged before the repository rename and
  declares the old module path, so it cannot be fetched. Use v0.1.1.

## [0.1.0] - 2026-07-15 [RETRACTED]

### Added

- Initial release: native Go port of matomo/device-detector (UA-string pipeline).
- Bot detection (1,083 corpus entries), OS parser (186 systems, platforms, families),
  client parsers (browsers with engine/engine-version, feed readers, libraries,
  media players, mobile apps, PIM), device parsers (2,084 brands, 14 device types),
  vendor fragments, and the full `DeviceDetector` post-detection heuristics chain.
- Embedded regex database via `go:embed` (`New()`), external database loading
  (`NewFromDir`/`NewFromFS`), version truncation and bot-skip options.
- Upstream fixture corpus replay in CI: 36,333 entries, zero-mismatch gate.
- Monthly automated upstream database sync workflow.

### Notes

- Database: matomo/device-detector `6f07f615` (post-6.4.6 master).
- Client Hints are not yet supported; hints-dependent fixture entries are
  excluded from the corpus gate until v0.2.

[Unreleased]: https://github.com/skalibog/device-detector-go/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/skalibog/device-detector-go/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/skalibog/device-detector-go/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/skalibog/device-detector-go/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/skalibog/device-detector-go/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/skalibog/device-detector-go/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/skalibog/device-detector-go/releases/tag/v0.1.0
