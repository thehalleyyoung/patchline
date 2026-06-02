#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/live-benchmark-anti-overfit-gate.json}"; OUT="${2:-results/generated/live-benchmark-anti-overfit}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.live-benchmark-anti-overfit-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "anti-overfitting audit" "make live-benchmark-anti-overfit-gate"; do grep -F "$phrase" docs/live-benchmark-anti-overfit.md README.md > /dev/null; done
bash scripts/live-benchmark-anti-overfit.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.live-benchmark-anti-overfit/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.live-benchmark-anti-overfit-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "live-benchmark-anti-overfit gate passed: every item scored with evidence on real self-data, unsupported item rejected"
