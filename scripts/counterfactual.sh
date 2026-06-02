#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/counterfactual-gate.json}"
OUT="${2:-results/generated/counterfactual}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.counterfactual-gate/v1" and (.counterfactuals|length) >= 1' "$SPEC" > /dev/null

jq '
  # Verdict model: dropping a column without a backfill is unsafe; otherwise safe.
  def verdict($s): (if ($s.drops_column and ($s.has_backfill | not)) then "unsafe" else "safe" end);
  .baseline as $base
  | (verdict($base)) as $base_verdict
  | {
      version: "patchline.counterfactual/v1",
      baseline_verdict: $base_verdict,
      results: [ .counterfactuals[]
        | . as $cf
        | ($base + $cf.change) as $perturbed
        | (verdict($perturbed)) as $v
        | {
            name: $cf.name,
            kind: $cf.kind,
            perturbed_verdict: $v,
            flipped: ($v != $base_verdict),
            consistent: (
              if $cf.kind == "causal" then ($v != $base_verdict)
              else ($v == $base_verdict) end
            )
          } ]
    }
  | .all_consistent = (.results | all(.[]; .consistent))
' "$SPEC" > "$OUT/counterfactual.json"

{
  echo "# Counterfactual repair eval"
  echo
  echo "Baseline verdict: $(jq -r '.baseline_verdict' "$OUT/counterfactual.json")"
  echo
  jq -r '.results[] | "- \(.name) [\(.kind)]: verdict=\(.perturbed_verdict) flipped=\(.flipped) consistent=\(.consistent)"' "$OUT/counterfactual.json"
} > "$OUT/counterfactual.md"
cp "$OUT/counterfactual.md" "$OUT/README.md"

echo "counterfactual worker: all_consistent=$(jq -r '.all_consistent' "$OUT/counterfactual.json")"
