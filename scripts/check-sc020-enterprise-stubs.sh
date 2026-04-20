#!/usr/bin/env bash
# SC-020 (spec 002 evolution — zero-tolerance body-grep):
# The legacy marketing string "This feature is a part of the Bifrost
# enterprise license" (and its related CTAs) MUST NOT appear anywhere
# in the enterprise component tree. Every descoped stub that previously
# rendered a ContactUsView teaser now renders a FeatureStatusPanel with
# honest status + tracking link.
#
# Supersedes the earlier path-whitelist model from spec 001 — there is
# no whitelist. Any match anywhere under ui/app/enterprise/components/
# is a violation, reported with file:line:excerpt.
#
# Runs against the source tree (stable across bundle fingerprints).

set -euo pipefail

red()    { printf '\033[0;31m%s\033[0m\n' "$*"; }
green()  { printf '\033[0;32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[1;33m%s\033[0m\n' "$*"; }

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

# --------------------------------------------------------------------------
# Forbidden substrings. Any of these appearing inside a file under
# ui/app/enterprise/components/ is a violation. Edit the list (don't add
# whitelists) if a new flavor of legacy-teaser copy needs blocking.
# --------------------------------------------------------------------------
FORBIDDEN=(
  "This feature is a part of the Bifrost enterprise license"
  "This feature is part of the Bifrost enterprise license"
  "Contact Us"
  "Contact Sales"
  "Upgrade to Enterprise"
  "would love to know"
)

yellow "Scanning ui/app/enterprise/components for forbidden marketing strings..."
echo

VIOLATIONS=0
SCANNED=0

while IFS= read -r -d '' file; do
  SCANNED=$((SCANNED + 1))
  rel="${file#"$REPO_ROOT/"}"
  # The shared ContactUsView component lives here and its props legitimately
  # name the pattern we're blocking elsewhere — skip the one file that
  # defines the component itself.
  case "$rel" in
    ui/app/enterprise/components/views/contactUsView.tsx) continue ;;
  esac
  for needle in "${FORBIDDEN[@]}"; do
    # grep -nF: numbered, fixed-string match. Prints file:line:excerpt.
    matches=$(grep -nF -- "$needle" "$file" 2>/dev/null || true)
    if [[ -n "$matches" ]]; then
      while IFS= read -r m; do
        red "❌ $rel:$m"
        VIOLATIONS=$((VIOLATIONS + 1))
      done <<< "$matches"
    fi
  done
done < <(find "$REPO_ROOT/ui/app/enterprise/components" -type f \( -name '*.tsx' -o -name '*.ts' \) -print0 2>/dev/null)

echo
echo "Scanned:    $SCANNED files"
echo "Violations: $VIOLATIONS"
echo

if (( VIOLATIONS > 0 )); then
  red "SC-020 check FAILED."
  echo
  echo "Each violation above names the file + line number. Fix by"
  echo "rendering a FeatureStatusPanel with honest status + tracking"
  echo "link instead of the legacy marketing copy. See"
  echo "specs/002-expose-hidden-enterprise-stubs/contracts/feature-status-panel.md"
  echo "for the component contract."
  exit 1
fi

green "SC-020 check PASSED."
