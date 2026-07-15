# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Until v1.0.0, minor releases may contain breaking API changes; patch releases never do.

Each release also notes the pinned matomo/device-detector database commit it ships.

## [Unreleased]

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

[Unreleased]: https://github.com/skalibog/device-detector-go/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/skalibog/device-detector-go/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/skalibog/device-detector-go/releases/tag/v0.1.0
