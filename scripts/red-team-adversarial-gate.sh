#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/red-team-adversarial-gate.json}"; OUT="${2:-results/generated/red-team-adversarial}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.red-team-adversarial-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "adversarial" "make red-team-adversarial-gate"; do grep -F "$phrase" docs/red-team-adversarial.md README.md > /dev/null; done
bash scripts/red-team-adversarial.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.red-team-adversarial/v1" and .all_caught==true and .evasion_rate==0 and .benign_clean==true' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.red-team-adversarial-gate-results/v1",evasion_rate:$r[0].evasion_rate,benign_clean:$r[0].benign_clean,verified:true}' > "$OUT/gate-summary.json"
echo "red-team-adversarial gate passed: zero evasions across adversarial suite, benign control clean"
