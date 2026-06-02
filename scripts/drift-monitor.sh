#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/drift-monitor-gate.json}"; OUT="${2:-results/generated/drift-monitor}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.drift-monitor-gate/v1"' "$SPEC" > /dev/null
jq '
  def r6: (. * 1000000 | round) / 1000000;
  # Total-variation distance over the union of categories.
  def tvd($p; $q):
    (((($p|keys) + ($q|keys)) | unique)) as $cats
    | (0.5 * ([ $cats[] as $c | (($p[$c] // 0) - ($q[$c] // 0) | if . < 0 then -. else . end) ] | add)) | r6;
  .threshold as $t | .baseline as $b
  | {
      version: "patchline.drift-monitor/v1",
      threshold: $t,
      same_tvd: tvd($b; .same),
      same_drift: (tvd($b; .same) > $t),
      shifted_tvd: tvd($b; .shifted),
      shifted_drift: (tvd($b; .shifted) > $t)
    }
' "$SPEC" > "$OUT/drift.json"
{ echo "# Corpus drift monitor"; echo; echo "Same TVD: $(jq -r '.same_tvd' "$OUT/drift.json") drift=$(jq -r '.same_drift' "$OUT/drift.json")"; echo "Shifted TVD: $(jq -r '.shifted_tvd' "$OUT/drift.json") drift=$(jq -r '.shifted_drift' "$OUT/drift.json")"; } > "$OUT/drift.md"
cp "$OUT/drift.md" "$OUT/README.md"
echo "drift-monitor worker: same=$(jq -r '.same_tvd' "$OUT/drift.json") shifted=$(jq -r '.shifted_tvd' "$OUT/drift.json")"
