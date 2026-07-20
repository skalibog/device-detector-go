# Roadmap to v1.0

This document is the plan for reaching a frozen, production-grade v1.0. It is
derived from a five-dimension audit (Client Hints parity, upstream drift, API
freeze, robustness/performance, ecosystem canon).

**Guiding constraint:** at v1.0 the public API and module path freeze forever
(a v2 needs a new import path). So every breaking change must land in the
0.x line, and — critically — the API must be internalized *before* additive
feature work, so that later features don't force further breakage.

## Sequencing insight

The parser subpackages (`parser`, `parser/client`, `parser/device`) currently
leak ~135 exported declarations, including the regex engine and YAML library.
Moving them under `internal/` is a breaking change — but once done, the
Client Hints work (which changes internal parser signatures to `Parse(ua,
hints)`) becomes **purely additive** to the public API. Therefore: freeze the
API first, add Client Hints second. Consumers break exactly once.

Two issues are live in the already-published v0.1.1 and jump the queue as a
security patch:

- **ReDoS**: a ~6 KB junk User-Agent pins one core for ~5 s; ~24 KB for ~60 s.
  `regexp2` is configured with no `MatchTimeout`. Attacker-controlled UAs
  (the entire point of this library) are a live denial-of-service vector.
- **Empty-regexes false-positive**: `CombineRegexes(nil)` yields `""`, which
  compiles to `(?:)` and matches every UA — a latent bug upstream fixed in
  6.5.1 (PR #8271). Confirmed reproducible here.

---

## v0.1.2 — Security patch (non-breaking, ship first)

| Item | Effort | Notes |
|------|--------|-------|
| `regexp2.MatchTimeout` on all compiled patterns, `WithMatchTimeout` option (default 50 ms) | M | fastclock is one shared goroutine — arming 19,771 regexps does not spawn 19,771 goroutines |
| UA length cap before parsing, `WithMaxUARawLength` (default 2048; corpus max is 401) | S | defense-in-depth; only effective *together* with MatchTimeout |
| Empty-regexes guard in `preMatchOverall`/`CombineRegexes` path | S | port of upstream PR #8271; fixes confirmed false-positive |
| `govulncheck` CI job (push/PR + weekly cron) | S | 2 direct deps; runtime is seconds |

Acceptance: every adversarial input (6–64 KB junk) returns in <500 ms; fixture
corpus stays 100.00%; `BenchmarkParse` regression <5%.

> Contract change to document: adversarial input now returns `(nil, err)`
> instead of a slow success. Callers must treat a Parse error as "unknown UA".

## v0.2.0 — API freeze (breaking; consumers migrate once)

| Item | Effort | Notes |
|------|--------|-------|
| Move `parser`, `parser/client`, `parser/device` → `internal/` | L | removes ~135 decls from the frozen surface |
| Root-owned result types + typed enums: `DeviceType`, `VersionTruncation`, `ClientType`; `Info.Device() int` → `Info.DeviceType() DeviceType` | M | keep matomo's numeric device-type ids (0–13, -1) for analytics interop |
| Partial-result `Parse` contract: never return `(nil, error)` | M | preserve computed OS/client on downstream stage error |
| Compile all client/device regexes at construction (fail-fast `New`) | M | today client/device compile lazily → bad external DB surfaces per-UA |
| Remove exported `Unknown = "UNK"` sentinel | S | nil/"" is the Go-idiomatic unknown signal |
| Root fast-path `(*DeviceDetector).IsBot(ua)` | S | preserve the cheap bot-only check that internalizing `parser` would remove |
| Normalize error prefixes to `devicedetector:`; document error contract | S | |
| Freeze-quality godoc: concurrency, zero-value, `Info` immutability | S | |
| `apidiff`/`gorelease` CI gate (informational pre-1.0, hard-fail from 1.0) | S | |

Ship with a CHANGELOG migration table (old symbol → new symbol). CHANGELOG
already has one retracted tag; be loud.

## v0.3.0 — Client Hints (additive thanks to v0.2.0 internalization)

Full parity with upstream's Client Hints support. 8 work items (2 L + 2 M +
4 S). All five `TODO(client-hints)` markers map 1:1 onto PHP branches.

