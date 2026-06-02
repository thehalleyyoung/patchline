#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/interop-gate.json}"
OUT="${2:-results/generated/interop-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.interop-gate/v1" and (.claim|length) > 200 and (.findings|length) >= 1' "$SPEC" > /dev/null

for phrase in "cross-tool interop" "SARIF" "make interop-gate"; do
  grep -F "$phrase" docs/interop.md README.md > /dev/null
done

bash scripts/interop.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in interop.json interop.md README.md findings.sarif.json roundtrip.json; do
  test -s "$OUT/$output"
done

# SARIF export has the expected shape; round-trip is lossless; malformed doc rejected.
jq -e '.runs[0].results | length == 2 and all(.[]; has("ruleId") and has("level"))' "$OUT/findings.sarif.json" > /dev/null
jq -e --slurpfile orig "$SPEC" '. == $orig[0].findings' "$OUT/roundtrip.json" > /dev/null
jq -e '
  .version == "patchline.interop/v1" and
  .roundtrip_lossless == true and
  .invalid_rejected == true
' "$OUT/interop.json" > /dev/null

jq -n --slurpfile r "$OUT/interop.json" '{
  version: "patchline.interop-gate-results/v1",
  roundtrip_lossless: $r[0].roundtrip_lossless,
  invalid_rejected: $r[0].invalid_rejected,
  verified: true
}' > "$OUT/gate-summary.json"

echo "interop gate passed: SARIF round-trip lossless, malformed interchange document rejected"
