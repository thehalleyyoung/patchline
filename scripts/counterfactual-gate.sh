#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/counterfactual-gate.json}"
OUT="${2:-results/generated/counterfactual-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.counterfactual-gate/v1" and (.claim|length) > 200 and (.counterfactuals|length) >= 1' "$SPEC" > /dev/null

for phrase in "counterfactual" "causally" "make counterfactual-gate"; do
  grep -F "$phrase" docs/counterfactual.md README.md > /dev/null
done

bash scripts/counterfactual.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in counterfactual.json counterfactual.md README.md; do
  test -s "$OUT/$output"
done

# Removing the backfill (causal) flips safe->unsafe; editing the comment (irrelevant)
# does not flip the verdict.
jq -e '
  .version == "patchline.counterfactual/v1" and
  .baseline_verdict == "safe" and
  .all_consistent == true and
  ([.results[] | select(.name=="remove-backfill")][0] | .flipped == true and .perturbed_verdict == "unsafe") and
  ([.results[] | select(.name=="edit-comment")][0] | .flipped == false and .perturbed_verdict == "safe")
' "$OUT/counterfactual.json" > /dev/null

jq -n --slurpfile r "$OUT/counterfactual.json" '{
  version: "patchline.counterfactual-gate-results/v1",
  baseline_verdict: $r[0].baseline_verdict,
  all_consistent: $r[0].all_consistent,
  verified: true
}' > "$OUT/gate-summary.json"

echo "counterfactual gate passed: causal perturbation flips verdict, irrelevant edit does not"
