#!/usr/bin/env bash
# Syncs the regex database and test fixtures from matomo/device-detector.
# Usage: scripts/sync-upstream.sh [ref]
set -euo pipefail

REF="${1:-6f07f615199851548db47a900815d2ea2cdcde08}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$ROOT/upstream"

rm -rf "$TMP"
git clone --filter=blob:none https://github.com/matomo-org/device-detector "$TMP"
git -C "$TMP" checkout --quiet "$REF"

rsync -a --delete "$TMP/regexes/" "$ROOT/data/regexes/"
rsync -a --delete "$TMP/Tests/fixtures/" "$ROOT/testdata/fixtures/"
cp "$TMP/LICENSE" "$ROOT/LICENSE"

rm -rf "$TMP"
echo "Synced regexes + fixtures from matomo/device-detector @ $REF"
echo "Remember to update the pinned commit in README.md and data/NOTICE.md"
