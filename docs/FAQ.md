# FAQ

## Is the LGPL-3.0 license a problem for my Go program?

This is the most common adoption question, so the honest version first.

`device-detector-go` is a derivative work of matomo/device-detector and is
therefore **LGPL-3.0-or-later** — the license cannot be changed (see
[CONTRIBUTING.md](../CONTRIBUTING.md)). Go links everything statically, and the
LGPL's dynamic-linking exception does not map cleanly onto static linking, so
the relevant clause is **LGPL §4** (combined works): you may ship a proprietary,
statically-linked binary that uses this library **provided you let a recipient
relink it against a modified version of the library.**

In practice, for a Go binary, that is satisfied by any of:

- **Publish the exact library source you built against** (a pinned commit /
  module version is enough — the Go module proxy already preserves it), and note
  it in your distribution. Your own code stays closed. This is the low-effort
  path most projects take.
- **Provide the linkable object form** of your application (e.g. the `.a`/object
  files, or the ability to rebuild) so a recipient can relink.
- **Use it only server-side.** LGPL obligations attach on *distribution* of the
  binary. If you run the library on your own servers and never ship the binary,
  there is nothing to convey (LGPL is not AGPL — there is no network-use clause).

You must also keep the copyright and license notices (they are in `LICENSE` and
`data/NOTICE.md`), and not present the library as your own work.

This is guidance, not legal advice — if your distribution model is unusual, ask
a lawyer. But for the two common cases — *SaaS/server-side* and *shipping a
binary while publishing the pinned library version* — LGPL is not a blocker.

## Why the `regexp2` engine instead of the standard library `regexp`?

The detection database uses PCRE features (lookahead, lookbehind, backreferences)
that Go's RE2-based `regexp` cannot express. `regexp2` is a faithful PCRE-style
engine, so the port reproduces upstream output exactly. The trade-off is that it
backtracks; see [Performance](../README.md#performance) and
[SECURITY.md](../SECURITY.md) for the bounds and the guards that are on by
default. A linear-time RE2 prefilter for the common patterns is on the roadmap.

## How does the detection data stay current?

The regex database and fixtures are vendored verbatim from a pinned
matomo/device-detector release (see [data/NOTICE.md](../data/NOTICE.md)). A
scheduled workflow re-syncs from upstream and opens a PR; the fixture gate then
proves the Go code still reproduces upstream output on the new corpus. Detection
*data* issues (a wrong or missing device) belong upstream — once fixed there, the
next sync brings them here. See [CONTRIBUTING.md](../CONTRIBUTING.md).

## Is it safe to parse attacker-controlled User-Agents?

Yes, with the default guards (length cap + per-match timeout). See
[SECURITY.md](../SECURITY.md) for tuning them for high-volume untrusted input.

## Does it support Client Hints?

Yes — `ParseWithHints`. See the [Client Hints](../README.md#client-hints)
section of the README.
