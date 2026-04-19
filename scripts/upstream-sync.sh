#!/usr/bin/env bash
# scripts/upstream-sync.sh — local equivalent of the upstream-sync workflow.
# Lets a maintainer dry-run the weekly sync from their workstation before
# (or instead of) the scheduled CI run.
#
# Constitution Principle XI rule 5.

set -euo pipefail

UPSTREAM_URL="${UPSTREAM_URL:-https://github.com/maximhq/bifrost.git}"
SYNC_BRANCH="sync/upstream-$(date -u +%Y-%m-%d)"

red()    { printf '\033[0;31m%s\033[0m\n' "$*"; }
green()  { printf '\033[0;32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[1;33m%s\033[0m\n' "$*"; }

if ! git remote get-url upstream >/dev/null 2>&1; then
  yellow "Adding upstream remote: $UPSTREAM_URL"
  git remote add upstream "$UPSTREAM_URL"
fi

yellow "Fetching upstream/main..."
git fetch upstream main

BEHIND=$(git rev-list --count HEAD..upstream/main)
if [ "$BEHIND" -eq 0 ]; then
  green "✅ Fork is up-to-date with upstream/main; nothing to do."
  exit 0
fi

yellow "Fork is $BEHIND commit(s) behind upstream/main."
yellow "Creating sync branch: $SYNC_BRANCH"
git checkout -b "$SYNC_BRANCH" main

yellow "Merging upstream/main with --no-ff..."
if git merge upstream/main --no-ff -m "sync: merge upstream/main ($(git rev-parse --short upstream/main))"; then
  green "✅ Merge clean. Run the test matrix:"
  echo "    make test-core"
  echo "    make test-plugins"
  echo "    make test-enterprise   # requires docker-compose.enterprise.yml stack"
  echo "    go test -tags=golden ./tests/golden/..."
  echo ""
  echo "If green, push: git push origin $SYNC_BRANCH"
  echo "Then open a PR titled: 'sync: upstream/main @ $(git rev-parse --short upstream/main)'"
else
  red "❌ Conflicts. Open UPSTREAM-SYNC.md and follow the conflict playbook."
  red "   When resolved: git add . && git commit && rerun the test matrix."
  exit 1
fi
