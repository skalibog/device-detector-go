#!/usr/bin/env bash
# Syncs the regex database and test fixtures from matomo/device-detector.
#
# Keeps a full local clone under upstream/ (gitignored) as an offline reference
# for porting logic and for diffing PHP between refs, e.g.:
#   git -C upstream diff <old-ref> <new-ref> -- 'Parser/**/*.php'
#
# Usage: scripts/sync-upstream.sh [ref]
set -euo pipefail

REF="${1:-e83ce7a97f05d8ee3e1eee7ea77a73b3bdfebd9a}" # matomo/device-detector 6.5.2
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
UP="$ROOT/upstream"

if [ -d "$UP/.git" ]; then
	git -C "$UP" fetch --quiet --tags origin
else
	git clone --quiet https://github.com/matomo-org/device-detector "$UP"
fi

git -C "$UP" checkout --quiet "$REF"

rsync -a --delete "$UP/regexes/" "$ROOT/data/regexes/"
rsync -a --delete "$UP/Tests/fixtures/" "$ROOT/testdata/fixtures/"
cp "$UP/LICENSE" "$ROOT/LICENSE"

echo "Synced regexes + fixtures from matomo/device-detector @ $REF"
echo "upstream/ kept as local reference (gitignored)."
echo "Remember to update the pin in README.md and data/NOTICE.md, and re-check the Go tables"
echo "(deviceBrands, availableBrowsers, browserFamilies, mobileOnlyBrowsers, operatingSystems,"
echo " osFamilies, availableEngines) against the new ref — the fixture gate will flag drift."
