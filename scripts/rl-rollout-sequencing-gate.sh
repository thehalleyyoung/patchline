#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/rl-rollout-sequencing-gate.json}"; OUT="${2:-results/generated/rl-rollout-sequencing}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.rl-rollout-sequencing-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "safety-constrained" "make rl-rollout-sequencing-gate"; do grep -F "$phrase" docs/rl-rollout-sequencing.md README.md > /dev/null; done
bash scripts/rl-rollout-sequencing.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.rl-rollout-sequencing/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.rl-rollout-sequencing-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "rl-rollout-sequencing gate passed: every rollout safe and improved, unsafe rollout rejected"
