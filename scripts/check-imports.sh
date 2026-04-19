#!/usr/bin/env bash
# Constitution Principle X — Dependency Hierarchy Respected
# Asserts: core → framework → plugins → transports → ui (no reverse imports)
#
# Specifically:
#   core/                              -> stdlib + core-internal only
#   framework/                         -> core + framework-internal + stdlib + 3rd party
#   plugins/<X>/                       -> core + framework + stdlib + 3rd party
#                                         BUT NOT plugins/<Y>/ (cross-plugin imports)
#   transports/                        -> core + framework + plugins (via plugin.Open) + stdlib + 3rd party
#
# Exit non-zero on any violation.

set -euo pipefail

VIOLATIONS=0

red()    { printf '\033[0;31m%s\033[0m\n' "$*"; }
green()  { printf '\033[0;32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[1;33m%s\033[0m\n' "$*"; }

# ---- core/ -----------------------------------------------------------------
yellow "Checking core/ imports..."
if grep -rE '"github.com/maximhq/bifrost/(framework|plugins|transports|ui)' \
        --include='*.go' core/ 2>/dev/null; then
  red "❌ core/ imports framework/, plugins/, transports/, or ui/ — violates Principle X"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# ---- framework/ ------------------------------------------------------------
yellow "Checking framework/ imports..."
if grep -rE '"github.com/maximhq/bifrost/(plugins|transports|ui)' \
        --include='*.go' framework/ 2>/dev/null; then
  red "❌ framework/ imports plugins/, transports/, or ui/ — violates Principle X"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# ---- plugins/ --------------------------------------------------------------
yellow "Checking plugins/ imports (no cross-plugin imports)..."
for plugin_dir in plugins/*/; do
  plugin_name=$(basename "$plugin_dir")
  # match imports of the form "github.com/maximhq/bifrost/plugins/<other>"
  # exclude self-imports of the same plugin
  if grep -rE "\"github.com/maximhq/bifrost/plugins/" --include='*.go' "$plugin_dir" 2>/dev/null \
       | grep -v "github.com/maximhq/bifrost/plugins/$plugin_name" \
       | grep -v "// allow-cross-plugin:" >/dev/null; then
    red "❌ plugins/$plugin_name imports another plugin — violates Principle X"
    grep -rE "\"github.com/maximhq/bifrost/plugins/" --include='*.go' "$plugin_dir" \
       | grep -v "github.com/maximhq/bifrost/plugins/$plugin_name" \
       | grep -v "// allow-cross-plugin:"
    VIOLATIONS=$((VIOLATIONS + 1))
  fi

  # plugins must not import transports or ui
  if grep -rE '"github.com/maximhq/bifrost/(transports|ui)' \
          --include='*.go' "$plugin_dir" 2>/dev/null; then
    red "❌ plugins/$plugin_name imports transports/ or ui/ — violates Principle X"
    VIOLATIONS=$((VIOLATIONS + 1))
  fi
done

# ---- transports/ -----------------------------------------------------------
yellow "Checking transports/ imports..."
if grep -rE '"github.com/maximhq/bifrost/ui' \
        --include='*.go' transports/ 2>/dev/null; then
  red "❌ transports/ imports ui/ — violates Principle X"
  VIOLATIONS=$((VIOLATIONS + 1))
fi

# ---- summary ---------------------------------------------------------------
if [ $VIOLATIONS -eq 0 ]; then
  green "✅ Import direction: clean (core→framework→plugins→transports→ui)"
  exit 0
else
  red "❌ Import direction: $VIOLATIONS violation(s) found"
  exit 1
fi
