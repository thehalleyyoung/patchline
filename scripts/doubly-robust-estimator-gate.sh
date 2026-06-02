#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/doubly-robust-estimator-gate.json}"; OUT="${2:-results/generated/doubly-robust-estimator}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.doubly-robust-estimator-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "doubly-robust estimator" "make doubly-robust-estimator-gate"; do grep -F "$phrase" docs/doubly-robust-estimator.md README.md > /dev/null; done
bash scripts/doubly-robust-estimator.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.doubly-robust-estimator/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.doubly-robust-estimator-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "doubly-robust-estimator gate passed: every item scored with evidence on real self-data, unsupported item rejected"
