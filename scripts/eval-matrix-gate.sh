#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/eval-matrix-gate.json}"
OUT="${2:-results/generated/eval-matrix-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.eval-matrix-gate/v1" and (.claim|length) > 200 and (.criteria|length) >= 1' "$SPEC" > /dev/null

for phrase in "evaluation matrix" "unsupported" "make eval-matrix-gate"; do
  grep -F "$phrase" docs/eval-matrix.md README.md > /dev/null
done

bash scripts/eval-matrix.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in eval-matrix.json eval-matrix.md README.md; do
  test -s "$OUT/$output"
done

# Every real criterion is backed by an existing artifact; the empty negative control is
# reported unsupported.
jq -e '
  .version == "patchline.eval-matrix/v1" and
  (.matrix | length) == 5 and
  .all_supported == true and
  (.unsupported | length) == 0 and
  (.matrix | all(.[]; .artifacts_present > 0)) and
  .negative_control.supported == false and
  .negative_control.artifacts_declared == 0
' "$OUT/eval-matrix.json" > /dev/null

jq -n --slurpfile r "$OUT/eval-matrix.json" '{
  version: "patchline.eval-matrix-gate-results/v1",
  all_supported: $r[0].all_supported,
  negative_control_unsupported: ($r[0].negative_control.supported | not),
  verified: true
}' > "$OUT/gate-summary.json"

echo "eval-matrix gate passed: all criteria artifact-backed, empty criterion reported unsupported"
