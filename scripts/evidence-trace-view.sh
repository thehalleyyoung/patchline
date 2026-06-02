#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/evidence-trace-view-gate.json}"; OUT="${2:-results/generated/evidence-trace-view}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.evidence-trace-view-gate/v1"' "$SPEC" > /dev/null
jq '

  .nodes as $N | .dangling as $D
  | ([ $N[].id ]) as $ids
  | ([ $N[] | .deps[] ]) as $deps
  | ([ $deps[] | . as $d | ($ids | index($d)) != null ] | all) as $resolved
  | ([ $N[] | select((.deps|length)==0) | select((.span|length)>0) ]|length) as $grounded
  | ([ $N[] | select((.deps|length)==0) ]|length) as $leaves
  | ([ $D[] | .deps[] ] | map(. as $d | ($D|map(.id)|index($d))!=null) | all) as $dresolved
  | {version:"patchline.evidence-trace-view/v1",
     nodes:($N|length), resolved:$resolved,
     leaves:$leaves, grounded:$grounded,
     all_grounded:($grounded==$leaves),
     dangling_resolved:$dresolved}

' "$SPEC" > "$OUT/out.json"
{ echo "# Evidence-trace explanation view"; echo; echo "Nodes $(jq -r .nodes "$OUT/out.json"); grounded $(jq -r .grounded "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "evidence-trace-view worker: resolved=$(jq -r .resolved "$OUT/out.json") all_grounded=$(jq -r .all_grounded "$OUT/out.json")"
