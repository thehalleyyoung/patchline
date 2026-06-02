#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/hosted-public-good-service-gate.json}"; OUT="${2:-results/generated/hosted-public-good-service}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.hosted-public-good-service-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "transparent cost" "make hosted-public-good-service-gate"; do grep -F "$phrase" docs/hosted-public-good-service.md README.md > /dev/null; done
bash scripts/hosted-public-good-service.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.hosted-public-good-service/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.hosted-public-good-service-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "hosted-public-good-service gate passed: every item scored with evidence on real self-data, unsupported item rejected"
