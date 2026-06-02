#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/interop-gate.json}"
OUT="${2:-results/generated/interop}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.interop-gate/v1" and (.findings|length) >= 1' "$SPEC" > /dev/null

# Native -> SARIF-style export.
jq '{
  "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
  version: "2.1.0",
  runs: [{
    tool: {driver: {name: "patchline"}},
    results: [ .findings[] | {
      ruleId: .rule,
      level: .level,
      message: {text: .message},
      locations: [{physicalLocation: {artifactLocation: {uri: .file}}}]
    }]
  }]
}' "$SPEC" > "$OUT/findings.sarif.json"

# SARIF -> native import (round-trip).
jq '[ .runs[0].results[] | {
  rule: .ruleId,
  level: .level,
  message: .message.text,
  file: .locations[0].physicalLocation.artifactLocation.uri
} ]' "$OUT/findings.sarif.json" > "$OUT/roundtrip.json"

# Validate a malformed SARIF doc (missing ruleId) is rejected.
jq '{runs:[{results:[{level:"error", message:{text:"x"}, locations:[{physicalLocation:{artifactLocation:{uri:"f.sql"}}}]}]}]}' "$SPEC" > "$OUT/invalid.sarif.json"
if jq -e '.runs[0].results | all(.[]; has("ruleId") and (.ruleId != null))' "$OUT/invalid.sarif.json" > /dev/null 2>&1; then
  invalid_rejected=false
else
  invalid_rejected=true
fi

# Compare native findings to round-tripped findings.
if jq -e --slurpfile orig "$SPEC" '. == ($orig[0].findings)' "$OUT/roundtrip.json" > /dev/null; then
  roundtrip_ok=true
else
  roundtrip_ok=false
fi

jq -n --argjson rt "$roundtrip_ok" --argjson inv "$invalid_rejected" \
  --slurpfile rtf "$OUT/roundtrip.json" '{
  version: "patchline.interop/v1",
  roundtrip_lossless: $rt,
  invalid_rejected: $inv,
  roundtripped: $rtf[0]
}' > "$OUT/interop.json"

{
  echo "# SARIF-style cross-tool interop"
  echo
  echo "Round-trip lossless: $(jq -r '.roundtrip_lossless' "$OUT/interop.json")"
  echo "Invalid interchange rejected: $(jq -r '.invalid_rejected' "$OUT/interop.json")"
} > "$OUT/interop.md"
cp "$OUT/interop.md" "$OUT/README.md"

echo "interop worker: roundtrip=$(jq -r '.roundtrip_lossless' "$OUT/interop.json") invalid_rejected=$(jq -r '.invalid_rejected' "$OUT/interop.json")"
