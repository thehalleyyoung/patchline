#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/human-timing-study-gate.json}"; OUT="${2:-results/generated/human-timing-study}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.human-timing-study-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "counterbalanced" "make human-timing-study-gate"; do grep -F "$phrase" docs/human-timing-study.md README.md > /dev/null; done
bash scripts/human-timing-study.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.human-timing-study/v1" and
  .balanced == true and .with_findings_faster == true and
  (.mean_reduction_sec > 100) and .unbalanced_is_balanced == false
' "$OUT/study.json" > /dev/null
jq -n --slurpfile r "$OUT/study.json" '{version:"patchline.human-timing-study-gate-results/v1", balanced:$r[0].balanced, mean_reduction_sec:$r[0].mean_reduction_sec, unbalanced_rejected:($r[0].unbalanced_is_balanced|not), verified:true}' > "$OUT/gate-summary.json"
echo "human-timing-study gate passed: balanced design, with-findings faster, unbalanced protocol rejected"
