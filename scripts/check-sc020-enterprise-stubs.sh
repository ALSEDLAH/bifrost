#!/usr/bin/env bash
# SC-020 (revised 2026-04-20 per SR-01 "reuse-over-new"):
# Every enterprise stub for an IN-SCOPE user story must render a real
# implementation — not the OSS ContactUsView fallback. DESCOPED stubs
# are whitelisted below and remain as fallback re-exports by design.
#
# Runs against the source tree (more stable than the minified bundle,
# since bundle filenames are content-hashed per build).
#
# Exit non-zero if any IN-SCOPE stub is still a fallback re-export.

set -euo pipefail

red()    { printf '\033[0;31m%s\033[0m\n' "$*"; }
green()  { printf '\033[0;32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[1;33m%s\033[0m\n' "$*"; }

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

# --------------------------------------------------------------------------
# DESCOPED paths — allowed to remain fallback re-exports. Each line is a
# source path relative to repo root. Keep sorted by user story for review.
# When a feature graduates to in-scope, REMOVE it from this list and
# ensure the real implementation lives at the same path.
# --------------------------------------------------------------------------
DESCOPED_STUBS=(
  # Infrastructure (not a feature stub) — contactUsView IS the shared
  # fallback component that every descoped stub re-exports from.
  "ui/app/enterprise/components/views/contactUsView.tsx"

  # US1 — Orgs/Workspaces management lives at
  # /workspace/governance/business-units (businessUnitsView) and
  # /workspace/governance/teams (teamsView) — both are real
  # implementations. These two paths are unused vestiges kept only to
  # satisfy the vite @enterprise alias resolver; no route mounts them.
  "ui/app/enterprise/components/orgs-workspaces/workspacesView.tsx"
  "ui/app/enterprise/components/orgs-workspaces/organizationSettingsView.tsx"

  # US3 — SSO (net-new SAML/OIDC handlers)
  "ui/app/enterprise/components/login/loginView.tsx"

  # US5 — Admin API Keys (upstream basic-auth already provides admin auth)
  "ui/app/enterprise/components/api-keys/apiKeysIndexView.tsx"
  "ui/app/enterprise/components/access-profiles/accessProfilesIndexView.tsx"

  # US6 — Central Guardrails (net-new plugin)
  "ui/app/enterprise/components/guardrails/guardrailsConfigurationView.tsx"
  "ui/app/enterprise/components/guardrails/guardrailsProviderView.tsx"

  # US7 — PII Redactor (net-new plugin)
  "ui/app/enterprise/components/pii-redactor/piiRedactorRulesView.tsx"
  "ui/app/enterprise/components/pii-redactor/piiRedactorProviderView.tsx"

  # US10 — Alerts (net-new plugin)
  "ui/app/enterprise/components/alert-channels/alertChannelsView.tsx"

  # US11 — Log Export (net-new plugin)
  "ui/app/enterprise/components/data-connectors/datadog/datadogConnectorView.tsx"
  "ui/app/enterprise/components/data-connectors/bigquery/bigqueryConnectorView.tsx"

  # US12 — Executive Dashboard (no upstream user-rankings endpoint)
  "ui/app/enterprise/components/user-rankings/userRankingsTab.tsx"

  # US14 — Prompt Deployments (net-new plugin work)
  "ui/app/enterprise/components/prompt-deployments/promptDeploymentView.tsx"
  "ui/app/enterprise/components/prompt-deployments/promptDeploymentsAccordionItem.tsx"

  # US19 — Cluster management (net-new)
  "ui/app/enterprise/components/cluster/clusterView.tsx"

  # US20 — SCIM (net-new handler)
  "ui/app/enterprise/components/scim/scimView.tsx"

  # US22 — Adaptive routing (net-new, depends on US16 config objects)
  "ui/app/enterprise/components/adaptive-routing/adaptiveRoutingView.tsx"

  # US30 subset — MCP Tool Groups (no upstream grouping), Large Payload
  # settings (no persistence endpoint)
  "ui/app/enterprise/components/mcp-tool-groups/mcpToolGroups.tsx"
  "ui/app/enterprise/components/large-payload/largePayloadSettingsFragment.tsx"
)

is_descoped() {
  local path="$1"
  for entry in "${DESCOPED_STUBS[@]}"; do
    [[ "$path" == "$entry" ]] && return 0
  done
  return 1
}

yellow "Scanning ui/app/enterprise/components for fallback re-exports..."

VIOLATIONS=0
CHECKED=0
WHITELISTED=0

while IFS= read -r -d '' file; do
  CHECKED=$((CHECKED + 1))
  # Detect the pattern used by generated stubs: a single re-export line.
  if grep -q 'from "\..*_fallbacks/enterprise/components/' "$file" 2>/dev/null; then
    rel="${file#"$REPO_ROOT/"}"
    if is_descoped "$rel"; then
      WHITELISTED=$((WHITELISTED + 1))
    else
      red "❌ IN-SCOPE stub still a fallback re-export: $rel"
      VIOLATIONS=$((VIOLATIONS + 1))
    fi
  fi
done < <(find "$REPO_ROOT/ui/app/enterprise/components" -type f -name '*.tsx' -print0 2>/dev/null)

echo
echo "Scanned:      $CHECKED stub candidates"
echo "Whitelisted:  $WHITELISTED (descoped per SR-01)"
echo "Violations:   $VIOLATIONS"
echo

if (( VIOLATIONS > 0 )); then
  red "SC-020 check FAILED: $VIOLATIONS in-scope stub(s) not implemented."
  echo
  echo "Either fill the stub with a real implementation, or — if the"
  echo "feature should be descoped — open a feature spec and add the"
  echo "path to DESCOPED_STUBS in this script."
  exit 1
fi

green "SC-020 check PASSED."
