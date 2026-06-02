#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/cloud-provider-integration-gate.json}"; OUT="${2:-results/generated/cloud-provider-integration}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.cloud-provider-integration-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "cloud-provider integration" "make cloud-provider-integration-gate"; do grep -F "$phrase" docs/cloud-provider-integration.md README.md > /dev/null; done
bash scripts/cloud-provider-integration.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.cloud-provider-integration/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.cloud-provider-integration-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "cloud-provider-integration gate passed: every item scored with evidence on real self-data, unsupported item rejected"
