# Contributing

Thanks for your interest! A few ground rules keep this port maintainable and legally clean.

## Where does my issue belong?

- **Wrong or missing detection for a UA** (unknown device, wrong brand/model/browser):
  in most cases this is a *database* issue — report it upstream to
  [matomo/device-detector](https://github.com/matomo-org/device-detector/issues).
  Once it lands upstream, the monthly sync brings it here. If the same UA parses
  correctly in upstream PHP but wrongly here, that is a **port bug** — file it here
  with the UA and both outputs.
- **API, performance, crashes, Go-specific behavior** — file it here.

## Licensing rules (non-negotiable)

- This project is a derivative work of matomo/device-detector and is licensed
  **LGPL-3.0-or-later**. All contributions are accepted under the same license.
- Port logic **only from the upstream PHP sources**. Do not copy code from other
  ports of device-detector (their licensing is often unclear); do not paste code
  whose provenance you cannot state.

## Development

```bash
make test            # unit tests + short fixture corpus
make test-fixtures   # full 36k corpus (~1 min) — must stay at 100.00%
make vet
golangci-lint run
```

- The fixture gate is the contract: a PR that changes parsing must keep the corpus
  at zero mismatches, or update the database pin together with the code and explain why.
- Match upstream behavior bit-for-bit, even where it looks odd — quirks are load-bearing
  (see the two different `matchUserAgent` anchors). Document intentional deviations in
  the PR and in code comments.
- New code needs tests. Keep parsers immutable after construction; `Parse` must stay
  safe for concurrent use (CI runs the race detector).

## Releases (maintainer notes)

1. Update `CHANGELOG.md` (move Unreleased → new version; note the database commit).
2. Tag: `git tag vX.Y.Z && git push --tags`. The release workflow publishes a GitHub
   Release with the changelog section; the Go module proxy picks up the tag automatically.
3. Never delete or re-tag a published version — the module proxy has already cached it.
   For a broken release, publish a fix and add a [`retract`](https://go.dev/ref/mod#go-mod-file-retract)
   directive in `go.mod`.
4. Database-only refreshes are patch releases (`v0.1.1`), API additions are minor.
