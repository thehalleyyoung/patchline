#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/human-timing-study-gate.json}"; OUT="${2:-results/generated/human-timing-study}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.human-timing-study-gate/v1"' "$SPEC" > /dev/null
jq '
  def r2: (.*100|round)/100;
  def balanced($ps): $ps | all(.[]; (.without_sec != null) and (.with_sec != null));
  def mean_delta($ps): ([ $ps[] | (.without_sec - .with_sec) ] | add) / ($ps|length);
  {
    version: "patchline.human-timing-study/v1",
    balanced: balanced(.participants),
    n: (.participants|length),
    mean_reduction_sec: (mean_delta(.participants) | r2),
    with_findings_faster: (mean_delta(.participants) > 0),
    unbalanced_is_balanced: balanced(.unbalanced_participants)
  }
' "$SPEC" > "$OUT/study.json"
{ echo "# Reviewer timing study protocol"; echo; echo "Balanced: $(jq -r '.balanced' "$OUT/study.json"); mean reduction: $(jq -r '.mean_reduction_sec' "$OUT/study.json")s"; echo "Unbalanced protocol balanced: $(jq -r '.unbalanced_is_balanced' "$OUT/study.json")"; } > "$OUT/study.md"
cp "$OUT/study.md" "$OUT/README.md"
echo "human-timing-study worker: balanced=$(jq -r '.balanced' "$OUT/study.json") reduction=$(jq -r '.mean_reduction_sec' "$OUT/study.json")"
