#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/proof-hole-graph-gate.json}"
OUT="${2:-results/generated/proof-hole-graph}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.proof-hole-graph-gate/v1" and (.evidence|length) >= 1' "$SPEC" > /dev/null

# Enumerate evidence subsets; keep dependency-satisfied subsets that meet the target;
# pick the minimum-cost feasible set (tie-break: fewer items, then lexicographic ids).
jq '
  def powerset: reduce .[] as $x ([[]]; . + map(. + [$x]));
  .evidence as $ev
  | (.current_uncertainty - .target_uncertainty) as $need
  | ([$ev[].id] | powerset) as $subsets
  | [ $subsets[]
      | . as $ids
      | ($ev | map(select(.id as $i | $ids | index($i)))) as $sel
      | ($sel | map(.reduction) | add // 0) as $tot
      | ($sel | all(.[]; .depends_on as $d | ($d | all(.[]; . as $dep | $ids | index($dep) != null)))) as $deps_ok
      | select(($tot >= $need) and $deps_ok)
      | { ids: ($ids | sort), cost: ($sel | map(.cost) | add // 0), size: ($ids | length), reduction: $tot }
    ] as $feasible
  | ($feasible | sort_by([.cost, .size, (.ids | join(","))])) as $ranked
  | {
      version: "patchline.proof-hole-graph/v1",
      need: $need,
      selected: ($ranked[0]),
      feasible_count: ($feasible | length)
    }
' "$SPEC" > "$OUT/proof-hole-graph.json"

{
  echo "# Proof-hole dependency graph: minimal evidence"
  echo
  echo "Required uncertainty reduction: $(jq -r .need "$OUT/proof-hole-graph.json")"
  echo
  echo "Selected evidence: $(jq -rc '.selected.ids' "$OUT/proof-hole-graph.json")"
  echo
  echo "Cost: $(jq -r '.selected.cost' "$OUT/proof-hole-graph.json"); reduction: $(jq -r '.selected.reduction' "$OUT/proof-hole-graph.json")"
} > "$OUT/proof-hole-graph.md"
cp "$OUT/proof-hole-graph.md" "$OUT/README.md"

echo "proof-hole-graph worker: selected $(jq -rc '.selected.ids' "$OUT/proof-hole-graph.json") cost=$(jq -r '.selected.cost' "$OUT/proof-hole-graph.json")"
