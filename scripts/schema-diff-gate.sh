#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/schema-diff-gate.json}"
OUT="${2:-results/generated/schema-diff}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.schema-diff-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null

for phrase in "reversible" "make schema-diff-gate"; do
  grep -F "$phrase" docs/schema-diff.md README.md > /dev/null
done

bash scripts/schema-diff.sh "$SPEC" "$OUT" > "$OUT.run.log"

# The script transforms A->B, its inverse transforms B->A, it is minimal, and the
# deliberately redundant script is detected as non-minimal.
jq -e '
  .version == "patchline.schema-diff/v1" and
  .forward_reproduces_b == true and
  .inverse_reproduces_a == true and
  .minimal == true and
  .redundant_is_minimal == false
' "$OUT/diff.json" > /dev/null

jq -n --slurpfile r "$OUT/diff.json" '{
  version: "patchline.schema-diff-gate-results/v1",
  reversible_both_ways: ($r[0].forward_reproduces_b and $r[0].inverse_reproduces_a),
  minimal: $r[0].minimal,
  redundant_rejected: ($r[0].redundant_is_minimal | not),
  verified: true
}' > "$OUT/gate-summary.json"

echo "schema-diff gate passed: edit script reversible both ways, minimal, redundant script rejected"
