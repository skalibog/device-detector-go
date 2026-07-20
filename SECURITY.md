# Security Policy

## Supported versions

Only the latest release receives fixes.

## Reporting a vulnerability

Please do not open public issues for security problems (e.g. patterns causing
catastrophic regex backtracking / denial of service on attacker-controlled UAs).

Report privately via [GitHub Security Advisories](https://github.com/skalibog/device-detector-go/security/advisories/new).
You will get a response within a week. Issues in the regex database itself may
need to be coordinated with upstream matomo/device-detector.

## Denial of service on untrusted user agents

The detection database mirrors upstream's design: one large backtracking
regex alternation per file. A crafted, oversized user agent can therefore
trigger heavy backtracking. Since v0.1.2 two guards are on by default:

- **Length cap** — user agents longer than `DefaultMaxUARawLength` (2048
  bytes) are truncated before parsing. The longest real user agent in the
  upstream corpus is ~400 bytes, so genuine traffic is never affected. This
  alone takes a ~24 KB junk input from ~60 s down to ~1 s.
- **Per-match timeout** — every compiled pattern carries `DefaultMatchTimeout`
  (1 s), so a single catastrophically-backtracking match is abandoned rather
  than running unbounded. On normal input it is effectively free.

For high-volume ingestion of untrusted user agents, tighten both:

```go
d, _ := devicedetector.New(
    devicedetector.WithMaxUARawLength(512),      // ~170 ms worst case
    devicedetector.WithMatchTimeout(100*time.Millisecond),
)
```

When a match times out, `Parse` returns `(nil, error)`; treat that as an
unknown user agent. A tuned `WithMaxUARawLength` may truncate the rare
legitimate user agent above the cap, so exact-parity callers can disable it
with `WithMaxUARawLength(0)` (matching upstream, which imposes no limit).

The durable fix — an RE2 linear-time prefilter that keeps backtracking off the
hot path entirely — is tracked for a later release in [ROADMAP.md](ROADMAP.md).
