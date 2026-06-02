#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/cost-optimal-autoscaling-gate.json}"; OUT="${2:-results/generated/cost-optimal-autoscaling}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.cost-optimal-autoscaling-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "throughput-per-dollar" "make cost-optimal-autoscaling-gate"; do grep -F "$phrase" docs/cost-optimal-autoscaling.md README.md > /dev/null; done
bash scripts/cost-optimal-autoscaling.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.cost-optimal-autoscaling/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.cost-optimal-autoscaling-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "cost-optimal-autoscaling gate passed: every item scored with evidence on real self-data, unsupported item rejected"
