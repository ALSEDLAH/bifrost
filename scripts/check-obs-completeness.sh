#!/usr/bin/env bash
# Constitution Principle VI — Observability Mandatory
# For every plugin listed in plugins/enterprise-gate/features.go (the
# enterprise feature manifest), assert it contains:
#   (a) at least one OTEL span (otel.Tracer or tracer.Start)
#   (b) at least one Prometheus metric registration (telemetry.Register* or prometheus.New*)
#   (c) at least one audit emit (audit.Emit)
#
# Plugins may opt out of a single signal with a `// obs-exempt: <reason>` comment;
# any opt-out must be documented in the plugin's feature plan.md "Deferred Observability" section.

set -euo pipefail

red()    { printf '\033[0;31m%s\033[0m\n' "$*"; }
green()  { printf '\033[0;32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[1;33m%s\033[0m\n' "$*"; }

MANIFEST=plugins/enterprise-gate/features.go
if [ ! -f "$MANIFEST" ]; then
  yellow "⚠ enterprise-gate feature manifest not yet present at $MANIFEST — skipping completeness check"
  yellow "  (this is normal during Phase 1; completeness check activates once Phase 2 lands)"
  exit 0
fi

# Extract plugin directory names from the manifest. Convention: each entry is a Go string literal
# matching plugins/<name>/. Example line in features.go:
#     {Name: "audit", Dir: "plugins/audit"},
PLUGINS=$(grep -oE 'plugins/[a-z][a-z0-9_-]+' "$MANIFEST" | sort -u | sed 's|plugins/||')

if [ -z "$PLUGINS" ]; then
  yellow "⚠ no enterprise plugins enumerated in $MANIFEST — nothing to check"
  exit 0
fi

VIOLATIONS=0
for plugin in $PLUGINS; do
  dir="plugins/$plugin"
  if [ ! -d "$dir" ]; then
    yellow "⚠ $dir referenced in manifest but directory does not yet exist — skipping"
    continue
  fi

  # Allow opt-out via comment: // obs-exempt: <reason>
  exempt_otel=$(grep -lE '// obs-exempt:.*otel'        "$dir"/*.go 2>/dev/null || true)
  exempt_prom=$(grep -lE '// obs-exempt:.*(prom|metric)' "$dir"/*.go 2>/dev/null || true)
  exempt_audit=$(grep -lE '// obs-exempt:.*audit'      "$dir"/*.go 2>/dev/null || true)

  has_otel=$(grep -rEl '(otel\.Tracer|\.Tracer\(.*\)\.Start|tracer\.Start)'       "$dir" --include='*.go' 2>/dev/null || true)
  has_prom=$(grep -rEl '(telemetry\.Register|prometheus\.New|promauto\.New)'      "$dir" --include='*.go' 2>/dev/null || true)
  has_audit=$(grep -rEl 'audit\.Emit'                                              "$dir" --include='*.go' 2>/dev/null || true)

  miss=()
  [ -z "$has_otel"  ] && [ -z "$exempt_otel"  ] && miss+=("OTEL span")
  [ -z "$has_prom"  ] && [ -z "$exempt_prom"  ] && miss+=("Prometheus metric")
  [ -z "$has_audit" ] && [ -z "$exempt_audit" ] && miss+=("audit emit")

  if [ ${#miss[@]} -gt 0 ]; then
    red "❌ plugins/$plugin missing: ${miss[*]}"
    VIOLATIONS=$((VIOLATIONS + 1))
  else
    green "✅ plugins/$plugin: OTEL + Prometheus + audit"
  fi
done

if [ $VIOLATIONS -eq 0 ]; then
  green "✅ Observability completeness: all enterprise plugins emit all three signals."
  exit 0
else
  red "❌ Observability completeness: $VIOLATIONS plugin(s) missing one or more signals."
  red "   Either wire the missing signal, OR add // obs-exempt: <reason> with justification in the feature's plan.md 'Deferred Observability' section."
  exit 1
fi
