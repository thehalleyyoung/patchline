#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/case-bundle-gate.json}"
OUT="${2:-results/generated/case-bundle-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.case-bundle-gate/v1" and (.claim|length) > 200 and (.deep_count >= 1)' "$SPEC" > /dev/null

for phrase in "case-study bundle" "lightweight" "make case-bundle-gate"; do
  grep -F "$phrase" docs/case-bundle.md README.md > /dev/null
done

bash scripts/case-bundle.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in case-bundle.json case-bundle.md README.md; do
  test -s "$OUT/$output"
done

# 8 deep studies all qualify on narrative depth; 40 lightweight examples; the truncated
# shallow narrative would NOT qualify (negative control).
jq -e '
  .version == "patchline.case-bundle/v1" and
  .deep_qualified == 8 and
  .deep_total == 8 and
  .lightweight_total == 40 and
  (.deep | all(.[]; (.narrative | length) >= .min_narrative_chars or (.narrative|length) >= 120)) and
  (.shallow_rejected.would_qualify == false)
' "$OUT/case-bundle.json" > /dev/null

jq -n --slurpfile r "$OUT/case-bundle.json" '{
  version: "patchline.case-bundle-gate-results/v1",
  deep_qualified: $r[0].deep_qualified,
  lightweight_total: $r[0].lightweight_total,
  shallow_rejected: ($r[0].shallow_rejected.would_qualify | not),
  verified: true
}' > "$OUT/gate-summary.json"

echo "case-bundle gate passed: 8 deep studies qualified, 40 lightweight examples, shallow narrative rejected"
