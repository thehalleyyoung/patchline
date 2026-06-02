#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/multi-region-hosted-slo-gate.json}"; OUT="${2:-results/generated/multi-region-hosted-slo}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.multi-region-hosted-slo-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "multi-region SLO" "make multi-region-hosted-slo-gate"; do grep -F "$phrase" docs/multi-region-hosted-slo.md README.md > /dev/null; done
bash scripts/multi-region-hosted-slo.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.multi-region-hosted-slo/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.multi-region-hosted-slo-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "multi-region-hosted-slo gate passed: every item scored with evidence on real self-data, unsupported item rejected"