| Item | Effort |
|------|--------|
| `ClientHints` core type + `NewClientHintsFromHeaders`/`FromMap` factories (port `ClientHints.php`) | M |
| UA restoration + shared hint helpers (port `AbstractParser` CH methods) | S |
| OS client-hints path (resolve `os.go` TODOs) — merge block, Windows version remap, app-id OS remaps | L |
| Browser CH reconciliation + `BrowserHints` lookup (browsers.yml, 325 entries) | L |
| MobileApp `AppHints` override (apps.yml, 162 entries) | S |
| Device CH fallback + `ParseWithHints(ua, hints)` orchestrator wiring | M |
| Enable hint fixtures; also compare `os_family`/`browser_family` | M |
| Database resync to upstream tag 6.5.1 + coordinated Go-table update | L |

Acceptance: full corpus **36,677/36,677** at 100.00% (all skips removed),
including the `os_family`/`browser_family` fields upstream compares.

Watch: PHP type-coercion quirks are load-bearing (`(int)` version casts,
`version_compare` prefix upgrade, ordered brand list — a slice, not a map).

## v0.4.0 — Pre-1.0 hardening

| Item | Effort | Gate? |
|------|--------|-------|
| RE2 stdlib-regexp prefilter fast-path, `regexp2` fallback for the 165/19,771 lookaround patterns (99.17% RE2-clean) | XL | perf, not correctness — the durable ReDoS fix |
| `FuzzParse` seeded from fixture UAs (panic + invariant + <1s/exec assertions) | M | ✅ |
| CI fuzz: 60 s smoke on PR, 30 m nightly cron | S | |
| Reproducible benchmarks: stratified corpus sample + `benchstat` + `docs/BENCHMARKS.md`; correct README heap figure (measured 141 MB, not 110) | M | ✅ |
| Real coverage (Codecov tokenless/OIDC) replacing the static 83% badge | S | ✅ |
| `docs/FAQ.md` leading with the LGPL/static-linking answer | S | ✅ |
| Pin-update policy: upstream tags only, quarterly cadence | S | ✅ |
| SHA-pin GitHub Actions; pin golangci-lint version | S | |

Acceptance: warm p99 parse <5 ms (from 43.8 ms), max <15 ms (from 84 ms);
`-fuzztime=1h` clean; benchmarks reproducible from repo.

## v1.0.0 — Freeze

- Final API review; `apidiff` gate flips to hard-fail.
- OpenSSF Scorecard workflow + badge (only after SHA-pinning + branch
  protection land — a premature low score is worse than none).
- README comparison table + `MIGRATION.md` from other Go UA parsers
  (public-docs-only sourcing; clean-room hard rule).
- Optional: Windows/macOS CI smoke; community pack (Discussions, labels).

---

## Explicitly deferred (post-1.0 or opt-in)

- **`goccy/go-yaml` migration** (yaml.v3 is archived but functional, no CVE) —
  not a 1.0 blocker; a standalone PR gated on the unchanged corpus. `OrderedMap`
  and the `flexString` verbatim-scalar path are the high-risk spots.
- **In-library result LRU** (`WithResultCache`) — biggest throughput lever for
  repeat-heavy RTB traffic, but opt-in; core stays pure/stateless by default.
- **Non-goals** (recorded so they aren't cargo-culted): GoReleaser (no
  binaries), SLSA attestations (no artifacts), CODEOWNERS (solo maintainer).

## Standing risks

- Hand-mirrored Go tables (`os_data.go`, `browser_maps.go`, `device.go`) drift
  silently between resyncs — the fixture gate only catches entries fixtures
  exercise. Table-diff tooling (a v0.4 opt item) closes this.
- MatchTimeout bounds one match, not a whole Parse; the length cap bounds match
  count. Both guards are only effective together.
- The RE2 fast-path touches the hottest parity-sensitive code (capture-group
  extraction → `BuildByMatch` `$1..$N`); the 100% gate mitigates, but it is XL
  and must not be rushed alongside the timeout work.
