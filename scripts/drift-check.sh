#!/usr/bin/env bash
# Constitution Principle XI rule 6 — fork drift watcher.
# Computes the cumulative line diff for files in .github/drift-watchlist.txt
# vs upstream/main. Fails if it grows beyond baseline + ceiling unless the
# commit message starts with `drift:` (explicit operator acknowledgment).

set -euo pipefail

red()    { printf '\033[0;31m%s\033[0m\n' "$*"; }
green()  { printf '\033[0;32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[1;33m%s\033[0m\n' "$*"; }

WATCHLIST=".github/drift-watchlist.txt"
BASELINE_FILE=".github/drift-baseline.json"

if [ ! -f "$WATCHLIST" ] || [ ! -f "$BASELINE_FILE" ]; then
  red "❌ Drift config missing ($WATCHLIST or $BASELINE_FILE)"
  exit 2
fi

# Ensure upstream remote exists. CI sets it up; local devs may need to.
if ! git remote get-url upstream >/dev/null 2>&1; then
  yellow "⚠ no 'upstream' remote configured; skipping drift check (configure with: git remote add upstream https://github.com/maximhq/bifrost.git)"
  exit 0
fi
git fetch upstream main --quiet 2>/dev/null || true

# Build watchlist as space-separated paths (skip blank lines and comments)
PATHS=$(grep -vE '^(#|$)' "$WATCHLIST" | tr '\n' ' ')
if [ -z "$PATHS" ]; then
  yellow "⚠ drift watchlist is empty"
  exit 0
fi

# Compute current diff size against upstream/main.
# `git diff --shortstat` example:  3 files changed, 47 insertions(+), 12 deletions(-)
SHORT=$(eval git diff --shortstat upstream/main -- $PATHS 2>/dev/null || true)
INS=$(echo "$SHORT" | grep -oE '[0-9]+ insertion' | grep -oE '^[0-9]+' || echo 0)
DEL=$(echo "$SHORT" | grep -oE '[0-9]+ deletion'  | grep -oE '^[0-9]+' || echo 0)
DIFF_LINES=$(( ${INS:-0} + ${DEL:-0} ))

BASELINE=$(grep -oE '"baseline_lines"\s*:\s*[0-9]+' "$BASELINE_FILE" | grep -oE '[0-9]+$')
CEILING=$(grep -oE '"ceiling_lines"\s*:\s*[0-9]+'   "$BASELINE_FILE" | grep -oE '[0-9]+$')
THRESHOLD=$(( BASELINE + CEILING ))

yellow "Watchlist drift: $DIFF_LINES lines (baseline $BASELINE + ceiling $CEILING = threshold $THRESHOLD)"

# Inspect HEAD commit message for the `drift:` opt-in prefix.
HEAD_MSG=$(git log -1 --pretty=%B)

if [ "$DIFF_LINES" -le "$THRESHOLD" ]; then
  green "✅ Drift within threshold."
  exit 0
fi

if echo "$HEAD_MSG" | head -1 | grep -qE '^drift:'; then
  yellow "⚠ Drift exceeds threshold but HEAD commit is prefixed 'drift:' — operator acknowledged. Allowing."
  exit 0
fi

red "❌ Drift exceeds threshold ($DIFF_LINES > $THRESHOLD)."
red "   Either:"
red "   1. Move new additions into a sibling file (Principle XI rule 1), OR"
red "   2. Prefix the HEAD commit message with 'drift:' to explicitly acknowledge it."
red ""
red "   Current diff against upstream/main:"
eval git diff --stat upstream/main -- $PATHS
exit 1
