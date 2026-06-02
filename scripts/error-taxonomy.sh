#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/error-taxonomy-gate.json}"
OUT="${2:-results/generated/error-taxonomy}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.error-taxonomy-gate/v1" and (.errors|length) >= 6' "$SPEC" > /dev/null

# Render a stable, sorted taxonomy index keyed by code.
jq '{
  version: "patchline.error-taxonomy/v1",
  errors: (.errors | sort_by(.code)),
  stages: ([.errors[].stage] | unique | sort),
  count: (.errors | length),
  unique_codes: (([.errors[].code] | length) == ([.errors[].code] | unique | length))
}' "$SPEC" > "$OUT/error-taxonomy.json"

{
  echo "# Error taxonomy"
  echo
  echo "| Code | Stage | Retryable | Remediation |"
  echo "|---|---|---|---|"
  jq -r '.errors[] | "| `\(.code)` | \(.stage) | \(.retryable) | \(.remediation) |"' "$OUT/error-taxonomy.json"
} > "$OUT/error-taxonomy.md"
cp "$OUT/error-taxonomy.md" "$OUT/README.md"

echo "error-taxonomy worker: $(jq -r .count "$OUT/error-taxonomy.json") errors across $(jq -r '.stages|length' "$OUT/error-taxonomy.json") stages"
